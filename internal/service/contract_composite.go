package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"runtime/debug"
	"strings"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/dashboard/generator"
	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/model"
	"gorm.io/gorm"
)

// CreateCompositeProposal 手动创建组合页提案：每区块选一个函数
// （任意资源）+ 视图形态 + 联动声明，生成自由组合页提案进
// ProposalInbox——区块间经 page_state 联动（选玩家→联动刷新
// 订单/背包等）。
func (s *ContractService) CreateCompositeProposal(
	ctx context.Context,
	gameID, env, pageKey string,
	sections []CompositeSectionRequest,
	componentTemplates []spec.ComponentTemplateUsage,
) (proposal *model.PageProposal, err error) {
	// 组合页创建链路重构——panic 显式转为错误（含栈）返回前端，避免
	// 500 空响应无诊断。生成段由 BuildCompositePageSpec 自守卫；此处守卫
	// upsertGeneratedProposal 持久化段（nil model 等确定性空指针）。
	defer func() {
		if r := recover(); r != nil {
			slog.Error("CreateCompositeProposal panic", "recover", r, "stack", string(debug.Stack()))
			err = fmt.Errorf("internal error creating composite page: %v", r)
		}
	}()
	generated, contracts, err := s.BuildCompositePageSpec(ctx, gameID, env, pageKey, sections, componentTemplates)
	if err != nil {
		return nil, err
	}
	proposalKey := compositeProposalKey(pageKey)
	if err := s.upsertGeneratedProposal(ctx, gameID, env, proposalKey, nil, contracts, generated); err != nil {
		return nil, err
	}
	return s.proposalModel.FindByScopeAndKey(ctx, gameID, env, proposalKey)
}

// sectionValidationErrors 组合页区块输入校验失败 → 结构化 400
// （error=validation_failed + details）。details 以请求体 wire 字段路径为键
// （sections[<i>].key 等），预览/保存入口据此把原因逐条内联展示到诊断列表，
// 而不是只有一条笼统 message 让前端无从定位。
func sectionValidationErrors(message string, details map[string]any) error {
	return errorx.NewBadRequestWithCode("validation_failed", message, details)
}

// sectionPath 区块输入字段路径，与请求体 wire 形态一致。
func sectionPath(index int, field string) string {
	if field == "" {
		return fmt.Sprintf("sections[%d]", index)
	}
	return fmt.Sprintf("sections[%d].%s", index, field)
}

// diagnosticsToDetails 生成诊断 → details（field 作键、message 作值，缺 field
// 时回退诊断 code）。
func diagnosticsToDetails(diagnostics []spec.Diagnostic) map[string]any {
	details := map[string]any{}
	for _, d := range diagnostics {
		key := strings.TrimSpace(d.Field)
		if key == "" {
			key = d.Code
		}
		if key == "" {
			continue
		}
		details[key] = d.Message
	}
	return details
}

