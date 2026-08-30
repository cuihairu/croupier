package generator

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/model"
)

// CompositeSectionInput 是组合页的一个区块输入：函数 + 视图形态 +
// 联动声明。函数契约决定 selector 骨架，视图参数可覆盖。
type CompositeSectionInput struct {
	FunctionID string
	View       string // table|fields|form|actions|toolbar
	Title      string
	Span       int
	AutoRun    bool
	RefreshOn  []string
	Display    string                        // ""(inline) | dialog
	RowActions []CompositeRowActionInput     // view=table
	Toolbar    []CompositeToolbarActionInput // view=toolbar
	OnSuccess  []string
}

// CompositeRowActionInput 表格行操作输入。
type CompositeRowActionInput struct {
	Label         string
	TargetSection string
	Params        map[string]string
	Danger        bool
}

// CompositeToolbarActionInput 工具栏按钮输入。
type CompositeToolbarActionInput struct {
	Label         string
	TargetSection string
	Params        map[string]string
	Danger        bool
}

// GenerateCompositePage 生成自由组合页：每区块绑定一个函数
// （table=查询列表 / fields=详情键值 / form=输入操作 / actions=按钮组），
// 区块间通过 page_state 联动——函数输出写 stateKey（OutputAssignment），
// 依赖区块声明 RefreshOn 在状态变化时自动重跑。
//
// selector 骨架按函数契约生成：必填输入映射 page_state（联动键或区块 key），
// 输出映射 {stateKey: section.key}。
func GenerateCompositePage(
	pageKey string,
	inputs []CompositeSectionInput,
	contracts []*model.FunctionContract,
	opts GenerateOptions,
) (spec.GeneratedPageSpec, bool) {
	opts = normalizeOptions(opts)
	pageKey = strings.TrimSpace(pageKey)
	if pageKey == "" || len(inputs) == 0 {
		return spec.GeneratedPageSpec{}, false
	}
	locale := opts.DefaultLocale

	byID := map[string]*model.FunctionContract{}
	for _, c := range contracts {
		if c != nil && strings.TrimSpace(c.FunctionID) != "" {
			byID[strings.TrimSpace(c.FunctionID)] = c
		}
	}

	sections := []spec.CompositeSection{}
	bindings := []spec.PageFunctionBinding{}
	var diags []spec.Diagnostic

	for _, in := range inputs {
		fid := strings.TrimSpace(in.FunctionID)
		contract, ok := byID[fid]
		if !ok {
			diags = append(diags, spec.Diagnostic{
				Field:   "composite.sections." + fid,
				Code:    "function_missing",
				Message: fmt.Sprintf("function %s not found; section skipped", fid),
			})
			continue
		}
		key := sanitizeSourceKey(fid)
		view := in.View
		if view == "" {
			view = defaultCompositeView(contract)
		}

		section := spec.CompositeSection{
			Key:              key,
			BindingID:        key,
			Title:            spec.LocalizedText{locale: firstNonEmptyStr(in.Title, fid)},
			View:             view,
			Span:             in.Span,
			AutoRun:          in.AutoRun,
			RefreshOn:        in.RefreshOn,
			Display:          in.Display,
			OnSuccessRefresh: in.OnSuccess,
		}
		if len(in.Toolbar) > 0 {
			tb := &spec.CompositeToolbarSpec{}
			for _, ta := range in.Toolbar {
				tb.Actions = append(tb.Actions, spec.CompositeToolbarAction{
					Label:         spec.LocalizedText{locale: ta.Label},
					TargetSection: ta.TargetSection,
					Params:        ta.Params,
					Danger:        ta.Danger,
				})
			}
			section.Toolbar = tb
		}
		if view == "table" {
			lv := buildListViewFromContract(contract, nil)
			if lv != nil {
				section.Table = &spec.CompositeTableSpec{
					Columns:     lv.Columns,
					Pagination:  lv.Pagination,
					RowSchema:   lv.RowSchema,
					IdentityKey: lv.IdentityKey,
				}
			}
		}
		if view == "table" && len(in.RowActions) > 0 {
			if section.Table == nil {
				section.Table = &spec.CompositeTableSpec{}
			}
			for _, ra := range in.RowActions {
				section.Table.RowActions = append(section.Table.RowActions, spec.CompositeRowAction{
					Label:         spec.LocalizedText{locale: ra.Label},
					TargetSection: ra.TargetSection,
					Params:        ra.Params,
					Danger:        ra.Danger,
				})
			}
		}
		if view == "form" || view == "actions" {
			// 弹窗/行内表单渲染需要输入 schema（FormPresentationSpec）
			if fp := buildFormFromContract(contract); fp != nil {
				section.Form = fp
			}
		}

		binding := spec.PageFunctionBinding{
			ID:         key,
			FunctionID: fid,
			Usage:      compositeUsage(view),
			Execution: spec.PageBindingExecution{
				Mode:           spec.PageExecutionModeSync,
				RequireConfirm: view == "form" && strings.Contains(contract.Risk.String(), "danger"),
			},
		}
		selectors := &spec.BindingSelectors{}
		if len(contract.InputSchema) > 0 {
			selectors.Input = compositeInputSelector(spec.JSONSchema(contract.InputSchema), key)
		}
		selectors.Output = compositeOutputAssignments(contract, view, key)
		if len(selectors.Input.Assignments) > 0 || len(selectors.Output) > 0 {
			binding.Selectors = selectors
		}

		sections = append(sections, section)
		bindings = append(bindings, binding)
	}

	if len(sections) == 0 {
		return spec.GeneratedPageSpec{Diagnostics: diags}, false
	}

	title := spec.LocalizedText{locale: fallbackLabel(strings.ReplaceAll(pageKey, "--", " "))}
	return spec.GeneratedPageSpec{
		PageSpec: spec.PageSpec{
			PageKey:    pageKey,
			Type:       spec.PageTypeComposite,
			Title:      title,
			Category:   spec.PageCategorySpec{Key: "composite", Labels: spec.LocalizedText{locale: "组合"}},
			Navigation: &spec.NavigationSpec{Title: title},
			Composite:  &spec.CompositePageSpec{Sections: sections},
			Bindings:   bindings,
		},
		Quality:     spec.GeneratedPageQualityBasic,
		Diagnostics: diags,
	}, true
}

