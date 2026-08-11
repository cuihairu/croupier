package function

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/cuihairu/croupier/internal/svc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---- cloneMetadata ----

func TestCloneMetadata_V2(t *testing.T) {
	t.Parallel()

	t.Run("nil map returns nil", func(t *testing.T) {
		t.Parallel()
		assert.Nil(t, cloneMetadata(nil))
	})

	t.Run("empty map returns nil", func(t *testing.T) {
		t.Parallel()
		assert.Nil(t, cloneMetadata(map[string]string{}))
	})

	t.Run("copies entries", func(t *testing.T) {
		t.Parallel()
		src := map[string]string{"a": "1", "b": "2"}
		out := cloneMetadata(src)
		assert.Equal(t, src, out)
		// mutation of out must not affect src
		out["a"] = "changed"
		assert.Equal(t, "1", src["a"])
	})
}

// ---- isApprovedContinuation ----

func TestIsApprovedContinuation_V2(t *testing.T) {
	t.Parallel()

	assert.False(t, isApprovedContinuation(nil))
	assert.False(t, isApprovedContinuation(map[string]string{}))
	assert.False(t, isApprovedContinuation(map[string]string{"approval_bypass": "pending"}))
	// " approved " is trimmed to "approved" so it should be true
	assert.True(t, isApprovedContinuation(map[string]string{"approval_bypass": " approved "}))
	assert.True(t, isApprovedContinuation(map[string]string{"approval_bypass": "approved"}))
	assert.True(t, isApprovedContinuation(map[string]string{"approval_bypass": "Approved"}))
	assert.True(t, isApprovedContinuation(map[string]string{"approval_bypass": " APPROVED "}))
}

// ---- isPageSnapshotGoverned ----

func TestIsPageSnapshotGoverned_V2(t *testing.T) {
	t.Parallel()

	assert.False(t, isPageSnapshotGoverned(nil))
	assert.False(t, isPageSnapshotGoverned(map[string]string{}))
	assert.False(t, isPageSnapshotGoverned(map[string]string{"page_snapshot_governance": "pending"}))
	// " validated " is trimmed to "validated" so it should be true
	assert.True(t, isPageSnapshotGoverned(map[string]string{"page_snapshot_governance": " validated "}))
	assert.True(t, isPageSnapshotGoverned(map[string]string{"page_snapshot_governance": "validated"}))
	assert.True(t, isPageSnapshotGoverned(map[string]string{"page_snapshot_governance": "Validated"}))
}

// ---- invokePayload ----

func TestInvokePayload_V2(t *testing.T) {
	t.Parallel()

	t.Run("nil request", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, []byte("null"), invokePayload(nil))
	})

	t.Run("payload present", func(t *testing.T) {
		t.Parallel()
		req := &FunctionInvokeRequest{
			Payload: json.RawMessage(`{"key":"val"}`),
			Params:  json.RawMessage(`{"should":"ignore"}`),
		}
		assert.JSONEq(t, `{"key":"val"}`, string(invokePayload(req)))
	})

	t.Run("only params", func(t *testing.T) {
		t.Parallel()
		req := &FunctionInvokeRequest{
			Params: json.RawMessage(`{"param":1}`),
		}
		assert.JSONEq(t, `{"param":1}`, string(invokePayload(req)))
	})

	t.Run("empty", func(t *testing.T) {
		t.Parallel()
		req := &FunctionInvokeRequest{}
		assert.JSONEq(t, `{}`, string(invokePayload(req)))
	})
}

// ---- rawJSONFromAny ----

func TestRawJSONFromAny_V2(t *testing.T) {
	t.Parallel()

	t.Run("nil returns nil", func(t *testing.T) {
		t.Parallel()
		assert.Nil(t, rawJSONFromAny(nil))
	})

	t.Run("RawMessage passthrough", func(t *testing.T) {
		t.Parallel()
		in := json.RawMessage(`{"a":1}`)
		out := rawJSONFromAny(in)
		assert.Equal(t, string(in), string(out))
		// mutation safety
		out[1] = 'x'
		assert.Equal(t, `{"a":1}`, string(in))
	})

	t.Run("[]byte", func(t *testing.T) {
		t.Parallel()
		out := rawJSONFromAny([]byte(`"hello"`))
		assert.Equal(t, `"hello"`, string(out))
	})

	t.Run("string", func(t *testing.T) {
		t.Parallel()
		out := rawJSONFromAny(`"world"`)
		assert.Equal(t, `"world"`, string(out))
	})

	t.Run("default marshal", func(t *testing.T) {
		t.Parallel()
		out := rawJSONFromAny(42)
		assert.Equal(t, `42`, string(out))
	})

	t.Run("unmarshalable value returns nil", func(t *testing.T) {
		t.Parallel()
		// channel cannot be marshalled
		out := rawJSONFromAny(make(chan int))
		assert.Nil(t, out)
	})
}

