package docker

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"strings"
	"sync"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/pkg/stdcopy"
)

// LogEntry is one demultiplexed container log line.
type LogEntry struct {
	Stream string
	Line   string
}

func (c *Client) containerTTY(ctx context.Context, id string) (bool, error) {
	insp, err := c.cli.ContainerInspect(ctx, id)
	if err != nil {
		return false, err
	}
	return insp.Config.Tty, nil
}

func copyLogs(src io.Reader, tty bool, dstout, dsterr io.Writer) (int64, error) {
	if tty {
		return io.Copy(dstout, src)
	}
	return stdcopy.StdCopy(dstout, dsterr, src)
}

func logOptions(opts LogOptions) container.LogsOptions {
	return container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Tail:       opts.Tail,
		Follow:     opts.Follow,
		Since:      opts.Since,
		Timestamps: true,
	}
}

// ReadLogLines fetches container logs, demultiplexes Docker's stream framing, and
// returns lines newest-first.
func (c *Client) ReadLogLines(ctx context.Context, id string, opts LogOptions) ([]string, error) {
	opts.Follow = false
	rc, err := c.cli.ContainerLogs(ctx, id, logOptions(opts))
	if err != nil {
		return nil, err
	}
	defer rc.Close()

	tty, err := c.containerTTY(ctx, id)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	if _, err := copyLogs(rc, tty, &buf, &buf); err != nil {
		return nil, err
	}
	lines := splitLogLines(buf.Bytes())
	return lines, nil
}

// StreamLogLines demultiplexes container logs and invokes emit once per line in
// chronological order (oldest first).
func (c *Client) StreamLogLines(ctx context.Context, id string, opts LogOptions, emit func(line string) error) error {
	rc, err := c.cli.ContainerLogs(ctx, id, logOptions(opts))
	if err != nil {
		return err
	}
	defer rc.Close()

	tty, err := c.containerTTY(ctx, id)
	if err != nil {
		return err
	}

	pr, pw := io.Pipe()
	go func() {
		_, copyErr := copyLogs(rc, tty, pw, pw)
		_ = pw.CloseWithError(copyErr)
	}()

	sc := bufio.NewScanner(pr)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	for sc.Scan() {
		if err := emit(sc.Text()); err != nil {
			return err
		}
	}
	if err := sc.Err(); err != nil {
		return err
	}
	return nil
}

// StreamLogEntries follows container logs and emits each line with its stream.
// TTY containers have no stdout/stderr framing, so every line is reported as stdout.
// emit may be called from two goroutines at once.
func (c *Client) StreamLogEntries(ctx context.Context, id string, opts LogOptions, emit func(entry LogEntry) error) error {
	rc, err := c.cli.ContainerLogs(ctx, id, logOptions(opts))
	if err != nil {
		return err
	}
	defer rc.Close()

	tty, err := c.containerTTY(ctx, id)
	if err != nil {
		return err
	}
	return readLogEntries(ctx, rc, tty, emit)
}

func readLogEntries(ctx context.Context, src io.Reader, tty bool, emit func(entry LogEntry) error) error {
	if tty {
		return scanLogEntries(ctx, src, "stdout", emit)
	}

	stdoutR, stdoutW := io.Pipe()
	stderrR, stderrW := io.Pipe()
	go func() {
		_, copyErr := stdcopy.StdCopy(stdoutW, stderrW, src)
		_ = stdoutW.CloseWithError(copyErr)
		_ = stderrW.CloseWithError(copyErr)
	}()

	var wg sync.WaitGroup
	errCh := make(chan error, 2)
	scan := func(r *io.PipeReader, stream string) {
		defer wg.Done()
		defer r.Close()
		if err := scanLogEntries(ctx, r, stream, emit); err != nil {
			errCh <- err
		}
	}
	wg.Add(2)
	go scan(stdoutR, "stdout")
	go scan(stderrR, "stderr")
	wg.Wait()
	select {
	case err := <-errCh:
		return err
	default:
		return nil
	}
}

func scanLogEntries(ctx context.Context, r io.Reader, stream string, emit func(entry LogEntry) error) error {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	sc.Split(newLogSplitter())
	for sc.Scan() {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := emit(LogEntry{Stream: stream, Line: sc.Text()}); err != nil {
			return err
		}
	}
	if err := sc.Err(); err != nil && ctx.Err() == nil {
		return err
	}
	return ctx.Err()
}

// newLogSplitter yields lines and discards any line that exceeds 1 MiB so a
// single oversized write cannot stop the follow.
func newLogSplitter() bufio.SplitFunc {
	const maxLine = 1024 * 1024
	discarding := false
	return func(data []byte, atEOF bool) (int, []byte, error) {
		if discarding {
			if i := bytes.IndexByte(data, '\n'); i >= 0 {
				discarding = false
				return i + 1, nil, nil
			}
			if len(data) == 0 && !atEOF {
				return 0, nil, nil
			}
			return len(data), nil, nil
		}
		if i := bytes.IndexByte(data, '\n'); i >= 0 {
			if i > maxLine {
				return i + 1, nil, nil
			}
			return i + 1, trimCR(data[:i]), nil
		}
		if atEOF {
			if len(data) == 0 || len(data) > maxLine {
				return len(data), nil, nil
			}
			return len(data), trimCR(data), nil
		}
		if len(data) >= maxLine {
			discarding = true
			return len(data), nil, nil
		}
		return 0, nil, nil
	}
}

func trimCR(b []byte) []byte {
	if len(b) > 0 && b[len(b)-1] == '\r' {
		return b[:len(b)-1]
	}
	return b
}

func splitLogLines(b []byte) []string {
	if len(b) == 0 {
		return nil
	}
	text := strings.TrimRight(string(b), "\n\r")
	if text == "" {
		return nil
	}
	return strings.Split(text, "\n")
}
