package spec

import (
	"strings"
)

// ---------------------------------------------------------------------------
// 一键同步 stale selector（sync-selectors）的 planner。
//
// 契约 schema 变化后，页面绑定的 selector 出现 target 消失 / 类型漂移 /
// 必填缺失等 stale 诊断，发布被 422 阻断。planner 对单个 binding 产出
// 「精准修复」：只动受影响的 assignment，保留全部未受影响映射（含
// Source/Kind/Path/Value/Transform 定制）。纯函数、深拷贝改副本，
// dry-run 与 apply 共用同一实现，杜绝预览与落库不一致。
//
// 修复不了的（composite 补 form、表单缺同名字段、推导不出必需输出等）
// 以 manual_required 条目报告，绝不静默摘除必需映射。
// ---------------------------------------------------------------------------

// SelectorSyncAction 是同步报告条目的动作枚举。
type SelectorSyncAction string

const (
	SelectorSyncKept         SelectorSyncAction = "kept"
	SelectorSyncRenamed      SelectorSyncAction = "renamed"
	SelectorSyncRemoved      SelectorSyncAction = "removed"
	SelectorSyncAdded        SelectorSyncAction = "added"
	SelectorSyncTypeChanged  SelectorSyncAction = "type_changed"
	SelectorSyncShapeUpdated SelectorSyncAction = "shape_updated"
	SelectorSyncManual       SelectorSyncAction = "manual_required"
)

// SelectorSyncConfidence 是重映射决策的置信度：high = prev schema 精确
// 命中（diff 唯一候选且 prev 与发布时点 schema 一致）；low = 启发式
// （无 prev 或 prev 不可信时的同父/同类型唯一候选）。
type SelectorSyncConfidence string

const (
	SelectorSyncConfidenceHigh SelectorSyncConfidence = "high"
	SelectorSyncConfidenceLow  SelectorSyncConfidence = "low"
)

// SelectorSyncInputEntry 是一个输入 assignment 的同步决策。
type SelectorSyncInputEntry struct {
	Target     string                 `json:"target"`
	Action     SelectorSyncAction     `json:"action"`
	NewTarget  string                 `json:"newTarget,omitempty"`
	SourceKind ValueSourceKind        `json:"sourceKind,omitempty"`
	Confidence SelectorSyncConfidence `json:"confidence,omitempty"`
	Reason     string                 `json:"reason"`
}

// SelectorSyncOutputEntry 是一个输出 assignment 的同步决策。
type SelectorSyncOutputEntry struct {
	StateKey   string                 `json:"stateKey"`
	Source     string                 `json:"source"`
	Action     SelectorSyncAction     `json:"action"`
	NewSource  string                 `json:"newSource,omitempty"`
	NewShape   OutputResultShape      `json:"newShape,omitempty"`
	Required   bool                   `json:"required"`
	Confidence SelectorSyncConfidence `json:"confidence,omitempty"`
	Reason     string                 `json:"reason"`
}

// BindingSelectorSyncReport 是单个 binding 的同步报告。
type BindingSelectorSyncReport struct {
	BindingID          string                    `json:"bindingId"`
	FunctionID         string                    `json:"functionId"`
	Changed            bool                      `json:"changed"`
	ExecutionModeFixed bool                      `json:"executionModeFixed,omitempty"`
	Input              []SelectorSyncInputEntry  `json:"input,omitempty"`
	Output             []SelectorSyncOutputEntry `json:"output,omitempty"`
	Manual             []Diagnostic              `json:"manual,omitempty"`
}

// OutputRechooser 重新推导一个 usage 的默认输出映射。生产实现由
// generator 包注入（RecomputeDefaultOutputs），避免 spec↔generator
// 循环依赖；测试可注入 stub。
type OutputRechooser func(usage PageBindingUsage, outputSchema JSONSchema) []OutputAssignment

