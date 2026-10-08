package cicd

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestNormalizeExtra_Float64Branch 直接单测覆盖 normalizeExtra 的 case float64 分支。
// 该分支在生产路径不可达（入参仅来自 JSON 列 json.Number 或 map[string]string），
// 但在函数层可直接构造 map[string]interface{} 触发。
func TestNormalizeExtra_Float64Branch(t *testing.T) {
	in := map[string]interface{}{"v": float64(3.14)}
	out := normalizeExtra(in)
	assert.Equal(t, "3.14", out["v"])
}

// TestNormalizeExtra_TypeCoercion 覆盖 bool/嵌套对象/string 等其它分支（补充完整度）。
func TestNormalizeExtra_TypeCoercion(t *testing.T) {
	in := map[string]interface{}{
		"num":  json.Number("42"),
		"flag": true,
		"obj":  map[string]any{"a": 1},
		"str":  "keep",
	}
	out := normalizeExtra(in)
	assert.Equal(t, "42", out["num"])
	assert.Equal(t, "true", out["flag"])
	assert.Equal(t, `{"a":1}`, out["obj"])
	assert.Equal(t, "keep", out["str"])
}
