package version

import "testing"

func TestIsValid(t *testing.T) {
	valid := []string{"1.0.0", "0.1.2", "v2.30.1", "1.0.0-alpha", "1.0.0+build.5", " 1.2.3 "}
	for _, v := range valid {
		if !IsValid(v) {
			t.Errorf("IsValid(%q) = false, want true", v)
		}
	}
	invalid := []string{"", "   ", "1", "1.2", "1.2.3.4", "2026-09-19", "v1", "latest", "1.x.3"}
	for _, v := range invalid {
		if IsValid(v) {
			t.Errorf("IsValid(%q) = true, want false", v)
		}
	}
}

func TestValidOrDefault(t *testing.T) {
	if got := ValidOrDefault("2026-09-19", "2.1.0", "3.0.0"); got != "2.1.0" {
		t.Errorf("first valid must win, got %q", got)
	}
	if got := ValidOrDefault("", "  ", "v1"); got != DefaultVersion {
		t.Errorf("all-invalid must fall back to default, got %q", got)
	}
	if got := ValidOrDefault(); got != DefaultVersion {
		t.Errorf("empty chain must fall back to default, got %q", got)
	}
	if got := ValidOrDefault(" v1.2.3 "); got != "v1.2.3" {
		t.Errorf("valid candidate must be trimmed, got %q", got)
	}
}
