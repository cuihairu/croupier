package handler

import (
	"github.com/cuihairu/croupier/internal/api/admin"
	"github.com/cuihairu/croupier/internal/api/meta"
	"github.com/cuihairu/croupier/internal/api/monitoring"
	"github.com/cuihairu/croupier/internal/svc"

	"github.com/gin-gonic/gin"
)

func registerMetaRoutes(g *gin.RouterGroup, ctx *svc.ServiceContext) {
	metaSvc := meta.NewService(ctx)
	metaHandler := meta.NewHandler(metaSvc)
	// server 启动时 RedirectTrailingSlash=false，只注册 "/" 会让前端
	// 请求的 GET /api/v1（无尾斜杠）404。双路径注册，与 admin 路由同款。
	g.GET("", metaHandler.Root)
	g.GET("/", metaHandler.Root)
}

func registerMonitoringPublicRoutes(r *gin.Engine, g *gin.RouterGroup, ctx *svc.ServiceContext) {
	monitoringSvc := monitoring.NewService(ctx)
	monitoringHandler := monitoring.NewHandler(monitoringSvc)
	r.GET("/healthz", monitoringHandler.Healthz)
	g.GET("/healthz", monitoringHandler.Healthz)
	// 就绪探针（ARCH-004）：独立于存活探针；要求集群时协调库/互联/租约
	// 不可用返回 503（双路径同 /healthz）。
	r.GET("/readyz", monitoringHandler.Ready)
	g.GET("/readyz", monitoringHandler.Ready)
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
