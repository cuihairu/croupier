package spec

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// sync-selectors planner 测试矩阵：输入/输出策略阶梯逐级覆盖。
// ---------------------------------------------------------------------------

func syncTestResourcePage(filters ...FilterSpec) PageSpec {
	return PageSpec{
		PageKey: "players",
		Type:    PageTypeResource,
		Title:   LocalizedText{"zh-CN": "玩家"},
		Resource: &ResourcePageSpec{
			ListView: &ListViewSpec{
				Filters: filters,
				RowSchema: JSONSchema(
					`{"type":"object","properties":{"uid":{"type":"string"},"name":{"type":"string"}}}`,
				),
			},
		},
	}
}

func syncTestTaskPage() PageSpec {
	return PageSpec{
		PageKey: "mail-send",
		Type:    PageTypeTask,
		Title:   LocalizedText{"zh-CN": "发邮件"},
		Task:    &TaskPageSpec{},
	}
}

func syncTestBinding(input []InputAssignment, output []OutputAssignment) PageFunctionBinding {
	return PageFunctionBinding{
		ID:         "list",
		FunctionID: "player.list",
		Usage:      BindingUsageQuery,
		Selectors: &BindingSelectors{
			Input:  SelectorAST{Assignments: input},
			Output: output,
		},
		Execution: PageBindingExecution{Mode: PageExecutionModeSync},
	}
}

func findSyncInputEntry(report BindingSelectorSyncReport, target string) (SelectorSyncInputEntry, bool) {
	for _, entry := range report.Input {
		if entry.Target == target {
			return entry, true
		}
	}
	return SelectorSyncInputEntry{}, false
}

func findSyncOutputEntry(report BindingSelectorSyncReport, stateKey string) (SelectorSyncOutputEntry, bool) {
	for _, entry := range report.Output {
		if entry.StateKey == stateKey {
			return entry, true
		}
	}
	return SelectorSyncOutputEntry{}, false
}

func syncAssignmentTargets(binding PageFunctionBinding) []string {
	out := make([]string, 0, len(binding.Selectors.Input.Assignments))
	for _, assignment := range binding.Selectors.Input.Assignments {
		out = append(out, assignment.Target)
	}
	return out
}

// 未受影响的 assignment 全部 kept：form/literal/selection+Transform 定制
// 原样保留，Changed=false。
func TestPlanBindingSelectorSyncKeepsUnaffectedCustomSources(t *testing.T) {
	page := syncTestResourcePage(FilterSpec{Key: "player_id", Title: LocalizedText{"zh-CN": "玩家"}, Type: "text"})
	inputSchema := JSONSchema(`{
		"type":"object",
		"properties":{
			"player_id":{"type":"string"},
			"page":{"type":"integer"},
			"extra":{"type":"array","items":{"type":"object"}}
		},
		"required":["player_id"]
	}`)
	outputSchema := JSONSchema(`{"type":"object","properties":{"rows":{"type":"array","items":{"type":"object"}}}}`)
	binding := syncTestBinding(
		[]InputAssignment{
			{Target: "/player_id", Source: ValueSource{Kind: SourceForm, Path: "/player_id"}},
			{Target: "/page", Source: ValueSource{Kind: SourceLiteral, Value: json.RawMessage(`1`)}},
			{Target: "/extra", Source: ValueSource{
				Kind:      SourceSelection,
				Path:      "/items",
				Transform: &TransformSpec{Type: TransformPick},
			}},
		},
		[]OutputAssignment{{StateKey: "items", Source: "/rows", Shape: OutputShapeCollection}},
	)
	fn := FunctionSpec{ID: "player.list", InputSchema: inputSchema, OutputSchema: outputSchema}

	report, synced := PlanBindingSelectorSync(page, binding, fn, SelectorSyncOptions{})

	assert.False(t, report.Changed)
	for _, target := range []string{"/player_id", "/page", "/extra"} {
		entry, ok := findSyncInputEntry(report, target)
		require.True(t, ok, "entry for %s", target)
		assert.Equal(t, SelectorSyncKept, entry.Action, target)
	}
	require.Len(t, synced.Selectors.Input.Assignments, 3)
	// literal 值与 selection+pick Transform 原样保留
	assert.Equal(t, json.RawMessage(`1`), synced.Selectors.Input.Assignments[1].Source.Value)
	require.NotNil(t, synced.Selectors.Input.Assignments[2].Source.Transform)
	assert.Equal(t, TransformPick, synced.Selectors.Input.Assignments[2].Source.Transform.Type)
	entry, ok := findSyncOutputEntry(report, "items")
	require.True(t, ok)
	assert.Equal(t, SelectorSyncKept, entry.Action)
}

