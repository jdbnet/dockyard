package tui

import (
	"context"
	"strings"

	"github.com/jdbnet/dockyard/internal/docker"
	"github.com/jdbnet/dockyard/internal/engine"
	tea "github.com/charmbracelet/bubbletea"
)

type logFollowMsg struct {
	line string
	err  error
	done bool
}

type logFollowStartedMsg struct {
	id     string
	ch     <-chan logFollowMsg
	cancel context.CancelFunc
}

type logFollowLineMsg string

type logFollowDoneMsg struct {
	err error
}

func (m *model) stopLogFollow() {
	if m.logFollowCancel != nil {
		m.logFollowCancel()
		m.logFollowCancel = nil
	}
	m.logFollowCh = nil
	m.logContainerID = ""
}

func dockerLogOpts(tail string, follow bool) docker.LogOptions {
	return docker.LogOptions{Tail: tail, Follow: follow}
}

func startLogFollow(eng *engine.Engine, id string) tea.Cmd {
	ch := make(chan logFollowMsg, 128)
	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		defer close(ch)
		err := eng.StreamLogs(ctx, id, dockerLogOpts("500", true), func(line string) error {
			select {
			case ch <- logFollowMsg{line: line}:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		})
		select {
		case <-ctx.Done():
		case ch <- logFollowMsg{done: true, err: err}:
		}
	}()

	return func() tea.Msg {
		return logFollowStartedMsg{id: id, ch: ch, cancel: cancel}
	}
}

func waitLogFollow(ch <-chan logFollowMsg) tea.Cmd {
	return func() tea.Msg {
		msg, ok := <-ch
		if !ok {
			return logFollowDoneMsg{}
		}
		if msg.done {
			return logFollowDoneMsg{err: msg.err}
		}
		return logFollowLineMsg(msg.line)
	}
}

func splitDockerTimestamp(line string) (ts, msg string, ok bool) {
	if len(line) < 21 || line[4] != '-' || line[10] != 'T' {
		return "", line, false
	}
	sp := strings.IndexByte(line, ' ')
	if sp <= 0 || sp > 35 {
		return "", line, false
	}
	return line[:sp], line[sp+1:], true
}

func formatLogLine(line string, showTS bool) string {
	if showTS {
		return line
	}
	if _, msg, ok := splitDockerTimestamp(line); ok {
		return msg
	}
	return line
}

func (m model) logsVisibleLines() int {
	visible := m.contentHeight() - 1
	if visible < 1 {
		return 1
	}
	return visible
}

func (m model) logsMaxScroll() int {
	lines := scrollableLines(m, m.logLines)
	return clampScroll(len(lines)-m.logsVisibleLines(), len(lines), m.logsVisibleLines())
}

func (m model) logsAtBottom() bool {
	return m.logViewport >= m.logsMaxScroll()
}

func (m model) scrollLogsToBottom() model {
	m.logViewport = m.logsMaxScroll()
	return m
}

func logsStatusFlags(showTS, autoScroll bool) (ts, as string) {
	ts, as = "off", "off"
	if showTS {
		ts = "on"
	}
	if autoScroll {
		as = "on"
	}
	return ts, as
}
