// Package component 中的模板生成器：函数契约 → 默认组件模板。
//
// 设计：每个注册的函数都自动生成一个对应的组件模板（单函数组件），
// 用户可以直接拖入使用；用户也可以选中画布多个节点保存为新的
// 组合组件（多函数封装）。
package component

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sort"
	"strings"

	"github.com/cuihairu/croupier/internal/dbenum"
	"github.com/cuihairu/croupier/internal/model"
)

// buildSingleFunctionTemplateFn 是 buildSingleFunctionTemplate 的可注入缝隙
// （默认与生产行为一致，测试中替换以覆盖 tpl==nil 防御分支）。
var buildSingleFunctionTemplateFn = buildSingleFunctionTemplate

// generateSingleFunctionTemplates 是 Handler.GenerateSingleFunctionTemplates 的
// 可注入缝隙（默认与生产行为一致，测试中替换以覆盖错误传播分支）。
var generateSingleFunctionTemplates = func(h *Handler, ctx context.Context, contracts []*model.FunctionContract) error {
	return h.GenerateSingleFunctionTemplates(ctx, contracts)
}

// GenerateSingleFunctionTemplates 为一批函数契约生成默认组件模板。
// 每个函数一个组件：list→表格、get→字段卡、其他→表单。
// 已存在的 builtin 同 key 模板会被更新（幂等）。
func (h *Handler) GenerateSingleFunctionTemplates(ctx context.Context, contracts []*model.FunctionContract) error {
	for _, c := range contracts {
		if c == nil || strings.TrimSpace(c.FunctionID) == "" {
			continue
		}
		tpl := buildSingleFunctionTemplateFn(c)
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
		switch c.Capability {
		case dbenum.CapabilityCollectionQuery:
			listFn = c
		case dbenum.CapabilityItemQuery:
			getFn = c
		case dbenum.CapabilityCreate:
			createFn = c
		case dbenum.CapabilityUpdate:
			updateFn = c
		case dbenum.CapabilityDelete:
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
	desc := crudTemplateDescription(crudMissingCapabilities(createFn, updateFn, deleteFn))

	tpl := &model.ComponentTemplate{
		Key:               "crud--" + sanitizeKey(resource),
		Name:              model.JSON(name),
		Description:       desc,
		Category:          "资源管理",
		Icon:              "AppstoreOutlined",
		RequiredFunctions: model.JSON(toJSONStringArray(fns)),
		Tree:              model.JSON(tree),
		Builtin:           true,
	}
	return h.model.UpsertBuiltin(ctx, tpl)
}

// crudMissingCapabilities 列出 CRUD 模板未覆盖的写能力（能力名与 dbenum 的
// capability 机器标识一致）。缺能力的入口不生成（不在树里留空节点），缺口因此
// 必须显式声明，否则模板卡片看起来是完整 CRUD、实际只有读流程。
func crudMissingCapabilities(createFn, updateFn, deleteFn *model.FunctionContract) []string {
	missing := make([]string, 0, 3)
	for _, op := range []struct {
		cap dbenum.Capability
		fn  *model.FunctionContract
	}{
		{dbenum.CapabilityCreate, createFn},
		{dbenum.CapabilityUpdate, updateFn},
		{dbenum.CapabilityDelete, deleteFn},
	} {
		if op.fn == nil {
			missing = append(missing, op.cap.String())
		}
	}
	return missing
}

// crudTemplateDescription 生成 CRUD 模板描述：能力齐备时说明完整流程；缺写能力时
// 点名缺口。模板列表与「从模板开始」卡片都展示该描述，用户在实例化前即知流程
// 不完整，而不是靠「树里没有对应节点」自行推断。
func crudTemplateDescription(missing []string) model.JSON {
	if len(missing) == 0 {
		return model.JSON(`{"zh-CN":"列表查看 → 详情 → 增删改操作（自动生成）"}`)
	}
	return model.JSON(fmt.Sprintf(`{"zh-CN":"列表查看 → 详情（自动生成）；缺 %s 能力，未生成对应入口"}`,
		strings.Join(missing, "/")))
}

// crudInputKey 从契约输入 schema 推导身份/主键字段名：优先常见命名（id/uid/
// uuid），其次唯一属性，兜底首个属性名。用于把表格行字段映射进详情/编辑/删除
// 表单参数——缺这层映射，行操作打开的弹窗拿不到当前行身份。
func crudInputKey(c *model.FunctionContract) string {
	if c == nil {
		return "id"
	}
	var schema struct {
		Properties map[string]json.RawMessage `json:"properties"`
	}
	if err := json.Unmarshal(c.InputSchema, &schema); err != nil || len(schema.Properties) == 0 {
		return "id"
	}
	names := make([]string, 0, len(schema.Properties))
	for n := range schema.Properties {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, preferred := range []string{"id", "uid", "uuid"} {
		for _, n := range names {
			if n == preferred {
				return n
			}
		}
	}
	return names[0]
}

// buildCRUDTree 构建 CRUD 组合组件的 PageNode 树。行操作按「行身份字段」把当
// 前行映射进详情/编辑/删除表单；删除走危险确认弹窗；新建走表格工具栏入口。
// 缺能力的写操作不生成对应入口（不在树里留空节点）。
func buildCRUDTree(listFn, getFn, createFn, updateFn, deleteFn *model.FunctionContract, fns *[]string) string {
	nodes := []string{}

	// 行身份字段（列表输出列名）取 item_query 契约的主键
	rowKey := crudInputKey(getFn)
	// 目标参数名 → 行字段表达式（编译期规范化为 row.字段 映射）
	paramOf := func(c *model.FunctionContract) string {
		return fmt.Sprintf(`{"%s":"{{row.%s}}"}`, crudInputKey(c), rowKey)
	}

	// 主表格 + 行操作（详情/编辑/删除）
	rowActions := []string{
		fmt.Sprintf(`{"label":"查看详情","targetSection":"detail-modal","params":%s}`, paramOf(getFn)),
	}
	if updateFn != nil {
		rowActions = append(rowActions,
			fmt.Sprintf(`{"label":"编辑","targetSection":"edit-modal","params":%s}`, paramOf(updateFn)))
	}
	if deleteFn != nil {
		rowActions = append(rowActions,
			fmt.Sprintf(`{"label":"删除","targetSection":"delete-modal","params":%s,"danger":true}`, paramOf(deleteFn)))
	}
	tableNode := fmt.Sprintf(`{"id":"table","type":"fnTable","props":{"functionId":%q,"title":%q,"span":24,"autoRun":true,"columns":[],"rowActions":[%s]}}`,
		listFn.FunctionID, listFn.ResourceKey, strings.Join(rowActions, ","))
	nodes = append(nodes, tableNode)

	// 详情弹窗
	detailModal := fmt.Sprintf(`{"id":"detail-modal","type":"modal","props":{"title":%q,"width":"medium","sectionKey":"detail-modal"},"children":[{"id":"detail-fields","type":"fnFields","props":{"functionId":%q,"span":24,"autoRun":true}}]}`,
		"详情", getFn.FunctionID)
	nodes = append(nodes, detailModal)

	// 新建入口（根级按钮编译为表格工具栏）+ 新建弹窗（如有 create）
	if createFn != nil {
		*fns = append(*fns, createFn.FunctionID)
		nodes = append(nodes,
			`{"id":"create-btn","type":"button","props":{"title":"新建","onClick":{"kind":"openModal","target":"create-modal"}}}`,
			fmt.Sprintf(`{"id":"create-modal","type":"modal","props":{"title":%q,"width":"medium","sectionKey":"create-modal"},"children":[{"id":"create-form","type":"fnForm","props":{"functionId":%q,"span":24,"display":"inline","onSuccess":{"kind":"refreshNode","target":"table"}}}]}`,
				"新建", createFn.FunctionID))
	}

	// 编辑弹窗（如有 update）
	if updateFn != nil {
		*fns = append(*fns, updateFn.FunctionID)
		updateModal := fmt.Sprintf(`{"id":"edit-modal","type":"modal","props":{"title":%q,"width":"medium","sectionKey":"edit-modal"},"children":[{"id":"edit-form","type":"fnForm","props":{"functionId":%q,"span":24,"display":"inline","onSuccess":{"kind":"refreshNode","target":"table"}}}]}`,
			"编辑", updateFn.FunctionID)
		nodes = append(nodes, updateModal)
	}

	// 删除弹窗（行操作带 danger → 打开前先弹确认）（如有 delete）
	if deleteFn != nil {
		*fns = append(*fns, deleteFn.FunctionID)
		deleteModal := fmt.Sprintf(`{"id":"delete-modal","type":"modal","props":{"title":%q,"width":"medium","sectionKey":"delete-modal"},"children":[{"id":"delete-form","type":"fnForm","props":{"functionId":%q,"span":24,"display":"inline","onSuccess":{"kind":"refreshNode","target":"table"}}}]}`,
			"删除确认", deleteFn.FunctionID)
		nodes = append(nodes, deleteModal)
	}

	return "[" + strings.Join(nodes, ",") + "]"
}

// GenerateQueryTemplate 为带必填/可选查询参数的 collection_query 函数生成
// 「查询面板 + 结果表格」组合模板（query--<fid>）：表单填参 → refreshOn
// 联动表格自动带参重跑。
func (h *Handler) GenerateQueryTemplate(ctx context.Context, c *model.FunctionContract) error {
	tpl := buildQueryTemplate(c)
	if tpl == nil {
		return nil
	}
	return h.model.UpsertBuiltin(ctx, tpl)
}

// buildQueryTemplate 从契约推导「查询组合」模板；无查询参数返回 nil。
func buildQueryTemplate(c *model.FunctionContract) *model.ComponentTemplate {
	if c == nil || c.Capability != dbenum.CapabilityCollectionQuery {
		return nil
	}
	var schema struct {
		Properties map[string]json.RawMessage `json:"properties"`
	}
	if err := json.Unmarshal(c.InputSchema, &schema); err != nil || len(schema.Properties) == 0 {
		return nil // 无查询参数：单表格模板已覆盖
	}

	fid := c.FunctionID
	resource := c.ResourceKey
	if resource == "" {
		resource = lastSegment(fid)
	}
	// refreshOnNode 引用表单节点 id：instantiateTemplate 重映射节点 id 后，
	// 编译期解析为表单区块 key（键位漂移安全）。
	tree := fmt.Sprintf(`[
	  {"id":"qform","type":"fnForm","props":{"functionId":%q,"title":"查询条件","span":24,"display":"inline","autoRun":false,"refreshOnNode":["qform"]}},
	  {"id":"qtable","type":"fnTable","props":{"functionId":%q,"title":"查询结果","span":24,"autoRun":false,"columns":[],"rowActions":[],"refreshOnNode":["qform"]}}
	]`, fid, fid)

	return &model.ComponentTemplate{
		Key:               "query--" + sanitizeKey(fid),
		Name:              model.JSON(fmt.Sprintf(`{"zh-CN":%q}`, resource+"·查询")),
		Description:       model.JSON(`{"zh-CN":"查询条件 + 结果表格（条件变化自动刷新）"}`),
		Category:          "查询组合",
		Icon:              "SearchOutlined",
		RequiredFunctions: model.JSON(fmt.Sprintf(`[%q]`, fid)),
		Tree:              model.JSON(tree),
		Builtin:           true,
	}
}

// RegenerateFromContracts 扫描全部契约，生成单函数模板 + CRUD 组合模板。
func (h *Handler) RegenerateFromContracts(ctx context.Context, contracts []*model.FunctionContract) error {
	// 1. 单函数模板
	if err := generateSingleFunctionTemplates(h, ctx, contracts); err != nil {
		return fmt.Errorf("single function templates: %w", err)
	}

	// 2. 带查询参数的 collection_query → 查询组合模板
	for _, c := range contracts {
		if err := h.GenerateQueryTemplate(ctx, c); err != nil {
			slog.WarnContext(ctx, "query template generate", "functionId", c.FunctionID, "err", err)
		}
	}

	// 3. 按资源分组，生成 CRUD 模板
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
