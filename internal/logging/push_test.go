package logging

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestPushPayloadAndAuth(t *testing.T) {
	var gotBody []byte
	var gotType, gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotType = r.Header.Get("Content-Type")
		gotAuth = r.Header.Get("Authorization")
		gotBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	b := newBatcher(1<<20, time.Hour)
	ts := int64(1_710_000_000_000_000_000)
	b.Add(time.Unix(0, ts), Item{
		ContainerID: "c1",
		Cursor:      Cursor{Timestamp: ts, Count: 1},
		Labels:      map[string]string{"host": "box", "container": "api", "stream": "stdout"},
		TS:          ts,
		Text:        "hello",
	})
	batch := b.Flush()
	result := postPush(t.Context(), nil, srv.URL, "basic", "ship", "secret", "", batch.Payload)
	if result.Err != nil {
		t.Fatal(result.Err)
	}
	if Classify(result.Status, result.Err) != pushOK {
		t.Fatalf("status %d", result.Status)
	}
	if gotType != "application/json" {
		t.Fatalf("content type %q", gotType)
	}
	wantAuth := "Basic " + base64.StdEncoding.EncodeToString([]byte("ship:secret"))
	if gotAuth != wantAuth {
		t.Fatalf("authorization %q", gotAuth)
	}
	var body pushRequest
	if err := json.Unmarshal(gotBody, &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Streams) != 1 {
		t.Fatalf("streams = %#v", body.Streams)
	}
	stream := body.Streams[0]
	if stream.Stream["host"] != "box" || stream.Stream["container"] != "api" || stream.Stream["stream"] != "stdout" {
		t.Fatalf("labels = %#v", stream.Stream)
	}
	if len(stream.Values) != 1 || stream.Values[0][0] != "1710000000000000000" || stream.Values[0][1] != "hello" {
		t.Fatalf("values = %#v", stream.Values)
	}

	result = postPush(t.Context(), nil, srv.URL, "bearer", "", "", "tok", batch.Payload)
	if result.Err != nil {
		t.Fatal(result.Err)
	}
	if gotAuth != "Bearer tok" {
		t.Fatalf("bearer auth %q", gotAuth)
	}
}

func TestClassifyAuthPause(t *testing.T) {
	if Classify(http.StatusUnauthorized, nil) != pushPause || Classify(http.StatusForbidden, nil) != pushPause {
		t.Fatal("401 and 403 should pause")
	}
	if Classify(http.StatusBadRequest, nil) != pushDrop {
		t.Fatal("400 should drop")
	}
	if Classify(http.StatusBadGateway, nil) != pushRetry {
		t.Fatal("502 should retry")
	}
}