// prev schema 可信：diff rename 候选唯一命中 → renamed(high)，source 定制保留。
func TestPlanBindingSelectorSyncRenameFromTrustedPrev(t *testing.T) {
	page := syncTestResourcePage(FilterSpec{Key: "player_id", Title: LocalizedText{"zh-CN": "玩家"}, Type: "text"})
	prevSchema := JSONSchema(`{"type":"object","properties":{"uid":{"type":"string"}},"required":["uid"]}`)
	newSchema := JSONSchema(`{"type":"object","properties":{"player_id":{"type":"string"}},"required":["player_id"]}`)
	binding := syncTestBinding(
		[]InputAssignment{{Target: "/uid", Source: ValueSource{Kind: SourceForm, Path: "/uid"}}},
		nil,
	)
	fn := FunctionSpec{ID: "player.list", InputSchema: newSchema, PreviousInputSchema: prevSchema}

	report, synced := PlanBindingSelectorSync(page, binding, fn, SelectorSyncOptions{PrevTrusted: true})

	entry, ok := findSyncInputEntry(report, "/uid")
	require.True(t, ok)
	assert.Equal(t, SelectorSyncRenamed, entry.Action)
	assert.Equal(t, "/player_id", entry.NewTarget)
	assert.Equal(t, SelectorSyncConfidenceHigh, entry.Confidence)
	assert.True(t, report.Changed)
	require.Len(t, synced.Selectors.Input.Assignments, 1)
	assert.Equal(t, "/player_id", synced.Selectors.Input.Assignments[0].Target)
	// source 表单路径不动——表单字段名没有变
	assert.Equal(t, SourceForm, synced.Selectors.Input.Assignments[0].Source.Kind)
	assert.Equal(t, "/uid", synced.Selectors.Input.Assignments[0].Source.Path)
}

// prev 有候选但不可信（digest 不一致/多跳漂移）→ 同样命中但 confidence=low。
func TestPlanBindingSelectorSyncRenameFromUntrustedPrev(t *testing.T) {
	page := syncTestResourcePage()
	prevSchema := JSONSchema(`{"type":"object","properties":{"uid":{"type":"string"}}}`)
	newSchema := JSONSchema(`{"type":"object","properties":{"player_id":{"type":"string"}}}`)
	binding := syncTestBinding(
		[]InputAssignment{{Target: "/uid", Source: ValueSource{Kind: SourceForm, Path: "/uid"}}},
		nil,
	)
	fn := FunctionSpec{ID: "player.list", InputSchema: newSchema, PreviousInputSchema: prevSchema}

	report, _ := PlanBindingSelectorSync(page, binding, fn, SelectorSyncOptions{PrevTrusted: false})

	entry, ok := findSyncInputEntry(report, "/uid")
	require.True(t, ok)
	assert.Equal(t, SelectorSyncRenamed, entry.Action)
	assert.Equal(t, SelectorSyncConfidenceLow, entry.Confidence)
}

// 无 prev：启发式（同父 × 未占用 × 可赋值）唯一命中 → renamed(low)。
func TestPlanBindingSelectorSyncRenameHeuristicWithoutPrev(t *testing.T) {
	page := syncTestResourcePage()
	newSchema := JSONSchema(`{"type":"object","properties":{"player_id":{"type":"string"}}}`)
	binding := syncTestBinding(
		[]InputAssignment{{Target: "/uid", Source: ValueSource{Kind: SourceForm, Path: "/uid"}}},
		nil,
	)
	fn := FunctionSpec{ID: "player.list", InputSchema: newSchema}

	report, synced := PlanBindingSelectorSync(page, binding, fn, SelectorSyncOptions{})

	entry, ok := findSyncInputEntry(report, "/uid")
	require.True(t, ok)
	assert.Equal(t, SelectorSyncRenamed, entry.Action)
	assert.Equal(t, "/player_id", entry.NewTarget)
	assert.Equal(t, SelectorSyncConfidenceLow, entry.Confidence)
	require.Len(t, synced.Selectors.Input.Assignments, 1)
	assert.Equal(t, "/player_id", synced.Selectors.Input.Assignments[0].Target)
}

// rename 歧义（两个同父同类型新字段）→ removed。
func TestPlanBindingSelectorSyncRenameAmbiguousRemoved(t *testing.T) {
	page := syncTestResourcePage()
	newSchema := JSONSchema(`{
		"type":"object",
		"properties":{
			"player_id":{"type":"string"},
			"server_id":{"type":"string"}
		}
	}`)
	binding := syncTestBinding(
		[]InputAssignment{{Target: "/uid", Source: ValueSource{Kind: SourceForm, Path: "/uid"}}},
		nil,
	)
	fn := FunctionSpec{ID: "player.list", InputSchema: newSchema}

	report, synced := PlanBindingSelectorSync(page, binding, fn, SelectorSyncOptions{})

	entry, ok := findSyncInputEntry(report, "/uid")
	require.True(t, ok)
	assert.Equal(t, SelectorSyncRemoved, entry.Action)
	assert.Empty(t, entry.NewTarget)
	assert.Empty(t, syncAssignmentTargets(synced))
}

