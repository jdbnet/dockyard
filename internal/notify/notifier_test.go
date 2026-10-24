package notify

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jdbnet/dockyard/internal/config"
)

func TestMaybeNotifyInstantSkipsUnchangedFingerprint(t *testing.T) {
	var posts atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		posts.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	cfg := config.NotificationsConfig{
		Enabled:  true,
		Schedule: config.NotifyScheduleConfig{Mode: ScheduleInstant},
		Webhooks: []config.WebhookConfig{{
			ID: "1", Name: "t", Type: "custom", URL: srv.URL, Enabled: true,
			CustomFields: []config.WebhookField{{Key: "message", Value: "{{count}}"}},
		}},
	}
	report := SampleReport()
	n := New(cfg, func() UpdateReport { return report })
	ctx := context.Background()

	n.MaybeNotifyInstant(ctx, report)
	n.MaybeNotifyInstant(ctx, report)
	if posts.Load() != 1 {
		t.Fatalf("expected one post, got %d", posts.Load())
	}
}

func TestNotifySkipsEmptyPending(t *testing.T) {
	var posts atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		posts.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	cfg := config.NotificationsConfig{
		Enabled: true,
		Webhooks: []config.WebhookConfig{{
			ID: "1", Name: "t", Type: "custom", URL: srv.URL, Enabled: true,
			CustomFields: []config.WebhookField{{Key: "message", Value: "ok"}},
		}},
	}
	n := New(cfg, func() UpdateReport { return UpdateReport{} })
	if err := n.Notify(context.Background(), UpdateReport{}); err != nil {
		t.Fatal(err)
	}
	if posts.Load() != 0 {
		t.Fatal("expected no posts")
	}
}

func TestPostWebhookRetriesOn503(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	wh := config.WebhookConfig{
		URL: srv.URL, Type: "custom", Auth: "none",
		CustomFields: []config.WebhookField{{Key: "message", Value: "test"}},
	}
	p, err := buildCustom(wh, "", SampleReport())
	if err != nil {
		t.Fatal(err)
	}
	if err := postWebhook(context.Background(), nil, wh, p); err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 2 {
		t.Fatalf("expected 2 calls, got %d", calls.Load())
	}
}

func TestResolveTestWebhookUsesStoredURL(t *testing.T) {
	stored := config.NotificationsConfig{
		Webhooks: []config.WebhookConfig{{
			ID: "abc", URL: "https://example.test/hook", Token: "secret", Auth: "bearer", Type: "custom",
		}},
	}
	wh, ok := resolveTestWebhook(stored, TestRequest{
		WebhookID: "abc",
		Webhook:   &WebhookUpdate{ID: "abc", URL: "", Token: ""},
	})
	if !ok || wh.URL != "https://example.test/hook" || wh.Token != "secret" {
		t.Fatalf("got %+v", wh)
	}
}

func TestIntervalNotifierFiresWithUnchangedPending(t *testing.T) {
	var posts atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		posts.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	report := SampleReport()
	cfg := config.NotificationsConfig{
		Enabled: true,
		Schedule: config.NotifyScheduleConfig{
			Mode:     ScheduleInterval,
			Interval: config.Duration{50 * time.Millisecond},
		},
		Webhooks: []config.WebhookConfig{{
			ID: "1", Name: "t", Type: "custom", URL: srv.URL, Enabled: true,
			CustomFields: []config.WebhookField{{Key: "message", Value: "{{count}}"}},
		}},
	}
	n := New(cfg, func() UpdateReport { return report })
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go n.runSchedule(ctx)
	time.Sleep(120 * time.Millisecond)
	cancel()
	if posts.Load() < 1 {
		t.Fatalf("expected at least one post, got %d", posts.Load())
	}
}
