package transport

import (
	"context"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"time"
)

// Retryable reports whether an HTTP status code should be retried.
func Retryable(status int) bool {
	return status == http.StatusTooManyRequests ||
		status == http.StatusInternalServerError ||
		status == http.StatusBadGateway ||
		status == http.StatusServiceUnavailable ||
		status == http.StatusGatewayTimeout
}

// DoWithRetry executes fn with exponential backoff + jitter.
// Retryable statuses (429, 5xx) and transport errors are retried.
// The response body of a retried response is drained and closed.
func DoWithRetry(ctx context.Context, attempts int, fn func() (*http.Response, error)) (*http.Response, error) {
	var lastErr error

	for attempt := 0; attempt < attempts; attempt++ {
		if attempt > 0 {
			if err := sleepCtx(ctx, backoff(attempt)); err != nil {
				return nil, err
			}
		}

		resp, err := fn()
		if err != nil {
			lastErr = err
			continue
		}

		if Retryable(resp.StatusCode) && attempt < attempts-1 {
			io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20))
			resp.Body.Close()
			lastErr = fmt.Errorf("http %d", resp.StatusCode)
			continue
		}

		return resp, nil
	}

	return nil, fmt.Errorf("after %d attempts: %w", attempts, lastErr)
}

// backoff computes exponential backoff with jitter: ~2^attempt * 250ms.
func backoff(attempt int) time.Duration {
	base := time.Duration(1<<uint(attempt)) * 250 * time.Millisecond
	jitter := time.Duration(rand.Int63n(int64(base / 4 + 1)))
	return base + jitter
}

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
