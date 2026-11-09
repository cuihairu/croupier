package extension

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/cuihairu/croupier/internal/common/errorx"
	extensioncatalog "github.com/cuihairu/croupier/internal/core/extension/catalog"
	extensioninstallation "github.com/cuihairu/croupier/internal/core/extension/installation"
	"github.com/cuihairu/croupier/internal/logic/utils"
	"gorm.io/gorm"
)

func catalogListQuery(req ExtensionCatalogListRequest) extensioncatalog.ListQuery {
	page := req.Page
	pageSize := req.PageSize
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	return extensioncatalog.ListQuery{
		Keyword: req.Keyword,
		Kind:    req.Kind,
		Status:  req.Status,
		Limit:   pageSize,
		Offset:  (page - 1) * pageSize,
	}
}

func installationListQuery(req ExtensionInstallationListRequest) extensioninstallation.ListQuery {
	page := req.Page
	pageSize := req.PageSize
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	return extensioninstallation.ListQuery{
		ExtensionID: req.ExtensionID,
		ScopeType:   req.ScopeType,
		ScopeID:     req.ScopeID,
		TargetType:  req.TargetType,
		TargetID:    req.TargetID,
		Status:      req.Status,
		Enabled:     req.Enabled,
		Limit:       pageSize,
		Offset:      (page - 1) * pageSize,
	}
}

func mapServiceError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return errorx.NewNotFound("resource not found")
	}
	if errors.Is(err, gorm.ErrInvalidDB) {
		return errorx.NewInternalError("extension service not initialized")
	}
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return errorx.NewConflict("resource already exists")
	}
	return err
}

func (s *Service) requireReadPermission(ctx context.Context, message string) error {
	_, _, err := utils.RequireAnyPermission(ctx, s.svcCtx, message,
		"admin:all",
		"extension:read",
		"extensions:read",
		"extension:manage",
		"extensions:manage",
	)
	return err
}

func (s *Service) requireWritePermission(ctx context.Context, message string) error {
	_, _, err := utils.RequireAnyPermission(ctx, s.svcCtx, message,
		"admin:all",
		"extension:write",
		"extensions:write",
		"extension:manage",
		"extensions:manage",
	)
	return err
}

func (s *Service) ResolveInstallationID(ctx context.Context, identifier string) (uint, error) {
	// 按目标类型 uint 自身宽度解析（strconv.IntSize），uint(id) 转换在任意平台
	// 都可证明安全（依据同 id_helpers.go ParseUintID 注释）。
	if id, err := strconv.ParseUint(strings.TrimSpace(identifier), 10, strconv.IntSize); err == nil {
		return uint(id), nil
	}
	item, err := s.findActiveInstallationByExtension(ctx, identifier)
	if err != nil {
		return 0, err
	}
	if item == nil {
		return 0, errorx.NewNotFound("installation not found for extension id")
	}
	return item.ID, nil
}

func mapString(m map[string]any, key string) string {
	if m == nil {
		return ""
	}
	raw, ok := m[key]
	if !ok || raw == nil {
		return ""
	}
	s := strings.TrimSpace(fmt.Sprint(raw))
	if s == "" || s == "<nil>" {
		return ""
	}
	return s
}

func normalizeExtensionID(id string) string {
	return strings.ToLower(strings.TrimSpace(id))
}

func extractCapabilities(manifest map[string]any) []string {
	if manifest == nil {
		return []string{}
	}
	raw, ok := manifest["capabilities"]
	if !ok {
		return []string{}
	}
	list, ok := raw.([]any)
	if !ok {
		return []string{}
	}
	out := make([]string, 0, len(list))
	seen := map[string]bool{}
	for _, item := range list {
		switch v := item.(type) {
		case string:
			key := strings.TrimSpace(v)
			if key == "" || seen[key] {
				continue
			}
			seen[key] = true
			out = append(out, key)
		case map[string]any:
			key := mapString(v, "id")
			if key == "" {
				key = mapString(v, "name")
			}
			if key == "" || seen[key] {
				continue
			}
			seen[key] = true
			out = append(out, key)
		}
	}
	return out
}

func extractTags(manifest map[string]any) []string {
	if manifest == nil {
		return []string{}
	}
	raw, ok := manifest["tags"]
	if !ok {
		return []string{}
	}
	list, ok := raw.([]any)
	if !ok {
		return []string{}
	}
	out := make([]string, 0, len(list))
	seen := map[string]bool{}
	for _, item := range list {
		tag := strings.TrimSpace(fmt.Sprint(item))
		if tag == "" || tag == "<nil>" || seen[tag] {
			continue
		}
		seen[tag] = true
		out = append(out, tag)
	}
	return out
}

func extractDefaultInstall(manifest map[string]any) bool {
	if manifest == nil {
		return false
	}
	raw, ok := manifest["default_install"]
	if !ok {
		raw = manifest["defaultInstall"]
	}
	switch v := raw.(type) {
	case bool:
		return v
	case string:
		s := strings.ToLower(strings.TrimSpace(v))
		return s == "true" || s == "1" || s == "yes"
	case float64:
		return v != 0
	case int:
		return v != 0
	default:
		return false
	}
}