// SelectorSyncOptions 是 planner 的可调参数。
type SelectorSyncOptions struct {
	// PrevTrusted 表示 fn 的 previous schema 与该 binding 发布时点的
	// schema 经 digest 比对一致（service 层用 freshness 双算法 digestMatch
	// 判定）。为 true 时 prev diff 的 rename 候选 confidence=high；
	// 否则 prev 不可信，rename 一律走启发式（low）。
	PrevTrusted bool
	// RecomputeDefaults 是 generator 注入的默认输出推导，用于必需输出
	// （items/detail/dataset 等）的重推导。可为 nil（推导降级）。
	RecomputeDefaults OutputRechooser
}

// PlanBindingSelectorSync 计算并应用单个 binding 的 selector 同步。
// 返回报告与同步后的 binding 副本（入参 binding 不被修改）。
func PlanBindingSelectorSync(
	page PageSpec,
	binding PageFunctionBinding,
	fn FunctionSpec,
	opts SelectorSyncOptions,
) (BindingSelectorSyncReport, PageFunctionBinding) {
	report := BindingSelectorSyncReport{
		BindingID:  strings.TrimSpace(binding.ID),
		FunctionID: strings.TrimSpace(binding.FunctionID),
	}

	synced := binding // 值拷贝；Selectors 重建为新实例
	changed := false

	// prev diff 预计算；非法 prev（数据损坏）不 panic，降级为无 prev
	//（启发式路径），与传 nil 行为一致。
	prevInputDiff := usableSchemaDiff(fn.PreviousInputSchema, fn.InputSchema)
	prevOutputDiff := usableSchemaDiff(fn.PreviousOutputSchema, fn.OutputSchema)

	syncedSelectors := BindingSelectors{}
	if binding.Selectors != nil {
		syncedSelectors.Input = binding.Selectors.Input
		syncedSelectors.Output = binding.Selectors.Output
	}

	inputChanged, inputEntries := syncInputAssignments(page, binding, fn, prevInputDiff, opts)
	syncedSelectors.Input = SelectorAST{Assignments: inputEntries.assignments}
	report.Input = inputEntries.entries
	changed = changed || inputChanged

	outputChanged, outputEntries := syncOutputAssignments(page, binding, fn, prevOutputDiff, opts)
	syncedSelectors.Output = outputEntries.assignments
	report.Output = outputEntries.entries
	changed = changed || outputChanged

	// execution mode 对齐（freshness 的 execution_mode_stale 同规则）：
	// task 函数必须 task 模式，其余 sync。task 辅助查询（status/events/
	// result）绑定的查询函数为 sync，不受影响。
	if want := syncExecutionMode(fn); binding.Execution.Mode != want {
		synced.Execution.Mode = want
		report.ExecutionModeFixed = true
		changed = true
	}

	synced.Selectors = &syncedSelectors
	report.Changed = changed
	return report, synced
}

// syncExecutionMode 与 freshness.executionModeForFunction 同规则。
func syncExecutionMode(fn FunctionSpec) PageExecutionMode {
	if fn.Execution == FunctionExecutionTask {
		return PageExecutionModeTask
	}
	return PageExecutionModeSync
}

// usableSchemaDiff 计算 old→new 的字段 diff；任一 schema 非法（diff 层
// 报 schema_diff_invalid_schema）时返回空 diff，行为与无 prev 一致。
func usableSchemaDiff(oldSchema JSONSchema, newSchema JSONSchema) SchemaDiffResult {
	result := DiffJSONSchemaFields(oldSchema, newSchema)
	for _, diag := range result.Diagnostics {
		if diag.Code == "schema_diff_invalid_schema" {
			return SchemaDiffResult{}
		}
	}
	return result
}

type inputSyncResult struct {
	assignments []InputAssignment
	entries     []SelectorSyncInputEntry
}

