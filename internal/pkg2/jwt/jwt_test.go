package jwt

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v4"
)

func TestGenerateToken(t *testing.T) {
	// Save original values
	originalSecret := defaultSecret
	originalExpiration := tokenExpiration
	defer func() {
		defaultSecret = originalSecret
		tokenExpiration = originalExpiration
	}()

	// Set test values
	defaultSecret = []byte("test-secret-key")
	tokenExpiration = 24 * time.Hour

	tests := []struct {
		name     string
		username string
		roles    []string
		adminID  uint
		wantErr  bool
	}{
		{
			name:     "valid token generation",
			username: "admin",
			roles:    []string{"admin", "super_admin"},
			adminID:  1,
			wantErr:  false,
		},
		{
			name:     "token with single role",
			username: "user",
			roles:    []string{"user"},
			adminID:  2,
			wantErr:  false,
		},
		{
			name:     "token with no roles",
			username: "guest",
			roles:    []string{},
			adminID:  3,
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token, err := GenerateToken(tt.username, tt.roles, tt.adminID)
			if (err != nil) != tt.wantErr {
				t.Errorf("GenerateToken() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && token == "" {
				t.Errorf("GenerateToken() returned empty token")
			}
		})
	}
}

func TestParseToken(t *testing.T) {
	// Save original values
	originalSecret := defaultSecret
	originalExpiration := tokenExpiration
	defer func() {
		defaultSecret = originalSecret
		tokenExpiration = originalExpiration
	}()

	// Set test values
	defaultSecret = []byte("test-secret-key")
	tokenExpiration = 24 * time.Hour

	// Generate a valid token for testing
	validToken, err := GenerateToken("admin", []string{"admin"}, 1)
	if err != nil {
		t.Fatalf("Failed to generate test token: %v", err)
	}

	tests := []struct {
		name       string
		token      string
		wantErr    bool
		wantUsername string
		wantAdminID uint
	}{
		{
			name:         "valid token",
			token:        validToken,
			wantErr:      false,
			wantUsername: "admin",
			wantAdminID:  1,
		},
		{
			name:    "empty token",
			token:   "",
			wantErr: true,
		},
		{
			name:    "invalid token format",
			token:   "invalid.token.format",
			wantErr: true,
		},
		{
			name:    "malformed token",
			token:   "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.invalid",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			claims, err := ParseToken(tt.token)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseToken() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if claims.Username != tt.wantUsername {
					t.Errorf("ParseToken() username = %v, want %v", claims.Username, tt.wantUsername)
				}
				if claims.AdminID != tt.wantAdminID {
					t.Errorf("ParseToken() adminID = %v, want %v", claims.AdminID, tt.wantAdminID)
				}
			}
		})
	}
}

func TestParseTokenWithWrongSecret(t *testing.T) {
	// Save original values
	originalSecret := defaultSecret
	originalExpiration := tokenExpiration
	defer func() {
		defaultSecret = originalSecret
		tokenExpiration = originalExpiration
	}()

	// Generate token with one secret
	defaultSecret = []byte("secret-one")
	tokenExpiration = 24 * time.Hour
	validToken, err := GenerateToken("admin", []string{"admin"}, 1)
	if err != nil {
		t.Fatalf("Failed to generate test token: %v", err)
	}

	// Change secret and try to parse
	defaultSecret = []byte("secret-two")
	_, err = ParseToken(validToken)
	if err == nil {
		t.Errorf("ParseToken() expected error with wrong secret, got nil")
	}
}

func TestParseTokenExpired(t *testing.T) {
	// Save original values
	originalSecret := defaultSecret
	originalExpiration := tokenExpiration
	defer func() {
		defaultSecret = originalSecret
		tokenExpiration = originalExpiration
	}()

	// Set test values with very short expiration
	defaultSecret = []byte("test-secret-key")
	tokenExpiration = 1 * time.Millisecond

	// Generate token that will expire immediately
	validToken, err := GenerateToken("admin", []string{"admin"}, 1)
	if err != nil {
		t.Fatalf("Failed to generate test token: %v", err)
	}

	// Wait for token to expire
	time.Sleep(10 * time.Millisecond)

	// Try to parse expired token
	_, err = ParseToken(validToken)
	if err == nil {
		t.Errorf("ParseToken() expected error for expired token, got nil")
	}
}

