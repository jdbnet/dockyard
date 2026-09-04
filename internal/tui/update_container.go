package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jdbnet/dockyard/internal/engine"
	tea "github.com/charmbracelet/bubbletea"
)

type updateDoneMsg struct {
	id     string
	result engine.UpdateContainerResult
	err    error
}

type updateSpinnerMsg struct{}

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

func updatingStateLabel(tick int) string {
	return spinnerFrames[tick%len(spinnerFrames)] + " updating"
}

func updateSpinnerTick() tea.Cmd {
	return tea.Tick(120*time.Millisecond, func(time.Time) tea.Msg {
		return updateSpinnerMsg{}
	})
}

func (m model) updateSpinnerCmdIfNeeded() tea.Cmd {
	if len(m.updating) == 0 {
		return nil
	}
	return updateSpinnerTick()
}

func updateContainerCmd(eng *engine.Engine, id string) tea.Cmd {
	return func() tea.Msg {
		result, err := eng.UpdateContainer(context.Background(), id)
		return updateDoneMsg{id: id, result: result, err: err}
	}
}

func (m model) startContainerUpdate(id string) (model, tea.Cmd) {
	if m.updating[id] {
		return m, m.updateSpinnerCmdIfNeeded()
	}
	m.updating[id] = true
	return m, tea.Batch(updateContainerCmd(m.eng, id), updateSpinnerTick())
}

func (m model) finishContainerUpdate(msg updateDoneMsg) (model, tea.Cmd) {
	delete(m.updating, msg.id)
	m.errMsg = ""
	m.statusMsg = ""

	var cmds []tea.Cmd
	if msg.err != nil {
		m.errMsg = msg.err.Error()
	} else {
		m.statusMsg = "updated"
		if msg.result.PreviousImage != nil {
			img := msg.result.PreviousImage
			m.confirm = &confirmDialog{
				message: fmt.Sprintf("Remove previous image %s?", previousImageLabel(img)),
				action: func() tea.Cmd {
					return func() tea.Msg {
						if err := m.eng.RemoveImage(context.Background(), img.ID, true); err != nil {
							return errLineMsg(err)
						}
						return statusLineMsg("image removed")
					}
				},
			}
		}
		cmds = append(cmds, refreshRowsCmd(m.eng, m.view))
	}
	if cmd := m.updateSpinnerCmdIfNeeded(); cmd != nil {
		cmds = append(cmds, cmd)
	}
	return m, tea.Batch(cmds...)
}

func previousImageLabel(img *engine.PreviousImage) string {
	if img == nil {
		return ""
	}
	if len(img.RepoTags) > 0 {
		return strings.Join(img.RepoTags, ", ")
	}
	return img.ShortID
}

func containerRowCols(row rowItem, updating bool, spinnerTick int) []string {
	if !updating {
		return row.cols
	}
	cols := append([]string(nil), row.cols...)
	if len(cols) > 2 {
		cols[2] = updatingStateLabel(spinnerTick)
	}
	return cols
}
