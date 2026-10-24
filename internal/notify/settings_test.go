package notify

import (
	"testing"

	"github.com/jdbnet/dockyard/internal/config"
)

func TestNormaliseScheduleModes(t *testing.T) {
	cfg := config.NotificationsConfig{
		Enabled:  true,
		Schedule: config.NotifyScheduleConfig{Mode: "daily", At: "10:30"},
		Webhooks: []config.WebhookConfig{{
			ID: "1", Name: "x", Type: "discord", URL: "https://example.test", Enabled: true,
		}},
	}
	out, err := Normalise(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if out.Schedule.Mode != ScheduleDaily || out.Schedule.At != "10:30" {
		t.Fatalf("%+v", out.Schedule)
	}
}

func TestMergeUpdateKeepsWebhookSecrets(t *testing.T) {
	current := config.NotificationsConfig{
		Webhooks: []config.WebhookConfig{{
			ID: "a", URL: "https://secret.test", Token: "tok", Type: "custom",
		}},
	}
	next := MergeUpdate(current, SettingsUpdate{
		Webhooks: []WebhookUpdate{{ID: "a", Name: "n", Type: "custom", URL: "", Token: ""}},
	})
	if next.Webhooks[0].URL != "https://secret.test" || next.Webhooks[0].Token != "tok" {
		t.Fatalf("secrets not preserved: %+v", next.Webhooks[0])
	}
}
