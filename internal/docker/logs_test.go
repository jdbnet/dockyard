package docker

import (
	"bytes"
	"testing"

	"github.com/docker/docker/pkg/stdcopy"
)

func TestSplitLogLines(t *testing.T) {
	got := splitLogLines([]byte("a\nb\n"))
	if len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Fatalf("unexpected lines: %#v", got)
	}
}

func TestCopyLogsDemuxesStreamHeader(t *testing.T) {
	var framed bytes.Buffer
	_, _ = stdcopy.NewStdWriter(&framed, stdcopy.Stdout).Write([]byte("hello\n"))

	var out bytes.Buffer
	if _, err := copyLogs(&framed, false, &out, &out); err != nil {
		t.Fatalf("copyLogs: %v", err)
	}
	if out.String() != "hello\n" {
		t.Fatalf("expected demuxed output, got %q", out.String())
	}
}
