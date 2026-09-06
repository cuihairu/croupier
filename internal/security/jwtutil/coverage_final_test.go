package jwtutil

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestParseRejectionPaths pins every reachable Parse error path. The
// remaining `!parsed.Valid` guard in Parse is defensive only: golang-jwt/jwt
// v4 returns a non-nil error for every invalid-token outcome (malformed
// segments, non-HMAC signing method, bad signature, invalid claims), so
// err == nil always implies parsed.Valid == true and the guard cannot be
// reached through any input.
func TestParseRejectionPaths(t *testing.T) {
	const secret = "cov-secret"
	issuedAt := time.Now().Add(-time.Hour)
	validToken, err := Sign(secret, "alice", []string{"admin"}, 1, 1, issuedAt)
	require.NoError(t, err)

	t.Run("valid token parses", func(t *testing.T) {
		claims, err := Parse(validToken, secret)
		require.NoError(t, err)
		assert.Equal(t, "alice", claims.Username)
		assert.Equal(t, 1, claims.TokenVersion)
	})

	t.Run("malformed token", func(t *testing.T) {
		_, err := Parse("not-a-jwt", secret)
		require.Error(t, err)
	})

	t.Run("wrong secret", func(t *testing.T) {
		_, err := Parse(validToken, "other-secret")
		require.Error(t, err)
	})

	t.Run("expired token", func(t *testing.T) {
		old := jwt.NewWithClaims(jwt.SigningMethodHS256, Claims{
			Username: "bob",
			RegisteredClaims: jwt.RegisteredClaims{
				IssuedAt:  jwt.NewNumericDate(issuedAt),
				ExpiresAt: jwt.NewNumericDate(issuedAt.Add(-time.Hour)),
			},
		})
		raw, signErr := old.SignedString([]byte(secret))
		require.NoError(t, signErr)
		_, err := Parse(raw, secret)
		require.Error(t, err)
	})

	t.Run("non-hmac signing method rejected", func(t *testing.T) {
		none := jwt.NewWithClaims(jwt.SigningMethodNone, Claims{Username: "eve"})
		raw, signErr := none.SignedString(jwt.UnsafeAllowNoneSignatureType)
		require.NoError(t, signErr)
		_, err := Parse(raw, secret)
		require.Error(t, err)
	})

	t.Run("empty secret", func(t *testing.T) {
		_, err := Parse(validToken, "")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "jwt secret is empty")
	})
}
