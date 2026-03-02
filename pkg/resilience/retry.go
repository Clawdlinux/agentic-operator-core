package resilience

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"time"

	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

// RetryConfig defines exponential backoff parameters.
type RetryConfig struct {
	MaxRetries     int
	InitialBackoff time.Duration
	MaxBackoff     time.Duration
	BackoffFactor  float64 // multiplier per attempt (default 2.0)
	Jitter         bool    // add randomised jitter to prevent thundering herd
}

// DefaultRetryConfig returns production-safe defaults.
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries:     3,
		InitialBackoff: 1 * time.Second,
		MaxBackoff:     30 * time.Second,
		BackoffFactor:  2.0,
		Jitter:         true,
	}
}

// RetryResult captures what happened during the retry loop.
type RetryResult struct {
	Attempts int
	LastErr  error
	Duration time.Duration
}

// WithRetry executes fn with exponential backoff.
// Returns (result, RetryResult) where result is nil on permanent failure.
func WithRetry[T any](ctx context.Context, cfg RetryConfig, operationName string, fn func(ctx context.Context) (T, error)) (T, RetryResult) {
	log := logf.FromContext(ctx)
	start := time.Now()
	var zero T

	for attempt := 0; attempt <= cfg.MaxRetries; attempt++ {
		result, err := fn(ctx)
		if err == nil {
			return result, RetryResult{Attempts: attempt + 1, Duration: time.Since(start)}
		}

		rr := RetryResult{Attempts: attempt + 1, LastErr: err, Duration: time.Since(start)}

		if attempt == cfg.MaxRetries {
			log.Error(err, "operation failed after all retries",
				"operation", operationName,
				"attempts", rr.Attempts,
				"duration", rr.Duration,
			)
			return zero, rr
		}

		backoff := cfg.backoffDuration(attempt)
		log.Info("retrying operation",
			"operation", operationName,
			"attempt", attempt+1,
			"maxRetries", cfg.MaxRetries,
			"backoff", backoff,
			"error", err.Error(),
		)

		select {
		case <-time.After(backoff):
			// continue
		case <-ctx.Done():
			return zero, RetryResult{Attempts: attempt + 1, LastErr: ctx.Err(), Duration: time.Since(start)}
		}
	}

	return zero, RetryResult{Attempts: cfg.MaxRetries + 1, LastErr: fmt.Errorf("exhausted retries for %s", operationName)}
}

// backoffDuration calculates delay with optional jitter.
func (c RetryConfig) backoffDuration(attempt int) time.Duration {
	d := float64(c.InitialBackoff) * math.Pow(c.BackoffFactor, float64(attempt))
	if d > float64(c.MaxBackoff) {
		d = float64(c.MaxBackoff)
	}
	if c.Jitter {
		d = d * (0.5 + rand.Float64()*0.5) // 50-100% of calculated delay
	}
	return time.Duration(d)
}
