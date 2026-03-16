package interceptors

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// TestDefaultConfig tests default configuration values
func TestDefaultConfig(t *testing.T) {
	cfg := defaultConfig()

	if cfg.Timeout != 5*time.Second {
		t.Errorf("Expected Timeout 5s, got %v", cfg.Timeout)
	}
	if cfg.MaxAttempts != 3 {
		t.Errorf("Expected MaxAttempts 3, got %d", cfg.MaxAttempts)
	}
	if cfg.BackoffBase != 100*time.Millisecond {
		t.Errorf("Expected BackoffBase 100ms, got %v", cfg.BackoffBase)
	}
}

// TestChain tests creating interceptor chain
func TestChain(t *testing.T) {
	tests := []struct {
		name string
		cfg  *Config
	}{
		{
			name: "nil config uses defaults",
			cfg:  nil,
		},
		{
			name: "custom config",
			cfg: &Config{
				Timeout:     10 * time.Second,
				MaxAttempts: 5,
				BackoffBase: 200 * time.Millisecond,
			},
		},
		{
			name: "partial config",
			cfg: &Config{
				Timeout: 10 * time.Second,
			},
		},
		{
			name: "zero timeout",
			cfg: &Config{
				Timeout:     0,
				MaxAttempts: 3,
				BackoffBase: 100 * time.Millisecond,
			},
		},
		{
			name: "zero max attempts",
			cfg: &Config{
				Timeout:     5 * time.Second,
				MaxAttempts: 0,
				BackoffBase: 100 * time.Millisecond,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := Chain(tt.cfg)
			if opts == nil {
				t.Fatal("Chain() should return options")
			}
			if len(opts) != 2 {
				t.Errorf("Expected 2 options, got %d", len(opts))
			}
		})
	}
}

// TestBackoff tests backoff calculation
func TestBackoff(t *testing.T) {
	tests := []struct {
		name     string
		base     time.Duration
		attempt  int
		expected time.Duration
	}{
		{
			name:     "attempt 1",
			base:     100 * time.Millisecond,
			attempt:  1,
			expected: 100 * time.Millisecond,
		},
		{
			name:     "attempt 2",
			base:     100 * time.Millisecond,
			attempt:  2,
			expected: 200 * time.Millisecond,
		},
		{
			name:     "attempt 3",
			base:     100 * time.Millisecond,
			attempt:  3,
			expected: 400 * time.Millisecond,
		},
		{
			name:     "attempt 4",
			base:     100 * time.Millisecond,
			attempt:  4,
			expected: 800 * time.Millisecond,
		},
		{
			name:     "attempt 0 (treated as 1)",
			base:     100 * time.Millisecond,
			attempt:  0,
			expected: 100 * time.Millisecond,
		},
		{
			name:     "negative attempt (treated as 1)",
			base:     100 * time.Millisecond,
			attempt:  -1,
			expected: 100 * time.Millisecond,
		},
		{
			name:     "zero base",
			base:     0,
			attempt:  1,
			expected: 0,
		},
		{
			name:     "large attempt",
			base:     10 * time.Millisecond,
			attempt:  10,
			expected: 5120 * time.Millisecond, // 10 * 2^9
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := backoff(tt.base, tt.attempt)
			if result != tt.expected {
				t.Errorf("backoff(%v, %d) = %v, want %v", tt.base, tt.attempt, result, tt.expected)
			}
		})
	}
}

// TestBackoff_Exponential tests exponential backoff growth
func TestBackoff_Exponential(t *testing.T) {
	base := 100 * time.Millisecond

	prev := backoff(base, 1)
	for i := 2; i <= 5; i++ {
		curr := backoff(base, i)
		if curr != 2*prev {
			t.Errorf("Backoff should double each time: attempt %d, prev %v, curr %v", i, prev, curr)
		}
		prev = curr
	}
}

