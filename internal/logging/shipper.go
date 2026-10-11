package logging

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/jdbnet/dockyard/internal/config"
)

// Shipper follows container logs and pushes them to a Loki-compatible endpoint.
type Shipper struct {
	dir     string
	host    string
	selfIDs []string
	source  Source
	client  *http.Client
	now     func() time.Time
	batcher *Batcher
	spool   *spool
	cursors *cursorStore
	handoff *handoff
	wake    chan struct{}
	root    context.Context

	mu            sync.Mutex
	settings      config.LoggingConfig
	paused        bool
	pauseCode     int
	errorCount    int64
	dropped       int64
	lastError     string
	lastSuccess   time.Time
	follows       map[string]followSlot
	failed        map[string]Skipped
	driverSkips   []Skipped
	runningIDs    map[string]struct{}
	down          map[string]bool
	baseline      map[string]struct{}
	baselineReady bool
	token         int
}

type followSlot struct {
	token  int
	cancel context.CancelFunc
}

// Options configures a shipper.
type Options struct {
	Dir      string
	Settings config.LoggingConfig
	Source   Source
	Host     string
	SelfIDs  []string
	Client   *http.Client
	Now      func() time.Time
}

// New prepares the spool directory and loads any unsent batches.
func New(opts Options) (*Shipper, error) {
	host := opts.Host
	if host == "" {
		host, _ = os.Hostname()
	}
	now := opts.Now
	if now == nil {
		now = time.Now
	}
	settings, err := Normalise(opts.Settings)
	if err != nil {
		settings = opts.Settings
		if settings.Type == "" {
			settings.Type = "gologs"
		}
		if settings.Auth == "" {
			settings.Auth = "none"
		}
	}
	s := &Shipper{
		dir:        opts.Dir,
		host:       host,
		selfIDs:    opts.SelfIDs,
		source:     opts.Source,
		client:     opts.Client,
		now:        now,
		batcher:    newBatcher(defaultBatchBytes, defaultBatchWait),
		handoff:    newHandoff(),
		wake:       make(chan struct{}, 1),
		settings:   settings,
		follows:    map[string]followSlot{},
		failed:     map[string]Skipped{},
		runningIDs: map[string]struct{}{},
		down:       map[string]bool{},
		baseline:   map[string]struct{}{},
		spool:      newSpool("", defaultSpoolBytes),
		cursors:    newCursorStore(""),
	}
	if err := s.load(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Shipper) load() error {
	if s.dir == "" {
		return nil
	}
	if err := os.MkdirAll(s.dir, 0o700); err != nil {
		return err
	}
	s.spool = newSpool(filepath.Join(s.dir, "spool.json"), defaultSpoolBytes)
	s.cursors = newCursorStore(filepath.Join(s.dir, "cursors.json"))
	if err := s.spool.Load(); err != nil {
		return err
	}
	if err := s.cursors.Load(); err != nil {
		return err
	}
	s.cursors.Overlay(s.spool.Snapshot())
	return nil
}

// Run reconciles containers and pushes until ctx is cancelled.
func (s *Shipper) Run(ctx context.Context, kicks <-chan struct{}) {
	s.mu.Lock()
	s.root = ctx
	s.mu.Unlock()

	go s.runWriter(ctx)
	go s.runPush(ctx)
	go s.runFlush(ctx)
	s.reconcile(ctx)

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			s.stopAll()
			s.submit(s.batcher.Flush())
			return
		case <-ticker.C:
			s.reconcile(ctx)
		case <-kicks:
			s.reconcile(ctx)
		}
	}
}

func (s *Shipper) runWriter(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case rec := <-s.handoff.ch:
			dropped, err := s.spool.Append(rec)
			if err != nil {
				s.noteFailure(err.Error())
			}
			if len(dropped) > 0 {
				cursors := map[string]Cursor{}
				for _, rec := range dropped {
					for id, cur := range rec.Cursors {
						cursors[id] = Later(cursors[id], cur)
					}
				}
				_ = s.cursors.Commit(cursors)
				s.addDropped(int64(len(dropped)))
			}
			s.poke()
		}
	}
}

func (s *Shipper) runFlush(ctx context.Context) {
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			s.submit(s.batcher.Flush())
			return
		case <-ticker.C:
			s.submit(s.batcher.FlushDue(s.now()))
		}
	}
}

func (s *Shipper) submit(batch *Batch) {
	if batch == nil || len(batch.Payload) == 0 {
		return
	}
	rec := Record{Payload: batch.Payload, Cursors: batch.Cursors}
	if s.handoff.Submit(rec) {
		return
	}
	_ = s.cursors.Commit(rec.Cursors)
	s.addDropped(1)
}

func (s *Shipper) poke() {
	select {
	case s.wake <- struct{}{}:
	default:
	}
}

