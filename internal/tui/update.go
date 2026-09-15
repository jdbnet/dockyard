package tui

import (
	"context"
	"fmt"
	"strings"

	"github.com/jdbnet/dockyard/internal/engine"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		if m.editMode == editCompose {
			m.composeTA = m.initComposeEditor(m.width, m.height)
		}
		if m.view == viewLogs && m.logAutoScroll {
			m = m.scrollLogsToBottom()
		}
		m.logHOffset = m.clampLogHOffset()
		return m, nil

	case tea.KeyMsg:
		if m.helpOpen {
			if msg.String() == "?" || msg.String() == "q" || msg.String() == "esc" {
				m.helpOpen = false
			}
			return m, nil
		}

		if m.confirm != nil {
			return m.updateConfirm(msg)
		}

		if m.editMode == editNewStackName {
			return m.updateNewStackName(msg)
		}
		if m.editMode == editCompose {
			return m.updateComposeEditor(msg)
		}

		if m.commandMode {
			return m.updateCommand(msg)
		}

		if m.filterActive {
			return m.updateFilter(msg)
		}

		switch m.view {
		case viewLogs:
			return m.updateLogs(msg)
		case viewInspect:
			return m.updateInspect(msg)
		case viewStacks:
			return m.updateStacks(msg)
		case viewImages:
			return m.updateImages(msg)
		}

		return m.updateContainers(msg)

	case engine.Event:
		if m.editMode != editNone {
			return m, waitEvent(m.events)
		}
		return m, tea.Batch(
			waitEvent(m.events),
			refreshRowsCmd(m.eng, m.view),
		)

	case rowsMsg:
		m.rows = []rowItem(msg)
		if m.cursor >= len(m.filteredRows()) {
			m.cursor = max(0, len(m.filteredRows())-1)
		}
		return m, nil

	case statusLineMsg:
		m.statusMsg = string(msg)
		m.errMsg = ""
		return m, nil

	case errLineMsg:
		m.errMsg = msg.Error()
		return m, nil

	case inspectMsg:
		m.inspectText = string(msg)
		m.view = viewInspect
		return m, nil

	case logFollowStartedMsg:
		m.stopLogFollow()
		m.logContainerID = msg.id
		m.logFollowCh = msg.ch
		m.logFollowCancel = msg.cancel
		m.view = viewLogs
		m.logLines = nil
		m.logAutoScroll = true
		m.logShowTS = true
		m.logViewport = 0
		m.logHOffset = 0
		m.filter = ""
		m.filterActive = false
		return m, waitLogFollow(msg.ch)

	case logFollowLineMsg:
		m.logLines = append(m.logLines, string(msg))
		if m.logAutoScroll {
			m = m.scrollLogsToBottom()
		}
		if m.logFollowCh != nil {
			return m, waitLogFollow(m.logFollowCh)
		}
		return m, nil

	case logFollowDoneMsg:
		m.logFollowCh = nil
		if msg.err != nil {
			m.errMsg = msg.err.Error()
		}
		return m, nil

	case updateDoneMsg:
		return m.finishContainerUpdate(msg)

	case updateSpinnerMsg:
		if len(m.updating) == 0 {
			return m, nil
		}
		m.spinnerTick++
		return m, updateSpinnerTick()

	case composeLoadMsg:
		m.stackName = msg.name
		m.stackNew = false
		m.stackEditable = msg.editable
		m.composeTA = m.initComposeEditor(m.width, m.height)
		m.composeTA.SetValue(msg.content)
		m.composeTA.Focus()
		m.editMode = editCompose
		if !msg.editable {
			m.statusMsg = "read-only stack"
		}
		return m, textarea.Blink

	case composeSavedMsg:
		name := msg.name
		isNew := msg.isNew
		m.editMode = editNone
		m.stackNew = false
		m.view = viewStacks
		m.statusMsg = "saved"
		if isNew {
			m.confirm = &confirmDialog{
				message: fmt.Sprintf("Start stack %s now?", name),
				action: func() tea.Cmd {
					return stackActionCmd(m.eng, name, "up")
				},
			}
		}
		return m, refreshRowsCmd(m.eng, viewStacks)
	}

	return m, nil
}

