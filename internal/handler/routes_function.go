package handler

import (
	"github.com/cuihairu/croupier/internal/api/function"
	"github.com/cuihairu/croupier/internal/api/functioncall"
	"github.com/cuihairu/croupier/internal/api/game"
	"github.com/cuihairu/croupier/internal/api/player"
	"github.com/cuihairu/croupier/internal/svc"

	"github.com/gin-gonic/gin"
)

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

	// B2：契约变更历史（版本快照流）
	g.GET("/:id/versions", functionHandler.ContractVersions)
	g.GET("/:id/versions/diff", functionHandler.ContractVersionDiff)
	g.GET("/:id/versions/:seq", functionHandler.ContractVersionDetail)

	// 函数级最低 SDK 版本门槛（低于配置值的 provider 注册时该函数不物化）
	g.GET("/:id/version-floor", functionHandler.VersionFloorGet)
	g.PUT("/:id/version-floor", functionHandler.VersionFloorPut)
	g.DELETE("/:id/version-floor", functionHandler.VersionFloorDelete)
	g.GET("/version-floors", functionHandler.VersionFloorsList)
	g.GET("/version-history", functionHandler.VersionHistoryIndex)
	g.POST("/version-floor/batch", functionHandler.VersionFloorBatch)

	// 待处理
	g.GET("/pending", functionHandler.Pending)

	// 批量操作
	g.POST("/batch-update", functionHandler.BatchUpdate)
	g.POST("/batch-copy", functionHandler.BatchCopy)
	g.POST("/batch-delete", functionHandler.BatchDelete)
	g.GET("/warnings", functionHandler.Warnings)
	// #34：警告过滤下拉聚合选项（静态段，与 /warnings/:key 共存）
	g.GET("/warnings/filter-options", functionHandler.WarningFilterOptions)
	g.DELETE("/warnings/:key", functionHandler.WarningDelete)
	g.POST("/warnings/read/:key", functionHandler.WarningMarkRead)
	g.POST("/warnings/read-all", functionHandler.WarningMarkAllRead)
	g.DELETE("/warnings", functionHandler.WarningDeleteAll)
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

	// 图标上传：静态段注册，与 /:id 共存（gin 静态优先匹配）。挂在 /games 下
	// 复用鉴权组；权限 games:write 在 service 层校验（同 Create/Update）。
	g.POST("/icons", gameHandler.UploadIcon)

	// 环境管理
	g.GET("/:id/envs", gameHandler.EnvsList)
	g.POST("/:id/envs", gameHandler.EnvAdd)
	g.PUT("/:id/envs/:envId", gameHandler.EnvUpdate)
	g.DELETE("/:id/envs/:envId", gameHandler.EnvDelete)
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
// 本地上传文件静态服务（file 驱动）
// ============================================================================
