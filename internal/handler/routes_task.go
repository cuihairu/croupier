package handler

import (
	"time"

	"github.com/cuihairu/croupier/internal/api/node"
	"github.com/cuihairu/croupier/internal/api/ops"
	"github.com/cuihairu/croupier/internal/api/schedule"
	"github.com/cuihairu/croupier/internal/api/task"
	"github.com/cuihairu/croupier/internal/ops/deploy"
	"github.com/cuihairu/croupier/internal/svc"

	"github.com/gin-gonic/gin"
)

// ============================================================================
// Task 路由注册
// ============================================================================
func registerTaskRoutes(g *gin.RouterGroup, ctx *svc.ServiceContext) {
	taskSvc := task.NewService(ctx)
	taskHandler := task.NewHandler(taskSvc)
	g.GET("", taskHandler.List)
	g.GET("/", taskHandler.List)
	// #23：操作者聚合选项（静态段注册，与 /:id 共存——gin 静态优先匹配）
	g.GET("/operator-options", taskHandler.OperatorOptions)
	g.POST("", taskHandler.Start)
	g.POST("/", taskHandler.Start)
	g.POST("/cancel", taskHandler.CancelByBody)
	g.POST("/:id/cancel", taskHandler.Cancel)
	g.GET("/:id", taskHandler.Detail)
	g.GET("/:id/events", taskHandler.Events)
}

func registerScheduleRoutes(g *gin.RouterGroup, ctx *svc.ServiceContext) {
	scheduleSvc := schedule.NewService(ctx)
	h := schedule.NewHandler(scheduleSvc)
	g.GET("", h.List)
	g.GET("/", h.List)
	g.POST("", h.Create)
	g.POST("/", h.Create)
	g.GET("/:id/runs", h.RunLogs)
	g.PUT("/:id/status", h.SetStatus)
	g.POST("/:id/trigger", h.TriggerNow)
	g.DELETE("/:id", h.Delete)
}

// ============================================================================
// Node 路由注册
// ============================================================================
func registerNodeRoutes(g *gin.RouterGroup, ctx *svc.ServiceContext) {
	nodeSvc := node.NewService(ctx)
	nodeHandler := node.NewHandler(nodeSvc)
	g.GET("", nodeHandler.List)
	g.GET("/", nodeHandler.List)
	g.GET("/:id/meta", nodeHandler.GetMeta)
	g.PUT("/:id/meta", nodeHandler.UpdateMeta)
	g.POST("/:id/drain", nodeHandler.Drain)
	g.POST("/:id/undrain", nodeHandler.Undrain)
	g.POST("/:id/restart", nodeHandler.Restart)
	g.GET("/commands", nodeHandler.Commands)
	g.GET("/cron-jobs", nodeHandler.ListAllCronJobs)
	g.GET("/:id/cron-jobs", nodeHandler.ListCronJobs)
}

