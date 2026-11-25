package extension

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/cuihairu/croupier/internal/core/extension/externalfunc"
	"github.com/cuihairu/croupier/internal/model"
)

func (s *Service) Capabilities(ctx context.Context, id uint) (*ExtensionCapabilitiesResponse, error) {
	if err := s.requireReadPermission(ctx, "无权查看扩展能力"); err != nil {
		return nil, err
	}
	item, err := s.svcCtx.Extensions.Installation.Get(ctx, id)
	if err != nil {
		return nil, mapServiceError(err)
	}
	bindings, err := s.svcCtx.Extensions.Installation.ListBindings(ctx, id)
	if err != nil {
		return nil, mapServiceError(err)
	}
	caps, details := extractCapabilityDetailsFromBindings(bindings)
	// Fallback to manifest capabilities when bindings are empty.
	// Design-debt cleanup: this block only runs when len(caps)==0, so the old
	// "iterate caps to pre-fill capSet" loop body was never executable; the
	// output of extractCapabilities is always non-empty strings and already
	// deduplicated, so the former empty-key/duplicate-key continue guards were
	// equally dead -- both dead branches have been removed.
	if len(caps) == 0 {
		manifest, err := s.resolveManifestForRelease(ctx, item.ExtensionID, item.ReleaseVersion)
		if err == nil {
			for _, key := range extractCapabilities(manifest) {
				caps = append(caps, key)
				details = append(details, ExtensionCapabilityDetail{
					Type:       "manifest",
					Key:        key,
					Capability: key,
					Operations: []string{},
					Source:     "manifest",
				})
			}
		}
	}
	return &ExtensionCapabilitiesResponse{
		Capabilities: caps,
		Details:      details,
	}, nil
}

func (s *Service) Pages(ctx context.Context, id uint) (*ExtensionPagesResponse, error) {
	if err := s.requireReadPermission(ctx, "无权查看扩展页面"); err != nil {
		return nil, err
	}
	item, err := s.svcCtx.Extensions.Installation.Get(ctx, id)
	if err != nil {
		return nil, mapServiceError(err)
	}
	bindings, err := s.svcCtx.Extensions.Installation.ListBindings(ctx, id)
	if err != nil {
		return nil, mapServiceError(err)
	}
	pages := extractPageDetailsFromBindings(bindings)
	if len(pages) == 0 {
		manifest, err := s.resolveManifestForRelease(ctx, item.ExtensionID, item.ReleaseVersion)
		if err == nil {
			if ui, ok := manifest["ui"].(map[string]any); ok {
				if rawPages, ok := ui["pages"].([]any); ok {
					for idx, raw := range rawPages {
						route := strings.TrimSpace(fmt.Sprint(raw))
						if route == "" || route == "<nil>" {
							continue
						}
						pages = append(pages, ExtensionPageItem{
							Type:   "manifest",
							Key:    route,
							Title:  route,
							Route:  route,
							Order:  idx + 1,
							Source: "manifest",
							Schema: map[string]any{},
						})
					}
				}
			}
		}
	}
	sort.SliceStable(pages, func(i, j int) bool {
		if pages[i].Order == pages[j].Order {
			return pages[i].Key < pages[j].Key
		}
		return pages[i].Order < pages[j].Order
	})
	return &ExtensionPagesResponse{
		Pages: pages,
	}, nil
}