// ---- rawJSONFromBytes ----

func TestRawJSONFromBytes_V2(t *testing.T) {
	t.Parallel()

	t.Run("nil returns nil", func(t *testing.T) {
		t.Parallel()
		assert.Nil(t, rawJSONFromBytes(nil))
	})

	t.Run("empty returns nil", func(t *testing.T) {
		t.Parallel()
		assert.Nil(t, rawJSONFromBytes([]byte{}))
	})

	t.Run("valid JSON", func(t *testing.T) {
		t.Parallel()
		out := rawJSONFromBytes([]byte(`{"k":"v"}`))
		assert.JSONEq(t, `{"k":"v"}`, string(out))
	})

	t.Run("invalid JSON gets encoded", func(t *testing.T) {
		t.Parallel()
		out := rawJSONFromBytes([]byte("not json"))
		var s string
		require.NoError(t, json.Unmarshal(out, &s))
		assert.Equal(t, "not json", s)
	})

	t.Run("mutation safety", func(t *testing.T) {
		t.Parallel()
		src := []byte(`{"a":1}`)
		out := rawJSONFromBytes(src)
		out[1] = 'x'
		assert.Equal(t, `{"a":1}`, string(src))
	})
}

// ---- matchAnyRole ----

func TestMatchAnyRole_V2(t *testing.T) {
	t.Parallel()

	assert.False(t, matchAnyRole(nil, nil))
	assert.False(t, matchAnyRole([]string{}, []string{}))
	assert.True(t, matchAnyRole([]string{"admin"}, []string{"admin"}))
	assert.True(t, matchAnyRole([]string{"Admin"}, []string{"admin"}))
	assert.True(t, matchAnyRole([]string{"admin"}, []string{"Admin"}))
	assert.False(t, matchAnyRole([]string{"guest"}, []string{"admin"}))
	assert.True(t, matchAnyRole([]string{"user", "viewer"}, []string{"admin", "viewer"}))
}

// ---- firstNonEmptySlice ----

func TestFirstNonEmptySlice_V2(t *testing.T) {
	t.Parallel()

	assert.Nil(t, firstNonEmptySlice(nil, nil))
	assert.Nil(t, firstNonEmptySlice([]string{}, nil))
	a := []string{"x"}
	assert.Equal(t, a, firstNonEmptySlice(a, []string{"y"}))
	assert.Equal(t, []string{"y"}, firstNonEmptySlice(nil, []string{"y"}))
}

// ---- firstNonEmptyMap ----

func TestFirstNonEmptyMap_V2(t *testing.T) {
	t.Parallel()

	assert.Nil(t, firstNonEmptyMap(nil, nil))
	assert.Nil(t, firstNonEmptyMap(map[string]string{}, nil))
	a := map[string]string{"k": "v"}
	assert.Equal(t, a, firstNonEmptyMap(a, map[string]string{"k2": "v2"}))
	assert.Equal(t, map[string]string{"k2": "v2"}, firstNonEmptyMap(nil, map[string]string{"k2": "v2"}))
}

// ---- trimString ----

func TestTrimString_V2(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "", trimString(""))
	assert.Equal(t, "hello", trimString("  hello  "))
	assert.Equal(t, "hello", trimString("hello"))
}

// ---- getLocalizedTextFromMetadata ----

func TestGetLocalizedTextFromMetadata_V2(t *testing.T) {
	t.Parallel()

	t.Run("nil metadata", func(t *testing.T) {
		t.Parallel()
		assert.Nil(t, getLocalizedTextFromMetadata(nil, "key"))
	})

	t.Run("key missing", func(t *testing.T) {
		t.Parallel()
		assert.Nil(t, getLocalizedTextFromMetadata(map[string]interface{}{}, "key"))
	})

	t.Run("map[string]string", func(t *testing.T) {
		t.Parallel()
		input := map[string]interface{}{
			"summary": map[string]string{"en": "hi", "zh": "你好"},
		}
		out := getLocalizedTextFromMetadata(input, "summary")
		assert.Equal(t, map[string]string{"en": "hi", "zh": "你好"}, out)
	})

	t.Run("map[string]interface{}", func(t *testing.T) {
		t.Parallel()
		input := map[string]interface{}{
			"summary": map[string]interface{}{"en": "hi", "zh": 123},
		}
		out := getLocalizedTextFromMetadata(input, "summary")
		assert.Equal(t, map[string]string{"en": "hi"}, out)
	})

	t.Run("non-map returns nil", func(t *testing.T) {
		t.Parallel()
		input := map[string]interface{}{"summary": "string"}
		assert.Nil(t, getLocalizedTextFromMetadata(input, "summary"))
	})
}

