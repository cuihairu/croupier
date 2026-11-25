package mq

import (
	"errors"
	"fmt"
	"os"
)

// Injectable seam: the production path is equivalent to calling
// newRedisFromEnv / newKafkaFromEnv directly. Both constructors' only return
// shape is (non-nil Queue, nil), so the (err != nil) and (q == nil) defensive
// branches below are reachable only when a fault is injected.
var (
	newRedisFromEnvFn = newRedisFromEnv
	newKafkaFromEnvFn = newKafkaFromEnv
)

// NewFromEnv builds a Queue based on env configuration.
//
// ANALYTICS_MQ_TYPE: redis|kafka|noop. Defaults to redis: the ingest
// service exists to forward events to MQ, so silently degrading to noop
// (dropping every event) is a misconfiguration foot-gun. Explicit
// ANALYTICS_MQ_TYPE=noop stays available for local debugging without Redis.
//
// Construction failures are returned as errors (fail-fast) instead of
// silently falling back to noop.
func NewFromEnv() (Queue, error) {
	t := os.Getenv("ANALYTICS_MQ_TYPE")
	if t == "" {
		t = "redis"
	}
	switch t {
	case "redis":
		q, err := newRedisFromEnvFn()
		if err != nil {
			return nil, fmt.Errorf("analytics redis mq: %w", err)
		}
		if q == nil {
			return nil, errors.New("analytics redis mq: nil publisher")
		}
		return q, nil
	case "kafka":
		q, err := newKafkaFromEnvFn()
		if err != nil {
			return nil, fmt.Errorf("analytics kafka mq: %w", err)
		}
		if q == nil {
			return nil, errors.New("analytics kafka mq: nil publisher")
		}
		return q, nil
	case "noop":
		return NewNoop(), nil
	default:
		return nil, fmt.Errorf("unsupported ANALYTICS_MQ_TYPE %q (supported: redis|kafka|noop)", t)
	}
}
