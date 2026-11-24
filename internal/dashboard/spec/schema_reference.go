package spec

// DiagSchemaReferenceUnresolved marks a schema $ref the extraction could not
// fully inline into the standalone request contract (SUI-008): a missing
// target, a cyclic reference past the inlining depth cap, or a reference
// outside the document. Field is the $ref pointer; the message names the
// boundary that stopped the inlining.
const DiagSchemaReferenceUnresolved = "openapi_schema_reference_unresolved"

// SchemaReferenceUnresolvedDiagnostic builds the proposal-level diagnostic
// recorded when a schema reference cannot be inlined into the standalone
// request contract (SUI-008). Severity is warning: the schema is still usable
// (partial inlining), but the unresolved reference is never silently dropped —
// the schema explicitly enters review instead of degrading to an anonymous
// object the operator cannot distinguish from a real one.
func SchemaReferenceUnresolvedDiagnostic(ref, reason string) Diagnostic {
	return Diagnostic{
		Code:     DiagSchemaReferenceUnresolved,
		Severity: SeverityWarning,
		Field:    ref,
		Message:  "schema reference " + ref + " could not be inlined (" + reason + "); the standalone schema enters pending review",
	}
}
