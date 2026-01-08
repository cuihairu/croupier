// Package ratelimit provides rate limiting utilities for platform providers.
package ratelimit

import (
	"context"
	"time"
)

// Limiter defines the rate limiting interface.
type Limiter interface {
	// Wait blocks until a token is available or context is canceled.
	Wait(ctx context.Context) error
}

// TokenBucket implements a token bucket rate limiter.
type TokenBucket struct {
	tokens     chan struct{}
	refillRate time.Duration
}

// NewTokenBucket creates a new token bucket rate limiter.
// requestsPerMinute is the maximum requests allowed per minute.
// burstSize is the maximum burst size.
func NewTokenBucket(requestsPerMinute, burstSize int) *TokenBucket {
	if requestsPerMinute <= 0 {
		requestsPerMinute = 60
	}
	if burstSize <= 0 {
		burstSize = requestsPerMinute / 60
		if burstSize < 1 {
			burstSize = 1
		}
	}

	tb := &TokenBucket{
		tokens:     make(chan struct{}, burstSize),
		refillRate: time.Minute / time.Duration(requestsPerMinute),
	}

	// Fill initial tokens
	for i := 0; i < burstSize; i++ {
		tb.tokens <- struct{}{}
	}

	// Start refill goroutine
	go tb.refill()

	return tb
}

// refill continuously adds tokens to the bucket.
func (tb *TokenBucket) refill() {
	ticker := time.NewTicker(tb.refillRate)
	defer ticker.Stop()

	for range ticker.C {
		select {
		case tb.tokens <- struct{}{}:
		default:
			// Bucket is full, discard token
		}
	}
}

// Wait waits for a token to be available.
func (tb *TokenBucket) Wait(ctx context.Context) error {
	select {
	case <-tb.tokens:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