// syncInputAssignments 输入策略阶梯（逐 assignment 保序）：
//  1. target 存在、类型未变、源可赋值 → kept（定制原样保留）
//  2. target 存在但类型漂移 → 保留 assignment + type_changed 报告
//     （不摘——required assignment 摘掉会让发布校验失败）
//  3. target 消失 → prev diff rename 候选（prevTrusted 时 high）；
//     否则启发式（同父指针 × 类型一致 × 未占用 × 源可赋值，唯一命中）；
//     零或多候选 → removed
//  4. required 差集补齐：非 composite 页补 form 同名映射（门禁：页面
//     表单 schema 必须含该 path）；composite 页一律 manual_required
//     （composite 输入只应来自 page_state/literal）。
func syncInputAssignments(
	page PageSpec,
	binding PageFunctionBinding,
	fn FunctionSpec,
	prevDiff SchemaDiffResult,
	opts SelectorSyncOptions,
) (bool, inputSyncResult) {
	out := inputSyncResult{}
	if binding.Selectors == nil {
		return false, out
	}
	newSchema := fn.InputSchema
	ctx := SelectorContextForBinding(page, binding)
	occupied := map[string]struct{}{}
	changed := false

	for _, assignment := range binding.Selectors.Input.Assignments {
		target := assignment.Target
		entry := SelectorSyncInputEntry{
			Target:     target,
			SourceKind: assignment.Source.Kind,
		}
		switch {
		case !schemaHasPath(newSchema, target):
			// 3. target 消失 → rename / removed
			newTarget, confidence, reason, ok := rechooseInputTarget(assignment, newSchema, prevDiff, occupied, ctx, opts)
			if ok {
				renamed := assignment
				renamed.Target = newTarget
				out.assignments = append(out.assignments, renamed)
				occupied[newTarget] = struct{}{}
				entry.Action = SelectorSyncRenamed
				entry.NewTarget = newTarget
				entry.Confidence = confidence
				entry.Reason = reason
				changed = true
			} else {
				entry.Action = SelectorSyncRemoved
				entry.Reason = "target no longer exists in the input schema and no unique rename candidate matched"
				changed = true
			}
		case hasChange(prevDiff.Changes, target, SchemaFieldTypeChanged) || !isAssignable(newSchema, target, assignment.Source, ctx):
			// 2. 类型漂移：保留 assignment，报告请人工核对
			out.assignments = append(out.assignments, assignment)
			occupied[target] = struct{}{}
			entry.Action = SelectorSyncTypeChanged
			entry.Reason = "target type changed or source is no longer assignable; kept for manual review"
		default:
			// 1. kept
			out.assignments = append(out.assignments, assignment)
			occupied[target] = struct{}{}
			entry.Action = SelectorSyncKept
			entry.Reason = "target unchanged and source assignable"
		}
		out.entries = append(out.entries, entry)
	}

	// 4. required 差集补齐
	if required, err := requiredPointers(newSchema); err == nil {
		for _, path := range sortedMapKeys(required) {
			if _, done := occupied[path]; done {
				continue
			}
			entry := SelectorSyncInputEntry{Target: path}
			switch {
			case page.Type == PageTypeComposite:
				entry.Action = SelectorSyncManual
				entry.Reason = "composite page inputs must come from page_state or literal; add the mapping manually"
			default:
				formSchema := FormSchemaForBinding(page, binding)
				if !schemaHasPath(formSchema, path) {
					entry.Action = SelectorSyncManual
					entry.Reason = "required input field has no same-name form field; add a mapping or extend the page form"
				} else {
					pointer := path
					out.assignments = append(out.assignments, InputAssignment{
						Target: pointer,
						Source: ValueSource{Kind: SourceForm, Path: pointer},
					})
					occupied[path] = struct{}{}
					entry.Action = SelectorSyncAdded
					entry.SourceKind = SourceForm
					entry.Reason = "new required input field mapped to the same-name form field; verify it is not a row/detail identity field"
					changed = true
				}
			}
			out.entries = append(out.entries, entry)
		}
	}
	return changed, out
}

