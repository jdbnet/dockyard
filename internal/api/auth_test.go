package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"git.jdbnet.co.uk/jamie/dockyard/internal/config"
)

func testConfig(auth bool) *config.Config {
	cfg, _ := config.LoadConfig("")
	if auth {
		cfg.Auth.Username = "admin"
		cfg.Auth.Password = "secret"
	} else {
		cfg.Auth.Username = ""
		cfg.Auth.Password = ""
	}
	return cfg
}

func TestAuthDisabledAllowsAPI(t *testing.T) {
	s := NewServer(testConfig(false), nil, "1.2.3")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/status", nil)
	rec := httptest.NewRecorder()
	s.server.Handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var status map[string]bool
	if err := json.Unmarshal(rec.Body.Bytes(), &status); err != nil {
		t.Fatal(err)
	}
	if status["auth_required"] {
		t.Fatal("expected auth not required")
	}
}

func TestAuthRequiredBlocksWithoutCookie(t *testing.T) {
	s := NewServer(testConfig(true), nil, "1.2.3")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/containers", nil)
	rec := httptest.NewRecorder()
	s.server.Handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestAuthLoginSetsCookie(t *testing.T) {
	s := NewServer(testConfig(true), nil, "1.2.3")

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"username":"admin","password":"secret"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.server.Handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var cookie *http.Cookie
	for _, c := range rec.Result().Cookies() {
		if c.Name == sessionCookieName {
			cookie = c
			break
		}
	}
	if cookie == nil || cookie.Value == "" {
		t.Fatal("expected session cookie")
	}

	req2 := httptest.NewRequest(http.MethodGet, "/api/v1/auth/status", nil)
	req2.AddCookie(cookie)
	rec2 := httptest.NewRecorder()
	s.server.Handler.ServeHTTP(rec2, req2)

	var status map[string]bool
	if err := json.Unmarshal(rec2.Body.Bytes(), &status); err != nil {
		t.Fatal(err)
	}
	if !status["authenticated"] {
		t.Fatalf("expected authenticated status: %+v", status)
	}
}
