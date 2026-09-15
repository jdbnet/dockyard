package tui

import (
	"context"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/jdbnet/dockyard/internal/docker"
	"github.com/jdbnet/dockyard/internal/engine"
	tea "github.com/charmbracelet/bubbletea"
)

type shellExec struct {
	eng *engine.Engine
	id  string
}

func (s *shellExec) SetStdin(_ io.Reader)  {}
func (s *shellExec) SetStdout(_ io.Writer) {}
func (s *shellExec) SetStderr(_ io.Writer) {}

func (s *shellExec) Run() error {
	session := docker.NewLocalTerminalSession()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGWINCH)
	defer signal.Stop(sigCh)
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-sigCh:
				session.NotifyResize()
			}
		}
	}()

	return s.eng.Shell(ctx, s.id, session)
}

func cmdShell(eng *engine.Engine, id string) tea.Cmd {
	return tea.Exec(&shellExec{eng: eng, id: id}, func(err error) tea.Msg {
		if err != nil {
			return errLineMsg(err)
		}
		return statusLineMsg("shell closed")
	})
}
