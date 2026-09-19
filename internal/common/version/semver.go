// Package version centralizes function-contract semver semantics so the
// registration gate, the OpenAPI provider chain and the OpenAPI binding
// materializer agree on what counts as a valid function version.
package version

import (
	"regexp"
	"strings"
)

// DefaultVersion is the fallback used when no candidate version is valid.
// Registration paths must never materialize a function with an empty or
// non-semver version: the server-side gate drops such functions entirely
// (invalid_version warning), so upstream producers normalize instead.
const DefaultVersion = "1.0.0"

var semverPattern = regexp.MustCompile(`^v?(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(?:-[0-9A-Za-z.-]+)?(?:\+[0-9A-Za-z.-]+)?$`)

// IsValid reports whether v is a valid semver (optional leading "v").
func IsValid(v string) bool {
	return semverPattern.MatchString(strings.TrimSpace(v))
}

// ValidOrDefault returns the first IsValid candidate, or DefaultVersion
// when none qualify. Empty candidates are skipped silently; callers that
// want to surface a warning should pre-check with IsValid themselves.
func ValidOrDefault(candidates ...string) string {
	for _, c := range candidates {
		if IsValid(c) {
			return strings.TrimSpace(c)
		}
	}
	return DefaultVersion
}
