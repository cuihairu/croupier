package svc

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/cuihairu/croupier/internal/config"
	"github.com/cuihairu/croupier/internal/runtime"
)

// isDevelopmentConfig reports whether the current configuration is in
// development mode.
//
// Fail-closed: any production signal — server.mode or the CROUPIER_MODE /
// CROUPIER_ENV environment variables set to prod/production/release — makes
// this return false (NOT development), so a leftover development setting
// cannot downgrade a production deployment. This is the AUD-008 fix and must
// stay equivalent to jwtutil.isDevelopmentMode (asserted by tests).
func isDevelopmentConfig(cfg config.Config) bool {
	return !isProductionSignal(cfg.Server.Mode) &&
		!isProductionSignal(os.Getenv("CROUPIER_MODE")) &&
		!isProductionSignal(os.Getenv("CROUPIER_ENV"))
}

// isProductionSignal reports whether a mode value is an explicit production
// signal. Matches prod/production/release (case-insensitive, whitespace
// trimmed). Empty or development values are not production signals. Kept in
// sync with the jwtutil package's same-named helper.
func isProductionSignal(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "prod", "production", "release":
		return true
	default:
		return false
	}
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