// required 差集：非 composite 页且表单有同名字段 → 补 form 映射（added）。
func TestPlanBindingSelectorSyncAddsRequiredFromForm(t *testing.T) {
	page := syncTestResourcePage(
		FilterSpec{Key: "player_id", Title: LocalizedText{"zh-CN": "玩家"}, Type: "text"},
		FilterSpec{Key: "region", Title: LocalizedText{"zh-CN": "区服"}, Type: "text"},
	)
	newSchema := JSONSchema(`{
		"type":"object",
		"properties":{
			"player_id":{"type":"string"},
			"region":{"type":"string"}
		},
		"required":["player_id","region"]
	}`)
	binding := syncTestBinding(
		[]InputAssignment{{Target: "/player_id", Source: ValueSource{Kind: SourceForm, Path: "/player_id"}}},
		nil,
	)
	fn := FunctionSpec{ID: "player.list", InputSchema: newSchema}

	report, synced := PlanBindingSelectorSync(page, binding, fn, SelectorSyncOptions{})

	entry, ok := findSyncInputEntry(report, "/region")
	require.True(t, ok)
	assert.Equal(t, SelectorSyncAdded, entry.Action)
	assert.Equal(t, SourceForm, entry.SourceKind)
	require.Len(t, synced.Selectors.Input.Assignments, 2)
	added := synced.Selectors.Input.Assignments[1]
	assert.Equal(t, "/region", added.Target)
	assert.Equal(t, SourceForm, added.Source.Kind)
	assert.Equal(t, "/region", added.Source.Path)
}

// required 差集：表单缺同名字段 → manual_required（不静默造映射）。
func TestPlanBindingSelectorSyncRequiredMissingFormManual(t *testing.T) {
	page := syncTestResourcePage(FilterSpec{Key: "player_id", Title: LocalizedText{"zh-CN": "玩家"}, Type: "text"})
	newSchema := JSONSchema(`{
		"type":"object",
		"properties":{
			"player_id":{"type":"string"},
			"token":{"type":"string"}
		},
		"required":["player_id","token"]
	}`)
	binding := syncTestBinding(
		[]InputAssignment{{Target: "/player_id", Source: ValueSource{Kind: SourceForm, Path: "/player_id"}}},
		nil,
	)
	fn := FunctionSpec{ID: "player.list", InputSchema: newSchema}

	report, synced := PlanBindingSelectorSync(page, binding, fn, SelectorSyncOptions{})

	entry, ok := findSyncInputEntry(report, "/token")
	require.True(t, ok)
	assert.Equal(t, SelectorSyncManual, entry.Action)
	require.Len(t, synced.Selectors.Input.Assignments, 1)
}

// S2 identity 语义命中：required 补齐时资源 identity 字段自动接 row 源
// （IdentityOf 注入 CapabilitySemantics.IdentityField）。门禁三关=
// isSourceAllowed（HasDetailView/IsRowAction）+ RowSchema 含该字段 + 类型
// 可赋值，全部通过才接 row；任一不满足回落 form 同名路径（见后续回落用例）。
func TestPlanBindingSelectorSyncIdentityRowSource(t *testing.T) {
	page := syncTestResourcePage(FilterSpec{Key: "keyword", Title: LocalizedText{"zh-CN": "关键字"}, Type: "text"})
	page.ResourceKey = "player"
	page.Resource.DetailView = &DetailViewSpec{}
	newSchema := JSONSchema(`{
		"type":"object",
		"properties":{"uid":{"type":"string"},"keyword":{"type":"string"}},
		"required":["uid","keyword"]
	}`)
	binding := syncTestBinding(
		[]InputAssignment{{Target: "/keyword", Source: ValueSource{Kind: SourceForm, Path: "/keyword"}}},
		nil,
	)
	fn := FunctionSpec{ID: "player.detail", InputSchema: newSchema}

	identityOf := func(resourceKey string) (string, bool) {
		assert.Equal(t, "player", resourceKey, "identity lookup key must be the page resourceKey")
		return "uid", true
	}
	report, synced := PlanBindingSelectorSync(page, binding, fn, SelectorSyncOptions{IdentityOf: identityOf})

	entry, ok := findSyncInputEntry(report, "/uid")
	require.True(t, ok)
	assert.Equal(t, SelectorSyncAdded, entry.Action)
	assert.Equal(t, SourceRow, entry.SourceKind)
	assert.Equal(t, SelectorSyncConfidenceHigh, entry.Confidence)
	assert.Contains(t, entry.Reason, "identity")
	require.Len(t, synced.Selectors.Input.Assignments, 2)
	added := synced.Selectors.Input.Assignments[1]
	assert.Equal(t, "/uid", added.Target)
	assert.Equal(t, SourceRow, added.Source.Kind)
	assert.Equal(t, "/uid", added.Source.Path)
}

