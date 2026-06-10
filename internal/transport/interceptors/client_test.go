package interceptors

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func TestDefaultConfig(t *testing.T) {
	cfg := defaultConfig()
	if cfg.Timeout != 5*time.Second {
		t.Errorf("expected Timeout 5s, got %v", cfg.Timeout)
	}
	if cfg.MaxAttempts != 3 {
		t.Errorf("expected MaxAttempts 3, got %d", cfg.MaxAttempts)
	}
	if cfg.BackoffBase != 100*time.Millisecond {
		t.Errorf("expected BackoffBase 100ms, got %v", cfg.BackoffBase)
	}
}

func TestChain_ReturnsOptions(t *testing.T) {
	tests := []struct {
		name string
		cfg  *Config
	}{
		{"nil config", nil},
		{"custom config", &Config{Timeout: 10 * time.Second, MaxAttempts: 5, BackoffBase: 200 * time.Millisecond}},
		{"zero timeout", &Config{Timeout: 0, MaxAttempts: 3}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := Chain(tt.cfg)
			if len(opts) != 2 {
				t.Errorf("expected 2 dial options, got %d", len(opts))
			}
		})
	}
}

func TestBackoff(t *testing.T) {
	tests := []struct {
		base    time.Duration
		attempt int
		want    time.Duration
	}{
		{100 * time.Millisecond, 1, 100 * time.Millisecond},
		{100 * time.Millisecond, 2, 200 * time.Millisecond},
		{100 * time.Millisecond, 3, 400 * time.Millisecond},
		{100 * time.Millisecond, 0, 100 * time.Millisecond},
		{100 * time.Millisecond, -1, 100 * time.Millisecond},
		{0, 1, 0},
		{50 * time.Millisecond, 4, 400 * time.Millisecond},
	}
	for _, tt := range tests {
		got := backoff(tt.base, tt.attempt)
		if got != tt.want {
			t.Errorf("backoff(%v, %d) = %v, want %v", tt.base, tt.attempt, got, tt.want)
		}
	}
}

func TestBackoff_Exponential(t *testing.T) {
	base := 100 * time.Millisecond
	prev := backoff(base, 1)
	for i := 2; i <= 5; i++ {
		curr := backoff(base, i)
		if curr != 2*prev {
			t.Errorf("attempt %d: expected %v, got %v", i, 2*prev, curr)
		}
		prev = curr
	}
}

type mockClientStream struct {
	grpc.ClientStream
}

func (m *mockClientStream) Header() (metadata.MD, error) { return nil, nil }
func (m *mockClientStream) Trailer() metadata.MD          { return nil }
func (m *mockClientStream) CloseSend() error              { return nil }
func (m *mockClientStream) Context() context.Context      { return context.Background() }
func (m *mockClientStream) SendMsg(msg interface{}) error { return nil }
func (m *mockClientStream) RecvMsg(msg interface{}) error { return nil }