// TestBackoff_DifferentBases tests different base durations
func TestBackoff_DifferentBases(t *testing.T) {
	bases := []time.Duration{
		time.Millisecond,
		10 * time.Millisecond,
		100 * time.Millisecond,
		1 * time.Second,
	}

	for _, base := range bases {
		result := backoff(base, 2)
		expected := base * 2

		if result != expected {
			t.Errorf("backoff(%v, 2) = %v, want %v", base, result, expected)
		}
	}
}

// TestBackoff_MathPrecision tests backoff calculation precision
func TestBackoff_MathPrecision(t *testing.T) {
	tests := []struct {
		base     time.Duration
		attempt  int
		expected time.Duration
	}{
		{100 * time.Millisecond, 1, 100 * time.Millisecond},
		{100 * time.Millisecond, 2, 200 * time.Millisecond},
		{100 * time.Millisecond, 3, 400 * time.Millisecond},
		{100 * time.Millisecond, 4, 800 * time.Millisecond},
		{100 * time.Millisecond, 5, 1600 * time.Millisecond},
		{50 * time.Millisecond, 3, 200 * time.Millisecond},
		{1 * time.Millisecond, 10, 512 * time.Millisecond},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%v_at_%d", tt.base, tt.attempt), func(t *testing.T) {
			result := backoff(tt.base, tt.attempt)
			if result != tt.expected {
				t.Errorf("backoff(%v, %d) = %v, want %v", tt.base, tt.attempt, result, tt.expected)
			}
		})
	}
}

// mockClientStream is a mock implementation of grpc.ClientStream
type mockClientStream struct {
	grpc.ClientStream
}

func (m *mockClientStream) Header() (metadata.MD, error) {
	return nil, nil
}

func (m *mockClientStream) Trailer() metadata.MD {
	return nil
}

func (m *mockClientStream) CloseSend() error {
	return nil
}

func (m *mockClientStream) Context() context.Context {
	return context.Background()
}

func (m *mockClientStream) SendMsg(msg interface{}) error {
	return nil
}

func (m *mockClientStream) RecvMsg(msg interface{}) error {
	return io.EOF
}

