package handler

import (
	"github.com/cuihairu/croupier/internal/api/alert"
	"github.com/cuihairu/croupier/internal/api/analytics"
	"github.com/cuihairu/croupier/internal/api/incident"
	"github.com/cuihairu/croupier/internal/svc"

	"github.com/gin-gonic/gin"
)

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
	// 阈值告警规则（指标上报即评估）
	g.GET("/rules", alertHandler.RulesList)
	g.POST("/rules", alertHandler.RulesCreate)
	g.PUT("/rules/:id", alertHandler.RulesUpdate)
	g.DELETE("/rules/:id", alertHandler.RulesDelete)
}

// ============================================================================
// Incident 路由注册（事故报表批 1：类别管理 + 事故登记）
// ============================================================================
func registerIncidentCategoryRoutes(g *gin.RouterGroup, ctx *svc.ServiceContext) {
	incidentSvc := incident.NewService(ctx)
	incidentHandler := incident.NewHandler(incidentSvc)
	g.GET("", incidentHandler.ListCategories)
	g.GET("/", incidentHandler.ListCategories)
	g.POST("", incidentHandler.CreateCategory)
	g.POST("/", incidentHandler.CreateCategory)
	g.PUT("/:id", incidentHandler.UpdateCategory)
	g.GET("/:id/usage", incidentHandler.CategoryUsage)
	g.DELETE("/:id", incidentHandler.DeleteCategory)
}

func registerIncidentRoutes(g *gin.RouterGroup, ctx *svc.ServiceContext) {
	incidentSvc := incident.NewService(ctx)
	incidentHandler := incident.NewHandler(incidentSvc)
	g.GET("", incidentHandler.ListIncidents)
	g.GET("/", incidentHandler.ListIncidents)
	g.POST("", incidentHandler.CreateIncident)
	g.POST("/", incidentHandler.CreateIncident)
	g.GET("/:id", incidentHandler.GetIncident)
	g.PUT("/:id", incidentHandler.UpdateIncident)
	g.POST("/:id/status", incidentHandler.TransitionIncident)
}

// registerIncidentReportRoutes 挂报表聚合 API（汇总/趋势/排行榜/责任人报告/
// 存量报表历史/手动重推）。
func registerIncidentReportRoutes(g *gin.RouterGroup, ctx *svc.ServiceContext) {
	incidentSvc := incident.NewService(ctx)
	incidentHandler := incident.NewHandler(incidentSvc)
	g.GET("/summary", incidentHandler.GetSummary)
	g.GET("/trend", incidentHandler.GetTrend)
	g.GET("/leaderboard", incidentHandler.GetLeaderboard)
	g.GET("/responsibility", incidentHandler.GetResponsibility)
	g.GET("/stored", incidentHandler.ListStoredReports)
	g.POST("/:id/repush", incidentHandler.RepushReport)
}

// ============================================================================
// Analytics 路由注册（合并所有 analytics_* 模块）
// ============================================================================
func registerAnalyticsRoutes(g *gin.RouterGroup, ctx *svc.ServiceContext) {
	analyticsSvc := analytics.NewService(ctx)
	analyticsHandler := analytics.NewHandler(analyticsSvc, ctx.Config.SSE)

	// Overview 模块
	g.GET("/overview", analyticsHandler.Overview)
	// Invocation analytics (audit_records aggregation)
	g.GET("/invocations", analyticsHandler.InvocationsList)
	g.GET("/invocations/trend", analyticsHandler.InvocationsTrend)
	g.GET("/invocations/summary", analyticsHandler.InvocationsSummary)
	// Warehouse (ClickHouse aggregate tables; 503 when DSN unset)
	g.GET("/warehouse/dau", analyticsHandler.WarehouseDAU)
	g.GET("/warehouse/online", analyticsHandler.WarehouseOnline)
	g.GET("/warehouse/revenue", analyticsHandler.WarehouseRevenue)
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

	// Facade 聚合门面（MOB-ANA-001）：移动端 dashboard 一次拉全部域摘要 +
	// 统一查询路由；分域 handler/路由保留不动（加法）。
	g.GET("/facade/summary", analyticsHandler.FacadeSummary)
	g.POST("/facade/query", analyticsHandler.FacadeQuery)
}

// registerAlertInboundRoute 挂告警入站 webhook 端点（第三方告警系统无法
// 携带 JWT；鉴权 = 来源级共享密钥的 X-Croupier-Signature HMAC-SHA256 签名
// + X-Croupier-Timestamp ±5 分钟防重放，校验在 handler——plugin-mechanism
// 设计 §5.3/M3。无配置的来源一律 404/403，非开放端点）。
func registerAlertInboundRoute(g *gin.RouterGroup, ctx *svc.ServiceContext) {
	alertSvc := alert.NewService(ctx)
	alertHandler := alert.NewHandler(alertSvc)
	g.POST("/alerts/inbound/:source", alertHandler.Inbound)
}

// registerExternalIncidentRoutes 挂对外开放 REST（incident-reports §9）：
//   - 公开端点（无 JWT）：POST /ext/incidents、GET /ext/incidents、
//     POST /ext/bugs——鉴权 = external_tokens Bearer token，校验在 handler
//     （第三方系统无法携带 JWT），限流按令牌名分桶；
//   - 令牌管理端点（ops 组，JWT + admin 角色）：/ext/tokens CRUD，
//     明文 token 仅创建时返回一次。
//
// 挂 /ext 命名空间避免与面板 JWT 路由 /incidents 冲突。
func registerExternalIncidentRoutes(v1, opsSoft *gin.RouterGroup, ctx *svc.ServiceContext) {
	extSvc := incident.NewExternalService(ctx)
	extHandler := incident.NewExternalHandler(extSvc, ctx.ExecutionLogWriter)
	v1.POST("/ext/incidents", extHandler.RegisterIncident)
	v1.GET("/ext/incidents", extHandler.ListIncidents)
	v1.POST("/ext/bugs", extHandler.CreateBug)
	tokens := opsSoft.Group("/ext/tokens")
	tokens.Use(incident.RequireAdminRole())
	tokens.POST("", extHandler.CreateToken)
	tokens.GET("", extHandler.ListTokens)
	tokens.PUT("/:id", extHandler.UpdateToken)
	tokens.DELETE("/:id", extHandler.DeleteToken)
}
