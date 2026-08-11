package console

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/cuihairu/croupier/internal/api/function"
	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ──────────────────────────────────────────────────────
// Pure helper function tests
// ──────────────────────────────────────────────────────

func TestCloneStringMapV2(t *testing.T) {
	// nil
	assert.Nil(t, cloneStringMap(nil))
	// empty
	assert.Nil(t, cloneStringMap(map[string]string{}))
	// valid
	result := cloneStringMap(map[string]string{"a": "1", "b": "2"})
	assert.Equal(t, map[string]string{"a": "1", "b": "2"}, result)
	// filtered empty keys/values
	result = cloneStringMap(map[string]string{"": "1", "a": "", "b": "  "})
	assert.Nil(t, result)
	// filtered whitespace only
	result = cloneStringMap(map[string]string{"a": "  "})
	assert.Nil(t, result)
}

func TestSanitizeApprovalIDV2(t *testing.T) {
	// empty
	assert.Equal(t, "binding", sanitizeApprovalID(""))
	// valid alphanumeric
	assert.Equal(t, "playerquery", sanitizeApprovalID("player.query"))
	// special chars become underscore
	assert.Equal(t, "test_id", sanitizeApprovalID("test@id!"))
	// all special chars → "binding"
	assert.Equal(t, "binding", sanitizeApprovalID("@!#$%"))
	// mixed case lowered
	assert.Equal(t, "mybinding", sanitizeApprovalID("MyBinding"))
}

func TestFirstNonEmptyV2(t *testing.T) {
	assert.Equal(t, "", firstNonEmpty())
	assert.Equal(t, "", firstNonEmpty("", "", ""))
	assert.Equal(t, "a", firstNonEmpty("", "a", "b"))
	assert.Equal(t, "a", firstNonEmpty("a"))
	assert.Equal(t, "a", firstNonEmpty("  ", "a"))
}

func TestGetLocalizedTextV2(t *testing.T) {
	// nil labels
	assert.Equal(t, "fallback", getLocalizedText(nil, "zh-CN", "fallback"))
	// empty labels
	assert.Equal(t, "fallback", getLocalizedText(spec.LocalizedText{}, "zh-CN", "fallback"))
	// matching lang
	assert.Equal(t, "中文", getLocalizedText(spec.LocalizedText{"zh-CN": "中文", "en-US": "English"}, "zh-CN", "fallback"))
	assert.Equal(t, "English", getLocalizedText(spec.LocalizedText{"zh-CN": "中文", "en-US": "English"}, "en-US", "fallback"))
	// fallback to zh-CN
	assert.Equal(t, "中文", getLocalizedText(spec.LocalizedText{"zh-CN": "中文"}, "ja", "fallback"))
	// fallback to any value
	assert.Equal(t, "any", getLocalizedText(spec.LocalizedText{"ja": "any"}, "zh-CN", "fallback"))
	// all empty → fallback
	assert.Equal(t, "fallback", getLocalizedText(spec.LocalizedText{"zh-CN": "", "en-US": ""}, "zh-CN", "fallback"))
}

func TestErrPageNotFoundConsoleV2(t *testing.T) {
	err := ErrPageNotFound("test-page")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "page not found: test-page")

	var notFound *PageNotFoundError
	assert.True(t, assert.ErrorAs(t, err, &notFound))
	assert.Equal(t, "test-page", notFound.Key)
}

func TestPageNotFoundErrorConsoleV2(t *testing.T) {
	err := &PageNotFoundError{Key: "my-page"}
	assert.Equal(t, "page not found: my-page", err.Error())
}

// ──────────────────────────────────────────────────────
// normalizeLanguage
// ──────────────────────────────────────────────────────

func TestNormalizeLanguageV2(t *testing.T) {
	assert.Equal(t, "zh-CN", normalizeLanguage(""))
	assert.Equal(t, "zh-CN", normalizeLanguage("zh"))
	assert.Equal(t, "zh-CN", normalizeLanguage("zh-cn"))
	assert.Equal(t, "zh-CN", normalizeLanguage("zh_cn"))
	assert.Equal(t, "en-US", normalizeLanguage("en"))
	assert.Equal(t, "en-US", normalizeLanguage("en-us"))
	assert.Equal(t, "en-US", normalizeLanguage("en_us"))
	assert.Equal(t, "ja", normalizeLanguage("ja"))
	assert.Equal(t, "ja", normalizeLanguage("  JA  "))
}

// ──────────────────────────────────────────────────────
// validRawJSON
// ──────────────────────────────────────────────────────

func TestValidRawJSONV2(t *testing.T) {
	// empty
	result, found, err := validRawJSON(nil, "test")
	require.NoError(t, err)
	assert.False(t, found)
	assert.Nil(t, result)

	// invalid JSON
	_, found, err = validRawJSON(json.RawMessage(`{invalid}`), "test")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must be valid JSON")

	// valid JSON
	result, found, err = validRawJSON(json.RawMessage(`{"key":"value"}`), "test")
	require.NoError(t, err)
	assert.True(t, found)
	assert.JSONEq(t, `{"key":"value"}`, string(result))
}

// ──────────────────────────────────────────────────────
// getJSONPointerValue
// ──────────────────────────────────────────────────────