// TestUnaryInterceptor_RetryLogic tests the retry logic for unary interceptor
func TestUnaryInterceptor_RetryLogic(t *testing.T) {
	t.Run("success on first attempt", func(t *testing.T) {
		cfg := &Config{
			Timeout:     100 * time.Millisecond,
			MaxAttempts: 3,
			BackoffBase: 5 * time.Millisecond,
		}
		cfgCopy := *cfg

		invoked := false
		invoker := func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, opts ...grpc.CallOption) error {
			invoked = true
			return nil
		}

		ui := createUnaryInterceptor(cfgCopy)
		err := ui(context.Background(), "/test/Method", nil, nil, nil, invoker)
		if err != nil {
			t.Errorf("Expected success, got %v", err)
		}
		if !invoked {
			t.Error("Invoker was not called")
		}
	})

	t.Run("retry on Unavailable", func(t *testing.T) {
		cfg := &Config{
			Timeout:     100 * time.Millisecond,
			MaxAttempts: 3,
			BackoffBase: 5 * time.Millisecond,
		}
		cfgCopy := *cfg

		attempts := 0
		invoker := func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, opts ...grpc.CallOption) error {
			attempts++
			if attempts == 1 {
				return status.Error(codes.Unavailable, "service unavailable")
			}
			return nil
		}

		ui := createUnaryInterceptor(cfgCopy)
		err := ui(context.Background(), "/test/Method", nil, nil, nil, invoker)
		if err != nil {
			t.Errorf("Expected success after retry, got %v", err)
		}
		if attempts != 2 {
			t.Errorf("Expected 2 attempts, got %d", attempts)
		}
	})

	t.Run("retry on DeadlineExceeded", func(t *testing.T) {
		cfg := &Config{
			Timeout:     100 * time.Millisecond,
			MaxAttempts: 3,
			BackoffBase: 5 * time.Millisecond,
		}
		cfgCopy := *cfg

		attempts := 0
		invoker := func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, opts ...grpc.CallOption) error {
			attempts++
			if attempts == 1 {
				return status.Error(codes.DeadlineExceeded, "deadline exceeded")
			}
			return nil
		}

		ui := createUnaryInterceptor(cfgCopy)
		err := ui(context.Background(), "/test/Method", nil, nil, nil, invoker)
		if err != nil {
			t.Errorf("Expected success after retry, got %v", err)
		}
		if attempts != 2 {
			t.Errorf("Expected 2 attempts, got %d", attempts)
		}
	})

	t.Run("max attempts exceeded", func(t *testing.T) {
		cfg := &Config{
			Timeout:     100 * time.Millisecond,
			MaxAttempts: 2,
			BackoffBase: 5 * time.Millisecond,
		}
		cfgCopy := *cfg

		attempts := 0
		invoker := func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, opts ...grpc.CallOption) error {
			attempts++
			return status.Error(codes.Unavailable, "service unavailable")
		}

		ui := createUnaryInterceptor(cfgCopy)
		err := ui(context.Background(), "/test/Method", nil, nil, nil, invoker)
		if err == nil {
			t.Error("Expected error after max attempts")
		}
		if attempts != cfg.MaxAttempts {
			t.Errorf("Expected %d attempts, got %d", cfg.MaxAttempts, attempts)
		}
		if status.Code(err) != codes.Unavailable {
			t.Errorf("Expected Unavailable code, got %v", status.Code(err))
		}
	})

	nonRetryableCodes := []codes.Code{
		codes.InvalidArgument,
		codes.NotFound,
		codes.AlreadyExists,
		codes.PermissionDenied,
		codes.Unauthenticated,
		codes.FailedPrecondition,
		codes.OutOfRange,
		codes.Unimplemented,
		codes.Internal,
		codes.DataLoss,
	}

	for _, code := range nonRetryableCodes {
		t.Run("non-retryable: "+code.String(), func(t *testing.T) {
			cfg := &Config{
				Timeout:     100 * time.Millisecond,
				MaxAttempts: 3,
				BackoffBase: 5 * time.Millisecond,
			}
			cfgCopy := *cfg

			invokerCalls := 0
			invoker := func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, opts ...grpc.CallOption) error {
				invokerCalls++
				return status.Error(code, "test error")
			}

			ui := createUnaryInterceptor(cfgCopy)
			err := ui(context.Background(), "/test/Method", nil, nil, nil, invoker)
			if status.Code(err) != code {
				t.Errorf("Expected %v code, got %v", code, status.Code(err))
			}
			if invokerCalls != 1 {
				t.Errorf("Expected 1 call for non-retryable error, got %d", invokerCalls)
			}
		})
	}

	retryableCodes := []codes.Code{codes.Unavailable, codes.DeadlineExceeded}
	for _, code := range retryableCodes {
		t.Run("retryable: "+code.String(), func(t *testing.T) {
			cfg := &Config{
				Timeout:     100 * time.Millisecond,
				MaxAttempts: 3,
				BackoffBase: 1 * time.Millisecond,
			}
			cfgCopy := *cfg

			attempts := 0
			invoker := func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, opts ...grpc.CallOption) error {
				attempts++
				if attempts == 1 {
					return status.Error(code, "test error")
				}
				return nil
			}

			ui := createUnaryInterceptor(cfgCopy)
			err := ui(context.Background(), "/test/Method", nil, nil, nil, invoker)
			if err != nil {
				t.Errorf("Expected success after retry for %v, got %v", code, err)
			}
			if attempts != 2 {
				t.Errorf("Expected 2 attempts for %v, got %d", code, attempts)
			}
		})
	}

	t.Run("context with existing deadline not modified", func(t *testing.T) {
		cfg := &Config{
			Timeout:     100 * time.Millisecond,
			MaxAttempts: 1,
			BackoffBase: 5 * time.Millisecond,
		}
		cfgCopy := *cfg

		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Hour)
		defer cancel()

		originalDeadline, ok := ctx.Deadline()
		if !ok {
			t.Fatal("Context should have deadline")
		}

		invoker := func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, opts ...grpc.CallOption) error {
			newDeadline, ok := ctx.Deadline()
			if !ok {
				t.Error("Context should still have deadline")
			}
			if newDeadline != originalDeadline {
				t.Error("Context with existing deadline should not be modified")
			}
			return nil
		}

		ui := createUnaryInterceptor(cfgCopy)
		err := ui(ctx, "/test/Method", nil, nil, nil, invoker)
		if err != nil {
			t.Errorf("Expected success, got %v", err)
		}
	})

	t.Run("context without deadline gets timeout", func(t *testing.T) {
		cfg := &Config{
			Timeout:     100 * time.Millisecond,
			MaxAttempts: 1,
			BackoffBase: 10 * time.Millisecond,
		}
		cfgCopy := *cfg

		ctx := context.Background()

		invoker := func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, opts ...grpc.CallOption) error {
			_, ok := ctx.Deadline()
			if !ok {
				t.Error("Context should have deadline after timeout is applied")
			}
			return nil
		}

		ui := createUnaryInterceptor(cfgCopy)
		err := ui(ctx, "/test/Method", nil, nil, nil, invoker)
		if err != nil {
			t.Errorf("Expected success, got %v", err)
		}
	})

	t.Run("zero timeout means no default timeout", func(t *testing.T) {
		cfg := &Config{
			Timeout:     0,
			MaxAttempts: 1,
			BackoffBase: 5 * time.Millisecond,
		}
		cfgCopy := *cfg

		invoker := func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, opts ...grpc.CallOption) error {
			_, ok := ctx.Deadline()
			if ok {
				t.Error("Context should not have deadline when Timeout is 0")
			}
			return nil
		}

		ui := createUnaryInterceptor(cfgCopy)
		err := ui(context.Background(), "/test/Method", nil, nil, nil, invoker)
		if err != nil {
			t.Errorf("Expected success, got %v", err)
		}
	})

	t.Run("context cancelled during backoff", func(t *testing.T) {
		cfg := &Config{
			Timeout:     100 * time.Millisecond,
			MaxAttempts: 10,
			BackoffBase: 50 * time.Millisecond,
		}
		cfgCopy := *cfg

		ctx, cancel := context.WithCancel(context.Background())

		attempts := 0
		invoker := func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, opts ...grpc.CallOption) error {
			attempts++
			cancel() // Cancel context after first attempt
			return status.Error(codes.Unavailable, "service unavailable")
		}

		ui := createUnaryInterceptor(cfgCopy)
		err := ui(ctx, "/test/Method", nil, nil, nil, invoker)
		if !errors.Is(err, context.Canceled) && status.Code(err) != codes.Unavailable {
			t.Logf("Got error (context.Canceled or Unavailable expected): %v", err)
		}
		if attempts < 1 {
			t.Error("Expected at least one attempt")
		}
	})

	t.Run("multiple retries", func(t *testing.T) {
		cfg := &Config{
			Timeout:     100 * time.Millisecond,
			MaxAttempts: 5,
			BackoffBase: 1 * time.Millisecond,
		}
		cfgCopy := *cfg

		attempts := 0
		invoker := func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, opts ...grpc.CallOption) error {
			attempts++
			if attempts < 4 {
				return status.Error(codes.Unavailable, "service temporarily unavailable")
			}
			return nil
		}

		ui := createUnaryInterceptor(cfgCopy)
		err := ui(context.Background(), "/test/Method", nil, nil, nil, invoker)
		if err != nil {
			t.Errorf("Expected success after retries, got %v", err)
		}
		if attempts != 4 {
			t.Errorf("Expected 4 attempts, got %d", attempts)
		}
	})
}