// S2 未注入（IdentityOf=nil）：required identity 字段回落现状——表单缺
// 同名字段 → manual_required（不静默造 row 源）。
func TestPlanBindingSelectorSyncIdentityNotInjectedFallsBack(t *testing.T) {
	page := syncTestResourcePage(FilterSpec{Key: "keyword", Title: LocalizedText{"zh-CN": "关键字"}, Type: "text"})
	page.Resource.DetailView = &DetailViewSpec{}
	newSchema := JSONSchema(`{
		"type":"object",
		"properties":{"uid":{"type":"string"}},
		"required":["uid"]
	}`)
	binding := syncTestBinding(nil, nil)
	fn := FunctionSpec{ID: "player.detail", InputSchema: newSchema}

	report, synced := PlanBindingSelectorSync(page, binding, fn, SelectorSyncOptions{})

	entry, ok := findSyncInputEntry(report, "/uid")
	require.True(t, ok)
	assert.Equal(t, SelectorSyncManual, entry.Action)
	assert.Empty(t, syncAssignmentTargets(synced))
}

// S2 门禁一：无 DetailView 的 query 页不接 row 源（补 row 会被发布级
// 校验 422）——回落 form 同名路径。
func TestPlanBindingSelectorSyncIdentityWithoutDetailViewFallsBack(t *testing.T) {
	page := syncTestResourcePage(
		FilterSpec{Key: "uid", Title: LocalizedText{"zh-CN": "玩家"}, Type: "text"},
	)
	newSchema := JSONSchema(`{
		"type":"object",
		"properties":{"uid":{"type":"string"}},
		"required":["uid"]
	}`)
	binding := syncTestBinding(nil, nil)
	fn := FunctionSpec{ID: "player.list", InputSchema: newSchema}

	report, synced := PlanBindingSelectorSync(page, binding, fn, SelectorSyncOptions{
		IdentityOf: func(string) (string, bool) { return "uid", true },
	})

	entry, ok := findSyncInputEntry(report, "/uid")
	require.True(t, ok)
	assert.Equal(t, SelectorSyncAdded, entry.Action)
	assert.Equal(t, SourceForm, entry.SourceKind, "must fall back to the same-name form source")
	require.Len(t, synced.Selectors.Input.Assignments, 1)
	assert.Equal(t, SourceForm, synced.Selectors.Input.Assignments[0].Source.Kind)
}

// S2 门禁二：RowSchema 缺 identity 字段（行上下文取不到值）——回落
// form 同名路径。
func TestPlanBindingSelectorSyncIdentityRowSchemaMissingFallsBack(t *testing.T) {
	page := PageSpec{
		PageKey:     "players",
		Type:        PageTypeResource,
		ResourceKey: "player",
		Title:       LocalizedText{"zh-CN": "玩家"},
		Resource: &ResourcePageSpec{
			ListView: &ListViewSpec{
				// RowSchema 只有 name——没有 uid
				RowSchema: JSONSchema(`{"type":"object","properties":{"name":{"type":"string"}}}`),
				Filters: []FilterSpec{
					{Key: "uid", Title: LocalizedText{"zh-CN": "玩家"}, Type: "text"},
				},
			},
			DetailView: &DetailViewSpec{},
		},
	}
	newSchema := JSONSchema(`{
		"type":"object",
		"properties":{"uid":{"type":"string"}},
		"required":["uid"]
	}`)
	binding := syncTestBinding(nil, nil)
	fn := FunctionSpec{ID: "player.detail", InputSchema: newSchema}

	report, _ := PlanBindingSelectorSync(page, binding, fn, SelectorSyncOptions{
		IdentityOf: func(string) (string, bool) { return "uid", true },
	})

	entry, ok := findSyncInputEntry(report, "/uid")
	require.True(t, ok)
	assert.Equal(t, SelectorSyncAdded, entry.Action)
	assert.Equal(t, SourceForm, entry.SourceKind, "row schema lacks the identity field; must fall back to form")
}