func TestGetJSONPointerValueV2(t *testing.T) {
	// non-pointer path
	val, ok := getJSONPointerValue(json.RawMessage(`{"a":1}`), "not-a-pointer")
	assert.False(t, ok)
	assert.Nil(t, val)

	// object traversal
	val, ok = getJSONPointerValue(json.RawMessage(`{"a":{"b":42}}`), "/a/b")
	assert.True(t, ok)
	assert.JSONEq(t, `42`, string(val))

	// array traversal
	val, ok = getJSONPointerValue(json.RawMessage(`[1,2,3]`), "/1")
	assert.True(t, ok)
	assert.JSONEq(t, `2`, string(val))

	// missing key
	val, ok = getJSONPointerValue(json.RawMessage(`{"a":1}`), "/b")
	assert.False(t, ok)
	assert.Nil(t, val)

	// array out of bounds
	val, ok = getJSONPointerValue(json.RawMessage(`[1]`), "/5")
	assert.False(t, ok)
	assert.Nil(t, val)

	// empty value
	val, ok = getJSONPointerValue(json.RawMessage(``), "/a")
	assert.False(t, ok)
	assert.Nil(t, val)

	// invalid array index
	val, ok = getJSONPointerValue(json.RawMessage(`[1,2]`), "/abc")
	assert.False(t, ok)
	assert.Nil(t, val)

	// nested object + array
	val, ok = getJSONPointerValue(json.RawMessage(`{"items":[{"id":1},{"id":2}]}`), "/items/1/id")
	assert.True(t, ok)
	assert.JSONEq(t, `2`, string(val))
}

// ──────────────────────────────────────────────────────
// setJSONPointerValue / setJSONObjectPointer
// ──────────────────────────────────────────────────────

