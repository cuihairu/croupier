package logic

import (
	"context"
	"time"

	"github.com/cuihairu/croupier/services/server/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type HealthzLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewHealthzLogic(ctx context.Context, svcCtx *svc.ServiceContext) *HealthzLogic {
	return &HealthzLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// Healthz returns basic health status - enhanced version for go-zero migration
func (l *HealthzLogic) Healthz() (string, error) {
	// Basic health check
	now := time.Now()

	// Check if ServiceContext is properly initialized
	if l.svcCtx == nil {
		return "error: service context not initialized", nil
	}

	// Check if service has been running for at least some time
	uptime := now.Sub(l.svcCtx.GetStartTime())
	if uptime < 0 {
		return "error: invalid service start time", nil
	}

	// Return enhanced health status
	return "ok", nil
}

// DetailedHealth returns detailed health information with system status
func (l *HealthzLogic) DetailedHealth() (map[string]interface{}, error) {
	now := time.Now()

	// Get basic service information
	uptime := now.Sub(l.svcCtx.GetStartTime())

	// Get health checks and statuses
	checks, statuses := l.svcCtx.HealthSnapshot()

	// Build health response
	health := map[string]interface{}{
		"service": map[string]interface{}{
			"name":    "croupier-api",
			"status":  "healthy",
			"uptime":  uptime.String(),
			"version": l.svcCtx.GetVersion(),
		},
		"health_checks": map[string]interface{}{
			"total":   len(checks),
			"healthy": len(statuses),
		},
		"timestamp": now.Format(time.RFC3339),
	}

	// Add health check details
	if len(checks) > 0 {
		healthCheckResults := make([]map[string]interface{}, 0, len(checks))
		for _, check := range checks {
			status := "unknown"
			lastCheck := "never"

			// Find corresponding status
			for _, st := range statuses {
				if st.ID == check.ID {
					status = map[bool]string{true: "healthy", false: "unhealthy"}[st.OK]
					lastCheck = st.CheckedAt.Format(time.RFC3339)
					break
				}
			}

			healthCheckResults = append(healthCheckResults, map[string]interface{}{
				"id":           check.ID,
				"kind":         check.Kind,
				"target":       check.Target,
				"status":       status,
				"last_checked": lastCheck,
				"interval_sec": check.IntervalSec,
			})
		}

		// Type assertion to safely update the nested map
		if hc, ok := health["health_checks"].(map[string]interface{}); ok {
			hc["details"] = healthCheckResults
		}
	}

	return health, nil
}
