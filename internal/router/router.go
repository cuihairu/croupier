package router

import (
	"github.com/cuihairu/croupier/internal/api/auth"
	"github.com/cuihairu/croupier/internal/api/profile"
	"github.com/cuihairu/croupier/internal/config"
	"github.com/cuihairu/croupier/internal/middleware"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/security/jwtutil"
	permissionservice "github.com/cuihairu/croupier/internal/service/permission"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// RegisterRoutes 注册所有路由
func RegisterRoutes(r *gin.Engine, cfg *config.Config) error {
	// 初始化数据库
	db, err := initDatabase(cfg)
	if err != nil {
		return err
	}

	// 创建 API 路由组
	api := r.Group("/api/v1")

	// 注册公开路由（无需认证）
	registerPublicRoutes(api, db, cfg)

	// 注册需要认证的路由
	registerAuthenticatedRoutes(api, db)

	return nil
}

// registerPublicRoutes 注册公开路由
func registerPublicRoutes(api *gin.RouterGroup, db *gorm.DB, cfg *config.Config) {
	// Auth 模块
	jwtSecret, _ := jwtutil.ResolveSecret(*cfg)
	authHandler := auth.NewHandler(auth.NewService(
		model.NewAdminModel(db),
		permissionservice.NewPermissionService(db),
		jwtSecret,
	))
	authGroup := api.Group("/auth")
	{
		authGroup.POST("/login", authHandler.Login)
		authGroup.POST("/logout", authHandler.Logout)
	}

	// Health check
	api.GET("/monitoring/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})
}

// registerAuthenticatedRoutes 注册需要认证的路由
func registerAuthenticatedRoutes(api *gin.RouterGroup, db *gorm.DB) {
	// 应用认证中间件
	authenticated := api.Group("")
	authenticated.Use(middleware.Auth())

	// Profile 模块
	profileHandler := profile.NewHandler(profile.NewService(
		model.NewAdminModel(db),
		model.NewGameModel(db),
		model.NewRoleModel(db),
	))
	profileGroup := authenticated.Group("/profile")
	{
		profileGroup.GET("", profileHandler.GetProfile)
		profileGroup.GET("/games", profileHandler.GetGames)
		profileGroup.PUT("", profileHandler.UpdateProfile)
		profileGroup.PUT("/password", profileHandler.ChangePassword)
	}

	// TODO: 注册其他模块路由
	// Admin 模块
	// registerAdminRoutes(authenticated, db)

	// Game 模块
	// registerGameRoutes(authenticated, db)

	// Player 模块
	// registerPlayerRoutes(authenticated, db)

	// Function 模块
	// registerFunctionRoutes(authenticated, db)
}

// initDatabase 初始化数据库连接
func initDatabase(cfg *config.Config) (*gorm.DB, error) {
	// TODO: 从配置加载数据库连接
	// 这里复用现有的数据库初始化逻辑
	return nil, nil
}
