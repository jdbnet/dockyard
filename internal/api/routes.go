package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"git.jdbnet.co.uk/jamie/dockyard/internal/docker"
	"git.jdbnet.co.uk/jamie/dockyard/internal/engine"
)

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func (s *Server) handleListContainers(w http.ResponseWriter, r *http.Request) {
	if r.URL.Query().Get("group") == "compose" {
		s.handleListCompose(w, r)
		return
	}
	list, err := s.eng.Containers(r.Context())
	if err != nil {
		actionError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *Server) handleListCompose(w http.ResponseWriter, r *http.Request) {
	list, err := s.eng.ComposeProjects(r.Context())
	if err != nil {
		actionError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *Server) handleGetContainer(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	list, err := s.eng.Containers(r.Context())
	if err != nil {
		actionError(w, err)
		return
	}
	for _, c := range list {
		if c.ID == id || c.ShortID == id || c.Name == id {
			raw, _ := s.eng.Inspect(r.Context(), c.ID)
			writeJSON(w, http.StatusOK, map[string]any{
				"container": c,
				"stats":     s.eng.ContainerStats(c.ID),
				"inspect":   json.RawMessage(raw),
			})
			return
		}
	}
	http.Error(w, "not found", http.StatusNotFound)
}

func (s *Server) handleContainerStats(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	stats := s.eng.ContainerStats(id)
	writeJSON(w, http.StatusOK, stats)
}

func (s *Server) handleContainerLogs(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	tail := r.URL.Query().Get("tail")
	if tail == "" {
		tail = "200"
	}
	lines, err := s.eng.LogLines(r.Context(), id, docker.LogOptions{Tail: tail})
	if err != nil {
		actionError(w, err)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	for _, line := range lines {
		_, _ = io.WriteString(w, line+"\n")
	}
}

func (s *Server) handleListImages(w http.ResponseWriter, r *http.Request) {
	list, err := s.eng.Images(r.Context())
	if err != nil {
		actionError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *Server) handleListVolumes(w http.ResponseWriter, r *http.Request) {
	list, err := s.eng.Volumes(r.Context())
	if err != nil {
		actionError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *Server) handleListNetworks(w http.ResponseWriter, r *http.Request) {
	list, err := s.eng.Networks(r.Context())
	if err != nil {
		actionError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func streamLogs(ctx context.Context, eng *engine.Engine, id string, w io.Writer) error {
	return eng.StreamLogs(ctx, id, docker.LogOptions{Tail: "100", Follow: true}, func(line string) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			_, err := io.WriteString(w, line+"\n")
			return err
		}
	})
}

func matchContainerID(list []engine.Container, id string) string {
	for _, c := range list {
		if c.ID == id || c.ShortID == id || strings.EqualFold(c.Name, id) {
			return c.ID
		}
	}
	return id
}
