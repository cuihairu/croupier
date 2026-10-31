package cluster

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"time"
)

// PeerTokenType 是互联认证令牌的类型声明值。它把互联令牌与管理员/机器
// JWT 区分开：互联令牌只能用于集群实例间握手（MsgServerHelloRequest），
// 不能当作管理面或 Agent 面身份使用。
const PeerTokenType = "cluster-peer"

// 互联令牌错误：区分「该拒绝」的根因，便于后续握手层映射到拒绝响应，
// 而不是把所有失败混成一个 generic error。
var (
	// ErrEmptySecret 表示未配置互联共享密钥。
	ErrEmptySecret = errors.New("cluster: peer token secret is empty")
	// ErrTokenInvalid 表示令牌格式或签名无效（非本集群签发/被篡改）。
	ErrTokenInvalid = errors.New("cluster: invalid peer token")
	// ErrTokenExpired 表示令牌已过有效期。
	ErrTokenExpired = errors.New("cluster: peer token expired")
	// ErrTokenTypeMismatch 表示令牌不是互联类型（如误用了管理员/机器 JWT）。
	ErrTokenTypeMismatch = errors.New("cluster: not a peer token")
	// ErrTokenInstanceMismatch 表示令牌绑定的实例身份与期望身份不一致。
	ErrTokenInstanceMismatch = errors.New("cluster: peer token instance mismatch")
)

// peerTokenHeader 是令牌 header（固定算法/类型）。
type peerTokenHeader struct {
	Alg string `json:"alg"`
	Typ string `json:"typ"`
}

// PeerTokenClaims 是互联认证令牌声明：绑定签发实例身份与任期 epoch。
// owner 侧后续握手层据此做身份校验与 fencing（epoch 不一致 = 僵尸实例）。
type PeerTokenClaims struct {
	TokenType  string `json:"tokenType"`
	InstanceID string `json:"instanceId"`
	Epoch      uint64 `json:"epoch"`
	// ExpiresAt 是 Unix 秒过期时间。0 表示不设过期（不推荐，仅测试）。
	ExpiresAt int64 `json:"exp,omitempty"`
}

// PeerTokenManager 签发/校验互联对端令牌，支持密钥轮换。
//
// 最小实现：HMAC-SHA256 compact 令牌（header.payload.signature，base64url）。
// 轮换语义：Rotate 后新令牌用新密钥签发，旧令牌在上一密钥宽限期内仍可
// 校验（验证先试当前、再试上一密钥），DropPrevious 才彻底作废旧密钥。
// 这满足多实例滚动重启时的「新旧密钥并发有效」窗口。
type PeerTokenManager struct {
	mu       sync.RWMutex
	current  []byte
	previous []byte
}

// NewPeerTokenManager 创建令牌管理器。secret 为空时返回 ErrEmptySecret。
func NewPeerTokenManager(secret string) (*PeerTokenManager, error) {
	if secret == "" {
		return nil, ErrEmptySecret
	}
	return &PeerTokenManager{current: []byte(secret)}, nil
}

// Issue 用当前密钥签发绑定 instanceID/epoch 的互联令牌。
// ttl<=0 取默认 30 分钟；issuedAt 零值取当前 UTC。
func (m *PeerTokenManager) Issue(instanceID string, epoch uint64, ttl time.Duration, issuedAt time.Time) (string, error) {
	if instanceID == "" {
		return "", errors.New("cluster: peer token instance id is required")
	}
	if ttl <= 0 {
		ttl = 30 * time.Minute
	}
	if issuedAt.IsZero() {
		issuedAt = Now().UTC()
	}
	header, err := json.Marshal(peerTokenHeader{Alg: "HS256", Typ: PeerTokenType})
	if err != nil {
		return "", err
	}
	claims, err := json.Marshal(PeerTokenClaims{
		TokenType:  PeerTokenType,
		InstanceID: instanceID,
		Epoch:      epoch,
		ExpiresAt:  issuedAt.Add(ttl).Unix(),
	})
	if err != nil {
		return "", err
	}
	payload := peerTokenB64(header) + "." + peerTokenB64(claims)

	m.mu.RLock()
	secret := append([]byte(nil), m.current...)
	m.mu.RUnlock()
	if len(secret) == 0 {
		return "", ErrEmptySecret
	}
	sig := peerTokenHMAC(secret, []byte(payload))
	return payload + "." + peerTokenB64(sig), nil
}

// Verify 校验令牌并返回声明。expectedInstanceID 非空时做身份绑定校验
// （不匹配 = 拒绝，防令牌被复制到另一实例复用）。签名先试当前密钥、
// 再试上一密钥（轮换宽限窗口内旧令牌仍有效）。
func (m *PeerTokenManager) Verify(token, expectedInstanceID string) (*PeerTokenClaims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, ErrTokenInvalid
	}
	payload := parts[0] + "." + parts[1]
	sig, err := peerTokenB64Decode(parts[2])
	if err != nil {
		return nil, ErrTokenInvalid
	}

	m.mu.RLock()
	current := append([]byte(nil), m.current...)
	previous := append([]byte(nil), m.previous...)
	m.mu.RUnlock()

	if len(current) == 0 || !hmac.Equal(peerTokenHMAC(current, []byte(payload)), sig) {
		if len(previous) == 0 || !hmac.Equal(peerTokenHMAC(previous, []byte(payload)), sig) {
			return nil, ErrTokenInvalid
		}
	}

	claimsB, err := peerTokenB64Decode(parts[1])
	if err != nil {
		return nil, ErrTokenInvalid
	}
	var claims PeerTokenClaims
	if err := json.Unmarshal(claimsB, &claims); err != nil {
		return nil, ErrTokenInvalid
	}
	if claims.TokenType != PeerTokenType {
		return nil, ErrTokenTypeMismatch
	}
	if claims.ExpiresAt > 0 && Now().Unix() > claims.ExpiresAt {
		return nil, ErrTokenExpired
	}
	if expectedInstanceID != "" && claims.InstanceID != expectedInstanceID {
		return nil, ErrTokenInstanceMismatch
	}
	return &claims, nil
}

// Rotate 轮换密钥：当前密钥降级为上一密钥（宽限校验），新密钥成为当前。
// 上一密钥在下次 Rotate 或 DropPrevious 时被替换/清除。
func (m *PeerTokenManager) Rotate(newSecret string) error {
	if newSecret == "" {
		return ErrEmptySecret
	}
	m.mu.Lock()
	m.previous = append([]byte(nil), m.current...)
	m.current = []byte(newSecret)
	m.mu.Unlock()
	return nil
}

// DropPrevious 清除上一密钥，使轮换前签发的旧令牌立即失效。
func (m *PeerTokenManager) DropPrevious() {
	m.mu.Lock()
	m.previous = nil
	m.mu.Unlock()
}

func peerTokenB64(b []byte) string {
	return base64.RawURLEncoding.EncodeToString(b)
}

func peerTokenB64Decode(s string) ([]byte, error) {
	return base64.RawURLEncoding.DecodeString(s)
}

func peerTokenHMAC(secret, payload []byte) []byte {
	h := hmac.New(sha256.New, secret)
	_, _ = h.Write(payload)
	return h.Sum(nil)
}
