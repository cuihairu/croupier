package function

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
)

// ---------------------------------------------------------------------------
// SimpleRegister
// ---------------------------------------------------------------------------

func TestSimpleRegister_Success(t *testing.T) {
	client := newMockClient()
	handler := func(ctx context.Context, input []byte) ([]byte, error) { return input, nil }

	if err := SimpleRegister(client, "player.kick", "player", "Kick Player", handler); err != nil {
		t.Fatalf("SimpleRegister: %v", err)
	}

	desc, ok := client.registeredFunctions["player.kick"]
	if !ok {
		t.Fatal("function should be registered on the client")
	}
	if desc.ID != "player.kick" || desc.Resource != "player" {
		t.Fatalf("unexpected descriptor %+v", desc)
	}
	if desc.Risk != "low" {
		t.Fatalf("SimpleRegister should default to low risk, got %q", desc.Risk)
	}
}

func TestSimpleRegister_EmptyIDFails(t *testing.T) {
	client := newMockClient()
	handler := func(ctx context.Context, input []byte) ([]byte, error) { return input, nil }
	if err := SimpleRegister(client, "", "player", "X", handler); err == nil {
		t.Fatal("empty ID must be rejected")
	}
}

func TestSimpleRegister_ClientError(t *testing.T) {
	client := &recordingClient{fail: true}
	handler := func(ctx context.Context, input []byte) ([]byte, error) { return input, nil }
	if err := SimpleRegister(client, "f", "r", "N", handler); err == nil {
		t.Fatal("client failure must propagate")
	}
}

// ---------------------------------------------------------------------------
// schemaTypeToString
// ---------------------------------------------------------------------------

func TestSchemaTypeToString_NilSchema(t *testing.T) {
	if got := schemaTypeToString(nil); got != "object" {
		t.Fatalf("nil schema should map to object, got %q", got)
	}
}

func TestSchemaTypeToString_NoType(t *testing.T) {
	s := &openapi3.Schema{}
	if got := schemaTypeToString(s); got != "object" {
		t.Fatalf("missing type should map to object, got %q", got)
	}
}

func TestSchemaTypeToString_ScalarTypes(t *testing.T) {
	for _, tc := range []struct{ typ, want string }{
		{"string", "string"},
		{"integer", "integer"},
		{"number", "number"},
		{"boolean", "boolean"},
		{"array", "array"},
	} {
		s := &openapi3.Schema{Type: &openapi3.Types{tc.typ}}
		if got := schemaTypeToString(s); got != tc.want {
			t.Fatalf("type %q: got %q want %q", tc.typ, got, tc.want)
		}
	}
}

// ---------------------------------------------------------------------------
// extractExtension value types
// ---------------------------------------------------------------------------

func TestExtractExtension_ValueTypes(t *testing.T) {
	exts := map[string]interface{}{
		"s": "str",
		"n": json.Number("7"),
		"b": true,
		"f": 1.5,
		"o": []string{"x"},
	}
	if got := extractExtension(exts, "s"); got != "str" {
		t.Fatalf("string ext = %q", got)
	}
	if got := extractExtension(exts, "n"); got != "7" {
		t.Fatalf("json.Number ext = %q", got)
	}
	if got := extractExtension(exts, "b"); got != "true" {
		t.Fatalf("bool ext = %q", got)
	}
	if got := extractExtension(exts, "f"); got != "1.5" {
		t.Fatalf("float ext = %q", got)
	}
	if got := extractExtension(exts, "o"); got != "[x]" {
		t.Fatalf("default ext = %q", got)
	}
}

func TestExtractExtension_MissingAndNil(t *testing.T) {
	if got := extractExtension(nil, "x"); got != "" {
		t.Fatalf("nil map should return empty, got %q", got)
	}
	if got := extractExtension(map[string]interface{}{}, "x"); got != "" {
		t.Fatalf("missing key should return empty, got %q", got)
	}
}

// ---------------------------------------------------------------------------
// deriveOperationID / deriveOperationName / toTitleCase branches
// ---------------------------------------------------------------------------

func TestDeriveOperationID_EmptyPathFallback(t *testing.T) {
	op := &openapi3.Operation{}
	if got := deriveOperationID(op, ""); got != "unknown.function" {
		t.Fatalf("empty path fallback = %q", got)
	}
}