func (m model) updateConfirm(msg tea.KeyMsg) (model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y":
		cmd := m.confirm.action()
		m.confirm = nil
		return m, cmd
	case "n", "N", "esc", "q":
		m.confirm = nil
		m.statusMsg = "cancelled"
	}
	return m, nil
}

func (m model) updateNewStackName(msg tea.KeyMsg) (model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.editMode = editNone
		m.view = viewStacks
		return m, refreshRowsCmd(m.eng, viewStacks)
	case "enter":
		name := strings.TrimSpace(m.nameInput.Value())
		if name == "" {
			m.statusMsg = "name required"
			return m, nil
		}
		m.stackName = name
		m.stackNew = true
		m.stackEditable = true
		m.composeTA = m.initComposeEditor(m.width, m.height)
		m.composeTA.SetValue(newStackTemplate())
		m.composeTA.Focus()
		m.editMode = editCompose
		return m, textarea.Blink
	default:
		var cmd tea.Cmd
		m.nameInput, cmd = m.nameInput.Update(msg)
		return m, cmd
	}
}

func (m model) updateComposeEditor(msg tea.KeyMsg) (model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		return m.backToStacks()
	case "ctrl+s":
		if !m.stackNew && !m.stackEditable {
			m.errMsg = "stack is read-only"
			return m, nil
		}
		content := m.composeTA.Value()
		name := m.stackName
		isNew := m.stackNew
		return m, func() tea.Msg {
			ctx := context.Background()
			var err error
			if isNew {
				err = m.eng.CreateStack(ctx, name, content, false)
			} else {
				err = m.eng.SaveStackCompose(ctx, name, content)
			}
			if err != nil {
				return errLineMsg(err)
			}
			return composeSavedMsg{name: name, isNew: isNew}
		}
	default:
		if !m.stackNew && !m.stackEditable {
			return m, nil
		}
		var cmd tea.Cmd
		m.composeTA, cmd = m.composeTA.Update(msg)
		return m, cmd
	}
}

type composeSavedMsg struct {
	name  string
	isNew bool
}

func (m model) updateContainers(msg tea.KeyMsg) (model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "q":
		return m, tea.Quit
	case ":", "/":
		return m.handleCommonKeys(msg)
	case "?", "h":
		m.helpOpen = true
	case "j", "down":
		if m.cursor < len(m.filteredRows())-1 {
			m.cursor++
		}
	case "k", "up":
		if m.cursor > 0 {
			m.cursor--
		}
	case "R":
		return m, tea.Batch(
			func() tea.Msg {
				_ = m.eng.ForceRefresh(context.Background())
				return statusLineMsg("refreshed")
			},
			refreshRowsCmd(m.eng, m.view),
		)
	case "c":
		m.view = viewStacks
		m.cursor = 0
		return m, refreshRowsCmd(m.eng, viewStacks)
	case "p":
		m.view = viewPorts
		m.cursor = 0
		return m, refreshRowsCmd(m.eng, viewPorts)
	case "enter", "d":
		return m, m.cmdInspect()
	case "l":
		return m, m.cmdLogs()
	case "e":
		sel := m.selected()
		if sel == nil || sel.id == "" {
			return m, nil
		}
		if sel.meta.State != "running" {
			m.statusMsg = "container is not running"
			return m, nil
		}
		return m, cmdShell(m.eng, sel.id)
	case "u":
		sel := m.selected()
		if sel != nil && sel.id != "" {
			return m.startContainerUpdate(sel.id)
		}
	case "s":
		return m, m.cmdAction("start", m.eng.Start)
	case "S":
		return m, m.cmdAction("stop", m.eng.Stop)
	case "r":
		return m, m.cmdAction("restart", m.eng.Restart)
	case "x":
		return m.cmdRemove()
	}
	return m, nil
}

