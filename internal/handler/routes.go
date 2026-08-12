package handler

import (
	"github.com/cuihairu/croupier/internal/api/admin"
	"github.com/cuihairu/croupier/internal/api/agent"
	"github.com/cuihairu/croupier/internal/api/alert"
	"github.com/cuihairu/croupier/internal/api/analytics"
	"github.com/cuihairu/croupier/internal/api/approval"
	"github.com/cuihairu/croupier/internal/api/assignment"
	"github.com/cuihairu/croupier/internal/api/audit"
	"github.com/cuihairu/croupier/internal/api/auth"
	"github.com/cuihairu/croupier/internal/api/backup"
	"github.com/cuihairu/croupier/internal/api/certificate"
	"github.com/cuihairu/croupier/internal/api/config"
	"github.com/cuihairu/croupier/internal/api/console"
	"github.com/cuihairu/croupier/internal/api/extension"
	"github.com/cuihairu/croupier/internal/api/faq"
	"github.com/cuihairu/croupier/internal/api/feedback"
	"github.com/cuihairu/croupier/internal/api/function"
	"github.com/cuihairu/croupier/internal/api/functioncall"
	"github.com/cuihairu/croupier/internal/api/game"
	"github.com/cuihairu/croupier/internal/api/message"
	"github.com/cuihairu/croupier/internal/api/meta"
	"github.com/cuihairu/croupier/internal/api/monitoring"
	"github.com/cuihairu/croupier/internal/api/node"
	"github.com/cuihairu/croupier/internal/api/openapi"
	"github.com/cuihairu/croupier/internal/api/ops"
	"github.com/cuihairu/croupier/internal/api/page"
	"github.com/cuihairu/croupier/internal/api/permission"
	"github.com/cuihairu/croupier/internal/api/platform"
	"github.com/cuihairu/croupier/internal/api/player"
	"github.com/cuihairu/croupier/internal/api/profile"
	"github.com/cuihairu/croupier/internal/api/provider"
	"github.com/cuihairu/croupier/internal/api/rate_limit"
	apiregistry "github.com/cuihairu/croupier/internal/api/registry"
	"github.com/cuihairu/croupier/internal/api/resource"
	"github.com/cuihairu/croupier/internal/api/resourcecatalog"
	"github.com/cuihairu/croupier/internal/api/role"
	"github.com/cuihairu/croupier/internal/api/routes"
	"github.com/cuihairu/croupier/internal/api/schema"
	"github.com/cuihairu/croupier/internal/api/storage"
	"github.com/cuihairu/croupier/internal/api/task"
	"github.com/cuihairu/croupier/internal/api/terms"
	"github.com/cuihairu/croupier/internal/api/ticket"
	functionapi "github.com/cuihairu/croupier/internal/function/api"
	"github.com/cuihairu/croupier/internal/function/registry"
	"github.com/cuihairu/croupier/internal/security/jwtutil"
	"github.com/cuihairu/croupier/internal/service"
	permissionservice "github.com/cuihairu/croupier/internal/service/permission"
	versioningservice "github.com/cuihairu/croupier/internal/service/versioning"
	"github.com/cuihairu/croupier/internal/svc"

	"github.com/gin-gonic/gin"
	"log/slog"
)

