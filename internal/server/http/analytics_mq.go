package httpserver

import (
	mq "github.com/cuihairu/croupier/internal/analytics/mq"
)

// initAnalyticsMQ selects an MQ implementation based on env vars.
// Supported values: "redis", "kafka", default "noop" (publishers do nothing).
func (s *Server) initAnalyticsMQ() {
	s.analyticsMQ = mq.NewFromEnv()
}
