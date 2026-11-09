package openapi

import (
	"strings"
	"unicode"
)

// pathToMethodName converts a path like "/api/role/{id}" to method name "getRole".
func pathToMethodName(apiPath, method string) string {
	// Remove path parameters
	cleanPath := strings.ReplaceAll(apiPath, "}", "")
	cleanPath = strings.ReplaceAll(cleanPath, "{", "")

	// Split by /
	segments := strings.Split(cleanPath, "/")
	segments = filterEmpty(segments)

	// Build method name: {method}{pathSegments}
	var name strings.Builder
	name.WriteString(strings.ToLower(method))
	for _, s := range segments {
		if s != "api" && s != "v1" && s != "v2" {
			name.WriteString(titleWords(s))
		}
	}

	return name.String()
}

// titleWords uppercases the first letter of each word in s. It mirrors the
// deprecated strings.Title semantics this package has always relied on:
// ASCII alphanumerics and underscore belong to a word, any other character
// starts a new one (e.g. "item_id"→"Item_id", "user-profile"→"User-Profile").
func titleWords(s string) string {
	prev := ' '
	out := make([]rune, 0, len(s))
	for _, r := range s {
		if isWordSeparator(prev) {
			r = unicode.ToTitle(r)
		}
		prev = r
		out = append(out, r)
	}
	return string(out)
}

// isWordSeparator reports whether r separates words for titleWords.
func isWordSeparator(r rune) bool {
	// ASCII alphanumerics and underscore are not separators.
	if r <= 0x7F {
		switch {
		case '0' <= r && r <= '9':
			return false
		case 'a' <= r && r <= 'z':
			return false
		case 'A' <= r && r <= 'Z':
			return false
		case r == '_':
			return false
		}
		return true
	}
	// Letters and digits are not separators.
	if unicode.IsLetter(r) || unicode.IsDigit(r) {
		return false
	}
	// Otherwise, all we can do for now is treat spaces as separators.
	return unicode.IsSpace(r)
}

func filterEmpty(ss []string) []string {
	var result []string
	for _, s := range ss {
		if s != "" {
			result = append(result, s)
		}
	}
	return result
}
