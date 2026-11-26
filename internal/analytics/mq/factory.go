package mq

import (
	"errors"
	"fmt"
	"os"
)

// Injectable seam: the production path is equivalent to calling
// newRedisFromEnv / newKafkaFromEnv directly. The redis constructor returns a
// real error for an unparseable REDIS_URL (DATA-043), so the (err != nil)
// branch below is reachable in production; the (q == nil) guards stay
// defensive. The kafka constructor's only return shape is (non-nil Queue,
// nil), so its branches trigger only on injected faults.
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
