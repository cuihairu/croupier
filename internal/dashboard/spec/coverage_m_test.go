package spec

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// 组 M 补测：publish 校验 / selector 参数校验 / sync-selectors planner
// 剩余未覆盖分支（static 区块矩阵、transform 参数防线、输出重推导阶梯）。
// ---------------------------------------------------------------------------

func mStaticSection(key string, bindingID string, view string, form *FormPresentationSpec) CompositeSection {
	return CompositeSection{
		Key:       key,
		BindingID: bindingID,
		View:      view,
		Static:    true,
		Form:      form,
	}
}

func mDiagCodes(diags []Diagnostic) []string {
	codes := make([]string, 0, len(diags))
	for _, diag := range diags {
		codes = append(codes, diag.Code)
	}
	return codes
}

// 常量表单（static）区块的四类发布阻断 + 干净用例。
func TestValidatePublishableCompositePageStaticSections(t *testing.T) {
	page := PageSpec{
		Type: PageTypeComposite,
		Composite: &CompositePageSpec{Sections: []CompositeSection{
			// static 却引用 binding → 冲突
			mStaticSection("s1", "b1", "form", &FormPresentationSpec{JSONSchema: JSONSchema(`{"type":"object"}`)}),
			// static 但 view 不是 form
			mStaticSection("s2", "", "table", &FormPresentationSpec{JSONSchema: JSONSchema(`{"type":"object"}`)}),
			// static + form 视图但缺 Form 呈现
			mStaticSection("s3", "", "form", nil),
			// static + form 视图但 jsonSchema 为空
			mStaticSection("s4", "", "form", &FormPresentationSpec{JSONSchema: JSONSchema("")}),
		}},
	}
	codes := mDiagCodes(ValidatePublishablePageShape(page))
	assert.Contains(t, codes, "composite_section_static_binding_conflict")
	assert.Contains(t, codes, "composite_section_static_view_invalid")
	// s3 / s4 均命中 schema_missing（Form 为 nil 与 jsonSchema 长度为 0 两个入口）
	assert.Contains(t, codes, "composite_section_static_schema_missing")

	// 干净的 static 区块：不产出任何 static 相关诊断。
	clean := PageSpec{
		Type: PageTypeComposite,
		Composite: &CompositePageSpec{Sections: []CompositeSection{
			mStaticSection("only", "", "form", &FormPresentationSpec{
				JSONSchema: JSONSchema(`{"type":"object","properties":{"q":{"type":"string"}}}`),
			}),
		}},
	}
	assert.Empty(t, ValidatePublishablePageShape(clean))
}

// validRenameParams：映射表键/值防线（空键、空值、非字符串值）。
func TestValidRenameParamsGuardClauses(t *testing.T) {
	assert.False(t, validRenameParams(nil))
	assert.False(t, validRenameParams(&TransformSpec{}))
	// 空白 from 键：静默产生空对象，必须拒绝
	assert.False(t, validRenameParams(&TransformSpec{
		Params: map[string]json.RawMessage{"": json.RawMessage(`"to"`)},
	}))
	// 空白 to 值
	assert.False(t, validRenameParams(&TransformSpec{
		Params: map[string]json.RawMessage{"from": json.RawMessage(`"   "`)},
	}))
	// to 不是 JSON 字符串（unmarshal 失败）
	assert.False(t, validRenameParams(&TransformSpec{
		Params: map[string]json.RawMessage{"from": json.RawMessage(`1`)},
	}))
	assert.True(t, validRenameParams(&TransformSpec{
		Params: map[string]json.RawMessage{"from": json.RawMessage(`"to"`)},
	}))
}

// validDefaultParams：nil / 缺 value 键 / 有 value 键。
func TestValidDefaultParamsGuardClauses(t *testing.T) {
	assert.False(t, validDefaultParams(nil))
	assert.False(t, validDefaultParams(&TransformSpec{}))
	assert.True(t, validDefaultParams(&TransformSpec{
		Params: map[string]json.RawMessage{"value": json.RawMessage(`0`)},
	}))
}