// S2 非 identity 的 required 字段不受影响（语义只命中 identity 字段）。
func TestPlanBindingSelectorSyncNonIdentityRequiredStaysForm(t *testing.T) {
	page := syncTestResourcePage(FilterSpec{Key: "token", Title: LocalizedText{"zh-CN": "令牌"}, Type: "text"})
	page.Resource.DetailView = &DetailViewSpec{}
	newSchema := JSONSchema(`{
		"type":"object",
		"properties":{"uid":{"type":"string"},"token":{"type":"string"}},
		"required":["uid","token"]
	}`)
	binding := syncTestBinding(
		[]InputAssignment{{Target: "/uid", Source: ValueSource{Kind: SourceForm, Path: "/uid"}}},
		nil,
	)
	fn := FunctionSpec{ID: "player.detail", InputSchema: newSchema}

	report, synced := PlanBindingSelectorSync(page, binding, fn, SelectorSyncOptions{
		IdentityOf: func(string) (string, bool) { return "uid", true },
	})

	// /uid 已 occupied（kept）；/token 非 identity → form 同名 added
	entry, ok := findSyncInputEntry(report, "/token")
	require.True(t, ok)
	assert.Equal(t, SelectorSyncAdded, entry.Action)
	assert.Equal(t, SourceForm, entry.SourceKind)
	require.Len(t, synced.Selectors.Input.Assignments, 2)
	assert.Equal(t, SourceForm, synced.Selectors.Input.Assignments[1].Source.Kind)
}

// composite 页：required 输入一律 manual_required（输入只应来自
// page_state/literal，不自动补 form）。
func TestPlanBindingSelectorSyncCompositeRequiredManual(t *testing.T) {
	page := PageSpec{
		PageKey: "overview",
		Type:    PageTypeComposite,
		Title:   LocalizedText{"zh-CN": "总览"},
		Composite: &CompositePageSpec{Sections: []CompositeSection{{
			Key:       "bag",
			BindingID: "bag",
			View:      "table",
		}}},
	}
	newSchema := JSONSchema(`{
		"type":"object",
		"properties":{"uid":{"type":"string"}},
		"required":["uid"]
	}`)
	binding := syncTestBinding(nil, nil)
	fn := FunctionSpec{ID: "bag.list", InputSchema: newSchema}

	report, synced := PlanBindingSelectorSync(page, binding, fn, SelectorSyncOptions{})

	entry, ok := findSyncInputEntry(report, "/uid")
	require.True(t, ok)
	assert.Equal(t, SelectorSyncManual, entry.Action)
	assert.Empty(t, syncAssignmentTargets(synced))
}

// target 存在但类型漂移：保留 assignment + type_changed（防摘掉 required）。
// 覆盖两条判定路径：prev diff 的 type_changed 记录、isAssignable 失败。
func TestPlanBindingSelectorSyncTypeChangedKept(t *testing.T) {
	page := syncTestResourcePage(FilterSpec{Key: "count", Title: LocalizedText{"zh-CN": "数量"}, Type: "number"})
	prevSchema := JSONSchema(`{"type":"object","properties":{"count":{"type":"integer"}}}`)
	newSchema := JSONSchema(`{"type":"object","properties":{"count":{"type":"string"}},"required":["count"]}`)
	prevBinding := syncTestBinding(
		[]InputAssignment{
			{Target: "/count", Source: ValueSource{Kind: SourceLiteral, Value: json.RawMessage(`1`)}},
		},
		nil,
	)
	fn := FunctionSpec{
		ID:                  "player.list",
		InputSchema:         newSchema,
		PreviousInputSchema: prevSchema,
	}

	report, synced := PlanBindingSelectorSync(page, prevBinding, fn, SelectorSyncOptions{PrevTrusted: true})

	entry, ok := findSyncInputEntry(report, "/count")
	require.True(t, ok)
	assert.Equal(t, SelectorSyncTypeChanged, entry.Action)
	require.Len(t, synced.Selectors.Input.Assignments, 1)
	assert.Equal(t, "/count", synced.Selectors.Input.Assignments[0].Target)

	// 路径二：无 prev，form number 源赋不进 string target
	formBinding := syncTestBinding(
		[]InputAssignment{{Target: "/count", Source: ValueSource{Kind: SourceForm, Path: "/count"}}},
		nil,
	)
	report2, synced2 := PlanBindingSelectorSync(page, formBinding, fn, SelectorSyncOptions{})
	entry2, ok := findSyncInputEntry(report2, "/count")
	require.True(t, ok)
	assert.Equal(t, SelectorSyncTypeChanged, entry2.Action)
	require.Len(t, synced2.Selectors.Input.Assignments, 1)
}

