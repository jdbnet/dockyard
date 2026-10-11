package api

import (
	"encoding/json"
	"net/http"

	"github.com/jdbnet/dockyard/internal/engine"
)

func (s *Server) handleGetSettings(w http.ResponseWriter, r *http.Request) {
	if s.eng == nil {
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
		return
	}
	writeJSON(w, http.StatusOK, s.eng.GeneralSettings())
}

func (s *Server) handlePutSettings(w http.ResponseWriter, r *http.Request) {
	if s.eng == nil {
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
		return
	}
	var update engine.GeneralUpdate
	if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	view, err := s.eng.UpdateGeneral(update)
	if err != nil {
		actionError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, view)
}