// ============================================================================
// Ops 路由注册
// ============================================================================
func registerOpsRoutes(g *gin.RouterGroup, ctx *svc.ServiceContext) {
	opsSvc := ops.NewService(ctx)
	opsHandler := ops.NewHandler(opsSvc)

	// OPS-004 部署发版编排（DB 与 ObjectStore 就绪即装配；nil 时端点 503）。
	if ctx.DB != nil && ctx.ObjectStore != nil {
		agent := ops.NewDeployAgent(ctx)
		ctx.Deployments = deploy.New(ctx.DeploymentModel, agent)
		ctx.Deployments.HealthRetry = 5 * time.Second
		ctx.Deployments.HealthRetries = 3
	}

	// Agent Ops
	g.GET("/agents", opsHandler.AgentsList)
	g.GET("/agents/metrics", opsHandler.AgentMetrics)
	g.GET("/agents/:agentId/system-info", opsHandler.AgentSystemInfo)
	g.GET("/agents/:agentId/supervisor", opsHandler.AgentSupervisor)
	g.GET("/agents/:agentId/supervisor/events", opsHandler.AgentSupervisorEvents)
	g.GET("/agents/synthetics/results", opsHandler.SyntheticsResults)
	g.GET("/agents/:agentId/supervisor/logs", opsHandler.AgentSupervisorLog)
	g.GET("/agents/:agentId/processes", opsHandler.AgentProcesses)
	g.POST("/agents/:agentId/processes/:name/restart", opsHandler.AgentProcessRestart)
	g.POST("/agents/:agentId/processes/:name/stop", opsHandler.AgentProcessStop)
	g.POST("/agents/:agentId/processes/:name/start", opsHandler.AgentProcessStart)
	g.POST("/agents/:agentId/processes/:name/update", opsHandler.AgentProcessUpdate)
	g.POST("/agents/:agentId/processes/:name/schedule/pause", opsHandler.AgentProcessSchedulePause)
	g.POST("/agents/:agentId/processes/:name/schedule/resume", opsHandler.AgentProcessScheduleResume)
	g.POST("/agents/:agentId/exec", opsHandler.AgentExecCommand)

	// 核心 Ops
	g.PUT("/agent-meta", opsHandler.AgentMeta)
	g.GET("/cluster", opsHandler.ClusterInfo)
	g.GET("/alerts", opsHandler.Alerts)
	g.POST("/alerts/silence", opsHandler.AlertSilence)
	// 风险事件工作流流转（确认/忽略/升级 + 备注；handled_by 取认证上下文）
	g.POST("/alerts/:id/handle", opsHandler.OpsAlertHandle)
	g.DELETE("/silences/:id", opsHandler.SilenceDelete)
	g.GET("/silences", opsHandler.Silences)

	// 备份
	g.POST("/backups", opsHandler.BackupCreate)
	g.GET("/backups", opsHandler.BackupsList)
	g.DELETE("/backups/:id", opsHandler.BackupDelete)
	g.GET("/backups/:id/download", opsHandler.BackupDownload)

	// 配置与状态
	g.GET("/config", opsHandler.Config)
	g.GET("/functions", opsHandler.Functions)
	g.GET("/health", opsHandler.HealthGet)
	g.POST("/health/run", opsHandler.HealthRun)
	g.PUT("/health", opsHandler.HealthUpdate)
	g.GET("/maintenance", opsHandler.MaintenanceGet)
	g.PUT("/maintenance", opsHandler.MaintenanceUpdate)

	// 系统维护（OPEN-ISSUES #52）：运行版本/启动时间/在线时长 + 检查更新
	// （只检查不升级，为 #53-57 运维家族留 /ops/system/* 组）
	g.GET("/system/runtime", opsHandler.SystemRuntime)
	g.POST("/system/check-update", opsHandler.SystemCheckUpdate)

	// 性能参数（OPEN-ISSUES #53）：生效值 + 运行时快照；逐键 L3 覆盖
	g.GET("/performance", opsHandler.PerformanceGet)
	g.PUT("/performance", opsHandler.PerformancePut)

	// 日志维护（OPEN-ISSUES #54）：留痕保留期 L3 覆盖 + 按时间手动清理 +
	// 服务器日志文件只读视图
	g.GET("/logs", opsHandler.LogsGet)
	g.PUT("/logs", opsHandler.LogsPut)
	g.POST("/logs/cleanup", opsHandler.LogsCleanupPost)
	// OPS-005 应用日志检索（按机/文件/级别/时间窗，节点详情面板消费）
	g.GET("/logs/search", opsHandler.AgentLogSearch)
	// OPS-006 配置下发与批操作
	g.POST("/batch", opsHandler.BatchOpCreateHandler)
	g.GET("/batch", opsHandler.BatchOpListHandler)
	g.GET("/batch/:id", opsHandler.BatchOpGetHandler)
	g.POST("/batch/:id/execute", opsHandler.BatchOpExecuteHandler)
	// OPS-009 运维 runbook 自动化（参数化步骤序列引用 OPS-004/006 能力）
	g.POST("/runbooks", opsHandler.RunbookCreateHandler)
	g.GET("/runbooks", opsHandler.RunbookListHandler)
	g.GET("/runbooks/:id", opsHandler.RunbookGetHandler)
	g.DELETE("/runbooks/:id", opsHandler.RunbookDeleteHandler)
	g.POST("/runbooks/:id/run", opsHandler.RunbookRunHandler)
	g.GET("/runbook-runs", opsHandler.RunbookRunListHandler)
	g.GET("/runbook-runs/:runId", opsHandler.RunbookRunGetHandler)
	// 第三方服务健康探针（OPEN-ISSUES #57）：webhook 四渠道/更新源/SMTP
	g.POST("/probes/:channel", opsHandler.ThirdPartyProbe)
	g.GET("/metrics", opsHandler.Metrics)
	g.GET("/mq", opsHandler.MQ)
	g.GET("/notifications", opsHandler.NotificationsGet)
	g.PUT("/notifications", opsHandler.NotificationsUpdate)
	g.GET("/services", opsHandler.Services)

	// 节点 Ops
	g.GET("/nodes", opsHandler.Nodes)
	g.POST("/cluster/lb-stats", opsHandler.LBStatsQuery)
	g.GET("/nodes/commands", opsHandler.NodeCommands)
	g.GET("/nodes/:nodeId", opsHandler.NodeDetail)
	g.GET("/nodes/:nodeId/meta", opsHandler.NodeMeta)
	g.POST("/nodes/:nodeId/drain", opsHandler.NodeDrain)
	g.POST("/nodes/:nodeId/undrain", opsHandler.NodeUndrain)
	g.POST("/nodes/:nodeId/restart", opsHandler.NodeRestart)

	// Agent Metrics History
	g.GET("/agent/metrics/history", opsHandler.AgentMetricsHistory)

	// capture 闸规则面板下发（capture-agent 简档 §6/§8.5：config_versions
	// capture.gates，namespace=ops；agent 经 wire 拉取热生效；回滚=历史
	// 版本内容再发一版，版本日志不可变）
	g.GET("/capture/gates", opsHandler.CaptureGatesGet)
	g.GET("/capture/gates/versions", opsHandler.CaptureGatesVersionsGet)
	g.PUT("/capture/gates", opsHandler.CaptureGatesPut)
	g.POST("/capture/gates/rollback", opsHandler.CaptureGatesRollback)
	// dry-run 发布预检：lint + config.ValidateGates（agent apply 同一校验真值）
	g.POST("/capture/gates/validate", opsHandler.CaptureGatesValidate)

	// probe 采样策略面板下发（评审简档 §13.3 C：config_versions probe.policy，
	// namespace=ops；agent 经 wire 拉取 + configsync 热生效；回滚=历史版本
	// 内容再发一版，版本日志不可变）
	g.GET("/probe/policy", opsHandler.ProbePolicyGet)
	g.GET("/probe/policy/versions", opsHandler.ProbePolicyVersionsGet)
	g.PUT("/probe/policy", opsHandler.ProbePolicyPut)
	g.POST("/probe/policy/rollback", opsHandler.ProbePolicyRollback)
	g.POST("/probe/policy/validate", opsHandler.ProbePolicyValidate)

	// devops 会话白名单面板下发（agent-core 简档 §7.6：config_versions
	// devops.allowlist，namespace=ops；agent 经 wire 拉取 + configsync 热换
	// 白名单，文件配置兜底；回滚=历史版本内容再发一版，版本日志不可变）
	g.GET("/devops/allowlist", opsHandler.DevopsAllowlistGet)
	g.GET("/devops/allowlist/versions", opsHandler.DevopsAllowlistVersionsGet)
	g.PUT("/devops/allowlist", opsHandler.DevopsAllowlistPut)
	g.POST("/devops/allowlist/rollback", opsHandler.DevopsAllowlistRollback)
	g.POST("/devops/allowlist/validate", opsHandler.DevopsAllowlistValidate)

	// server 手动诊断探针（评审简档 §4.2/§4.3）：对 server 探针面已配置的
	// 目标即时探一次（与周期探测共用同一探针实例与状态机）
	g.POST("/probe/diagnose", opsHandler.ProbeDiagnoseHandler)

	// 云资源供给引擎（OPS-001）：provision.enabled=false 时各端点如实报
	// 503 provision_not_configured（不假装已开启）。提交/续行不阻塞（后台
	// apply），销毁同步（terraform destroy 原语，无等注册段）。
	g.POST("/provision", opsHandler.ProvisionSubmit)
	g.GET("/provision", opsHandler.ProvisionList)
	g.GET("/provision/:runId", opsHandler.ProvisionGet)
	g.POST("/provision/:runId/resume", opsHandler.ProvisionResume)
	g.POST("/provision/:runId/destroy", opsHandler.ProvisionDestroyResume)
	g.PUT("/provision/:runId/protection", opsHandler.ProvisionProtection)
	g.DELETE("/provision/:runId", opsHandler.ProvisionDestroy)

	// 远程电源管理（OPS 电源管理）：powerops.enabled=false 时各端点如实报
	// 503 powerops_not_configured。提交/续行不阻塞（后台执行+状态确认）；
	// 机器清单挂 nodes.meta.power（PUT power-config 写入，无新表）。
	g.POST("/power", opsHandler.PowerSubmit)
	g.GET("/power", opsHandler.PowerList)
	g.GET("/power/:requestId", opsHandler.PowerGet)
	g.POST("/power/:requestId/resume", opsHandler.PowerResume)
	g.POST("/power/:requestId/reject", opsHandler.PowerReject)
	g.PUT("/nodes/:nodeId/power-config", opsHandler.PowerConfigPut)

	// OPS-007 成本控制：创建前估算 + 节点成本标签/闲置回收（可配）+ 预算告警。
	// 成本目录为内置参考价（生产价需接入真实账单/价格 API，估算精度登记为
	// 部署后对账）；budget 为按 scope 配置，告警走通知渠道。
	g.POST("/cost/estimate", opsHandler.CostEstimateHandler)
	g.GET("/cost/summary", opsHandler.CostSummaryHandler)
	g.GET("/cost/idle", opsHandler.CostIdleHandler)
	g.GET("/cost/budget", opsHandler.CostBudgetGetHandler)
	g.PUT("/cost/budget", opsHandler.CostBudgetPutHandler)

	// OPS-004 运维部署发版：包仓库 + 发版单分批 + 健康门禁 + 回滚。
	// 编排未装配（DB/对象存储缺）时各端点如实报 503 deploy_not_configured。
	g.POST("/deploy/packages", opsHandler.DeployPackageUpload)
	g.GET("/deploy/packages", opsHandler.DeployPackageList)
	g.POST("/deployments", opsHandler.DeploymentCreate)
	g.GET("/deployments", opsHandler.DeploymentList)
	g.GET("/deployments/:id", opsHandler.DeploymentGet)
	g.POST("/deployments/:id/start", opsHandler.DeploymentStart)
	g.POST("/deployments/:id/rollback", opsHandler.DeploymentRollback)

	// probe 在线汇总（评审简档 §13.2）：状态矩阵 + 证据，不出合成分。
	// scope 走 X-Game-ID/X-Env 头（与告警列表同口径）。在线趋势列在 SDK
	// metric 入口接入前为「未接入」一等状态。
	g.GET("/probe/summary", opsHandler.ProbeSummaryHandler)
}
