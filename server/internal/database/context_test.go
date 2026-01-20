package database

import (
	"testing"
	"time"
)

func TestDefaultTimeout(t *testing.T) {
	ctx, cancel := DefaultTimeout()
	defer cancel()

	// Check context is not nil
	if ctx == nil {
		t.Fatal("DefaultTimeout should return a non-nil context")
	}

	// Check cancel function is not nil
	if cancel == nil {
		t.Fatal("DefaultTimeout should return a non-nil cancel function")
	}

	// Check that context has a deadline
	deadline, ok := ctx.Deadline()
	if !ok {
		t.Fatal("DefaultTimeout context should have a deadline")
	}

	// Deadline should be approximately 5 seconds from now
	timeUntilDeadline := time.Until(deadline)
	if timeUntilDeadline < 4*time.Second || timeUntilDeadline > 6*time.Second {
		t.Errorf("DefaultTimeout deadline should be ~5 seconds, got %v", timeUntilDeadline)
	}
}

func TestTimeout(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
	}{
		{"1 second", 1 * time.Second},
		{"10 seconds", 10 * time.Second},
		{"100 milliseconds", 100 * time.Millisecond},
		{"1 minute", 1 * time.Minute},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := Timeout(tt.duration)
			defer cancel()

			// Check context is not nil
			if ctx == nil {
				t.Fatal("Timeout should return a non-nil context")
			}

			// Check cancel function is not nil
			if cancel == nil {
				t.Fatal("Timeout should return a non-nil cancel function")
			}

			// Check that context has a deadline
			deadline, ok := ctx.Deadline()
			if !ok {
				t.Fatal("Timeout context should have a deadline")
			}

			// Deadline should be approximately the specified duration from now
			timeUntilDeadline := time.Until(deadline)
			margin := tt.duration / 10 // 10% margin
			if margin < 50*time.Millisecond {
				margin = 50 * time.Millisecond
			}

			if timeUntilDeadline < tt.duration-margin || timeUntilDeadline > tt.duration+margin {
				t.Errorf("Timeout deadline should be ~%v, got %v", tt.duration, timeUntilDeadline)
			}
		})
	}
}

func TestContextCancellation(t *testing.T) {
	ctx, cancel := DefaultTimeout()

	// Context should not be done initially
	select {
	case <-ctx.Done():
		t.Fatal("Context should not be done before cancel is called")
	default:
		// Expected
	}

	// Cancel the context
	cancel()

	// Context should be done after cancel
	select {
	case <-ctx.Done():
		// Expected
	default:
		t.Fatal("Context should be done after cancel is called")
	}

	// Check error is context canceled
	if ctx.Err() == nil {
		t.Fatal("Context error should not be nil after cancel")
	}
}

func TestTimeoutWithZeroDuration(t *testing.T) {
	// Zero duration means deadline is immediate
	ctx, cancel := Timeout(0)
	defer cancel()

	// Context with zero timeout should be done almost immediately
	// But let's just verify it returns valid values
	if ctx == nil {
		t.Fatal("Timeout(0) should return a non-nil context")
	}
	if cancel == nil {
		t.Fatal("Timeout(0) should return a non-nil cancel function")
	}
}

func TestMultipleDefaultTimeouts(t *testing.T) {
	// Create multiple contexts to ensure they're independent
	ctx1, cancel1 := DefaultTimeout()
	defer cancel1()

	ctx2, cancel2 := DefaultTimeout()
	defer cancel2()

	// Canceling one should not affect the other
	cancel1()

	select {
	case <-ctx1.Done():
		// Expected
	default:
		t.Fatal("ctx1 should be done after cancel1")
	}

	select {
	case <-ctx2.Done():
		t.Fatal("ctx2 should not be done after cancel1")
	default:
		// Expected
	}
}

func BenchmarkDefaultTimeout(b *testing.B) {
	for i := 0; i < b.N; i++ {
		ctx, cancel := DefaultTimeout()
		cancel()
		_ = ctx
	}
}

func BenchmarkTimeout(b *testing.B) {
	for i := 0; i < b.N; i++ {
		ctx, cancel := Timeout(5 * time.Second)
		cancel()
		_ = ctx
	}
}
