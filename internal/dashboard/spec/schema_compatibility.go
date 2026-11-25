package spec

// Request compatibility statuses (SUI-011) classify how far the platform can
// carry one OpenAPI operation's request contract into a generated page:
//
//	direct   – every declared input can be rendered and sent as-is.
//	degraded – a structure is understood but cannot be rendered as a
//	           structured form; the operator can still edit the JSON payload
//	           and the request is sent.
//	manual   – the request cannot be sent through the executor without a human
//	           changing the source first (unsupported media type, unresolved
//	           reference, or a variant branch that has to be chosen).
const (
	RequestCompatibilityDirect   = "direct"
	RequestCompatibilityDegraded = "degraded"
	RequestCompatibilityManual   = "manual"
)

// DiagSchemaCompatibilityDegraded marks a request structure the form renderer
// cannot show as fields but the executor can still send (SUI-011).
const DiagSchemaCompatibilityDegraded = "openapi_schema_compatibility_degraded"

// DiagSchemaCompatibilityManual marks a request the executor cannot send as
// declared (SUI-011); the operation must not get a directly executable
// default proposal.
const DiagSchemaCompatibilityManual = "openapi_schema_compatibility_manual"

// SchemaCompatibilityDiagnostic builds the contract-level diagnostic for one
// incompatible request construct. Field is the JSON pointer of the construct
// inside the request contract so an operator can find it in the source;
// detail names the construct and what to change. Severity stays warning: the
// contract is stored and the operation stays visible, the status is carried on
// the diagnostic code and drives the proposal quality gate instead.
func SchemaCompatibilityDiagnostic(status, field, detail string) Diagnostic {
	return Diagnostic{
		Code:     compatibilityCode(status),
		Severity: SeverityWarning,
		Field:    field,
		Message:  detail,
	}
}

func compatibilityCode(status string) string {
	if status == RequestCompatibilityManual {
		return DiagSchemaCompatibilityManual
	}
	return DiagSchemaCompatibilityDegraded
}
