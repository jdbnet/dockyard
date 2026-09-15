package docker

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/pkg/stdcopy"
)

// TerminalSession is a bidirectional TTY bridge for interactive exec.
type TerminalSession interface {
	Read(p []byte) (int, error)
	Write(p []byte) (int, error)
	Size() (cols, rows uint)
}

// ResizeNotifier can receive terminal resize events during an exec session.
type ResizeNotifier interface {
	TerminalSession
	OnResize(func(cols, rows uint))
}

// ShellCommands returns shell candidates in priority order.
func ShellCommands() [][]string {
	return [][]string{
		{"/bin/bash"},
		{"/bin/sh"},
	}
}

// Exec runs a command in a container and returns combined stdout/stderr.
func (c *Client) Exec(ctx context.Context, id string, cmd []string) (string, error) {
	execResp, err := c.cli.ContainerExecCreate(ctx, id, container.ExecOptions{
		Cmd:          cmd,
		AttachStdout: true,
		AttachStderr: true,
		Tty:          false,
	})
	if err != nil {
		return "", err
	}

	attach, err := c.cli.ContainerExecAttach(ctx, execResp.ID, container.ExecAttachOptions{Tty: false})
	if err != nil {
		return "", err
	}
	defer attach.Close()

	var out bytes.Buffer
	_, copyErr := stdcopy.StdCopy(&out, &out, attach.Reader)
	if copyErr != nil && !errors.Is(copyErr, io.EOF) {
		return out.String(), copyErr
	}
	return strings.TrimRight(out.String(), "\n"), nil
}

// InteractiveExec runs an interactive TTY session in a container.
func (c *Client) InteractiveExec(ctx context.Context, id string, cmd []string, session TerminalSession) error {
	execResp, err := c.cli.ContainerExecCreate(ctx, id, container.ExecOptions{
		Cmd:          cmd,
		AttachStdin:  true,
		AttachStdout: true,
		AttachStderr: true,
		Tty:          true,
	})
	if err != nil {
		return err
	}

	attach, err := c.cli.ContainerExecAttach(ctx, execResp.ID, container.ExecAttachOptions{Tty: true})
	if err != nil {
		return err
	}
	defer attach.Close()

	cols, rows := session.Size()
	if cols > 0 && rows > 0 {
		_ = c.cli.ContainerExecResize(ctx, execResp.ID, container.ResizeOptions{
			Width:  cols,
			Height: rows,
		})
	}

	if notifier, ok := session.(ResizeNotifier); ok {
		notifier.OnResize(func(cols, rows uint) {
			if cols == 0 || rows == 0 {
				return
			}
			_ = c.cli.ContainerExecResize(ctx, execResp.ID, container.ResizeOptions{
				Width:  cols,
				Height: rows,
			})
		})
	}

	conn := attach.Conn
	errCh := make(chan error, 2)
	var wg sync.WaitGroup

	wg.Add(2)
	go func() {
		defer wg.Done()
		_, err := io.Copy(session, conn)
		errCh <- err
	}()
	go func() {
		defer wg.Done()
		_, err := io.Copy(conn, session)
		errCh <- err
	}()

	select {
	case <-ctx.Done():
		attach.Close()
		wg.Wait()
		return ctx.Err()
	case err := <-errCh:
		attach.Close()
		wg.Wait()
		if err != nil && !errors.Is(err, io.EOF) {
			return err
		}
		return nil
	}
}

// InteractiveExecShell tries bash then sh for an interactive session.
func (c *Client) InteractiveExecShell(ctx context.Context, id string, session TerminalSession) error {
	var lastErr error
	for _, cmd := range ShellCommands() {
		err := c.InteractiveExec(ctx, id, cmd, session)
		if err == nil {
			return nil
		}
		lastErr = err
		if !isMissingExecutable(err) {
			return err
		}
	}
	if lastErr != nil {
		return lastErr
	}
	return fmt.Errorf("no shell available in container")
}

func isMissingExecutable(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "executable file not found") ||
		strings.Contains(msg, "no such file or directory") ||
		strings.Contains(msg, "cannot run")
}
