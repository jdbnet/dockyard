package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"time"

	"git.jdbnet.co.uk/jamie/dockyard/internal/config"
	"git.jdbnet.co.uk/jamie/dockyard/internal/engine"
)

const sessionCookieName = "dockyard_session"

type Server struct {
	cfg          *config.Config
	eng          *engine.Engine
	version      string
	server       *http.Server
	mux          *http.ServeMux
	sessionToken string
}

func NewServer(cfg *config.Config, eng *engine.Engine, version string) *Server {
	mux := http.NewServeMux()
	token := make([]byte, 32)
	_, _ = rand.Read(token)
	s := &Server{
		cfg:          cfg,
		eng:          eng,
		version:      version,
		mux:          mux,
		sessionToken: hex.EncodeToString(token),
		server: &http.Server{
			Addr:              cfg.Addr(),
			Handler:           nil,
			ReadHeaderTimeout: 10 * time.Second,
		},
	}
	s.registerRoutes()
	s.server.Handler = cors(s.authMiddleware(mux))
	return s
}

func cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) Start() error {
	return s.server.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.server.Shutdown(ctx)
}

func (s *Server) registerRoutes() {
	s.mux.HandleFunc("GET /api/v1/health", s.handleHealth)
	s.mux.HandleFunc("GET /api/v1/auth/status", s.handleAuthStatus)
	s.mux.HandleFunc("POST /api/v1/auth/login", s.handleAuthLogin)
	s.mux.HandleFunc("POST /api/v1/auth/logout", s.handleAuthLogout)

	s.mux.HandleFunc("GET /api/v1/containers", s.handleListContainers)
	s.mux.HandleFunc("GET /api/v1/containers/{id}", s.handleGetContainer)
	s.mux.HandleFunc("GET /api/v1/containers/{id}/stats", s.handleContainerStats)
	s.mux.HandleFunc("GET /api/v1/containers/{id}/logs", s.handleContainerLogs)
	s.mux.HandleFunc("POST /api/v1/containers/{id}/start", s.handleContainerStart)
	s.mux.HandleFunc("POST /api/v1/containers/{id}/stop", s.handleContainerStop)
	s.mux.HandleFunc("POST /api/v1/containers/{id}/restart", s.handleContainerRestart)
	s.mux.HandleFunc("DELETE /api/v1/containers/{id}", s.handleContainerRemove)

	s.mux.HandleFunc("GET /api/v1/stacks", s.handleListStacks)
	s.mux.HandleFunc("GET /api/v1/stacks/{name}/compose", s.handleGetStackCompose)
	s.mux.HandleFunc("PUT /api/v1/stacks/{name}/compose", s.handleSaveStackCompose)
	s.mux.HandleFunc("POST /api/v1/stacks", s.handleCreateStack)
	s.mux.HandleFunc("POST /api/v1/stacks/{name}/up", s.handleStackUp)
	s.mux.HandleFunc("POST /api/v1/stacks/{name}/down", s.handleStackDown)
	s.mux.HandleFunc("POST /api/v1/stacks/{name}/update", s.handleStackUpdate)
	s.mux.HandleFunc("DELETE /api/v1/stacks/{name}", s.handleDeleteStack)
	s.mux.HandleFunc("POST /api/v1/containers/{id}/update", s.handleUpdateContainer)

	s.mux.HandleFunc("GET /api/v1/compose", s.handleListCompose)
	s.mux.HandleFunc("GET /api/v1/images", s.handleListImages)
	s.mux.HandleFunc("POST /api/v1/images/prune", s.handlePruneImages)
	s.mux.HandleFunc("DELETE /api/v1/images/{id}", s.handleRemoveImage)
	s.mux.HandleFunc("GET /api/v1/volumes", s.handleListVolumes)
	s.mux.HandleFunc("DELETE /api/v1/volumes/{name}", s.handleRemoveVolume)
	s.mux.HandleFunc("GET /api/v1/networks", s.handleListNetworks)
	s.mux.HandleFunc("GET /api/v1/ports", s.handleListPorts)
	s.mux.HandleFunc("DELETE /api/v1/networks/{id}", s.handleRemoveNetwork)

	s.mux.HandleFunc("GET /ws/events", s.handleWSEvents)
	s.mux.HandleFunc("GET /ws/stats", s.handleWSStats)
	s.mux.HandleFunc("GET /ws/logs/{id}", s.handleWSLogs)

	s.mux.HandleFunc("GET /", s.handleSPA)
	s.mux.HandleFunc("GET /{$}", s.handleSPA)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if err := s.eng.DockerPing(r.Context()); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "error", "error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{
		"status":  "ok",
		"docker":  "connected",
		"version": s.version,
	})
}

func actionError(w http.ResponseWriter, err error) {
	writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
}

func containerAction(w http.ResponseWriter, r *http.Request, fn func(context.Context, string) error) {
	id := r.PathValue("id")
	if id == "" {
		http.Error(w, "missing id", http.StatusBadRequest)
		return
	}
	if err := fn(r.Context(), id); err != nil {
		actionError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "id": id})
}

func (s *Server) handleContainerStart(w http.ResponseWriter, r *http.Request) {
	containerAction(w, r, s.eng.Start)
}

func (s *Server) handleContainerStop(w http.ResponseWriter, r *http.Request) {
	containerAction(w, r, s.eng.Stop)
}

func (s *Server) handleContainerRestart(w http.ResponseWriter, r *http.Request) {
	containerAction(w, r, s.eng.Restart)
}

func (s *Server) handleContainerRemove(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := s.eng.Remove(r.Context(), id, true); err != nil {
		actionError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleRemoveImage(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := s.eng.RemoveImage(r.Context(), id, true); err != nil {
		actionError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handlePruneImages(w http.ResponseWriter, r *http.Request) {
	result, err := s.eng.PruneUnusedImages(r.Context())
	if err != nil {
		actionError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleRemoveVolume(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if err := s.eng.RemoveVolume(r.Context(), name, true); err != nil {
		actionError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleRemoveNetwork(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := s.eng.RemoveNetwork(r.Context(), id); err != nil {
		actionError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func formatAddr(cfg *config.Config) string {
	return fmt.Sprintf("http://%s", cfg.Addr())
}
