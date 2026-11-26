package extension

import (
	"testing"
)

// Split from service_test.go; test bodies moved verbatim.

func TestParseSemVersion(t *testing.T) {
	cases := []struct {
		name      string
		input     string
		wantValid bool
		wantMajor int
		wantMinor int
		wantPatch int
		wantParts int
	}{
		{name: "full_version", input: "1.2.3", wantValid: true, wantMajor: 1, wantMinor: 2, wantPatch: 3, wantParts: 3},
		{name: "two_parts", input: "1.2", wantValid: true, wantMajor: 1, wantMinor: 2, wantPatch: 0, wantParts: 2},
		{name: "one_part", input: "1", wantValid: true, wantMajor: 1, wantMinor: 0, wantPatch: 0, wantParts: 1},
		{name: "with_v_prefix", input: "v1.2.3", wantValid: true, wantMajor: 1, wantMinor: 2, wantPatch: 3, wantParts: 3},
		{name: "with_build_meta", input: "1.2.3+build123", wantValid: true, wantMajor: 1, wantMinor: 2, wantPatch: 3, wantParts: 3},
		{name: "with_pre_release", input: "1.2.3-alpha", wantValid: true, wantMajor: 1, wantMinor: 2, wantPatch: 3, wantParts: 3},
		{name: "empty_string", input: "", wantValid: false},
		{name: "too_many_parts", input: "1.2.3.4", wantValid: false},
		{name: "no_parts", input: "...", wantValid: false},
		{name: "whitespace_only", input: "   ", wantValid: false},
		{name: "non_numeric", input: "a.b.c", wantValid: false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, ok := parseSemVersion(c.input)
			if ok != c.wantValid {
				t.Fatalf("validity mismatch, got=%v want=%v", ok, c.wantValid)
			}
			if ok {
				if got.major != c.wantMajor {
					t.Fatalf("major mismatch, got=%d want=%d", got.major, c.wantMajor)
				}
				if got.minor != c.wantMinor {
					t.Fatalf("minor mismatch, got=%d want=%d", got.minor, c.wantMinor)
				}
				if got.patch != c.wantPatch {
					t.Fatalf("patch mismatch, got=%d want=%d", got.patch, c.wantPatch)
				}
				if got.parts != c.wantParts {
					t.Fatalf("parts mismatch, got=%d want=%d", got.parts, c.wantParts)
				}
			}
		})
	}
}

func TestCompareSemVersion(t *testing.T) {
	cases := []struct {
		name     string
		a        semVersion
		b        semVersion
		expected int
	}{
		{name: "equal", a: semVersion{major: 1, minor: 2, patch: 3}, b: semVersion{major: 1, minor: 2, patch: 3}, expected: 0},
		{name: "major_greater", a: semVersion{major: 2, minor: 0, patch: 0}, b: semVersion{major: 1, minor: 9, patch: 9}, expected: 1},
		{name: "major_lesser", a: semVersion{major: 1, minor: 9, patch: 9}, b: semVersion{major: 2, minor: 0, patch: 0}, expected: -1},
		{name: "minor_greater", a: semVersion{major: 1, minor: 3, patch: 0}, b: semVersion{major: 1, minor: 2, patch: 9}, expected: 1},
		{name: "minor_lesser", a: semVersion{major: 1, minor: 2, patch: 9}, b: semVersion{major: 1, minor: 3, patch: 0}, expected: -1},
		{name: "patch_greater", a: semVersion{major: 1, minor: 2, patch: 4}, b: semVersion{major: 1, minor: 2, patch: 3}, expected: 1},
		{name: "patch_lesser", a: semVersion{major: 1, minor: 2, patch: 3}, b: semVersion{major: 1, minor: 2, patch: 4}, expected: -1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := compareSemVersion(c.a, c.b)
			if got != c.expected {
				t.Fatalf("unexpected result, got=%d expected=%d", got, c.expected)
			}
		})
	}
}

