package jwtutil

import (
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

// ResolveSecret returns the configured JWT secret or a safe development fallback.
func ResolveSecret(cfg config.Config) (secret string, fallback bool) {
	secret = strings.TrimSpace(cfg.Auth.JWTSecret)
	if secret == "" {
		return devSecret, true
	}
	return secret, false
}

// DevSecret exposes the fallback secret for other packages that need to compare.
func DevSecret() string {
	return devSecret
}