func RegisterHandlers(r *gin.Engine, serverCtx *svc.ServiceContext) {
	v1 := r.Group("/api/v1")

	// 公开路由（无认证）
	registerAuthRoutes(v1.Group("/auth"), serverCtx) // 修复：/api/v1/auth/login
	registerMetaRoutes(v1, serverCtx)
	registerMonitoringPublicRoutes(r, v1.Group("/monitoring"), serverCtx)
	registerRegistryRoutes(v1.Group("/registry"), serverCtx) // 公开访问
	registerOpenAPIReadRoutes(v1, serverCtx)                 // OpenAPI 只保留契约查看公开路由

	// 需要认证的路由（使用 Authority 中间件）
	protected := v1.Group("/")
	protected.Use(serverCtx.Authority)

	// Scope-dependent 路由：需要 X-Game-ID/X-Env 的接口。
	// GameDBMiddleware 仅挂在此组，解析 scope 并注入 context。
	scoped := protected.Group("/")
	scoped.Use(svc.GameDBMiddleware(serverCtx))

	// Scope-independent 路由：不需要 game/env scope。
	{
		registerAdminRoutes(protected.Group("/admin"), serverCtx)
		registerGameRoutes(protected.Group("/games"), serverCtx)
		registerNodeRoutes(protected.Group("/nodes"), serverCtx)
		registerStorageRoutes(protected.Group("/storage"), serverCtx)
		registerAgentRoutes(protected.Group("/agent"), serverCtx)
		registerAlertRoutes(protected.Group("/alerts"), serverCtx)
		registerBackupRoutes(protected.Group("/backups"), serverCtx)
		registerCertificateRoutes(protected.Group("/certificates"), serverCtx)
		registerExtensionRoutes(protected.Group("/extensions"), serverCtx)
		registerAgentExtensionCompatRoutes(protected.Group("/agents"), serverCtx)
		registerFAQRoutes(protected.Group("/faqs"), serverCtx)
		registerMessageRoutes(protected.Group("/messages"), serverCtx)
		registerMonitoringProtectedRoutes(protected.Group("/monitoring"), serverCtx)
		registerPermissionRoutes(protected.Group("/permissions"), serverCtx)
		registerPlatformRoutes(protected.Group("/platforms"), serverCtx)
		registerProfileRoutes(protected.Group("/profile"), serverCtx)
		registerProviderRoutes(protected.Group("/providers"), serverCtx)
		registerRateLimitRoutes(protected.Group("/rate-limits"), serverCtx)
		registerRoleRoutes(protected.Group("/roles"), serverCtx)
		registerSchemaRoutes(protected.Group("/schemas"), serverCtx)
		registerTermsRoutes(protected.Group("/terms"), serverCtx)
		registerTicketRoutes(protected.Group("/tickets"), serverCtx)
		registerRegistryShortcutRoutes(protected, serverCtx)
		registerAuditRoutes(protected, serverCtx)
	}

	// Scope-dependent 路由
	{
		registerConsoleRoutes(scoped.Group("/console"), serverCtx)
		// Proposals are materialized per game/environment. They must use the
		// same resolved scope as registration, pages, and console execution.
		registerProposalRoutes(scoped.Group("/proposals"), serverCtx)
		// Proposals are materialized per game/environment. They must use the
		// same resolved scope as registration, pages, and console execution.
		registerProposalRoutes(scoped.Group("/proposals"), serverCtx)
		registerFunctionRoutes(scoped.Group("/functions"), serverCtx)
		registerFunctionCallRoutes(scoped.Group("/function-calls"), serverCtx)
		registerFunctionMetadataRoutes(scoped.Group("/metadata"), serverCtx)
		registerOpsRoutes(scoped.Group("/ops"), serverCtx)
		registerPageRoutes(scoped.Group("/pages"), serverCtx)
		registerAnalyticsRoutes(scoped.Group("/analytics"), serverCtx)
		registerApprovalRoutes(scoped.Group("/approvals"), serverCtx)
		registerAssignmentRoutes(scoped.Group("/assignments"), serverCtx)
		registerConfigRoutes(scoped.Group("/configs"), serverCtx)
		registerResourceRoutes(scoped.Group("/resources"), serverCtx)
		registerResourceCatalogRoutes(scoped.Group("/resource-catalog"), serverCtx)
		registerVersioningRoutes(scoped.Group("/versioning"), serverCtx)
		registerFeedbackRoutes(scoped.Group("/feedback"), serverCtx)
		registerPlayerRoutes(scoped.Group("/players"), serverCtx)
		registerTaskRoutes(scoped.Group("/tasks"), serverCtx)
		registerOpenAPISourceRoutes(scoped.Group("/openapi"), serverCtx)
<<<<<<< HEAD
		registerAuditRoutes(v1, serverCtx)
=======
>>>>>>> 00f57f914 (feat: add tests for profile and backup helpers)
	}
}

func registerAuthRoutes(g *gin.RouterGroup, ctx *svc.ServiceContext) {
	jwtSecret, err := jwtutil.ResolveSecret(ctx.Config)
	if err != nil {
		// Log error but continue with fallback for compatibility
		slog.Default().Warn("JWT secret resolution warning, using fallback", "error", err)
		// Use fallback for backward compatibility
		if jwtSecret == "" {
			jwtSecret = jwtutil.DevSecret()
		}
	}
	authSvc := auth.NewService(ctx.AdminModel, permissionservice.NewPermissionService(ctx.DB), jwtSecret, ctx.OpsStateStore).
		WithGameModel(ctx.GameModel)
	authHandler := auth.NewHandler(authSvc)
	g.POST("/login", authHandler.Login)
	g.POST("/logout", authHandler.Logout)
	g.POST("/check", ctx.Authority, authHandler.Check)
	g.POST("/check/batch", ctx.Authority, authHandler.BatchCheck)
}

func registerMetaRoutes(g *gin.RouterGroup, ctx *svc.ServiceContext) {
	metaSvc := meta.NewService(ctx)
	metaHandler := meta.NewHandler(metaSvc)
	g.GET("/", metaHandler.Root)
}

func registerMonitoringPublicRoutes(r *gin.Engine, g *gin.RouterGroup, ctx *svc.ServiceContext) {
	monitoringSvc := monitoring.NewService(ctx)
	monitoringHandler := monitoring.NewHandler(monitoringSvc)
	r.GET("/healthz", monitoringHandler.Healthz)
	g.GET("/healthz", monitoringHandler.Healthz)
}

func registerMonitoringProtectedRoutes(g *gin.RouterGroup, ctx *svc.ServiceContext) {
	monitoringSvc := monitoring.NewService(ctx)
	monitoringHandler := monitoring.NewHandler(monitoringSvc)
	g.GET("/metrics", monitoringHandler.Metrics)
	g.GET("/status", monitoringHandler.Status)
}

func registerAdminRoutes(g *gin.RouterGroup, ctx *svc.ServiceContext) {
	adminSvc := admin.NewService(ctx)
	adminHandler := admin.NewHandler(adminSvc)
	g.GET("", adminHandler.List)
	g.GET("/", adminHandler.List)
	g.POST("", adminHandler.Create)
	g.POST("/", adminHandler.Create)
	g.GET("/:id", adminHandler.Get)
	g.PUT("/:id", adminHandler.Update)
	g.DELETE("/:id", adminHandler.Delete)
	g.POST("/:id/password-reset", adminHandler.PasswordReset)
	g.GET("/:id/games", adminHandler.GetGames)
	g.PUT("/:id/games", adminHandler.UpdateGames)
}

