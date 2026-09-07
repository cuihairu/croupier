package executionlog

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cuihairu/croupier/internal/model"
)

// hugeNumberMarshaler 的 MarshalJSON 产出 1e999：语法上是合法 JSON 数字
// （marshal/compact 通过），但往返解码为 float64 溢出，命中 normalizeJSON
// 的 Unmarshal 失败降级分支。
type hugeNumberMarshaler struct{}

func (hugeNumberMarshaler) MarshalJSON() ([]byte, error) { return []byte("1e999"), nil }

func TestEncodePayloadInvalidJSONRoundTrip(t *testing.T) {
	out := encodePayload(hugeNumberMarshaler{}, DefaultMaxPayloadBytes)
	var decoded map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(out), &decoded), "输出必须是合法 JSON: %s", out)
	assert.Contains(t, decoded, "logUnserializable")
	assert.False(t, exceedsBytes(hugeNumberMarshaler{}, DefaultMaxPayloadBytes))
}

// marshal 失败（注入缝隙）→ encodePayload 落到 logEncodeError 摘要、
// exceedsBytes 视为不超限。
func TestEncodePayloadMarshalErrorSeam(t *testing.T) {
	orig := marshalPayload
	marshalPayload = func(v interface{}) ([]byte, error) { return nil, errors.New("boom") }
	t.Cleanup(func() { marshalPayload = orig })

	assert.Equal(t,
		model.JSON(`{"logEncodeError":true}`),
		encodePayload(map[string]interface{}{"k": "v"}, DefaultMaxPayloadBytes))
	assert.False(t, exceedsBytes(map[string]interface{}{"k": "v"}, DefaultMaxPayloadBytes))
}
