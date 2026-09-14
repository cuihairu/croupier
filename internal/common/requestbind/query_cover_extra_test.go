package requestbind

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 值类型结构体：gin 的 form binding 依然执行字段校验并失败（required），
// fallback 中 rv.Kind() != reflect.Ptr → 直接返回 ValidateStruct 的错误。
type valueStructRequiredDTO struct {
	Name string `form:"name" binding:"required"`
}

func TestBindQueryCompat_ValueStructFallsBackToValidate(t *testing.T) {
	ctx := newBindContext(t, "age=25")

	var req valueStructRequiredDTO
	err := BindQueryCompat(ctx, req)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "required")
}

// *map[string]int：gin 报 "can not convert to map of strings"，
// fallback 中指针指向非结构体 → ValidateStruct 对非结构体返回 nil。
func TestBindQueryCompat_PointerToMapFallsBackToValidate(t *testing.T) {
	ctx := newBindContext(t, "a=1")

	m := map[string]int{}
	err := BindQueryCompat(ctx, &m)
	assert.NoError(t, err)
	assert.Equal(t, map[string]int{}, m)
}

// 含未导出字段的指针结构体：form binding 因 page=abc 无法转 int 失败进
// fallback，循环对 CanSet=false 的未导出字段 continue（覆盖该分支），
// int 解析失败静默跳过，ValidateStruct 通过返回 nil。
type withUnexportedFieldDTO struct {
	Page int `form:"page"`
	mark string
}

func TestBindQueryCompat_UnexportedFieldSkippedInFallback(t *testing.T) {
	ctx := newBindContext(t, "page=abc")

	req := withUnexportedFieldDTO{mark: "kept"}
	err := BindQueryCompat(ctx, &req)
	assert.NoError(t, err)
	assert.Equal(t, 0, req.Page, "非数字 query 不应写入 int 字段")
	assert.Equal(t, "kept", req.mark)
}
