package notify

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jdbnet/dockyard/internal/config"
)

const maxResponseBody = 4096
const maxRetryAfter = 2 * time.Minute

type payload struct {
	body        []byte
	contentType string
}

func postWebhook(ctx context.Context, client *http.Client, wh config.WebhookConfig, p payload) error {
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	var res postResult
	res = executePost(ctx, client, wh, p, true)
	if res.Err != nil {
		return res.Err
	}
	if res.Status >= 200 && res.Status < 300 {
		return nil
	}
	if res.Body != "" {
		return fmt.Errorf("HTTP %d: %s", res.Status, res.Body)
	}
	return fmt.Errorf("HTTP %d", res.Status)
}

type postResult struct {
	Status int
	Body   string
	Err    error
}

func executePost(ctx context.Context, client *http.Client, wh config.WebhookConfig, p payload, allowRetry bool) postResult {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, wh.URL, bytes.NewReader(p.body))
	if err != nil {
		return postResult{Err: err}
	}
	req.Header.Set("Content-Type", p.contentType)
	switch wh.Auth {
	case "basic":
		req.SetBasicAuth(wh.Username, wh.Token)
	case "bearer":
		req.Header.Set("Authorization", "Bearer "+wh.Token)
	}
	resp, err := client.Do(req)
	if err != nil {
		return postResult{Err: err}
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, maxResponseBody))
	res := postResult{Status: resp.StatusCode, Body: strings.TrimSpace(string(raw))}
	if allowRetry && shouldRetry(res) {
		wait := 2 * time.Second
		if resp.StatusCode == http.StatusTooManyRequests {
			wait = retryAfterFromHeader(resp.Header.Get("Retry-After"))
		}
		if wait > 0 {
			timer := time.NewTimer(wait)
			select {
			case <-ctx.Done():
				timer.Stop()
				return postResult{Err: ctx.Err()}
			case <-timer.C:
			}
		}
		return executePost(ctx, client, wh, p, false)
	}
	return res
}

func shouldRetry(res postResult) bool {
	if res.Err != nil {
		return true
	}
	return res.Status == http.StatusTooManyRequests || res.Status >= 500
}

func retryAfterFromHeader(v string) time.Duration {
	v = strings.TrimSpace(v)
	if v == "" {
		return 2 * time.Second
	}
	if secs, err := strconv.Atoi(v); err == nil && secs >= 0 {
		d := time.Duration(secs) * time.Second
		if d > maxRetryAfter {
			return maxRetryAfter
		}
		return d
	}
	if t, err := http.ParseTime(v); err == nil {
		d := time.Until(t)
		if d < 0 {
			return 0
		}
		if d > maxRetryAfter {
			return maxRetryAfter
		}
		return d
	}
	return 2 * time.Second
}