func registerExtensionRoutes(g *gin.RouterGroup, ctx *svc.ServiceContext) {
	extensionSvc := extension.NewService(ctx)
	extensionHandler := extension.NewHandler(extensionSvc)
	g.GET("/catalog", extensionHandler.CatalogList)
	g.GET("/catalog/:id", extensionHandler.CatalogDetail)
	g.GET("/catalog/:id/releases", extensionHandler.CatalogReleases)
	g.GET("/installations", extensionHandler.InstallationList)
	g.POST("/install", extensionHandler.Install)
	g.GET("/installations/:id", extensionHandler.InstallationDetail)
	g.GET("/installations/:id/config-schema", extensionHandler.ConfigSchema)
	g.GET("/installations/:id/config", extensionHandler.Config)
	g.GET("/installations/:id/capabilities", extensionHandler.Capabilities)
	g.GET("/installations/:id/pages", extensionHandler.Pages)
	g.PUT("/installations/:id/config", extensionHandler.UpdateConfig)
	g.POST("/installations/:id/test-connection", extensionHandler.TestConnection)
	g.POST("/installations/:id/health-check", extensionHandler.HealthCheck)
	g.POST("/installations/:id/enable", extensionHandler.Enable)
	g.POST("/installations/:id/disable", extensionHandler.Disable)
	g.POST("/installations/:id/upgrade", extensionHandler.Upgrade)
	g.POST("/installations/:id/reconcile", extensionHandler.Reconcile)
	g.DELETE("/installations/:id", extensionHandler.Uninstall)
	g.GET("/installations/:id/events", extensionHandler.Events)
	// Compatibility routes aligned with target API shape:
	// /api/v1/extensions/:id/*
	g.GET("/:id/config-schema", extensionHandler.CompatConfigSchema)
	g.GET("/:id/config", extensionHandler.CompatConfig)
	g.PUT("/:id/config", extensionHandler.CompatUpdateConfig)
	g.POST("/:id/test-connection", extensionHandler.CompatTestConnection)
	g.GET("/:id/capabilities", extensionHandler.CompatCapabilities)
	g.GET("/:id/pages", extensionHandler.CompatPages)
	g.POST("/:id/health-check", extensionHandler.CompatHealthCheck)
	g.POST("/:id/enable", extensionHandler.CompatEnable)
	g.POST("/:id/disable", extensionHandler.CompatDisable)
	g.POST("/:id/upgrade", extensionHandler.CompatUpgrade)
	g.POST("/:id/reconcile", extensionHandler.CompatReconcile)
	g.DELETE("/:id/uninstall", extensionHandler.CompatUninstall)
	g.GET("/:id/events", extensionHandler.CompatEvents)
	g.GET("/agents/:agentId/sync-payload", extensionHandler.AgentSyncPayload)
}

func registerAgentExtensionCompatRoutes(g *gin.RouterGroup, ctx *svc.ServiceContext) {
	extensionSvc := extension.NewService(ctx)
	extensionHandler := extension.NewHandler(extensionSvc)
	g.GET("/:id/extensions", extensionHandler.AgentExtensions)
	g.POST("/:id/extensions/sync", extensionHandler.AgentExtensionsSync)
}

func registerRoutesRoutes(g *gin.RouterGroup, ctx *svc.ServiceContext) {
	routesSvc := routes.NewService()
	routesHandler := routes.NewHandler(routesSvc)
	g.GET("/", routesHandler.GetRoutes)
}

// ============================================================================
// Function 路由注册
// ============================================================================
func registerFunctionRoutes(g *gin.RouterGroup, ctx *svc.ServiceContext) {
	functionSvc := function.NewService(ctx)
	functionHandler := function.NewHandler(functionSvc)

	// 基础 CRUD
	g.GET("", functionHandler.List)
	g.GET("/", functionHandler.List)
	g.GET("/:id", functionHandler.Detail)
	g.DELETE("/:id", functionHandler.Delete)

	// 函数操作
	g.POST("/:id/enable", functionHandler.Enable)
	g.POST("/:id/disable", functionHandler.Disable)
	g.POST("/:id/copy", functionHandler.Copy)
	g.POST("/:id/invoke", functionHandler.Invoke)
	g.POST("/:id/publish", functionHandler.Publish)

	// 函数实例
	g.GET("/:id/instances", functionHandler.Instances)
	g.GET("/instances", functionHandler.InstancesAll)

	// 权限管理
	g.GET("/:id/permissions", functionHandler.Permissions)
	g.PUT("/:id/permissions", functionHandler.PermissionsUpdate)

	// 历史与分析
	g.GET("/:id/history", functionHandler.History)
	g.GET("/:id/analytics", functionHandler.Analytics)

	// 描述符
	g.GET("/descriptors", functionHandler.Descriptors)

	// 待处理
	g.GET("/pending", functionHandler.Pending)

	// 批量操作
	g.POST("/batch-update", functionHandler.BatchUpdate)
	g.POST("/batch-copy", functionHandler.BatchCopy)
	g.POST("/batch-delete", functionHandler.BatchDelete)
	g.GET("/warnings", functionHandler.Warnings)
}

func registerFunctionCallRoutes(g *gin.RouterGroup, ctx *svc.ServiceContext) {
	functionCallSvc := functioncall.NewService(ctx)
	functionCallHandler := functioncall.NewHandler(functionCallSvc)
	g.GET("", functionCallHandler.List)
	g.GET("/", functionCallHandler.List)
	g.GET("/stats", functionCallHandler.Stats)
	g.GET("/:id", functionCallHandler.Detail)
	g.POST("/:id/rerun", functionCallHandler.Rerun)
	g.POST("/:id/cancel", functionCallHandler.Cancel)
}

