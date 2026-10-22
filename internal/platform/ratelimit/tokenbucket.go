// Package ratelimit provides rate limiting utilities for platform providers.
package ratelimit

import (
	"context"

	"golang.org/x/time/rate"
)

// Limiter defines the rate limiting interface.
type Limiter interface {
	// Wait blocks until a token is available or context is canceled.
	Wait(ctx context.Context) error
}

// TokenBucket 令牌桶限流器，桶层委托 golang.org/x/time/rate.Limiter（Go 官方
// 扩展库，单调时钟记账）。取代旧手写 channel 桶——后者 ticker 逐个回填、每桶
// 常驻一条 goroutine，且等待语义对慢机时序敏感（TestSlidingWindowWaitVariants
// 反复 flaky 即同族证据）。能力矩阵 #2（Batch B）：换。
type TokenBucket struct {
	lim *rate.Limiter
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

	return &TokenBucket{
		lim: rate.NewLimiter(rate.Limit(requestsPerMinute)/60.0, burstSize),
	}
}

// Wait waits for a token to be available.
func (tb *TokenBucket) Wait(ctx context.Context) error {
	return tb.lim.Wait(ctx)
}

// Allow 非阻塞预检：有令牌立即取走并返回 true，无令牌立即 false。
func (tb *TokenBucket) Allow() bool {
	return tb.lim.Allow()
}
