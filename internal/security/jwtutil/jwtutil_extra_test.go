package jwtutil

import (
	"sync"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestResetGlobalSecretForTesting tests the ResetGlobalSecretForTesting function
func TestResetGlobalSecretForTesting(t *testing.T) {
	// Save the initial state
	initialSecret := globalSecret

	// Test reset and restore
	t.Run("resets and restores secret", func(t *testing.T) {
		// Reset with a new secret
		restore := ResetGlobalSecretForTesting("new-test-secret")
		assert.Equal(t, "new-test-secret", GetGlobalSecret())

		// Restore the original state
		restore()
		if initialSecret == "" {
			assert.Equal(t, "", GetGlobalSecret())
		} else {
			assert.Equal(t, initialSecret, GetGlobalSecret())
		}
	})

	t.Run("reset allows InitGlobalSecret to work again", func(t *testing.T) {
		// Reset global state
		globalSecret = ""
		secretOnce = sync.Once{}

		// Now InitGlobalSecret should work
		InitGlobalSecret("first-secret")
		assert.Equal(t, "first-secret", GetGlobalSecret())

		// Reset and try again
		restore := ResetGlobalSecretForTesting("second-secret")
		assert.Equal(t, "second-secret", GetGlobalSecret())

		// InitGlobalSecret should work after reset
		InitGlobalSecret("third-secret")
		// sync.Once was reset, so this should NOT take effect
		// Wait - ResetGlobalSecretForTesting resets secretOnce,
		// so InitGlobalSecret WILL work after it
		assert.Equal(t, "third-secret", GetGlobalSecret())

		restore()
	})

	t.Run("restore function restores original state", func(t *testing.T) {
		// Set a known state
		globalSecret = "known-state"
		secretOnce = sync.Once{}

		// Reset
		restore := ResetGlobalSecretForTesting("reset-state")
		assert.Equal(t, "reset-state", GetGlobalSecret())

		// Restore
		restore()
		assert.Equal(t, "known-state", GetGlobalSecret())
	})
}

// TestParseUnexpectedSigningMethod tests Parse with non-HMAC token
func TestParseUnexpectedSigningMethod(t *testing.T) {
	// Create a token with a different signing method (none)
	// We need to craft a token that has a non-HMAC method
	token := jwt.NewWithClaims(jwt.SigningMethodNone, &Claims{
		Username: "testuser",
		Roles:    []string{"admin"},
		AdminID:  123,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "testuser",
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
		},
	})

	tokenStr, err := token.SignedString(jwt.UnsafeAllowNoneSignatureType)
	require.NoError(t, err)

	// Parse should reject the non-HMAC token
	_, err = Parse(tokenStr, "test-secret")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unexpected signing method")
}

// TestParseWithDifferentSigningMethods tests various signing methods
func TestParseWithDifferentSigningMethods(t *testing.T) {
	secret := "test-secret"
	token, err := Sign(secret, "testuser", []string{"admin"}, 123, time.Now())
	require.NoError(t, err)

	t.Run("valid HMAC token", func(t *testing.T) {
		claims, err := Parse(token, secret)
		require.NoError(t, err)
		assert.Equal(t, "testuser", claims.Username)
	})

	t.Run("token signed with different secret", func(t *testing.T) {
		_, err := Parse(token, "wrong-secret")
		assert.Error(t, err)
	})
}

// TestSignAndParseRoundTrip tests complete sign and parse round trip
func TestSignAndParseRoundTrip(t *testing.T) {
	secret := "roundtrip-secret"

	t.Run("multiple users", func(t *testing.T) {
		users := []struct {
			username string
			roles    []string
			adminID  uint
		}{
			{"alice", []string{"admin", "user"}, 1},
			{"bob", []string{"user"}, 2},
			{"charlie", []string{}, 3},
		}

		for _, u := range users {
			token, err := Sign(secret, u.username, u.roles, u.adminID, time.Now())
			require.NoError(t, err)

			claims, err := Parse(token, secret)
			require.NoError(t, err)
			assert.Equal(t, u.username, claims.Username)
			assert.Equal(t, u.roles, claims.Roles)
			assert.Equal(t, u.adminID, claims.AdminID)
		}
	})
}