func extractCapabilityDetailsFromBindings(bindings []model.ExtensionRuntimeBinding) ([]string, []ExtensionCapabilityDetail) {
	capSet := map[string]bool{}
	caps := make([]string, 0)
	detailIndex := map[string]int{}
	details := make([]ExtensionCapabilityDetail, 0)
	addCap := func(capability string) {
		key := strings.TrimSpace(capability)
		if key == "" || capSet[key] {
			return
		}
		capSet[key] = true
		caps = append(caps, key)
	}
	appendOperation := func(detail *ExtensionCapabilityDetail, operations []string) {
		// Design-debt cleanup: callers always pass &details[idx], so detail is
		// never nil; the former nil guard branch was unreachable and has been
		// removed.
		seen := map[string]bool{}
		for _, op := range detail.Operations {
			seen[strings.TrimSpace(op)] = true
		}
		for _, op := range operations {
			key := strings.TrimSpace(op)
			if key == "" || seen[key] {
				continue
			}
			seen[key] = true
			detail.Operations = append(detail.Operations, key)
		}
	}
	appendPermissions := func(detail *ExtensionCapabilityDetail, permissions map[string]string) {
		if len(permissions) == 0 {
			return
		}
		if detail.Permissions == nil {
			detail.Permissions = map[string]string{}
		}
		// Design-debt cleanup: permissions comes from parseStringMapAny, whose
		// output already filters out empty keys and empty values; the former
		// empty-string continue branch was unreachable and has been removed.
		for op, perm := range permissions {
			detail.Permissions[op] = perm
		}
	}
	appendConfigKeys := func(detail *ExtensionCapabilityDetail, keys []string) {
		if detail == nil || len(keys) == 0 {
			return
		}
		seen := map[string]bool{}
		for _, key := range detail.ConfigKeys {
			seen[strings.TrimSpace(key)] = true
		}
		for _, key := range keys {
			k := strings.TrimSpace(key)
			if k == "" || seen[k] {
				continue
			}
			seen[k] = true
			detail.ConfigKeys = append(detail.ConfigKeys, k)
		}
	}
	for _, b := range bindings {
		raw := strings.TrimSpace(b.BindingType + ":" + b.BindingKey)
		if raw != "" && raw != ":" {
			addCap(raw)
		}
		bt := strings.ToLower(strings.TrimSpace(b.BindingType))
		switch bt {
		case "capability":
			capability := strings.TrimSpace(b.BindingKey)
			if capability == "" {
				continue
			}
			addCap(capability)
			spec := map[string]any{}
			if len(bytes.TrimSpace(b.SpecJSON)) > 0 {
				_ = json.Unmarshal(b.SpecJSON, &spec)
			}
			operations := parseStringSliceAny(spec["operations"])
			permissions := parseStringMapAny(spec["permissions"])
			configKeys := parseStringSliceAny(spec["config_keys"])
			if idx, exists := detailIndex[capability]; exists {
				appendOperation(&details[idx], operations)
				appendPermissions(&details[idx], permissions)
				appendConfigKeys(&details[idx], configKeys)
				continue
			}
			detailIndex[capability] = len(details)
			details = append(details, ExtensionCapabilityDetail{
				Type:        bt,
				Key:         capability,
				Capability:  capability,
				Operations:  operations,
				Permissions: permissions,
				ConfigKeys:  configKeys,
				Source:      "binding",
			})
		case "provider", "openapi":
			spec := map[string]any{}
			if len(bytes.TrimSpace(b.SpecJSON)) > 0 {
				_ = json.Unmarshal(b.SpecJSON, &spec)
			}
			parsed, ok := externalfunc.ParseProviderBinding(b.BindingKey, spec)
			if !ok {
				continue
			}
			// Design-debt cleanup: when ok, parsed.Provider is the non-empty
			// output of SanitizeKey (limited to [a-z0-9_.-]), and applying
			// SanitizeKey to it again inside Capability is idempotent and
			// still non-empty, so the former capability=="" branch was
			// unreachable and has been removed.
			capability := externalfunc.Capability(parsed.Provider)
			addCap(capability)
			if idx, exists := detailIndex[capability]; exists {
				appendOperation(&details[idx], parsed.Operations)
				continue
			}
			detailIndex[capability] = len(details)
			details = append(details, ExtensionCapabilityDetail{
				Type:       bt,
				Key:        parsed.Provider,
				Capability: capability,
				Provider:   parsed.Provider,
				Operations: append([]string{}, parsed.Operations...),
				Source:     "binding",
			})
		case "function":
			provider, method, ok := externalfunc.ParseFunctionID(strings.TrimSpace(b.BindingKey))
			if !ok {
				continue
			}
			provider = externalfunc.SanitizeKey(provider)
			method = externalfunc.SanitizeKey(method)
			if provider == "" || method == "" {
				continue
			}
			capability := externalfunc.Capability(provider)
			addCap(capability)
			if idx, exists := detailIndex[capability]; exists {
				appendOperation(&details[idx], []string{method})
				continue
			}
			detailIndex[capability] = len(details)
			details = append(details, ExtensionCapabilityDetail{
				Type:       bt,
				Key:        strings.TrimSpace(b.BindingKey),
				Capability: capability,
				Provider:   provider,
				Operations: []string{method},
				Source:     "binding",
			})
		}
	}
	return caps, details
}

