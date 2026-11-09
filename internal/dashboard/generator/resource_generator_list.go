package generator

import (
	"encoding/json"
	"sort"
	"strings"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/model"
)

func defaultListView() *spec.ListViewSpec {
	return &spec.ListViewSpec{
		Columns: []spec.ColumnSpec{},
	}
}

func buildFiltersFromContract(contract *model.FunctionContract, semantics *model.CapabilitySemantics) []spec.FilterSpec {
	if contract == nil || len(contract.InputSchema) == 0 {
		return nil
	}
	properties := objectProperty(parseJSONObject(json.RawMessage(contract.InputSchema)), "properties")
	if len(properties) == 0 {
		return nil
	}
	pageField := strings.TrimSpace(semantics.PageFieldName)
	pageSizeField := strings.TrimSpace(semantics.PageSizeFieldName)
	keys := make([]string, 0, len(properties))
	for key := range properties {
		if key == pageField || key == pageSizeField {
			continue
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)
	filters := make([]spec.FilterSpec, 0, len(keys))
	for _, key := range keys {
		prop := properties[key]
		humanTitle := fallbackLabel(key)
		if humanTitle == "" {
			humanTitle = key
		}
		filters = append(filters, spec.FilterSpec{
			Key:     key,
			Title:   spec.LocalizedText{"zh-CN": humanTitle, "en-US": humanTitle},
			Type:    inferFilterType(prop),
			Options: enumOptionsFromSchema(prop),
		})
	}
	return filters
}

func paginationFromContract(contract *model.FunctionContract, semantics *model.CapabilitySemantics) *spec.PaginationSpec {
	if contract == nil || len(contract.InputSchema) == 0 || semantics == nil {
		return nil
	}
	properties := objectProperty(parseJSONObject(json.RawMessage(contract.InputSchema)), "properties")
	if len(properties) == 0 {
		return nil
	}
	// page 命中即启用分页；page_size 仅在语义声明且 schema 收录时参与
	// selector 映射。此前两者缺一即返回 nil——契约只有 page 无 page_size
	// 时分页整体关闭，listQuerySchema 不注入 current，而 selector 已把
	// page 映射到 /current，发布校验永远失败（页面不可发布）。
	pageField := strings.TrimSpace(semantics.PageFieldName)
	if pageField == "" {
		return nil
	}
	if _, ok := properties[pageField]; !ok {
		return nil
	}
	return &spec.PaginationSpec{
		Enabled:     true,
		DefaultSize: 20,
		PageSizes:   []int{10, 20, 50, 100},
	}
}

// commonPageFieldVariants 是分页字段的常见命名变体。SDK 侧 payload key
// 命名随游戏方风格（snake/camel），语义聚合的默认 page/page_size 若在
// 合同 schema 中不存在，按变体表回退探测并回写 semantics，使分页 selector
// 与 UI 不因命名差异静默失效。
var (
	commonPageFieldVariants     = []string{"page", "pageNo", "pageNum", "current", "offset"}
	commonPageSizeFieldVariants = []string{"page_size", "pageSize", "perPage", "limit"}
)

// resolvePaginationFields 校正 semantics 的分页字段命名到合同 schema 实际
// 存在的字段（原地回写），返回是否两者都可用。
func resolvePaginationFields(properties map[string]json.RawMessage, semantics *model.CapabilitySemantics) bool {
	if semantics == nil {
		return false
	}
	pageField := strings.TrimSpace(semantics.PageFieldName)
	if _, ok := properties[pageField]; !ok {
		for _, candidate := range commonPageFieldVariants {
			if _, ok := properties[candidate]; ok {
				pageField = candidate
				break
			}
		}
	}
	pageSizeField := strings.TrimSpace(semantics.PageSizeFieldName)
	if _, ok := properties[pageSizeField]; !ok {
		for _, candidate := range commonPageSizeFieldVariants {
			if _, ok := properties[candidate]; ok {
				pageSizeField = candidate
				break
			}
		}
	}
	if pageField == "" || pageSizeField == "" {
		return false
	}
	if _, ok := properties[pageField]; !ok {
		return false
	}
	if _, ok := properties[pageSizeField]; !ok {
		return false
	}
	semantics.PageFieldName = pageField
	semantics.PageSizeFieldName = pageSizeField
	return true
}

func findCollectionItemsSchema(raw spec.JSONSchema, semantics *model.CapabilitySemantics) json.RawMessage {
	root := parseJSONObject(json.RawMessage(raw))
	if len(root) == 0 {
		return nil
	}
	if items := objectProperty(root, "items"); len(items) > 0 {
		return rawObject(items)
	}

	properties := objectProperty(root, "properties")
	if len(properties) == 0 {
		return nil
	}
	for _, key := range collectionArrayKeys(semantics) {
		prop := objectProperty(properties, key)
		if len(prop) == 0 {
			continue
		}
		if items := objectProperty(prop, "items"); len(items) > 0 {
			return rawObject(items)
		}
	}
	return nil
}

func collectionArrayKeys(semantics *model.CapabilitySemantics) []string {
	keys := []string{}
	if semantics != nil {
		if key := strings.TrimSpace(semantics.ItemsFieldName); key != "" {
			keys = append(keys, key)
		}
	}
	return append(keys, "items", "list", "rows", "data")
}