// rechooseInputTarget 为消失的 target 找重映射目标。优先 prev diff 的
// rename 候选（唯一命中）；否则启发式：新 schema 中与原 target 同父
// 指针、类型一致、未被占用、且现有 source 可赋值的字段，唯一命中才用。
func rechooseInputTarget(
	assignment InputAssignment,
	newSchema JSONSchema,
	prevDiff SchemaDiffResult,
	occupied map[string]struct{},
	ctx SelectorContext,
	opts SelectorSyncOptions,
) (string, SelectorSyncConfidence, string, bool) {
	if len(prevDiff.RenameCandidates) > 0 {
		var matches []FieldRenameCandidate
		for _, candidate := range prevDiff.RenameCandidates {
			if candidate.OldPath == assignment.Target {
				matches = append(matches, candidate)
			}
		}
		usable := filterUnoccupied(matches, occupied)
		if len(usable) == 1 && isAssignable(newSchema, usable[0].NewPath, assignment.Source, ctx) {
			confidence := SelectorSyncConfidenceLow
			reason := "heuristically matched by same parent, type, and required flag (previous schema unverified)"
			if opts.PrevTrusted {
				confidence = SelectorSyncConfidenceHigh
				reason = "renamed according to the verified previous schema diff (same parent, type, and required flag)"
			}
			return usable[0].NewPath, confidence, reason, true
		}
	}

	// 启发式：同父 × 未占用 × 现有 source 可赋值（类型一致性由
	// isAssignable 经 source 消费——消失 target 在新 schema 中已无类型
	// 可取），唯一命中才用。
	parent := parentPointer(assignment.Target)
	newFields, ok := schemaFieldSnapshots(newSchema)
	if !ok {
		return "", "", "", false
	}
	var candidates []string
	for path := range newFields {
		if parentPointer(path) != parent {
			continue
		}
		if _, taken := occupied[path]; taken {
			continue
		}
		if !isAssignable(newSchema, path, assignment.Source, ctx) {
			continue
		}
		candidates = append(candidates, path)
	}
	if len(candidates) == 1 {
		return candidates[0], SelectorSyncConfidenceLow, "heuristically matched the only new field under the same parent that accepts the current source", true
	}
	return "", "", "", false
}

func filterUnoccupied(candidates []FieldRenameCandidate, occupied map[string]struct{}) []FieldRenameCandidate {
	out := make([]FieldRenameCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		if _, taken := occupied[candidate.NewPath]; taken {
			continue
		}
		out = append(out, candidate)
	}
	return out
}

type outputSyncResult struct {
	assignments []OutputAssignment
	entries     []SelectorSyncOutputEntry
}