// defaultCompositeView 按函数能力推导默认视图。
func defaultCompositeView(c *model.FunctionContract) string {
	switch c.Capability.String() {
	case "collection_query":
		return "table"
	case "item_query":
		return "fields"
	default:
		return "form"
	}
}

// compositeUsage 按视图推导 binding usage。
func compositeUsage(view string) spec.PageBindingUsage {
	switch view {
	case "table", "fields":
		return spec.BindingUsageQuery
	default:
		return spec.BindingUsageAction
	}
}

// compositeInputSelector：必填字段缺省映射 page_state.<sectionKey>（联动）。
// 联动键即区块 key——上游区块的输出写同名 stateKey。
func compositeInputSelector(schema spec.JSONSchema, sectionKey string) spec.SelectorAST {
	ast := spec.SelectorAST{}
	for _, target := range sortedRequired(requiredProperties(schema)) {
		// page_state 无嵌套 path（composite 状态按区块整体存储，渲染层
		// 做同名字段合并），selector 声明目标字段即可。
		ast.Assignments = append(ast.Assignments, spec.InputAssignment{
			Target: "/" + target,
			Source: spec.ValueSource{
				Kind: spec.SourcePageState,
				Key:  sectionKey,
			},
		})
	}
	return ast
}

// compositePageStateKeys 用于 composite 输入的 page_state 上下文：区块
// key 即状态键。

// requiredProperties 提取顶层 required 字段名。
func requiredProperties(schema spec.JSONSchema) map[string]bool {
	out := map[string]bool{}
	var obj map[string]interface{}
	if err := json.Unmarshal(schema, &obj); err != nil {
		return out
	}
	rawReq, ok := obj["required"].([]interface{})
	if !ok {
		return out
	}
	for _, r := range rawReq {
		if s, ok := r.(string); ok {
			out[s] = true
		}
	}
	return out
}

// compositeOutputAssignments：输出根/常用字段写 stateKey=sectionKey，
// 供下游区块（RefreshOn: [sectionKey]）消费。
func compositeOutputAssignments(c *model.FunctionContract, view, sectionKey string) []spec.OutputAssignment {
	// 统一 object：页面状态以整个响应对象存储（table 视图在渲染层
	// 读 data.items）。collection shape 要求 schema 顶层是数组，与
	// 业务函数的 {items,total} 包装形态不符，会被发布校验拒绝。
	_ = view
	return []spec.OutputAssignment{
		{StateKey: sectionKey, Source: "", Shape: spec.OutputShapeObject},
	}
}

func sortedRequired(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j] < out[j-1]; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}

func firstNonEmptyStr(values ...string) string {
	for _, v := range values {
		if s := strings.TrimSpace(v); s != "" {
			return s
		}
	}
	return ""
}