func (m model) updateImages(msg tea.KeyMsg) (model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit
	case ":", "/":
		return m.handleCommonKeys(msg)
	case "?", "h":
		m.helpOpen = true
	case "j", "down":
		if m.cursor < len(m.filteredRows())-1 {
			m.cursor++
		}
	case "k", "up":
		if m.cursor > 0 {
			m.cursor--
		}
	case "R":
		return m, tea.Batch(
			func() tea.Msg {
				_ = m.eng.ForceRefresh(context.Background())
				return statusLineMsg("refreshed")
			},
			refreshRowsCmd(m.eng, viewImages),
		)
	case "x":
		return m.cmdRemoveImage()
	case "P":
		unused := 0
		for _, r := range m.rows {
			if len(r.cols) >= 4 && r.cols[3] == "unused" {
				unused++
			}
		}
		if unused == 0 {
			m.statusMsg = "no unused images"
			return m, nil
		}
		m.confirm = &confirmDialog{
			message: fmt.Sprintf("Prune %d unused image(s)?", unused),
			action: func() tea.Cmd {
				return func() tea.Msg {
					result, err := m.eng.PruneUnusedImages(context.Background())
					if err != nil {
						return errLineMsg(err)
					}
					msg := fmt.Sprintf("pruned %d/%d image(s), reclaimed %s", result.Deleted, result.Attempted, fmtSize(int64(result.SpaceReclaimed)))
					if len(result.Errors) > 0 {
						if result.Deleted == 0 {
							return errLineMsg(fmt.Errorf("%s: %s", msg, result.Errors[0]))
						}
						msg += fmt.Sprintf("; %d failed: %s", len(result.Errors), result.Errors[0])
					}
					return statusLineMsg(msg)
				}
			},
		}
		return m, nil
	}
	return m, nil
}

func (m model) updateStacks(msg tea.KeyMsg) (model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "q", "esc":
		m.view = viewContainers
		m.cursor = 0
		return m, refreshRowsCmd(m.eng, viewContainers)
	case ":", "/":
		return m.handleCommonKeys(msg)
	case "?", "h":
		m.helpOpen = true
	case "j", "down":
		if m.cursor < len(m.filteredRows())-1 {
			m.cursor++
		}
	case "k", "up":
		if m.cursor > 0 {
			m.cursor--
		}
	case "R":
		return m, refreshRowsCmd(m.eng, viewStacks)
	case "n":
		m.editMode = editNewStackName
		m.nameInput = m.initNameInput()
		m.nameInput.Focus()
		return m, textinput.Blink
	case "enter", "e":
		sel := m.selected()
		if sel == nil || sel.id == "" {
			return m, nil
		}
		return m, loadStackComposeCmd(m.eng, sel.id)
	case "u":
		sel := m.selected()
		if sel != nil && sel.id != "" {
			return m, stackActionCmd(m.eng, sel.id, "update")
		}
	case "s":
		sel := m.selected()
		if sel != nil && sel.id != "" {
			return m, stackActionCmd(m.eng, sel.id, "up")
		}
	case "S":
		sel := m.selected()
		if sel != nil && sel.id != "" {
			return m, stackActionCmd(m.eng, sel.id, "down")
		}
	case "x":
		sel := m.selected()
		if sel == nil || sel.id == "" {
			return m, nil
		}
		if !sel.stack.Managed {
			m.errMsg = "cannot delete external stack"
			return m, nil
		}
		name := sel.id
		m.confirm = &confirmDialog{
			message: fmt.Sprintf("Delete stack %s?", name),
			action: func() tea.Cmd {
				return stackActionCmd(m.eng, name, "delete")
			},
		}
	}
	return m, nil
}

