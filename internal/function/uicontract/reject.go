package uicontract

import "strings"

var forbiddenRegistrationKeys = map[string]struct{}{
	"ui":           {},
	"x-ui":         {},
	"x_ui":         {},
	"x-formily":    {},
	"x_formily":    {},
	"formily":      {},
	"layout":       {},
	"x-layout":     {},
	"x_layout":     {},
	"components":   {},
	"x-components": {},
	"x_components": {},
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