// ============================================================================
// Game 路由注册
// ============================================================================
func registerGameRoutes(g *gin.RouterGroup, ctx *svc.ServiceContext) {
	gameSvc := game.NewService(ctx)
	gameHandler := game.NewHandler(gameSvc)

	// 基础 CRUD
	g.GET("", gameHandler.List)
	g.GET("/", gameHandler.List)
	g.POST("", gameHandler.Create)
	g.POST("/", gameHandler.Create)
	g.GET("/:id", gameHandler.Detail)
	g.PUT("/:id", gameHandler.Update)
	g.DELETE("/:id", gameHandler.Delete)

	// 环境管理
	g.GET("/:id/envs", gameHandler.EnvsList)
	g.POST("/:id/envs", gameHandler.EnvAdd)
	g.PUT("/:id/envs/:envId", gameHandler.EnvUpdate)
	g.DELETE("/:id/envs/:envId", gameHandler.EnvDelete)
}

// ============================================================================
// Task 路由注册
// ============================================================================
func registerTaskRoutes(g *gin.RouterGroup, ctx *svc.ServiceContext) {
	taskSvc := task.NewService(ctx)
	taskHandler := task.NewHandler(taskSvc)
	g.GET("", taskHandler.List)
	g.GET("/", taskHandler.List)
	g.POST("", taskHandler.Start)
	g.POST("/", taskHandler.Start)
	g.POST("/cancel", taskHandler.CancelByBody)
	g.POST("/:id/cancel", taskHandler.Cancel)
	g.GET("/:id", taskHandler.Detail)
	g.GET("/:id/events", taskHandler.Events)
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
}

// ============================================================================
// Ops 路由注册
// ============================================================================
func registerOpsRoutes(g *gin.RouterGroup, ctx *svc.ServiceContext) {
	opsSvc := ops.NewService(ctx)
	opsHandler := ops.NewHandler(opsSvc)

	// Agent Ops
	g.GET("/agents", opsHandler.AgentsList)
	g.GET("/agents/metrics", opsHandler.AgentMetrics)
	g.GET("/agents/:agentId/system-info", opsHandler.AgentSystemInfo)
	g.GET("/agents/:agentId/processes", opsHandler.AgentProcesses)
	g.POST("/agents/:agentId/processes/:name/restart", opsHandler.AgentProcessRestart)
	g.POST("/agents/:agentId/processes/:name/stop", opsHandler.AgentProcessStop)
	g.POST("/agents/:agentId/processes/:name/start", opsHandler.AgentProcessStart)
	g.POST("/agents/:agentId/exec", opsHandler.AgentExecCommand)

	// 核心 Ops
	g.PUT("/agent-meta", opsHandler.AgentMeta)
	g.GET("/alerts", opsHandler.Alerts)
	g.POST("/alerts/silence", opsHandler.AlertSilence)
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
	g.GET("/metrics", opsHandler.Metrics)
	g.GET("/mq", opsHandler.MQ)
	g.GET("/notifications", opsHandler.NotificationsGet)
	g.PUT("/notifications", opsHandler.NotificationsUpdate)
	g.GET("/services", opsHandler.Services)

	// 节点 Ops
	g.GET("/nodes", opsHandler.Nodes)
	g.GET("/nodes/commands", opsHandler.NodeCommands)
	g.GET("/nodes/:nodeId/meta", opsHandler.NodeMeta)
	g.POST("/nodes/:nodeId/drain", opsHandler.NodeDrain)
	g.POST("/nodes/:nodeId/undrain", opsHandler.NodeUndrain)
	g.POST("/nodes/:nodeId/restart", opsHandler.NodeRestart)

	// Agent Metrics History
	g.GET("/agent/metrics/history", opsHandler.AgentMetricsHistory)
}

// ============================================================================
// Storage 路由注册
// ============================================================================
func registerStorageRoutes(g *gin.RouterGroup, ctx *svc.ServiceContext) {
	storageSvc := storage.NewService(ctx)
	storageHandler := storage.NewHandler(storageSvc)
	g.GET("/signed-url", storageHandler.SignedURL)
	g.GET("/objects", storageHandler.ListObjects)
	g.POST("/objects", storageHandler.UploadObject)
	g.DELETE("/objects", storageHandler.DeleteObject)
	g.POST("/objects/batch-delete", storageHandler.BatchDeleteObjects)
	g.POST("/directories", storageHandler.CreateDirectory)
	g.POST("/directories/rename", storageHandler.RenameDirectory)
}

// ============================================================================
// Registry 路由注册（公开访问，无需认证）
// ============================================================================
func registerRegistryRoutes(g *gin.RouterGroup, ctx *svc.ServiceContext) {
	registrySvc := apiregistry.NewService(ctx)
	registryHandler := apiregistry.NewHandler(registrySvc)
	g.GET("/", registryHandler.GetRegistry)
}

// ============================================================================
// Audit 路由注册（公开访问，在 v1 根路径）
// ============================================================================
func registerAuditRoutes(g *gin.RouterGroup, ctx *svc.ServiceContext) {
	auditSvc := audit.NewService(ctx)
	auditHandler := audit.NewHandler(auditSvc)
	g.GET("/audit", auditHandler.GetAuditLogs)  // 支持 GET（前端兼容）
	g.POST("/audit", auditHandler.GetAuditLogs) // 支持 POST（原接口）
}

// ============================================================================
// OpenAPI 路由注册（公开访问，在 v1 根路径）
// ============================================================================
func registerOpenAPIReadRoutes(g *gin.RouterGroup, ctx *svc.ServiceContext) {
	openapiSvc := openapi.NewService(ctx)
	openapiHandler := openapi.NewHandler(openapiSvc)
	g.GET("/functions/:id/openapi", openapiHandler.GetSpec)
	g.POST("/functions/_openapi-batch", openapiHandler.BatchGetSpec)
	g.GET("/openapi/spec", openapiHandler.GetDocument)
}

