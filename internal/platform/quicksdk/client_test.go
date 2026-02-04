package quicksdk

import (
	"context"
	"sync"
	"testing"
	"time"
)

// TestRateLimiter_BasicAcquire verifies that tokens can be acquired up to burst size.
func TestRateLimiter_BasicAcquire(t *testing.T) {
	rl := newRateLimiter(60) // 60 requests per minute = 1 per second

	// Should be able to acquire initial burst tokens immediately
	// Burst size = 60/60 = 1
	for i := 0; i < 1; i++ {
		err := rl.Wait(context.Background())
		if err != nil {
			t.Fatalf("Expected to acquire token %d, got error: %v", i, err)
		}
	}
}

// TestRateLimiter_Refill verifies that tokens are refilled over time.
func TestRateLimiter_Refill(t *testing.T) {
	rl := newRateLimiter(60) // 60 requests per minute = 1 per second, burst = 1

	// Acquire the initial token
	err := rl.Wait(context.Background())
	if err != nil {
		t.Fatalf("Expected to acquire first token, got error: %v", err)
	}

	// Try to acquire another token immediately - should block briefly
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	start := time.Now()
	err = rl.Wait(ctx)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("Expected to acquire token after refill, got error: %v", err)
	}

	// Should have waited approximately 1 second for refill
	if elapsed < 500*time.Millisecond {
		t.Logf("Warning: Refill was quick (%v), may have acquired from burst", elapsed)
	}
	if elapsed > 1500*time.Millisecond {
		t.Logf("Warning: Refill took longer than expected (%v)", elapsed)
	}
}

// TestRateLimiter_ContextCancellation verifies that Wait respects context cancellation.
func TestRateLimiter_ContextCancellation(t *testing.T) {
	rl := newRateLimiter(60) // burst = 1

	// Acquire the only token
	err := rl.Wait(context.Background())
	if err != nil {
		t.Fatalf("Expected to acquire first token, got error: %v", err)
	}

	// Cancel context while waiting
	ctx, cancel := context.WithCancel(context.Background())

	// Start a goroutine that will cancel after a short delay
	done := make(chan error, 1)
	go func() {
		time.AfterFunc(10*time.Millisecond, func() {
			cancel()
		})
		done <- rl.Wait(ctx)
	}()

	select {
	case err := <-done:
		if err == nil {
			t.Error("Expected error when context is canceled, got nil")
		} else if err != context.Canceled {
			t.Errorf("Expected context.Canceled, got: %v (type: %T)", err, err)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("Test timed out waiting for Wait to return")
	}
}

// TestRateLimiter_Timeout verifies that Wait returns context timeout error.
func TestRateLimiter_Timeout(t *testing.T) {
	rl := newRateLimiter(60) // burst = 1

	// Acquire the only token
	err := rl.Wait(context.Background())
	if err != nil {
		t.Fatalf("Expected to acquire first token, got error: %v", err)
	}

	// Use a timeout context
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	err = rl.Wait(ctx)
	if err == nil {
		t.Error("Expected timeout error, got nil")
	} else if err != context.DeadlineExceeded {
		t.Errorf("Expected context.DeadlineExceeded, got: %v (type: %T)", err, err)
	}
}

// TestRateLimiter_HighRate verifies higher request rates work correctly.
func TestRateLimiter_HighRate(t *testing.T) {
	rl := newRateLimiter(600) // 600 requests per minute = 10 per second, burst = 10

	// Should be able to acquire burst tokens immediately
	for i := 0; i < 10; i++ {
		err := rl.Wait(context.Background())
		if err != nil {
			t.Fatalf("Expected to acquire token %d, got error: %v", i, err)
		}
	}

	// Next request should wait for refill
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	start := time.Now()
	err := rl.Wait(ctx)
	elapsed := time.Since(start)

	if err != nil {
		t.Logf("Wait returned error (may have timed out): %v, elapsed: %v", err, elapsed)
	}

	// Should have waited at least some time for refill
	if elapsed < 50*time.Millisecond {
		t.Errorf("Expected to wait for refill, but returned quickly in %v", elapsed)
	}
}

// TestRateLimiter_ConcurrentAccess verifies thread safety under concurrent access.
func TestRateLimiter_ConcurrentAccess(t *testing.T) {
	rl := newRateLimiter(600) // burst = 10

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	var wg sync.WaitGroup
	successCount := int32(0)
	failCount := int32(0)

	// Try to acquire from 20 goroutines
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if rl.Wait(ctx) == nil {
				successCount++
			} else {
				failCount++
			}
		}()
	}

	wg.Wait()

	// Should successfully acquire up to burst size (10)
	// Some may succeed if refill happens during test
	if successCount < 10 {
		t.Errorf("Expected at least 10 successful acquires, got %d", successCount)
	}
	if successCount > 12 {
		t.Logf("Note: More than burst tokens acquired (%d), refill occurred during test", successCount)
	}
	t.Logf("Success: %d, Failed: %d", successCount, failCount)
}

// TestRateLimiter_ZeroRate verifies default rate is used for zero input.
func TestRateLimiter_ZeroRate(t *testing.T) {
	rl := newRateLimiter(0) // Should use default 1000

	// Burst size should be at least 1
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err := rl.Wait(ctx)
	if err != nil {
		t.Errorf("Expected to acquire token with default rate, got error: %v", err)
	}
}