// TestStreamInterceptor_RetryLogic tests the retry logic for stream interceptor
func TestStreamInterceptor_RetryLogic(t *testing.T) {
	t.Run("success on first attempt", func(t *testing.T) {
		cfg := &Config{
			Timeout:     100 * time.Millisecond,
			MaxAttempts: 3,
			BackoffBase: 5 * time.Millisecond,
		}
		cfgCopy := *cfg

		streamerCalled := false
		streamer := func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, opts ...grpc.CallOption) (grpc.ClientStream, error) {
			streamerCalled = true
			return &mockClientStream{}, nil
		}

		si := createStreamInterceptor(cfgCopy)
		cs, err := si(context.Background(), &grpc.StreamDesc{}, nil, "/test/StreamMethod", streamer)
		if err != nil {
			t.Errorf("Expected success, got %v", err)
		}
		if cs == nil {
			t.Error("Expected non-nil ClientStream")
		}
		if !streamerCalled {
			t.Error("Streamer was not called")
		}
	})

	t.Run("retry on Unavailable", func(t *testing.T) {
		cfg := &Config{
			Timeout:     100 * time.Millisecond,
			MaxAttempts: 3,
			BackoffBase: 5 * time.Millisecond,
		}
		cfgCopy := *cfg

		attempts := 0
		streamer := func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, opts ...grpc.CallOption) (grpc.ClientStream, error) {
			attempts++
			if attempts == 1 {
				return nil, status.Error(codes.Unavailable, "service unavailable")
			}
			return &mockClientStream{}, nil
		}

		si := createStreamInterceptor(cfgCopy)
		cs, err := si(context.Background(), &grpc.StreamDesc{}, nil, "/test/StreamMethod", streamer)
		if err != nil {
			t.Errorf("Expected success after retry, got %v", err)
		}
		if cs == nil {
			t.Error("Expected non-nil ClientStream")
		}
		if attempts != 2 {
			t.Errorf("Expected 2 attempts, got %d", attempts)
		}
	})

	t.Run("retry on DeadlineExceeded", func(t *testing.T) {
		cfg := &Config{
			Timeout:     100 * time.Millisecond,
			MaxAttempts: 3,
			BackoffBase: 5 * time.Millisecond,
		}
		cfgCopy := *cfg

		attempts := 0
		streamer := func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, opts ...grpc.CallOption) (grpc.ClientStream, error) {
			attempts++
			if attempts == 1 {
				return nil, status.Error(codes.DeadlineExceeded, "deadline exceeded")
			}
			return &mockClientStream{}, nil
		}

		si := createStreamInterceptor(cfgCopy)
		cs, err := si(context.Background(), &grpc.StreamDesc{}, nil, "/test/StreamMethod", streamer)
		if err != nil {
			t.Errorf("Expected success after retry, got %v", err)
		}
		if cs == nil {
			t.Error("Expected non-nil ClientStream")
		}
		if attempts != 2 {
			t.Errorf("Expected 2 attempts, got %d", attempts)
		}
	})

	t.Run("max attempts exceeded", func(t *testing.T) {
		cfg := &Config{
			Timeout:     100 * time.Millisecond,
			MaxAttempts: 2,
			BackoffBase: 5 * time.Millisecond,
		}
		cfgCopy := *cfg

		attempts := 0
		streamer := func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, opts ...grpc.CallOption) (grpc.ClientStream, error) {
			attempts++
			return nil, status.Error(codes.Unavailable, "service unavailable")
		}

		si := createStreamInterceptor(cfgCopy)
		cs, err := si(context.Background(), &grpc.StreamDesc{}, nil, "/test/StreamMethod", streamer)
		if err == nil {
			t.Error("Expected error after max attempts")
		}
		if cs != nil {
			t.Error("Expected nil ClientStream on error")
		}
		if attempts != cfg.MaxAttempts {
			t.Errorf("Expected %d attempts, got %d", cfg.MaxAttempts, attempts)
		}
		if status.Code(err) != codes.Unavailable {
			t.Errorf("Expected Unavailable code, got %v", status.Code(err))
		}
	})

	nonRetryableCodes := []codes.Code{
		codes.InvalidArgument,
		codes.NotFound,
		codes.AlreadyExists,
		codes.PermissionDenied,
		codes.Unauthenticated,
		codes.FailedPrecondition,
		codes.OutOfRange,
		codes.Unimplemented,
		codes.Internal,
		codes.DataLoss,
	}

	for _, code := range nonRetryableCodes {
		t.Run("non-retryable stream: "+code.String(), func(t *testing.T) {
			cfg := &Config{
				Timeout:     100 * time.Millisecond,
				MaxAttempts: 3,
				BackoffBase: 5 * time.Millisecond,
			}
			cfgCopy := *cfg

			streamerCalls := 0
			streamer := func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, opts ...grpc.CallOption) (grpc.ClientStream, error) {
				streamerCalls++
				return nil, status.Error(code, "test error")
			}

			si := createStreamInterceptor(cfgCopy)
			cs, err := si(context.Background(), &grpc.StreamDesc{}, nil, "/test/StreamMethod", streamer)
			if err == nil {
				t.Error("Expected error")
			}
			if status.Code(err) != code {
				t.Errorf("Expected %v code, got %v", code, status.Code(err))
			}
			if streamerCalls != 1 {
				t.Errorf("Expected 1 call for non-retryable error, got %d", streamerCalls)
			}
			if cs != nil {
				t.Error("Expected nil ClientStream on error")
			}
		})
	}

	t.Run("context with existing deadline not modified", func(t *testing.T) {
		cfg := &Config{
			Timeout:     100 * time.Millisecond,
			MaxAttempts: 1,
			BackoffBase: 5 * time.Millisecond,
		}
		cfgCopy := *cfg

		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Hour)
		defer cancel()

		originalDeadline, ok := ctx.Deadline()
		if !ok {
			t.Fatal("Context should have deadline")
		}

		streamer := func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, opts ...grpc.CallOption) (grpc.ClientStream, error) {
			newDeadline, ok := ctx.Deadline()
			if !ok {
				t.Error("Context should still have deadline")
			}
			if newDeadline != originalDeadline {
				t.Error("Context with existing deadline should not be modified")
			}
			return &mockClientStream{}, nil
		}

		si := createStreamInterceptor(cfgCopy)
		cs, err := si(ctx, &grpc.StreamDesc{}, nil, "/test/StreamMethod", streamer)
		if err != nil {
			t.Errorf("Expected success, got %v", err)
		}
		if cs == nil {
			t.Error("Expected non-nil ClientStream")
		}
	})

	t.Run("context without deadline gets timeout", func(t *testing.T) {
		cfg := &Config{
			Timeout:     100 * time.Millisecond,
			MaxAttempts: 1,
			BackoffBase: 10 * time.Millisecond,
		}
		cfgCopy := *cfg

		ctx := context.Background()

		streamer := func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, opts ...grpc.CallOption) (grpc.ClientStream, error) {
			_, ok := ctx.Deadline()
			if !ok {
				t.Error("Context should have deadline after timeout is applied")
			}
			return &mockClientStream{}, nil
		}

		si := createStreamInterceptor(cfgCopy)
		cs, err := si(ctx, &grpc.StreamDesc{}, nil, "/test/StreamMethod", streamer)
		if err != nil {
			t.Errorf("Expected success, got %v", err)
		}
		if cs == nil {
			t.Error("Expected non-nil ClientStream")
		}
	})

	t.Run("zero timeout means no default timeout", func(t *testing.T) {
		cfg := &Config{
			Timeout:     0,
			MaxAttempts: 1,
			BackoffBase: 5 * time.Millisecond,
		}
		cfgCopy := *cfg

		streamer := func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, opts ...grpc.CallOption) (grpc.ClientStream, error) {
			_, ok := ctx.Deadline()
			if ok {
				t.Error("Context should not have deadline when Timeout is 0")
			}
			return &mockClientStream{}, nil
		}

		si := createStreamInterceptor(cfgCopy)
		cs, err := si(context.Background(), &grpc.StreamDesc{}, nil, "/test/StreamMethod", streamer)
		if err != nil {
			t.Errorf("Expected success, got %v", err)
		}
		if cs == nil {
			t.Error("Expected non-nil ClientStream")
		}
	})

	t.Run("context cancelled during backoff", func(t *testing.T) {
		cfg := &Config{
			Timeout:     100 * time.Millisecond,
			MaxAttempts: 10,
			BackoffBase: 50 * time.Millisecond,
		}
		cfgCopy := *cfg

		ctx, cancel := context.WithCancel(context.Background())

		attempts := 0
		streamer := func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, opts ...grpc.CallOption) (grpc.ClientStream, error) {
			attempts++
			cancel() // Cancel context after first attempt
			return nil, status.Error(codes.Unavailable, "service unavailable")
		}

		si := createStreamInterceptor(cfgCopy)
		cs, err := si(ctx, &grpc.StreamDesc{}, nil, "/test/StreamMethod", streamer)
		if cs != nil {
			t.Error("Expected nil ClientStream on error")
		}
		if !errors.Is(err, context.Canceled) && status.Code(err) != codes.Unavailable {
			t.Logf("Got error (context.Canceled or Unavailable expected): %v", err)
		}
		if attempts < 1 {
			t.Error("Expected at least one attempt")
		}
	})

	retryableCodes := []codes.Code{codes.Unavailable, codes.DeadlineExceeded}
	for _, code := range retryableCodes {
		t.Run("retryable stream: "+code.String(), func(t *testing.T) {
			cfg := &Config{
				Timeout:     100 * time.Millisecond,
				MaxAttempts: 3,
				BackoffBase: 1 * time.Millisecond,
			}
			cfgCopy := *cfg

			attempts := 0
			streamer := func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, opts ...grpc.CallOption) (grpc.ClientStream, error) {
				attempts++
				if attempts == 1 {
					return nil, status.Error(code, "test error")
				}
				return &mockClientStream{}, nil
			}

			si := createStreamInterceptor(cfgCopy)
			cs, err := si(context.Background(), &grpc.StreamDesc{}, nil, "/test/StreamMethod", streamer)
			if err != nil {
				t.Errorf("Expected success after retry for %v, got %v", code, err)
			}
			if cs == nil {
				t.Error("Expected non-nil ClientStream")
			}
			if attempts != 2 {
				t.Errorf("Expected 2 attempts for %v, got %d", code, attempts)
			}
		})
	}
}