func registerOpenAPISourceRoutes(g *gin.RouterGroup, ctx *svc.ServiceContext) {
	openapiSvc := openapi.NewService(ctx)
	openapiHandler := openapi.NewHandler(openapiSvc)
	g.GET("/sources", openapiHandler.ListSources)
	g.POST("/sources", openapiHandler.CreateSource)
	g.GET("/sources/:sourceId", openapiHandler.GetSource)
	g.PUT("/sources/:sourceId", openapiHandler.UpdateSource)
	g.GET("/sources/:sourceId/diagnostics", openapiHandler.SourceDiagnostics)
	g.POST("/sources/:sourceId/bindings", openapiHandler.CreateBinding)
	g.DELETE("/sources/:sourceId/bindings/:bindingId", openapiHandler.DeleteBinding)
}

// ============================================================================
// Agent 路由注册
// ============================================================================
func registerAgentRoutes(g *gin.RouterGroup, ctx *svc.ServiceContext) {
	agentSvc := agent.NewService(ctx)
	agentHandler := agent.NewHandler(agentSvc)
	g.GET("/analytics-filters", agentHandler.GetAnalyticsFilters)
	g.POST("/meta", agentHandler.UpdateMeta)
}

// ============================================================================
// Alert 路由注册
// ============================================================================
func registerAlertRoutes(g *gin.RouterGroup, ctx *svc.ServiceContext) {
	alertSvc := alert.NewService(ctx)
	alertHandler := alert.NewHandler(alertSvc)
	g.GET("", alertHandler.List)
	g.GET("/", alertHandler.List)
	g.POST("/:id/silence", alertHandler.Silence)
	g.GET("/silences", alertHandler.SilencesList)
	g.DELETE("/silences/:id", alertHandler.SilenceDelete)
}

// ============================================================================
// Analytics 路由注册（合并所有 analytics_* 模块）
// ============================================================================
func registerAnalyticsRoutes(g *gin.RouterGroup, ctx *svc.ServiceContext) {
	analyticsSvc := analytics.NewService(ctx)
	analyticsHandler := analytics.NewHandler(analyticsSvc)

	// Overview 模块
	g.GET("/overview", analyticsHandler.Overview)
	g.GET("/realtime", analyticsHandler.Realtime)
	g.GET("/realtime/series", analyticsHandler.RealtimeSeries)
	g.POST("/ingest", analyticsHandler.Ingest)
	g.GET("/filters", analyticsHandler.FiltersGet)
	g.PUT("/filters", analyticsHandler.FiltersUpdate)

	// Behavior 模块 - 使用 /behavior 子路径
	behaviorGroup := g.Group("/behavior")
	{
		behaviorGroup.GET("/", analyticsHandler.Behavior)
		behaviorGroup.GET("/events", analyticsHandler.BehaviorEvents)
		behaviorGroup.GET("/paths", analyticsHandler.BehaviorPaths)
		behaviorGroup.POST("/funnel", analyticsHandler.BehaviorFunnel)
		behaviorGroup.GET("/adoption", analyticsHandler.BehaviorAdoption)
		behaviorGroup.GET("/adoption/breakdown", analyticsHandler.BehaviorAdoptionBreakdown)
	}

	// Payments 模块 - 使用 /payments 子路径
	paymentsGroup := g.Group("/payments")
	{
		paymentsGroup.GET("/", analyticsHandler.Payments)
		paymentsGroup.GET("/summary", analyticsHandler.PaymentsSummary)
		paymentsGroup.GET("/product-trend", analyticsHandler.PaymentsProductTrend)
		paymentsGroup.GET("/transactions", analyticsHandler.PaymentsTransactions)
		paymentsGroup.POST("/ingest", analyticsHandler.PaymentsIngest)
	}

	// Retention 模块 - 使用 /retention 和 /levels 子路径
	g.GET("/retention", analyticsHandler.Retention)
	g.GET("/levels", analyticsHandler.Levels)
	g.GET("/levels/episodes", analyticsHandler.LevelsEpisodes)
	g.GET("/levels/maps", analyticsHandler.LevelsMaps)
}

// ============================================================================
// Approval 路由注册
// ============================================================================
func registerApprovalRoutes(g *gin.RouterGroup, ctx *svc.ServiceContext) {
	approvalSvc := approval.NewService(ctx)
	approvalHandler := approval.NewHandler(approvalSvc)
	g.GET("/", approvalHandler.List)
	g.GET("/:id", approvalHandler.Get)
	g.POST("/:id/approve", approvalHandler.Approve)
	g.POST("/:id/reject", approvalHandler.Reject)
}

// ============================================================================
// Assignment 路由注册
// ============================================================================
func registerAssignmentRoutes(g *gin.RouterGroup, ctx *svc.ServiceContext) {
	assignmentSvc := assignment.NewService(ctx)
	assignmentHandler := assignment.NewHandler(assignmentSvc)
	g.GET("", assignmentHandler.List)
	g.GET("/", assignmentHandler.List)
	g.GET("/history", assignmentHandler.History)
	g.PUT("", assignmentHandler.Update)
	g.PUT("/", assignmentHandler.Update)
}

