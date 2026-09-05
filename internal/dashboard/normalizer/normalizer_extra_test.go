package normalizer

import (
	"strings"
	"testing"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
)

// 缺少 ID：立即返回 id_missing 错误。
func TestNormalizeMissingID(t *testing.T) {
	got := Normalize(DescriptorInput{})
	if len(got.Diagnostics) != 1 || got.Diagnostics[0].Code != "id_missing" {
		t.Fatalf("expected id_missing, got %+v", got.Diagnostics)
	}
}

// 非法 JSON schema：分别产生 input/output schema_invalid 诊断。
func TestNormalizeInvalidSchemas(t *testing.T) {
	got := Normalize(DescriptorInput{
		ID:           "player.ban",
		InputSchema:  `{broken`,
		OutputSchema: `{broken`,
	})
	codes := map[string]bool{}
	for _, d := range got.Diagnostics {
		codes[d.Code] = true
	}
	if !codes["input_schema_invalid"] || !codes["output_schema_invalid"] {
		t.Fatalf("expected schema invalid diagnostics, got %+v", got.Diagnostics)
	}
}

// 缺失 output schema 不产生诊断；缺失 input schema 产生 warning。
func TestNormalizeMissingInputSchemaWarns(t *testing.T) {
	got := Normalize(DescriptorInput{ID: "player.ban"})
	found := false
	for _, d := range got.Diagnostics {
		if d.Code == "input_schema_missing" && d.Severity == spec.SeverityWarning {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected input_schema_missing warning, got %+v", got.Diagnostics)
	}
}

func TestNormalizeRiskAliases(t *testing.T) {
	cases := map[string]spec.RiskLevel{
		" low ":    spec.RiskSafe,
		"MEDIUM":   spec.RiskWarning,
		"critical": spec.RiskDanger,
		"safe":     spec.RiskSafe,
		"warning":  spec.RiskWarning,
		"high":     spec.RiskHigh,
		"danger":   spec.RiskDanger,
		"":         "",
		"horse":    "",
	}
	for in, want := range cases {
		if got := normalizeRisk(in); got != want {
			t.Fatalf("normalizeRisk(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestNormalizeCapabilityEmptyValue(t *testing.T) {
	_, diags := normalizeCapability("", "player.ban")
	if len(diags) != 1 || diags[0].Code != "capability_missing" {
		t.Fatalf("expected capability_missing, got %+v", diags)
	}
}

func TestIsStableKeyEdges(t *testing.T) {
	if isStableKey("") {
		t.Fatal("empty should be unstable")
	}
	if isStableKey("-lead") {
		t.Fatal("leading dash should be unstable")
	}
	if !isStableKey("9queue") {
		t.Fatal("leading digit is valid")
	}
	if isStableKey("has space") {
		t.Fatal("space should be unstable")
	}
}

func TestInferCategoryFromKeyEdges(t *testing.T) {
	if got := inferCategoryFromKey(" player.ban "); got != "player" {
		t.Fatalf("inferCategoryFromKey trimmed = %q", got)
	}
	if got := inferCategoryFromKey("plainkey"); got != "plainkey" {
		t.Fatalf("no dot = key itself, got %q", got)
	}
	if got := inferCategoryFromKey(".hidden"); got != ".hidden" {
		t.Fatalf("leading dot (idx==0) does not count → whole key, got %q", got)
	}
	if strings.TrimSpace("") != "" {
		t.Fatal("sanity")
	}
}