// BuildCompositePageSpec 生成组合页 PageSpec 而不落库：供编辑器实时预览消费
// 与「保存→发布」同一份生成链——区块契约解析、static 区块交错、模板快照与
// 发布级 selector 校验全部共用，预览产物与提案/发布产物逐字段一致。
// 仅只读副作用（查函数契约与术语词表），不产生提案/草稿/审计。
// 返回生成产物（含 quality/diagnostics）与参与生成的契约，供持久化段复用。
func (s *ContractService) BuildCompositePageSpec(
	ctx context.Context,
	gameID, env, pageKey string,
	sections []CompositeSectionRequest,
	componentTemplates []spec.ComponentTemplateUsage,
) (generated spec.GeneratedPageSpec, contracts []*model.FunctionContract, err error) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("BuildCompositePageSpec panic", "recover", r, "stack", string(debug.Stack()))
			err = fmt.Errorf("internal error building composite page: %v", r)
		}
	}()

	gameID = strings.TrimSpace(gameID)
	env = strings.TrimSpace(env)
	pageKey = strings.TrimSpace(pageKey)
	// 上传即成页（D1）：单区块（单函数）组合页合法——仅要求 pageKey 非空。
	if pageKey == "" {
		return spec.GeneratedPageSpec{}, nil, sectionValidationErrors("pageKey is required", map[string]any{"pageKey": "pageKey is required"})
	}

	inputs := make([]generator.CompositeSectionInput, 0, len(sections))
	staticSections := make([]spec.CompositeSection, 0)
	// 每个 static 区块在输入流中的位置（其前面的非 static 输入数）——
	// 生成后按此位置交错合并，保持「常量表单在前」等输入顺序。
	staticPos := make([]int, 0)
	genCount := 0
	seenKey := map[string]bool{}
	contractCache := map[string]*model.FunctionContract{}
	for i, sec := range sections {
		key := strings.TrimSpace(sec.Key)
		if sec.Static {
			// 常量表单：不查契约、不生成绑定；表单 schema 由编辑器透传
			if key == "" {
				return spec.GeneratedPageSpec{}, nil, sectionValidationErrors("static section requires key", map[string]any{sectionPath(i, "key"): "static section requires key"})
			}
			if seenKey[key] {
				msg := fmt.Sprintf("duplicate section key %q", key)
				return spec.GeneratedPageSpec{}, nil, sectionValidationErrors(msg, map[string]any{sectionPath(i, "key"): msg})
			}
			seenKey[key] = true
			if sec.Form == nil || len(sec.Form.JSONSchema) == 0 {
				return spec.GeneratedPageSpec{}, nil, sectionValidationErrors(fmt.Sprintf("static section %q requires form.jsonSchema", key), map[string]any{sectionPath(i, "form.jsonSchema"): "static section requires form.jsonSchema"})
			}
			section := spec.CompositeSection{
				Key:       key,
				View:      "form",
				Title:     spec.LocalizedText{"zh-CN": strings.TrimSpace(sec.Title)},
				Span:      sec.Span,
				AutoRun:   false,
				RefreshOn: sec.RefreshOn,
				Static:    true,
				Form:      sec.Form,
				// 弹窗/页签/卡片标注透传（display=tab 同 group 渲染进同一
				// Tabs、tab 聚合对应页；display=card 同 group 渲染进同一卡片）
				// ——static 区块不经过生成器，须在此落位。
				Display:     sec.Display,
				Group:       sec.Group,
				Tab:         localizedTextOf(sec.Tab),
				CardTitle:   localizedTextOf(sec.CardTitle),
				VisibleWhen: sec.VisibleWhen,
				// 级联失败策略（U9）：static 区块不执行但可被下游 refreshOn
				// 引用，字段同样透传保 round-trip。
				CascadePolicy: sec.CascadePolicy,
			}
			staticSections = append(staticSections, section)
			staticPos = append(staticPos, genCount)
			continue
		}
		fid := strings.TrimSpace(sec.FunctionID)
		if fid == "" {
			continue
		}
		// 同函数多实例：不再丢弃，靠区块 key 唯一区分
		//（同一 player.list 可拖两个表格分别配列/联动）。
		if key == "" {
			key = fid
		}
		if seenKey[key] {
			msg := fmt.Sprintf("duplicate section key %q (同函数多实例请提供不同 key)", key)
			return spec.GeneratedPageSpec{}, nil, sectionValidationErrors(msg, map[string]any{sectionPath(i, "key"): msg})
		}
		seenKey[key] = true
		contract, ok := contractCache[fid]
		if !ok {
			var err error
			contract, err = s.contractModel.FindByScopeAndFunctionID(ctx, gameID, env, fid)
			if err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					// 区块引用的函数未登记契约：画布配置问题，可定位到
					// sections[i].functionId（DB 故障仍走 500 兜底）。
					msg := fmt.Sprintf("function %s contract not found", fid)
					return spec.GeneratedPageSpec{}, nil, sectionValidationErrors(msg, map[string]any{sectionPath(i, "functionId"): msg})
				}
				return spec.GeneratedPageSpec{}, nil, fmt.Errorf("function %s contract not found: %w", fid, err)
			}
			contractCache[fid] = contract
		}
		contracts = append(contracts, contract)
		in := generator.CompositeSectionInput{
			Key: key,
			// Group 弹窗分组必须透传：渲染端 openDialog 按 group 聚合同弹窗
			// 区块（PageRenderer groupOf），丢失后弹窗永远无法打开；
			// tab 组名同理由渲染端按 group 聚合 Tabs。
			Group:       sec.Group,
			FunctionID:  fid,
			View:        sec.View,
			Title:       sec.Title,
			Span:        sec.Span,
			AutoRun:     sec.AutoRun,
			RefreshOn:   sec.RefreshOn,
			Display:     sec.Display,
			Tab:         sec.Tab,
			CardTitle:   sec.CardTitle,
			Columns:     sec.Columns,
			Pagination:  sec.Pagination,
			Selection:   sec.Selection,
			OnSuccess:   sec.OnSuccessRefresh,
			Events:      convEvents(sec.Events),
			VisibleWhen: sec.VisibleWhen,
			// 级联失败策略（U9）：编辑器透传，渲染端按策略处理上游失败。
			CascadePolicy: sec.CascadePolicy,
		}
		for _, ra := range sec.RowActions {
			in.RowActions = append(in.RowActions, generator.CompositeRowActionInput{
				Label:         ra.Label,
				TargetSection: ra.TargetSection,
				Params:        ra.Params,
				Danger:        ra.Danger,
				Chain:         convChain(ra.Chain),
			})
		}
		for _, ta := range sec.ToolbarActions {
			in.Toolbar = append(in.Toolbar, generator.CompositeToolbarActionInput{
				Label:         ta.Label,
				TargetSection: ta.TargetSection,
				Params:        ta.Params,
				Danger:        ta.Danger,
				Chain:         convChain(ta.Chain),
			})
		}
		for _, ia := range sec.InputAssignments {
			src := spec.ValueSource{Kind: spec.ValueSourceKind(ia.Kind)}
			switch src.Kind {
			case spec.SourcePageState:
				src.Key = ia.Key
				src.Path = ia.Path
			case spec.SourceRow, spec.SourceForm, spec.SourceDetail:
				src.Path = ia.Path
			case spec.SourceLiteral:
				src.Value = ia.Value
			}
			// U8 transform 透传：缺此处会让编辑器编译产出的 default/rename
			// wire 在提案入口被静默丢弃（执行端永远收不到变换）。
			src.Transform = ia.Transform
			in.InputAssignments = append(in.InputAssignments, spec.InputAssignment{
				Target: ia.Target,
				Source: src,
			})
		}
		inputs = append(inputs, in)
		genCount++
	}

	opts := generator.DefaultGenerateOptions()
	opts.Terms = s.loadTermDictionary(ctx)
	generated, ok := generator.GenerateCompositePage(pageKey, inputs, contracts, opts)
	if !ok {
		// 生成器拒绝（function_missing 等）时诊断已带原因，随 details 返回；
		// 空输入流（全 static 区块或全空 functionId）无诊断，给出兜底原因。
		details := diagnosticsToDetails(generated.Diagnostics)
		if len(details) == 0 {
			details = map[string]any{"sections": "no functional section to generate (all sections static or functionId empty)"}
		}
		msg := "composite page cannot be generated (see diagnostics)"
		return spec.GeneratedPageSpec{}, nil, sectionValidationErrors(msg, details)
	}
	// 常量表单区块（static）：不经过生成器（无契约/绑定），按输入流原位置
	// 交错合并进生成 sections——static 在前的输入不应被追加到页尾；
	// 表单 schema 由编辑器设计期定义并透传。
	if len(staticSections) > 0 {
		// GenerateCompositePage 的唯一成功 return 恒构造
		// Composite: &spec.CompositePageSpec{...}（ok=false 已在上方返回
		// 错误），原 Composite==nil 防御分支为死代码，已删。
		gen := generated.Composite.Sections
		merged := make([]spec.CompositeSection, 0, len(gen)+len(staticSections))
		gi := 0
		for i, s := range staticSections {
			// 先取该 static 位置之前的生成区块（生成器与输入同序；
			// function_missing 跳项时 gi 边界自然收敛）
			for gi < staticPos[i] && gi < len(gen) {
				merged = append(merged, gen[gi])
				gi++
			}
			merged = append(merged, s)
		}
		merged = append(merged, gen[gi:]...)
		generated.Composite.Sections = merged
	}
	// 页面级模板快照（U11 更新提醒）：编辑器登记的 key+digest 原样透传进
	// PageSpec，随提案→草稿→发布全程 JSON 持久（回读比对用，不参与校验）。
	if len(componentTemplates) > 0 {
		seenTpl := map[string]bool{}
		normalized := make([]spec.ComponentTemplateUsage, 0, len(componentTemplates))
		for _, usage := range componentTemplates {
			key := strings.TrimSpace(usage.Key)
			if key == "" || seenTpl[key] {
				continue
			}
			seenTpl[key] = true
			normalized = append(normalized, spec.ComponentTemplateUsage{Key: key, Digest: strings.TrimSpace(usage.Digest)})
		}
		if len(normalized) > 0 {
			generated.ComponentTemplates = normalized
		}
	}
	// 统一校验规则（单一规则源）：提案创建即运行发布级 selector 校验——与
	// AcceptAndPublishProposal 的硬门槛共用 CollectBindingSelectorIssues。
	// 违规以 error 级诊断前置暴露（Proposal Inbox「需要处理」可见），质量
	// 降级为 needs_review；不再出现「保存看似可发布、点发布才 422」。
	if issues := CollectBindingSelectorIssues(FunctionSpecsFromContracts(contracts), generated.PageSpec); len(issues) > 0 {
		for field, message := range issues {
			generated.Diagnostics = append(generated.Diagnostics, spec.Diagnostic{
				Code:     "publish_validation_failed",
				Severity: spec.SeverityError,
				Message:  message,
				Field:    field,
			})
		}
		generated.Quality = spec.GeneratedPageQualityNeedsReview
	}
	return generated, contracts, nil
}

