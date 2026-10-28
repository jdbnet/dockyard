package api

import (
	"context"
	_ "embed"
	"net/http"
	"strings"
	"time"

	"github.com/jdbnet/dockyard/internal/engine"
)

//go:embed openapi.yaml
var openAPISpec []byte

type externalContainer struct {
	ID             string   `json:"id"`
	Name           string   `json:"name"`
	Image          string   `json:"image"`
	State          string   `json:"state"`
	Status         string   `json:"status"`
	Uptime         string   `json:"uptime"`
	Ports          []string `json:"ports"`
	CPUPct         float64  `json:"cpu_pct"`
	MemBytes       uint64   `json:"mem_bytes"`
	MemPct         float64  `json:"mem_pct"`
	AutoUpdate     bool     `json:"auto_update"`
	UpdateInterval string   `json:"update_interval,omitempty"`
}

type externalStats struct {
	Containers int            `json:"containers"`
	Running    int            `json:"running"`
	CPUPct     float64        `json:"cpu_pct"`
	MemBytes   uint64         `json:"mem_bytes"`
	NetRxBps   float64        `json:"net_rx_bps"`
	NetTxBps   float64        `json:"net_tx_bps"`
	States     map[string]int `json:"states"`
}

func toExternalContainer(c engine.Container) externalContainer {
	ports := c.Ports
	if ports == nil {
		ports = []string{}
	}
	return externalContainer{
		ID:             c.ID,
		Name:           c.Name,
		Image:          c.Image,
		State:          c.State,
		Status:         c.Status,
		Uptime:         c.Uptime,
		Ports:          ports,
		CPUPct:         c.CPUPct,
		MemBytes:       c.MemBytes,
		MemPct:         c.MemPct,
		AutoUpdate:     c.AutoUpdate,
		UpdateInterval: c.UpdateInterval,
	}
}

func summarizeStats(list []engine.Container) externalStats {
	out := externalStats{States: map[string]int{}}
	var rx, tx float64
	for _, c := range list {
		out.Containers++
		state := c.State
		if state == "" {
			state = "unknown"
		}
		out.States[state]++
		if c.State != "running" {
			continue
		}
		out.Running++
		out.CPUPct += c.CPUPct
		out.MemBytes += c.MemBytes
		rx += sampleRate(c.Sparkline, true)
		tx += sampleRate(c.Sparkline, false)
	}
	out.NetRxBps = rx
	out.NetTxBps = tx
	return out
}

func sampleRate(points []engine.StatPoint, rx bool) float64 {
	if len(points) < 2 {
		return 0
	}
	prev := points[len(points)-2]
	last := points[len(points)-1]
	dt := last.Timestamp.Sub(prev.Timestamp).Seconds()
	if dt <= 0 {
		return 0
	}
	var delta float64
	if rx {
		delta = float64(last.NetRx) - float64(prev.NetRx)
	} else {
		delta = float64(last.NetTx) - float64(prev.NetTx)
	}
	if delta < 0 {
		return 0
	}
	return delta / dt
}

func findContainer(list []engine.Container, id string) (engine.Container, bool) {
	for _, c := range list {
		if c.ID == id || c.ShortID == id || strings.EqualFold(c.Name, id) {
			return c, true
		}
	}
	return engine.Container{}, false
}

func (s *Server) handleExternalStats(w http.ResponseWriter, r *http.Request) {
	if s.eng == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "unavailable"})
		return
	}
	list, err := s.eng.Containers(r.Context())
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, summarizeStats(list))
}

func (s *Server) handleExternalContainers(w http.ResponseWriter, r *http.Request) {
	if s.eng == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "unavailable"})
		return
	}
	list, err := s.eng.Containers(r.Context())
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	out := make([]externalContainer, 0, len(list))
	for _, c := range list {
		out = append(out, toExternalContainer(c))
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleExternalContainer(w http.ResponseWriter, r *http.Request) {
	if s.eng == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "unavailable"})
		return
	}
	list, err := s.eng.Containers(r.Context())
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	c, ok := findContainer(list, r.PathValue("id"))
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	writeJSON(w, http.StatusOK, toExternalContainer(c))
}

func (s *Server) handleOpenAPI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/yaml")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(openAPISpec)
}

func (s *Server) handleExternalStart(w http.ResponseWriter, r *http.Request) {
	s.externalAction(w, r, func(ctx context.Context, id string) error {
		return s.eng.Start(ctx, id)
	})
}

func (s *Server) handleExternalStop(w http.ResponseWriter, r *http.Request) {
	s.externalAction(w, r, func(ctx context.Context, id string) error {
		return s.eng.Stop(ctx, id)
	})
}

func (s *Server) handleExternalRestart(w http.ResponseWriter, r *http.Request) {
	s.externalAction(w, r, func(ctx context.Context, id string) error {
		return s.eng.Restart(ctx, id)
	})
}

func (s *Server) externalAction(w http.ResponseWriter, r *http.Request, fn func(context.Context, string) error) {
	if s.eng == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "unavailable"})
		return
	}
	id := r.PathValue("id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing id"})
		return
	}
	if err := fn(r.Context(), id); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "id": id})
}

func externalWrite(scope, method string) bool {
	if method == http.MethodGet || method == http.MethodHead {
		return true
	}
	return scope == "write"
}

func (s *Server) allowExternal(keyID string) bool {
	s.keysMu.Lock()
	defer s.keysMu.Unlock()
	if s.keyWindows == nil {
		s.keyWindows = map[string]keyWindow{}
	}
	now := time.Now()
	win := s.keyWindows[keyID]
	if win.start.IsZero() || now.Sub(win.start) >= time.Minute {
		win = keyWindow{start: now}
	}
	if win.count >= 60 {
		s.keyWindows[keyID] = win
		return false
	}
	win.count++
	s.keyWindows[keyID] = win
	return true
}
