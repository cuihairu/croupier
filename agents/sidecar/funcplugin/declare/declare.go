// Package declare validates function adapter plugin declarations and
// builds the shared function metadata the sidecar registration chain
// consumes (todo.md:35). It is pure Go — no gRPC, no go-plugin — so it
// compiles in both default and no_go_plugin builds: trimmed binaries
// still fail closed on invalid declarations before any host exists.
//
// The namespace rule is the permission half of the contract: a plugin
// only ever declares ids under "plugin.<plugin-name>.", so it cannot
// shadow SDK provider or platform functions, and duplicate declarations
// are rejected instead of silently upserted.
package declare

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	functionv1 "github.com/cuihairu/croupier/pkg/pb/croupier/function/v1"
)

// maxSchemaBytes caps one declared JSON Schema; declarations are bounded
// config-shaped metadata, not documents.
const maxSchemaBytes = 64 << 10

// Descriptor is one plugin-declared function. It mirrors the wire
// FuncDescriptor without dragging the gRPC protocol package into
// trimmed builds.
type Descriptor struct {
	ID           string
	Summary      string
	ParamSchema  string
	ResultSchema string
	ErrorSchema  string
	RiskLevel    string
}

// Risk levels (closed set) mapped onto FunctionSecurity.RiskLevel.
const (
	RiskLow    = "low"
	RiskMedium = "medium"
	RiskHigh   = "high"
	RiskDanger = "danger"
)

var validRisk = map[string]bool{"": true, RiskLow: true, RiskMedium: true, RiskHigh: true, RiskDanger: true}

// validFunctionName matches the lowercase dotted id shape the server
// register boundary normalizes to (`functionIDPattern`); declarations
// outside it would register under a rewritten key.
var validFunctionName = regexp.MustCompile(`^[a-z0-9]+(?:[._-][a-z0-9]+)*$`).MatchString

// Namespace returns the only id prefix plugin functions may live under.
func Namespace(pluginName string) string { return "plugin." + pluginName + "." }

// ParseFunctionOwner returns the plugin name owning a namespaced
// function id, or "" when the id is not a plugin function.
func ParseFunctionOwner(functionID string) string {
	rest, ok := strings.CutPrefix(functionID, "plugin.")
	if !ok {
		return ""
	}
	name, _, found := strings.Cut(rest, ".")
	if !found || name == "" {
		return ""
	}
	return name
}

// ValidatePluginName enforces the plugin identity charset so the
// namespace prefix stays unambiguous.
func ValidatePluginName(name string) error {
	if len(name) < 2 || len(name) > 64 {
		return fmt.Errorf("plugin name %q must be 2..64 chars", name)
	}
	for i, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
		case r == '-' && i > 0:
		default:
			return fmt.Errorf("plugin name %q must match [a-z0-9][a-z0-9-]*", name)
		}
	}
	return nil
}

// Validate checks one declared function against the contract: namespace,
// schema shape and the closed risk set. Every rejection is an explicit
// error — the host never registers a partially valid declaration.
func Validate(pluginName string, d Descriptor) error {
	if err := ValidatePluginName(pluginName); err != nil {
		return err
	}
	prefix := Namespace(pluginName)
	fn, ok := strings.CutPrefix(d.ID, prefix)
	if !ok || fn == "" {
		return fmt.Errorf("function id %q must live under the plugin namespace %s*", d.ID, prefix)
	}
	if strings.ContainsAny(fn, " \t\r\n") || strings.Contains(fn, "..") {
		return fmt.Errorf("function id %q has an invalid function name part %q", d.ID, fn)
	}
	// The server lowercases every function id at the registration
	// boundary; a declared id that is not already lowercase-dotted would
	// register under a rewritten key while the agent dispatch seam still
	// matches the declared one — reject that mismatch at declaration time
	// so the id is stable end to end (lowercase letters, digits and
	// ./_/- separators, no empty segments).
	if !validFunctionName(fn) {
		return fmt.Errorf("function id %q must use a lowercase [a-z0-9] name part with . _ - separators (server normalizes ids to lowercase; mixed case would silently rewrite the registered id)", d.ID)
	}
	if err := validateObjectSchema(d.ParamSchema, "param_schema"); err != nil {
		return err
	}
	if err := validateObjectSchema(d.ResultSchema, "result_schema"); err != nil {
		return err
	}
	if d.ErrorSchema != "" {
		if err := validateSchema(d.ErrorSchema, "error_schema"); err != nil {
			return err
		}
	}
	if !validRisk[d.RiskLevel] {
		return fmt.Errorf("risk_level %q is outside the closed set low|medium|high|danger", d.RiskLevel)
	}
	return nil
}

