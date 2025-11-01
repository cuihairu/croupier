package descriptor

import (
    "os"
    "path/filepath"
    "testing"
)

func TestLoadAll_SkipsUISchemas(t *testing.T) {
    dir := t.TempDir()
    // valid descriptor
    _ = os.WriteFile(filepath.Join(dir, "foo.json"), []byte(`{"id":"x.y","version":"1.0.0"}`), 0o644)
    // ui schema that should be ignored
    _ = os.MkdirAll(filepath.Join(dir, "ui"), 0o755)
    _ = os.WriteFile(filepath.Join(dir, "ui", "x.schema.json"), []byte(`{"$schema":"draft"}`), 0o644)
    list, err := LoadAll(dir)
    if err != nil { t.Fatalf("LoadAll: %v", err) }
    if len(list) != 1 { t.Fatalf("expected 1, got %d", len(list)) }
    if list[0].ID != "x.y" { t.Fatalf("unexpected id: %s", list[0].ID) }
}

