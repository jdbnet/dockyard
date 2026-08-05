package docker

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"strings"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/pkg/stdcopy"
)

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
