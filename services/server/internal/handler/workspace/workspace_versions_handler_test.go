package workspace

import "testing"

func TestResolveCurrentVersion(t *testing.T) {
	items := []map[string]interface{}{
		{"version": 4, "isCurrentDraft": true},
		{"version": 3, "isCurrentPublished": true},
	}

	if got := resolveCurrentVersion(items, "isCurrentDraft"); got != 4 {
		t.Fatalf("expected draft version 4, got %d", got)
	}
	if got := resolveCurrentVersion(items, "isCurrentPublished"); got != 3 {
		t.Fatalf("expected published version 3, got %d", got)
	}
	if got := resolveCurrentVersion(items, "missingFlag"); got != 0 {
		t.Fatalf("expected 0 for missing flag, got %d", got)
	}
}
