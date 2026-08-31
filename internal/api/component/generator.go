// Package component 中的模板生成器：函数契约 → 默认组件模板。
//
// 设计：每个注册的函数都自动生成一个对应的组件模板（单函数组件），
// 用户可以直接拖入使用；用户也可以选中画布多个节点保存为新的
// 组合组件（多函数封装）。
package component

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/cuihairu/croupier/internal/dbenum"
	"github.com/cuihairu/croupier/internal/model"
)

// GenerateSingleFunctionTemplates 为一批函数契约生成默认组件模板。
// 每个函数一个组件：list→表格、get→字段卡、其他→表单。
// 已存在的 builtin 同 key 模板会被更新（幂等）。
func (h *Handler) GenerateSingleFunctionTemplates(ctx context.Context, contracts []*model.FunctionContract) error {
	for _, c := range contracts {
		if c == nil || strings.TrimSpace(c.FunctionID) == "" {
			continue
		}
		tpl := buildSingleFunctionTemplate(c)
		if tpl == nil {
			continue
		}
		if err := h.model.UpsertBuiltin(ctx, tpl); err != nil {
			slog.WarnContext(ctx, "component template generate", "functionId", c.FunctionID, "err", err)
		}
	}
	return nil
}

// buildSingleFunctionTemplate 从单个契约推导组件模板。
func buildSingleFunctionTemplate(c *model.FunctionContract) *model.ComponentTemplate {
	fid := c.FunctionID
	resource := c.ResourceKey
	if resource == "" {
		parts := strings.SplitN(fid, ".", 2)
		if len(parts) > 0 {
			resource = parts[0]
		}
	}

	// 推导视图类型
	view := "form" // 默认：表单
	switch c.Capability {
	case dbenum.CapabilityCollectionQuery:
		view = "table"
	case dbenum.CapabilityItemQuery:
		view = "fields"
	}

	// 构建最小 tree（与编辑器 PageNode 同构）
	tree := buildNodeJSON(fid, view, resource)

	// 名称
	name := fmt.Sprintf(`{"zh-CN":%q}`, resource+"·"+lastSegment(fid))
	desc := fmt.Sprintf(`{"zh-CN":%q}`, fid)

	return &model.ComponentTemplate{
		Key:               "fn--" + sanitizeKey(fid),
		Name:              model.JSON(name),
		Description:       model.JSON(desc),
		Category:          "函数组件",
		Icon:              iconForView(view),
		RequiredFunctions: model.JSON(fmt.Sprintf(`[%q]`, fid)),
		Tree:              model.JSON(tree),
		Builtin:           true,
	}
}

// buildNodeJSON 构建单函数组件的 PageNode JSON。
func buildNodeJSON(fid, view, resource string) string {
	var node string
	switch view {
	case "table":
		node = fmt.Sprintf(`[{"id":"fn","type":"fnTable","props":{"functionId":%q,"title":%q,"span":24,"autoRun":true,"columns":[],"rowActions":[]}}]`, fid, resource)
	case "fields":
		node = fmt.Sprintf(`[{"id":"fn","type":"fnFields","props":{"functionId":%q,"title":%q,"span":12,"autoRun":true}}]`, fid, resource)
	default:
		node = fmt.Sprintf(`[{"id":"fn","type":"fnForm","props":{"functionId":%q,"title":%q,"span":24,"display":"inline"}}]`, fid, resource)
	}
	return node
}

// GenerateCRUDTemplate 为具有完整 CRUD 能力的资源生成「资源管理」组合组件。
// 需要 collection_query + item_query + 至少一个写操作。
func (h *Handler) GenerateCRUDTemplate(ctx context.Context, resource string, contracts []*model.FunctionContract) error {
	var listFn, getFn, createFn, updateFn, deleteFn *model.FunctionContract
	for _, c := range contracts {
		if c.ResourceKey != resource {
			continue
		}
		switch {
		case c.Capability == dbenum.CapabilityCollectionQuery:
			listFn = c
		case c.Capability == dbenum.CapabilityItemQuery:
			getFn = c
		case c.Capability == dbenum.CapabilityCreate:
			createFn = c
		case c.Capability == dbenum.CapabilityUpdate:
			updateFn = c
		case c.Capability == dbenum.CapabilityDelete:
			deleteFn = c
		}
	}

	// 至少需要 list + get
	if listFn == nil || getFn == nil {
		return nil
	}

	var fns []string
	fns = append(fns, listFn.FunctionID, getFn.FunctionID)

	// 构建组合 tree：表格 + 详情弹窗 + 操作弹窗
	tree := buildCRUDTree(listFn, getFn, createFn, updateFn, deleteFn, &fns)

	name := fmt.Sprintf(`{"zh-CN":%q}`, resource+" 管理")
	desc := fmt.Sprintf(`{"zh-CN":%q}`, "列表查看 → 详情 → 增删改操作（自动生成）")

	tpl := &model.ComponentTemplate{
		Key:               "crud--" + sanitizeKey(resource),
		Name:              model.JSON(name),
		Description:       model.JSON(desc),
		Category:          "资源管理",
		Icon:              "AppstoreOutlined",
		RequiredFunctions: model.JSON(toJSONStringArray(fns)),
		Tree:              model.JSON(tree),
		Builtin:           true,
	}
	return h.model.UpsertBuiltin(ctx, tpl)
}