// ============================================================================
// Backup 路由注册
// ============================================================================
func registerBackupRoutes(g *gin.RouterGroup, ctx *svc.ServiceContext) {
	backupSvc := backup.NewService(ctx)
	backupHandler := backup.NewHandler(backupSvc)
	g.GET("", backupHandler.List)
	g.GET("/", backupHandler.List)
	g.POST("", backupHandler.Create)
	g.POST("/", backupHandler.Create)
	g.DELETE("/:id", backupHandler.Delete)
	g.GET("/:id/download", backupHandler.Download)
}

// ============================================================================
// Certificate 路由注册
// ============================================================================
func registerCertificateRoutes(g *gin.RouterGroup, ctx *svc.ServiceContext) {
	certificateSvc := certificate.NewService(ctx)
	certificateHandler := certificate.NewHandler(certificateSvc)
	g.GET("", certificateHandler.List)
	g.GET("/", certificateHandler.List)
	g.POST("", certificateHandler.Add)
	g.POST("/", certificateHandler.Add)
	g.GET("/:id", certificateHandler.Get)
	g.POST("/:id/check", certificateHandler.Check)
	g.DELETE("/:id", certificateHandler.Delete)
	g.GET("/stats", certificateHandler.Stats)
	g.POST("/alerts", certificateHandler.AddAlert)
	g.GET("/alerts", certificateHandler.AlertsList)
	g.POST("/check-all", certificateHandler.CheckAll)
	g.GET("/domain-info", certificateHandler.DomainInfo)
	g.GET("/expiring", certificateHandler.Expiring)
}

// ============================================================================
// Config 路由注册
// ============================================================================
func registerConfigRoutes(g *gin.RouterGroup, ctx *svc.ServiceContext) {
	configSvc := config.NewService(ctx)
	configHandler := config.NewHandler(configSvc)
	g.GET("", configHandler.List)
	g.GET("/", configHandler.List)
	g.POST("", configHandler.Upsert)
	g.POST("/", configHandler.Upsert)
	g.GET("/version", configHandler.GetVersion)
	g.GET("/versions", configHandler.ListVersions)
	g.GET("/:id", configHandler.Get)
	g.PUT("/:id", configHandler.Save)
	g.POST("/:id/validate", configHandler.Validate)
	g.GET("/:id/versions", configHandler.ListVersionsByID)
	g.GET("/:id/versions/:version", configHandler.GetVersionByID)
}

// ============================================================================
// FAQ 路由注册
// ============================================================================
func registerFAQRoutes(g *gin.RouterGroup, ctx *svc.ServiceContext) {
	faqSvc := faq.NewService(ctx)
	faqHandler := faq.NewHandler(faqSvc)
	g.GET("", faqHandler.List)
	g.GET("/", faqHandler.List)
	g.POST("", faqHandler.Create)
	g.POST("/", faqHandler.Create)
	g.PUT("/:id", faqHandler.Update)
	g.DELETE("/:id", faqHandler.Delete)
	g.GET("/categories", faqHandler.Categories)
}

// ============================================================================
// Feedback 路由注册
// ============================================================================
func registerFeedbackRoutes(g *gin.RouterGroup, ctx *svc.ServiceContext) {
	feedbackSvc := feedback.NewService(ctx)
	feedbackHandler := feedback.NewHandler(feedbackSvc)
	g.GET("", feedbackHandler.List)
	g.GET("/", feedbackHandler.List)
	g.POST("", feedbackHandler.Create)
	g.POST("/", feedbackHandler.Create)
	g.PUT("/:id", feedbackHandler.Update)
	g.DELETE("/:id", feedbackHandler.Delete)
	g.GET("/stats", feedbackHandler.Stats)
}

// ============================================================================
// Message 路由注册
// ============================================================================
func registerMessageRoutes(g *gin.RouterGroup, ctx *svc.ServiceContext) {
	messageSvc := message.NewService(ctx)
	messageHandler := message.NewHandler(messageSvc, ctx.Config.SSE)
	g.GET("", messageHandler.List)
	g.GET("/", messageHandler.List)
	g.POST("", messageHandler.Send)
	g.POST("/", messageHandler.Send)
	g.GET("/:id", messageHandler.Get)
	g.POST("/:id/read", messageHandler.Read)
	g.GET("/unread-count", messageHandler.UnreadCount)
	g.GET("/stream", messageHandler.Stream)
}

func registerPermissionRoutes(g *gin.RouterGroup, ctx *svc.ServiceContext) {
	permissionSvc := permission.NewService(ctx)
	permissionHandler := permission.NewHandler(permissionSvc)
	g.GET("", permissionHandler.List)
	g.GET("/", permissionHandler.List)
	g.GET("/:id", permissionHandler.Detail)
}

// ============================================================================
// Platform 路由注册
// ============================================================================
func registerPlatformRoutes(g *gin.RouterGroup, ctx *svc.ServiceContext) {
	platformSvc := platform.NewService(ctx)
	platformHandler := platform.NewHandler(platformSvc)
	g.POST("/call", platformHandler.Call)
	g.GET("", platformHandler.List)
	g.GET("/", platformHandler.List)
	g.GET("/:platform/methods", platformHandler.Methods)
}

// ============================================================================
// Player 路由注册
// ============================================================================
func registerPlayerRoutes(g *gin.RouterGroup, ctx *svc.ServiceContext) {
	playerSvc := player.NewService(ctx)
	playerHandler := player.NewHandler(playerSvc)
	g.GET("", playerHandler.List)
	g.GET("/", playerHandler.List)
	g.POST("", playerHandler.Create)
	g.POST("/", playerHandler.Create)
	g.GET("/:id", playerHandler.Detail)
	g.PUT("/:id", playerHandler.Update)
	g.DELETE("/:id", playerHandler.Delete)
	g.POST("/:id/balance", playerHandler.Balance)
}

