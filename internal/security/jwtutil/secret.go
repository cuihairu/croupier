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
	secretMu     sync.RWMutex
	globalSecret string
	secretInited bool
)

// InitGlobalSecret initializes the global JWT secret.
// This is called during service initialization and must be called before
// any middleware or service that needs to verify JWT tokens.
// First caller wins: later calls are no-ops so the key cannot rotate at runtime.
func InitGlobalSecret(secret string) {
	secretMu.Lock()
	defer secretMu.Unlock()
	if !secretInited {
		globalSecret = secret
		secretInited = true
	}
}

// GetGlobalSecret returns the globally initialized JWT secret.
// Returns empty string if InitGlobalSecret was not called.
func GetGlobalSecret() string {
	secretMu.RLock()
	defer secretMu.RUnlock()
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
// restore function. Production code must never call this: the global secret
// is first-init-wins precisely so the key cannot change at runtime. Tests
// that share a process need it because whichever test initializes first wins
// and later InitGlobalSecret calls silently no-op, causing order-dependent
// signature failures.
//
// The reset window is re-armed: InitGlobalSecret takes effect again until
// restore runs. restore() puts back the previous value AND the previous
// initialized state, so first-init-wins holds again afterwards. Leaving the
// window armed past restore would let a later test's InitGlobalSecret
// (e.g. middleware TestInitJWTSecret under -shuffle=on) silently re-key the
// secret and poison every downstream token-signing test.
//
// Usage: defer jwtutil.ResetGlobalSecretForTesting(secret)()
func ResetGlobalSecretForTesting(secret string) (restore func()) {
	secretMu.Lock()
	prevSecret, prevInited := globalSecret, secretInited
	globalSecret, secretInited = secret, false
	secretMu.Unlock()
	return func() {
		secretMu.Lock()
		defer secretMu.Unlock()
		globalSecret, secretInited = prevSecret, prevInited
	}
}
