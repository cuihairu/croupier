package agent

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	agentlocal "github.com/cuihairu/croupier/internal/platform/agentlocal"
)

// writeProviderFixture 生成 tmpDir：spec.json（可空，空则 static methods）
// + providers.yaml（version 可空，空则不写 config.version 键）。
func writeProviderFixture(t *testing.T, specDoc map[string]any, configVersion string) string {
	t.Helper()
	dir := t.TempDir()
	specPath := ""
	if specDoc != nil {
		specPath = filepath.Join(dir, "spec.json")
		raw, err := json.Marshal(specDoc)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(specPath, raw, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	yamlBody := "providers:\n  pv:\n    enabled: true\n    type: openapi\n    config:\n      base_url: http://127.0.0.1:1\n"
	if specPath != "" {
		yamlBody += "      openapiSpec: " + specPath + "\n"
	} else {
		yamlBody += "      methods:\n        - name: get_user\n          path: /api/user\n          method: GET\n"
	}
	if configVersion != "" {
		yamlBody += "      version: \"" + configVersion + "\"\n"
	}
	if err := os.WriteFile(filepath.Join(dir, "providers.yaml"), []byte(yamlBody), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func specDocWith(infoVersion string, ops map[string]map[string]string) map[string]any {
	paths := map[string]any{}
	for op, ext := range ops {
		get := map[string]any{"operationId": op}
		for k, v := range ext {
			get[k] = v
		}
		paths["/"+op] = map[string]any{"get": get}
	}
	return map[string]any{
		"openapi": "3.0.3",
		"info":    map[string]any{"title": "svc", "version": infoVersion},
		"paths":   paths,
	}
}

func loadedVersions(t *testing.T, dir string) (map[string]string, string) {
	t.Helper()
	store := agentlocal.NewLocalStore()
	pm := NewProviderManager(store, dir, nil)
	if err := pm.Load(context.Background()); err != nil {
		t.Fatalf("Load: %v", err)
	}
	got := map[string]string{}
	for fid, meta := range store.FunctionMetadata() {
		got[fid] = meta.Version
	}
	release := ""
	for _, insts := range store.List() {
		for _, in := range insts {
			release = in.Version
		}
	}
	return got, release
}

func TestProviderVersionConfigExplicitWins(t *testing.T) {
	versions, release := loadedVersions(t, writeProviderFixture(t, nil, "2.1.0"))
	if versions["pv.get_user"] != "2.1.0" {
		t.Fatalf("function version = %q, want 2.1.0", versions["pv.get_user"])
	}
	if release != "2.1.0" {
		t.Fatalf("provider release = %q, want 2.1.0", release)
	}
}

func TestProviderVersionConfigInvalidFallsBack(t *testing.T) {
	versions, release := loadedVersions(t, writeProviderFixture(t, nil, "2026-09"))
	if versions["pv.get_user"] != "1.0.0" || release != "1.0.0" {
		t.Fatalf("invalid config.version must fall back to 1.0.0, got %q/%q", versions["pv.get_user"], release)
	}
}

func TestProviderVersionFromInfoVersion(t *testing.T) {
	dir := writeProviderFixture(t, specDocWith("3.4.5", map[string]map[string]string{
		"op_a": nil, "op_b": nil,
	}), "")
	versions, release := loadedVersions(t, dir)
	if versions["pv.op_a"] != "3.4.5" || versions["pv.op_b"] != "3.4.5" {
		t.Fatalf("valid info.version must be used, got %v", versions)
	}
	if release != "3.4.5" {
		t.Fatalf("provider release must follow info.version, got %q", release)
	}
}

func TestProviderVersionOperationXVersionOverrides(t *testing.T) {
	dir := writeProviderFixture(t, specDocWith("3.4.5", map[string]map[string]string{
		"op_a": {"x-version": "0.2.0"},
		"op_b": {"x-version": "not-semver"},
	}), "")
	versions, _ := loadedVersions(t, dir)
	if versions["pv.op_a"] != "0.2.0" {
		t.Fatalf("valid x-version must override, got %q", versions["pv.op_a"])
	}
	if versions["pv.op_b"] != "3.4.5" {
		t.Fatalf("invalid x-version must fall back to batch version, got %q", versions["pv.op_b"])
	}
}

func TestProviderVersionNonSemverInfoFallsBackToDefault(t *testing.T) {
	dir := writeProviderFixture(t, specDocWith("2026-09-19", map[string]map[string]string{
		"op_a": nil,
	}), "")
	versions, release := loadedVersions(t, dir)
	if versions["pv.op_a"] != "1.0.0" || release != "1.0.0" {
		t.Fatalf("non-semver info.version must fall back to 1.0.0, got %v/%q", versions, release)
	}
}
