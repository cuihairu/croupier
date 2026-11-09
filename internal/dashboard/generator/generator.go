// Package generator creates conservative default PageSpec suggestions from
// normalized function capability contracts. It does not read registration-side
// UI, menu, mapping, pagination, column, task view, or chart view extensions.
package generator

import (
	"strings"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
)

// GenerateOptions controls page generation behavior.
type GenerateOptions struct {
	DefaultLocale   string
	Functions       map[string]spec.FunctionSpec
	TaskSemantics   map[string]spec.TaskSemantic
	ReportSemantics map[string]spec.ReportSemantic
	// Terms maps "domain/alias" to localized display text sourced from the
	// platform term dictionary. The generator uses it for title fallbacks
	// (category display names are owned by the menu system since T-M8);
	// it never overrides explicit summaries from registration.
	Terms TermDictionary
}

// TermDictionary maps term aliases to localized display labels, keyed by
// "domain/alias" (both lowercased). Domain is "resource" or "operation".
type TermDictionary map[string]spec.LocalizedText

// Lookup resolves a term by domain and alias.
func (t TermDictionary) Lookup(domain, alias string) (spec.LocalizedText, bool) {
	if t == nil {
		return nil, false
	}
	domain = strings.ToLower(strings.TrimSpace(domain))
	alias = strings.ToLower(strings.TrimSpace(alias))
	if domain == "" || alias == "" {
		return nil, false
	}
	text, ok := t[domain+"/"+alias]
	return text, ok
}

// DefaultGenerateOptions returns default options.
func DefaultGenerateOptions() GenerateOptions {
	return GenerateOptions{DefaultLocale: "zh-CN"}
}

// GenerateForOperation creates an operation, task, or report candidate from a
// single executable capability.
func GenerateForOperation(op spec.OperationSpec, opts GenerateOptions) spec.GeneratedPageSpec {
	opts = normalizeOptions(opts)
	switch {
	case isReportOperation(op):
		return GenerateReportPageForOperation(op, opts)
	case isTaskOperation(op):
		return GenerateTaskPageForOperation(op, opts)
	default:
		return GenerateOperationPageForOperation(op, opts)
	}
}