func TestUnaryRetryInterceptor(t *testing.T) {
	t.Run("success on first attempt", func(t *testing.T) {
		ui := NewUnaryRetryInterceptor(Config{Timeout: time.Second, MaxAttempts: 3, BackoffBase: time.Millisecond})
		called := false
		err := ui(context.Background(), "/test/M", nil, nil, nil, func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, opts ...grpc.CallOption) error {
			called = true
			return nil
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !called {
			t.Error("invoker not called")
		}
	})

	t.Run("retry on Unavailable", func(t *testing.T) {
		ui := NewUnaryRetryInterceptor(Config{Timeout: time.Second, MaxAttempts: 3, BackoffBase: time.Millisecond})
		attempts := 0
		err := ui(context.Background(), "/test/M", nil, nil, nil, func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, opts ...grpc.CallOption) error {
			attempts++
			if attempts == 1 {
				return status.Error(codes.Unavailable, "unavailable")
			}
			return nil
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if attempts != 2 {
			t.Errorf("expected 2 attempts, got %d", attempts)
		}
	})

	t.Run("retry on DeadlineExceeded", func(t *testing.T) {
		ui := NewUnaryRetryInterceptor(Config{Timeout: time.Second, MaxAttempts: 3, BackoffBase: time.Millisecond})
		attempts := 0
		err := ui(context.Background(), "/test/M", nil, nil, nil, func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, opts ...grpc.CallOption) error {
			attempts++
			if attempts == 1 {
				return status.Error(codes.DeadlineExceeded, "timeout")
			}
			return nil
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if attempts != 2 {
			t.Errorf("expected 2 attempts, got %d", attempts)
		}
	})

	t.Run("max attempts exceeded", func(t *testing.T) {
		ui := NewUnaryRetryInterceptor(Config{Timeout: 100 * time.Millisecond, MaxAttempts: 2, BackoffBase: time.Millisecond})
		attempts := 0
		err := ui(context.Background(), "/test/M", nil, nil, nil, func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, opts ...grpc.CallOption) error {
			attempts++
			return status.Error(codes.Unavailable, "unavailable")
		})
		if status.Code(err) != codes.Unavailable {
			t.Errorf("expected Unavailable, got %v", status.Code(err))
		}
		if attempts != 2 {
			t.Errorf("expected 2 attempts, got %d", attempts)
		}
	})

	t.Run("non-retryable error", func(t *testing.T) {
		ui := NewUnaryRetryInterceptor(Config{Timeout: time.Second, MaxAttempts: 3, BackoffBase: time.Millisecond})
		attempts := 0
		err := ui(context.Background(), "/test/M", nil, nil, nil, func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, opts ...grpc.CallOption) error {
			attempts++
			return status.Error(codes.InvalidArgument, "bad")
		})
		if status.Code(err) != codes.InvalidArgument {
			t.Errorf("expected InvalidArgument, got %v", status.Code(err))
		}
		if attempts != 1 {
			t.Errorf("expected 1 attempt, got %d", attempts)
		}
	})

	t.Run("context cancelled during backoff", func(t *testing.T) {
		ui := NewUnaryRetryInterceptor(Config{Timeout: time.Second, MaxAttempts: 10, BackoffBase: 50 * time.Millisecond})
		ctx, cancel := context.WithCancel(context.Background())
		attempts := 0
		err := ui(ctx, "/test/M", nil, nil, nil, func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, opts ...grpc.CallOption) error {
			attempts++
			cancel()
			return status.Error(codes.Unavailable, "unavailable")
		})
		if !errors.Is(err, context.Canceled) && status.Code(err) != codes.Unavailable {
			t.Logf("got error: %v", err)
		}
		if attempts < 1 {
			t.Error("expected at least 1 attempt")
		}
	})

	t.Run("existing deadline preserved", func(t *testing.T) {
		ui := NewUnaryRetryInterceptor(Config{Timeout: 100 * time.Millisecond, MaxAttempts: 1, BackoffBase: time.Millisecond})
		ctx, cancel := context.WithTimeout(context.Background(), time.Hour)
		defer cancel()
		origDeadline, _ := ctx.Deadline()
		_ = ui(ctx, "/test/M", nil, nil, nil, func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, opts ...grpc.CallOption) error {
			dl, ok := ctx.Deadline()
			if !ok || dl != origDeadline {
				t.Error("deadline should be preserved")
			}
			return nil
		})
	})

	t.Run("no deadline when timeout is 0", func(t *testing.T) {
		ui := NewUnaryRetryInterceptor(Config{Timeout: 0, MaxAttempts: 1, BackoffBase: time.Millisecond})
		_ = ui(context.Background(), "/test/M", nil, nil, nil, func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, opts ...grpc.CallOption) error {
			if _, ok := ctx.Deadline(); ok {
				t.Error("should not have deadline when timeout is 0")
			}
			return nil
		})
	})

	t.Run("context deadline exceeded during backoff wait", func(t *testing.T) {
		ui := NewUnaryRetryInterceptor(Config{Timeout: 0, MaxAttempts: 10, BackoffBase: 100 * time.Millisecond})
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()
		attempts := 0
		err := ui(ctx, "/test/M", nil, nil, nil, func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, opts ...grpc.CallOption) error {
			attempts++
			return status.Error(codes.Unavailable, "unavailable")
		})
		if err == nil {
			t.Error("expected error")
		}
	})

	t.Run("non-grpc error", func(t *testing.T) {
		ui := NewUnaryRetryInterceptor(Config{Timeout: time.Second, MaxAttempts: 3, BackoffBase: time.Millisecond})
		attempts := 0
		err := ui(context.Background(), "/test/M", nil, nil, nil, func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, opts ...grpc.CallOption) error {
			attempts++
			return errors.New("plain error")
		})
		if err == nil {
			t.Error("expected error")
		}
		if attempts != 1 {
			t.Errorf("expected 1 attempt, got %d", attempts)
		}
	})
}

