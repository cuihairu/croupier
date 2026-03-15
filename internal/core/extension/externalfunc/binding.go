package externalfunc

import (
	"fmt"
	"strings"
)

type ProviderBinding struct {
	Provider   string
	Type       string
	Operations []string
	Enabled    bool
	Config     map[string]any
}

func ParseProviderBinding(bindingKey string, spec map[string]any) (ProviderBinding, bool) {
	out := ProviderBinding{
		Enabled: true,
	}
	provider := SanitizeKey(firstNonEmpty(
		stringValue(spec, "provider"),
		stringValue(spec, "provider_name"),
		stringValue(spec, "name"),
		stringValue(spec, "id"),
		bindingKey,
	))
	if provider == "" {
		return ProviderBinding{}, false
	}
	out.Provider = provider

	typ := SanitizeKey(firstNonEmpty(stringValue(spec, "type"), "openapi"))
	if typ == "" {
		typ = "openapi"
	}
	out.Type = typ

	ops := stringSliceValue(spec, "operations")
	if len(ops) == 0 {
		if op := stringValue(spec, "operation"); op != "" {
			ops = []string{op}
		}
	}
	if len(ops) == 0 {
		ops = []string{"invoke"}
	}
	out.Operations = dedupNonEmptySanitized(ops)
	if len(out.Operations) == 0 {
		out.Operations = []string{"invoke"}
	}

	if enabled, ok := boolValue(spec, "enabled"); ok {
		out.Enabled = enabled
	}
	out.Config = extractConfig(spec)
	return out, true
}

func extractConfig(spec map[string]any) map[string]any {
	out := map[string]any{}
	if spec == nil {
		return out
	}
	if raw, ok := spec["config"].(map[string]any); ok {
		for k, v := range raw {
			out[k] = v
		}
		return out
	}
	for k, v := range spec {
		key := strings.ToLower(strings.TrimSpace(k))
		switch key {
		case "provider", "provider_name", "name", "id", "bindingkey", "type", "driver", "enabled", "operations", "operation", "config":
			continue
		default:
			out[k] = v
		}
	}
	return out
}

func dedupNonEmptySanitized(values []string) []string {
	out := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for _, item := range values {
		key := SanitizeKey(item)
		if key == "" {
			continue
		}
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, key)
	}
	return out
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		s := strings.TrimSpace(v)
		if s != "" {
			return s
		}
	}
	return ""
}

func stringValue(m map[string]any, key string) string {
	if m == nil {
		return ""
	}
	v, ok := m[key]
	if !ok || v == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(v))
}

func stringSliceValue(m map[string]any, key string) []string {
	if m == nil {
		return nil
	}
	raw, ok := m[key]
	if !ok || raw == nil {
		return nil
	}
	switch items := raw.(type) {
	case []string:
		out := make([]string, 0, len(items))
		for _, item := range items {
			out = append(out, strings.TrimSpace(item))
		}
		return out
	case []any:
		out := make([]string, 0, len(items))
		for _, item := range items {
			s := strings.TrimSpace(fmt.Sprint(item))
			if s == "" || s == "<nil>" {
				continue
			}
			out = append(out, s)
		}
		return out
	case string:
		s := strings.TrimSpace(items)
		if s == "" {
			return nil
		}
		return []string{s}
	default:
		return nil
	}
}

func boolValue(m map[string]any, key string) (bool, bool) {
	if m == nil {
		return false, false
	}
	v, ok := m[key]
	if !ok || v == nil {
		return false, false
	}
	switch val := v.(type) {
	case bool:
		return val, true
	case string:
		s := strings.ToLower(strings.TrimSpace(val))
		if s == "" {
			return false, false
		}
		return !(s == "false" || s == "0" || s == "no" || s == "off"), true
	case int:
		return val != 0, true
	case int64:
		return val != 0, true
	case float64:
		return val != 0, true
	default:
		return false, false
	}
}