// binding.Selectors 为 nil：输入侧直接空结果，输出侧仍走必需输出补齐。
func TestPlanBindingSelectorSyncNilSelectors(t *testing.T) {
	page := PageSpec{
		Type: PageTypeResource,
		Resource: &ResourcePageSpec{ListView: &ListViewSpec{
			RowSchema: JSONSchema(`{"type":"object","properties":{"id":{"type":"string"}}}`),
		}},
	}
	binding := PageFunctionBinding{
		ID:         "list",
		FunctionID: "player.list",
		Usage:      BindingUsageQuery,
		Execution:  PageBindingExecution{Mode: PageExecutionModeSync},
	}
	fn := FunctionSpec{
		ID:           "player.list",
		InputSchema:  JSONSchema(`{}`),
		OutputSchema: JSONSchema(`{"type":"object","properties":{"items":{"type":"array"}}}`),
	}

	report, synced := PlanBindingSelectorSync(page, binding, fn, SelectorSyncOptions{})

	assert.Empty(t, report.Input)
	require.NotNil(t, synced.Selectors)
	// 必需输出 items 缺失且推导不出 → manual_required
	entry, ok := findSyncOutputEntry(report, "items")
	require.True(t, ok)
	assert.Equal(t, SelectorSyncManual, entry.Action)
}

// identityTargetForPage：语义未命中（ok=false / 空字段）时回落空串。
func TestIdentityTargetForPageLookupMiss(t *testing.T) {
	page := PageSpec{ResourceKey: "players"}
	fn := FunctionSpec{}

	notFound := SelectorSyncOptions{IdentityOf: func(resourceKey string) (string, bool) {
		assert.Equal(t, "players", resourceKey)
		return "", false
	}}
	assert.Empty(t, identityTargetForPage(page, fn, notFound))

	blankField := SelectorSyncOptions{IdentityOf: func(string) (string, bool) {
		return "  ", true
	}}
	assert.Empty(t, identityTargetForPage(page, fn, blankField))

	// resourceKey 回退函数声明的 Resource
	fallback := SelectorSyncOptions{IdentityOf: func(resourceKey string) (string, bool) {
		assert.Equal(t, "rewards", resourceKey)
		return "reward_id", true
	}}
	assert.Equal(t, "/reward_id", identityTargetForPage(PageSpec{}, FunctionSpec{Resource: "rewards"}, fallback))
}

// rechooseInputTarget：schema 非法降级 + 启发式各 continue 分支 + 唯一命中。
func TestRechooseInputTargetHeuristicLadder(t *testing.T) {
	source := ValueSource{Kind: SourceForm, Path: "/name"}
	ctx := SelectorContext{FormSchema: JSONSchema(`{"type":"object","properties":{"name":{"type":"string"}}}`)}
	assignment := InputAssignment{Target: "/old", Source: source}

	// 新 schema 非法 JSON：快照失败，直接无候选
	_, _, _, ok := rechooseInputTarget(assignment, JSONSchema(`{not-json`), SchemaDiffResult{}, map[string]struct{}{}, ctx, SelectorSyncOptions{})
	assert.False(t, ok)

	// 启发式：同父指针下 occupied / 类型不可赋值 / 不同父 全部跳过，唯一命中 /new2
	schema := JSONSchema(`{
		"type":"object",
		"properties":{
			"new1":{"type":"string"},
			"new2":{"type":"string"},
			"count":{"type":"integer"},
			"sub":{"type":"object","properties":{"leaf":{"type":"string"}}}
		}
	}`)
	newTarget, confidence, _, ok := rechooseInputTarget(
		assignment, schema, SchemaDiffResult{}, map[string]struct{}{"/new1": {}}, ctx, SelectorSyncOptions{},
	)
	require.True(t, ok)
	assert.Equal(t, "/new2", newTarget)
	assert.Equal(t, SelectorSyncConfidenceLow, confidence)
}

// filterUnoccupied：被占用的 rename 候选被过滤。
func TestFilterUnoccupiedDirect(t *testing.T) {
	got := filterUnoccupied([]FieldRenameCandidate{
		{OldPath: "/a", NewPath: "/x"},
		{OldPath: "/b", NewPath: "/y"},
	}, map[string]struct{}{"/x": {}})
	require.Len(t, got, 1)
	assert.Equal(t, "/y", got[0].NewPath)
}

// requiredOutputStateKey 必需输出矩阵与 ValidateRequiredOutputAssignments 同步。
func TestRequiredOutputStateKeyMatrix(t *testing.T) {
	cases := []struct {
		pageType PageType
		usage    PageBindingUsage
		stateKey string
		shape    OutputResultShape
	}{
		{PageTypeResource, BindingUsageQuery, "items", OutputShapeCollection},
		{PageTypeResource, BindingUsageDetail, "detail", OutputShapeObject},
		{PageTypeReport, BindingUsageReport, "dataset", OutputShapeDataset},
		{PageTypeTask, BindingUsageTaskStatus, "taskStatus", OutputShapeObject},
		{PageTypeTask, BindingUsageTaskEvents, "taskEvents", OutputShapeCollection},
		{PageTypeTask, BindingUsageTaskResult, "taskResult", ""},
		{PageTypeOperation, BindingUsageAction, "", ""},
	}
	for _, tc := range cases {
		stateKey, shape := requiredOutputStateKey(PageSpec{Type: tc.pageType}, PageFunctionBinding{Usage: tc.usage})
		assert.Equal(t, tc.stateKey, stateKey, "%s/%s", tc.pageType, tc.usage)
		assert.Equal(t, tc.shape, shape, "%s/%s", tc.pageType, tc.usage)
	}
}

