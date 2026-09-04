package api

import (
	"context"
	"encoding/json"
	"net/http"
)

type stackCreateRequest struct {
	Name    string `json:"name"`
	Content string `json:"content"`
	Start   bool   `json:"start"`
}

type stackSaveRequest struct {
	Content string `json:"content"`
}

func (s *Server) handleListStacks(w http.ResponseWriter, r *http.Request) {
	list, err := s.eng.Stacks(r.Context())
	if err != nil {
		actionError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *Server) handleGetStackCompose(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	info, err := s.eng.GetStackCompose(r.Context(), name)
	if err != nil {
		actionError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, info)
}

func (s *Server) handleSaveStackCompose(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	var req stackSaveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		actionError(w, err)
		return
	}
	if err := s.eng.SaveStackCompose(r.Context(), name, req.Content); err != nil {
		actionError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleCreateStack(w http.ResponseWriter, r *http.Request) {
	var req stackCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		actionError(w, err)
		return
	}
	if err := s.eng.CreateStack(r.Context(), req.Name, req.Content, req.Start); err != nil {
		actionError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "name": req.Name})
}

func (s *Server) handleStackAction(w http.ResponseWriter, r *http.Request, fn func(context.Context, string) error) {
	name := r.PathValue("name")
	if err := fn(r.Context(), name); err != nil {
		actionError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleUpdateContainer(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	result, err := s.eng.UpdateContainer(r.Context(), id)
	if err != nil {
		actionError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status":          "ok",
		"previous_image": result.PreviousImage,
	})
}

func (s *Server) handleDeleteStack(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if err := s.eng.DeleteStack(r.Context(), name); err != nil {
		actionError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleStackUp(w http.ResponseWriter, r *http.Request) {
	s.handleStackAction(w, r, s.eng.StackUp)
}

func (s *Server) handleStackDown(w http.ResponseWriter, r *http.Request) {
	s.handleStackAction(w, r, s.eng.StackDown)
}

func (s *Server) handleStackUpdate(w http.ResponseWriter, r *http.Request) {
	s.handleStackAction(w, r, s.eng.UpdateStack)
}
