package externalfunc

import "strings"

func BuildFunctionID(provider, method string) string {
	p := SanitizeKey(provider)
	m := SanitizeKey(method)
	if p == "" || m == "" {
		return ""
	}
	return "external." + p + "." + m
}

func ParseFunctionID(functionID string) (provider string, method string, ok bool) {
	fid := strings.TrimSpace(functionID)
	if !strings.HasPrefix(strings.ToLower(fid), "external.") {
		return "", "", false
	}
	parts := strings.Split(fid, ".")
	if len(parts) < 3 {
		return "", "", false
	}
	provider = strings.TrimSpace(parts[1])
	method = strings.TrimSpace(strings.Join(parts[2:], "."))
	if provider == "" || method == "" {
		return "", "", false
	}
	return provider, method, true
}

func SanitizeKey(s string) string {
	src := strings.TrimSpace(strings.ToLower(s))
	if src == "" {
		return ""
	}
	out := make([]rune, 0, len(src))
	for _, r := range src {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' || r == '-' || r == '.' {
			out = append(out, r)
			continue
		}
		if r == ' ' || r == '/' {
			out = append(out, '_')
		}
	}
	return strings.Trim(string(out), "._-")
}