// 输出同步阶梯（经 PlanBindingSelectorSync 端到端验证）。
func mSyncResourceQueryBinding(output []OutputAssignment) (PageSpec, PageFunctionBinding) {
	page := PageSpec{
		Type: PageTypeResource,
		Resource: &ResourcePageSpec{ListView: &ListViewSpec{
			RowSchema: JSONSchema(`{"type":"object","properties":{"id":{"type":"string"}}}`),
		}},
	}
	binding := PageFunctionBinding{
		ID:         "list",
		FunctionID: "player.list",
		Usage:      BindingUsageQuery,
		Selectors:  &BindingSelectors{Output: output},
		Execution:  PageBindingExecution{Mode: PageExecutionModeSync},
	}
	return page, binding
}

// 必需输出 shape 漂移 + generator 默认推导命中 → renamed（重接到新数组字段）。
func TestSyncOutputRequiredShapeMismatchRenamed(t *testing.T) {
	page, binding := mSyncResourceQueryBinding([]OutputAssignment{
		{StateKey: "items", Source: "/rows", Shape: OutputShapeCollection},
	})
	fn := FunctionSpec{
		ID:          "player.list",
		InputSchema: JSONSchema(`{}`),
		OutputSchema: JSONSchema(
			`{"type":"object","properties":{"rows":{"type":"object"},"list":{"type":"array","items":{"type":"object"}}}}`,
		),
	}
	opts := SelectorSyncOptions{
		RecomputeDefaults: func(usage PageBindingUsage, outputSchema JSONSchema) []OutputAssignment {
			return []OutputAssignment{{StateKey: "items", Source: "/list", Shape: OutputShapeCollection}}
		},
	}

	report, synced := PlanBindingSelectorSync(page, binding, fn, opts)

	entry, ok := findSyncOutputEntry(report, "items")
	require.True(t, ok)
	assert.Equal(t, SelectorSyncRenamed, entry.Action)
	assert.Equal(t, "/list", entry.NewSource)
	// 重推导改写了 source（apply 会落库），Changed 必须置位——否则
	// dry-run 报告"无变更"而 apply 实际写入了变更。
	assert.True(t, report.Changed)
	require.Len(t, synced.Selectors.Output, 1)
	assert.Equal(t, "/list", synced.Selectors.Output[0].Source)
}

// 非必需输出 shape 漂移且无法安全标量化（exotic type）→ 保留 + type_changed。
func TestSyncOutputNonRequiredExoticTypeKept(t *testing.T) {
	page, binding := mSyncResourceQueryBinding([]OutputAssignment{
		{StateKey: "misc", Source: "/misc", Shape: OutputShapeTask},
	})
	fn := FunctionSpec{
		ID:          "player.list",
		InputSchema: JSONSchema(`{}`),
		OutputSchema: JSONSchema(
			`{"type":"object","properties":{"misc":{"type":"null"}}}`,
		),
	}

	report, synced := PlanBindingSelectorSync(page, binding, fn, SelectorSyncOptions{})

	entry, ok := findSyncOutputEntry(report, "misc")
	require.True(t, ok)
	assert.Equal(t, SelectorSyncTypeChanged, entry.Action)
	assert.False(t, entry.Required)
	require.Len(t, synced.Selectors.Output, 1)
	assert.Equal(t, OutputShapeTask, synced.Selectors.Output[0].Shape)
	assert.Equal(t, "/misc", synced.Selectors.Output[0].Source)
}

