package tui

import (
	"context"
	"fmt"
	"strings"

	"git.jdbnet.co.uk/jamie/dockyard/internal/config"
	"git.jdbnet.co.uk/jamie/dockyard/internal/engine"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

func Run(ctx context.Context, eng *engine.Engine, cfg *config.Config) error {
	m := newModel(eng, cfg)
	p := tea.NewProgram(m, tea.WithAltScreen())
	go func() {
		<-ctx.Done()
		p.Quit()
	}()
	_, err := p.Run()
	return err
}

type viewKind int

const (
	viewContainers viewKind = iota
	viewStacks
	viewImages
	viewVolumes
	viewNetworks
	viewLogs
	viewInspect
	viewHelp
)

type model struct {
	eng          *engine.Engine
	cfg          *config.Config
	view         viewKind
	rows         []rowItem
	cursor       int
	filter       string
	filterActive bool
	commandMode  bool
	commandInput string
	confirm      *confirmDialog
	statusMsg    string
	errMsg       string
	width        int
	height       int
	events       <-chan engine.Event
	logLines     []string
	logViewport  int
	logContainerID  string
	logFollowCh     <-chan logFollowMsg
	logFollowCancel context.CancelFunc
	logAutoScroll   bool
	logShowTS       bool
	inspectText  string
	helpOpen     bool

	editMode  editMode
	stackName string
	stackNew  bool
	nameInput textinput.Model
	composeTA textarea.Model
}

type rowItem struct {
	id      string
	cols    []string
	meta    engine.Container
	stack   engine.Stack
	isStack bool
}

type confirmDialog struct {
	message string
	action  func() tea.Cmd
}

func newModel(eng *engine.Engine, cfg *config.Config) model {
	m := model{
		eng:    eng,
		cfg:    cfg,
		view:   viewContainers,
		events: eng.Subscribe(),
	}
	m.nameInput = m.initNameInput()
	return m
}

func (m model) Init() tea.Cmd {
	return tea.Batch(
		tea.EnterAltScreen,
		waitEvent(m.events),
		refreshRowsCmd(m.eng, m.view),
	)
}

func waitEvent(ch <-chan engine.Event) tea.Cmd {
	return func() tea.Msg {
		ev, ok := <-ch
		if !ok {
			return nil
		}
		return ev
	}
}

func refreshRowsCmd(eng *engine.Engine, view viewKind) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		switch view {
		case viewContainers:
			list, err := eng.Containers(ctx)
			if err != nil {
				return errLineMsg(err)
			}
			return rowsMsg(buildContainerRows(list))
		case viewStacks:
			stacks, err := eng.Stacks(ctx)
			if err != nil {
				return errLineMsg(err)
			}
			return rowsMsg(buildStackRows(stacks))
		case viewImages:
			list, err := eng.Images(ctx)
			if err != nil {
				return errLineMsg(err)
			}
			return rowsMsg(buildImageRows(list))
		case viewVolumes:
			list, err := eng.Volumes(ctx)
			if err != nil {
				return errLineMsg(err)
			}
			return rowsMsg(buildVolumeRows(list))
		case viewNetworks:
			list, err := eng.Networks(ctx)
			if err != nil {
				return errLineMsg(err)
			}
			return rowsMsg(buildNetworkRows(list))
		}
		return nil
	}
}

type rowsMsg []rowItem
type statusLineMsg string
type errLineMsg error
type inspectMsg string

func buildContainerRows(list []engine.Container) []rowItem {
	rows := make([]rowItem, 0, len(list))
	for _, c := range list {
		stack := c.ComposeProject
		if stack == "" {
			stack = "-"
		}
		health := c.Health
		if health == "" {
			health = "-"
		}
		if c.RestartLoop {
			health = "LOOP"
		}
		rows = append(rows, rowItem{
			id: c.ID,
			cols: []string{
				c.Name,
				stack,
				c.State,
				engine.FormatPct(c.CPUPct),
				engine.FormatBytes(c.MemBytes),
				c.Uptime,
				health,
			},
			meta: c,
		})
	}
	return rows
}

func buildImageRows(list []engine.Image) []rowItem {
	rows := make([]rowItem, 0, len(list))
	for _, img := range list {
		tag := strings.Join(img.RepoTags, ", ")
		if tag == "" {
			tag = "<none>"
		}
		unused := ""
		if img.Unused {
			unused = "unused"
		}
		rows = append(rows, rowItem{
			id:   img.ID,
			cols: []string{img.ShortID, tag, fmtSize(img.Size), unused, fmt.Sprintf("%d", img.Containers)},
		})
	}
	return rows
}

func buildVolumeRows(list []engine.Volume) []rowItem {
	rows := make([]rowItem, 0, len(list))
	for _, v := range list {
		unused := ""
		if v.Unused {
			unused = "unused"
		}
		rows = append(rows, rowItem{
			id:   v.Name,
			cols: []string{v.Name, v.Driver, v.Scope, unused},
		})
	}
	return rows
}

func buildNetworkRows(list []engine.Network) []rowItem {
	rows := make([]rowItem, 0, len(list))
	for _, n := range list {
		rows = append(rows, rowItem{
			id:   n.ID,
			cols: []string{n.Name, n.Driver, n.Scope, fmt.Sprintf("%d", n.Containers)},
		})
	}
	return rows
}

func fmtSize(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

func (m model) filteredRows() []rowItem {
	if m.filter == "" {
		return m.rows
	}
	f := strings.ToLower(m.filter)
	var out []rowItem
	for _, r := range m.rows {
		line := strings.ToLower(strings.Join(r.cols, " "))
		if strings.Contains(line, f) {
			out = append(out, r)
		}
	}
	return out
}

func (m model) selected() *rowItem {
	rows := m.filteredRows()
	if m.cursor >= 0 && m.cursor < len(rows) {
		r := rows[m.cursor]
		return &r
	}
	return nil
}