func (s *Service) resolveCatalogMetadata(ctx context.Context, extensionID, latestVersion string) (bool, []string) {
	_, releases, err := s.svcCtx.Extensions.Catalog.Get(ctx, extensionID)
	if err != nil || len(releases) == 0 {
		return false, []string{}
	}
	raw := firstManifest(releases)
	for _, r := range releases {
		if strings.EqualFold(strings.TrimSpace(r.Version), strings.TrimSpace(latestVersion)) {
			raw = string(r.ManifestJSON)
			break
		}
	}
	manifest := s.svcCtx.Extensions.Manifest.MustJSON(raw)
	return extractDefaultInstall(manifest), extractTags(manifest)
}

type semVersion struct {
	major int
	minor int
	patch int
	parts int
}

func matchVersionConstraint(current, constraint string) bool {
	cur, ok := parseSemVersion(current)
	if !ok {
		return false
	}
	c := strings.TrimSpace(constraint)
	if c == "" {
		return true
	}
	clauses := strings.Split(c, ",")
	for _, rawClause := range clauses {
		clause := strings.TrimSpace(rawClause)
		if clause == "" {
			continue
		}
		if !matchSingleClause(cur, clause) {
			return false
		}
	}
	return true
}

func matchSingleClause(current semVersion, clause string) bool {
	op := ""
	rest := clause
	switch {
	case strings.HasPrefix(clause, ">="), strings.HasPrefix(clause, "<="):
		op, rest = clause[:2], clause[2:]
	case strings.HasPrefix(clause, ">"), strings.HasPrefix(clause, "<"),
		strings.HasPrefix(clause, "="), strings.HasPrefix(clause, "^"), strings.HasPrefix(clause, "~"):
		op, rest = clause[:1], clause[1:]
	default:
		op = "="
	}
	target, ok := parseSemVersion(rest)
	if !ok {
		return false
	}
	cmp := compareSemVersion(current, target)

	// applyVersionOp 对未知操作符恒返 false（显式拒绝），直接传播即可。
	return applyVersionOp(op, current, target, cmp)
}

// applyVersionOp 按 op 对版本比较结果分派。抽为独立函数使未知操作符的
// 拒绝分支可被单测直接以任意 op 驱动（matchSingleClause 内部的 op 恒为
// 前缀提取的七种字面量之一，查表 miss 仅在 opMatchers 被裁剪时出现）。
func applyVersionOp(op string, current, target semVersion, cmp int) bool {
	fn, exists := opMatchers[op]
	if !exists {
		return false
	}
	return fn(current, target, cmp)
}

// opMatchers 是版本约束操作符的分派表。
var opMatchers = map[string]func(current, target semVersion, cmp int) bool{
	"=":  func(_, _ semVersion, cmp int) bool { return cmp == 0 },
	">":  func(_, _ semVersion, cmp int) bool { return cmp > 0 },
	">=": func(_, _ semVersion, cmp int) bool { return cmp >= 0 },
	"<":  func(_, _ semVersion, cmp int) bool { return cmp < 0 },
	"<=": func(_, _ semVersion, cmp int) bool { return cmp <= 0 },
	"^": func(current, target semVersion, _ int) bool {
		return matchCaretConstraint(current, target)
	},
	"~": func(current, target semVersion, _ int) bool {
		return matchTildeConstraint(current, target)
	},
}

func matchCaretConstraint(current, base semVersion) bool {
	if compareSemVersion(current, base) < 0 {
		return false
	}
	var upper semVersion
	if base.major > 0 {
		upper = semVersion{major: base.major + 1}
	} else if base.minor > 0 {
		upper = semVersion{major: 0, minor: base.minor + 1}
	} else {
		upper = semVersion{major: 0, minor: 0, patch: base.patch + 1}
	}
	return compareSemVersion(current, upper) < 0
}

func matchTildeConstraint(current, base semVersion) bool {
	if compareSemVersion(current, base) < 0 {
		return false
	}
	var upper semVersion
	if base.parts <= 1 {
		upper = semVersion{major: base.major + 1}
	} else {
		upper = semVersion{major: base.major, minor: base.minor + 1}
	}
	return compareSemVersion(current, upper) < 0
}

func parseSemVersion(raw string) (semVersion, bool) {
	s := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(raw), "v"))
	if s == "" {
		return semVersion{}, false
	}
	if idx := strings.IndexAny(s, "+-"); idx >= 0 {
		s = s[:idx]
	}
	parts := strings.Split(s, ".")
	if len(parts) == 0 || len(parts) > 3 {
		return semVersion{}, false
	}
	out := semVersion{parts: len(parts)}
	parsePart := func(i int, dst *int) bool {
		if i >= len(parts) {
			*dst = 0
			return true
		}
		p := strings.TrimSpace(parts[i])
		if p == "" {
			return false
		}
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return false
		}
		*dst = n
		return true
	}
	if !parsePart(0, &out.major) || !parsePart(1, &out.minor) || !parsePart(2, &out.patch) {
		return semVersion{}, false
	}
	return out, true
}

func compareSemVersion(a, b semVersion) int {
	if a.major != b.major {
		if a.major < b.major {
			return -1
		}
		return 1
	}
	if a.minor != b.minor {
		if a.minor < b.minor {
			return -1
		}
		return 1
	}
	if a.patch != b.patch {
		if a.patch < b.patch {
			return -1
		}
		return 1
	}
	return 0
}
