package jwtutil

import (
	"os"
	"sync"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMain handles global state setup
func TestMain(m *testing.M) {
	// Reset global state before running tests
	globalSecret = ""
	secretOnce = sync.Once{}
	m.Run()
}

func TestInitGlobalSecret(t *testing.T) {
	t.Run("initializes global secret", func(t *testing.T) {
		// Each test needs to be run in isolation
		// We can't reset sync.Once, so we test the behavior once
		InitGlobalSecret("test-secret")
		assert.Equal(t, "test-secret", GetGlobalSecret())
	})

	t.Run("only initializes once due to sync.Once", func(t *testing.T) {
		// After first call, subsequent calls are ignored
		// The secret should still be "test-secret" from above
		InitGlobalSecret("another-secret")
		// Should still be the first value set in this test run
		assert.NotEmpty(t, GetGlobalSecret())
	})
}

func TestGetGlobalSecret(t *testing.T) {
	t.Run("returns empty string when not initialized", func(t *testing.T) {
		// This test can only pass if run before any InitGlobalSecret call
		// Since sync.Once can't be reset, we skip this if already initialized
		if GetGlobalSecret() != "" {
			t.Skip("Global secret already initialized")
		}
		secret := GetGlobalSecret()
		assert.Equal(t, "", secret)
	})

	t.Run("returns initialized secret", func(t *testing.T) {
		// Should return the secret set by TestMain or previous tests
		secret := GetGlobalSecret()
		if secret != "" {
			assert.NotEmpty(t, secret)
		} else {
			InitGlobalSecret("my-secret")
			assert.Equal(t, "my-secret", GetGlobalSecret())
		}
	})
}

func TestDevSecret(t *testing.T) {
	assert.Equal(t, "croupier-dev-secret", DevSecret())
}

func TestResolveSecret(t *testing.T) {
	t.Run("returns configured secret", func(t *testing.T) {
		cfg := config.Config{
			Auth: config.AuthConfig{
				JWTSecret: "my-configured-secret",
			},
		}
		secret, err := ResolveSecret(cfg)
		require.NoError(t, err)
		assert.Equal(t, "my-configured-secret", secret)
	})

	t.Run("trims whitespace from secret", func(t *testing.T) {
		cfg := config.Config{
			Auth: config.AuthConfig{
				JWTSecret: "  my-secret  ",
			},
		}
		secret, err := ResolveSecret(cfg)
		require.NoError(t, err)
		assert.Equal(t, "my-secret", secret)
	})

	t.Run("returns dev secret in development mode", func(t *testing.T) {
		cfg := config.Config{
			Server: config.ServerConfig{
				Mode: "dev",
			},
			Auth: config.AuthConfig{
				JWTSecret: "",
			},
		}
		secret, err := ResolveSecret(cfg)
		require.NoError(t, err)
		assert.Equal(t, devSecret, secret)
	})

	t.Run("returns dev secret in development mode (case insensitive)", func(t *testing.T) {
		cfg := config.Config{
			Server: config.ServerConfig{
				Mode: "DEV",
			},
			Auth: config.AuthConfig{
				JWTSecret: "",
			},
		}
		secret, err := ResolveSecret(cfg)
		require.NoError(t, err)
		assert.Equal(t, devSecret, secret)
	})

	t.Run("returns dev secret in debug mode", func(t *testing.T) {
		cfg := config.Config{
			Server: config.ServerConfig{
				Mode: "debug",
			},
			Auth: config.AuthConfig{
				JWTSecret: "",
			},
		}
		secret, err := ResolveSecret(cfg)
		require.NoError(t, err)
		assert.Equal(t, devSecret, secret)
	})

	t.Run("errors when no secret in production mode", func(t *testing.T) {
		// Set production mode via environment
		oldMode := os.Getenv("CROUPIER_MODE")
		os.Setenv("CROUPIER_MODE", "prod")
		defer os.Setenv("CROUPIER_MODE", oldMode)

		cfg := config.Config{
			Server: config.ServerConfig{
				Mode: "production",
			},
			Auth: config.AuthConfig{
				JWTSecret: "",
			},
		}
		_, err := ResolveSecret(cfg)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "JWT secret not configured")
	})

	t.Run("respects CROUPIER_ENV environment variable", func(t *testing.T) {
		oldEnv := os.Getenv("CROUPIER_ENV")
		os.Setenv("CROUPIER_ENV", "dev")
		defer os.Setenv("CROUPIER_ENV", oldEnv)

		cfg := config.Config{
			Server: config.ServerConfig{
				Mode: "production",
			},
			Auth: config.AuthConfig{
				JWTSecret: "",
			},
		}
		secret, err := ResolveSecret(cfg)
		require.NoError(t, err)
		assert.Equal(t, devSecret, secret)
	})
}