// ============================================================================
// Profile 路由注册
// ============================================================================
func registerProfileRoutes(g *gin.RouterGroup, ctx *svc.ServiceContext) {
	profileSvc := profile.NewService(ctx.AdminModel, ctx.GameModel, ctx.RoleModel, ctx.OpsStateStore)
	profileHandler := profile.NewHandler(profileSvc)
	g.GET("", profileHandler.GetProfile)     // /api/v1/profile
	g.GET("/", profileHandler.GetProfile)    // /api/v1/profile/
	g.PUT("", profileHandler.UpdateProfile)  // /api/v1/profile
	g.PUT("/", profileHandler.UpdateProfile) // /api/v1/profile/
	g.PUT("/password", profileHandler.ChangePassword)
	g.GET("/permissions", profileHandler.GetPermissions)
	g.GET("/games", profileHandler.GetGames)
	g.PATCH("/scope", profileHandler.UpdateScope)
}

// ============================================================================
// Provider 路由注册
// ============================================================================
func registerProviderRoutes(g *gin.RouterGroup, ctx *svc.ServiceContext) {
	providerSvc := provider.NewService(ctx)
	providerHandler := provider.NewHandler(providerSvc)
	g.GET("", providerHandler.List)
	g.GET("/", providerHandler.List)
	g.GET("/capabilities", providerHandler.Capabilities)
	g.GET("/descriptors", providerHandler.Descriptors)
	g.GET("/:id", providerHandler.Get)
	g.GET("/:id/resources", providerHandler.Resources)
	g.DELETE("/:id", providerHandler.Delete)
	g.POST("/:id/reload", providerHandler.Reload)
}

// ============================================================================
// Resource 路由注册
// ============================================================================
func registerResourceRoutes(g *gin.RouterGroup, ctx *svc.ServiceContext) {
	resourceSvc := resource.NewService(ctx)
	resourceHandler := resource.NewHandler(resourceSvc)
	g.GET("", resourceHandler.List)
	g.GET("/", resourceHandler.List)
	g.GET("/:resourceKey", resourceHandler.Detail)
	g.GET("/:resourceKey/operations", resourceHandler.Operations)
}

// ============================================================================
// Resource Catalog 路由注册
// ============================================================================
func registerResourceCatalogRoutes(g *gin.RouterGroup, ctx *svc.ServiceContext) {
	service := resourcecatalog.NewService(ctx.DB, ctx.AuditService)
	handler := resourcecatalog.NewHandler(service)
	g.GET("", handler.List)
	g.GET("/:resourceKey", handler.Detail)
	g.PUT("/:resourceKey/semantics", handler.UpdateSemantics)
	g.GET("/:resourceKey/semantics/versions", handler.ListSemanticVersions)
	g.GET("/:resourceKey/conflicts", handler.ListConflicts)
	g.POST("/:resourceKey/conflicts/:field/resolve", handler.ResolveConflict)
}

// ============================================================================
// Page 路由注册
// ============================================================================
func registerPageRoutes(g *gin.RouterGroup, ctx *svc.ServiceContext) {
	pageSvc := page.NewService(ctx)
	pageHandler := page.NewHandler(pageSvc)
	g.GET("", pageHandler.ListDrafts)
	g.GET("/", pageHandler.ListDrafts)
	g.GET("/:pageKey", pageHandler.GetDraft)
	g.PUT("/:pageKey", pageHandler.SaveDraft)
	g.POST("/:pageKey/validate", pageHandler.Validate)
	g.POST("/:pageKey/preview", pageHandler.Preview)
	g.POST("/:pageKey/publish", pageHandler.Publish)
	g.POST("/:pageKey/unpublish", pageHandler.Unpublish)
	g.GET("/:pageKey/versions", pageHandler.Versions)
	g.GET("/:pageKey/versions/:versionId", pageHandler.VersionDetail)
	g.POST("/:pageKey/rollback", pageHandler.Rollback)
}

func registerProposalRoutes(g *gin.RouterGroup, ctx *svc.ServiceContext) {
	proposalSvc := service.NewProposalService(ctx.DB)
	proposalHandler := service.NewProposalHandler(proposalSvc)
	g.GET("", proposalHandler.ListProposals)
	g.GET("/", proposalHandler.ListProposals)
	g.GET("/inbox", proposalHandler.Inbox)
	g.GET("/:proposalKey", proposalHandler.GetProposal)
	g.POST("/:proposalKey/accept", proposalHandler.AcceptProposal)
	g.POST("/:proposalKey/accept-and-publish", proposalHandler.AcceptAndPublishProposal)
	g.POST("/:proposalKey/reject", proposalHandler.RejectProposal)
}

func registerVersioningRoutes(g *gin.RouterGroup, ctx *svc.ServiceContext) {
	versioningSvc := versioningservice.NewService(ctx.DB)
	versioningHandler := versioningservice.NewHandler(versioningSvc)
	g.GET("/pages/:pageKey/chain", versioningHandler.GetChangeChain)
	g.GET("/pages/:pageKey/diff", versioningHandler.Diff)
	g.POST("/pages/:pageKey/merge", versioningHandler.Merge)
	g.POST("/pages/:pageKey/rollback-draft", versioningHandler.RollbackDraft)
	g.POST("/pages/:pageKey/rollback-publish", versioningHandler.RollbackPublish)
	g.POST("/pages/:pageKey/regenerate", versioningHandler.RegenerateProposal)
	g.POST("/pages/:pageKey/republish", versioningHandler.Republish)
}

