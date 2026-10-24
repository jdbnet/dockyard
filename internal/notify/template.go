package notify

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/jdbnet/dockyard/internal/config"
)

// Template variables: {{count}} {{scanned_at}} {{summary}} {{summary_oneline}} {{summary_br}} {{link}} {{names}} {{more}}
// {{summary}} uses real newlines (Discord, Slack, Teams, JSON). Custom webhooks render {{summary}} with <br> for HTML dashboards.

func DefaultMessageTemplate() string {
	return "{{count}} container image update(s)\n\n{{summary}}"
}

func DefaultTitleTemplate() string {
	return "Container image updates"
}

func renderTemplate(publicURL string, report UpdateReport, template string, htmlBreaks bool) string {
	if template == "" {
		return ""
	}
	vars := templateVars(publicURL, report, htmlBreaks)
	keys := make([]string, 0, len(vars))
	for k := range vars {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return len(keys[i]) > len(keys[j]) })
	out := template
	for _, key := range keys {
		out = strings.ReplaceAll(out, "{{"+key+"}}", vars[key])
	}
	return out
}

func renderedMessage(wh config.WebhookConfig, publicURL string, report UpdateReport) string {
	t := strings.TrimSpace(wh.MessageTemplate)
	if t == "" {
		t = DefaultMessageTemplate()
	}
	msg := renderTemplate(publicURL, report, t, false)
	link := updatesLink(publicURL)
	if link != "" && !strings.Contains(t, "{{link}}") && !strings.Contains(msg, link) {
		if wh.Type == "slack" {
			msg += "\n<" + link + "|Open updates>"
		} else if wh.Type == "discord" {
			msg += "\n\n[Open updates](" + link + ")"
		}
	}
	return msg
}

func renderedTitle(wh config.WebhookConfig, publicURL string, report UpdateReport) string {
	t := strings.TrimSpace(wh.TitleTemplate)
	if t == "" {
		t = DefaultTitleTemplate()
	}
	return renderTemplate(publicURL, report, t, false)
}

func templateVars(publicURL string, report UpdateReport, htmlBreaks bool) map[string]string {
	lines, more := summarizeLines(report.Pending)
	lineSep := "\n"
	moreSep := "\n"
	if htmlBreaks {
		lineSep = "<br>"
		moreSep = "<br>"
	}
	summary := strings.Join(lines, lineSep)
	if more > 0 {
		summary += fmt.Sprintf("%s…and %d more", moreSep, more)
	}
	summaryNewlines := strings.Join(lines, "\n")
	if more > 0 {
		summaryNewlines += fmt.Sprintf("\n…and %d more", more)
	}
	summaryBR := strings.Join(lines, "<br>")
	if more > 0 {
		summaryBR += fmt.Sprintf("<br>…and %d more", more)
	}
	oneline := make([]string, 0, len(report.Pending))
	names := make([]string, 0, len(report.Pending))
	for _, row := range report.Pending {
		oneline = append(oneline, fmt.Sprintf("%s (%s)", row.Name, row.Image))
		names = append(names, row.Name)
	}
	summaryOne := strings.Join(oneline, "; ")
	if len(oneline) > maxSummaryLines {
		summaryOne = strings.Join(oneline[:maxSummaryLines], "; ")
		if more > 0 {
			summaryOne += fmt.Sprintf("; and %d more", more)
		}
	}
	link := updatesLink(publicURL)
	moreStr := ""
	if more > 0 {
		moreStr = fmt.Sprintf("%d", more)
	}
	return map[string]string{
		"count":           fmt.Sprintf("%d", len(report.Pending)),
		"scanned_at":      report.ScannedAt.UTC().Format(time.RFC3339),
		"summary":         summary,
		"summary_plain":   summaryNewlines,
		"summary_br":      summaryBR,
		"summary_oneline": summaryOne,
		"link":            link,
		"names":           strings.Join(names, ", "),
		"more":            moreStr,
	}
}