// syncOutputAssignments 输出策略阶梯：
//  1. source 存在且 shape 匹配 → kept
//  2. source 存在但 shape 不符 → 必需输出重推导（rechoose / 根对象
//     fallback）；非必需修正 shape（array→collection / object→object /
//     其余 scalar，task/dataset 不自动猜）并报 shape_updated
//  3. source 消失：必需 stateKey（ValidateRequiredOutputAssignments 的
//     六类矩阵）→ rename 候选 / rechoose / 根对象 fallback 重推导，
//     全部失败则保留原 assignment + manual_required（绝不摘成缺失，
//     否则发布校验 binding_output_selector_invalid 直接失败）；
//     非必需 → rename 候选，无则 removed
//  4. 必需输出整体缺失（从未配置或此前被摘）→ 同 3 的重推导补一条
//     （added）；推导不出 → manual_required。
func syncOutputAssignments(
	page PageSpec,
	binding PageFunctionBinding,
	fn FunctionSpec,
	prevDiff SchemaDiffResult,
	opts SelectorSyncOptions,
) (bool, outputSyncResult) {
	out := outputSyncResult{}
	newSchema := fn.OutputSchema
	reqStateKey, reqShape := requiredOutputStateKey(page, binding)
	changed := false

	if binding.Selectors != nil {
		for _, assignment := range binding.Selectors.Output {
			entry := SelectorSyncOutputEntry{
				StateKey: assignment.StateKey,
				Source:   assignment.Source,
				Required: strings.TrimSpace(assignment.StateKey) == reqStateKey && reqStateKey != "",
			}
			sourceExists := len(newSchema) == 0 || schemaHasPath(newSchema, assignment.Source)
			shapeOK := outputShapeMatchesSchema(assignment.Shape, newSchema, assignment.Source)
			switch {
			case sourceExists && shapeOK:
				out.assignments = append(out.assignments, assignment)
				entry.Action = SelectorSyncKept
				entry.Reason = "source exists and shape matches"
			case sourceExists && !shapeOK:
				updated, _, _, ok := rechooseOutputSource(binding.Usage, assignment, newSchema, prevDiff, reqShape, opts, true)
				if entry.Required && ok {
					out.assignments = append(out.assignments, updated)
					entry.Action = SelectorSyncRenamed
					entry.NewSource = updated.Source
					entry.Confidence = SelectorSyncConfidenceHigh
					entry.Reason = "required output source re-derived to match the new schema shape"
				} else if !entry.Required {
					if newShape, ok2 := scalarizedOutputShape(newSchema, assignment.Source); ok2 && newShape != "" {
						updated := assignment
						updated.Shape = newShape
						out.assignments = append(out.assignments, updated)
						entry.Action = SelectorSyncShapeUpdated
						entry.NewShape = newShape
						entry.Reason = "output source type changed; shape updated to match"
						changed = true
					} else {
						out.assignments = append(out.assignments, assignment)
						entry.Action = SelectorSyncTypeChanged
						entry.Reason = "output source type changed; kept for manual review"
					}
				} else {
					// 必需但推导不出：保留原 assignment，绝不摘成缺失
					out.assignments = append(out.assignments, assignment)
					entry.Action = SelectorSyncManual
					entry.Reason = "required output source shape no longer matches and could not be re-derived; map it manually"
				}
			default:
				// source 消失
				updated, confidence, reason, ok := rechooseOutputSource(binding.Usage, assignment, newSchema, prevDiff, reqShape, opts, false)
				if ok {
					out.assignments = append(out.assignments, updated)
					entry.Action = SelectorSyncRenamed
					entry.NewSource = updated.Source
					entry.Confidence = confidence
					entry.Reason = reason
					changed = true
				} else if entry.Required {
					out.assignments = append(out.assignments, assignment)
					entry.Action = SelectorSyncManual
					entry.Reason = "required output source no longer exists and could not be re-derived; map it manually"
				} else {
					entry.Action = SelectorSyncRemoved
					entry.Reason = "output source no longer exists in the schema"
					changed = true
				}
			}
			out.entries = append(out.entries, entry)
		}
	}

	// 4. 必需输出整体缺失 → 重推导补一条
	if reqStateKey != "" && !outputHasStateKey(out.assignments, reqStateKey, reqShape) {
		placeholder := OutputAssignment{StateKey: reqStateKey, Source: "", Shape: reqShape}
		entry := SelectorSyncOutputEntry{StateKey: reqStateKey, Required: true}
		if updated, confidence, reason, ok := rechooseOutputSource(binding.Usage, placeholder, newSchema, prevDiff, reqShape, opts, false); ok {
			out.assignments = append(out.assignments, updated)
			entry.Action = SelectorSyncAdded
			entry.NewSource = updated.Source
			entry.Confidence = confidence
			entry.Reason = reason
			changed = true
		} else {
			entry.Action = SelectorSyncManual
			entry.Reason = "required output assignment is missing and could not be derived; map it manually"
		}
		out.entries = append(out.entries, entry)
	}
	return changed, out
}

// requiredOutputStateKey 与 ValidateRequiredOutputAssignments 的必需
// stateKey 矩阵保持同步。taskResult 为任意 shape（reqShape 传空）。
func requiredOutputStateKey(page PageSpec, binding PageFunctionBinding) (string, OutputResultShape) {
	switch {
	case page.Type == PageTypeResource && binding.Usage == BindingUsageQuery:
		return "items", OutputShapeCollection
	case page.Type == PageTypeResource && binding.Usage == BindingUsageDetail:
		return "detail", OutputShapeObject
	case page.Type == PageTypeReport && binding.Usage == BindingUsageReport:
		return "dataset", OutputShapeDataset
	case page.Type == PageTypeTask && binding.Usage == BindingUsageTaskStatus:
		return "taskStatus", OutputShapeObject
	case page.Type == PageTypeTask && binding.Usage == BindingUsageTaskEvents:
		return "taskEvents", OutputShapeCollection
	case page.Type == PageTypeTask && binding.Usage == BindingUsageTaskResult:
		return "taskResult", ""
	default:
		return "", ""
	}
}

