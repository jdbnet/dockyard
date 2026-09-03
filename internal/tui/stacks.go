package tui

import (
	"context"
	"fmt"

	"github.com/jdbnet/dockyard/internal/compose"
	"github.com/jdbnet/dockyard/internal/engine"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

type editMode int

const (
	editNone editMode = iota
	editNewStackName
	editCompose
)

func stackCols() []colSpec {
	return []colSpec{
		{header: "NAME", min: 14, max: 0},
		{header: "RUN/TOTAL", min: 10, max: 10},
		{header: "SRC", min: 8, max: 8},
		{header: "PATH", min: 20, max: 0},
	}
}

func buildStackRows(stacks []engine.Stack) []rowItem {
	rows := make([]rowItem, 0, len(stacks))
	for _, s := range stacks {
		src := "managed"
		if !s.Managed {
			src = "external"
		}
		if s.IsEditable() {
			src += "·edit"
		}
		rows = append(rows, rowItem{
			id:   s.Name,
			stack: s,
			cols: []string{
				s.Name,
				formatRunning(s.RunningCount, s.ContainerCount),
				src,
				s.Path,
			},
		})
	}
	return rows
}

func formatRunning(running, total int) string {
	if total == 0 {
		return "0/0"
	}
	return fmt.Sprintf("%d/%d", running, total)
}

func (m *model) initNameInput() textinput.Model {
	ti := textinput.New()
	ti.Placeholder = "stack-name"
	ti.CharLimit = 64
	ti.Width = 40
	ti.Prompt = "name: "
	return ti
}

func (m *model) initComposeEditor(width, height int) textarea.Model {
	ta := textarea.New()
	ta.SetWidth(max(20, width-4))
	ta.SetHeight(max(5, height-8))
	ta.ShowLineNumbers = true
	return ta
}

func loadStackComposeCmd(eng *engine.Engine, name string) tea.Cmd {
	return func() tea.Msg {
		info, err := eng.GetStackCompose(context.Background(), name)
		if err != nil {
			return errLineMsg(err)
		}
		return composeLoadMsg{
			name:     info.Name,
			content:  info.Content,
			editable: info.Editable,
			managed:  info.Managed,
			path:     info.Path,
		}
	}
}

type composeLoadMsg struct {
	name     string
	content  string
	editable bool
	managed  bool
	path     string
}

func saveStackCmd(eng *engine.Engine, name, content string, isNew, start bool) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		var err error
		if isNew {
			err = eng.CreateStack(ctx, name, content, start)
		} else {
			err = eng.SaveStackCompose(ctx, name, content)
		}
		if err != nil {
			return errLineMsg(err)
		}
		return statusLineMsg("saved")
	}
}

func stackActionCmd(eng *engine.Engine, name string, action string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		var err error
		switch action {
		case "up":
			err = eng.StackUp(ctx, name)
		case "down":
			err = eng.StackDown(ctx, name)
		case "update":
			err = eng.UpdateStack(ctx, name)
		case "delete":
			err = eng.DeleteStack(ctx, name)
		}
		if err != nil {
			return errLineMsg(err)
		}
		return statusLineMsg(action + " ok")
	}
}

func updateContainerCmd(eng *engine.Engine, id string) tea.Cmd {
	return func() tea.Msg {
		if err := eng.UpdateContainer(context.Background(), id); err != nil {
			return errLineMsg(err)
		}
		return statusLineMsg("updated")
	}
}

func newStackTemplate() string {
	return compose.DefaultTemplate
}
