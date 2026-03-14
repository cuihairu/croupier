package jwtutil

import (
	"strings"

	"github.com/cuihairu/croupier/internal/config"
)

const devSecret = "croupier-dev-secret"

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