func TestMatchSingleClause(t *testing.T) {
	cases := []struct {
		name    string
		current string
		clause  string
		expect  bool
	}{
		{name: "exact_match", current: "1.2.3", clause: "1.2.3", expect: true},
		{name: "exact_no_match", current: "1.2.4", clause: "1.2.3", expect: false},
		{name: "greater_than_true", current: "1.3.0", clause: ">1.2.3", expect: true},
		{name: "greater_than_false", current: "1.2.0", clause: ">1.2.3", expect: false},
		{name: "caret_same_major", current: "1.2.0", clause: "^1.0.0", expect: true},
		{name: "caret_next_major", current: "2.0.0", clause: "^1.2.3", expect: false},
		{name: "tilde_two_parts", current: "1.2.9", clause: "~1.2.0", expect: true},
		{name: "tilde_next_minor", current: "1.3.0", clause: "~1.2.0", expect: false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cur, ok := parseSemVersion(c.current)
			if !ok {
				t.Fatalf("invalid current version: %s", c.current)
			}
			got := matchSingleClause(cur, c.clause)
			if got != c.expect {
				t.Fatalf("unexpected result, current=%s clause=%s got=%v expect=%v", c.current, c.clause, got, c.expect)
			}
		})
	}
}

func TestMatchCaretConstraint(t *testing.T) {
	cases := []struct {
		name    string
		current semVersion
		base    semVersion
		expect  bool
	}{
		{name: "within_range", current: semVersion{major: 1, minor: 5, patch: 0}, base: semVersion{major: 1, minor: 2, patch: 3}, expect: true},
		{name: "at_base", current: semVersion{major: 1, minor: 2, patch: 3}, base: semVersion{major: 1, minor: 2, patch: 3}, expect: true},
		{name: "next_major", current: semVersion{major: 2, minor: 0, patch: 0}, base: semVersion{major: 1, minor: 2, patch: 3}, expect: false},
		{name: "below_base", current: semVersion{major: 1, minor: 1, patch: 9}, base: semVersion{major: 1, minor: 2, patch: 3}, expect: false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := matchCaretConstraint(c.current, c.base)
			if got != c.expect {
				t.Fatalf("unexpected result, got=%v expect=%v", got, c.expect)
			}
		})
	}
}

func TestMatchTildeConstraint(t *testing.T) {
	cases := []struct {
		name    string
		current semVersion
		base    semVersion
		expect  bool
	}{
		{name: "within_patch_range", current: semVersion{major: 1, minor: 2, patch: 9}, base: semVersion{major: 1, minor: 2, patch: 3}, expect: true},
		{name: "at_base", current: semVersion{major: 1, minor: 2, patch: 3}, base: semVersion{major: 1, minor: 2, patch: 3}, expect: true},
		{name: "next_minor", current: semVersion{major: 1, minor: 3, patch: 0}, base: semVersion{major: 1, minor: 2, patch: 3}, expect: true},
		{name: "below_base", current: semVersion{major: 1, minor: 2, patch: 2}, base: semVersion{major: 1, minor: 2, patch: 3}, expect: false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := matchTildeConstraint(c.current, c.base)
			if got != c.expect {
				t.Fatalf("unexpected result, got=%v expect=%v", got, c.expect)
			}
		})
	}
}

func TestMatchVersionConstraint(t *testing.T) {
	cases := []struct {
		name       string
		current    string
		constraint string
		expect     bool
	}{
		{name: "exact_true", current: "1.2.3", constraint: "1.2.3", expect: true},
		{name: "exact_false", current: "1.2.4", constraint: "1.2.3", expect: false},
		{name: "gte_true", current: "1.3.0", constraint: ">=1.2.3", expect: true},
		{name: "lt_true", current: "1.2.2", constraint: "<1.2.3", expect: true},
		{name: "range_true", current: "1.4.0", constraint: ">=1.2.0, <2.0.0", expect: true},
		{name: "range_false", current: "2.1.0", constraint: ">=1.2.0, <2.0.0", expect: false},
		{name: "caret_true", current: "1.5.1", constraint: "^1.2.3", expect: true},
		{name: "caret_false", current: "2.0.0", constraint: "^1.2.3", expect: false},
		{name: "tilde_true", current: "1.2.9", constraint: "~1.2.3", expect: true},
		{name: "tilde_false", current: "1.3.0", constraint: "~1.2.3", expect: false},
		{name: "v_prefix", current: "v1.2.3", constraint: ">=1.2.0", expect: true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := matchVersionConstraint(c.current, c.constraint)
			if got != c.expect {
				t.Fatalf("unexpected result, current=%s constraint=%s got=%v expect=%v", c.current, c.constraint, got, c.expect)
			}
		})
	}
}