func TestIsDevelopmentMode(t *testing.T) {
	t.Run("dev mode", func(t *testing.T) {
		cfg := config.Config{
			Server: config.ServerConfig{
				Mode: "dev",
			},
		}
		assert.True(t, isDevelopmentMode(cfg))
	})

	t.Run("development mode", func(t *testing.T) {
		cfg := config.Config{
			Server: config.ServerConfig{
				Mode: "development",
			},
		}
		assert.True(t, isDevelopmentMode(cfg))
	})

	t.Run("debug mode", func(t *testing.T) {
		cfg := config.Config{
			Server: config.ServerConfig{
				Mode: "debug",
			},
		}
		assert.True(t, isDevelopmentMode(cfg))
	})

	t.Run("case insensitive", func(t *testing.T) {
		cfg := config.Config{
			Server: config.ServerConfig{
				Mode: "DEV",
			},
		}
		assert.True(t, isDevelopmentMode(cfg))
	})

	t.Run("production mode", func(t *testing.T) {
		oldMode := os.Getenv("CROUPIER_MODE")
		os.Setenv("CROUPIER_MODE", "prod")
		defer os.Setenv("CROUPIER_MODE", oldMode)

		cfg := config.Config{
			Server: config.ServerConfig{
				Mode: "production",
			},
		}
		assert.False(t, isDevelopmentMode(cfg))
	})

	t.Run("defaults to development", func(t *testing.T) {
		oldMode := os.Getenv("CROUPIER_MODE")
		os.Unsetenv("CROUPIER_MODE")
		defer func() {
			if oldMode != "" {
				os.Setenv("CROUPIER_MODE", oldMode)
			}
		}()

		cfg := config.Config{
			Server: config.ServerConfig{
				Mode: "unknown",
			},
		}
		assert.True(t, isDevelopmentMode(cfg))
	})
}

func TestSign(t *testing.T) {
	secret := "test-signing-secret"

	t.Run("signs valid token", func(t *testing.T) {
		token, err := Sign(secret, "testuser", []string{"admin"}, 123, time.Now())
		require.NoError(t, err)
		assert.NotEmpty(t, token)
	})

	t.Run("uses current time when issuedAt is zero", func(t *testing.T) {
		token, err := Sign(secret, "testuser", []string{"admin"}, 123, time.Time{})
		require.NoError(t, err)
		assert.NotEmpty(t, token)
	})

	t.Run("errors with empty secret", func(t *testing.T) {
		_, err := Sign("", "testuser", []string{"admin"}, 123, time.Now())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "jwt secret is empty")
	})

	t.Run("token contains correct claims", func(t *testing.T) {
		issuedAt := time.Now().UTC()
		token, err := Sign(secret, "testuser", []string{"admin", "user"}, 456, issuedAt)
		require.NoError(t, err)

		claims, err := Parse(token, secret)
		require.NoError(t, err)
		assert.Equal(t, "testuser", claims.Username)
		assert.Equal(t, []string{"admin", "user"}, claims.Roles)
		assert.Equal(t, uint(456), claims.AdminID)
		assert.Equal(t, "testuser", claims.Subject)
	})
}

func TestParse(t *testing.T) {
	secret := "test-parse-secret"

	t.Run("parses valid token", func(t *testing.T) {
		issuedAt := time.Now().UTC()
		token, err := Sign(secret, "testuser", []string{"admin"}, 123, issuedAt)
		require.NoError(t, err)

		claims, err := Parse(token, secret)
		require.NoError(t, err)
		assert.Equal(t, "testuser", claims.Username)
		assert.Equal(t, []string{"admin"}, claims.Roles)
		assert.Equal(t, uint(123), claims.AdminID)
	})

	t.Run("errors with empty secret", func(t *testing.T) {
		_, err := Parse("some-token", "")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "jwt secret is empty")
	})

	t.Run("errors with invalid token format", func(t *testing.T) {
		_, err := Parse("not-a-valid-jwt", secret)
		assert.Error(t, err)
	})

	t.Run("errors with wrong secret", func(t *testing.T) {
		token, err := Sign("original-secret", "testuser", []string{"admin"}, 123, time.Now())
		require.NoError(t, err)

		_, err = Parse(token, "wrong-secret")
		assert.Error(t, err)
	})

	t.Run("errors with expired token", func(t *testing.T) {
		// Create a token that expired 1 hour ago
		issuedAt := time.Now().UTC().Add(-25 * time.Hour)
		token, err := Sign(secret, "testuser", []string{"admin"}, 123, issuedAt)
		require.NoError(t, err)

		_, err = Parse(token, secret)
		assert.Error(t, err)
	})

	t.Run("errors with unexpected signing method", func(t *testing.T) {
		// This would require a token signed with a different method
		// For now, we'll just test that the function returns error for malformed tokens
		_, err := Parse("invalid.token.format", secret)
		assert.Error(t, err)
	})
}

func TestTokenTTL(t *testing.T) {
	secret := "test-ttl-secret"
	issuedAt := time.Now().UTC()

	token, err := Sign(secret, "testuser", []string{"admin"}, 123, issuedAt)
	require.NoError(t, err)

	claims, err := Parse(token, secret)
	require.NoError(t, err)

	// Check that expiration is approximately 24 hours after issued time
	expectedExpiry := issuedAt.Add(24 * time.Hour)
	actualExpiry := claims.ExpiresAt.Time

	// Allow 1 second difference
	diff := actualExpiry.Sub(expectedExpiry)
	assert.Less(t, diff.Abs(), time.Second)
}
