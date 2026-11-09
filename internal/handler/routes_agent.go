package handler

import (
	"github.com/cuihairu/croupier/internal/api/agent"
	"github.com/cuihairu/croupier/internal/api/provider"
	"github.com/cuihairu/croupier/internal/svc"

	"github.com/gin-gonic/gin"
)

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
// Provider 路由注册
// ============================================================================
// registerProviderSdkStatsRoutes 单独把 sdk-stats 挂在 scoped 组：
// SDK 分布数据按游戏隔离展示（#38），需要 GameDBMiddleware 解析并鉴权
// X-Game-ID/X-Env 后注入 context；其余 /providers 路由仍与 scope 无关，
// 留在 protected 组。Service/Handler 是 svcCtx 的无状态包装，独立构造
// 一份不与 registerProviderRoutes 的实例冲突。
func registerProviderSdkStatsRoutes(g *gin.RouterGroup, ctx *svc.ServiceContext) {
	providerHandler := provider.NewHandler(provider.NewService(ctx))
	g.GET("/sdk-stats", providerHandler.SdkStats)
	// #2：元数据过滤下拉选项按全局 scope 聚合（#11 EAV 表），同样需要
	// GameDBMiddleware 的 scope 鉴权注入。
	g.GET("/meta-options", providerHandler.MetaOptions)
}

func registerProviderRoutes(g *gin.RouterGroup, ctx *svc.ServiceContext) {
	providerSvc := provider.NewService(ctx)
	providerHandler := provider.NewHandler(providerSvc)
	g.GET("", providerHandler.List)
	g.GET("/", providerHandler.List)
	g.GET("/capabilities", providerHandler.Capabilities)
	g.GET("/descriptors", providerHandler.Descriptors)
	// #38：sdk-stats 需按全局游戏 scope 过滤，已迁至 scoped 组
	// （registerProviderSdkStatsRoutes），此处不再注册。
	g.GET("/:id", providerHandler.Get)
	g.GET("/:id/resources", providerHandler.Resources)
	g.DELETE("/:id", providerHandler.Delete)
	g.POST("/:id/reload", providerHandler.Reload)
}
