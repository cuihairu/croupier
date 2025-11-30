package utils

import (
	"fmt"
	"unicode"
)

// ValidatePassword ensures password is non-empty and contains no whitespace.
func ValidatePassword(password string) (string, error) {
	if password == "" {
		return "", fmt.Errorf("密码不能为空")
	}
	for _, r := range password {
		if unicode.IsSpace(r) {
			return "", fmt.Errorf("密码不能包含空格")
		}
	}
	return password, nil
}
