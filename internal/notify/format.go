package notify

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/jdbnet/dockyard/internal/config"
)

const maxSummaryLines = 15

func BuildPayload(wh config.WebhookConfig, publicURL string, report UpdateReport) (payload, error) {
	switch wh.Type {
	case "custom":
		return buildCustom(wh, publicURL, report)
	case "discord":
		return buildDiscord(wh, publicURL, report)
	case "slack":
		return buildSlack(wh, publicURL, report)
	case "teams":
		return buildTeams(wh, publicURL, report)
	default:
		return payload{}, fmt.Errorf("unknown webhook type %q", wh.Type)
	}
}

func buildCustom(wh config.WebhookConfig, publicURL string, report UpdateReport) (payload, error) {
	if len(wh.CustomFields) == 0 {
		return payload{}, fmt.Errorf("custom webhook requires at least one field")
	}
	body := map[string]string{}
	for _, field := range wh.CustomFields {
		key := strings.TrimSpace(field.Key)
		if key == "" {
			continue
		}
		body[key] = renderTemplate(publicURL, report, field.Value)
	}
	if len(body) == 0 {
		return payload{}, fmt.Errorf("custom webhook requires at least one field with a key")
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return payload{}, err
	}
	return payload{body: raw, contentType: "application/json"}, nil
}

func summarizeLines(pending []PendingRow) (lines []string, more int) {
	for i, row := range pending {
		if i >= maxSummaryLines {
			more = len(pending) - maxSummaryLines
			break
		}
		lines = append(lines, fmt.Sprintf("• %s (%s)", row.Name, row.Image))
	}
	return lines, more
}

func updatesLink(publicURL string) string {
	publicURL = strings.TrimRight(strings.TrimSpace(publicURL), "/")
	if publicURL == "" {
		return ""
	}
	return publicURL + "/updates"
}

func buildDiscord(wh config.WebhookConfig, publicURL string, report UpdateReport) (payload, error) {
	desc := renderedMessage(wh, publicURL, report)
	title := renderedTitle(wh, publicURL, report)
	embed := map[string]any{
		"title":       title,
		"description": desc,
		"color":       0x1ebe8a,
	}
	body := map[string]any{
		"embeds": []any{embed},
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return payload{}, err
	}
	return payload{body: raw, contentType: "application/json"}, nil
}

func buildSlack(wh config.WebhookConfig, publicURL string, report UpdateReport) (payload, error) {
	text := renderedMessage(wh, publicURL, report)
	raw, err := json.Marshal(map[string]string{"text": text})
	if err != nil {
		return payload{}, err
	}
	return payload{body: raw, contentType: "application/json"}, nil
}

func buildTeams(wh config.WebhookConfig, publicURL string, report UpdateReport) (payload, error) {
	title := renderedTitle(wh, publicURL, report)
	text := renderedMessage(wh, publicURL, report)
	bodyItems := []any{
		map[string]any{
			"type":   "TextBlock",
			"size":   "Large",
			"weight": "Bolder",
			"text":   title,
			"wrap":   true,
		},
		map[string]any{
			"type": "TextBlock",
			"text": text,
			"wrap": true,
		},
	}
	link := updatesLink(publicURL)
	if link != "" && !strings.Contains(wh.MessageTemplate, "{{link}}") {
		bodyItems = append(bodyItems, map[string]any{
			"type": "ActionSet",
			"actions": []any{
				map[string]any{
					"type":  "Action.OpenUrl",
					"title": "Open updates",
					"url":   link,
				},
			},
		})
	}
	card := map[string]any{
		"$schema": "http://adaptivecards.io/schemas/adaptive-card.json",
		"type":    "AdaptiveCard",
		"version": "1.4",
		"body":    bodyItems,
	}
	msg := map[string]any{
		"type": "message",
		"attachments": []any{
			map[string]any{
				"contentType": "application/vnd.microsoft.card.adaptive",
				"content":     card,
			},
		},
	}
	raw, err := json.Marshal(msg)
	if err != nil {
		return payload{}, err
	}
	return payload{body: raw, contentType: "application/json"}, nil
}

func SampleReport() UpdateReport {
	now := time.Now().UTC()
	return UpdateReport{
		ScannedAt: now,
		Pending: []PendingRow{
			{
				ContainerID:  "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
				ShortID:      "aaaaaaaaaaaa",
				Name:         "sample-app",
				Image:        "ghcr.io/example/app:latest",
				LocalDigest:  "sha256:abc",
				RemoteDigest: "sha256:def",
			},
			{
				ContainerID:  "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
				ShortID:      "bbbbbbbbbbbb",
				Name:         "sample-db",
				Image:        "postgres:16",
				LocalDigest:  "sha256:111",
				RemoteDigest: "sha256:222",
			},
		},
	}
}
