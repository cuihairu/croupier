// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package svc

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/cuihairu/croupier/services/server/internal/config"
	"github.com/cuihairu/croupier/services/server/internal/model"
	"github.com/cuihairu/croupier/services/server/internal/runtime"
	"github.com/cuihairu/croupier/services/server/internal/service/permission"
	"gorm.io/gorm"
)

type ServiceContext struct {
	Config            config.Config
	AdminManager      *AdminManager
	DB                *gorm.DB
	PermissionService *permission.PermissionService
	AdminModel        *model.AdminModel
}

func NewServiceContext(c config.Config) *ServiceContext {
	// 初始化数据库连接
	db, err := openDatabase(c)
	if err != nil {
		panic(fmt.Sprintf("Failed to connect to database: %v", err))
	}

	// 自动迁移数据库模型
	if err := autoMigrate(db); err != nil {
		panic(fmt.Sprintf("Failed to auto migrate database: %v", err))
	}

	// 创建服务
	permissionService := permission.NewPermissionService(db)
	adminModel := model.NewAdminModel(db)

	// 创建管理员管理器（基于JSON文件）
	configDir := resolveBootstrapAuthDir(c)
	adminManager := NewAdminManager(configDir)
	if err := adminManager.Initialize(); err != nil {
		// 如果初始化失败，记录错误但不停止服务
		// 这样可以让服务启动，但登录功能可能受限
		// 在生产环境中应该更严格地处理这个错误
	}

	return &ServiceContext{
		Config:            c,
		AdminManager:      adminManager,
		DB:                db,
		PermissionService: permissionService,
		AdminModel:        adminModel,
	}
}

// autoMigrate runs all necessary database migrations
func autoMigrate(db *gorm.DB) error {
	// Import all model packages and run their AutoMigrate functions
	if err := model.AutoMigrate(db); err != nil {
		return fmt.Errorf("failed to migrate server models: %w", err)
	}

	// Add other model migrations here when they are created
	return nil
}

func resolveBootstrapAuthDir(c config.Config) string {
	baseDir := resolveBootstrapBaseDir(c)
	if baseDir == "" {
		return runtime.DefaultBootstrapDataDir()
	}
	return baseDir
}

func resolveBootstrapBaseDir(c config.Config) string {
	if dir := strings.TrimSpace(c.BootstrapData.BaseDir); dir != "" {
		return toAbs(dir)
	}

	if c.Auth.UsersConfig != "" {
		return filepath.Dir(toAbs(c.Auth.UsersConfig))
	}

	return runtime.DefaultBootstrapDataDir()
}

func toAbs(p string) string {
	if p == "" {
		return ""
	}
	if filepath.IsAbs(p) {
		return filepath.Clean(p)
	}
	if abs, err := filepath.Abs(p); err == nil {
		return abs
	}
	return p
}