func outputHasStateKey(assignments []OutputAssignment, stateKey string, shape OutputResultShape) bool {
	for _, assignment := range assignments {
		if strings.TrimSpace(assignment.StateKey) != stateKey {
			continue
		}
		if shape == "" || assignment.Shape == shape {
			return true
		}
	}
	return false
}

// rechooseOutputSource 为输出 assignment 重新推导 source（shape 不符时
// 传 shapeMismatch=true 跳过 rename 候选；source 消失走全阶梯）：
//  1. prev diff rename 候选（唯一命中，prevTrusted 时 high）
//  2. RecomputeDefaults 的默认推导中找同 stateKey 的条目
//  3. 根对象 fallback：Source=""（整对象），当根类型与期望 shape 匹配
//     （generator 对 taskStatus 就是这么生成的）
func rechooseOutputSource(
	usage PageBindingUsage,
	assignment OutputAssignment,
	newSchema JSONSchema,
	prevDiff SchemaDiffResult,
	reqShape OutputResultShape,
	opts SelectorSyncOptions,
	shapeMismatch bool,
) (OutputAssignment, SelectorSyncConfidence, string, bool) {
	// 1. rename 候选（仅 source 消失场景有意义）
	if !shapeMismatch && len(prevDiff.RenameCandidates) > 0 {
		var usable []FieldRenameCandidate
		for _, candidate := range prevDiff.RenameCandidates {
			if candidate.OldPath != assignment.Source {
				continue
			}
			if !schemaHasPath(newSchema, candidate.NewPath) {
				continue
			}
			usable = append(usable, candidate)
		}
		if len(usable) == 1 && outputShapeMatchesSchema(assignment.Shape, newSchema, usable[0].NewPath) {
			updated := assignment
			updated.Source = usable[0].NewPath
			confidence := SelectorSyncConfidenceLow
			reason := "heuristically renamed by the previous schema diff (previous schema unverified)"
			if opts.PrevTrusted {
				confidence = SelectorSyncConfidenceHigh
				reason = "renamed according to the verified previous schema diff"
			}
			return updated, confidence, reason, true
		}
	}

	// 2. generator 默认推导
	if opts.RecomputeDefaults != nil {
		for _, candidate := range opts.RecomputeDefaults(usage, newSchema) {
			if strings.TrimSpace(candidate.StateKey) != strings.TrimSpace(assignment.StateKey) {
				continue
			}
			if reqShape != "" && candidate.Shape != reqShape {
				continue
			}
			updated := assignment
			updated.Source = candidate.Source
			if candidate.Shape != "" {
				updated.Shape = candidate.Shape
			}
			if updated.Source != "" && len(newSchema) > 0 && !schemaHasPath(newSchema, updated.Source) {
				continue
			}
			return updated, SelectorSyncConfidenceLow, "re-derived from the generator default output mapping", true
		}
	}

	// 3. 根对象 fallback（Source="" 表示整对象输出）
	if len(newSchema) > 0 && outputShapeMatchesSchema(assignment.Shape, newSchema, "") {
		if rootType, ok := schemaTypeAtPath(newSchema, ""); ok && rootType != "" {
			updated := assignment
			updated.Source = ""
			return updated, SelectorSyncConfidenceHigh, "required output source re-derived from the schema root", true
		}
	}
	return OutputAssignment{}, "", "", false
}

// scalarizedOutputShape 按新 schema 的 source 类型映射安全 shape：
// array→collection、object→object、其余 scalar。task/dataset 语义特殊
// 不自动猜（返回 false）。
func scalarizedOutputShape(newSchema JSONSchema, source string) (OutputResultShape, bool) {
	sourceType, ok := schemaTypeAtPath(newSchema, source)
	if !ok || sourceType == "" {
		return "", false
	}
	switch sourceType {
	case "array":
		return OutputShapeCollection, true
	case "object":
		return OutputShapeObject, true
	case "string", "number", "integer", "boolean":
		return OutputShapeScalar, true
	default:
		return "", false
	}
}
