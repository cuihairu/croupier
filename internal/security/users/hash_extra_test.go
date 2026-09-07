package users

import (
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

// TestHashPassword_TooLong 覆盖 HashPassword 中 bcrypt 报错分支（store.go:26）：
// x/crypto bcrypt 对超过 72 字节的密码返回 ErrPasswordTooLong。
func TestHashPassword_TooLong(t *testing.T) {
	long := strings.Repeat("a", 73)

	hashed, err := HashPassword(long)
	if err == nil {
		t.Fatal("expected error for password longer than 72 bytes")
	}
	if err != bcrypt.ErrPasswordTooLong {
		t.Errorf("expected bcrypt.ErrPasswordTooLong, got %v", err)
	}
	if hashed != "" {
		t.Errorf("expected empty hash on error, got %q", hashed)
	}
}

// TestHashPassword_MaxAllowedLength 验证 72 字节边界值仍可正常哈希。
func TestHashPassword_MaxAllowedLength(t *testing.T) {
	hashed, err := HashPassword(strings.Repeat("a", 72))
	if err != nil {
		t.Fatalf("unexpected error at 72-byte boundary: %v", err)
	}
	if !strings.HasPrefix(hashed, "$2") {
		t.Errorf("expected bcrypt hash format, got %q", hashed)
	}
}
