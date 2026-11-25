package spec

import (
	"fmt"
	"strings"
)

// formFunctionTarget pairs one form carrier with the function it executes,
// following the structural carrier→binding references of the page model
// (SUI-033).
type formFunctionTarget struct {
	field      string
	form       *FormPresentationSpec
	functionID string
}

// formFunctionTargets collects every form carrier that has a structural
// function mapping:
//
//	resource.actions[i].form  → actions[i].bindingId
//	operation.form            → the single usage=action binding
//	task.form                 → the single usage=task binding
//	report.queryForm          → the single usage=report binding
//	composite.sections[i].form→ sections[i].bindingId (static sections skipped)
//
// Carriers without a structural mapping — resource createForm/updateForm,
// whose submission wiring is generator-internal — are skipped so the check
// never produces false positives; their drift surfaces through the form's own
// jsonSchema when the draft regenerates from the contract.
func formFunctionTargets(page PageSpec) []formFunctionTarget {
	bindings := make(map[string]PageFunctionBinding, len(page.Bindings))
	for _, binding := range page.Bindings {
		if id := strings.TrimSpace(binding.ID); id != "" {
			bindings[id] = binding
		}
	}
	// usageTarget returns the binding of a usage when the page declares
	// exactly one; ambiguous or missing usages are skipped (no false positives).
	usageTarget := func(usage PageBindingUsage) (PageFunctionBinding, bool) {
		var found []PageFunctionBinding
		for _, binding := range page.Bindings {
			if binding.Usage == usage {
				found = append(found, binding)
			}
		}
		if len(found) != 1 {
			return PageFunctionBinding{}, false
		}
		return found[0], true
	}
	var targets []formFunctionTarget
	add := func(field string, form *FormPresentationSpec, binding PageFunctionBinding, ok bool) {
		if !ok || form == nil {
			return
		}
		functionID := strings.TrimSpace(binding.FunctionID)
		if functionID == "" {
			return
		}
		targets = append(targets, formFunctionTarget{field: field, form: form, functionID: functionID})
	}
	if page.Resource != nil {
		for i := range page.Resource.Actions {
			action := &page.Resource.Actions[i]
			binding, ok := bindings[strings.TrimSpace(action.BindingID)]
			add(fmt.Sprintf("resource.actions[%d].form", i), action.Form, binding, ok)
		}
	}
	if page.Operation != nil {
		binding, ok := usageTarget(BindingUsageAction)
		add("operation.form", page.Operation.Form, binding, ok)
	}
	if page.Task != nil {
		binding, ok := usageTarget(BindingUsageTask)
		add("task.form", page.Task.Form, binding, ok)
	}
	if page.Report != nil {
		binding, ok := usageTarget(BindingUsageReport)
		add("report.queryForm", page.Report.QueryForm, binding, ok)
	}
	if page.Composite != nil {
		for i := range page.Composite.Sections {
			section := &page.Composite.Sections[i]
			if section.Static {
				continue
			}
			binding, ok := bindings[strings.TrimSpace(section.BindingID)]
			add(fmt.Sprintf("composite.sections[%d].form", i), section.Form, binding, ok)
		}
	}
	return targets
}

// ValidateVariantContractFreshness re-resolves every named display variant's
// field keys against the CURRENT input schema of the function its carrier
// form executes (SUI-033). The form's own jsonSchema may be stale — a draft
// that has not been regenerated yet, or a rollback target whose schema is
// frozen in the publish snapshot — so this check is what keeps a publish or a
// versioning-rollback preflight from resurrecting variants that reference
// contract fields deleted or renamed in the meantime. Each failure is
// reported as form_variant_contract_stale, located at the variant field key
// and naming the affected variant key plus the function.
//
// The check runs on publish paths only: save/preview may legitimately lag the
// current contract (stale-draft model — regeneration re-derives the form
// schema and surfaces the same drift per variant through
// form_variant_field_unknown).
func ValidateVariantContractFreshness(page PageSpec, functions map[string]FunctionSpec) []Diagnostic {
	var diags []Diagnostic
	for _, target := range formFunctionTargets(page) {
		fn, ok := functions[target.functionID]
		if !ok || len(fn.InputSchema) == 0 {
			// Missing functions are reported by binding validation; without a
			// current input schema there is nothing to compare against.
			continue
		}
		root, resolvable := formSchemaRoot(fn.InputSchema)
		if !resolvable {
			continue
		}
		for i := range target.form.Variants {
			variant := &target.form.Variants[i]
			variantKey := strings.TrimSpace(variant.Key)
			base := fmt.Sprintf("%s.variants[%d]", target.field, i)
			for j := range variant.Fields {
				item := &variant.Fields[j]
				if strings.TrimSpace(item.Key) == "" {
					continue // shape validation reports the missing key
				}
				if _, reason := resolveFormField(root, item.Key); reason != "" {
					d := publishShapeDiagnostic(
						"form_variant_contract_stale",
						fmt.Sprintf("variant %q field %q no longer resolves in the current input schema of function %s: %s",
							variantKey, item.Key, target.functionID, reason),
						fmt.Sprintf("%s.fields[%d].key", base, j),
					)
					d.FunctionID = target.functionID
					diags = append(diags, d)
				}
			}
			for gi, group := range variant.Groups {
				for fj, fieldKey := range group.Fields {
					if strings.TrimSpace(fieldKey) == "" {
						continue
					}
					if _, reason := resolveFormField(root, fieldKey); reason != "" {
						d := publishShapeDiagnostic(
							"form_variant_contract_stale",
							fmt.Sprintf("variant %q group field %q no longer resolves in the current input schema of function %s: %s",
								variantKey, fieldKey, target.functionID, reason),
							fmt.Sprintf("%s.groups[%d].fields[%d].key", base, gi, fj),
						)
						d.FunctionID = target.functionID
						diags = append(diags, d)
					}
				}
			}
		}
	}
	return diags
}
