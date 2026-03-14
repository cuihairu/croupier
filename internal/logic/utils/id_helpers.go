package utils

import (
	"strconv"
	"strings"

	"github.com/cuihairu/croupier/internal/common/errorx"
)

// ParseUintID parses string ID to uint with label.
func ParseUintID(id, label string) (uint, error) {
	if strings.TrimSpace(id) == "" {
		return 0, errorx.NewBadRequest(label + "不能为空")
	}
	value, err := strconv.ParseUint(id, 10, 64)
	if err != nil {
		return 0, errorx.NewBadRequest(label + "无效")
	}
	if value == 0 {
		return 0, errorx.NewBadRequest(label + "必须大于0")
	}
	return uint(value), nil
}