func extractPageDetailsFromBindings(bindings []model.ExtensionRuntimeBinding) []ExtensionPageItem {
	items := make([]ExtensionPageItem, 0)
	seen := map[string]bool{}
	for _, b := range bindings {
		bt := strings.ToLower(strings.TrimSpace(b.BindingType))
		if bt != "page" && bt != "ui" && bt != "navigation" {
			continue
		}
		spec := map[string]any{}
		if len(bytes.TrimSpace(b.SpecJSON)) > 0 {
			_ = json.Unmarshal(b.SpecJSON, &spec)
		}
		key := strings.TrimSpace(b.BindingKey)
		if key == "" {
			key = strings.TrimSpace(fmt.Sprint(spec["id"]))
		}
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		specString := func(field string) string {
			value := strings.TrimSpace(fmt.Sprint(spec[field]))
			if value == "<nil>" {
				return ""
			}
			return value
		}
		title := specString("title")
		route := specString("route")
		icon := specString("icon")
		group := specString("group")
		requiredPermission := specString("required_permission")
		order := 0
		if rawOrder, ok := spec["order"]; ok {
			// Design-debt cleanup: spec is the product of json.Unmarshal (a
			// map[string]any), so the dynamic type of a JSON number is always
			// float64; the former int/int64 branches were unreachable and have
			// been removed.
			if v, isNum := rawOrder.(float64); isNum {
				order = int(v)
			}
		}
		if order <= 0 {
			order = len(items) + 1
		}
		if title == "" {
			title = key
		}
		if route == "" || route == "<nil>" {
			route = "/" + strings.ReplaceAll(key, ".", "/")
		}
		items = append(items, ExtensionPageItem{
			Type:               bt,
			Key:                key,
			Title:              title,
			Route:              route,
			Icon:               icon,
			Group:              group,
			Order:              order,
			RequiredPermission: requiredPermission,
			Source:             "binding",
			Schema:             spec,
		})
	}
	return items
}

func parseStringSliceAny(raw any) []string {
	list, ok := raw.([]any)
	if !ok {
		return []string{}
	}
	out := make([]string, 0, len(list))
	seen := map[string]bool{}
	for _, item := range list {
		s := strings.TrimSpace(fmt.Sprint(item))
		if s == "" || s == "<nil>" || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}

func parseStringMapAny(raw any) map[string]string {
	m, ok := raw.(map[string]any)
	if !ok {
		return map[string]string{}
	}
	out := make(map[string]string, len(m))
	for k, v := range m {
		key := strings.TrimSpace(k)
		val := strings.TrimSpace(fmt.Sprint(v))
		if key == "" || val == "" || val == "<nil>" {
			continue
		}
		out[key] = val
	}
	return out
}

func (s *Service) HealthCheck(ctx context.Context, id uint, operator string) (*ExtensionHealthCheckResponse, error) {
	if err := s.requireWritePermission(ctx, "无权执行扩展健康检查"); err != nil {
		return nil, err
	}
	item, err := s.svcCtx.Extensions.Installation.Get(ctx, id)
	if err != nil {
		return nil, mapServiceError(err)
	}
	status := deriveExtensionHealthStatus(item)
	_ = s.svcCtx.Extensions.Installation.RecordEvent(
		ctx,
		id,
		"health_check",
		"info",
		"extension health-check executed",
		operator,
		fmt.Sprintf(`{"status":"%s"}`, status),
	)
	return &ExtensionHealthCheckResponse{
		Status:    status,
		CheckedAt: time.Now().Unix(),
	}, nil
}

func connectionStatusByInstallation(item *model.ExtensionInstallation) string {
	if item == nil {
		return "unknown"
	}
	if strings.EqualFold(item.Status, "uninstalled") || strings.EqualFold(item.DesiredState, "uninstalled") {
		return "uninstalled"
	}
	if item.Enabled {
		return "ok"
	}
	return "disabled"
}