func TestDeriveOperationID_FromPathSegments(t *testing.T) {
	op := &openapi3.Operation{}
	if got := deriveOperationID(op, "/api/players/{id}"); got != "api.players.{id}" {
		t.Fatalf("path-derived ID = %q", got)
	}
	if got := deriveOperationID(op, "/single"); got != "single" {
		t.Fatalf("single segment ID = %q", got)
	}
}

func TestDeriveOperationName_Branches(t *testing.T) {
	if got := deriveOperationName(nil); got != "Unnamed Function" {
		t.Fatalf("nil op name = %q", got)
	}
	if got := deriveOperationName(&openapi3.Operation{}); got != "Unnamed Function" {
		t.Fatalf("empty op name = %q", got)
	}
	if got := deriveOperationName(&openapi3.Operation{OperationID: "player_ban"}); got != "Player Ban" {
		t.Fatalf("title case name = %q", got)
	}
	if got := deriveOperationName(&openapi3.Operation{Summary: "S"}); got != "S" {
		t.Fatalf("summary name = %q", got)
	}
}

func TestToTitleCase_EdgeCases(t *testing.T) {
	if got := toTitleCase(""); got != "" {
		t.Fatalf("empty = %q", got)
	}
	if got := toTitleCase("a_b_c"); got != "A B C" {
		t.Fatalf("multi word = %q", got)
	}
	if got := toTitleCase("_lead"); got != " Lead" {
		t.Fatalf("leading underscore = %q", got)
	}
}

// ---------------------------------------------------------------------------
// parseRiskLevel full mapping
// ---------------------------------------------------------------------------

func TestParseRiskLevel_AllLevels(t *testing.T) {
	cases := map[string]RiskLevel{
		"low": RiskLow, "safe": RiskLow,
		"medium": RiskMedium, "moderate": RiskMedium, "MEDIUM": RiskMedium,
		"high":   RiskHigh,
		"danger": RiskDanger, "critical": RiskDanger,
		"bogus": RiskMedium, "": RiskMedium,
	}
	for in, want := range cases {
		if got := parseRiskLevel(in); got != want {
			t.Fatalf("parseRiskLevel(%q) = %v, want %v", in, got, want)
		}
	}
}

// ---------------------------------------------------------------------------
// openAPIToMetadata nil path item skip
// ---------------------------------------------------------------------------

func TestOpenAPIToMetadata_NilPathItemSkipped(t *testing.T) {
	r := NewRegistryWithLogger(newMockClient(), &NoOpLogger{})
	paths := &openapi3.Paths{}
	paths.Set("/ghost", nil)
	doc := &openapi3.T{Paths: paths}
	metadatas, err := r.openAPIToMetadata(doc, nil)
	if err != nil {
		t.Fatalf("openAPIToMetadata: %v", err)
	}
	if len(metadatas) != 0 {
		t.Fatalf("nil path item should yield no metadata, got %d", len(metadatas))
	}
}

func TestOpenAPIToMetadata_NilOperationSkipped(t *testing.T) {
	r := NewRegistryWithLogger(newMockClient(), &NoOpLogger{})
	item := &openapi3.PathItem{}
	// Operations() with no methods yields an empty map; add a nil op via Get/Put maps.
	item.Put = nil
	paths := &openapi3.Paths{}
	paths.Set("/x", item)
	doc := &openapi3.T{Paths: paths}
	metadatas, err := r.openAPIToMetadata(doc, nil)
	if err != nil {
		t.Fatalf("openAPIToMetadata: %v", err)
	}
	if len(metadatas) != 0 {
		t.Fatalf("no operations should yield no metadata, got %d", len(metadatas))
	}
}

// ---------------------------------------------------------------------------
// schemaToJSONSchema nil schema
// ---------------------------------------------------------------------------

func TestSchemaToJSONSchema_NilSchema(t *testing.T) {
	got, err := schemaToJSONSchema(nil)
	if err != nil {
		t.Fatalf("schemaToJSONSchema(nil): %v", err)
	}
	if got != "{}" {
		t.Fatalf("nil schema should serialize to {}, got %q", got)
	}
}
