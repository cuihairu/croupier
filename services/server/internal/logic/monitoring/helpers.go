package monitoring

import (
	"context"
	"time"

	reg "github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/cuihairu/croupier/services/server/internal/logic/utils"
	"github.com/cuihairu/croupier/services/server/internal/svc"
)

var serverBootTime = time.Now().UTC()

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

func summarizeOpsState(ctx *svc.ServiceContext) map[string]interface{} {
	summary := map[string]interface{}{
		"ok": false,
	}
	if ctx == nil || ctx.OpsStateStore == nil {
		summary["error"] = "ops state store not initialized"
		return summary
	}

	state := ctx.OpsStateStore.Snapshot()
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
