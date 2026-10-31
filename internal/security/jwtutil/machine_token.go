package jwtutil

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v4"
)

// MachineTokenType 是受限机器身份的 tokenType 声明值。它把机器 JWT 与
// 管理员 JWT（Claims，无此字段）在同一个 HMAC secret 下区分开：机器 token
// 只能用于注册的机器端点，不能当作管理员身份使用。
const MachineTokenType = "agent-sync"

// MachineTokenHeader 是 Agent 拉取器发送受限机器身份 token 的专用请求头。
// 与管理员 Authorization: Bearer 分流，避免两种 token 类型在同一个 header
// 下互相冒充。
const MachineTokenHeader = "X-Croupier-Agent-Token"

// machineIdentityKey 是请求上下文里机器身份的键。用非导出类型做键避免
// 与字符串键冲突。
type machineIdentityKeyType struct{}

var machineIdentityKey = machineIdentityKeyType{}

// WithMachineIdentity 把机器身份放入请求上下文（中间件认证通过后调用）。
func WithMachineIdentity(ctx context.Context, claims *MachineClaims) context.Context {
	return context.WithValue(ctx, machineIdentityKey, claims)
}

// MachineIdentityFrom 从请求上下文取机器身份；无机器身份（管理员/公开）
// 返回 false。
func MachineIdentityFrom(ctx context.Context) (*MachineClaims, bool) {
	claims, ok := ctx.Value(machineIdentityKey).(*MachineClaims)
	return claims, ok
}

// MachineClaims 是 Agent 受限机器身份的 JWT 声明集：绑定单个 agentID 与
// 其 scope（gameId|env）。签名与管理员 JWT 同一 HMAC secret，但 tokenType
// 判型 + 路径白名单使机器 token 无法用于管理面。
type MachineClaims struct {
	TokenType string `json:"tokenType"`
	AgentID   string `json:"agentId"`
	GameID    string `json:"gameId"`
	Env       string `json:"env"`
	jwt.RegisteredClaims
}

// SignMachine 签发受限机器身份 token。ttl<=0 取默认 30 天；issuedAt 零值
// 取当前 UTC。返回的 token 绑定 agentID 与 scope（gameID|env），仅能访问
// 注册的机器端点。
func SignMachine(secret, agentID, gameID, env string, ttl time.Duration, issuedAt time.Time) (string, error) {
	if secret == "" {
		return "", errors.New("jwt secret is empty")
	}
	if agentID == "" {
		return "", errors.New("agent id is required")
	}
	if ttl <= 0 {
		ttl = 30 * 24 * time.Hour
	}
	if issuedAt.IsZero() {
		issuedAt = time.Now().UTC()
	}
	claims := MachineClaims{
		TokenType: MachineTokenType,
		AgentID:   agentID,
		GameID:    gameID,
		Env:       env,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   agentID,
			IssuedAt:  jwt.NewNumericDate(issuedAt),
			ExpiresAt: jwt.NewNumericDate(issuedAt.Add(ttl)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

// ParseMachine 校验并解析受限机器身份 token。仅接受 HS256 签名且
// tokenType=="agent-sync"（管理员 JWT 无该字段，此处解析失败）。
func ParseMachine(tokenStr, secret string) (*MachineClaims, error) {
	if secret == "" {
		return nil, errors.New("jwt secret is empty")
	}
	claims := &MachineClaims{}
	parsed, err := parseWithClaims(tokenStr, claims, func(token *jwt.Token) (interface{}, error) {
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
	if claims.TokenType != MachineTokenType {
		return nil, errors.New("not a machine token")
	}
	if claims.AgentID == "" {
		return nil, errors.New("machine token missing agent id")
	}
	return claims, nil
}
