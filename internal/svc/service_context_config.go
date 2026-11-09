package svc

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/cuihairu/croupier/internal/config"
	"github.com/cuihairu/croupier/internal/runtime"
)

// isDevelopmentConfig checks if the current configuration is in development mode.
func isDevelopmentConfig(cfg config.Config) bool {
	// Check server mode configuration
	if strings.EqualFold(cfg.Server.Mode, "dev") || strings.EqualFold(cfg.Server.Mode, "development") || strings.EqualFold(cfg.Server.Mode, "debug") {
		return true
	}
	// Check environment variable
	if env := strings.TrimSpace(os.Getenv("CROUPIER_ENV")); env != "" {
		if strings.EqualFold(env, "dev") || strings.EqualFold(env, "development") {
			return true
		}
	}
	// Default to development if not explicitly set to production
	if strings.EqualFold(os.Getenv("CROUPIER_MODE"), "prod") || strings.EqualFold(os.Getenv("CROUPIER_MODE"), "production") {
		return false
	}
	return true
}

func resolveBootstrapAuthDir(c config.Config) string {
	// 空值兜底已删（原 if baseDir == ""）：resolveBootstrapBaseDir 的
	// 三个出口全部非空（BaseDir/UsersConfig 目录/DefaultBootstrapDataDir），
	// baseDir 恒有值。
	return resolveBootstrapBaseDir(c)
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

func resolveTaskRoutingDir(c config.Config) string {
	if dir := strings.TrimSpace(c.AgentDispatch.TaskRoutingDir); dir != "" {
		return toAbs(dir)
	}

	return "data"
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
