package jwtutil

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v4"
)

// Claims represents the JWT claims we issue/parse.
type Claims struct {
	Username string   `json:"username"`
	Roles    []string `json:"roles"`
	AdminID  uint     `json:"admin_id"`
	jwt.RegisteredClaims
}

const tokenTTL = 24 * time.Hour

// Sign issues a JWT for the provided user/roles using the shared secret.
func Sign(secret string, username string, roles []string, adminID uint, issuedAt time.Time) (string, error) {
	if secret == "" {
		return "", errors.New("jwt secret is empty")
	}
	if issuedAt.IsZero() {
		issuedAt = time.Now().UTC()
	}
	claims := Claims{
		Username: username,
		Roles:    roles,
		AdminID:  adminID,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   username,
			IssuedAt:  jwt.NewNumericDate(issuedAt),
			ExpiresAt: jwt.NewNumericDate(issuedAt.Add(tokenTTL)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

// Parse validates and parses a JWT returning our structured claims.
func Parse(tokenStr, secret string) (*Claims, error) {
	if secret == "" {
		return nil, errors.New("jwt secret is empty")
	}

	claims := &Claims{}
	parsed, err := jwt.ParseWithClaims(tokenStr, claims, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %T", token.Method)
		}
		return []byte(secret), nil
	})
	if err != nil {
		return nil, err
	}
	if !parsed.Valid {
		return nil, errors.New("invalid token")
	}
	return claims, nil
}
