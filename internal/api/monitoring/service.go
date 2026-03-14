package monitoring

import (
	"context"
	"time"

	"github.com/cuihairu/croupier/internal/logic/utils"
	reg "github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/cuihairu/croupier/internal/svc"
)

var serverBootTime = time.Now().UTC()

type Service struct {
	svcCtx *svc.ServiceContext
}

func NewService(svcCtx *svc.ServiceContext) *Service {
	return &Service{svcCtx: svcCtx}
}

// Healthz returns health check status
func (s *Service) Healthz(ctx context.Context, req *HealthzRequest) (*HealthzResponse, error) {
	dbStatus := checkDatabaseHealth(ctx, s.svcCtx)
	registryStatus, _ := collectRegistryStats(s.svcCtx.RegistryStore)
	opsStatus := summarizeOpsState(s.svcCtx)

	ok := componentHealthy(dbStatus) && componentHealthy(registryStatus) && componentHealthy(opsStatus)
	message := "OK"
	if !ok {
		message = "DEGRADED"
	}

	data := map[string]interface{}{
		"ok":             ok,
		"timestamp":      utils.FormatTimestamp(time.Now()),
		"uptime_seconds": uptimeSeconds(),
		"components": map[string]interface{}{
			"database": dbStatus,
			"registry": registryStatus,
			"ops":      opsStatus,
		},
	}

	return &HealthzResponse{
		Code:    0,
		Message: message,
		Data:    data,
	}, nil
}

// Metrics returns system metrics
func (s *Service) Metrics(ctx context.Context, req *MetricsRequest) (*MetricsResponse, error) {
	dbStatus := checkDatabaseHealth(ctx, s.svcCtx)
	registryStatus, snapshots := collectRegistryStats(s.svcCtx.RegistryStore)
	opsStatus := summarizeOpsState(s.svcCtx)

	counts := map[string]interface{}{
		"agents_total":         registryStatus["agents_total"],
		"agents_healthy":       registryStatus["agents_healthy"],
		"functions_registered": registryStatus["functions_registered"],
		"maintenance_windows":  opsStatus["maintenance_windows"],
		"health_checks":        opsStatus["health_checks"],
		"alerts":               opsStatus["alerts"],
	}

	data := map[string]interface{}{
		"timestamp": utils.FormatTimestamp(time.Now()),
		"counts":    counts,
		"database": map[string]interface{}{
			"ok":         dbStatus["ok"],
			"latency_ms": dbStatus["latency_ms"],
			"driver":     dbStatus["driver"],
		},
		"registry": map[string]interface{}{
			"ok":       registryStatus["ok"],
			"agents":   snapshots,
			"metadata": registryStatus,
		},
		"ops": map[string]interface{}{
			"ok":            opsStatus["ok"],
			"mq_type":       opsStatus["mq_type"],
			"mq_lengths":    opsStatus["mq_lengths"],
			"health":        opsStatus["health_status"],
			"notifications": opsStatus["notifications"],
		},
	}

	return &MetricsResponse{
		Code:    0,
		Message: "OK",
		Data:    data,
	}, nil
}

// Status returns detailed system status
func (s *Service) Status(ctx context.Context, req *StatusRequest) (*StatusResponse, error) {
	dbStatus := checkDatabaseHealth(ctx, s.svcCtx)
	registryStatus, snapshots := collectRegistryStats(s.svcCtx.RegistryStore)
	opsStatus := summarizeOpsState(s.svcCtx)

	ok := componentHealthy(dbStatus) && componentHealthy(registryStatus) && componentHealthy(opsStatus)

	data := map[string]interface{}{
		"ok":             ok,
		"timestamp":      utils.FormatTimestamp(time.Now()),
		"uptime_seconds": uptimeSeconds(),
		"database":       dbStatus,
		"registry":       registryStatus,
		"ops":            opsStatus,
		"agents":         snapshots,
	}

	return &StatusResponse{
		Code:    0,
		Message: "OK",
		Data:    data,
	}, nil
}

func checkDatabaseHealth(ctx context.Context, svcCtx *svc.ServiceContext) map[string]interface{} {
	status := map[string]interface{}{
		"ok":     false,
		"driver": "",
	}
	if svcCtx != nil {
		status["driver"] = svcCtx.Config.Database.Driver
	}
	if svcCtx == nil || svcCtx.DB == nil {
		status["error"] = "database not initialized"
		return status
	}

	start := time.Now()
	err := svcCtx.DB.WithContext(ctx).Exec("SELECT 1").Error
	status["latency_ms"] = time.Since(start).Milliseconds()
	if err != nil {
		status["error"] = err.Error()
		return status
	}
	status["ok"] = true
	return status
}

func collectRegistryStats(store *reg.Store) (map[string]interface{}, []map[string]interface{}) {
	stats := map[string]interface{}{
		"ok":                   false,
		"agents_total":         0,
		"agents_healthy":       0,
		"functions_registered": 0,
	}
	snapshots := make([]map[string]interface{}, 0)
	if store == nil {
		stats["error"] = "registry store not initialized"
		return stats, snapshots
	}

	store.Mu().RLock()
	defer store.Mu().RUnlock()

	for _, sess := range store.AgentsUnsafe() {
		if sess == nil {
			continue
		}
		stats["agents_total"] = stats["agents_total"].(int) + 1
		if time.Until(sess.ExpireAt) > 0 {
			stats["agents_healthy"] = stats["agents_healthy"].(int) + 1
		}
		stats["functions_registered"] = stats["functions_registered"].(int) + utils.CountEnabledFunctions(sess.Functions)

		if snapshot := utils.BuildOpsAgentSnapshot(sess); snapshot != nil {
			snapshots = append(snapshots, snapshot)
		}
	}

	stats["ok"] = true
	return stats, snapshots
}

func summarizeOpsState(svcCtx *svc.ServiceContext) map[string]interface{} {
	summary := map[string]interface{}{
		"ok": false,
	}
	if svcCtx == nil || svcCtx.OpsStateStore == nil {
		summary["error"] = "ops state store not initialized"
		return summary
	}

	state := svcCtx.OpsStateStore.Snapshot()
	summary["ok"] = true
	summary["maintenance_windows"] = len(state.Maintenance.Windows)
	summary["health_checks"] = len(state.Health.Checks)
	summary["health_status"] = len(state.Health.Status)
	summary["notifications"] = len(state.Notifications.Channels)
	summary["alerts"] = len(state.Alerts.Silences)
	summary["mq_type"] = state.MQ.Type
	if state.MQ.Lengths != nil {
		summary["mq_lengths"] = state.MQ.Lengths
	}
	return summary
}

func componentHealthy(status map[string]interface{}) bool {
	if status == nil {
		return false
	}
	if ok, exists := status["ok"].(bool); exists {
		return ok
	}
	return false
}

func uptimeSeconds() int64 {
	return int64(time.Since(serverBootTime).Seconds())
}
