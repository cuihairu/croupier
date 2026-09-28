package utils

import (
	"fmt"
	"time"
	"unicode"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/platform/settings"
)

// Password validation requirements
const (
	MinPasswordLength = 8
	MaxPasswordLength = 128
)

// Common weak passwords that should be rejected
// This is a basic list - in production, consider using a comprehensive database
var weakPasswords = map[string]bool{
	"password":    true,
	"12345678":    true,
	"123456789":   true,
	"qwerty123":   true,
	"abc12345":    true,
	"password123": true,
	"admin123":    true,
	"root123":     true,
	"letmein":     true,
	"welcome":     true,
	"monkey":      true,
	"dragon":      true,
	"master":      true,
	"hello123":    true,
	"football":    true,
	"iloveyou":    true,
	"princess":    true,
	"adobe123":    true,
	"admin":       true,
	"root":        true,
	"test123":     true,
	"guest":       true,
	"123456":      true,
	"1234567":     true,
}

// ValidatePassword ensures password meets security requirements.
//
// Requirements:
// - Non-empty and no whitespace
// - Length between 8 and 128 characters
// - Contains at least 2 of: uppercase, lowercase, digits, special chars
// - Not a common weak password
func ValidatePassword(password string) (string, error) {
	if password == "" {
		return "", errorx.NewBadRequest("密码不能为空")
	}

	// Check for whitespace
	for _, r := range password {
		if unicode.IsSpace(r) {
			return "", errorx.NewBadRequest("密码不能包含空格")
		}
	}

	// Check length
	if len(password) < MinPasswordLength {
		return "", errorx.NewBadRequest("密码长度至少为8个字符")
	}
	if len(password) > MaxPasswordLength {
		return "", errorx.NewBadRequest("密码长度不能超过128个字符")
	}

	// Check for weak passwords
	if weakPasswords[password] {
		return "", errorx.NewBadRequest("密码过于简单，请使用更复杂的密码")
	}

	// Check character variety (at least 2 of 4 categories)
	var hasUpper, hasLower, hasDigit, hasSpecial bool
	for _, r := range password {
		switch {
		case unicode.IsUpper(r):
			hasUpper = true
		case unicode.IsLower(r):
			hasLower = true
		case unicode.IsDigit(r):
			hasDigit = true
		case unicode.IsPunct(r) || unicode.IsSymbol(r):
			hasSpecial = true
		}
	}

	varietyCount := 0
	if hasUpper {
		varietyCount++
	}
	if hasLower {
		varietyCount++
	}
	if hasDigit {
		varietyCount++
	}
	if hasSpecial {
		varietyCount++
	}

	if varietyCount < 2 {
		return "", errorx.NewBadRequest("密码必须包含大写字母、小写字母、数字、特殊字符中的至少两种")
	}

	// 账号安全策略叠加（site settings，默认全关 = 纯内置基线）：管理员可
	// 收紧最小长度、强制大写/特殊字符；建号/重置/自助改密共用同一入口。
	if err := applySecurityPolicy(password, hasUpper, hasSpecial); err != nil {
		return "", err
	}

	return password, nil
}

// applySecurityPolicy 校验 settings.security.* 账号安全策略中「密码强度」
// 三个键（minLength / requireUppercase / requireSpecial）。minLength 只收紧
// 不放宽：0 或小于内置 8 时按内置基线（上方已校验），策略值更大才追加。
func applySecurityPolicy(password string, hasUpper, hasSpecial bool) error {
	policy := settings.Current().SecurityPolicy()
	if policy.PasswordMinLength > MinPasswordLength && len(password) < policy.PasswordMinLength {
		return errorx.NewBadRequest(fmt.Sprintf("密码长度至少为 %d 个字符（账号安全策略）", policy.PasswordMinLength))
	}
	if policy.PasswordRequireUppercase && !hasUpper {
		return errorx.NewBadRequest("密码必须包含大写字母（账号安全策略）")
	}
	if policy.PasswordRequireSpecial && !hasSpecial {
		return errorx.NewBadRequest("密码必须包含特殊字符（账号安全策略）")
	}
	return nil
}

// PasswordExpiresAtFromPolicy 按账号安全策略计算密码过期时刻：
// maxAgeDays<=0 = 永不过期（nil）。改密/建号在策略开启时落
// admins.password_expires_at，复用既有 mustChangePassword 强制改密链
// （登录响应 mustChangePassword=true，前端强制走改密流程）。
func PasswordExpiresAtFromPolicy() *time.Time {
	days := settings.Current().SecurityPolicy().PasswordMaxAgeDays
	if days <= 0 {
		return nil
	}
	expiresAt := time.Now().UTC().Add(time.Duration(days) * 24 * time.Hour)
	return &expiresAt
}

// ValidatePasswordForUser validates password with optional username check.
// This rejects passwords that contain the username (case-insensitive).
func ValidatePasswordForUser(password, username string) error {
	if _, err := ValidatePassword(password); err != nil {
		return err
	}

	// Check if password contains username (security risk)
	if username != "" && containsIgnoreCase(password, username) {
		return errorx.NewBadRequest("密码不能包含用户名")
	}

	return nil
}

// containsIgnoreCase checks if substr is contained in s (case-insensitive).
func containsIgnoreCase(s, substr string) bool {
	s = toLowerFold(s)
	substr = toLowerFold(substr)
	return len(s) >= len(substr) && indexOf(s, substr) >= 0
}

// toLowerFold converts a string to lowercase for case-insensitive comparison.
func toLowerFold(s string) string {
	// Simple ASCII lowercase - sufficient for username comparison
	result := make([]byte, 0, len(s))
	for _, r := range s {
		if r >= 'A' && r <= 'Z' {
			result = append(result, byte(r)+'a'-'A')
		} else {
			result = append(result, byte(r))
		}
	}
	return string(result)
}

// indexOf finds the index of substr in s, or -1 if not found.
func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
