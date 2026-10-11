package logging

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jdbnet/dockyard/internal/config"
)

type fakeSource struct {
	targets []Target
	follow  func(ctx context.Context, id, since string, emit func(Entry) error) error
}

func (f *fakeSource) Targets(context.Context) ([]Target, error) {
	return f.targets, nil
}

func (f *fakeSource) Follow(ctx context.Context, id, since string, emit func(Entry) error) error {
	if f.follow != nil {
		return f.follow(ctx, id, since, emit)
	}
	<-ctx.Done()
	return ctx.Err()
}

func TestAuthPauseDoesNotRetryUntilSettingsSaved(t *testing.T) {
	for _, code := range []int{http.StatusUnauthorized, http.StatusForbidden} {
		t.Run(http.StatusText(code), func(t *testing.T) {
			var calls atomic.Int32
			var allow atomic.Bool
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				if !allow.Load() {
					w.WriteHeader(code)
					return
				}
				w.WriteHeader(http.StatusNoContent)
			}))
			defer srv.Close()

			s := testShipper(t, config.LoggingConfig{Enabled: true, URL: srv.URL, Type: "gologs", Auth: "bearer", Token: "bad"})
			if _, err := s.spool.Append(Record{Payload: []byte(`{"streams":[]}`), Cursors: map[string]Cursor{"c": {Timestamp: 1, Count: 1}}}); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			go s.runPush(ctx)

			waitFor(t, func() bool { return s.Status().State == "paused_auth" && s.Status().PauseCode == code })
			if s.Status().ErrorCount != 0 {
				t.Fatalf("auth pause incremented errors: %d", s.Status().ErrorCount)
			}
			pausedCalls := calls.Load()
			time.Sleep(150 * time.Millisecond)
			if calls.Load() != pausedCalls {
				t.Fatalf("retried while paused: %d calls", calls.Load())
			}

			allow.Store(true)
			s.Update(config.LoggingConfig{Enabled: true, URL: srv.URL, Type: "gologs", Auth: "bearer", Token: "good", CollectByDefault: true})
			waitFor(t, func() bool { return s.Status().State == "running" && !s.Status().LastSuccess.IsZero() })
			if _, ok := s.spool.Peek(); ok {
				t.Fatal("batch was not acknowledged after resume")
			}
		})
	}
}

func TestReconcileSkipsSelfAndUnsupportedDrivers(t *testing.T) {
	self := "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789"
	followed := make(chan string, 1)
	src := &fakeSource{
		targets: []Target{
			{ID: self, Name: "dockyard", State: "running", LogDriver: "json-file"},
			{ID: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", Name: "syslogd", State: "running", LogDriver: "syslog"},
			{ID: "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc", Name: "optin", State: "running", LogDriver: "local", Labels: map[string]string{"dockyard.logs": "true"}},
		},
		follow: func(ctx context.Context, id, since string, emit func(Entry) error) error {
			followed <- id
			<-ctx.Done()
			return ctx.Err()
		},
	}
	s := testShipper(t, config.LoggingConfig{Enabled: true, URL: "http://logs.example/loki/api/v1/push", CollectByDefault: true})
	s.source = src
	s.selfIDs = []string{self[:12]}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	s.mu.Lock()
	s.root = ctx
	s.mu.Unlock()
	s.reconcile(ctx)

	select {
	case id := <-followed:
		if id != "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc" {
			t.Fatalf("followed %s", id)
		}
	case <-time.After(time.Second):
		t.Fatal("opted-in container was not followed")
	}
	status := s.Status()
	if status.ErrorCount != 0 {
		t.Fatalf("error count = %d", status.ErrorCount)
	}
	if status.Followed != 1 {
		t.Fatalf("followed = %d", status.Followed)
	}
	if len(status.NotCollectable) != 1 || status.NotCollectable[0].Name != "syslogd" || status.NotCollectable[0].Reason != "log driver syslog" {
		t.Fatalf("not collectable = %#v", status.NotCollectable)
	}
}

func TestFollowFailureIsNotAnError(t *testing.T) {
	src := &fakeSource{
		targets: []Target{{ID: "c1", Name: "app", State: "running", LogDriver: "json-file"}},
		follow: func(context.Context, string, string, func(Entry) error) error {
			return errors.New("broken stream")
		},
	}
	s := testShipper(t, config.LoggingConfig{Enabled: true, URL: "http://logs.example/loki/api/v1/push", CollectByDefault: true})
	s.source = src
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	s.mu.Lock()
	s.root = ctx
	s.mu.Unlock()
	s.reconcile(ctx)
	waitFor(t, func() bool {
		status := s.Status()
		return len(status.NotCollectable) == 1 && status.Followed == 0
	})
	if s.Status().ErrorCount != 0 {
		t.Fatalf("error count = %d", s.Status().ErrorCount)
	}
	if s.Status().NotCollectable[0].Reason == "" {
		t.Fatal("missing reason")
	}
}

func TestHandoffDropsWithoutBlocking(t *testing.T) {
	s := testShipper(t, config.LoggingConfig{})
	if !s.handoff.Submit(Record{Payload: []byte("queued")}) {
		t.Fatal("expected the handoff to accept one record")
	}
	start := time.Now()
	s.submit(&Batch{Payload: []byte("dropped"), Cursors: map[string]Cursor{"c": {Timestamp: 9, Count: 2}}})
	if time.Since(start) > 50*time.Millisecond {
		t.Fatal("submit blocked")
	}
	if s.Status().Dropped != 1 {
		t.Fatalf("dropped = %d", s.Status().Dropped)
	}
	if s.cursors.Mem("c").Timestamp != 9 || s.cursors.Mem("c").Count != 2 {
		t.Fatalf("cursor = %+v", s.cursors.Mem("c"))
	}
}

func TestSpoolDropsOldest(t *testing.T) {
	sp := newSpool(filepath.Join(t.TempDir(), "spool.json"), 10)
	if _, err := sp.Append(Record{Payload: []byte(`{"o":1}`), Cursors: map[string]Cursor{"old": {Timestamp: 1, Count: 1}}}); err != nil {
		t.Fatal(err)
	}
	dropped, err := sp.Append(Record{Payload: []byte(`{"n":2}`), Cursors: map[string]Cursor{"new": {Timestamp: 2, Count: 1}}})
	if err != nil {
		t.Fatal(err)
	}
	if len(dropped) != 1 || dropped[0].Cursors["old"].Timestamp != 1 {
		t.Fatalf("dropped = %#v", dropped)
	}
	got, ok := sp.Peek()
	if !ok || string(got.Payload) != `{"n":2}` {
		t.Fatalf("head = %q %v", got.Payload, ok)
	}
}

func testShipper(t *testing.T, settings config.LoggingConfig) *Shipper {
	t.Helper()
	if settings.Type == "" {
		settings.Type = "gologs"
	}
	if settings.Auth == "" {
		settings.Auth = "none"
	}
	s, err := New(Options{Dir: t.TempDir(), Settings: settings, Host: "box"})
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func waitFor(t *testing.T, ready func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if ready() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("timed out waiting for condition")
}
