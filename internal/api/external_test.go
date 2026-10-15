package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jdbnet/dockyard/internal/config"
	"github.com/jdbnet/dockyard/internal/engine"
)

func TestNewAPIKeyStoresHashAndPrefix(t *testing.T) {
	record, raw, err := newAPIKey("dashboard", "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(raw, "dy_") || len(raw) < apiKeyPrefixLen {
		t.Fatalf("unexpected key %q", raw)
	}
	if record.Prefix != raw[:apiKeyPrefixLen] {
		t.Fatalf("prefix %q", record.Prefix)
	}
	if record.Hash == raw || record.Hash != hashAPIKey(raw) {
		t.Fatal("expected sha256 of the key, not the key itself")
	}
	if record.Scope != "read" {
		t.Fatalf("scope %q", record.Scope)
	}
}

func TestExternalAuthAndRateLimit(t *testing.T) {
	cfg := testConfig(false)
	record, raw, err := newAPIKey("reader", "read")
	if err != nil {
		t.Fatal(err)
	}
	cfg.APIKeys = []config.APIKey{record}
	s := NewServer(cfg, nil, "test")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/external/stats", nil)
	rec := httptest.NewRecorder()
	s.server.Handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("missing key: got %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/v1/external/containers/abc/start", nil)
	req.Header.Set("Authorization", "Bearer "+raw)
	rec = httptest.NewRecorder()
	s.server.Handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("read key start: got %d body %s", rec.Code, rec.Body.String())
	}

	var last int
	for i := 0; i < 61; i++ {
		req = httptest.NewRequest(http.MethodGet, "/api/v1/external/stats", nil)
		req.Header.Set("Authorization", "Bearer "+raw)
		rec = httptest.NewRecorder()
		s.server.Handler.ServeHTTP(rec, req)
		last = rec.Code
	}
	if last != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", last)
	}
}

func TestAPIKeyManagementRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte("web:\n  port: 8080\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.LoadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	cfg.Auth.Username = "admin"
	cfg.Auth.Password = "secret"
	s := NewServer(cfg, nil, "test")

	login := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"username":"admin","password":"secret"}`))
	loginRec := httptest.NewRecorder()
	s.server.Handler.ServeHTTP(loginRec, login)
	var cookie *http.Cookie
	for _, c := range loginRec.Result().Cookies() {
		if c.Name == sessionCookieName {
			cookie = c
		}
	}
	if cookie == nil {
		t.Fatal("missing session")
	}

	create := httptest.NewRequest(http.MethodPost, "/api/v1/settings/api-keys", strings.NewReader(`{"name":"dash","scope":"write"}`))
	create.AddCookie(cookie)
	createRec := httptest.NewRecorder()
	s.server.Handler.ServeHTTP(createRec, create)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", createRec.Code, createRec.Body.String())
	}
	var created struct {
		ID  string `json:"id"`
		Key string `json:"key"`
	}
	if err := json.Unmarshal(createRec.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.Key == "" || strings.Contains(createRec.Body.String(), hashAPIKey(created.Key)) {
		t.Fatal("response should include the raw key once and not the hash")
	}

	list := httptest.NewRequest(http.MethodGet, "/api/v1/settings/api-keys", nil)
	list.AddCookie(cookie)
	listRec := httptest.NewRecorder()
	s.server.Handler.ServeHTTP(listRec, list)
	if strings.Contains(listRec.Body.String(), created.Key) {
		t.Fatal("list must not include the raw key")
	}

	del := httptest.NewRequest(http.MethodDelete, "/api/v1/settings/api-keys/"+created.ID, nil)
	del.AddCookie(cookie)
	delRec := httptest.NewRecorder()
	s.server.Handler.ServeHTTP(delRec, del)
	if delRec.Code != http.StatusOK {
		t.Fatalf("delete: %d %s", delRec.Code, delRec.Body.String())
	}

	reloaded, err := config.LoadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(reloaded.APIKeys) != 0 {
		t.Fatalf("expected key removed from disk, got %d", len(reloaded.APIKeys))
	}
}

func TestSummarizeStats(t *testing.T) {
	now := time.Now()
	stats := summarizeStats([]engine.Container{
		{
			State: "running", CPUPct: 10, MemBytes: 100,
			Sparkline: []engine.StatPoint{
				{Timestamp: now.Add(-2 * time.Second), NetRx: 100, NetTx: 50},
				{Timestamp: now, NetRx: 300, NetTx: 90},
			},
		},
		{State: "exited"},
	})
	if stats.Containers != 2 || stats.Running != 1 || stats.CPUPct != 10 || stats.MemBytes != 100 {
		t.Fatalf("%+v", stats)
	}
	if stats.NetRxBps != 100 || stats.NetTxBps != 20 {
		t.Fatalf("rates %+v", stats)
	}
	if stats.States["running"] != 1 || stats.States["exited"] != 1 {
		t.Fatalf("states %+v", stats.States)
	}
}