// ---- getBoolFromMetadata extended ----

func TestGetBoolFromMetadataExtended_V2(t *testing.T) {
	t.Parallel()

	meta := map[string]interface{}{
		"b_true":  true,
		"b_false": false,
		"s_true":  "true",
		"s_one":   "1",
		"s_other": "yes",
		"i_non0":  42,
		"i_zero":  0,
		"f_non0":  1.5,
		"f_zero":  0.0,
	}

	assert.True(t, getBoolFromMetadata(meta, "b_true"))
	assert.False(t, getBoolFromMetadata(meta, "b_false"))
	assert.True(t, getBoolFromMetadata(meta, "s_true"))
	assert.True(t, getBoolFromMetadata(meta, "s_one"))
	assert.False(t, getBoolFromMetadata(meta, "s_other"))
	assert.True(t, getBoolFromMetadata(meta, "i_non0"))
	assert.False(t, getBoolFromMetadata(meta, "i_zero"))
	assert.True(t, getBoolFromMetadata(meta, "f_non0"))
	assert.False(t, getBoolFromMetadata(meta, "f_zero"))
	assert.False(t, getBoolFromMetadata(meta, "missing"))
}

// ---- getIntFromMetadata string parse ----

func TestGetIntFromMetadata_V2(t *testing.T) {
	t.Parallel()

	assert.Equal(t, 0, getIntFromMetadata(nil, "k"))
	assert.Equal(t, 0, getIntFromMetadata(map[string]interface{}{}, "k"))
	assert.Equal(t, 5, getIntFromMetadata(map[string]interface{}{"k": 5}, "k"))
	assert.Equal(t, 5, getIntFromMetadata(map[string]interface{}{"k": 5.0}, "k"))
	assert.Equal(t, 99, getIntFromMetadata(map[string]interface{}{"k": "99"}, "k"))
	assert.Equal(t, 0, getIntFromMetadata(map[string]interface{}{"k": "abc"}, "k"))
	assert.Equal(t, 0, getIntFromMetadata(map[string]interface{}{"k": true}, "k"))
}

// ---- getStringSliceFromMetadata []string case ----

func TestGetStringSliceFromMetadata_V2(t *testing.T) {
	t.Parallel()

	assert.Empty(t, getStringSliceFromMetadata(nil, "k"))
	assert.Empty(t, getStringSliceFromMetadata(map[string]interface{}{}, "k"))

	// []string type
	assert.Equal(t, []string{"a", "b"}, getStringSliceFromMetadata(map[string]interface{}{"k": []string{"a", "b"}}, "k"))

	// []interface{} with mixed types
	out := getStringSliceFromMetadata(map[string]interface{}{"k": []interface{}{"a", 123, "b"}}, "k")
	assert.Equal(t, []string{"a", "b"}, out)

	// empty slice
	assert.NotNil(t, getStringSliceFromMetadata(map[string]interface{}{"k": []string{}}, "k"))
}

// ---- functionsPending ----

func TestFunctionsPending_V2(t *testing.T) {
	t.Parallel()
	svcCtx := &svc.ServiceContext{}
	ctx := context.Background()
	resp, err := functionsPending(ctx, svcCtx, &FunctionsPendingRequest{})
	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Empty(t, resp.Items)
}

// ---- functionAnalytics ----

func TestFunctionAnalytics_V2(t *testing.T) {
	t.Parallel()
	svcCtx := setupTestServiceContext(t)
	ctx := context.Background()
	resp, err := functionAnalytics(ctx, svcCtx, &FunctionAnalyticsRequest{ID: "x"})
	require.NoError(t, err)
	assert.Equal(t, int64(0), resp.TotalCalls)
	assert.Equal(t, 100.0, resp.SuccessRate)
}

// ---- functionCopy ----

func TestFunctionCopy_V2(t *testing.T) {
	t.Parallel()
	svcCtx := &svc.ServiceContext{}
	ctx := context.Background()
	resp, err := functionCopy(ctx, svcCtx, &FunctionCopyRequest{ID: "f1"})
	require.NoError(t, err)
	assert.Equal(t, "f1", resp.FunctionId)
}

// ---- functionPublish ----

