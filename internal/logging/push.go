package logging

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const maxPushResponse = 4096

type pushClass int

const (
	pushOK pushClass = iota
	pushRetry
	pushPause
	pushDrop
)

type pushResult struct {
	Status int
	Body   string
	Err    error
}

// Classify maps a push response to success, retry, auth pause, or drop.
func Classify(status int, transport error) pushClass {
	if transport != nil {
		return pushRetry
	}
	if status >= 200 && status < 300 {
		return pushOK
	}
	if status == http.StatusUnauthorized || status == http.StatusForbidden {
		return pushPause
	}
	if status == http.StatusRequestTimeout || status == http.StatusTooManyRequests || status >= 500 {
		return pushRetry
	}
	if status >= 400 && status < 500 {
		return pushDrop
	}
	return pushRetry
}

func postPush(ctx context.Context, client *http.Client, url, auth, username, password, token string, body []byte) pushResult {
	if client == nil {
		client = http.DefaultClient
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return pushResult{Err: err}
	}
	req.Header.Set("Content-Type", "application/json")
	switch auth {
	case "basic":
		req.SetBasicAuth(username, password)
	case "bearer":
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := client.Do(req)
	if err != nil {
		return pushResult{Err: err}
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, maxPushResponse))
	return pushResult{Status: resp.StatusCode, Body: strings.TrimSpace(string(raw))}
}

func (s *Shipper) runPush(ctx context.Context) {
	backoff := time.Second
	for {
		if ctx.Err() != nil {
			return
		}
		if !s.pushEnabled() || s.authPaused() {
			if !waitWake(ctx, s.wake) {
				return
			}
			continue
		}
		rec, ok := s.spool.Peek()
		if !ok {
			if !waitWake(ctx, s.wake) {
				return
			}
			continue
		}
		settings := s.snapshotSettings()
		callCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		result := postPush(callCtx, s.client, settings.URL, settings.Auth, settings.Username, settings.Password, settings.Token, rec.Payload)
		cancel()
		switch Classify(result.Status, result.Err) {
		case pushOK:
			_ = s.spool.Ack()
			_ = s.cursors.Commit(rec.Cursors)
			s.noteSuccess()
			backoff = time.Second
		case pushPause:
			s.pauseAuth(result.Status)
		case pushDrop:
			_ = s.spool.Ack()
			_ = s.cursors.Commit(rec.Cursors)
			s.noteFailure(pushFailure(result))
			backoff = time.Second
		default:
			s.noteFailure(pushFailure(result))
			if !sleepOrWake(ctx, s.wake, backoff) {
				return
			}
			if backoff < 30*time.Second {
				backoff *= 2
				if backoff > 30*time.Second {
					backoff = 30 * time.Second
				}
			}
		}
	}
}

func pushFailure(result pushResult) string {
	if result.Err != nil {
		return result.Err.Error()
	}
	if result.Body != "" {
		return fmt.Sprintf("destination returned %d: %s", result.Status, result.Body)
	}
	return fmt.Sprintf("destination returned %d", result.Status)
}

func waitWake(ctx context.Context, wake <-chan struct{}) bool {
	select {
	case <-ctx.Done():
		return false
	case <-wake:
		return true
	}
}

func sleepOrWake(ctx context.Context, wake <-chan struct{}, d time.Duration) bool {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-wake:
		return true
	case <-timer.C:
		return true
	}
}
