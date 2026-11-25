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

	return &HealthzResponse{
		OK:            componentHealthy(dbStatus) && componentHealthy(registryStatus) && componentHealthy(opsStatus),
		Timestamp:     utils.FormatTimestamp(time.Now()),
		UptimeSeconds: uptimeSeconds(),
		Components: MonitoringComponents{
			Database: dbStatus,
			Registry: registryStatus,
			Ops:      opsStatus,
		},
	}, nil
}

// Ready is the readiness probe verdict (ARCH-004): the readiness face beyond
// the liveness probe (/healthz) — on top of database/registry/ops state it adds
// the cluster requirement: when cluster.enabled, an unavailable coordination
// store (member registration + lease) or interconnect listener → not_ready
// (HTTP 503, mapped by the Handler); only an explicit allowStandaloneDegrade
// permits degrading to standalone during startup (the degraded state is
// observable in the Cluster component).
func (s *Service) Ready(ctx context.Context, req *ReadyRequest) (*ReadyResponse, error) {
	dbStatus := checkDatabaseHealth(ctx, s.svcCtx)
	registryStatus, _ := collectRegistryStats(s.svcCtx.RegistryStore)
	opsStatus := summarizeOpsState(s.svcCtx)
	clusterReady, clusterReasons, clusterComp := clusterReadinessStatus(s.svcCtx)

	reasons := make([]string, 0)
	if !componentHealthy(dbStatus) {
		reasons = append(reasons, dbReadinessReason(dbStatus))
	}
	if !componentHealthy(registryStatus) {
		reasons = append(reasons, "registry not initialized")
	}
	if !componentHealthy(opsStatus) {
		reasons = append(reasons, "ops state store not initialized")
	}
	reasons = append(reasons, clusterReasons...)

	ready := componentHealthy(dbStatus) && componentHealthy(registryStatus) &&
		componentHealthy(opsStatus) && clusterReady
	status := "ok"
	if !ready {
		status = "not_ready"
	}
	return &ReadyResponse{
		Status:    status,
		Timestamp: utils.FormatTimestamp(time.Now()),
		Reasons:   reasons,
		Cluster:   clusterComp,
	}, nil
}

// dbReadinessReason explains why the database component is unavailable (stable
// phrase plus driver/error detail).
func dbReadinessReason(dbStatus map[string]interface{}) string {
	reason := "database unavailable"
	if driver, _ := dbStatus["driver"].(string); driver != "" {
		reason += " (" + driver + ")"
	}
	if errMsg, _ := dbStatus["error"].(string); errMsg != "" {
		reason += ": " + errMsg
	}
	return reason
}

// Metrics returns system metrics
func (s *Service) Metrics(ctx context.Context, req *MetricsRequest) (*MetricsResponse, error) {
	dbStatus := checkDatabaseHealth(ctx, s.svcCtx)
	registryStatus, snapshots := collectRegistryStats(s.svcCtx.RegistryStore)
	opsStatus := summarizeOpsState(s.svcCtx)

	counts := map[string]interface{}{
		"agentsTotal":         registryStatus["agentsTotal"],
		"agentsHealthy":       registryStatus["agentsHealthy"],
		"functionsRegistered": registryStatus["functionsRegistered"],
		"maintenanceWindows":  opsStatus["maintenanceWindows"],
		"healthChecks":        opsStatus["healthChecks"],
		"alerts":              opsStatus["alerts"],
	}

	return &MetricsResponse{
		Timestamp: utils.FormatTimestamp(time.Now()),
		Counts:    counts,
		Database: MonitoringComponentStatus{
			"ok":        dbStatus["ok"],
			"latencyMs": dbStatus["latencyMs"],
			"driver":    dbStatus["driver"],
		},
		Registry: map[string]interface{}{
			"ok":       registryStatus["ok"],
			"agents":   normalizeAgentSnapshots(snapshots),
			"metadata": registryStatus,
		},
		Ops: map[string]interface{}{
			"ok":            opsStatus["ok"],
			"mqType":        opsStatus["mqType"],
			"mqLengths":     opsStatus["mqLengths"],
			"healthStatus":  opsStatus["healthStatus"],
			"notifications": opsStatus["notifications"],
		},
	}, nil
}

