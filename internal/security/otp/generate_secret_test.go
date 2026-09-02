package otp

// 覆盖目标：GenerateSecret 正常路径（160-bit 随机 secret 的 base32 编码、
// 唯一性以及与 VerifyTOTP 的往返校验）。rand.Read 失败分支无法注入，不做覆盖。

import (
	"crypto/rand"
	"encoding/base32"
	"testing"
	"time"
)

func TestGenerateSecret_Base32NoPadding(t *testing.T) {
	secret, err := GenerateSecret()
	if err != nil {
		t.Fatalf("GenerateSecret() error = %v", err)
	}
	// 20 字节 = 160 bit，base32 无填充编码固定 32 字符。
	if len(secret) != 32 {
		t.Fatalf("secret length = %d, want 32", len(secret))
	}
	decoded, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(secret)
	if err != nil {
		t.Fatalf("secret %q is not valid base32: %v", secret, err)
	}
	if len(decoded) != 20 {
		t.Fatalf("decoded length = %d, want 20", len(decoded))
	}
}

func TestGenerateSecret_Unique(t *testing.T) {
	seen := make(map[string]bool, 32)
	for i := 0; i < 32; i++ {
		secret, err := GenerateSecret()
		if err != nil {
			t.Fatalf("GenerateSecret() error = %v", err)
		}
		if seen[secret] {
			t.Fatalf("duplicate secret generated: %s", secret)
		}
		seen[secret] = true
	}
}

func TestGenerateSecret_VerifyRoundTrip(t *testing.T) {
	secret, err := GenerateSecret()
	if err != nil {
		t.Fatalf("GenerateSecret() error = %v", err)
	}
	decoded, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(secret)
	if err != nil {
		t.Fatalf("decode secret: %v", err)
	}

	step := time.Now().Unix() / 30
	code := hotp(decoded, uint64(step), 6)
	if !VerifyTOTP(secret, code, 0) {
		t.Fatalf("VerifyTOTP with current-step code should succeed, secret=%s code=%s", secret, code)
	}
	// 前后各一个时间步的码在 skew=1 下也应通过。
	if !VerifyTOTP(secret, hotp(decoded, uint64(step+1), 6), 1) {
		t.Fatal("VerifyTOTP with next-step code and skew=1 should succeed")
	}
	if VerifyTOTP(secret, hotp(decoded, uint64(step+5), 6), 0) {
		t.Fatal("VerifyTOTP with out-of-window code and skew=0 should fail")
	}
}

func TestGenerateSecret_NotAllZero(t *testing.T) {
	// 连续生成多个全零 secret 的概率可忽略，用于确认熵源可用。
	zero := make([]byte, 20)
	allZero := true
	for i := 0; i < 8; i++ {
		secret, err := GenerateSecret()
		if err != nil {
			t.Fatalf("GenerateSecret() error = %v", err)
		}
		decoded, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(secret)
		if err != nil {
			t.Fatalf("decode secret: %v", err)
		}
		for j := range decoded {
			if decoded[j] != zero[j] {
				allZero = false
			}
		}
	}
	if allZero {
		t.Fatal("GenerateSecret returned all-zero secrets; entropy source looks broken")
	}
	_ = rand.Reader // 保持与生产实现相同的熵源引用
}
