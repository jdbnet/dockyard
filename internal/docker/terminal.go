package docker

import (
	"io"
	"os"

	"github.com/moby/term"
)

// LocalTerminalSession bridges the host TTY for in-process interactive exec.
type LocalTerminalSession struct {
	in       io.Reader
	out      io.Writer
	sizeFD   uintptr
	resizeFn func(cols, rows uint)
}

func NewLocalTerminalSession() *LocalTerminalSession {
	fd, isTTY := term.GetFdInfo(os.Stdin)
	if !isTTY {
		fd = 0
	}
	return &LocalTerminalSession{
		in:     os.Stdin,
		out:    os.Stdout,
		sizeFD: fd,
	}
}

func (s *LocalTerminalSession) Read(p []byte) (int, error) {
	return s.in.Read(p)
}

func (s *LocalTerminalSession) Write(p []byte) (int, error) {
	return s.out.Write(p)
}

func (s *LocalTerminalSession) Size() (cols, rows uint) {
	if s.sizeFD == 0 {
		return 80, 24
	}
	ws, err := term.GetWinsize(s.sizeFD)
	if err != nil || ws == nil || ws.Width <= 0 || ws.Height <= 0 {
		return 80, 24
	}
	return uint(ws.Width), uint(ws.Height)
}

func (s *LocalTerminalSession) OnResize(fn func(cols, rows uint)) {
	s.resizeFn = fn
}

func (s *LocalTerminalSession) NotifyResize() {
	if s.resizeFn == nil {
		return
	}
	cols, rows := s.Size()
	s.resizeFn(cols, rows)
}