func (m model) handleCommonKeys(msg tea.KeyMsg) (model, tea.Cmd) {
	switch msg.String() {
	case ":":
		m.commandMode = true
		m.commandInput = ""
	case "/":
		m.filterActive = true
		m.filter = ""
	}
	return m, nil
}

func (m model) updateCommand(msg tea.KeyMsg) (model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.commandMode = false
		return m, nil
	case "tab":
		if suffix := execSuggestion(m.commandInput); suffix != "" {
			m.commandInput += suffix
			return m, nil
		}
		_, suffix := commandSuggestion(m.commandInput)
		if suffix != "" {
			m.commandInput += suffix
		}
		return m, nil
	case "enter":
		m.commandMode = false
		cmd := strings.TrimSpace(strings.TrimPrefix(m.commandInput, ":"))
		if strings.HasPrefix(cmd, "exec ") {
			shellCmd := strings.TrimSpace(strings.TrimPrefix(cmd, "exec "))
			if shellCmd == "" {
				m.statusMsg = "usage: exec <command>"
				return m, nil
			}
			sel := m.selected()
			if sel == nil || sel.id == "" {
				return m, nil
			}
			parts := strings.Fields(shellCmd)
			return m, func() tea.Msg {
				output, err := m.eng.Exec(context.Background(), sel.id, parts)
				if err != nil {
					return errLineMsg(err)
				}
				if output != "" {
					return statusLineMsg(output)
				}
				return statusLineMsg("exec ok")
			}
		}
		if def, ok := resolveNavigatorCommand(cmd); ok {
			if def.quit {
				return m, tea.Quit
			}
			m.view = def.view
			m.cursor = 0
			return m, refreshRowsCmd(m.eng, m.view)
		}
		m.statusMsg = "unknown command: " + cmd
		return m, nil
	default:
		if msg.Type == tea.KeyBackspace {
			if len(m.commandInput) > 0 {
				m.commandInput = m.commandInput[:len(m.commandInput)-1]
			}
		} else if len(msg.Runes) > 0 {
			m.commandInput += string(msg.Runes)
		}
	}
	return m, nil
}

func (m model) updateFilter(msg tea.KeyMsg) (model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.filterActive = false
		m.filter = ""
		m.cursor = 0
	case "enter":
		m.filterActive = false
	default:
		if msg.Type == tea.KeyBackspace {
			if len(m.filter) > 0 {
				m.filter = m.filter[:len(m.filter)-1]
			}
		} else if len(msg.Runes) > 0 {
			m.filter += string(msg.Runes)
		}
		m.cursor = 0
	}
	if m.view == viewLogs {
		m.logHOffset = m.clampLogHOffset()
	}
	return m, nil
}

func (m model) updateLogs(msg tea.KeyMsg) (model, tea.Cmd) {
	lines := scrollableLines(m, m.logLines)
	page := m.scrollPageSize()
	visible := m.logsVisibleLines()
	hStep := horizontalScrollStep(m.width)
	switch msg.String() {
	case "q", "esc":
		m.stopLogFollow()
		return m.backToList()
	case "t":
		m.logShowTS = !m.logShowTS
		m.logHOffset = m.clampLogHOffset()
	case "s":
		m.logAutoScroll = !m.logAutoScroll
		if m.logAutoScroll {
			m = m.scrollLogsToBottom()
		}
	case "g":
		m.logViewport = 0
		m.logAutoScroll = false
	case "G":
		m.logAutoScroll = true
		m = m.scrollLogsToBottom()
	case "h", "left":
		m.logHOffset = clampScroll(m.logHOffset-hStep, maxDisplayWidth(m.formattedLogLines()), m.width)
	case "l", "right":
		m.logHOffset = clampScroll(m.logHOffset+hStep, maxDisplayWidth(m.formattedLogLines()), m.width)
	case "H":
		m.logHOffset = 0
	case "L":
		m.logHOffset = m.logsMaxHOffset()
	case "j", "down":
		m.logViewport = clampScroll(m.logViewport+1, len(lines), visible)
		if !m.logsAtBottom() {
			m.logAutoScroll = false
		}
	case "k", "up":
		m.logViewport = clampScroll(m.logViewport-1, len(lines), visible)
		if !m.logsAtBottom() {
			m.logAutoScroll = false
		}
	case "pgdown", "f", " ":
		m.logViewport = clampScroll(m.logViewport+page, len(lines), visible)
		if !m.logsAtBottom() {
			m.logAutoScroll = false
		}
	case "pgup", "b":
		m.logViewport = clampScroll(m.logViewport-page, len(lines), visible)
		if !m.logsAtBottom() {
			m.logAutoScroll = false
		}
	case "/":
		m.filterActive = true
	}
	return m, nil
}