func TestStreamRetryInterceptor(t *testing.T) {
	t.Run("success on first attempt", func(t *testing.T) {
		si := NewStreamRetryInterceptor(Config{Timeout: time.Second, MaxAttempts: 3, BackoffBase: time.Millisecond})
		cs, err := si(context.Background(), nil, nil, "/test/S", func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, opts ...grpc.CallOption) (grpc.ClientStream, error) {
			return &mockClientStream{}, nil
		})
		if err != nil || cs == nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("retry on Unavailable", func(t *testing.T) {
		si := NewStreamRetryInterceptor(Config{Timeout: time.Second, MaxAttempts: 3, BackoffBase: time.Millisecond})
		attempts := 0
		cs, err := si(context.Background(), nil, nil, "/test/S", func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, opts ...grpc.CallOption) (grpc.ClientStream, error) {
			attempts++
			if attempts == 1 {
				return nil, status.Error(codes.Unavailable, "unavailable")
			}
			return &mockClientStream{}, nil
		})
		if err != nil || cs == nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if attempts != 2 {
			t.Errorf("expected 2 attempts, got %d", attempts)
		}
	})

	t.Run("retry on DeadlineExceeded", func(t *testing.T) {
		si := NewStreamRetryInterceptor(Config{Timeout: time.Second, MaxAttempts: 3, BackoffBase: time.Millisecond})
		attempts := 0
		cs, err := si(context.Background(), nil, nil, "/test/S", func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, opts ...grpc.CallOption) (grpc.ClientStream, error) {
			attempts++
			if attempts == 1 {
				return nil, status.Error(codes.DeadlineExceeded, "timeout")
			}
			return &mockClientStream{}, nil
		})
		if err != nil || cs == nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if attempts != 2 {
			t.Errorf("expected 2 attempts, got %d", attempts)
		}
	})

	t.Run("max attempts exceeded", func(t *testing.T) {
		si := NewStreamRetryInterceptor(Config{Timeout: 100 * time.Millisecond, MaxAttempts: 2, BackoffBase: time.Millisecond})
		attempts := 0
		_, err := si(context.Background(), nil, nil, "/test/S", func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, opts ...grpc.CallOption) (grpc.ClientStream, error) {
			attempts++
			return nil, status.Error(codes.Unavailable, "unavailable")
		})
		if status.Code(err) != codes.Unavailable {
			t.Errorf("expected Unavailable, got %v", status.Code(err))
		}
		if attempts != 2 {
			t.Errorf("expected 2 attempts, got %d", attempts)
		}
	})

	t.Run("non-retryable error", func(t *testing.T) {
		si := NewStreamRetryInterceptor(Config{Timeout: time.Second, MaxAttempts: 3, BackoffBase: time.Millisecond})
		attempts := 0
		_, err := si(context.Background(), nil, nil, "/test/S", func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, opts ...grpc.CallOption) (grpc.ClientStream, error) {
			attempts++
			return nil, status.Error(codes.PermissionDenied, "forbidden")
		})
		if status.Code(err) != codes.PermissionDenied {
			t.Errorf("expected PermissionDenied, got %v", status.Code(err))
		}
		if attempts != 1 {
			t.Errorf("expected 1 attempt, got %d", attempts)
		}
	})

	t.Run("context cancelled during backoff", func(t *testing.T) {
		si := NewStreamRetryInterceptor(Config{Timeout: time.Second, MaxAttempts: 10, BackoffBase: 50 * time.Millisecond})
		ctx, cancel := context.WithCancel(context.Background())
		attempts := 0
		_, err := si(ctx, nil, nil, "/test/S", func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, opts ...grpc.CallOption) (grpc.ClientStream, error) {
			attempts++
			cancel()
			return nil, status.Error(codes.Unavailable, "unavailable")
		})
		if !errors.Is(err, context.Canceled) && status.Code(err) != codes.Unavailable {
			t.Logf("got error: %v", err)
		}
	})

	t.Run("existing deadline preserved", func(t *testing.T) {
		si := NewStreamRetryInterceptor(Config{Timeout: 100 * time.Millisecond, MaxAttempts: 1, BackoffBase: time.Millisecond})
		ctx, cancel := context.WithTimeout(context.Background(), time.Hour)
		defer cancel()
		origDeadline, _ := ctx.Deadline()
		_, _ = si(ctx, nil, nil, "/test/S", func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, opts ...grpc.CallOption) (grpc.ClientStream, error) {
			dl, ok := ctx.Deadline()
			if !ok || dl != origDeadline {
				t.Error("deadline should be preserved")
			}
			return &mockClientStream{}, nil
		})
	})

	t.Run("no deadline when timeout is 0", func(t *testing.T) {
		si := NewStreamRetryInterceptor(Config{Timeout: 0, MaxAttempts: 1, BackoffBase: time.Millisecond})
		_, _ = si(context.Background(), nil, nil, "/test/S", func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, opts ...grpc.CallOption) (grpc.ClientStream, error) {
			if _, ok := ctx.Deadline(); ok {
				t.Error("should not have deadline when timeout is 0")
			}
			return &mockClientStream{}, nil
		})
	})

	t.Run("non-grpc error", func(t *testing.T) {
		si := NewStreamRetryInterceptor(Config{Timeout: time.Second, MaxAttempts: 3, BackoffBase: time.Millisecond})
		attempts := 0
		_, err := si(context.Background(), nil, nil, "/test/S", func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, opts ...grpc.CallOption) (grpc.ClientStream, error) {
			attempts++
			return nil, errors.New("plain error")
		})
		if err == nil {
			t.Error("expected error")
		}
		if attempts != 1 {
			t.Errorf("expected 1 attempt, got %d", attempts)
		}
	})

	t.Run("context deadline exceeded during backoff wait", func(t *testing.T) {
		si := NewStreamRetryInterceptor(Config{Timeout: 0, MaxAttempts: 10, BackoffBase: 100 * time.Millisecond})
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()
		_, err := si(ctx, nil, nil, "/test/S", func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, opts ...grpc.CallOption) (grpc.ClientStream, error) {
			return nil, status.Error(codes.Unavailable, "unavailable")
		})
		if err == nil {
			t.Error("expected error")
		}
	})
}

func TestChain_Concurrency(t *testing.T) {
	cfg := &Config{Timeout: 5 * time.Second, MaxAttempts: 3, BackoffBase: 100 * time.Millisecond}
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			opts := Chain(cfg)
			if len(opts) != 2 {
				t.Errorf("expected 2 options")
			}
		}()
	}
	wg.Wait()
}