// CompositeSectionRequest 组合页区块输入。
type CompositeSectionRequest struct {
	// Key 区块唯一标识（同函数多实例时必填以区分；缺省=函数 id）。
	Key        string   `json:"key,omitempty"`
	FunctionID string   `json:"functionId"`
	View       string   `json:"view"` // table|fields|form|actions|toolbar（空=按能力推导）
	Title      string   `json:"title,omitempty"`
	Span       int      `json:"span,omitempty"`
	AutoRun    bool     `json:"autoRun,omitempty"`
	RefreshOn  []string `json:"refreshOn,omitempty"`
	// Group 弹窗分组：display=dialog 且同 group 的区块渲染进同一弹窗；
	// display=tab 时为页签组名（同组渲染进同一 Tabs）。
	Group string `json:"group,omitempty"`
	// Tab 页签标签（display=tab）：同 group 内按标签聚合到 Tabs 对应页。
	Tab string `json:"tab,omitempty"`
	// CardTitle 卡片分组标题（display=card）：同 group 区块渲染进同一
	// 卡片时的组标题；缺省回退组名。
	CardTitle string `json:"cardTitle,omitempty"`
	// Columns 显式展示列覆盖（view=table）：nil=契约默认；空数组=隐藏全部。
	Columns []string `json:"columns,omitempty"`
	// Pagination 显式分页配置（view=table）：nil=生成器默认（组合页关闭
	// 服务端分页）；enabled=true 时落位到区块 table.pagination，发布态
	// 表格启用服务端分页（defaultSize/pageSizes 随配随传）。
	Pagination *spec.PaginationSpec `json:"pagination,omitempty"`
	// Selection 行选择配置（view=table，D2 批量目标核对）：nil 或
	// multiple=false=既有单选 radio；multiple=true 落位到区块
	// table.selection，发布态表格启用 checkbox 多选与选择栏。
	Selection *spec.CompositeSelectionSpec `json:"selection,omitempty"`
	// Static 常量表单：不绑定函数，Form.jsonSchema 由编辑器设计期定义，
	// 值仅写入页面状态供 refreshOn/动作链消费（不执行、无审计面）。
	Static bool `json:"static,omitempty"`
	// Form 静态区块的表单展示规格（Static=true 时必填；非静态区块由
	// 服务端从契约生成，忽略该字段）。
	Form *spec.FormPresentationSpec `json:"form,omitempty"`
	// InputAssignments 显式参数映射（覆盖按契约自动生成的同名 target）：
	// target=/参数路径；kind=page_state（key=上游区块 key，path=/字段）
	// 或 literal（value=固定值）。未声明的 required 参数仍走自动映射。
	InputAssignments []CompositeInputAssignmentRequest `json:"inputAssignments,omitempty"`
	// Display inline（默认）| dialog（弹窗表单，按钮触发）。
	Display string `json:"display,omitempty"`
	// RowActions 表格行操作（view=table）。
	RowActions []CompositeRowActionRequest `json:"rowActions,omitempty"`
	// ToolbarActions 工具栏按钮（view=toolbar）。
	ToolbarActions []CompositeToolbarActionRequest `json:"toolbarActions,omitempty"`
	// OnSuccessRefresh 操作成功后自动重跑的区块 key。
	OnSuccessRefresh []string `json:"onSuccessRefresh,omitempty"`
	// Events 通用事件绑定（发布触发点）。
	Events []EventBindingReq `json:"events,omitempty"`
	// VisibleWhen 区块级条件显示：按页面状态（ConditionSpec.Key=来源
	// 区块 key）求值，false 时不渲染该区块（执行不变）。
	VisibleWhen *spec.ConditionSpec `json:"visibleWhen,omitempty"`
	// CascadePolicy refreshOn 级联失败策略：上游依赖执行失败时本区块
	// 行为（clear|keep|pause；空=pause 缺省）。
	CascadePolicy string `json:"cascadePolicy,omitempty"`
}

