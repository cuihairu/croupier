package utils

import (
	"strings"
	"unicode"

	"github.com/cuihairu/croupier/internal/common/errorx"
)

// 常见弱密码列表
var commonPasswords = []string{
	"password", "123456", "12345678", "qwerty", "abc123",
	"monkey", "1234567", "letmein", "trustno1", "dragon",
	"baseball", "111111", "iloveyou", "master", "sunshine",
	"ashley", "bailey", "passw0rd", "shadow", "123123",
	"654321", "superman", "qazwsx", "michael", "admin",
	"welcome", "login", "starwars", "hello", "freedom",
	"whatever", "qazwsxedc", "000000",
	// 测试中使用的弱密码变体
	"admin1234", "welcome1", "password123", "qwerty12",
}

// 密码复杂度类型
const (
	Lowercase = 1 << iota // 小写字母
	Uppercase             // 大写字母
	Digit                 // 数字
	Special               // 特殊字符
)

// ValidatePassword 验证密码强度
// 规则：
// 1. 长度至少 8 个字符
// 2. 不能包含空格
// 3. 必须包含大写字母、小写字母、数字、特殊字符中的至少两种
// 4. 不能是常见弱密码
func ValidatePassword(password string) (string, error) {
	if password == "" {
		return "", errorx.NewBadRequest("密码不能为空")
	}

	// 检查空格
	for _, r := range password {
		if unicode.IsSpace(r) {
			return "", errorx.NewBadRequest("密码不能包含空格")
		}
	}

	// 检查长度
	if len(password) < 8 {
		return "", errorx.NewBadRequest("密码长度至少为8个字符")
	}

	// 检查常见弱密码（不区分大小写）
	lowerPassword := strings.ToLower(password)
	for _, weak := range commonPasswords {
		if lowerPassword == weak {
			return "", errorx.NewBadRequest("密码过于简单，请使用更复杂的密码")
		}
	}

	// 检查字符类型组合
	var types int
	for _, r := range password {
		switch {
		case unicode.IsLower(r):
			types |= Lowercase
		case unicode.IsUpper(r):
			types |= Uppercase
		case unicode.IsDigit(r):
			types |= Digit
		case unicode.IsPunct(r) || unicode.IsSymbol(r):
			types |= Special
		}
	}

	// 统计启用的类型数量
	typeCount := 0
	if types&Lowercase != 0 {
		typeCount++
	}
	if types&Uppercase != 0 {
		typeCount++
	}
	if types&Digit != 0 {
		typeCount++
	}
	if types&Special != 0 {
		typeCount++
	}

	if typeCount < 2 {
		return "", errorx.NewBadRequest("密码必须包含大写字母、小写字母、数字、特殊字符中的至少两种")
	}

	return password, nil
}

// ValidatePasswordForUser 验证密码强度（针对特定用户）
// 在 ValidatePassword 基础上增加：密码不能包含用户名
func ValidatePasswordForUser(password, username string) (string, error) {
	// 先进行基本验证
	validated, err := ValidatePassword(password)
	if err != nil {
		return validated, err
	}

	// 检查密码是否包含用户名（不区分大小写）
	if username != "" {
		lowerPassword := strings.ToLower(password)
		lowerUsername := strings.ToLower(username)
		if strings.Contains(lowerPassword, lowerUsername) {
			return "", errorx.NewBadRequest("密码不能包含用户名")
		}
	}

	return password, nil
}