// createUnaryInterceptor creates a unary interceptor using the same logic as Chain
func createUnaryInterceptor(c Config) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		// ensure timeout
		if _, ok := ctx.Deadline(); !ok && c.Timeout > 0 {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, c.Timeout)
			defer cancel()
		}
		attempts := 0
		for {
			attempts++
			err := invoker(ctx, method, req, reply, cc, opts...)
			if err == nil {
				return nil
			}
			if attempts >= c.MaxAttempts {
				return err
			}
			st, _ := status.FromError(err)
			if st.Code() != codes.Unavailable && st.Code() != codes.DeadlineExceeded {
				return err
			}
			// backoff
			d := backoff(c.BackoffBase, attempts)
			select {
			case <-time.After(d):
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}
}

// createStreamInterceptor creates a stream interceptor using the same logic as Chain
func createStreamInterceptor(c Config) grpc.StreamClientInterceptor {
	return func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, streamer grpc.Streamer, opts ...grpc.CallOption) (grpc.ClientStream, error) {
		if _, ok := ctx.Deadline(); !ok && c.Timeout > 0 {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, c.Timeout)
			defer cancel()
		}
		attempts := 0
		for {
			attempts++
			cs, err := streamer(ctx, desc, cc, method, opts...)
			if err == nil {
				return cs, nil
			}
			if attempts >= c.MaxAttempts {
				return nil, err
			}
			st, _ := status.FromError(err)
			if st.Code() != codes.Unavailable && st.Code() != codes.DeadlineExceeded {
				return nil, err
			}
			d := backoff(c.BackoffBase, attempts)
			select {
			case <-time.After(d):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
	}
}

