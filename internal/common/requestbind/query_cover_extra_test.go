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
