package jwtutil

import (
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/cuihairu/croupier/internal/config"
)

const devSecret = "croupier-dev-secret"

var (
	globalSecret string
	secretOnce   sync.Once
)

// InitGlobalSecret initializes the global JWT secret.
// This is called during service initialization and must be called before
// any middleware or service that needs to verify JWT tokens.
func InitGlobalSecret(secret string) {
	secretOnce.Do(func() {
		globalSecret = secret
	})
}

// GetGlobalSecret returns the globally initialized JWT secret.
// Returns empty string if InitGlobalSecret was not called.
func GetGlobalSecret() string {
	return globalSecret
}

// ResolveSecret returns the configured JWT secret.
// In development mode, returns a safe fallback if not configured.
// In production mode, fails fast if JWT secret is not configured.
func ResolveSecret(cfg config.Config) (string, error) {
	secret := strings.TrimSpace(cfg.Auth.JWTSecret)
	if secret == "" {
		// Check if running in development mode
		if isDevelopmentMode(cfg) {
			return devSecret, nil
		}
		return "", fmt.Errorf("JWT secret not configured and not in development mode - set CROUPIER_AUTH_JWT_SECRET or auth.jwtSecret in config")
	}
	return secret, nil
}

// isDevelopmentMode checks if the server is running in development mode.
func isDevelopmentMode(cfg config.Config) bool {
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

// DevSecret exposes the fallback secret for other packages that need to compare.
// This should only be used in development mode.
func DevSecret() string {
	return devSecret
}

// ResetGlobalSecretForTesting re-keys the global JWT secret and returns a
// restore function. Production code must never call this: InitGlobalSecret
// is a sync.Once precisely so the key cannot change at runtime. Tests that
// share a process need it because whichever test initializes first wins and
// later InitGlobalSecret calls silently no-op, causing order-dependent
// signature failures.
//
// Usage: defer jwtutil.ResetGlobalSecretForTesting(secret)()
func ResetGlobalSecretForTesting(secret string) (restore func()) {
	prevSecret := globalSecret
	prevInitialized := globalSecret != ""
	secretOnce = sync.Once{}
	globalSecret = secret
	return func() {
		// 不能拷贝 sync.Once；用全新实例并按需回放原值。
		secretOnce = sync.Once{}
		globalSecret = prevSecret
		if !prevInitialized {
			globalSecret = ""
		}
	}
}