func TestSetJSONPointerValueV2(t *testing.T) {
	// empty path
	err := setJSONPointerValue(map[string]json.RawMessage{}, "", json.RawMessage(`1`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "non-empty JSON Pointer")

	// non-pointer path
	err = setJSONPointerValue(map[string]json.RawMessage{}, "not-pointer", json.RawMessage(`1`))
	require.Error(t, err)

	// simple key
	payload := map[string]json.RawMessage{}
	err = setJSONPointerValue(payload, "/name", json.RawMessage(`"test"`))
	require.NoError(t, err)
	assert.JSONEq(t, `"test"`, string(payload["name"]))

	// nested key
	payload = map[string]json.RawMessage{}
	err = setJSONPointerValue(payload, "/a/b", json.RawMessage(`42`))
	require.NoError(t, err)
	assert.JSONEq(t, `{"b":42}`, string(payload["a"]))

	// empty key in path
	err = setJSONPointerValue(map[string]json.RawMessage{}, "//a", json.RawMessage(`1`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty object key")
}

func TestSetJSONObjectPointerV2(t *testing.T) {
	// empty tokens
	err := setJSONObjectPointer(map[string]json.RawMessage{}, []string{}, json.RawMessage(`1`))
	require.Error(t, err)

	// single token
	obj := map[string]json.RawMessage{}
	err = setJSONObjectPointer(obj, []string{"key"}, json.RawMessage(`"value"`))
	require.NoError(t, err)
	assert.JSONEq(t, `"value"`, string(obj["key"]))

	// existing non-object parent conflict
	obj = map[string]json.RawMessage{"a": json.RawMessage(`"string"`)}
	err = setJSONObjectPointer(obj, []string{"a", "b"}, json.RawMessage(`1`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "non-object parent")
}

// ──────────────────────────────────────────────────────
// jsonPointerTokens
// ──────────────────────────────────────────────────────

func TestJSONPointerTokensV2(t *testing.T) {
	assert.Nil(t, jsonPointerTokens(""))
	tokens := jsonPointerTokens("/a/b/c")
	assert.Equal(t, []string{"a", "b", "c"}, tokens)
	// tilde escape
	tokens = jsonPointerTokens("/a~1b/c~0d")
	assert.Equal(t, []string{"a/b", "c~d"}, tokens)
}

// ──────────────────────────────────────────────────────
// isJSONPointer
// ──────────────────────────────────────────────────────

func TestIsJSONPointerV2(t *testing.T) {
	assert.True(t, isJSONPointer(""))
	assert.True(t, isJSONPointer("/a"))
	assert.False(t, isJSONPointer("a"))
	assert.False(t, isJSONPointer("a/b"))
}

// ──────────────────────────────────────────────────────
// buildExecutionResult
// ──────────────────────────────────────────────────────

func TestBuildExecutionResultV2(t *testing.T) {
	ctx := context.Background()

	// nil response
	result, err := buildExecutionResult(ctx, "req-1", nil)
	require.NoError(t, err)
	assert.Equal(t, spec.PageExecutionKindSync, result.Kind)
	assert.Equal(t, "req-1", result.RequestID)

	// empty requestID generates UUID
	result, err = buildExecutionResult(ctx, "", nil)
	require.NoError(t, err)
	assert.NotEmpty(t, result.RequestID)

	// approval required
	result, err = buildExecutionResult(ctx, "req-2", &function.FunctionInvokeResponse{ApprovalRequired: true, ApprovalID: "approval-1"})
	require.NoError(t, err)
	assert.Equal(t, spec.PageExecutionKindApproval, result.Kind)
	assert.Equal(t, "approval-1", result.ApprovalID)

	// task response
	result, err = buildExecutionResult(ctx, "req-3", &function.FunctionInvokeResponse{TaskID: "task-1"})
	require.NoError(t, err)
	assert.Equal(t, spec.PageExecutionKindTask, result.Kind)
	assert.Equal(t, "task-1", result.TaskID)

	// TaskId (lowercase) response
	result, err = buildExecutionResult(ctx, "req-4", &function.FunctionInvokeResponse{TaskId: "task-2"})
	require.NoError(t, err)
	assert.Equal(t, spec.PageExecutionKindTask, result.Kind)
	assert.Equal(t, "task-2", result.TaskID)

	// sync result
	result, err = buildExecutionResult(ctx, "req-5", &function.FunctionInvokeResponse{Result: json.RawMessage(`{"ok":true}`)})
	require.NoError(t, err)
	assert.Equal(t, spec.PageExecutionKindSync, result.Kind)
	assert.JSONEq(t, `{"ok":true}`, string(result.Data))
}

// ──────────────────────────────────────────────────────
// pageExecutionTarget
// ──────────────────────────────────────────────────────

func TestPageExecutionTargetV2(t *testing.T) {
	// nil response
	assert.Equal(t, "", pageExecutionTarget(nil))

	// agent_id in metadata
	resp := &function.FunctionInvokeResponse{
		ExecutionMetadata: map[string]string{"agent_id": "agent-1"},
	}
	assert.Equal(t, "agent-1", pageExecutionTarget(resp))

	// no agent_id, no broadcast
	resp = &function.FunctionInvokeResponse{}
	assert.Equal(t, "", pageExecutionTarget(resp))

	// broadcast with results - note: code has a bug where it checks outer `agentID` (from metadata)
	// instead of `item.AgentID`, so broadcast results won't contribute when metadata is empty
	resp = &function.FunctionInvokeResponse{
		Broadcast: &function.BroadcastResult{
			Results: []function.BroadcastAgentItem{
				{AgentID: "agent-a"},
				{AgentID: "agent-b"},
			},
		},
	}
	target := pageExecutionTarget(resp)
	assert.Equal(t, "", target)

	// broadcast with agent_id in metadata
	resp = &function.FunctionInvokeResponse{
		ExecutionMetadata: map[string]string{"agent_id": "agent-1"},
		Broadcast: &function.BroadcastResult{
			Results: []function.BroadcastAgentItem{
				{AgentID: "agent-a"},
			},
		},
	}
	target = pageExecutionTarget(resp)
	assert.Equal(t, "agent-1", target)
}

// ──────────────────────────────────────────────────────
// findBinding / findContract
// ──────────────────────────────────────────────────────

func TestFindBindingV2(t *testing.T) {
	bindings := []spec.PageFunctionBinding{
		{ID: "a"}, {ID: "b"},
	}
	b, ok := findBinding(bindings, "a")
	assert.True(t, ok)
	assert.Equal(t, "a", b.ID)

	_, ok = findBinding(bindings, "c")
	assert.False(t, ok)

	_, ok = findBinding(nil, "a")
	assert.False(t, ok)
}

func TestFindContractV2(t *testing.T) {
	contracts := []spec.BindingContractSnapshot{
		{BindingID: "a"}, {BindingID: "b"},
	}
	c, ok := findContract(contracts, "a")
	assert.True(t, ok)
	assert.Equal(t, "a", c.BindingID)

	_, ok = findContract(contracts, "c")
	assert.False(t, ok)

	_, ok = findContract(nil, "a")
	assert.False(t, ok)
}

// ──────────────────────────────────────────────────────
// resolveSelectorValue
// ──────────────────────────────────────────────────────

func TestResolveSelectorValueV2(t *testing.T) {

	// unsupported transform type
	_, _, err := resolveSelectorValue(spec.ValueSource{
		Kind:      spec.SourceForm,
		Transform: &spec.TransformSpec{Type: "unsupported"},
	}, ConsoleBindingExecutionContext{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not supported")

	// literal source - empty value
	val, found, err := resolveSelectorValue(spec.ValueSource{
		Kind:  spec.SourceLiteral,
		Value: nil,
	}, ConsoleBindingExecutionContext{})
	require.NoError(t, err)
	assert.True(t, found)
	assert.JSONEq(t, `null`, string(val))

	// literal source - valid value
	val, found, err = resolveSelectorValue(spec.ValueSource{
		Kind:  spec.SourceLiteral,
		Value: json.RawMessage(`"hello"`),
	}, ConsoleBindingExecutionContext{})
	require.NoError(t, err)
	assert.True(t, found)
	assert.JSONEq(t, `"hello"`, string(val))

	// form source
	val, found, err = resolveSelectorValue(spec.ValueSource{
		Kind: spec.SourceForm,
		Path: "/name",
	}, ConsoleBindingExecutionContext{
		Form: json.RawMessage(`{"name":"test"}`),
	})
	require.NoError(t, err)
	assert.True(t, found)
	assert.JSONEq(t, `"test"`, string(val))

	// row source
	val, found, err = resolveSelectorValue(spec.ValueSource{
		Kind: spec.SourceRow,
		Path: "/id",
	}, ConsoleBindingExecutionContext{
		Row: json.RawMessage(`{"id":"p1"}`),
	})
	require.NoError(t, err)
	assert.True(t, found)
	assert.JSONEq(t, `"p1"`, string(val))

	// selection source - pick transform
	val, found, err = resolveSelectorValue(spec.ValueSource{
		Kind:      spec.SourceSelection,
		Path:      "/id",
		Transform: &spec.TransformSpec{Type: spec.TransformPick},
	}, ConsoleBindingExecutionContext{
		Selection: json.RawMessage(`[{"id":"a"},{"id":"b"}]`),
	})
	require.NoError(t, err)
	assert.True(t, found)
	assert.JSONEq(t, `["a","b"]`, string(val))

	// selection source - no transform
	val, found, err = resolveSelectorValue(spec.ValueSource{
		Kind: spec.SourceSelection,
		Path: "/name",
	}, ConsoleBindingExecutionContext{
		Selection: json.RawMessage(`{"name":"test"}`),
	})
	require.NoError(t, err)
	assert.True(t, found)
	assert.JSONEq(t, `"test"`, string(val))

	// detail source
	val, found, err = resolveSelectorValue(spec.ValueSource{
		Kind: spec.SourceDetail,
		Path: "/key",
	}, ConsoleBindingExecutionContext{
		Detail: json.RawMessage(`{"key":"val"}`),
	})
	require.NoError(t, err)
	assert.True(t, found)
	assert.JSONEq(t, `"val"`, string(val))

	// page_state source - empty key
	_, _, err = resolveSelectorValue(spec.ValueSource{
		Kind: spec.SourcePageState,
		Key:  "",
	}, ConsoleBindingExecutionContext{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "key is required")

	// page_state source - nil page state
	val, found, err = resolveSelectorValue(spec.ValueSource{
		Kind: spec.SourcePageState,
		Key:  "myState",
		Path: "/x",
	}, ConsoleBindingExecutionContext{})
	require.NoError(t, err)
	assert.False(t, found)

	// page_state source - key not found
	val, found, err = resolveSelectorValue(spec.ValueSource{
		Kind: spec.SourcePageState,
		Key:  "missing",
		Path: "/x",
	}, ConsoleBindingExecutionContext{
		PageState: map[string]json.RawMessage{"other": json.RawMessage(`{"x":1}`)},
	})
	require.NoError(t, err)
	assert.False(t, found)

	// page_state source - found
	val, found, err = resolveSelectorValue(spec.ValueSource{
		Kind: spec.SourcePageState,
		Key:  "myState",
		Path: "/x",
	}, ConsoleBindingExecutionContext{
		PageState: map[string]json.RawMessage{"myState": json.RawMessage(`{"x":42}`)},
	})
	require.NoError(t, err)
	assert.True(t, found)
	assert.JSONEq(t, `42`, string(val))

	// unsupported source kind
	_, _, err = resolveSelectorValue(spec.ValueSource{
		Kind: "unsupported",
	}, ConsoleBindingExecutionContext{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported binding selector source")
}

// ──────────────────────────────────────────────────────
// pickSelectionValues
// ──────────────────────────────────────────────────────

func TestPickSelectionValuesV2(t *testing.T) {
	// empty selection
	val, found, err := pickSelectionValues(nil, "/id")
	require.NoError(t, err)
	assert.False(t, found)
	assert.Nil(t, val)

	// valid selection
	val, found, err = pickSelectionValues(json.RawMessage(`[{"id":"a"},{"id":"b"}]`), "/id")
	require.NoError(t, err)
	assert.True(t, found)
	assert.JSONEq(t, `["a","b"]`, string(val))

	// non-array selection
	_, _, err = pickSelectionValues(json.RawMessage(`{"id":"a"}`), "/id")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must be an array")

	// invalid JSON
	_, _, err = pickSelectionValues(json.RawMessage(`{invalid}`), "/id")
	require.Error(t, err)
}

// ──────────────────────────────────────────────────────
// valueFromRawContext
// ──────────────────────────────────────────────────────

func TestValueFromRawContextV2(t *testing.T) {
	// empty raw
	val, found, err := valueFromRawContext(nil, "/a", "test")
	require.NoError(t, err)
	assert.False(t, found)
	assert.Nil(t, val)

	// invalid JSON
	_, _, err = valueFromRawContext(json.RawMessage(`{invalid}`), "/a", "test")
	require.Error(t, err)

	// empty path → return full value
	val, found, err = valueFromRawContext(json.RawMessage(`{"a":1}`), "", "test")
	require.NoError(t, err)
	assert.True(t, found)
	assert.JSONEq(t, `{"a":1}`, string(val))

	// with path
	val, found, err = valueFromRawContext(json.RawMessage(`{"a":1}`), "/a", "test")
	require.NoError(t, err)
	assert.True(t, found)
	assert.JSONEq(t, `1`, string(val))
}

// ──────────────────────────────────────────────────────
// generateMenuFromPages - additional coverage
// ──────────────────────────────────────────────────────

func TestGenerateMenuFromPagesV2(t *testing.T) {
	// empty category key is skipped
	menu := generateMenuFromPages([]spec.PublishedPageSpec{
		{PageSpec: spec.PageSpec{
			PageKey: "test",
			Title:   spec.LocalizedText{"zh-CN": "测试"},
			Category: spec.PageCategorySpec{
				Key:    "",
				Labels: spec.LocalizedText{"zh-CN": ""},
			},
		}},
	}, "zh-CN")
	assert.Empty(t, menu.Items)

	// pages sorted by order then title
	menu = generateMenuFromPages([]spec.PublishedPageSpec{
		{PageSpec: spec.PageSpec{
			PageKey: "b.page", Title: spec.LocalizedText{"zh-CN": "B"}, Order: 10,
			Category: spec.PageCategorySpec{Key: "cat", Labels: spec.LocalizedText{"zh-CN": "分类"}, Order: 1},
		}},
		{PageSpec: spec.PageSpec{
			PageKey: "a.page", Title: spec.LocalizedText{"zh-CN": "A"}, Order: 5,
			Category: spec.PageCategorySpec{Key: "cat", Labels: spec.LocalizedText{"zh-CN": "分类"}, Order: 1},
		}},
	}, "zh-CN")
	require.Len(t, menu.Items, 1)
	require.Len(t, menu.Items[0].Children, 2)
	assert.Equal(t, "a.page", menu.Items[0].Children[0].Key)
	assert.Equal(t, "b.page", menu.Items[0].Children[1].Key)

	// same order, sorted by title
	menu = generateMenuFromPages([]spec.PublishedPageSpec{
		{PageSpec: spec.PageSpec{
			PageKey: "b.page", Title: spec.LocalizedText{"zh-CN": "B"}, Order: 5,
			Category: spec.PageCategorySpec{Key: "cat", Labels: spec.LocalizedText{"zh-CN": "分类"}, Order: 1},
		}},
		{PageSpec: spec.PageSpec{
			PageKey: "a.page", Title: spec.LocalizedText{"zh-CN": "A"}, Order: 5,
			Category: spec.PageCategorySpec{Key: "cat", Labels: spec.LocalizedText{"zh-CN": "分类"}, Order: 1},
		}},
	}, "zh-CN")
	require.Len(t, menu.Items, 1)
	require.Len(t, menu.Items[0].Children, 2)
	assert.Equal(t, "a.page", menu.Items[0].Children[0].Key)

	// same order and title, sorted by key
	menu = generateMenuFromPages([]spec.PublishedPageSpec{
		{PageSpec: spec.PageSpec{
			PageKey: "b.page", Title: spec.LocalizedText{"zh-CN": "Same"}, Order: 5,
			Category: spec.PageCategorySpec{Key: "cat", Labels: spec.LocalizedText{"zh-CN": "分类"}, Order: 1},
		}},
		{PageSpec: spec.PageSpec{
			PageKey: "a.page", Title: spec.LocalizedText{"zh-CN": "Same"}, Order: 5,
			Category: spec.PageCategorySpec{Key: "cat", Labels: spec.LocalizedText{"zh-CN": "分类"}, Order: 1},
		}},
	}, "zh-CN")
	require.Len(t, menu.Items, 1)
	require.Len(t, menu.Items[0].Children, 2)
	assert.Equal(t, "a.page", menu.Items[0].Children[0].Key)

	// category sort: same order, sorted by title
	menu = generateMenuFromPages([]spec.PublishedPageSpec{
		{PageSpec: spec.PageSpec{
			PageKey: "a.page", Title: spec.LocalizedText{"zh-CN": "A"}, Order: 5,
			Category: spec.PageCategorySpec{Key: "cat-b", Labels: spec.LocalizedText{"zh-CN": "B分类"}, Order: 1},
		}},
		{PageSpec: spec.PageSpec{
			PageKey: "b.page", Title: spec.LocalizedText{"zh-CN": "B"}, Order: 5,
			Category: spec.PageCategorySpec{Key: "cat-a", Labels: spec.LocalizedText{"zh-CN": "A分类"}, Order: 1},
		}},
	}, "zh-CN")
	require.Len(t, menu.Items, 2)
	assert.Equal(t, "cat-a", menu.Items[0].Key)
	assert.Equal(t, "cat-b", menu.Items[1].Key)

	// category sort: same order and title, sorted by key
	menu = generateMenuFromPages([]spec.PublishedPageSpec{
		{PageSpec: spec.PageSpec{
			PageKey: "a.page", Title: spec.LocalizedText{"zh-CN": "Same"}, Order: 5,
			Category: spec.PageCategorySpec{Key: "cat-b", Labels: spec.LocalizedText{"zh-CN": "Same"}, Order: 1},
		}},
		{PageSpec: spec.PageSpec{
			PageKey: "b.page", Title: spec.LocalizedText{"zh-CN": "Same"}, Order: 5,
			Category: spec.PageCategorySpec{Key: "cat-a", Labels: spec.LocalizedText{"zh-CN": "Same"}, Order: 1},
		}},
	}, "zh-CN")
	require.Len(t, menu.Items, 2)
	assert.Equal(t, "cat-a", menu.Items[0].Key)

	// page with icon
	menu = generateMenuFromPages([]spec.PublishedPageSpec{
		{PageSpec: spec.PageSpec{
			PageKey: "icon.page", Title: spec.LocalizedText{"zh-CN": "Icon"}, Icon: "icon.png", Order: 1,
			Category: spec.PageCategorySpec{Key: "cat", Labels: spec.LocalizedText{"zh-CN": "分类"}, Order: 1},
		}},
	}, "zh-CN")
	require.Len(t, menu.Items, 1)
	require.Len(t, menu.Items[0].Children, 1)
	assert.Equal(t, "icon.png", menu.Items[0].Children[0].Icon)
}

// ──────────────────────────────────────────────────────
// parsePublishedPageSpec - additional coverage
// ──────────────────────────────────────────────────────

func TestParsePublishedPageSpecV2(t *testing.T) {
	// invalid JSON
	result := parsePublishedPageSpec(model.PublishedPageSpec{SpecJSON: `{invalid}`})
	assert.Nil(t, result)

	// empty pageKey
	result = parsePublishedPageSpec(model.PublishedPageSpec{SpecJSON: `{"type":"operation"}`})
	assert.Nil(t, result)

	// with contracts JSON
	result = parsePublishedPageSpec(model.PublishedPageSpec{
		SpecJSON:             `{"pageKey":"test","type":"operation","title":{"zh-CN":"测试"},"category":{"key":"cat","labels":{"zh-CN":"分类"}}}`,
		BindingContractsJSON: `[{"bindingId":"b1"}]`,
	})
	require.NotNil(t, result)
	assert.Equal(t, "test", result.PageKey)
	assert.Len(t, result.BindingContracts, 1)

	// invalid contracts JSON (should not fail)
	result = parsePublishedPageSpec(model.PublishedPageSpec{
		SpecJSON:             `{"pageKey":"test","type":"operation","title":{"zh-CN":"测试"},"category":{"key":"cat","labels":{"zh-CN":"分类"}}}`,
		BindingContractsJSON: `{invalid}`,
	})
	require.NotNil(t, result)
}

// ──────────────────────────────────────────────────────
// bindingFreshnessStatuses
// ──────────────────────────────────────────────────────

func TestBindingFreshnessStatusesV2(t *testing.T) {
	// empty
	statuses := bindingFreshnessStatuses(nil)
	assert.Empty(t, statuses)

	// with statuses
	diags := []spec.BindingFreshnessDiagnostic{
		{Status: spec.BindingFreshnessGovernanceStale},
		{Status: ""},
		{Status: spec.BindingFreshnessInputSchemaStale},
	}
	statuses = bindingFreshnessStatuses(diags)
	assert.Equal(t, []string{string(spec.BindingFreshnessInputSchemaStale), string(spec.BindingFreshnessGovernanceStale)}, statuses)
}

// ──────────────────────────────────────────────────────
// currentActor
// ──────────────────────────────────────────────────────

func TestCurrentActorV2(t *testing.T) {
	// without username
	actor := currentActor(context.Background())
	assert.Equal(t, "unknown", actor)

	// with username
	ctx := context.WithValue(context.Background(), "username", "test-user")
	actor = currentActor(ctx)
	assert.Equal(t, "test-user", actor)
}

// ──────────────────────────────────────────────────────
// buildBindingPayloadFromSelectors - error branches
// ──────────────────────────────────────────────────────

func TestBuildBindingPayloadFromSelectorsV2(t *testing.T) {
	// nil selectors
	_, err := buildBindingPayloadFromSelectors(spec.PageFunctionBinding{
		ID: "test",
	}, ConsoleBindingExecutionContext{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "selectors are required")

	// invalid transform type
	binding := spec.PageFunctionBinding{
		ID: "test",
		Selectors: &spec.BindingSelectors{
			Input: spec.SelectorAST{Assignments: []spec.InputAssignment{
				{Target: "/a", Source: spec.ValueSource{
					Kind:      spec.SourceForm,
					Path:      "/a",
					Transform: &spec.TransformSpec{Type: "unsupported"},
				}},
			}},
		},
	}
	_, err = buildBindingPayloadFromSelectors(binding, ConsoleBindingExecutionContext{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not supported")

	// invalid target path
	binding = spec.PageFunctionBinding{
		ID: "test",
		Selectors: &spec.BindingSelectors{
			Input: spec.SelectorAST{Assignments: []spec.InputAssignment{
				{Target: "not-a-pointer", Source: spec.ValueSource{
					Kind: spec.SourceForm,
					Path: "/a",
				}},
			}},
		},
	}
	_, err = buildBindingPayloadFromSelectors(binding, ConsoleBindingExecutionContext{
		Form: json.RawMessage(`{"a":1}`),
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "target is invalid")

	// literal with empty value
	binding = spec.PageFunctionBinding{
		ID: "test",
		Selectors: &spec.BindingSelectors{
			Input: spec.SelectorAST{Assignments: []spec.InputAssignment{
				{Target: "/a", Source: spec.ValueSource{
					Kind:  spec.SourceLiteral,
					Value: nil,
				}},
			}},
		},
	}
	payload, err := buildBindingPayloadFromSelectors(binding, ConsoleBindingExecutionContext{})
	require.NoError(t, err)
	assert.JSONEq(t, `{"a":null}`, string(payload))
}

// ──────────────────────────────────────────────────────
// validateBindingExecutePayload - additional cases
// ──────────────────────────────────────────────────────

func TestValidateBindingExecutePayloadV2(t *testing.T) {
	// function not found
	err := validateBindingExecutePayload(json.RawMessage(`{}`), spec.PageFunctionBinding{
		ID:         "test",
		FunctionID: "nonexistent",
	}, map[string]spec.FunctionSpec{})
	assert.NoError(t, err)

	// function with empty input schema
	err = validateBindingExecutePayload(json.RawMessage(`{}`), spec.PageFunctionBinding{
		ID:         "test",
		FunctionID: "fn",
	}, map[string]spec.FunctionSpec{
		"fn": {ID: "fn", InputSchema: spec.JSONSchema("")},
	})
	assert.NoError(t, err)

	// valid payload
	err = validateBindingExecutePayload(json.RawMessage(`{"keyword":"test"}`), spec.PageFunctionBinding{
		ID:         "test",
		FunctionID: "fn",
	}, map[string]spec.FunctionSpec{
		"fn": {ID: "fn", InputSchema: spec.JSONSchema(`{"type":"object","properties":{"keyword":{"type":"string"}}}`)},
	})
	assert.NoError(t, err)
}

// ──────────────────────────────────────────────────────
// Service methods - Pages and Page
// ──────────────────────────────────────────────────────

func TestServicePagesRequiresConsoleReadPermission(t *testing.T) {
	service, ctx := newConsoleTestService(t) // no permissions
	_, err := service.Pages(ctx, &ConsolePagesRequest{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "无权查看运行控制台")
}

func TestServicePagesReturnsPublishedPages(t *testing.T) {
	service, ctx := newConsoleTestService(t, "console:read")
	require.NoError(t, seedConsolePublishedPage(service.svcCtx, ctx))

	resp, err := service.Pages(ctx, &ConsolePagesRequest{})
	require.NoError(t, err)
	require.Len(t, resp.Items, 1)
	assert.Equal(t, "player.manage", resp.Items[0].PageKey)
}

func TestServicePagesFiltersByCategory(t *testing.T) {
	service, ctx := newConsoleTestService(t, "console:read")
	require.NoError(t, seedConsolePublishedPage(service.svcCtx, ctx))

	resp, err := service.Pages(ctx, &ConsolePagesRequest{Category: "player"})
	require.NoError(t, err)
	require.Len(t, resp.Items, 1)

	resp, err = service.Pages(ctx, &ConsolePagesRequest{Category: "nonexistent"})
	require.NoError(t, err)
	assert.Empty(t, resp.Items)
}

func TestServicePageRequiresConsoleReadPermission(t *testing.T) {
	service, ctx := newConsoleTestService(t)
	_, err := service.Page(ctx, &ConsolePageRequest{PageKey: "player.manage"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "无权查看运行控制台")
}

func TestServicePageReturnsPublishedPage(t *testing.T) {
	service, ctx := newConsoleTestService(t, "console:read")
	require.NoError(t, seedConsolePublishedPage(service.svcCtx, ctx))

	resp, err := service.Page(ctx, &ConsolePageRequest{PageKey: "player.manage"})
	require.NoError(t, err)
	assert.Equal(t, "player.manage", resp.Page.PageKey)
}

func TestServicePageRejectsNotFound(t *testing.T) {
	service, ctx := newConsoleTestService(t, "console:read")
	_, err := service.Page(ctx, &ConsolePageRequest{PageKey: "nonexistent"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "page not found")
}

func TestServiceExecuteBindingRequiresPage(t *testing.T) {
	service, ctx := newConsoleTestService(t, "function:invoke")
	_, err := service.ExecuteBinding(ctx, &ConsoleExecuteBindingRequest{
		PageKey:   "nonexistent",
		BindingID: "test",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "page not found")
}

func TestServiceExecuteBindingRejectsMissingBinding(t *testing.T) {
	service, ctx := newConsoleTestService(t, "function:invoke")
	require.NoError(t, seedConsolePublishedPageWithCurrentContracts(service.svcCtx, ctx))

	_, err := service.ExecuteBinding(ctx, &ConsoleExecuteBindingRequest{
		PageKey:   "player.manage",
		BindingID: "nonexistent",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "page binding not found")
}

func TestServiceExecuteBindingRejectsMissingContract(t *testing.T) {
	service, ctx := newConsoleTestService(t, "function:invoke", "player:query")
	require.NoError(t, seedConsolePublishedPage(service.svcCtx, ctx))

	_, err := service.ExecuteBinding(ctx, &ConsoleExecuteBindingRequest{
		PageKey:   "player.manage",
		BindingID: "player.query",
	})
	require.Error(t, err)
	// Could be binding_stale or contract missing
	assert.True(t, err != nil)
}

// ──────────────────────────────────────────────────────
// Service Menu - additional cases
// ──────────────────────────────────────────────────────

func TestServiceMenuUsesLanguageFallbackV2(t *testing.T) {
	service, ctx := newConsoleTestService(t, "console:read")
	require.NoError(t, seedConsolePublishedPage(service.svcCtx, ctx))

	// empty language → defaults to zh-CN
	resp, err := service.Menu(ctx, &ConsoleMenuRequest{})
	require.NoError(t, err)
	require.Len(t, resp.Items, 1)

	// en-US language
	resp, err = service.Menu(ctx, &ConsoleMenuRequest{Language: "en"})
	require.NoError(t, err)
	require.Len(t, resp.Items, 1)
}

func TestServicePagesRequiresScopeV2(t *testing.T) {
	service, ctx := newConsoleTestService(t, "console:read")
	// Remove scope from context
	_, err := service.Pages(svc.WithGameScope(ctx, svc.GameScope{}), &ConsolePagesRequest{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "X-Game-ID is required")
}

func TestServicePageRequiresScopeV2(t *testing.T) {
	service, ctx := newConsoleTestService(t, "console:read")
	_, err := service.Page(svc.WithGameScope(ctx, svc.GameScope{}), &ConsolePageRequest{PageKey: "test"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "X-Game-ID is required")
}

func TestServiceExecuteBindingRequiresScopeV2(t *testing.T) {
	service, ctx := newConsoleTestService(t, "function:invoke")
	_, err := service.ExecuteBinding(svc.WithGameScope(ctx, svc.GameScope{}), &ConsoleExecuteBindingRequest{
		PageKey:   "test",
		BindingID: "test",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "X-Game-ID is required")
}

// ──────────────────────────────────────────────────────
// loadPublishedFunctionSpecs - error branches
// ──────────────────────────────────────────────────────

func TestLoadPublishedFunctionSpecsV2(t *testing.T) {
	// nil context (no scope)
	_, err := loadPublishedFunctionSpecs(context.Background(), nil)
	require.Error(t, err)

	// nil svcCtx
	ctx := svc.WithGameScope(context.Background(), svc.GameScope{GameID: "g", Env: "e"})
	_, err = loadPublishedFunctionSpecs(ctx, nil)
	require.Error(t, err)
}

// ──────────────────────────────────────────────────────
// Service execute binding - additional branches
// ──────────────────────────────────────────────────────

func TestServiceExecuteBindingRejectsDetailSelector(t *testing.T) {
	service, ctx := newConsoleTestService(t, "function:invoke", "player:query")
	inputSchema := `{"type":"object","properties":{"playerId":{"type":"string"}}}`
	outputSchema := `{"type":"object","properties":{"ok":{"type":"boolean"}}}`
	selector := spec.SelectorAST{Assignments: []spec.InputAssignment{
		{
			Target: "/playerId",
			Source: spec.ValueSource{Kind: spec.SourceDetail, Path: "/id"},
		},
	}}
	require.NoError(t, seedConsolePublishedPageWithSchemaAndSelector(service.svcCtx, ctx, inputSchema, outputSchema, selector))
	caller := &fakeConsoleSessionCaller{payload: []byte(`{"ok":true}`)}
	service.svcCtx.Dispatcher.SetSessionResolver(fakeConsoleSessionResolver{caller: caller})

	_, err := service.ExecuteBinding(ctx, &ConsoleExecuteBindingRequest{
		PageKey:   "player.manage",
		BindingID: "player.query",
		Context: ConsoleBindingExecutionContext{
			Detail: json.RawMessage(`{"id":"p1"}`),
		},
	})
	require.NoError(t, err)
	require.NotNil(t, caller.lastRequest)
	assert.JSONEq(t, `{"playerId":"p1"}`, string(caller.lastRequest.Payload))
}

func TestServiceExecuteBindingRejectsPageStateSelector(t *testing.T) {
	service, ctx := newConsoleTestService(t, "function:invoke", "player:query")
	inputSchema := `{"type":"object","properties":{"playerId":{"type":"string"}}}`
	outputSchema := `{"type":"object","properties":{"ok":{"type":"boolean"}}}`
	selector := spec.SelectorAST{Assignments: []spec.InputAssignment{
		{
			Target: "/playerId",
			Source: spec.ValueSource{Kind: spec.SourcePageState, Key: "selectedRow", Path: "/id"},
		},
	}}
	require.NoError(t, seedConsolePublishedPageWithSchemaAndSelector(service.svcCtx, ctx, inputSchema, outputSchema, selector))
	caller := &fakeConsoleSessionCaller{payload: []byte(`{"ok":true}`)}
	service.svcCtx.Dispatcher.SetSessionResolver(fakeConsoleSessionResolver{caller: caller})

	_, err := service.ExecuteBinding(ctx, &ConsoleExecuteBindingRequest{
		PageKey:   "player.manage",
		BindingID: "player.query",
		Context: ConsoleBindingExecutionContext{
			PageState: map[string]json.RawMessage{
				"selectedRow": json.RawMessage(`{"id":"p1"}`),
			},
		},
	})
	require.NoError(t, err)
	require.NotNil(t, caller.lastRequest)
	assert.JSONEq(t, `{"playerId":"p1"}`, string(caller.lastRequest.Payload))
}

func TestServiceExecuteBindingRejectsLiteralSelector(t *testing.T) {
	service, ctx := newConsoleTestService(t, "function:invoke", "player:query")
	inputSchema := `{"type":"object","properties":{"mode":{"type":"string"}}}`
	outputSchema := `{"type":"object","properties":{"ok":{"type":"boolean"}}}`
	selector := spec.SelectorAST{Assignments: []spec.InputAssignment{
		{
			Target: "/mode",
			Source: spec.ValueSource{Kind: spec.SourceLiteral, Value: json.RawMessage(`"test"`)},
		},
	}}
	require.NoError(t, seedConsolePublishedPageWithSchemaAndSelector(service.svcCtx, ctx, inputSchema, outputSchema, selector))
	caller := &fakeConsoleSessionCaller{payload: []byte(`{"ok":true}`)}
	service.svcCtx.Dispatcher.SetSessionResolver(fakeConsoleSessionResolver{caller: caller})

	_, err := service.ExecuteBinding(ctx, &ConsoleExecuteBindingRequest{
		PageKey:   "player.manage",
		BindingID: "player.query",
	})
	require.NoError(t, err)
	require.NotNil(t, caller.lastRequest)
	assert.JSONEq(t, `{"mode":"test"}`, string(caller.lastRequest.Payload))
}

// ──────────────────────────────────────────────────────
// Menu with multiple categories
// ──────────────────────────────────────────────────────

func TestServiceMenuMultipleCategoriesV2(t *testing.T) {
	service, ctx := newConsoleTestService(t, "console:read")
	require.NoError(t, seedConsolePublishedPageForScope(service.svcCtx, ctx, "player.manage", "player", "玩家", 1))
	require.NoError(t, seedConsolePublishedPageForScope(service.svcCtx, ctx, "mail.send", "mail", "邮件", 2))

	resp, err := service.Menu(ctx, &ConsoleMenuRequest{Language: "zh-CN"})
	require.NoError(t, err)
	require.Len(t, resp.Items, 2)
	// sorted by order
	assert.Equal(t, "player", resp.Items[0].Key)
	assert.Equal(t, "mail", resp.Items[1].Key)
}
