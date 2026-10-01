package tui

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/jdbnet/dockyard/internal/config"
	"github.com/jdbnet/dockyard/internal/engine"
)

func Run(ctx context.Context, eng *engine.Engine, cfg *config.Config, version string) error {
	m := newModel(eng, cfg, version)
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
	viewPorts
	viewUpdates
	viewLogs
	viewInspect
	viewHelp
)

type model struct {
	eng             *engine.Engine
	cfg             *config.Config
	version         string
	view            viewKind
	rows            []rowItem
	cursor          int
	filter          string
	filterActive    bool
	commandMode     bool
	commandInput    string
	confirm         *confirmDialog
	statusMsg       string
	errMsg          string
	width           int
	height          int
	events          <-chan engine.Event
	logLines        []string
	logViewport     int
	logHOffset      int
	logContainerID  string
	logFollowCh     <-chan logFollowMsg
	logFollowCancel context.CancelFunc
	logAutoScroll   bool
	logShowTS       bool
	updating        map[string]bool
	spinnerTick     int
	inspectText     string
	helpOpen        bool

	editMode      editMode
	stackName     string
	stackNew      bool
	stackEditable bool
	nameInput     textinput.Model
	composeTA     textarea.Model
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

func newModel(eng *engine.Engine, cfg *config.Config, version string) model {
	m := model{
		eng:      eng,
		cfg:      cfg,
		version:  version,
		view:     viewContainers,
		events:   eng.Subscribe(),
		updating: make(map[string]bool),
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
		case viewPorts:
			list, err := eng.Ports(ctx)
			if err != nil {
				return errLineMsg(err)
			}
			return rowsMsg(buildPortRows(list))
		case viewUpdates:
			return rowsMsg(buildUpdateRows(eng.Updates()))
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

func buildUpdateRows(report engine.UpdatesReport) []rowItem {
	rows := make([]rowItem, 0, len(report.Pending)+len(report.Failures))
	for _, item := range report.Pending {
		rows = append(rows, rowItem{
			id: item.ContainerID,
			cols: []string{
				item.Name,
				item.Image,
				shortDigest(item.LocalDigest),
				shortDigest(item.RemoteDigest),
			},
		})
	}
	for _, item := range report.Failures {
		errText := item.Error
		if errText == "" {
			errText = "check failed"
		}
		rows = append(rows, rowItem{
			cols: []string{item.Name, item.Image, "error", errText},
		})
	}
	return rows
}

func shortDigest(value string) string {
	if value == "" {
		return "-"
	}
	value = strings.TrimPrefix(value, "sha256:")
	if len(value) > 12 {
		return value[:12]
	}
	return value
}

func buildPortRows(list []engine.PortBinding) []rowItem {
	rows := make([]rowItem, 0, len(list))
	for _, p := range list {
		stack := p.ComposeProject
		if stack == "" {
			stack = "-"
		}
		service := p.ComposeService
		if service == "" {
			service = "-"
		}
		rows = append(rows, rowItem{
			id: p.ContainerID,
			cols: []string{
				fmt.Sprintf("%d", p.HostPort),
				fmt.Sprintf("%d", p.ContainerPort),
				p.Protocol,
				p.ContainerName,
				stack,
				service,
				p.ContainerState,
			},
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
