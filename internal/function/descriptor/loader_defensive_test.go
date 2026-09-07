package descriptor

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadAll_DuplicateCaseKeyDefensiveEmptyID(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "dup.json"), []byte(`{"id":"keep","Id":""}`), 0o644); err != nil {
		t.Fatal(err)
	}
	descs, err := LoadAll(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(descs) != 0 {
		t.Fatalf("expected defensive skip of empty descriptor ID, got %d", len(descs))
	}
}