// buildCRUDTree 构建 CRUD 组合组件的 PageNode 树。
func buildCRUDTree(listFn, getFn, createFn, updateFn, deleteFn *model.FunctionContract, fns *[]string) string {
	nodes := []string{}

	// 主表格
	tableNode := fmt.Sprintf(`{"id":"table","type":"fnTable","props":{"functionId":%q,"title":%q,"span":24,"autoRun":true,"columns":[],"rowActions":[{"label":"查看详情","targetSection":"detail-modal"}]}}`,
		listFn.FunctionID, listFn.ResourceKey)
	nodes = append(nodes, tableNode)

	// 详情弹窗
	detailModal := fmt.Sprintf(`{"id":"detail-modal","type":"modal","props":{"title":%q,"width":"medium"},"children":[{"id":"detail-fields","type":"fnFields","props":{"functionId":%q,"span":24,"autoRun":true}}]}`,
		"详情", getFn.FunctionID)
	nodes = append(nodes, detailModal)

	// 新建弹窗（如有 create）
	if createFn != nil {
		createModal := fmt.Sprintf(`{"id":"create-modal","type":"modal","props":{"title":%q,"width":"medium"},"children":[{"id":"create-form","type":"fnForm","props":{"functionId":%q,"span":24,"display":"inline","onSuccess":{"kind":"refreshNode","target":"table"}}}]}`,
			"新建", createFn.FunctionID)
		nodes = append(nodes, createModal)
	}

	// 编辑弹窗（如有 update）
	if updateFn != nil {
		updateModal := fmt.Sprintf(`{"id":"edit-modal","type":"modal","props":{"title":%q,"width":"medium"},"children":[{"id":"edit-form","type":"fnForm","props":{"functionId":%q,"span":24,"display":"inline","onSuccess":{"kind":"refreshNode","target":"table"}}}]}`,
			"编辑", updateFn.FunctionID)
		nodes = append(nodes, updateModal)
	}

	return "[" + strings.Join(nodes, ",") + "]"
}

// RegenerateFromContracts 扫描全部契约，生成单函数模板 + CRUD 组合模板。
func (h *Handler) RegenerateFromContracts(ctx context.Context, contracts []*model.FunctionContract) error {
	// 1. 单函数模板
	if err := h.GenerateSingleFunctionTemplates(ctx, contracts); err != nil {
		return fmt.Errorf("single function templates: %w", err)
	}

	// 2. 按资源分组，生成 CRUD 模板
	byResource := make(map[string][]*model.FunctionContract)
	for _, c := range contracts {
		if c.ResourceKey != "" {
			byResource[c.ResourceKey] = append(byResource[c.ResourceKey], c)
		}
	}
	for resource := range byResource {
		if err := h.GenerateCRUDTemplate(ctx, resource, byResource[resource]); err != nil {
			slog.WarnContext(ctx, "crud template generate", "resource", resource, "err", err)
		}
	}

	return nil
}

// 辅助函数

func sanitizeKey(s string) string {
	var b strings.Builder
	for _, ch := range strings.ToLower(s) {
		if (ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') || ch == '-' || ch == '_' || ch == '.' {
			b.WriteRune(ch)
		} else {
			b.WriteByte('-')
		}
	}
	return b.String()
}

func lastSegment(s string) string {
	parts := strings.Split(s, ".")
	return parts[len(parts)-1]
}

func iconForView(view string) string {
	switch view {
	case "table":
		return "TableOutlined"
	case "fields":
		return "ProfileOutlined"
	default:
		return "FormOutlined"
	}
}

func toJSONStringArray(arr []string) string {
	var b strings.Builder
	b.WriteByte('[')
	for i, s := range arr {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(fmt.Sprintf("%q", s))
	}
	b.WriteByte(']')
	return b.String()
}
