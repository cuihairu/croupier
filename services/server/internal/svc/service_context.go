// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package svc

import (
	"path/filepath"
	"strings"

	"github.com/cuihairu/croupier/services/server/internal/config"
	"github.com/cuihairu/croupier/services/server/internal/runtime"
)

type ServiceContext struct {
	Config       config.Config
	AdminManager *AdminManager
}

func NewServiceContext(c config.Config) *ServiceContext {
	// 创建管理员管理器
	configDir := resolveBootstrapAuthDir(c)

	adminManager := NewAdminManager(configDir)
	if err := adminManager.Initialize(); err != nil {
		// 如果初始化失败，记录错误但不停止服务
		// 这样可以让服务启动，但登录功能可能受限
		// 在生产环境中应该更严格地处理这个错误
	}

	return &ServiceContext{
		Config:       c,
		AdminManager: adminManager,
	}
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