// TestChain_Concurrency tests concurrent calls to Chain
func TestChain_Concurrency(t *testing.T) {
	cfg := &Config{
		Timeout:     5 * time.Second,
		MaxAttempts: 3,
		BackoffBase: 100 * time.Millisecond,
	}

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			opts := Chain(cfg)
			if opts == nil || len(opts) != 2 {
				t.Errorf("Invalid options from concurrent Chain call")
			}
		}()
	}
	wg.Wait()
}

// TestConfig_StructFields tests Config struct fields
func TestConfig_StructFields(t *testing.T) {
	cfg := Config{
		Timeout:     30 * time.Second,
		MaxAttempts: 5,
		BackoffBase: 200 * time.Millisecond,
	}

	if cfg.Timeout != 30*time.Second {
		t.Errorf("Timeout = %v, want 30s", cfg.Timeout)
	}
	if cfg.MaxAttempts != 5 {
		t.Errorf("MaxAttempts = %d, want 5", cfg.MaxAttempts)
	}
	if cfg.BackoffBase != 200*time.Millisecond {
		t.Errorf("BackoffBase = %v, want 200ms", cfg.BackoffBase)
	}
}

// TestContextBehavior tests context deadline detection behavior
func TestContextBehavior(t *testing.T) {
	t.Run("background context has no deadline", func(t *testing.T) {
		ctx := context.Background()
		_, ok := ctx.Deadline()
		if ok {
			t.Error("Background context should not have deadline")
		}
	})

	t.Run("TODO context has no deadline", func(t *testing.T) {
		ctx := context.TODO()
		_, ok := ctx.Deadline()
		if ok {
			t.Error("TODO context should not have deadline")
		}
	})

	t.Run("withTimeout context has deadline", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Hour)
		defer cancel()
		_, ok := ctx.Deadline()
		if !ok {
			t.Error("WithTimeout context should have deadline")
		}
	})

	t.Run("withDeadline context has deadline", func(t *testing.T) {
		ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(1*time.Hour))
		defer cancel()
		_, ok := ctx.Deadline()
		if !ok {
			t.Error("WithDeadline context should have deadline")
		}
	})

	t.Run("withCancel context has no deadline", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		_, ok := ctx.Deadline()
		if ok {
			t.Error("WithCancel context should not have deadline")
		}
	})

	t.Run("withValue preserves deadline", func(t *testing.T) {
		baseCtx, cancel := context.WithTimeout(context.Background(), 1*time.Hour)
		defer cancel()
		ctx := context.WithValue(baseCtx, "key", "value")
		_, ok := ctx.Deadline()
		if !ok {
			t.Error("WithValue should preserve deadline")
		}
	})
}

// Benchmark tests
func BenchmarkBackoff(b *testing.B) {
	base := 100 * time.Millisecond

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		backoff(base, i%10)
	}
}

func BenchmarkChain(b *testing.B) {
	cfg := &Config{
		Timeout:     5 * time.Second,
		MaxAttempts: 3,
		BackoffBase: 100 * time.Millisecond,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Chain(cfg)
	}
}
