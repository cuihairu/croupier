package dbtype

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// json.RawMessage.UnmarshalJSON 对非空接收者恒返回 nil error（原文透传，
// 解析阶段不校验），UnmarshalJSON 的错误分支仅在缝隙注入故障时可达。
// 验证错误被原样透传且接收者保持原值不被覆盖。
func TestJSONUnmarshalJSON_RawMessageErrorSeam(t *testing.T) {
	orig := unmarshalRawMessage
	unmarshalRawMessage = func(*json.RawMessage, []byte) error {
		return errors.New("raw message explode")
	}
	t.Cleanup(func() { unmarshalRawMessage = orig })

	var j JSON = JSON(`{"keep":1}`)
	err := j.UnmarshalJSON([]byte(`{"x":1}`))
	require.EqualError(t, err, "raw message explode")
	assert.Equal(t, `{"keep":1}`, j.String())
}
