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
		log.Printf("[analytics-mq] redis requested; using noop placeholder (implement me)")
		return NewNoop()
	case "kafka":
		log.Printf("[analytics-mq] kafka requested; using noop placeholder (implement me)")
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
