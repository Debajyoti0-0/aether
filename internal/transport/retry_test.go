package transport

import (
	"context"
	"fmt"
	"net/http"
	"testing"
)

func TestRetryable(t *testing.T) {
	cases := map[int]bool{
		429: true, 500: true, 502: true, 503: true, 504: true,
		200: false, 400: false, 401: false, 404: false,
	}
	for status, want := range cases {
		if got := Retryable(status); got != want {
			t.Errorf("Retryable(%d) = %t, want %t", status, got, want)
		}
	}
}

func TestDoWithRetrySuccessFirst(t *testing.T) {
	attempts := 0
	resp, err := DoWithRetry(context.Background(), 3, func() (*http.Response, error) {
		attempts++
		return &http.Response{StatusCode: 200, Body: http.NoBody}, nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("status = %d", resp.StatusCode)
	}
	if attempts != 1 {
		t.Errorf("attempts = %d, want 1", attempts)
	}
}

func TestDoWithRetryExhausts(t *testing.T) {
	attempts := 0
	_, err := DoWithRetry(context.Background(), 3, func() (*http.Response, error) {
		attempts++
		return nil, fmt.Errorf("boom")
	})
	if err == nil {
		t.Fatal("expected error after exhausting retries")
	}
	if attempts != 3 {
		t.Errorf("attempts = %d, want 3", attempts)
	}
}

func TestDoWithRetryContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := DoWithRetry(ctx, 3, func() (*http.Response, error) {
		return nil, fmt.Errorf("boom")
	})
	if err == nil {
		t.Fatal("expected context error")
	}
}