func TestFunctionPublish_V2(t *testing.T) {
	t.Parallel()
	svcCtx := &svc.ServiceContext{}
	ctx := context.Background()
	resp, err := functionPublish(ctx, svcCtx, &FunctionPublishRequest{ID: "f1"})
	require.NoError(t, err)
	assert.True(t, resp.Published)
}

// ---- functionWarnings ----

func TestFunctionWarnings_V2(t *testing.T) {
	t.Parallel()
	svcCtx := &svc.ServiceContext{}
	ctx := context.Background()
	resp, err := functionWarnings(ctx, svcCtx, &FunctionWarningsRequest{FunctionID: "f1"})
	require.NoError(t, err)
	assert.Empty(t, resp.Items)
}

// ---- functionHistory ----

func TestFunctionHistory_V2(t *testing.T) {
	t.Parallel()
	svcCtx := setupTestServiceContext(t)
	ctx := context.Background()
	resp, err := functionHistory(ctx, svcCtx, &FunctionHistoryRequest{ID: "f1"})
	require.NoError(t, err)
	require.Len(t, resp.Items, 1)
	assert.Equal(t, "function_created", resp.Items[0].Action)
}

// ---- batchCopyFunctions ----

func TestBatchCopyFunctions_V2(t *testing.T) {
	t.Parallel()
	svcCtx := &svc.ServiceContext{}
	ctx := context.Background()
	resp, err := batchCopyFunctions(ctx, svcCtx, &BatchCopyFunctionsRequest{
		Functions: []FunctionCopyRequest{{ID: "a"}, {ID: "b"}},
	})
	require.NoError(t, err)
	assert.Len(t, resp.Results, 2)
}

// ---- batchDeleteFunctions ----

func TestBatchDeleteFunctions_V2(t *testing.T) {
	t.Parallel()
	svcCtx := setupTestServiceContext(t)
	ctx := context.Background()
	createTestFunction(t, svcCtx.DB, "del1", "Del 1")
	createTestFunction(t, svcCtx.DB, "del2", "Del 2")

	resp, err := batchDeleteFunctions(ctx, svcCtx, &BatchDeleteFunctionsRequest{
		FunctionIds: []string{"del1", "del2", "nonexistent"},
	})
	require.NoError(t, err)
	assert.Len(t, resp.Deleted, 3) // all succeed even nonexistent
	assert.Empty(t, resp.Failed)
}

// ---- batchUpdateFunctions (via logic) ----

func TestBatchUpdateFunctions_V2(t *testing.T) {
	t.Parallel()
	svcCtx := setupTestServiceContext(t)
	ctx := context.Background()

	resp, err := batchUpdateFunctions(ctx, svcCtx, &BatchUpdateFunctionsRequest{
		FunctionIds: []string{"f1", "f2"},
		Enabled:     true,
	})
	require.NoError(t, err)
	assert.NotNil(t, resp)
}

// ---- descriptors ----

func TestDescriptors_V2(t *testing.T) {
	t.Parallel()
	svcCtx := setupTestServiceContext(t)
	ctx := context.Background()
	createTestFunction(t, svcCtx.DB, "desc1", "Descriptor 1")

	resp, err := descriptors(ctx, svcCtx, &DescriptorsRequest{GameId: "test-game"})
	require.NoError(t, err)
	assert.NotNil(t, resp)
}

// ---- functionPermissions ----

func TestFunctionPermissions_V2(t *testing.T) {
	t.Parallel()
	svcCtx := setupTestServiceContext(t)
	ctx := context.Background()
	createTestFunction(t, svcCtx.DB, "perm1", "Perm 1")

	resp, err := functionPermissions(ctx, svcCtx, &FunctionPermissionsRequest{ID: "perm1"})
	require.NoError(t, err)
	assert.NotNil(t, resp)
}

// ---- functionPermissionsUpdate ----

func TestFunctionPermissionsUpdate_V2(t *testing.T) {
	t.Parallel()
	svcCtx := setupTestServiceContext(t)
	ctx := context.Background()
	createTestFunction(t, svcCtx.DB, "permupd1", "Perm Update 1")

	err := functionPermissionsUpdate(ctx, svcCtx, &FunctionPermissionsUpdateRequest{
		ID: "permupd1",
		Permissions: []FunctionPermission{
			{Resource: "function", Actions: []string{"invoke"}, Roles: []string{"admin"}},
		},
	})
	require.NoError(t, err)
}

// ---- functionInstancesAll ----

func TestFunctionInstancesAll_V2(t *testing.T) {
	t.Parallel()
	svcCtx := setupTestServiceContext(t)
	ctx := context.Background()

	resp, err := functionInstancesAll(ctx, svcCtx, &FunctionInstancesAllRequest{})
	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Empty(t, resp.Instances)
}