func TestMatchSingleClause_EdgeCases(t *testing.T) {
	t.Run("empty clause", func(t *testing.T) {
		cur, ok := parseSemVersion("1.2.3")
		if !ok {
			t.Fatal("failed to parse version")
		}
		// Empty clause - parseSemVersion returns false for empty string
		got := matchSingleClause(cur, "")
		if got {
			t.Fatalf("expected false for empty clause (fails to parse)")
		}
	})

	t.Run("whitespace clause", func(t *testing.T) {
		cur, ok := parseSemVersion("1.2.3")
		if !ok {
			t.Fatal("failed to parse version")
		}
		got := matchSingleClause(cur, "  ")
		// Whitespace fails to parse as semVersion, so returns false
		if got {
			t.Fatalf("expected false for whitespace clause (fails to parse)")
		}
	})
}

func TestMatchVersionConstraint_EdgeCases(t *testing.T) {
	t.Run("empty constraint", func(t *testing.T) {
		got := matchVersionConstraint("1.2.3", "")
		if !got {
			t.Fatalf("expected true for empty constraint")
		}
	})

	t.Run("whitespace constraint", func(t *testing.T) {
		got := matchVersionConstraint("1.2.3", "  ,  ")
		// Whitespace clauses fail to parse, but empty clauses in comma-separated list are skipped
		if !got {
			t.Fatalf("expected true for whitespace constraint (skipped)")
		}
	})

	t.Run("multiple clauses", func(t *testing.T) {
		got := matchVersionConstraint("1.5.0", ">=1.2.0, <2.0.0")
		if !got {
			t.Fatalf("expected true for valid range")
		}
	})

	t.Run("multiple clauses failing", func(t *testing.T) {
		got := matchVersionConstraint("2.5.0", ">=1.2.0, <2.0.0")
		if got {
			t.Fatalf("expected false for version outside range")
		}
	})
}

func TestParseSemVersion_PreRelease(t *testing.T) {
	cases := []struct {
		input     string
		wantValid bool
		wantMajor int
		wantMinor int
		wantPatch int
	}{
		{input: "1.2.3-alpha.1", wantValid: true, wantMajor: 1, wantMinor: 2, wantPatch: 3},
		{input: "1.2.3-beta", wantValid: true, wantMajor: 1, wantMinor: 2, wantPatch: 3},
		{input: "1.2.3-rc1", wantValid: true, wantMajor: 1, wantMinor: 2, wantPatch: 3},
	}
	for _, c := range cases {
		t.Run(c.input, func(t *testing.T) {
			got, ok := parseSemVersion(c.input)
			if ok != c.wantValid {
				t.Fatalf("validity mismatch, got=%v want=%v", ok, c.wantValid)
			}
			if ok {
				if got.major != c.wantMajor {
					t.Fatalf("major mismatch, got=%d want=%d", got.major, c.wantMajor)
				}
			}
		})
	}
}

func TestCompareSemVersion_Equal(t *testing.T) {
	a := semVersion{major: 1, minor: 2, patch: 3}
	b := semVersion{major: 1, minor: 2, patch: 3}
	if compareSemVersion(a, b) != 0 {
		t.Fatal("expected versions to be equal")
	}
}

func TestMatchCaretConstraint_ZeroVersions(t *testing.T) {
	t.Run("zero major with parts field", func(t *testing.T) {
		// For ^0.2.3, it allows >=0.2.3 <0.3.0
		current := semVersion{major: 0, minor: 2, patch: 5, parts: 3}
		base := semVersion{major: 0, minor: 2, patch: 3, parts: 3}
		if !matchCaretConstraint(current, base) {
			t.Fatal("expected true for 0.2.5 vs ^0.2.3")
		}
	})

	t.Run("zero minor with parts field - at boundary", func(t *testing.T) {
		// For ^0.0.2, it allows >=0.0.2 <0.0.3
		// So 0.0.2 should match
		current := semVersion{major: 0, minor: 0, patch: 2, parts: 3}
		base := semVersion{major: 0, minor: 0, patch: 2, parts: 3}
		if !matchCaretConstraint(current, base) {
			t.Fatal("expected true for 0.0.2 vs ^0.0.2 (exact match)")
		}
	})

	t.Run("zero minor with parts field - outside upper bound", func(t *testing.T) {
		// For ^0.0.2, upper bound is 0.0.3, so 0.0.3 should NOT match
		current := semVersion{major: 0, minor: 0, patch: 3, parts: 3}
		base := semVersion{major: 0, minor: 0, patch: 2, parts: 3}
		if matchCaretConstraint(current, base) {
			t.Fatal("expected false for 0.0.3 vs ^0.0.2 (at upper bound)")
		}
	})
}

func TestMatchTildeConstraint_SinglePart(t *testing.T) {
	current := semVersion{major: 1, minor: 0, patch: 5}
	base := semVersion{major: 1, minor: 0, patch: 0, parts: 1}
	if !matchTildeConstraint(current, base) {
		t.Fatal("expected true for single part version")
	}
}

