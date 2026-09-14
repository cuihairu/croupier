package utils

import (
	"math"
	"strconv"
	"strings"

	"github.com/cuihairu/croupier/internal/common/errorx"
)

// ParseUintID parses string ID to uint with label.
func ParseUintID(id, label string) (uint, error) {
	if strings.TrimSpace(id) == "" {
		return 0, errorx.NewBadRequest(label + "不能为空")
	}
	// 按目标类型 uint 自身的宽度解析（strconv.IntSize）：ParseUint 的成功结果
	// 上界即 uint 上界，uint(value) 转换在任意平台都可证明安全（CodeQL
	// go/incorrect-integer-conversion 认可），无需引入 64 位平台恒假的死分支守卫。
	value, err := strconv.ParseUint(id, 10, strconv.IntSize)
	if err != nil {
		return 0, errorx.NewBadRequest(label + "无效")
	}
	if value == 0 {
		return 0, errorx.NewBadRequest(label + "必须大于0")
	}
	// 平台 ID 在 API 契约中以 int64 回显（DTO Id 字段约定），超出 MaxInt64 的
	// 输入不可表示，直接拒绝；该分支真实可达（可测），同时使下游
	// int64(解析结果) 转换可证明安全（CodeQL UpperBoundCheckGuard 降级位宽）。
	if value > math.MaxInt64 {
		return 0, errorx.NewBadRequest(label + "超出有效范围")
	}
	return uint(value), nil
}
