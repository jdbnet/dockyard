package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOriginAllowed(t *testing.T) {
	cases := []struct {
		origin  string
		reqHost string
		want    bool
	}{
		{"", "127.0.0.1:8080", true},
		{"http://127.0.0.1:8080", "127.0.0.1:8080", true},
		{"http://localhost:3000", "127.0.0.1:8080", true},
		{"http://127.0.0.1:3000", "192.168.1.5:8080", true},
		{"http://192.168.1.5:8080", "192.168.1.5:8080", true},
		{"http://evil.example", "192.168.1.5:8080", false},
		{"://bad", "127.0.0.1:8080", false},
	}
	for _, tc := range cases {
		got := originAllowed(tc.origin, tc.reqHost)
		if got != tc.want {
			t.Fatalf("originAllowed(%q, %q) = %v, want %v", tc.origin, tc.reqHost, got, tc.want)
		}
	}
}

func TestCORSReflectsAllowedOrigin(t *testing.T) {
	h := cors(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	req.Host = "127.0.0.1:8080"
	req.Header.Set("Origin", "http://localhost:3000")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Header().Get("Access-Control-Allow-Origin") != "http://localhost:3000" {
		t.Fatalf("unexpected ACAO: %q", rec.Header().Get("Access-Control-Allow-Origin"))
	}

	req = httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	req.Host = "192.168.1.5:8080"
	req.Header.Set("Origin", "http://evil.example")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatalf("expected no ACAO for disallowed origin, got %q", rec.Header().Get("Access-Control-Allow-Origin"))
	}
}