// Status returns detailed system status
func (s *Service) Status(ctx context.Context, req *StatusRequest) (*StatusResponse, error) {
	dbStatus := checkDatabaseHealth(ctx, s.svcCtx)
	registryStatus, snapshots := collectRegistryStats(s.svcCtx.RegistryStore)
	opsStatus := summarizeOpsState(s.svcCtx)

	ok := componentHealthy(dbStatus) && componentHealthy(registryStatus) && componentHealthy(opsStatus)

	return &StatusResponse{
		OK:            ok,
		Timestamp:     utils.FormatTimestamp(time.Now()),
		UptimeSeconds: uptimeSeconds(),
		Database:      dbStatus,
		Registry:      registryStatus,
		Ops:           opsStatus,
		Agents:        normalizeAgentSnapshots(snapshots),
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
	status["latencyMs"] = time.Since(start).Milliseconds()
	if err != nil {
		status["error"] = err.Error()
		return status
	}
	status["ok"] = true
	return status
}

func collectRegistryStats(store *reg.Store) (map[string]interface{}, []map[string]interface{}) {
	stats := map[string]interface{}{
		"ok":                  false,
		"agentsTotal":         0,
		"agentsHealthy":       0,
		"functionsRegistered": 0,
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
		stats["agentsTotal"] = stats["agentsTotal"].(int) + 1
		if time.Until(sess.ExpireAt) > 0 {
			stats["agentsHealthy"] = stats["agentsHealthy"].(int) + 1
		}
		stats["functionsRegistered"] = stats["functionsRegistered"].(int) + utils.CountEnabledFunctions(sess.Functions)

		if snapshot := utils.BuildOpsAgentSnapshot(sess); snapshot != nil {
			snapshots = append(snapshots, normalizeAgentSnapshot(snapshot))
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
	summary["maintenanceWindows"] = len(state.Maintenance.Windows)
	summary["healthChecks"] = len(state.Health.Checks)
	summary["healthStatus"] = len(state.Health.Status)
	summary["notifications"] = len(state.Notifications.Channels)
	summary["alerts"] = len(state.Alerts.Silences)
	summary["mqType"] = state.MQ.Type
	if state.MQ.Lengths != nil {
		summary["mqLengths"] = state.MQ.Lengths
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

// clusterReadinessStatus computes the cluster readiness component state
// (ARCH-004):
//   - ClusterReadiness is nil (cluster not enabled) → no cluster requirement,
//     treated as ready;
//   - joined (member registration succeeded) → ready only when the lease is
//     healthy AND the interconnect listener bound successfully; either one
//     unavailable → 503 ("cluster required" semantics — permitting degrade does
//     not excuse runtime failures);
//   - not joined (coordination store/registration unavailable at startup) →
//     ready only with an explicit allowStandaloneDegrade (a sanctioned
//     standalone degrade, observable in the component state), otherwise 503
//     (cluster required but cannot serve).
func clusterReadinessStatus(svcCtx *svc.ServiceContext) (bool, []string, MonitoringComponentStatus) {
	comp := MonitoringComponentStatus{"ok": false, "required": false}
	if svcCtx == nil {
		return false, []string{"cluster: service context unavailable"}, comp
	}
	if svcCtx.ClusterReadiness == nil {
		comp["ok"] = true
		return true, nil, comp
	}
	comp["required"] = true
	snap := svcCtx.ClusterReadiness.Snapshot()
	if snap.Joined {
		comp["joined"] = true
		comp["leaseHealthy"] = snap.LeaseHealthy
		comp["interconnectUp"] = snap.InterconnectUp
		reasons := make([]string, 0, 2)
		if !snap.LeaseHealthy {
			reason := "cluster: member lease unavailable"
			if snap.LeaseErr != "" {
				reason += ": " + snap.LeaseErr
			}
			reasons = append(reasons, reason)
		}
		if !snap.InterconnectUp {
			reason := "cluster: interconnect listener unavailable"
			if snap.InterconnectErr != "" {
				reason += ": " + snap.InterconnectErr
			}
			reasons = append(reasons, reason)
		}
		ready := snap.LeaseHealthy && snap.InterconnectUp
		comp["ok"] = ready
		return ready, reasons, comp
	}
	comp["joined"] = false
	if snap.Degraded != "" {
		comp["degraded"] = snap.Degraded
	}
	if svcCtx.Config.Cluster.AllowStandaloneDegrade {
		// Standalone degrade explicitly permitted: sanctioned mode, ready
		// (the degrade fact lives in comp).
		comp["ok"] = true
		return true, nil, comp
	}
	reason := "cluster: coordination store unavailable"
	if snap.Degraded != "" {
		reason += ": " + snap.Degraded
	}
	return false, []string{reason}, comp
}

func uptimeSeconds() int64 {
	return int64(time.Since(serverBootTime).Seconds())
}

func normalizeAgentSnapshots(items []map[string]interface{}) []map[string]interface{} {
	out := make([]map[string]interface{}, 0, len(items))
	for _, item := range items {
		out = append(out, normalizeAgentSnapshot(item))
	}
	return out
}

func normalizeAgentSnapshot(item map[string]interface{}) map[string]interface{} {
	if item == nil {
		return nil
	}
	return map[string]interface{}{
		"id":             item["id"],
		"agentId":        firstNonNil(item["agent_id"], item["agentId"], item["id"]),
		"gameId":         firstNonNil(item["game_id"], item["gameId"]),
		"env":            item["env"],
		"type":           item["type"],
		"addr":           item["addr"],
		"rpcAddr":        firstNonNil(item["rpc_addr"], item["rpcAddr"], item["addr"]),
		"ip":             item["ip"],
		"version":        item["version"],
		"region":         item["region"],
		"zone":           item["zone"],
		"labels":         item["labels"],
		"functions":      item["functions"],
		"providers":      normalizeProviders(item["providers"]),
		"providersCount": firstNonNil(item["providers_count"], item["providersCount"]),
		"healthy":        item["healthy"],
		"expiresInSec":   firstNonNil(item["expires_in_sec"], item["expiresInSec"]),
		"lastSeen":       firstNonNil(item["last_seen"], item["lastSeen"]),
		"activeConns":    firstNonNil(item["active_conns"], item["activeConns"]),
		"totalRequests":  firstNonNil(item["total_requests"], item["totalRequests"]),
		"failedRequests": firstNonNil(item["failed_requests"], item["failedRequests"]),
		"errorRate":      firstNonNil(item["error_rate"], item["errorRate"]),
		"avgLatencyMs":   firstNonNil(item["avg_latency_ms"], item["avgLatencyMs"]),
		"qpsLimit":       firstNonNil(item["qps_limit"], item["qpsLimit"]),
		"qps1m":          firstNonNil(item["qps_1m"], item["qps1m"]),
	}
}

func normalizeProviders(value interface{}) []map[string]interface{} {
	items, ok := value.([]map[string]interface{})
	if !ok || len(items) == 0 {
		return nil
	}
	out := make([]map[string]interface{}, 0, len(items))
	for _, item := range items {
		out = append(out, map[string]interface{}{
			"providerId":   firstNonNil(item["provider_id"], item["providerId"]),
			"gameId":       firstNonNil(item["game_id"], item["gameId"]),
			"env":          item["env"],
			"addr":         item["addr"],
			"version":      item["version"],
			"lastSeenUnix": firstNonNil(item["last_seen_unix"], item["lastSeenUnix"]),
			"functionIds":  firstNonNil(item["function_ids"], item["functionIds"]),
			"functions":    item["functions"],
		})
	}
	return out
}

func firstNonNil(values ...interface{}) interface{} {
	for _, value := range values {
		if value != nil {
			return value
		}
	}
	return nil
}
