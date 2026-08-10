package function

import (
	"encoding/json"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// ---------------------------------------------------------------------------
// rawJSONFromBytes
// ---------------------------------------------------------------------------

func TestRawJSONFromBytes(t *testing.T) {
	assert.Nil(t, rawJSONFromBytes(nil))
	assert.Nil(t, rawJSONFromBytes([]byte{}))

	valid := rawJSONFromBytes([]byte(`{"a":1}`))
	assert.True(t, json.Valid(valid))
	assert.JSONEq(t, `{"a":1}`, string(valid))

	// Invalid JSON gets quoted as string
	invalid := rawJSONFromBytes([]byte(`not json`))
	assert.NotNil(t, invalid)
	assert.True(t, json.Valid(invalid))
}

// ---------------------------------------------------------------------------
// rawJSONFromValue
// ---------------------------------------------------------------------------

func TestRawJSONFromValue(t *testing.T) {
	assert.Nil(t, rawJSONFromValue(nil))

	// json.RawMessage
	raw := json.RawMessage(`{"x":1}`)
	result := rawJSONFromValue(raw)
	assert.JSONEq(t, `{"x":1}`, string(result))

	// []byte
	result = rawJSONFromValue([]byte(`"hello"`))
	assert.JSONEq(t, `"hello"`, string(result))

	// string
	result = rawJSONFromValue(`  {"y":2}  `)
	assert.JSONEq(t, `{"y":2}`, string(result))

	// struct
	type S struct{ Name string }
	result = rawJSONFromValue(S{Name: "test"})
	assert.JSONEq(t, `{"Name":"test"}`, string(result))

	// map
	result = rawJSONFromValue(map[string]int{"a": 1})
	assert.JSONEq(t, `{"a":1}`, string(result))
}

// ---------------------------------------------------------------------------
// jsonValueFromRaw
// ---------------------------------------------------------------------------

func TestJsonValueFromRaw(t *testing.T) {
	v, err := jsonValueFromRaw(nil)
	require.NoError(t, err)
	assert.Nil(t, v)

	v, err = jsonValueFromRaw(json.RawMessage(`"hello"`))
	require.NoError(t, err)
	assert.Equal(t, "hello", v)

	v, err = jsonValueFromRaw(json.RawMessage(`42`))
	require.NoError(t, err)
	assert.Equal(t, float64(42), v)

	v, err = jsonValueFromRaw(json.RawMessage(`{"a":1}`))
	require.NoError(t, err)
	m, ok := v.(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, float64(1), m["a"])

	_, err = jsonValueFromRaw(json.RawMessage(`bad`))
	assert.Error(t, err)
}

// ---------------------------------------------------------------------------
// jsonObjectFromRaw
// ---------------------------------------------------------------------------

func TestJsonObjectFromRaw(t *testing.T) {
	m, err := jsonObjectFromRaw(nil)
	require.NoError(t, err)
	assert.Nil(t, m)

	m, err = jsonObjectFromRaw(json.RawMessage(`{"key":"val"}`))
	require.NoError(t, err)
	assert.Equal(t, "val", m["key"])

	_, err = jsonObjectFromRaw(json.RawMessage(`"not object"`))
	assert.Error(t, err)

	_, err = jsonObjectFromRaw(json.RawMessage(`bad`))
	assert.Error(t, err)
}

// ---------------------------------------------------------------------------
// BatchCopyFunctions
// ---------------------------------------------------------------------------

func TestBatchCopyFunctions_EmptyIds(t *testing.T) {
	svcCtx := setupBatchTestContext(t)
	logic := NewBatchCopyFunctionsLogic(t.Context(), svcCtx)
	resp, err := logic.BatchCopyFunctions(&BatchCopyFunctionsRequest{FunctionIds: []string{}})
	require.NoError(t, err)
	assert.Equal(t, 0, resp.Updated)
	assert.Empty(t, resp.Copied)
}

// ---------------------------------------------------------------------------
// BatchDeleteFunctions
// ---------------------------------------------------------------------------

func TestBatchDeleteFunctions_EmptyIds(t *testing.T) {
	svcCtx := setupBatchTestContext(t)
	logic := NewBatchDeleteFunctionsLogic(t.Context(), svcCtx)
	resp, err := logic.BatchDeleteFunctions(&BatchDeleteFunctionsRequest{FunctionIds: []string{}})
	require.NoError(t, err)
	assert.Equal(t, 0, resp.Updated)
	assert.Empty(t, resp.Failed)
}

// ---------------------------------------------------------------------------
// BatchUpdateFunctions
// ---------------------------------------------------------------------------

func TestBatchUpdateFunctions_EmptyIds(t *testing.T) {
	svcCtx := setupBatchTestContext(t)
	logic := NewBatchUpdateFunctionsLogic(t.Context(), svcCtx)
	resp, err := logic.BatchUpdateFunctions(&BatchUpdateFunctionsRequest{FunctionIds: []string{}, Enabled: true})
	require.NoError(t, err)
	assert.Equal(t, 0, resp.Updated)
	assert.Empty(t, resp.Failed)
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func setupBatchTestContext(t *testing.T) *svc.ServiceContext {
	t.Helper()
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Function{}))
	return &svc.ServiceContext{
		DB:            db,
		FunctionModel: model.NewFunctionModel(db),
	}
}