func TestMatchVersionConstraint_SimpleEquality(t *testing.T) {
	// When constraint is just a version number, it's an exact match
	if !matchVersionConstraint("1.2.3", "1.2.3") {
		t.Fatal("expected true for exact version match")
	}

	if matchVersionConstraint("1.2.4", "1.2.3") {
		t.Fatal("expected false for different version")
	}
}

func TestMatchVersionConstraint_LessThan(t *testing.T) {
	if !matchVersionConstraint("1.2.2", "<1.2.3") {
		t.Fatal("expected true for less than")
	}

	if matchVersionConstraint("1.2.3", "<1.2.3") {
		t.Fatal("expected false for equal version with <")
	}

	if matchVersionConstraint("1.2.4", "<1.2.3") {
		t.Fatal("expected false for greater version with <")
	}
}

func TestMatchVersionConstraint_LessThanOrEqual(t *testing.T) {
	if !matchVersionConstraint("1.2.2", "<=1.2.3") {
		t.Fatal("expected true for less than or equal")
	}

	if !matchVersionConstraint("1.2.3", "<=1.2.3") {
		t.Fatal("expected true for equal version with <=")
	}

	if matchVersionConstraint("1.2.4", "<=1.2.3") {
		t.Fatal("expected false for greater version with <=")
	}
}

func TestMatchVersionConstraint_GreaterThan(t *testing.T) {
	if !matchVersionConstraint("1.2.4", ">1.2.3") {
		t.Fatal("expected true for greater than")
	}

	if matchVersionConstraint("1.2.3", ">1.2.3") {
		t.Fatal("expected false for equal version with >")
	}

	if matchVersionConstraint("1.2.2", ">1.2.3") {
		t.Fatal("expected false for less version with >")
	}
}

func TestMatchVersionConstraint_GreaterThanOrEqual(t *testing.T) {
	if !matchVersionConstraint("1.2.4", ">=1.2.3") {
		t.Fatal("expected true for greater than or equal")
	}

	if !matchVersionConstraint("1.2.3", ">=1.2.3") {
		t.Fatal("expected true for equal version with >=")
	}

	if matchVersionConstraint("1.2.2", ">=1.2.3") {
		t.Fatal("expected false for less version with >=")
	}
}

func TestMatchVersionConstraint_Prefix(t *testing.T) {
	// Test with v prefix - parseSemVersion handles v prefix
	if !matchVersionConstraint("v1.2.3", ">=1.2.0") {
		t.Fatal("expected true for version with v prefix")
	}

	// V prefix is not handled (only lowercase v)
	if !matchVersionConstraint("1.2.3", ">=1.2.0") {
		t.Fatal("expected true for version constraint")
	}
}

func TestMatchVersionConstraint_BuildMetadata(t *testing.T) {
	// Test with build metadata (should be ignored)
	if !matchVersionConstraint("1.2.3+build123", ">=1.2.0") {
		t.Fatal("expected true for version with build metadata")
	}

	if !matchVersionConstraint("1.2.3-alpha+build123", ">=1.2.0") {
		t.Fatal("expected true for version with pre-release and build")
	}
}

func TestMatchVersionConstraint_TwoPartVersion(t *testing.T) {
	// Test with two-part versions (1.2)
	// Two-part version "1.2" means exactly 1.2.0, not >=1.2
	if !matchVersionConstraint("1.2", "1.2") {
		t.Fatal("expected true for exact match of two-part version")
	}

	// 1.2.0 should match 1.2
	if !matchVersionConstraint("1.2.0", "1.2") {
		t.Fatal("expected true for 1.2.0 matching constraint 1.2")
	}

	// But 1.2.5 won't match - it's an exact match
	if matchVersionConstraint("1.2.5", "1.2") {
		t.Fatal("expected false - two-part version is exact match to 1.2.0")
	}
}

func TestMatchVersionConstraint_OnePartVersion(t *testing.T) {
	// Test with one-part versions (1)
	if !matchVersionConstraint("1", "1") {
		t.Fatal("expected true for exact match of one-part version")
	}

	// One-part version "1" means exactly 1.0.0, so 1.5 won't match
	if matchVersionConstraint("1.5", "1") {
		t.Fatal("expected false - one-part version is exact match")
	}

	// But 1.0 matches 1
	if !matchVersionConstraint("1.0", "1") {
		t.Fatal("expected true for 1.0 matching constraint 1")
	}
}
