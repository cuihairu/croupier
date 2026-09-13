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
	// 无需再检查 value > math.MaxUint：ParseUint(bitSize=64) 的成功结果上界
	// 即 math.MaxUint64，而本项目目标平台（linux/amd64、linux/arm64 等 64
	// 位平台）下 uint 为 64 位，MaxUint == MaxUint64，该比较恒假。
	return uint(value), nil
}
