package generator

import (
	"fmt"
	"strings"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/model"
)

// CompositeResourceInput 是组合页的一个资源条目。
type CompositeResourceInput struct {
	ResourceKey string
	Semantics   *model.CapabilitySemantics
	Contracts   []*model.FunctionContract
}

// GenerateCompositePage 从多个资源生成组合页：每资源复用单资源生成器
// 产出 view 与 bindings（bindingId 加 "<resourceKey>." 前缀，view 内
// BindingID 引用同步改写），Console 按 tab 渲染每资源视图。
//
// 至少需要 minResources 个可生成资源（有 collection_query + identity），
// 不可生成的资源记入诊断但不阻塞其余资源。
func GenerateCompositePage(pageKey string, inputs []CompositeResourceInput, opts GenerateOptions) (spec.GeneratedPageSpec, bool) {
	opts = normalizeOptions(opts)
	pageKey = strings.TrimSpace(pageKey)
	if pageKey == "" {
		return spec.GeneratedPageSpec{}, false
	}
	locale := opts.DefaultLocale

	blocks := []spec.CompositeResourceBlock{}
	bindings := []spec.PageFunctionBinding{}
	var diags []spec.Diagnostic
	failed := []string{}

	for _, in := range inputs {
		rk := strings.TrimSpace(in.ResourceKey)
		if rk == "" || in.Semantics == nil {
			continue
		}
		gen, ok := GenerateResourcePageProposal(in.Semantics, in.Contracts, opts)
		if !ok {
			failed = append(failed, rk)
			diags = append(diags, spec.Diagnostic{
				Field:   "composite.resources." + rk,
				Message: fmt.Sprintf("resource %s cannot generate a view (missing collection_query or identity); skipped", rk),
			})
			continue
		}

		// binding ID 前缀改写 + view 内引用同步
		prefix := rk + "."
		renamed := map[string]string{}
		for i := range gen.Bindings {
			old := gen.Bindings[i].ID
			newID := prefix + old
			renamed[old] = newID
			gen.Bindings[i].ID = newID
		}
		rewriteViewBindingRefs(&gen.PageSpec, renamed)
		bindings = append(bindings, gen.Bindings...)

		blockTitle := gen.Title
		if term, ok := opts.Terms.Lookup("resource", rk); ok && len(term) > 0 {
			blockTitle = term
		}
		blocks = append(blocks, spec.CompositeResourceBlock{
			ResourceKey: rk,
			Title:       blockTitle,
			View:        *gen.Resource,
		})
		diags = append(diags, gen.Diagnostics...)
	}

	if len(blocks) < 2 {
		if len(blocks) == 0 {
			return spec.GeneratedPageSpec{Diagnostics: diags}, false
		}
		diags = append(diags, spec.Diagnostic{
			Field:   "composite.resources",
			Message: "composite page expects 2+ generatable resources; produced a single block",
		})
	}
	if len(failed) > 0 {
		diags = append(diags, spec.Diagnostic{
			Field:   "composite.skipped",
			Message: "skipped resources: " + strings.Join(failed, ", "),
		})
	}

	title := spec.LocalizedText{locale: fallbackLabel(strings.ReplaceAll(pageKey, "--", " "))}
	return spec.GeneratedPageSpec{
		PageSpec: spec.PageSpec{
			PageKey:    pageKey,
			Type:       spec.PageTypeComposite,
			Title:      title,
			Category:   spec.PageCategorySpec{Key: "composite", Labels: spec.LocalizedText{locale: "组合"}},
			Navigation: &spec.NavigationSpec{Title: title},
			Composite:  &spec.CompositePageSpec{Resources: blocks},
			Bindings:   bindings,
		},
		Quality:     compositeQuality(blocks, diags),
		Diagnostics: diags,
	}, true
}

// rewriteViewBindingRefs 把 view 结构里的 BindingID 引用按改名映射重写。
func rewriteViewBindingRefs(p *spec.PageSpec, renamed map[string]string) {
	r := p.Resource
	if r == nil {
		return
	}
	sub := func(id string) string {
		if v, ok := renamed[id]; ok {
			return v
		}
		return id
	}
	if r.ListView != nil {
		for i, a := range r.ListView.RowActions {
			r.ListView.RowActions[i].BindingID = sub(a.BindingID)
		}
		for i, a := range r.ListView.BatchActions {
			r.ListView.BatchActions[i].BindingID = sub(a.BindingID)
		}
		for i, a := range r.ListView.ToolbarActions {
			r.ListView.ToolbarActions[i].BindingID = sub(a.BindingID)
		}
	}
	if r.DeleteAction != nil {
		r.DeleteAction.BindingID = sub(r.DeleteAction.BindingID)
	}
}

func compositeQuality(blocks []spec.CompositeResourceBlock, diags []spec.Diagnostic) spec.GeneratedPageQuality {
	hasErr := false
	for _, d := range diags {
		if strings.Contains(strings.ToLower(d.Message), "missing") || strings.Contains(strings.ToLower(d.Message), "invalid") {
			hasErr = true
			break
		}
	}
	switch {
	case len(blocks) >= 2 && !hasErr:
		return spec.GeneratedPageQualityReady
	case len(blocks) >= 1:
		return spec.GeneratedPageQualityBasic
	default:
		return spec.GeneratedPageQualityNeedsReview
	}
}