// 必需输出（items）source 消失：无 rename 候选时走 RecomputeDefaults 重推导
// → renamed；后置 ValidateRequiredOutputAssignments 必须通过。
func TestPlanBindingSelectorSyncRequiredOutputRechosen(t *testing.T) {
	page := syncTestResourcePage()
	newSchema := JSONSchema(`{"type":"object","properties":{"rows":{"type":"array","items":{"type":"object"}}}}`)
	binding := syncTestBinding(
		nil,
		[]OutputAssignment{{StateKey: "items", Source: "/data", Shape: OutputShapeCollection}},
	)
	fn := FunctionSpec{
		ID:           "player.list",
		OutputSchema: newSchema,
	}
	rechooser := func(usage PageBindingUsage, schema JSONSchema) []OutputAssignment {
		assert.Equal(t, BindingUsageQuery, usage)
		return []OutputAssignment{{StateKey: "items", Source: "/rows", Shape: OutputShapeCollection}}
	}

	report, synced := PlanBindingSelectorSync(page, binding, fn, SelectorSyncOptions{RecomputeDefaults: rechooser})

	entry, ok := findSyncOutputEntry(report, "items")
	require.True(t, ok)
	assert.Equal(t, SelectorSyncRenamed, entry.Action)
	assert.Equal(t, "/rows", entry.NewSource)
	assert.True(t, entry.Required)
	assert.True(t, report.Changed)
	require.Len(t, synced.Selectors.Output, 1)
	assert.Equal(t, "/rows", synced.Selectors.Output[0].Source)
	assert.Empty(t, ValidateRequiredOutputAssignments(synced, page))
}

// 必需输出 source 消失且 prev diff 有唯一 rename 候选（trusted）→ renamed(high)。
func TestPlanBindingSelectorSyncRequiredOutputRenameFromPrev(t *testing.T) {
	page := syncTestResourcePage()
	prevSchema := JSONSchema(`{"type":"object","properties":{"data":{"type":"array","items":{"type":"object"}}}}`)
	newSchema := JSONSchema(`{"type":"object","properties":{"rows":{"type":"array","items":{"type":"object"}}}}`)
	binding := syncTestBinding(
		nil,
		[]OutputAssignment{{StateKey: "items", Source: "/data", Shape: OutputShapeCollection}},
	)
	fn := FunctionSpec{
		ID:                   "player.list",
		OutputSchema:         newSchema,
		PreviousOutputSchema: prevSchema,
	}

	report, synced := PlanBindingSelectorSync(page, binding, fn, SelectorSyncOptions{PrevTrusted: true})

	entry, ok := findSyncOutputEntry(report, "items")
	require.True(t, ok)
	assert.Equal(t, SelectorSyncRenamed, entry.Action)
	assert.Equal(t, "/rows", entry.NewSource)
	assert.Equal(t, SelectorSyncConfidenceHigh, entry.Confidence)
	require.Len(t, synced.Selectors.Output, 1)
	assert.Equal(t, "/rows", synced.Selectors.Output[0].Source)
	assert.Empty(t, ValidateRequiredOutputAssignments(synced, page))
}

// 必需输出推导全失败时 fallback 到根对象（Source=""）：generator 对 task
// 页 taskStatus 的默认映射就是整对象。
func TestPlanBindingSelectorSyncRequiredOutputRootFallback(t *testing.T) {
	page := syncTestTaskPage()
	binding := PageFunctionBinding{
		ID:         "status",
		FunctionID: "mail.send",
		Usage:      BindingUsageTaskStatus,
		Selectors: &BindingSelectors{
			Output: []OutputAssignment{{StateKey: "taskStatus", Source: "/status", Shape: OutputShapeObject}},
		},
		Execution: PageBindingExecution{Mode: PageExecutionModeSync},
	}
	fn := FunctionSpec{
		ID:           "mail.send",
		OutputSchema: JSONSchema(`{"type":"object","properties":{"progress":{"type":"integer"}}}`),
	}

	report, synced := PlanBindingSelectorSync(page, binding, fn, SelectorSyncOptions{})

	entry, ok := findSyncOutputEntry(report, "taskStatus")
	require.True(t, ok)
	assert.Equal(t, SelectorSyncRenamed, entry.Action)
	assert.True(t, entry.Required)
	assert.Equal(t, SelectorSyncConfidenceHigh, entry.Confidence)
	require.Len(t, synced.Selectors.Output, 1)
	assert.Equal(t, "", synced.Selectors.Output[0].Source)
	assert.Empty(t, ValidateRequiredOutputAssignments(synced, page))
}

