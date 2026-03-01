package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDiscoverFiles_FindsOpenAPISpec(t *testing.T) {
	dir := t.TempDir()
	spec := `openapi: 3.0.3
info:
  title: Test API
  version: "1.0.0"
paths:
  /ping:
    get:
      operationId: ping
      responses:
        "200":
          description: ok
`
	if err := os.WriteFile(filepath.Join(dir, "openapi.yaml"), []byte(spec), 0o644); err != nil {
		t.Fatalf("write openapi file failed: %v", err)
	}

	pb := &PackBuilder{
		InputDir:    dir,
		Name:        "test-pack",
		Version:     "1.0.0",
		AutoConvert: true,
	}
	manifest, err := pb.discoverFiles()
	if err != nil {
		t.Fatalf("discoverFiles failed: %v", err)
	}
	if len(manifest.OpenAPISpecs) != 1 || manifest.OpenAPISpecs[0] != "openapi.yaml" {
		t.Fatalf("unexpected OpenAPISpecs: %#v", manifest.OpenAPISpecs)
	}
}

func TestGenerateOpenAPISpecs_FromLegacyDescriptor(t *testing.T) {
	dir := t.TempDir()
	descriptor := `{
  "id": "player.ban",
  "version": "1.0.0",
  "summary": "Ban Player",
  "params": {"type":"object","properties":{"playerId":{"type":"string"}}},
  "returns": {"type":"object","properties":{"ok":{"type":"boolean"}}},
  "category": "moderation",
  "entity": "Player",
  "operation": "update"
}`
	descPath := filepath.Join(dir, "player.ban.json")
	if err := os.WriteFile(descPath, []byte(descriptor), 0o644); err != nil {
		t.Fatalf("write descriptor file failed: %v", err)
	}

	pb := &PackBuilder{
		InputDir:    dir,
		Name:        "test-pack",
		Version:     "1.0.0",
		AutoConvert: true,
	}
	manifest := &Manifest{
		Name:         "test-pack",
		Version:      "1.0.0",
		Descriptors:  []string{"player.ban.json"},
		OpenAPISpecs: []string{},
	}
	gen, err := pb.generateOpenAPISpecs(manifest)
	if err != nil {
		t.Fatalf("generateOpenAPISpecs failed: %v", err)
	}
	if len(gen) != 1 {
		t.Fatalf("expected 1 generated OpenAPI doc, got %d", len(gen))
	}
	if len(manifest.OpenAPISpecs) != 1 {
		t.Fatalf("expected manifest to contain generated openapi spec, got %#v", manifest.OpenAPISpecs)
	}
}
