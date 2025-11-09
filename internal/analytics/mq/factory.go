package mq

import (
	"log"
	"os"
)

// NewFromEnv builds a Queue based on env configuration.
// ANALYTICS_MQ_TYPE: redis|kafka|noop (default)
// For redis/kafka, real implementations can be added without changing callers.
func NewFromEnv() Queue {
	t := os.Getenv("ANALYTICS_MQ_TYPE")
	switch t {
	case "redis":
		if q, err := newRedisFromEnv(); err == nil && q != nil {
			log.Printf("[analytics-mq] redis publisher enabled")
			return q
		}
		log.Printf("[analytics-mq] redis requested; fallback to noop (build with -tags redis_mq to enable)")
		return NewNoop()
	case "kafka":
		if q, err := newKafkaFromEnv(); err == nil && q != nil {
			return q
		}
		log.Printf("[analytics-mq] kafka requested; fallback to noop")
		return NewNoop()
	default:
		if t == "" {
			log.Printf("[analytics-mq] ANALYTICS_MQ_TYPE not set; using noop")
		} else {
			log.Printf("[analytics-mq] unsupported type %q; using noop", t)
		}
		return NewNoop()
	}
}
