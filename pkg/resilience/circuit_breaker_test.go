package resilience

import (
	"testing"
	"time"
)

func TestCircuitBreaker_StartsClose(t *testing.T) {
	cb := DefaultCircuitBreaker()
	if cb.State() != CircuitClosed {
		t.Errorf("expected closed, got %v", cb.State())
	}
}

func TestCircuitBreaker_OpensAfterThreshold(t *testing.T) {
	cb := NewCircuitBreaker(3, 2, 100*time.Millisecond)
	for i := 0; i < 3; i++ {
		cb.RecordFailure()
	}
	if cb.State() != CircuitOpen {
		t.Errorf("expected open after 3 failures, got %v", cb.State())
	}
	if err := cb.Allow(); err != ErrCircuitOpen {
		t.Errorf("expected ErrCircuitOpen, got %v", err)
	}
}

func TestCircuitBreaker_TransitionsToHalfOpen(t *testing.T) {
	cb := NewCircuitBreaker(2, 1, 50*time.Millisecond)
	cb.RecordFailure()
	cb.RecordFailure()
	if cb.State() != CircuitOpen {
		t.Fatalf("expected open, got %v", cb.State())
	}

	// Wait for reset timeout
	time.Sleep(60 * time.Millisecond)
	if err := cb.Allow(); err != nil {
		t.Fatalf("expected Allow() after timeout, got %v", err)
	}
	if cb.State() != CircuitHalfOpen {
		t.Errorf("expected half-open after timeout, got %v", cb.State())
	}
}

func TestCircuitBreaker_ClosesAfterSuccessInHalfOpen(t *testing.T) {
	cb := NewCircuitBreaker(2, 2, 50*time.Millisecond)
	cb.RecordFailure()
	cb.RecordFailure()
	time.Sleep(60 * time.Millisecond)
	_ = cb.Allow() // transitions to half-open

	cb.RecordSuccess()
	cb.RecordSuccess()
	if cb.State() != CircuitClosed {
		t.Errorf("expected closed after 2 successes in half-open, got %v", cb.State())
	}
}

func TestCircuitBreaker_ReOpensOnFailureInHalfOpen(t *testing.T) {
	cb := NewCircuitBreaker(2, 2, 50*time.Millisecond)
	cb.RecordFailure()
	cb.RecordFailure()
	time.Sleep(60 * time.Millisecond)
	_ = cb.Allow() // half-open

	cb.RecordFailure()
	if cb.State() != CircuitOpen {
		t.Errorf("expected re-open after failure in half-open, got %v", cb.State())
	}
}

func TestCircuitBreaker_SuccessResetsClosed(t *testing.T) {
	cb := NewCircuitBreaker(3, 1, 100*time.Millisecond)
	cb.RecordFailure()
	cb.RecordFailure()
	if cb.Failures() != 2 {
		t.Errorf("expected 2 failures, got %d", cb.Failures())
	}
	cb.RecordSuccess()
	if cb.Failures() != 0 {
		t.Errorf("expected failures reset to 0 after success, got %d", cb.Failures())
	}
}

func TestCircuitState_String(t *testing.T) {
	tests := []struct {
		state    CircuitState
		expected string
	}{
		{CircuitClosed, "closed"},
		{CircuitOpen, "open"},
		{CircuitHalfOpen, "half-open"},
		{CircuitState(99), "unknown"},
	}
	for _, tt := range tests {
		if got := tt.state.String(); got != tt.expected {
			t.Errorf("State(%d).String() = %q, want %q", tt.state, got, tt.expected)
		}
	}
}
