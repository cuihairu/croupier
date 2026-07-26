package uicontract

import "strings"

var forbiddenRegistrationKeys = map[string]struct{}{
	"categorydisplay":     {},
	"category_display":    {},
	"displayname":         {},
	"display_name":        {},
	"entitydisplay":       {},
	"entity_display":      {},
	"formily":             {},
	"layout":              {},
	"menu":                {},
	"operationdisplay":    {},
	"operation_display":   {},
	"operationkind":       {},
	"operation_kind":      {},
	"pagehint":            {},
	"page_hint":           {},
	"pageschema":          {},
	"page_schema":         {},
	"placement":           {},
	"route":               {},
	"routes":              {},
	"tablecolumns":        {},
	"table_columns":       {},
	"ui":                  {},
	"x-category-display":  {},
	"x_category_display":  {},
	"x-components":        {},
	"x_components":        {},
	"x-display-name":      {},
	"x_display_name":      {},
	"x-entity-display":    {},
	"x_entity_display":    {},
	"x-formily":           {},
	"x_formily":           {},
	"x-layout":            {},
	"x_layout":            {},
	"x-menu":              {},
	"x-operation-display": {},
	"x_operation_display": {},
	"x-operation-kind":    {},
	"x_operation_kind":    {},
	"x-page":              {},
	"x-page-hint":         {},
	"x_page_hint":         {},
	"x-page-schema":       {},
	"x_page_schema":       {},
	"x-placement":         {},
	"x_placement":         {},
	"x-route":             {},
	"x-routes":            {},
	"x-table-columns":     {},
	"x_table_columns":     {},
	"x-ui":                {},
	"x_ui":                {},
}

// ForbiddenRegistrationKey reports whether key tries to attach UI concerns to
// a function capability contract.
func ForbiddenRegistrationKey(key string) (string, bool) {
	normalized := strings.TrimSpace(strings.ToLower(key))
	if normalized == "" {
		return "", false
	}
	normalized = strings.ReplaceAll(normalized, " ", "")
	if _, ok := forbiddenRegistrationKeys[normalized]; ok {
		return normalized, true
	}
	return normalized, false
}
