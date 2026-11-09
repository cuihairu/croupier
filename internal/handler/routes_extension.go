package handler

import (
	"github.com/cuihairu/croupier/internal/api/extension"
	"github.com/cuihairu/croupier/internal/api/routes"
	"github.com/cuihairu/croupier/internal/svc"

	"github.com/gin-gonic/gin"
)

func registerExtensionRoutes(g *gin.RouterGroup, ctx *svc.ServiceContext) {
	extensionSvc := extension.NewService(ctx)
	extensionHandler := extension.NewHandler(extensionSvc)
	g.GET("/catalog", extensionHandler.CatalogList)
	g.GET("/catalog/:id", extensionHandler.CatalogDetail)
	g.GET("/catalog/:id/releases", extensionHandler.CatalogReleases)
	// catalog 写路径（#46 批次 3）：登记 / 更新（含上下架）/ 移除 / 发布版本
	g.POST("/catalog", extensionHandler.CatalogCreate)
	g.PUT("/catalog/:id", extensionHandler.CatalogUpdate)
	g.DELETE("/catalog/:id", extensionHandler.CatalogDelete)
	g.POST("/catalog/:id/releases", extensionHandler.CatalogReleasePublish)
	// pack(.tgz) 导入自动登记（#46 批次 6）：解 manifest.json → 登记 + 发布版本 + 工件入库
	g.POST("/packs/import", extensionHandler.PackImport)
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
	g.GET("/agents/:agentId/sync-payload", extensionHandler.AgentSyncPayload)
}

func registerAgentExtensionRoutes(g *gin.RouterGroup, ctx *svc.ServiceContext) {
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
