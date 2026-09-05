package registrationguard

import (
	"testing"
)

// ForbiddenPresentationField：空/纯空白 key 提前返回。
func TestForbiddenPresentationFieldEmptyKey(t *testing.T) {
	field, forbidden := ForbiddenPresentationField("   ")
	if forbidden || field != "" {
		t.Fatalf("empty key should not be forbidden, got %q %v", field, forbidden)
	}
	field, forbidden = ForbiddenPresentationField("DISPLAY-NAME")
	if !forbidden || field != "display-name" {
		t.Fatalf("case-insensitive match failed: %q %v", field, forbidden)
	}
}

// outputSchema 命中 / 双 schema 均干净时返回 false。
func TestFindPresentationViolationOutputSchemaAndClean(t *testing.T) {
	violation, found := FindPresentationViolation(nil, ``, `{"type":"object","x-menu":"Reports"}`)
	if !found {
		t.Fatal("outputSchema violation should be found")
	}
	if violation.Field != "x-menu" || violation.Location != "outputSchema.x-menu" {
		t.Fatalf("unexpected violation %+v", violation)
	}

	if _, found := FindPresentationViolation(nil, `{"type":"object"}`, `{"type":"object"}`); found {
		t.Fatal("clean schemas should report no violation")
	}
}
