package token

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

type Manager struct{ secret []byte }

func NewManager(secret string) *Manager { return &Manager{secret: []byte(secret)} }

type claims struct {
	Sub   string   `json:"sub"`
	Roles []string `json:"roles"`
	Exp   int64    `json:"exp"`
}

func b64enc(b []byte) string          { return base64.RawURLEncoding.EncodeToString(b) }
func b64dec(s string) ([]byte, error) { return base64.RawURLEncoding.DecodeString(s) }

func (m *Manager) Sign(username string, roles []string, ttl time.Duration) (string, error) {
	header := map[string]string{"alg": "HS256", "typ": "JWT"}
	h, _ := json.Marshal(header)
	c, _ := json.Marshal(claims{Sub: username, Roles: roles, Exp: time.Now().Add(ttl).Unix()})
	payload := b64enc(h) + "." + b64enc(c)
	mac := hmac.New(sha256.New, m.secret)
	mac.Write([]byte(payload))
	sig := mac.Sum(nil)
	return payload + "." + b64enc(sig), nil
}

func (m *Manager) Verify(tok string) (string, []string, error) {
	parts := strings.Split(tok, ".")
	if len(parts) != 3 {
		return "", nil, errors.New("bad token")
	}
	mac := hmac.New(sha256.New, m.secret)
	mac.Write([]byte(parts[0] + "." + parts[1]))
	sig := mac.Sum(nil)
	got, err := b64dec(parts[2])
	if err != nil {
		return "", nil, err
	}
	if !hmac.Equal(sig, got) {
		return "", nil, errors.New("bad signature")
	}
	cb, err := b64dec(parts[1])
	if err != nil {
		return "", nil, err
	}
	var c claims
	if err := json.Unmarshal(cb, &c); err != nil {
		return "", nil, err
	}
	if c.Exp > 0 && time.Now().Unix() > c.Exp {
		return "", nil, errors.New("expired")
	}
	return c.Sub, c.Roles, nil
}
