package docker

import (
	"bytes"
	"context"
	"sync"
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

func TestReadLogEntriesSeparatesStdoutAndStderr(t *testing.T) {
	var framed bytes.Buffer
	_, _ = stdcopy.NewStdWriter(&framed, stdcopy.Stdout).Write([]byte("from-out\n"))
	_, _ = stdcopy.NewStdWriter(&framed, stdcopy.Stderr).Write([]byte("from-err\n"))

	var mu sync.Mutex
	var got []LogEntry
	err := readLogEntries(context.Background(), bytes.NewReader(framed.Bytes()), false, func(entry LogEntry) error {
		mu.Lock()
		got = append(got, entry)
		mu.Unlock()
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	found := map[string]string{}
	for _, entry := range got {
		found[entry.Stream] = entry.Line
	}
	if found["stdout"] != "from-out" || found["stderr"] != "from-err" {
		t.Fatalf("streams = %#v", found)
	}
}
