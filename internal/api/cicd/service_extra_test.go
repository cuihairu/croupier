package cicd

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestNormalizeExtra_Float64Branch unit-tests the case float64 branch of
// normalizeExtra directly. The branch is unreachable via production paths (inputs
// only come from the JSON column as json.Number or from map[string]string), but can
// be triggered at the function level by building a map[string]interface{} directly.
func TestNormalizeExtra_Float64Branch(t *testing.T) {
	in := map[string]interface{}{"v": float64(3.14)}
	out := normalizeExtra(in)
	assert.Equal(t, "3.14", out["v"])
}

// TestNormalizeExtra_TypeCoercion covers the remaining branches (bool / nested object
// / string) for completeness.
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