func (m model) updateInspect(msg tea.KeyMsg) (model, tea.Cmd) {
	lines := strings.Split(m.inspectText, "\n")
	page := m.scrollPageSize()
	visible := m.contentHeight() - 1
	if visible < 1 {
		visible = 1
	}
	switch msg.String() {
	case "q", "esc":
		return m.backToList()
	case "j", "down":
		m.logViewport = clampScroll(m.logViewport+1, len(lines), visible)
	case "k", "up":
		m.logViewport = clampScroll(m.logViewport-1, len(lines), visible)
	case "pgdown", "f", " ":
		m.logViewport = clampScroll(m.logViewport+page, len(lines), visible)
	case "pgup", "b":
		m.logViewport = clampScroll(m.logViewport-page, len(lines), visible)
	}
	return m, nil
}

func (m model) cmdInspect() tea.Cmd {
	sel := m.selected()
	if sel == nil || sel.id == "" {
		return nil
	}
	id := sel.id
	return func() tea.Msg {
		raw, err := m.eng.Inspect(context.Background(), id)
		if err != nil {
			return errLineMsg(err)
		}
		return inspectMsg(raw)
	}
}

func (m model) cmdLogs() tea.Cmd {
	sel := m.selected()
	if sel == nil || sel.id == "" {
		return nil
	}
	return startLogFollow(m.eng, sel.id)
}

func (m model) cmdAction(label string, fn func(context.Context, string) error) tea.Cmd {
	sel := m.selected()
	if sel == nil || sel.id == "" {
		return nil
	}
	id := sel.id
	return func() tea.Msg {
		if err := fn(context.Background(), id); err != nil {
			return errLineMsg(err)
		}
		return statusLineMsg(label + " ok")
	}
}

func (m model) cmdRemove() (model, tea.Cmd) {
	sel := m.selected()
	if sel == nil || sel.id == "" {
		return m, nil
	}
	id := sel.id
	m.confirm = &confirmDialog{
		message: fmt.Sprintf("Remove %s?", sel.cols[0]),
		action: func() tea.Cmd {
			return func() tea.Msg {
				if err := m.eng.Remove(context.Background(), id, true); err != nil {
					return errLineMsg(err)
				}
				return statusLineMsg("removed")
			}
		},
	}
	return m, nil
}

func (m model) cmdRemoveImage() (model, tea.Cmd) {
	sel := m.selected()
	if sel == nil || sel.id == "" {
		return m, nil
	}
	id := sel.id
	label := sel.cols[0]
	if len(sel.cols) > 1 && sel.cols[1] != "" {
		label = sel.cols[1]
	}
	m.confirm = &confirmDialog{
		message: fmt.Sprintf("Remove image %s?", label),
		action: func() tea.Cmd {
			return func() tea.Msg {
				if err := m.eng.RemoveImage(context.Background(), id, true); err != nil {
					return errLineMsg(err)
				}
				return statusLineMsg("removed")
			}
		},
	}
	return m, nil
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
