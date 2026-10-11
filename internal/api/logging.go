package api

import (
	"encoding/json"
	"net/http"

	"github.com/jdbnet/dockyard/internal/logging"
)

func (s *Server) handleGetLogging(w http.ResponseWriter, r *http.Request) {
	if s.eng == nil {
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
		return
	}
	writeJSON(w, http.StatusOK, s.eng.LoggingSettings())
}

func (s *Server) handlePutLogging(w http.ResponseWriter, r *http.Request) {
	if s.eng == nil {
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
		return
	}
	var update logging.SettingsUpdate
	if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	view, err := s.eng.UpdateLogging(update)
	if err != nil {
		actionError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, view)
}

func (s *Server) handleTestLogging(w http.ResponseWriter, r *http.Request) {
	if s.eng == nil {
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
		return
	}
	var update logging.SettingsUpdate
	if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	if err := s.eng.TestLogging(r.Context(), update); err != nil {
		actionError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleLoggingStatus(w http.ResponseWriter, r *http.Request) {
	if s.eng == nil {
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
		return
	}
	writeJSON(w, http.StatusOK, s.eng.LoggingStatus())
}
