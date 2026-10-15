package api

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/jdbnet/dockyard/internal/config"
)

const (
	apiKeyPrefix    = "dy_"
	apiKeyPrefixLen = 12 // "dy_" plus 8 hex digits
)

func hashAPIKey(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func newAPIKey(name, scope string) (config.APIKey, string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return config.APIKey{}, "", fmt.Errorf("name is required")
	}
	switch scope {
	case "":
		scope = "read"
	case "read", "write":
	default:
		return config.APIKey{}, "", fmt.Errorf("scope must be read or write")
	}

	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return config.APIKey{}, "", err
	}
	idBytes := make([]byte, 16)
	if _, err := rand.Read(idBytes); err != nil {
		return config.APIKey{}, "", err
	}
	raw := apiKeyPrefix + hex.EncodeToString(secret)
	prefix := raw
	if len(prefix) > apiKeyPrefixLen {
		prefix = prefix[:apiKeyPrefixLen]
	}
	record := config.APIKey{
		ID:        hex.EncodeToString(idBytes),
		Name:      name,
		Prefix:    prefix,
		Hash:      hashAPIKey(raw),
		Scope:     scope,
		CreatedAt: time.Now().UTC().Truncate(time.Second),
	}
	return record, raw, nil
}

func (s *Server) lookupAPIKey(raw string) (config.APIKey, bool) {
	hash := hashAPIKey(raw)
	s.keysMu.Lock()
	defer s.keysMu.Unlock()
	for _, key := range s.cfg.APIKeys {
		if key.Hash == hash {
			return key, true
		}
	}
	return config.APIKey{}, false
}

func (s *Server) touchAPIKey(id string) {
	s.keysMu.Lock()
	defer s.keysMu.Unlock()
	now := time.Now().UTC().Truncate(time.Second)
	for i := range s.cfg.APIKeys {
		if s.cfg.APIKeys[i].ID != id {
			continue
		}
		s.cfg.APIKeys[i].LastUsedAt = &now
		if s.keyPersisted == nil {
			s.keyPersisted = map[string]time.Time{}
		}
		if now.Sub(s.keyPersisted[id]) < time.Minute {
			return
		}
		s.keyPersisted[id] = now
		_ = s.cfg.SaveAPIKeys()
		return
	}
}

func (s *Server) handleListAPIKeys(w http.ResponseWriter, r *http.Request) {
	s.keysMu.Lock()
	keys := append([]config.APIKey(nil), s.cfg.APIKeys...)
	s.keysMu.Unlock()
	if keys == nil {
		keys = []config.APIKey{}
	}
	writeJSON(w, http.StatusOK, keys)
}

func (s *Server) handleCreateAPIKey(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name  string `json:"name"`
		Scope string `json:"scope"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	record, raw, err := newAPIKey(body.Name, body.Scope)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	s.keysMu.Lock()
	s.cfg.APIKeys = append(s.cfg.APIKeys, record)
	err = s.cfg.SaveAPIKeys()
	if err != nil {
		s.cfg.APIKeys = s.cfg.APIKeys[:len(s.cfg.APIKeys)-1]
	}
	s.keysMu.Unlock()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"id":         record.ID,
		"name":       record.Name,
		"prefix":     record.Prefix,
		"scope":      record.Scope,
		"created_at": record.CreatedAt,
		"key":        raw,
	})
}

func (s *Server) handleDeleteAPIKey(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	s.keysMu.Lock()
	defer s.keysMu.Unlock()
	next := make([]config.APIKey, 0, len(s.cfg.APIKeys))
	found := false
	for _, key := range s.cfg.APIKeys {
		if key.ID == id {
			found = true
			continue
		}
		next = append(next, key)
	}
	if !found {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	if next == nil {
		next = []config.APIKey{}
	}
	s.cfg.APIKeys = next
	delete(s.keyPersisted, id)
	if err := s.cfg.SaveAPIKeys(); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
