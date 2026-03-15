package externalfunc

import "strings"

type Binding struct {
	BindingType string
	BindingKey  string
	Spec        map[string]any
}

func DiscoverProviderOperations(bindings []Binding) map[string][]string {
	out := map[string][]string{}
	for _, b := range bindings {
		bt := strings.ToLower(strings.TrimSpace(b.BindingType))
		switch bt {
		case "provider", "openapi":
			parsed, ok := ParseProviderBinding(b.BindingKey, b.Spec)
			if !ok {
				continue
			}
			addProviderOperations(out, parsed.Provider, parsed.Operations)
		case "function":
			provider, method, ok := ParseFunctionID(strings.TrimSpace(b.BindingKey))
			if !ok {
				continue
			}
			provider = SanitizeKey(provider)
			method = SanitizeKey(method)
			addProviderOperations(out, provider, []string{method})
		}
	}
	return out
}

func addProviderOperations(out map[string][]string, provider string, operations []string) {
	p := SanitizeKey(provider)
	if p == "" {
		return
	}
	if _, exists := out[p]; !exists {
		out[p] = []string{}
	}
	seen := map[string]bool{}
	for _, item := range out[p] {
		seen[strings.TrimSpace(item)] = true
	}
	for _, op := range operations {
		key := SanitizeKey(op)
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		out[p] = append(out[p], key)
	}
}
