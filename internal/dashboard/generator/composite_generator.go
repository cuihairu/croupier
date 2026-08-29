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
	View       string // table|fields|form|actions
	Title      string
	Span       int
	AutoRun    bool
	RefreshOn  []string
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
			Key:       key,
			BindingID: key,
			Title:     spec.LocalizedText{locale: firstNonEmptyStr(in.Title, fid)},
			View:      view,
			Span:      in.Span,
			AutoRun:   in.AutoRun,
			RefreshOn: in.RefreshOn,
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
		ast.Assignments = append(ast.Assignments, spec.InputAssignment{
			Target: "/" + target,
			Source: spec.ValueSource{
				Kind: spec.SourcePageState,
				Key:  sectionKey,
				Path: "/" + target,
			},
		})
	}
	return ast
}

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
	shape := spec.OutputShapeObject
	if view == "table" {
		shape = spec.OutputShapeCollection
	}
	return []spec.OutputAssignment{
		{StateKey: sectionKey, Source: "", Shape: shape},
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