// CompositeInputAssignmentRequest 显式参数映射条目：
// target=/参数路径；kind=page_state（key=上游区块 key，path=/字段）
// 或 literal（value=固定值）。
type CompositeInputAssignmentRequest struct {
	Target string          `json:"target"`
	Kind   string          `json:"kind"`
	Key    string          `json:"key,omitempty"`
	Path   string          `json:"path,omitempty"`
	Value  json.RawMessage `json:"value,omitempty"`
	// Transform 受控值变换（U8）：编辑器编译产 default（缺省兜底）/
	// rename（整对象改名）wire；透传到 spec 的 InputAssignments，
	// 执行端（console resolveSelectorValue）按 TransformSpec 应用。
	Transform *spec.TransformSpec `json:"transform,omitempty"`
}

// EventBindingReq 事件绑定请求。
type EventBindingReq struct {
	Event  string          `json:"event"`
	Action ActionStepReq   `json:"action"`
	Chain  []ActionStepReq `json:"chain,omitempty"`
}

// CompositeRowActionRequest 表格行操作输入。
type CompositeRowActionRequest struct {
	Label         string            `json:"label"`
	TargetSection string            `json:"targetSection,omitempty"`
	Params        map[string]string `json:"params,omitempty"`
	Danger        bool              `json:"danger,omitempty"`
	Chain         []ActionStepReq   `json:"chain,omitempty"`
}

