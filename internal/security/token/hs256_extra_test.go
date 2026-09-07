package token

import (
	"crypto/hmac"
	"crypto/sha256"
	"testing"
)

// signSegment 用给定密钥对任意 "h.p" 输入计算合法签名段，
// 使 Verify 能通过签名校验、抵达后续 payload 解码步骤。
func signSegment(secret, head, payload string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(head + "." + payload))
	return b64enc(mac.Sum(nil))
}

// TestManager_Verify_InvalidPayloadBase64 覆盖 Verify 中
// claims 段 base64 解码失败分支（hs256.go:53）：
// 签名合法（覆盖 "!!!" 这串非法 base64url 字符）但 payload 无法解码。
func TestManager_Verify_InvalidPayloadBase64(t *testing.T) {
	m := NewManager("s3cret")

	tok := "AAA.!!!" + "." + signSegment("s3cret", "AAA", "!!!")

	username, roles, err := m.Verify(tok)
	if err == nil {
		t.Fatalf("expected base64 decode error, got user=%q roles=%v", username, roles)
	}
}

// TestManager_Verify_InvalidClaimsJSON 覆盖 Verify 中
// claims JSON 反序列化失败分支（hs256.go:57）：
// payload 是合法 base64 但内容不是 JSON。
func TestManager_Verify_InvalidClaimsJSON(t *testing.T) {
	m := NewManager("s3cret")

	payload := b64enc([]byte("not-json-at-all"))
	tok := "AAA." + payload + "." + signSegment("s3cret", "AAA", payload)

	username, roles, err := m.Verify(tok)
	if err == nil {
		t.Fatalf("expected json unmarshal error, got user=%q roles=%v", username, roles)
	}
}
