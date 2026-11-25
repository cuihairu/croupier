package requestbind

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Value-typed struct: gin's form binding still runs field validation and fails
// (required), and in the fallback rv.Kind() != reflect.Ptr → the
// ValidateStruct error is returned directly.
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

// *map[string]int: gin reports "can not convert to map of strings", and in
// the fallback the pointer targets a non-struct → ValidateStruct returns nil
// for non-structs.
func TestBindQueryCompat_PointerToMapFallsBackToValidate(t *testing.T) {
	ctx := newBindContext(t, "a=1")

	m := map[string]int{}
	err := BindQueryCompat(ctx, &m)
	assert.NoError(t, err)
	assert.Equal(t, map[string]int{}, m)
}

// Pointer struct containing unexported fields: form binding fails because
// page=abc cannot convert to int and enters the fallback, where the loop
// continues on the unexported field with CanSet=false (covering that branch),
// the int parse failure is silently skipped, and ValidateStruct passes
// returning nil.
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
