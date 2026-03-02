package resilience

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestWithRetry_SuccessFirstAttempt(t *testing.T) {
	cfg := RetryConfig{MaxRetries: 3, InitialBackoff: 10 * time.Millisecond, MaxBackoff: 100 * time.Millisecond, BackoffFactor: 2.0}
	result, rr := WithRetry(context.Background(), cfg, "test-op", func(_ context.Context) (string, error) {
		return "hello", nil
	})
	if result != "hello" {
		t.Errorf("expected 'hello', got %q", result)
	}
	if rr.Attempts != 1 {
		t.Errorf("expected 1 attempt, got %d", rr.Attempts)
	}
	if rr.LastErr != nil {
		t.Errorf("expected nil error, got %v", rr.LastErr)
	}
}

func TestWithRetry_SuccessAfterRetries(t *testing.T) {
	cfg := RetryConfig{MaxRetries: 3, InitialBackoff: 10 * time.Millisecond, MaxBackoff: 100 * time.Millisecond, BackoffFactor: 2.0}
	attempt := 0
	result, rr := WithRetry(context.Background(), cfg, "test-op", func(_ context.Context) (int, error) {
		attempt++
		if attempt < 3 {
			return 0, errors.New("transient error")
		}
		return 42, nil
	})
	if result != 42 {
		t.Errorf("expected 42, got %d", result)
	}
	if rr.Attempts != 3 {
		t.Errorf("expected 3 attempts, got %d", rr.Attempts)
	}
}

func TestWithRetry_AllRetriesExhausted(t *testing.T) {
	cfg := RetryConfig{MaxRetries: 2, InitialBackoff: 10 * time.Millisecond, MaxBackoff: 100 * time.Millisecond, BackoffFactor: 2.0}
	_, rr := WithRetry(context.Background(), cfg, "test-op", func(_ context.Context) (string, error) {
		return "", errors.New("persistent error")
	})
	if rr.LastErr == nil {
		t.Error("expected error after exhausting retries")
	}
	if rr.Attempts != 3 { // initial + 2 retries
		t.Errorf("expected 3 attempts (1 initial + 2 retries), got %d", rr.Attempts)
	}
}

func TestWithRetry_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cfg := RetryConfig{MaxRetries: 5, InitialBackoff: 1 * time.Second, MaxBackoff: 10 * time.Second, BackoffFactor: 2.0}

	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	_, rr := WithRetry(ctx, cfg, "test-op", func(_ context.Context) (string, error) {
		return "", errors.New("error")
	})

	if !errors.Is(rr.LastErr, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", rr.LastErr)
	}
}

func TestBackoffDuration_ExponentialGrowth(t *testing.T) {
	cfg := RetryConfig{InitialBackoff: 100 * time.Millisecond, MaxBackoff: 10 * time.Second, BackoffFactor: 2.0, Jitter: false}
	d0 := cfg.backoffDuration(0)
	d1 := cfg.backoffDuration(1)
	d2 := cfg.backoffDuration(2)
	if d0 != 100*time.Millisecond {
		t.Errorf("attempt 0: expected 100ms, got %v", d0)
	}
	if d1 != 200*time.Millisecond {
		t.Errorf("attempt 1: expected 200ms, got %v", d1)
	}
	if d2 != 400*time.Millisecond {
		t.Errorf("attempt 2: expected 400ms, got %v", d2)
	}
}

func TestBackoffDuration_MaxCap(t *testing.T) {
	cfg := RetryConfig{InitialBackoff: 1 * time.Second, MaxBackoff: 5 * time.Second, BackoffFactor: 10.0, Jitter: false}
	d := cfg.backoffDuration(5) // 1s * 10^5 = way over max
	if d != 5*time.Second {
		t.Errorf("expected max backoff 5s, got %v", d)
	}
}