// Update stores new settings and clears an authentication pause.
func (s *Shipper) Update(settings config.LoggingConfig) {
	wasEnabled := s.snapshotSettings().Enabled
	s.mu.Lock()
	s.settings = settings
	s.paused = false
	s.pauseCode = 0
	if settings.Enabled && !wasEnabled {
		s.baselineReady = false
		s.baseline = map[string]struct{}{}
	}
	s.mu.Unlock()
	s.poke()
}

// Settings returns the saved logging configuration, including secrets.
func (s *Shipper) Settings() config.LoggingConfig {
	return s.snapshotSettings()
}

// Test pushes one line with the draft settings and does not clear an authentication pause.
func (s *Shipper) Test(ctx context.Context, draft config.LoggingConfig) error {
	body, err := TestPayload(s.host)
	if err != nil {
		return err
	}
	callCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	result := postPush(callCtx, s.client, draft.URL, draft.Auth, draft.Username, draft.Password, draft.Token, body)
	if result.Err != nil {
		return result.Err
	}
	if result.Status == http.StatusUnauthorized || result.Status == http.StatusForbidden {
		return errAuth(result.Status)
	}
	if result.Status < 200 || result.Status >= 300 {
		return errStatus(result)
	}
	return nil
}

func (s *Shipper) Status() Status {
	buffered := s.spool.Size()
	s.mu.Lock()
	defer s.mu.Unlock()
	state := "running"
	switch {
	case !s.settings.Enabled:
		state = "disabled"
	case s.paused:
		state = "paused_auth"
	}
	skipped := make([]Skipped, 0, len(s.driverSkips)+len(s.failed))
	seen := map[string]struct{}{}
	for _, item := range s.driverSkips {
		skipped = append(skipped, item)
		seen[item.ID] = struct{}{}
	}
	for id, item := range s.failed {
		if _, ok := s.runningIDs[id]; !ok {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		skipped = append(skipped, item)
	}
	sort.Slice(skipped, func(i, j int) bool {
		if skipped[i].Name == skipped[j].Name {
			return skipped[i].ID < skipped[j].ID
		}
		return skipped[i].Name < skipped[j].Name
	})
	st := Status{
		State:          state,
		LastError:      s.lastError,
		ErrorCount:     s.errorCount,
		BufferedBytes:  buffered,
		Dropped:        s.dropped,
		Followed:       len(s.follows),
		NotCollectable: skipped,
	}
	if s.paused {
		st.PauseCode = s.pauseCode
	}
	st.LastSuccess = s.lastSuccess
	return st
}

func (s *Shipper) reconcile(ctx context.Context) {
	settings := s.snapshotSettings()
	if !settings.Enabled || settings.URL == "" || s.source == nil {
		if settings.Enabled && settings.URL == "" {
			s.setLastError("destination URL is required")
		}
		s.stopAll()
		s.mu.Lock()
		s.driverSkips = nil
		s.runningIDs = map[string]struct{}{}
		s.mu.Unlock()
		return
	}
	targets, err := s.source.Targets(ctx)
	if err != nil {
		s.setLastError(err.Error())
		return
	}

	s.mu.Lock()
	if !s.baselineReady {
		s.baseline = map[string]struct{}{}
		for _, t := range targets {
			s.baseline[t.ID] = struct{}{}
		}
		s.baselineReady = true
	}
	running := map[string]struct{}{}
	for _, t := range targets {
		running[t.ID] = struct{}{}
		if s.down[t.ID] {
			delete(s.failed, t.ID)
			s.down[t.ID] = false
		}
	}
	for id := range s.down {
		if _, ok := running[id]; !ok {
			s.down[id] = true
		}
	}
	for id := range s.failed {
		if _, ok := running[id]; !ok {
			s.down[id] = true
		}
	}
	s.runningIDs = running

	var driverSkips []Skipped
	want := map[string]Target{}
	for _, t := range targets {
		decision := Decide(t, settings.CollectByDefault, s.selfIDs)
		if decision.NotCollectable != "" {
			driverSkips = append(driverSkips, Skipped{ID: t.ID, Name: t.Name, Reason: decision.NotCollectable})
			continue
		}
		if !decision.Collect {
			continue
		}
		if failed, ok := s.failed[t.ID]; ok {
			driverSkips = append(driverSkips, failed)
			continue
		}
		want[t.ID] = t
	}
	s.driverSkips = driverSkips
	for id := range s.follows {
		if _, ok := want[id]; !ok {
			s.stopFollowLocked(id)
		}
	}
	root := s.root
	var start []Target
	for id, t := range want {
		if _, ok := s.follows[id]; ok {
			continue
		}
		start = append(start, t)
	}
	s.mu.Unlock()

	for _, target := range start {
		s.startFollow(root, target)
	}
}

func (s *Shipper) startFollow(root context.Context, t Target) {
	if root == nil {
		return
	}
	s.mu.Lock()
	if _, ok := s.follows[t.ID]; ok {
		s.mu.Unlock()
		return
	}
	s.token++
	token := s.token
	ctx, cancel := context.WithCancel(root)
	s.follows[t.ID] = followSlot{token: token, cancel: cancel}
	_, fromNow := s.baseline[t.ID]
	s.mu.Unlock()
	go s.follow(ctx, t, token, fromNow)
}

func (s *Shipper) follow(ctx context.Context, t Target, token int, fromNow bool) {
	defer s.clearFollow(t.ID, token)
	tries := 0
	for {
		if ctx.Err() != nil {
			return
		}
		cur := s.cursors.Mem(t.ID)
		since := ""
		if cur.Timestamp > 0 {
			since = SinceString(cur)
		} else if fromNow {
			since = s.now().UTC().Format(time.RFC3339Nano)
		}
		resume := NewResume(cur)
		err := s.source.Follow(ctx, t.ID, since, func(entry Entry) error {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			ts, text, ok := ParseDockerLine(entry.Line)
			if !ok {
				ts = s.now().UnixNano()
				text = entry.Line
			}
			if len(text) > maxLineBytes {
				s.addDropped(1)
				return nil
			}
			if !resume.Keep(ts) {
				return nil
			}
			next := resume.Cursor()
			s.cursors.SetMem(t.ID, next)
			if batch := s.batcher.Add(s.now(), Item{
				ContainerID: t.ID,
				Cursor:      next,
				Labels:      StreamLabels(s.host, t, entry.Stream),
				TS:          ts,
				Text:        text,
			}); batch != nil {
				s.submit(batch)
			}
			return nil
		})
		if ctx.Err() != nil {
			return
		}
		if !s.stillRunning(t.ID) {
			return
		}
		if tries >= 1 {
			reason := "follow returned no log data"
			if err != nil {
				reason = "follow failed: " + err.Error()
			}
			s.failCollect(t.ID, t.Name, reason)
			return
		}
		tries++
	}
}

func (s *Shipper) failCollect(id, name, reason string) {
	s.mu.Lock()
	s.failed[id] = Skipped{ID: id, Name: name, Reason: reason}
	s.mu.Unlock()
}

func (s *Shipper) stillRunning(id string) bool {
	if s.source != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		targets, err := s.source.Targets(ctx)
		cancel()
		if err == nil {
			for _, target := range targets {
				if target.ID == id && (target.State == "" || target.State == "running") {
					return true
				}
			}
			s.mu.Lock()
			s.down[id] = true
			delete(s.runningIDs, id)
			s.mu.Unlock()
			return false
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.runningIDs[id]
	return ok
}

func (s *Shipper) clearFollow(id string, token int) {
	s.mu.Lock()
	if slot, ok := s.follows[id]; ok && slot.token == token {
		delete(s.follows, id)
	}
	s.mu.Unlock()
}

func (s *Shipper) stopAll() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for id := range s.follows {
		s.stopFollowLocked(id)
	}
}

func (s *Shipper) stopFollowLocked(id string) {
	slot, ok := s.follows[id]
	if !ok {
		return
	}
	delete(s.follows, id)
	slot.cancel()
}

func (s *Shipper) snapshotSettings() config.LoggingConfig {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.settings
}

func (s *Shipper) pushEnabled() bool {
	settings := s.snapshotSettings()
	return settings.Enabled && settings.URL != ""
}

func (s *Shipper) authPaused() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.paused
}

