package function

// Final coverage boost: builder approval/hint edges, registry NoOp logger,
// OpenAPI load/validate failures and nil path items.

import (
	"math"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
)

func TestMetadataBuilderSetApproval(t *testing.T) {
	m, err := NewMetadataBuilder().
		SetID("fn.approval").
		SetApproval("gm.player.purge").
		Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if !m.ApprovalRequired || m.ApprovalPolicyKey != "gm.player.purge" {
		t.Fatalf("approval not applied: %+v", m)
	}
}

func TestMetadataBuilderSetFieldHintMarshalFailure(t *testing.T) {
	b := NewMetadataBuilder().SetID("fn.hint.nan")
	b.SetFieldHint("amount", "x-widget", math.NaN())
	if _, err := b.Build(); err == nil {
		t.Fatal("NaN hint value must produce a marshal error")
	}
}

func TestNormalizeHintKeyEmpty(t *testing.T) {
	if _, ok := normalizeHintKey("   "); ok {
		t.Fatal("blank hint must be rejected")
	}
}

func TestNoOpLoggerMethods(t *testing.T) {
	l := &NoOpLogger{}
	l.Debug("d")
	l.Info("i")
	l.Warn("w")
	l.Error("e")
}

func TestRegisterFromOpenAPILoadFailure(t *testing.T) {
	r := NewRegistryWithLogger(newMockClient(), &NoOpLogger{})
	err := r.RegisterFromOpenAPI([]byte(`{"openapi": `), nil, func(string) Handler { return nil })
	if err == nil {
		t.Fatal("expected load failure for malformed JSON")
	}
}

func TestRegisterFromOpenAPIValidateFailure(t *testing.T) {
	r := NewRegistryWithLogger(newMockClient(), &NoOpLogger{})
	spec := []byte(`{
		"openapi": "3.0.3",
		"info": {"title": "t", "version": "1.0.0"},
		"paths": {"/a": {"$ref": "#/components/missing"}}
	}`)
	err := r.RegisterFromOpenAPI(spec, nil, func(string) Handler { return nil })
	if err == nil {
		t.Fatal("expected validate failure for dangling $ref")
	}
}

func TestOpenAPIToMetadataNilPathItemAndOperation(t *testing.T) {
	r := NewRegistryWithLogger(newMockClient(), &NoOpLogger{})
	paths := openapi3.NewPaths()
	paths.Map()["/nil-item"] = nil
	doc := &openapi3.T{
		OpenAPI: "3.0.3",
		Info:    &openapi3.Info{Title: "t", Version: "1"},
		Paths:   paths,
	}
	metadatas, err := r.openAPIToMetadata(doc, nil)
	if err != nil {
		t.Fatalf("nil path items must be skipped: %v", err)
	}
	if len(metadatas) != 0 {
		t.Fatalf("expected no metadata, got %d", len(metadatas))
	}

	// nil operation inside a path item is skipped as well
	paths2 := openapi3.NewPaths()
	paths2.Map()["/nil-op"] = &openapi3.PathItem{Get: (*openapi3.Operation)(nil)}
	doc2 := &openapi3.T{OpenAPI: "3.0.3", Info: &openapi3.Info{Title: "t", Version: "1"}, Paths: paths2}
	metadatas, err = r.openAPIToMetadata(doc2, nil)
	if err != nil {
		t.Fatalf("nil operations must be skipped: %v", err)
	}
	if len(metadatas) != 0 {
		t.Fatalf("expected no metadata, got %d", len(metadatas))
	}
}
