package logging

import (
	"encoding/json"
	"sort"
	"strconv"
	"time"
)

type pushRequest struct {
	Streams []pushStream `json:"streams"`
}

type pushStream struct {
	Stream map[string]string `json:"stream"`
	Values [][]string        `json:"values"`
}

func encodeStreams(groups []*group) ([]byte, error) {
	req := pushRequest{Streams: make([]pushStream, 0, len(groups))}
	for _, g := range groups {
		values := make([][]string, 0, len(g.lines))
		for _, line := range g.lines {
			values = append(values, []string{strconv.FormatInt(line.ts, 10), line.text})
		}
		req.Streams = append(req.Streams, pushStream{Stream: g.labels, Values: values})
	}
	return json.Marshal(req)
}

// TestPayload is the single line sent by the Test connection button.
func TestPayload(host string) ([]byte, error) {
	if host == "" {
		host = "dockyard"
	}
	req := pushRequest{
		Streams: []pushStream{{
			Stream: map[string]string{
				"host":      host,
				"container": "dockyard",
				"stream":    "stdout",
			},
			Values: [][]string{{
				strconv.FormatInt(time.Now().UnixNano(), 10),
				"dockyard logging test",
			}},
		}},
	}
	return json.Marshal(req)
}

func labelKey(labels map[string]string) string {
	keys := make([]string, 0, len(labels))
	for key := range labels {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var b []byte
	for _, key := range keys {
		b = append(b, key...)
		b = append(b, '=')
		b = append(b, labels[key]...)
		b = append(b, '\n')
	}
	return string(b)
}