func (s *Shipper) pauseAuth(code int) {
	s.mu.Lock()
	s.paused = true
	s.pauseCode = code
	s.lastError = authMessage(code)
	s.mu.Unlock()
}

func (s *Shipper) noteSuccess() {
	s.mu.Lock()
	s.lastSuccess = s.now()
	s.lastError = ""
	s.paused = false
	s.pauseCode = 0
	s.mu.Unlock()
}

func (s *Shipper) noteFailure(msg string) {
	s.mu.Lock()
	s.errorCount++
	s.lastError = msg
	s.mu.Unlock()
}

func (s *Shipper) setLastError(msg string) {
	s.mu.Lock()
	s.lastError = msg
	s.mu.Unlock()
}

func (s *Shipper) addDropped(n int64) {
	s.mu.Lock()
	s.dropped += n
	s.mu.Unlock()
}

func authMessage(code int) string {
	return "authentication failed (" + itoa(code) + ")"
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [16]byte
	i := len(b)
	neg := n < 0
	if neg {
		n = -n
	}
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}

type statusError struct {
	msg string
}

func (e statusError) Error() string { return e.msg }

func errAuth(code int) error {
	return statusError{msg: authMessage(code)}
}

func errStatus(result pushResult) error {
	return statusError{msg: pushFailure(result)}
}