// 必需输出 shape 漂移且推导不出 → 保留原映射 + manual_required（绝不摘成缺失）。
func TestSyncOutputRequiredShapeMismatchManual(t *testing.T) {
	page, binding := mSyncResourceQueryBinding([]OutputAssignment{
		{StateKey: "items", Source: "/rows", Shape: OutputShapeCollection},
	})
	fn := FunctionSpec{
		ID:          "player.list",
		InputSchema: JSONSchema(`{}`),
		OutputSchema: JSONSchema(
			`{"type":"object","properties":{"rows":{"type":"object"}}}`,
		),
	}

	report, synced := PlanBindingSelectorSync(page, binding, fn, SelectorSyncOptions{})

	entry, ok := findSyncOutputEntry(report, "items")
	require.True(t, ok)
	assert.Equal(t, SelectorSyncManual, entry.Action)
	require.Len(t, synced.Selectors.Output, 1)
	assert.Equal(t, "/rows", synced.Selectors.Output[0].Source)
}

// rechooseOutputSource 直接覆盖 rename/默认推导阶梯内的 continue 分支。
func TestRechooseOutputSourceLadderContinues(t *testing.T) {
	schema := JSONSchema(`{"type":"object","properties":{"b":{"type":"array"}}}`)
	assignment := OutputAssignment{StateKey: "items", Source: "/a", Shape: OutputShapeCollection}

	// rename 候选 OldPath 与 assignment.Source 不匹配 → continue
	_, _, _, ok := rechooseOutputSource(BindingUsageQuery, assignment, schema, SchemaDiffResult{
		RenameCandidates: []FieldRenameCandidate{{OldPath: "/zz", NewPath: "/b"}},
	}, OutputShapeCollection, SelectorSyncOptions{}, false)
	assert.False(t, ok)

	// rename 候选 NewPath 不在新 schema 中 → continue
	_, _, _, ok = rechooseOutputSource(BindingUsageQuery, assignment, schema, SchemaDiffResult{
		RenameCandidates: []FieldRenameCandidate{{OldPath: "/a", NewPath: "/gone"}},
	}, OutputShapeCollection, SelectorSyncOptions{}, false)
	assert.False(t, ok)

	// 默认推导条目 shape 与 reqShape 不符 → continue
	_, _, _, ok = rechooseOutputSource(BindingUsageQuery, assignment, schema, SchemaDiffResult{},
		OutputShapeCollection, SelectorSyncOptions{
			RecomputeDefaults: func(PageBindingUsage, JSONSchema) []OutputAssignment {
				return []OutputAssignment{{StateKey: "items", Source: "/b", Shape: OutputShapeObject}}
			},
		}, false)
	assert.False(t, ok)

	// 默认推导条目 source 不在新 schema 中 → continue
	_, _, _, ok = rechooseOutputSource(BindingUsageQuery, assignment, schema, SchemaDiffResult{},
		OutputShapeCollection, SelectorSyncOptions{
			RecomputeDefaults: func(PageBindingUsage, JSONSchema) []OutputAssignment {
				return []OutputAssignment{{StateKey: "items", Source: "/missing", Shape: OutputShapeCollection}}
			},
		}, false)
	assert.False(t, ok)

	// 默认推导命中：source 换成 /b
	updated, confidence, _, ok := rechooseOutputSource(BindingUsageQuery, assignment, schema, SchemaDiffResult{},
		OutputShapeCollection, SelectorSyncOptions{
			RecomputeDefaults: func(PageBindingUsage, JSONSchema) []OutputAssignment {
				return []OutputAssignment{{StateKey: "items", Source: "/b", Shape: OutputShapeCollection}}
			},
		}, false)
	require.True(t, ok)
	assert.Equal(t, "/b", updated.Source)
	assert.Equal(t, SelectorSyncConfidenceLow, confidence)
}

// scalarizedOutputShape：安全 shape 标量化矩阵。
func TestScalarizedOutputShapeMatrix(t *testing.T) {
	schema := JSONSchema(`{
		"type":"object",
		"properties":{
			"s":{"type":"string"},
			"n":{"type":"number"},
			"b":{"type":"boolean"},
			"o":{"type":"object"},
			"a":{"type":"array"},
			"nil":{"type":"null"},
			"untyped":{}
		}
	}`)
	cases := []struct {
		source string
		shape  OutputResultShape
		ok     bool
	}{
		{"/a", OutputShapeCollection, true},
		{"/o", OutputShapeObject, true},
		{"/s", OutputShapeScalar, true},
		{"/n", OutputShapeScalar, true},
		{"/b", OutputShapeScalar, true},
		{"/nil", "", false},
		{"/untyped", "", false},
		{"/missing", "", false},
	}
	for _, tc := range cases {
		shape, ok := scalarizedOutputShape(schema, tc.source)
		assert.Equal(t, tc.shape, shape, tc.source)
		assert.Equal(t, tc.ok, ok, tc.source)
	}
}
