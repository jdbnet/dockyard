package api

import (
	"encoding/json"
	"net/http"

	"github.com/jdbnet/dockyard/internal/notify"
)

func (s *Server) handleGetNotifications(w http.ResponseWriter, r *http.Request) {
	if s.eng == nil {
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
		return
	}
	writeJSON(w, http.StatusOK, s.eng.NotificationSettings())
}

func (s *Server) handlePutNotifications(w http.ResponseWriter, r *http.Request) {
	if s.eng == nil {
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
		return
	}
	var update notify.SettingsUpdate
	if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	view, err := s.eng.UpdateNotifications(update)
	if err != nil {
		actionError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, view)
}

func (s *Server) handleTestNotifications(w http.ResponseWriter, r *http.Request) {
	if s.eng == nil {
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
		return
	}
	var req notify.TestRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	if err := s.eng.TestNotification(r.Context(), req); err != nil {
		actionError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}
