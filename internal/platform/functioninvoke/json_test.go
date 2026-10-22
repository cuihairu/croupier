package functioninvoke

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 随实现从 internal/api/function 迁入：非 JSON 字节走 JSON 字符串编码，
// 编码器故障时返回 nil 而不是半截输出。
func TestRawJSONFromBytes_MarshalStringFailure(t *testing.T) {
	orig := marshalRawString
	marshalRawString = func(v any) ([]byte, error) {
		return nil, errors.New("raw string marshal boom")
	}
	t.Cleanup(func() { marshalRawString = orig })

	assert.Nil(t, RawJSONFromBytes([]byte("plain text")))
}

func TestRawJSONFromBytes(t *testing.T) {
	assert.Nil(t, RawJSONFromBytes(nil))
	assert.Nil(t, RawJSONFromBytes([]byte{}))
	assert.JSONEq(t, `{"a":1}`, string(RawJSONFromBytes([]byte(`{"a":1}`))))
	assert.Equal(t, `"plain text"`, string(RawJSONFromBytes([]byte("plain text"))))
}

func TestRawJSONFromAny(t *testing.T) {
	assert.Nil(t, RawJSONFromAny(nil))
	assert.JSONEq(t, `{"a":1}`, string(RawJSONFromAny(json.RawMessage(`{"a":1}`))))
	assert.Equal(t, `"txt"`, string(RawJSONFromAny("txt")))
	assert.JSONEq(t, `{"n":2}`, string(RawJSONFromAny(map[string]int{"n": 2})))
}

func TestDedupeStrings(t *testing.T) {
	assert.Equal(t, []string{"a", "b"}, DedupeStrings([]string{"a", "", "b", "a"}))
	assert.Empty(t, DedupeStrings(nil))
}

func TestValidateInvokeRoute(t *testing.T) {
	require.Error(t, ValidateInvokeRoute(nil))

	req := &Request{Route: "  "}
	require.NoError(t, ValidateInvokeRoute(req))
	assert.Equal(t, "lb", req.Route)

	require.Error(t, ValidateInvokeRoute(&Request{Route: "targeted"}))
	require.Error(t, ValidateInvokeRoute(&Request{Route: "hash"}))
	require.Error(t, ValidateInvokeRoute(&Request{Route: "broadcast", Mode: "async"}))
	require.Error(t, ValidateInvokeRoute(&Request{Route: "bogus"}))
	require.NoError(t, ValidateInvokeRoute(&Request{Route: "broadcast", Mode: "sync"}))
}

func TestInvokePayload(t *testing.T) {
	assert.Equal(t, []byte("null"), InvokePayload(nil))
	assert.Equal(t, []byte(`{"p":1}`), InvokePayload(&Request{Payload: []byte(`{"p":1}`)}))
	assert.Equal(t, []byte(`{"q":1}`), InvokePayload(&Request{Params: []byte(`{"q":1}`)}))
	assert.Equal(t, []byte("{}"), InvokePayload(&Request{}))
}

func TestCloneMetadata(t *testing.T) {
	assert.Nil(t, CloneMetadata(nil))
	src := map[string]string{"a": "1"}
	out := CloneMetadata(src)
	out["b"] = "2"
	assert.Equal(t, "1", src["a"])
	_, ok := src["b"]
	assert.False(t, ok)
}

func TestMetadataPredicates(t *testing.T) {
	assert.False(t, IsApprovedContinuation(nil))
	assert.True(t, IsApprovedContinuation(map[string]string{"approvalBypass": " Approved "}))
	assert.False(t, IsPageSnapshotGoverned(nil))
	assert.True(t, IsPageSnapshotGoverned(map[string]string{"pageSnapshotGovernance": "validated"}))
}
