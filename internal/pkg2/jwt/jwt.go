package jwt

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"log/slog"
)

// Claims JWT 声明
type Claims struct {
	Username string   `json:"username"`
	Roles    []string `json:"roles"`
	AdminID  uint     `json:"admin_id"`
	jwt.RegisteredClaims
}

var (
	// 默认密钥（应该从配置文件读取）
	defaultSecret = []byte("your-secret-key-change-in-production")
	// Token 有效期
	tokenExpiration = 24 * time.Hour
)

// SetSecret 设置 JWT 密钥
func SetSecret(secret string) {
	if secret != "" {
		defaultSecret = []byte(secret)
		slog.Info("JWT secret set", "length", len(secret))
	}
}

// SetExpiration 设置 Token 有效期
func SetExpiration(duration time.Duration) {
	tokenExpiration = duration
}

// GenerateToken 生成 JWT token
func GenerateToken(username string, roles []string, adminID uint) (string, error) {
	now := time.Now()
	claims := Claims{
		Username: username,
		Roles:    roles,
		AdminID:  adminID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(tokenExpiration)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(defaultSecret)
}

// ParseToken 解析 JWT token
func ParseToken(tokenString string) (*Claims, error) {
	slog.Info("Parsing JWT token", "secret_length", len(defaultSecret))
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return defaultSecret, nil
	})

	if err != nil {
		slog.Error("JWT parse failed", "error", err)
		return nil, err
	}

	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}

	return nil, errors.New("invalid token")
}

// RefreshToken 刷新 token
func RefreshToken(tokenString string) (string, error) {
	claims, err := ParseToken(tokenString)
	if err != nil {
		return "", err
	}

	// 如果 token 还有超过 1 小时才过期，不刷新
	if time.Until(claims.ExpiresAt.Time) > time.Hour {
		return tokenString, nil
	}

	// 生成新 token
	return GenerateToken(claims.Username, claims.Roles, claims.AdminID)
}