// ============================================================================
// Console 路由注册
// ============================================================================
func registerConsoleRoutes(g *gin.RouterGroup, ctx *svc.ServiceContext) {
	consoleSvc := console.NewService(ctx)
	consoleHandler := console.NewHandler(consoleSvc)
	g.GET("/menu", consoleHandler.Menu)
	g.GET("/pages", consoleHandler.Pages)
	g.GET("/pages/:pageKey", consoleHandler.Page)
	g.POST("/pages/:pageKey/bindings/:bindingId/execute", consoleHandler.ExecuteBinding)
}

func registerRoleRoutes(g *gin.RouterGroup, ctx *svc.ServiceContext) {
	roleSvc := role.NewService(ctx)
	roleHandler := role.NewHandler(roleSvc)
	g.GET("", roleHandler.RolesList)
	g.GET("/", roleHandler.RolesList)
	g.POST("", roleHandler.RoleCreate)
	g.POST("/", roleHandler.RoleCreate)
	g.GET("/:id", roleHandler.RoleDetail)
	g.PUT("/:id", roleHandler.RoleUpdate)
	g.DELETE("/:id", roleHandler.RoleDelete)
	g.PUT("/:id/permissions", roleHandler.RoleUpdate)
}

// ============================================================================
// RateLimit 路由注册
// ============================================================================
func registerRateLimitRoutes(g *gin.RouterGroup, ctx *svc.ServiceContext) {
	rateLimitSvc := rate_limit.NewService(ctx)
	rateLimitHandler := rate_limit.NewHandler(rateLimitSvc)
	g.GET("", rateLimitHandler.List)
	g.GET("/", rateLimitHandler.List)
	g.GET("/:id", rateLimitHandler.Get)
	g.PUT("", rateLimitHandler.Upsert)
	g.PUT("/", rateLimitHandler.Upsert)
	g.DELETE("/:id", rateLimitHandler.Delete)
	g.POST("/preview", rateLimitHandler.Preview)
}

// ============================================================================
// Schema 路由注册
// ============================================================================
func registerSchemaRoutes(g *gin.RouterGroup, ctx *svc.ServiceContext) {
	schemaSvc := schema.NewService(ctx)
	schemaHandler := schema.NewHandler(schemaSvc)
	g.GET("", schemaHandler.List)
	g.GET("/", schemaHandler.List)
	g.POST("", schemaHandler.Create)
	g.POST("/", schemaHandler.Create)
	g.GET("/:id", schemaHandler.Get)
	g.PUT("/:id", schemaHandler.Update)
	g.DELETE("/:id", schemaHandler.Delete)
	g.POST("/:id/validate", schemaHandler.Validate)
	g.POST("/raw-validate", schemaHandler.RawValidate)
	g.GET("/:id/ui-config", schemaHandler.GetUIConfig)
	g.PUT("/:id/ui-config", schemaHandler.UpdateUIConfig)
}

// ============================================================================
// Terms 路由注册
// ============================================================================
func registerTermsRoutes(g *gin.RouterGroup, ctx *svc.ServiceContext) {
	termsSvc := terms.NewService(ctx)
	termsHandler := terms.NewHandler(termsSvc)
	g.GET("", termsHandler.List)
	g.GET("/", termsHandler.List)
	g.PUT("", termsHandler.Upsert)
	g.PUT("/", termsHandler.Upsert)
	g.DELETE("", termsHandler.Delete)
	g.DELETE("/", termsHandler.Delete)
}

// ============================================================================
// Ticket 路由注册
// ============================================================================
func registerTicketRoutes(g *gin.RouterGroup, ctx *svc.ServiceContext) {
	ticketSvc := ticket.NewService(ctx)
	ticketHandler := ticket.NewHandler(ticketSvc)
	g.GET("", ticketHandler.List)
	g.GET("/", ticketHandler.List)
	g.POST("", ticketHandler.Create)
	g.POST("/", ticketHandler.Create)
	g.GET("/:id", ticketHandler.Get)
	g.PUT("/:id", ticketHandler.Update)
	g.DELETE("/:id", ticketHandler.Delete)
	g.POST("/:id/transition", ticketHandler.Transition)
	g.GET("/:id/comments", ticketHandler.GetComments)
	g.POST("/:id/comments", ticketHandler.CreateComment)
}

// ============================================================================
// 兼容前端的快捷路由
// ============================================================================
func registerRegistryShortcutRoutes(g *gin.RouterGroup, ctx *svc.ServiceContext) {
	opsSvc := ops.NewService(ctx)
	opsHandler := ops.NewHandler(opsSvc)
	g.GET("/registry/services", opsHandler.Services)
}

// ============================================================================
// Function Metadata 路由注册（基于新的 FunctionMetadata protobuf）
// ============================================================================
func registerFunctionMetadataRoutes(g *gin.RouterGroup, ctx *svc.ServiceContext) {
	// Create a shared registry store for function metadata
	store := registry.NewStore()
	service := functionapi.NewService(store)
	handler := functionapi.NewHandler(service)

	functions := g.Group("/functions")
	{
		functions.GET("", handler.ListFunctions)
		functions.POST("", handler.RegisterFunction)
		functions.GET("/:id", handler.GetFunction)
		functions.PUT("/:id", handler.UpdateFunction)
		functions.DELETE("/:id", handler.DeleteFunction)
		functions.GET("/resources", handler.GetResources)
		functions.GET("/tags", handler.GetTags)
	}
}
