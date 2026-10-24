package notify

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/jdbnet/dockyard/internal/config"
)

func TestDiscordTruncation(t *testing.T) {
	pending := make([]PendingRow, 0, 20)
	for i := 0; i < 20; i++ {
		pending = append(pending, PendingRow{
			Name:  "svc-" + string(rune('a'+i)),
			Image: "img:latest",
		})
	}
	report := UpdateReport{Pending: pending}
	wh := config.WebhookConfig{Type: "discord"}
	p, err := buildDiscord(wh, "https://dockyard.example", report)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(p.body), "and 5 more") {
		t.Fatalf("expected truncation marker, got %s", p.body)
	}
}

func TestTeamsAdaptiveCardJSON(t *testing.T) {
	wh := config.WebhookConfig{Type: "teams"}
	p, err := buildTeams(wh, "", SampleReport())
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(p.body, &doc); err != nil {
		t.Fatal(err)
	}
	if doc["type"] != "message" {
		t.Fatalf("unexpected payload %v", doc["type"])
	}
}

func TestSummaryMultilineForDiscord(t *testing.T) {
	wh := config.WebhookConfig{Type: "discord", MessageTemplate: "{{summary}}"}
	msg := renderedMessage(wh, "", SampleReport())
	if !strings.Contains(msg, "\n") {
		t.Fatalf("expected newlines in summary, got %q", msg)
	}
	if strings.Contains(msg, "<br>") {
		t.Fatal("discord summary should not use br tags")
	}
}

func TestCustomSummaryUsesBR(t *testing.T) {
	wh := config.WebhookConfig{
		Type: "custom",
		CustomFields: []config.WebhookField{
			{Key: "message", Value: "{{summary}}"},
		},
	}
	p, err := buildCustom(wh, "", SampleReport())
	if err != nil {
		t.Fatal(err)
	}
	// json.Marshal escapes < as \u003c
	if !strings.Contains(string(p.body), "br") {
		t.Fatalf("expected br in custom summary, got %s", p.body)
	}
	var decoded map[string]string
	if err := json.Unmarshal(p.body, &decoded); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(decoded["message"], "<br>") {
		t.Fatalf("expected <br> after decode, got %q", decoded["message"])
	}
}

func TestCustomFieldsJSONEscape(t *testing.T) {
	report := UpdateReport{
		Pending: []PendingRow{{
			Name:  `app "quotes"`,
			Image: "img:latest",
		}},
	}
	wh := config.WebhookConfig{
		Type: "custom",
		CustomFields: []config.WebhookField{
			{Key: "message", Value: `Hello {{names}}`},
			{Key: "category", Value: "updates"},
		},
	}
	p, err := buildCustom(wh, "", report)
	if err != nil {
		t.Fatal(err)
	}
	var body map[string]string
	if err := json.Unmarshal(p.body, &body); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(body["message"], `"quotes"`) {
		t.Fatalf("expected escaped content in parsed JSON, got %q", body["message"])
	}
	if body["category"] != "updates" {
		t.Fatalf("got %q", body["category"])
	}
}

func TestCustomMessageTemplateOnSlack(t *testing.T) {
	wh := config.WebhookConfig{
		Type:            "slack",
		MessageTemplate: "Alert: {{count}} pending — {{names}}",
	}
	p, err := buildSlack(wh, "", SampleReport())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(p.body), "Alert: 2 pending") {
		t.Fatalf("got %s", p.body)
	}
}