// validateObjectSchema requires a bounded, valid JSON Schema whose
// top-level type is object: params and results are JSON objects by
// contract, so a declaration that cannot promise that fails at
// registration instead of at call time.
func validateObjectSchema(schema, field string) error {
	if err := validateSchema(schema, field); err != nil {
		return err
	}
	var head struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal([]byte(schema), &head); err != nil {
		return fmt.Errorf("%s: %w", field, err)
	}
	if head.Type != "object" {
		return fmt.Errorf("%s: top-level type must be \"object\", got %q", field, head.Type)
	}
	return nil
}

func validateSchema(schema, field string) error {
	if schema == "" {
		return fmt.Errorf("%s is required", field)
	}
	if len(schema) > maxSchemaBytes {
		return fmt.Errorf("%s exceeds %d bytes", field, maxSchemaBytes)
	}
	if !json.Valid([]byte(schema)) {
		return fmt.Errorf("%s is not valid JSON", field)
	}
	return nil
}

// Metadata builds the functionv1.FunctionMetadata for a validated
// descriptor so plugin functions enter the SAME registration, approval
// and audit chain as SDK functions (todo.md:35: 复用函数注册、权限审批、
// 审计及路由链). Plugin functions are always audit-logged; the risk
// level is the plugin's declared one, and approval/permission remain
// server-side policy — the plugin adds no bypass.
func Metadata(pluginName, pluginVersion string, d Descriptor) (*functionv1.FunctionMetadata, error) {
	if err := Validate(pluginName, d); err != nil {
		return nil, err
	}
	risk := d.RiskLevel
	if risk == "" {
		risk = RiskLow
	}
	md := &functionv1.FunctionMetadata{
		Id:           d.ID,
		Version:      pluginVersion,
		Resource:     "plugin." + pluginName,
		Tags:         []string{"plugin", pluginName},
		Name:         d.ID,
		Description:  d.Summary,
		InputSchema:  d.ParamSchema,
		OutputSchema: d.ResultSchema,
		Execution:    "sync",
		Behavior: &functionv1.FunctionBehavior{
			// v1 maps read-only adapters (the reference plugin is a GET):
			// low risk reads as query/idempotent, anything riskier reads
			// as a non-idempotent command until the descriptor grows an
			// explicit behavior field — conservative by default.
			Mode:       behaviorMode(risk),
			Idempotent: risk == RiskLow,
		},
		Security: &functionv1.FunctionSecurity{
			RiskLevel: riskFunctionv1(risk),
			AuditLog:  true,
		},
	}
	if md.Version == "" {
		md.Version = "0.0.0"
	}
	return md, nil
}

func riskFunctionv1(risk string) functionv1.FunctionSecurity_RiskLevel {
	switch risk {
	case RiskMedium:
		return functionv1.FunctionSecurity_RISK_LEVEL_MEDIUM
	case RiskHigh:
		return functionv1.FunctionSecurity_RISK_LEVEL_HIGH
	case RiskDanger:
		return functionv1.FunctionSecurity_RISK_LEVEL_DANGER
	default:
		return functionv1.FunctionSecurity_RISK_LEVEL_LOW
	}
}

func behaviorMode(risk string) functionv1.FunctionBehavior_Mode {
	if risk == RiskLow {
		return functionv1.FunctionBehavior_MODE_QUERY
	}
	return functionv1.FunctionBehavior_MODE_COMMAND
}