func TestRefreshToken(t *testing.T) {
	// Save original values
	originalSecret := defaultSecret
	originalExpiration := tokenExpiration
	defer func() {
		defaultSecret = originalSecret
		tokenExpiration = originalExpiration
	}()

	// Set test values
	defaultSecret = []byte("test-secret-key")
	tokenExpiration = 24 * time.Hour

	t.Run("refresh token that needs refresh (expires soon)", func(t *testing.T) {
		tokenExpiration = 30 * time.Minute
		validToken, err := GenerateToken("admin", []string{"admin"}, 1)
		if err != nil {
			t.Fatalf("Failed to generate test token: %v", err)
		}

		refreshedToken, err := RefreshToken(validToken)
		if err != nil {
			t.Errorf("RefreshToken() error = %v", err)
			return
		}

		// Verify the refreshed token is valid
		claims, err := ParseToken(refreshedToken)
		if err != nil {
			t.Errorf("ParseToken() on refreshed token error = %v", err)
			return
		}

		if claims.Username != "admin" {
			t.Errorf("RefreshToken() username = %v, want admin", claims.Username)
		}
		if claims.AdminID != 1 {
			t.Errorf("RefreshToken() adminID = %v, want 1", claims.AdminID)
		}
	})

	t.Run("refresh token that is still valid (not near expiry)", func(t *testing.T) {
		tokenExpiration = 24 * time.Hour
		validToken, err := GenerateToken("admin", []string{"admin"}, 1)
		if err != nil {
			t.Fatalf("Failed to generate test token: %v", err)
		}

		// Refresh should return same token since it's still valid
		refreshedToken, err := RefreshToken(validToken)
		if err != nil {
			t.Errorf("RefreshToken() error = %v", err)
			return
		}

		if refreshedToken != validToken {
			t.Errorf("RefreshToken() should return same token when valid, got different token")
		}
	})

	t.Run("refresh invalid token", func(t *testing.T) {
		tokenExpiration = 24 * time.Hour
		invalidToken := "invalid.token.format"

		_, err := RefreshToken(invalidToken)
		if err == nil {
			t.Errorf("RefreshToken() expected error for invalid token, got nil")
		}
	})
}

func TestSetSecret(t *testing.T) {
	// Save original values
	originalSecret := defaultSecret
	defer func() {
		defaultSecret = originalSecret
	}()

	newSecret := "new-test-secret"
	SetSecret(newSecret)

	if string(defaultSecret) != newSecret {
		t.Errorf("SetSecret() defaultSecret = %v, want %v", string(defaultSecret), newSecret)
	}

	// Verify token generated with new secret can be parsed
	token, err := GenerateToken("admin", []string{"admin"}, 1)
	if err != nil {
		t.Errorf("GenerateToken() with new secret error = %v", err)
		return
	}

	_, err = ParseToken(token)
	if err != nil {
		t.Errorf("ParseToken() with new secret error = %v", err)
	}
}

func TestSetExpiration(t *testing.T) {
	// Save original values
	originalExpiration := tokenExpiration
	defer func() {
		tokenExpiration = originalExpiration
	}()

	newExpiration := 12 * time.Hour
	SetExpiration(newExpiration)

	if tokenExpiration != newExpiration {
		t.Errorf("SetExpiration() tokenExpiration = %v, want %v", tokenExpiration, newExpiration)
	}
}

func TestTokenClaimsFields(t *testing.T) {
	// Save original values
	originalSecret := defaultSecret
	originalExpiration := tokenExpiration
	defer func() {
		defaultSecret = originalSecret
		tokenExpiration = originalExpiration
	}()

	defaultSecret = []byte("test-secret-key")
	tokenExpiration = 24 * time.Hour

	username := "testuser"
	roles := []string{"admin", "editor", "viewer"}
	adminID := uint(42)

	token, err := GenerateToken(username, roles, adminID)
	if err != nil {
		t.Fatalf("GenerateToken() error = %v", err)
	}

	// Parse the token
	parsedToken, err := jwt.ParseWithClaims(token, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		return defaultSecret, nil
	})
	if err != nil {
		t.Fatalf("jwt.ParseWithClaims() error = %v", err)
	}

	claims, ok := parsedToken.Claims.(*Claims)
	if !ok {
		t.Fatalf("Could not parse claims")
	}

	// Verify all fields
	if claims.Username != username {
		t.Errorf("Claims.Username = %v, want %v", claims.Username, username)
	}

	if len(claims.Roles) != len(roles) {
		t.Errorf("Claims.Roles length = %v, want %v", len(claims.Roles), len(roles))
	}

	for i, role := range roles {
		if claims.Roles[i] != role {
			t.Errorf("Claims.Roles[%d] = %v, want %v", i, claims.Roles[i], role)
		}
	}

	if claims.AdminID != adminID {
		t.Errorf("Claims.AdminID = %v, want %v", claims.AdminID, adminID)
	}

	// Verify standard claims
	if claims.ExpiresAt == nil {
		t.Errorf("Claims.ExpiresAt should not be nil")
	}

	if claims.IssuedAt == nil {
		t.Errorf("Claims.IssuedAt should not be nil")
	}

	if claims.NotBefore == nil {
		t.Errorf("Claims.NotBefore should not be nil")
	}

	// Verify expiration is approximately 24 hours from now
	expectedExpiry := time.Now().Add(24 * time.Hour)
	diff := claims.ExpiresAt.Time.Sub(expectedExpiry)
	if diff < -time.Minute || diff > time.Minute {
		t.Errorf("Claims.ExpiresAt.Time = %v, want approximately %v", claims.ExpiresAt.Time, expectedExpiry)
	}
}