// 必需输出推导不出（无 rename、无 rechoose、根类型不匹配）→ 保留原
// assignment + manual_required，绝不摘成缺失。
func TestPlanBindingSelectorSyncRequiredOutputManualKeepsAssignment(t *testing.T) {
	page := syncTestResourcePage()
	newSchema := JSONSchema(`{"type":"object","properties":{"detail":{"type":"object"}}}`)
	binding := syncTestBinding(
		nil,
		[]OutputAssignment{{StateKey: "items", Source: "/data", Shape: OutputShapeCollection}},
	)
	fn := FunctionSpec{ID: "player.list", OutputSchema: newSchema}

	report, synced := PlanBindingSelectorSync(page, binding, fn, SelectorSyncOptions{})

	entry, ok := findSyncOutputEntry(report, "items")
	require.True(t, ok)
	assert.Equal(t, SelectorSyncManual, entry.Action)
	assert.True(t, entry.Required)
	// 原 assignment 原样保留：必需输出校验仍通过（source 失效由
	// ValidateOutputAssignments 报，而不是缺 assignment）
	require.Len(t, synced.Selectors.Output, 1)
	assert.Equal(t, "/data", synced.Selectors.Output[0].Source)
	assert.Empty(t, ValidateRequiredOutputAssignments(synced, page))
}

// 非必需输出 source 消失且无候选 → removed。
func TestPlanBindingSelectorSyncNonRequiredOutputRemoved(t *testing.T) {
	page := syncTestResourcePage()
	newSchema := JSONSchema(`{"type":"object","properties":{"rows":{"type":"array","items":{"type":"object"}}}}`)
	binding := syncTestBinding(
		nil,
		[]OutputAssignment{
			{StateKey: "items", Source: "/rows", Shape: OutputShapeCollection},
			{StateKey: "total", Source: "/gone", Shape: OutputShapeScalar},
		},
	)
	fn := FunctionSpec{ID: "player.list", OutputSchema: newSchema}

	report, synced := PlanBindingSelectorSync(page, binding, fn, SelectorSyncOptions{})

	entry, ok := findSyncOutputEntry(report, "total")
	require.True(t, ok)
	assert.Equal(t, SelectorSyncRemoved, entry.Action)
	require.Len(t, synced.Selectors.Output, 1)
	assert.Equal(t, "items", synced.Selectors.Output[0].StateKey)
}

// 非必需输出 source 存在但类型变（scalar→array）→ shape_updated。
func TestPlanBindingSelectorSyncOutputShapeUpdated(t *testing.T) {
	page := syncTestResourcePage()
	newSchema := JSONSchema(`{
		"type":"object",
		"properties":{
			"rows":{"type":"array","items":{"type":"object"}},
			"total":{"type":"array","items":{"type":"object"}}
		}
	}`)
	binding := syncTestBinding(
		nil,
		[]OutputAssignment{
			{StateKey: "items", Source: "/rows", Shape: OutputShapeCollection},
			{StateKey: "total", Source: "/total", Shape: OutputShapeScalar},
		},
	)
	fn := FunctionSpec{ID: "player.list", OutputSchema: newSchema}

	report, synced := PlanBindingSelectorSync(page, binding, fn, SelectorSyncOptions{})

	entry, ok := findSyncOutputEntry(report, "total")
	require.True(t, ok)
	assert.Equal(t, SelectorSyncShapeUpdated, entry.Action)
	assert.Equal(t, OutputShapeCollection, entry.NewShape)
	require.Len(t, synced.Selectors.Output, 2)
	assert.Equal(t, OutputShapeCollection, synced.Selectors.Output[1].Shape)
	assert.Empty(t, ValidateRequiredOutputAssignments(synced, page))
}

// 必需输出整体缺失（从未配置）→ 重推导补一条（added）。
func TestPlanBindingSelectorSyncRequiredOutputMissingAdded(t *testing.T) {
	page := syncTestResourcePage()
	newSchema := JSONSchema(`{"type":"object","properties":{"rows":{"type":"array","items":{"type":"object"}}}}`)
	binding := syncTestBinding(
		nil,
		[]OutputAssignment{{StateKey: "total", Source: "/rows", Shape: OutputShapeScalar}},
	)
	fn := FunctionSpec{ID: "player.list", OutputSchema: newSchema}
	rechooser := func(usage PageBindingUsage, schema JSONSchema) []OutputAssignment {
		return []OutputAssignment{{StateKey: "items", Source: "/rows", Shape: OutputShapeCollection}}
	}

	report, synced := PlanBindingSelectorSync(page, binding, fn, SelectorSyncOptions{RecomputeDefaults: rechooser})

	entry, ok := findSyncOutputEntry(report, "items")
	require.True(t, ok)
	assert.Equal(t, SelectorSyncAdded, entry.Action)
	assert.Equal(t, "/rows", entry.NewSource)
	require.Len(t, synced.Selectors.Output, 2)
	assert.Equal(t, "items", synced.Selectors.Output[1].StateKey)
	assert.Empty(t, ValidateRequiredOutputAssignments(synced, page))
}

