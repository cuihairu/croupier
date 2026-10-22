package declare

import (
	"strings"
	"testing"
)

// validDescriptor is a well-formed declaration under the httpget
// namespace, used as the base of the validation matrix.
func validDescriptor() Descriptor {
	return Descriptor{
		ID:           "plugin.httpget.health",
		Summary:      "readiness of the game http interface",
		ParamSchema:  `{"type":"object"}`,
		ResultSchema: `{"type":"object"}`,
		RiskLevel:    RiskLow,
	}
}

// TestValidateAcceptsWellFormedDeclaration pins the happy path: a
// namespaced, object-schema declaration validates and yields metadata
// with the shared-chain fields filled (resource, tags, sync, audit).
func TestValidateAcceptsWellFormedDeclaration(t *testing.T) {
	if err := Validate("httpget", validDescriptor()); err != nil {
		t.Fatalf("valid declaration rejected: %v", err)
	}
	md, err := Metadata("httpget", "1.0.0", validDescriptor())
	if err != nil {
		t.Fatalf("metadata: %v", err)
	}
	if md.Id != "plugin.httpget.health" || md.Resource != "plugin.httpget" || md.Execution != "sync" {
		t.Fatalf("metadata identity: %+v", md)
	}
	if md.Security == nil || !md.Security.AuditLog || md.Security.RiskLevel != 1 /* LOW */ {
		t.Fatalf("metadata security: %+v", md.Security)
	}
	if md.Behavior == nil || md.Behavior.Mode != 1 /* QUERY */ || !md.Behavior.Idempotent {
		t.Fatalf("metadata behavior: %+v", md.Behavior)
	}
}

// TestValidateRejectsForeignNamespace pins the namespace boundary: a
// plugin may only declare ids under plugin.<name>., so it cannot shadow
// SDK provider or platform functions.
func TestValidateRejectsForeignNamespace(t *testing.T) {
	for _, id := range []string{
		"player.ban",          // SDK-shaped id
		"plugin.other.health", // different plugin's namespace
		"plugin.httpget",      // missing function part
		"plugin..health",      // empty function part
	} {
		d := validDescriptor()
		d.ID = id
		if err := Validate("httpget", d); err == nil {
			t.Fatalf("foreign namespace id %q must be rejected", id)
		}
	}
}

// TestValidateRejectsInvalidSchemas pins schema shape: schemas must be
// valid JSON with a top-level object type, bounded in size.
func TestValidateRejectsInvalidSchemas(t *testing.T) {
	cases := map[string]func(*Descriptor){
		"param schema not json":     func(d *Descriptor) { d.ParamSchema = `{"type":` },
		"param schema not object":   func(d *Descriptor) { d.ParamSchema = `{"type":"string"}` },
		"param schema missing type": func(d *Descriptor) { d.ParamSchema = `{}` },
		"result schema not json":    func(d *Descriptor) { d.ResultSchema = `{"type":` },
		"result schema not object":  func(d *Descriptor) { d.ResultSchema = `{"type":"array"}` },
		"error schema not json":     func(d *Descriptor) { d.ErrorSchema = `nope` },
		"param schema empty":        func(d *Descriptor) { d.ParamSchema = `` },
		"param schema oversized":    func(d *Descriptor) { d.ParamSchema = `{"type":"object","x":"` + strings.Repeat("a", 64<<10) + `"}` },
	}
	for name, mutate := range cases {
		d := validDescriptor()
		mutate(&d)
		if err := Validate("httpget", d); err == nil {
			t.Fatalf("%s must be rejected", name)
		}
	}
}

// TestValidateRejectsBadPluginNameAndRisk pins the plugin identity and
// closed risk set.
func TestValidateRejectsBadPluginNameAndRisk(t *testing.T) {
	if err := ValidatePluginName(""); err == nil {
		t.Fatal("blank plugin name must be rejected")
	}
	if err := ValidatePluginName("HTTP"); err == nil {
		t.Fatal("uppercase plugin name must be rejected")
	}
	d := validDescriptor()
	d.RiskLevel = "secret"
	if err := Validate("httpget", d); err == nil {
		t.Fatal("risk outside the closed set must be rejected")
	}
	d.RiskLevel = RiskDanger
	if err := Validate("httpget", d); err != nil {
		t.Fatalf("danger risk must be accepted: %v", err)
	}
}

// TestValidateRejectsNonLowercaseFunctionName pins the id normalization
// boundary: the server register boundary lowercases ids, so a mixed-case
// declaration would register under a rewritten key while the agent
// dispatch seam matches the declared one. Declarations must be lowercase
// dotted ids, rejected here at declaration time.
func TestValidateRejectsNonLowercaseFunctionName(t *testing.T) {
	for _, id := range []string{
		"plugin.httpget.banPlayer", // mixed case rewrites server-side
		"plugin.httpget.Health",
		"plugin.httpget.-lead",
		"plugin.httpget.double..part",
	} {
		d := validDescriptor()
		d.ID = id
		if err := Validate("httpget", d); err == nil {
			t.Fatalf("non-lowercase or malformed function id %q must be rejected", id)
		}
	}
	// Separators the server id pattern accepts stay valid.
	for _, id := range []string{
		"plugin.httpget.ban-player",
		"plugin.httpget.ban_player",
		"plugin.httpget.v2.fetch",
	} {
		d := validDescriptor()
		d.ID = id
		if err := Validate("httpget", d); err != nil {
			t.Fatalf("lowercase dotted id %q must be accepted: %v", id, err)
		}
	}
}
