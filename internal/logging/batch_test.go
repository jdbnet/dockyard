package logging

import (
	"encoding/json"
	"testing"
	"time"
)

func TestBatchFlushesOnSize(t *testing.T) {
	b := newBatcher(1, time.Hour)
	now := time.Unix(0, 1_710_000_000_000_000_000)
	labels := map[string]string{"host": "box", "stream": "stdout"}
	first := b.Add(now, Item{ContainerID: "c1", Cursor: Cursor{Timestamp: now.UnixNano(), Count: 1}, Labels: labels, TS: now.UnixNano(), Text: "one"})
	if first != nil {
		t.Fatal("first line should stay open")
	}
	second := b.Add(now, Item{ContainerID: "c1", Cursor: Cursor{Timestamp: now.UnixNano(), Count: 2}, Labels: labels, TS: now.UnixNano(), Text: "two"})
	if second == nil {
		t.Fatal("second line should flush the first batch")
	}
	var body pushRequest
	if err := json.Unmarshal(second.Payload, &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Streams) != 1 || len(body.Streams[0].Values) != 1 || body.Streams[0].Values[0][1] != "one" {
		t.Fatalf("flushed payload = %#v", body.Streams)
	}
	if second.Cursors["c1"].Count != 1 {
		t.Fatalf("flushed cursor = %+v", second.Cursors["c1"])
	}
	rest := b.Flush()
	if rest == nil {
		t.Fatal("expected the second line to remain buffered")
	}
	if err := json.Unmarshal(rest.Payload, &body); err != nil {
		t.Fatal(err)
	}
	if body.Streams[0].Values[0][1] != "two" {
		t.Fatalf("remaining line = %#v", body.Streams[0].Values)
	}
}

func TestBatchFlushesOnTime(t *testing.T) {
	wait := 2 * time.Second
	b := newBatcher(1<<20, wait)
	start := time.Unix(1_700_000_000, 0)
	if batch := b.Add(start, Item{Labels: map[string]string{"stream": "stderr"}, TS: 10, Text: "hello"}); batch != nil {
		t.Fatal("line should wait for the time limit")
	}
	if batch := b.FlushDue(start.Add(wait - time.Millisecond)); batch != nil {
		t.Fatal("flushed before the time limit")
	}
	batch := b.FlushDue(start.Add(wait))
	if batch == nil {
		t.Fatal("expected a flush at the time limit")
	}
	var body pushRequest
	if err := json.Unmarshal(batch.Payload, &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Streams) != 1 || body.Streams[0].Values[0][1] != "hello" {
		t.Fatalf("payload = %#v", body)
	}
}
