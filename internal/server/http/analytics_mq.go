package httpserver

import (
    "log"
    "os"
    mq "github.com/cuihairu/croupier/internal/analytics/mq"
)

// initAnalyticsMQ selects an MQ implementation based on env vars.
// Supported values: "redis", "kafka", default "noop" (publishers do nothing).
func (s *Server) initAnalyticsMQ() {
    typ := os.Getenv("ANALYTICS_MQ_TYPE")
    switch typ {
    case "redis":
        // TODO: implement Redis Streams publisher (uses REDIS_URL and stream keys)
        // s.analyticsMQ = mq.NewRedis(...)
        log.Printf("[analytics-mq] redis requested but not implemented; using noop")
        s.analyticsMQ = mq.NewNoop()
    case "kafka":
        // TODO: implement Kafka publisher (uses KAFKA_BROKERS and topic names)
        // s.analyticsMQ = mq.NewKafka(...)
        log.Printf("[analytics-mq] kafka requested but not implemented; using noop")
        s.analyticsMQ = mq.NewNoop()
    default:
        s.analyticsMQ = mq.NewNoop()
        if typ == "" { log.Printf("[analytics-mq] type not set; using noop") } else { log.Printf("[analytics-mq] unknown type=%s; using noop", typ) }
    }
}