// CompositeToolbarActionRequest 工具栏按钮输入。
type CompositeToolbarActionRequest struct {
	Label         string            `json:"label"`
	TargetSection string            `json:"targetSection,omitempty"`
	Params        map[string]string `json:"params,omitempty"`
	Danger        bool              `json:"danger,omitempty"`
	Chain         []ActionStepReq   `json:"chain,omitempty"`
}

// ActionStepReq 动作链单步。
type ActionStepReq struct {
	Kind   string            `json:"kind"`
	Target string            `json:"target"`
	Params map[string]string `json:"params,omitempty"`
}

// convChain 请求动作链 → generator 输入。
func convChain(steps []ActionStepReq) []spec.CompositeActionStep {
	if len(steps) == 0 {
		return nil
	}
	out := make([]spec.CompositeActionStep, 0, len(steps))
	for _, st := range steps {
		out = append(out, spec.CompositeActionStep{Kind: st.Kind, Target: st.Target, Params: st.Params})
	}
	return out
}

// localizedTextOf 页签标签 string → 系统默认语言单条目（同 Title 模式）。
func localizedTextOf(v string) spec.LocalizedText {
	if t := strings.TrimSpace(v); t != "" {
		return spec.LocalizedText{"zh-CN": t}
	}
	return nil
}

// convEvents 事件绑定请求 → generator 输入。
func convEvents(evs []EventBindingReq) []spec.CompositeEventBinding {
	if len(evs) == 0 {
		return nil
	}
	out := make([]spec.CompositeEventBinding, 0, len(evs))
	for _, ev := range evs {
		out = append(out, spec.CompositeEventBinding{
			Event:  ev.Event,
			Action: spec.CompositeActionStep{Kind: ev.Action.Kind, Target: ev.Action.Target, Params: ev.Action.Params},
			Chain:  convChain(ev.Chain),
		})
	}
	return out
}

func compositeProposalKey(pageKey string) string {
	return "composite--" + strings.TrimSpace(pageKey)
}
