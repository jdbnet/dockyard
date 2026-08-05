package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

func (s *Server) authEnabled() bool {
	return s.cfg.AuthEnabled()
}

func (s *Server) isPublicPath(path string) bool {
	switch path {
	case "/api/v1/auth/login", "/api/v1/auth/logout", "/api/v1/auth/status", "/api/v1/health":
		return true
	}
	return false
}

func (s *Server) validSession(r *http.Request) bool {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil {
		return false
	}
	return cookie.Value != "" && cookie.Value == s.sessionToken
}

func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.authEnabled() || s.isPublicPath(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}

		if strings.HasPrefix(r.URL.Path, "/api/") || strings.HasPrefix(r.URL.Path, "/ws/") {
			if !s.validSession(r) {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
		}

		next.ServeHTTP(w, r)
	})
}

func (s *Server) handleAuthStatus(w http.ResponseWriter, r *http.Request) {
	resp := map[string]bool{
		"auth_required": s.authEnabled(),
		"authenticated": !s.authEnabled() || s.validSession(r),
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleAuthLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if !s.authEnabled() {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
		return
	}

	var creds struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&creds); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	if creds.Username != s.cfg.Auth.Username || creds.Password != s.cfg.Auth.Password {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    s.sessionToken,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Now().Add(7 * 24 * time.Hour),
	})
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleAuthLogout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Now().Add(-1 * time.Hour),
	})
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
