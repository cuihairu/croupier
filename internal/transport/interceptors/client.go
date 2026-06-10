package interceptors

import (
	"context"
	"math"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Config is a minimal retry/timeout configuration.
type Config struct {
	Timeout     time.Duration // per-call default timeout if context has no deadline
	MaxAttempts int           // including first attempt
	BackoffBase time.Duration // backoff base
}

func defaultConfig() Config {
	return Config{Timeout: 5 * time.Second, MaxAttempts: 3, BackoffBase: 100 * time.Millisecond}
}

// Chain returns dial options with unary/stream interceptors for timeout and simple retry.
func Chain(cfg *Config) []grpc.DialOption {
	c := defaultConfig()
	if cfg != nil {
		c = *cfg
	}
	return []grpc.DialOption{
		grpc.WithChainUnaryInterceptor(NewUnaryRetryInterceptor(c)),
		grpc.WithChainStreamInterceptor(NewStreamRetryInterceptor(c)),
	}
}

// NewUnaryRetryInterceptor creates a unary client interceptor with timeout and retry.
func NewUnaryRetryInterceptor(c Config) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
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
			d := backoff(c.BackoffBase, attempts)
			select {
			case <-time.After(d):
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}
}

// NewStreamRetryInterceptor creates a stream client interceptor with timeout and retry.
func NewStreamRetryInterceptor(c Config) grpc.StreamClientInterceptor {
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

func backoff(base time.Duration, attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	pow := math.Pow(2, float64(attempt-1))
	return time.Duration(float64(base) * pow)
}
