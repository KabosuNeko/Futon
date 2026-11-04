package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

func httpGet(ctx context.Context, client *http.Client, endpoint, userAgent string) (*http.Response, error) {
	return httpDo(ctx, client, func() (*http.Request, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			return nil, fmt.Errorf("tạo request: %w", err)
		}
		req.Header.Set("User-Agent", userAgent)
		req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8")
		req.Header.Set("Accept-Language", "vi-VN,vi;q=0.9,en-US;q=0.8,en;q=0.7")
		return req, nil
	})
}

// httpDo retries network errors and 5xx responses with incremental backoff.
// The builder runs per attempt so POST bodies and headers are rebuilt fresh;
// it must attach ctx to the request. A cancelled ctx stops retrying at once.
func httpDo(ctx context.Context, client *http.Client, build func() (*http.Request, error)) (*http.Response, error) {
	client = ensureClient(client)
	var lastErr error
	for attempt := 0; attempt <= providerMaxRetries; attempt++ {
		if attempt > 0 {
			if err := sleepCtx(ctx, time.Duration(attempt)*500*time.Millisecond); err != nil {
				return nil, err
			}
		}
		req, err := build()
		if err != nil {
			return nil, err
		}

		resp, err := client.Do(req)
		if err != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			lastErr = fmt.Errorf("không kết nối được %s: %w", req.URL.Host, unwrapURLError(err))
			continue
		}
		if resp.StatusCode >= 500 && resp.StatusCode < 600 {
			resp.Body.Close()
			lastErr = fmt.Errorf("HTTP %d", resp.StatusCode)
			continue
		}
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
		}
		return resp, nil
	}
	return nil, lastErr
}

// unwrapURLError drops the *url.Error wrapper, whose message repeats the full
// request URL; the UI shows the host and the underlying cause instead.
func unwrapURLError(err error) error {
	var ue *url.Error
	if errors.As(err, &ue) {
		return ue.Err
	}
	return err
}

// sleepCtx waits for d or until ctx is cancelled.
func sleepCtx(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

func ensureClient(client *http.Client) *http.Client {
	if client.Timeout == 0 {
		c := *client
		c.Timeout = providerTimeout
		return &c
	}
	return client
}
