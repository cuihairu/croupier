package utils

import (
	"fmt"
	"strconv"
	"strings"
)

// ParseUintID parses string ID to uint with label.
func ParseUintID(id, label string) (uint, error) {
	if strings.TrimSpace(id) == "" {
		return 0, fmt.Errorf("%s不能为空", label)
	}
	value, err := strconv.ParseUint(id, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%s无效: %w", label, err)
	}
	if value == 0 {
		return 0, fmt.Errorf("%s必须大于0", label)
	}
	return uint(value), nil
}