// execution mode 对齐：task 函数绑成 sync → 修正为 task；sync 函数绑成
// task → 修正回 sync（不造非法组合）。
func TestPlanBindingSelectorSyncExecutionModeFixed(t *testing.T) {
	page := syncTestResourcePage()
	schema := JSONSchema(`{"type":"object","properties":{"q":{"type":"string"}}}`)
	outputSchema := JSONSchema(`{"type":"object","properties":{"rows":{"type":"array","items":{"type":"object"}}}}`)
	binding := syncTestBinding(
		[]InputAssignment{{Target: "/q", Source: ValueSource{Kind: SourceForm, Path: "/q"}}},
		[]OutputAssignment{{StateKey: "items", Source: "/rows", Shape: OutputShapeCollection}},
	)
	taskFn := FunctionSpec{ID: "player.export", Execution: FunctionExecutionTask, InputSchema: schema, OutputSchema: outputSchema}

	report, synced := PlanBindingSelectorSync(page, binding, taskFn, SelectorSyncOptions{})
	assert.True(t, report.ExecutionModeFixed)
	assert.Equal(t, PageExecutionModeTask, synced.Execution.Mode)

	syncFn := FunctionSpec{ID: "player.list", Execution: FunctionExecutionSync, InputSchema: schema, OutputSchema: outputSchema}
	taskBinding := binding
	taskBinding.Execution.Mode = PageExecutionModeTask
	report2, synced2 := PlanBindingSelectorSync(page, taskBinding, syncFn, SelectorSyncOptions{})
	assert.True(t, report2.ExecutionModeFixed)
	assert.Equal(t, PageExecutionModeSync, synced2.Execution.Mode)

	// 已对齐时不标 fixed（Changed 也不因 mode 置位）
	report3, synced3 := PlanBindingSelectorSync(page, binding, syncFn, SelectorSyncOptions{})
	assert.False(t, report3.ExecutionModeFixed)
	assert.False(t, report3.Changed)
	assert.Equal(t, PageExecutionModeSync, synced3.Execution.Mode)
}

// 非法 prev（数据损坏）不 panic：usableSchemaDiff 降级为空 diff，
// 同步走启发式路径照常产出计划。
func TestPlanBindingSelectorSyncInvalidPrevSchemaNoPanic(t *testing.T) {
	page := syncTestResourcePage()
	newSchema := JSONSchema(`{"type":"object","properties":{"player_id":{"type":"string"}}}`)
	binding := syncTestBinding(
		[]InputAssignment{{Target: "/uid", Source: ValueSource{Kind: SourceForm, Path: "/uid"}}},
		nil,
	)
	fn := FunctionSpec{
		ID:                  "player.list",
		InputSchema:         newSchema,
		PreviousInputSchema: JSONSchema(`{not valid json`),
	}

	assert.NotPanics(t, func() {
		report, synced := PlanBindingSelectorSync(page, binding, fn, SelectorSyncOptions{PrevTrusted: true})
		entry, ok := findSyncInputEntry(report, "/uid")
		require.True(t, ok)
		// 降级启发式：唯一同父候选，confidence=low（prev 不可用）
		assert.Equal(t, SelectorSyncRenamed, entry.Action)
		assert.Equal(t, SelectorSyncConfidenceLow, entry.Confidence)
		require.Len(t, synced.Selectors.Input.Assignments, 1)
		assert.Equal(t, "/player_id", synced.Selectors.Input.Assignments[0].Target)
	})
}

// planner 是纯函数：入参 binding 不被修改。
func TestPlanBindingSelectorSyncDoesNotMutateInput(t *testing.T) {
	page := syncTestResourcePage()
	newSchema := JSONSchema(`{
		"type":"object",
		"properties":{
			"player_id":{"type":"string"},
			"extra":{"type":"array"}
		},
		"required":["player_id"]
	}`)
	binding := syncTestBinding(
		[]InputAssignment{
			{Target: "/uid", Source: ValueSource{Kind: SourceForm, Path: "/uid"}},
			{Target: "/extra", Source: ValueSource{Kind: SourceLiteral, Value: json.RawMessage(`[]`)}},
		},
		[]OutputAssignment{{StateKey: "items", Source: "/rows", Shape: OutputShapeCollection}},
	)
	fn := FunctionSpec{ID: "player.list", InputSchema: newSchema}

	_, _ = PlanBindingSelectorSync(page, binding, fn, SelectorSyncOptions{})

	require.NotNil(t, binding.Selectors)
	require.Len(t, binding.Selectors.Input.Assignments, 2)
	assert.Equal(t, "/uid", binding.Selectors.Input.Assignments[0].Target)
	assert.Equal(t, "/extra", binding.Selectors.Input.Assignments[1].Target)
}
