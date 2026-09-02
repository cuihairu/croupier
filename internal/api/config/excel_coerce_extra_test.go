// 覆盖目标：excel 编译的 validCellType/coerceCell 类型转换矩阵。
package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidCellType_Matrix(t *testing.T) {
	for _, ok := range []string{"int", "string", "float", "bool"} {
		assert.True(t, validCellType(ok), ok)
	}
	for _, bad := range []string{"", "number", "text", "INT"} {
		assert.False(t, validCellType(bad), bad)
	}
}

func TestCoerceCell_Matrix(t *testing.T) {
	// int
	v, err := coerceCell("42", "int")
	require.NoError(t, err)
	assert.EqualValues(t, 42, v)
	_, err = coerceCell("4.5", "int")
	require.Error(t, err)

	// float
	v, err = coerceCell("1.5", "float")
	require.NoError(t, err)
	assert.EqualValues(t, 1.5, v)
	_, err = coerceCell("abc", "float")
	require.Error(t, err)

	// bool 多形态
	v, err = coerceCell("true", "bool")
	require.NoError(t, err)
	assert.Equal(t, true, v)
	v, err = coerceCell("1", "bool")
	require.NoError(t, err)
	assert.Equal(t, true, v)
	v, err = coerceCell("FALSE", "bool")
	require.NoError(t, err)
	assert.Equal(t, false, v)
	_, err = coerceCell("maybe", "bool")
	require.Error(t, err)

	// string 直通
	v, err = coerceCell("raw", "string")
	require.NoError(t, err)
	assert.Equal(t, "raw", v)
}
