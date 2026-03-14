package function

import (
	"context"
	"encoding/json"
	"regexp"
	"sort"
	"strings"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/logic/utils"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"

	"github.com/getkin/kin-openapi/openapi3"
)

type DescriptorsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

var nodeSeparatorDupRE = regexp.MustCompile(`[_\-.]{2,}`)

// 获取函数描述符列表
func NewDescriptorsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DescriptorsLogic {
	return &DescriptorsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DescriptorsLogic) Descriptors(req *DescriptorsRequest) ([]map[string]interface{}, error) {
	category := strings.TrimSpace(req.Type)
	termAliasMap := map[string]map[string]string{}
	termDisplayMap := map[string]map[string]map[string]string{}
	if l.svcCtx.TermDictModel != nil {
		if aliases, err := l.svcCtx.TermDictModel.AliasMap(l.ctx); err == nil {
			termAliasMap = aliases
		}
		if displays, err := loadTermDisplayMap(l.ctx, l.svcCtx.TermDictModel); err == nil {
			termDisplayMap = displays
		}
	}

	// Try to load current user (may be empty for public access)
	username, _ := utils.CurrentUsername(l.ctx)
	var roles []model.Role
	var permIDs []string
	var hasPermission bool

	if username != "" {
		// Authenticated request: check permissions
		_, rolesFromDB, err := utils.LoadCurrentAdmin(l.ctx, l.svcCtx)
		if err != nil {
			return nil, err
		}
		roles = rolesFromDB
		roleNames := utils.RoleNamesFromModels(roles)
		permIDsFromDB, err := utils.PermissionIDsFromRoles(l.ctx, l.svcCtx, roles)
		if err != nil {
			return nil, err
		}
		permIDs = permIDsFromDB
		hasPermission = utils.HasAdminRole(roleNames) || utils.HasPermissionID(permIDs, "functions:read") || utils.HasPermissionID(permIDs, "*")
		if !hasPermission {
			return nil, errorx.NewForbidden("无权访问函数目录")
		}
	}
	// Unauthenticated request: skip permission check (public read access)

	// 1) DB descriptor templates
	templates, err := l.svcCtx.FunctionModel.ListDescriptorTemplates(l.ctx, category)
	if err != nil {
		return nil, err
	}

	byID := map[string]map[string]interface{}{}
	for _, t := range templates {
		fid := strings.TrimSpace(t.DescriptorID)
		if fid == "" {
			continue
		}
		params := any(t.Schema)
		if params == nil {
			params = defaultParamsSchema()
		}

		byID[fid] = map[string]interface{}{
			"id":          fid,
			"version":     "",
			"category":    firstNonEmpty(strings.TrimSpace(t.Category), inferCategory(fid)),
			"description": strings.TrimSpace(t.Description),
			"params":      params,
			"outputs":     nil,
			"menuSource":  "default",
		}
	}

	// 2) Runtime registry functions (SDK->Agent)
	if store := l.svcCtx.RegistryStore; store != nil {
		store.Mu().RLock()
		for _, sess := range store.AgentsUnsafe() {
			if sess == nil {
				continue
			}
			for fid, meta := range sess.Functions {
				fid = strings.TrimSpace(fid)
				if fid == "" {
					continue
				}
				d := byID[fid]
				if d == nil {
					d = map[string]interface{}{
						"id":         fid,
						"version":    "",
						"category":   inferCategory(fid),
						"params":     defaultParamsSchema(),
						"outputs":    nil,
						"menuSource": "default",
					}
					byID[fid] = d
				}
				// pick best version (lexicographically, semver-friendly enough for now)
				if v := strings.TrimSpace(meta.Version); v != "" {
					if cur, _ := d["version"].(string); cur == "" || v > cur {
						d["version"] = v
					}
				}
				// best-effort enabled state
				if meta.Enabled {
					d["enabled"] = true
				}
			}
		}
		store.Mu().RUnlock()

		// 3) OpenAPI operations are the primary descriptor source.
		operations := store.ListOpenAPIOperations()
		for fid, op := range operations {
			fid = strings.TrimSpace(fid)
			if fid == "" {
				continue
			}
			d := byID[fid]
			if d == nil {
				d = map[string]interface{}{
					"id":         fid,
					"version":    "",
					"category":   inferCategory(fid),
					"params":     defaultParamsSchema(),
					"outputs":    nil,
					"menuSource": "default",
				}
				byID[fid] = d
			}
			if schema := extractOperationRequestSchema(op); schema != nil {
				d["params"] = schema
			}
			if op.Extensions != nil {
				if cat, exists := op.Extensions["x-category"]; exists {
					if catStr, ok := cat.(string); ok && catStr != "" {
						d["category"] = catStr
					}
				}
				if risk, exists := op.Extensions["x-risk"]; exists {
					if riskStr, ok := risk.(string); ok {
						d["risk"] = riskStr
					}
				}
				if entity, exists := op.Extensions["x-entity"]; exists {
					if entityStr, ok := entity.(string); ok {
						entityKey := normalizeTerm(termAliasMap, "entity", entityStr)
						d["entity"] = entityKey
						if disp := termDisplay(termDisplayMap, "entity", entityKey); len(disp) > 0 {
							d["entity_display"] = disp
						}
					}
				}
				if operation, exists := op.Extensions["x-operation"]; exists {
					if opStr, ok := operation.(string); ok {
						operationKey := normalizeTerm(termAliasMap, "operation", opStr)
						d["operation"] = operationKey
						if disp := termDisplay(termDisplayMap, "operation", operationKey); len(disp) > 0 {
							d["operation_display"] = disp
						}
					}
				}
			}
			if _, ok := d["entity"].(string); !ok {
				entity, operation := inferEntityOperationFromID(fid)
				if entity != "" {
					entityKey := normalizeTerm(termAliasMap, "entity", entity)
					d["entity"] = entityKey
					if disp := termDisplay(termDisplayMap, "entity", entityKey); len(disp) > 0 {
						d["entity_display"] = disp
					}
				}
				if operation != "" {
					operationKey := normalizeTerm(termAliasMap, "operation", operation)
					d["operation"] = operationKey
					if disp := termDisplay(termDisplayMap, "operation", operationKey); len(disp) > 0 {
						d["operation_display"] = disp
					}
				}
			}
			if op.Summary != "" {
				d["description"] = op.Summary
			}
			if op.Description != "" {
				d["description"] = op.Description
			}
		}
	}

	// 4) Merge persisted per-function menu overrides from DB metadata.
	menus, err := l.svcCtx.FunctionModel.ListFunctionMenus(l.ctx)
	if err != nil {
		return nil, err
	}
	for fid, menu := range menus {
		d := byID[fid]
		if d == nil {
			d = map[string]interface{}{
				"id":         fid,
				"version":    "",
				"category":   inferCategory(fid),
				"params":     defaultParamsSchema(),
				"outputs":    nil,
				"menuSource": "default",
			}
			byID[fid] = d
		}
		base := defaultMenu()
		mergeShallow(base, menu)
		d["menu"] = base
		d["menuSource"] = "metadata"
	}

	// 5) Ensure every descriptor has menu defaults.
	for _, d := range byID {
		entity, _ := d["entity"].(string)
		category, _ := d["category"].(string)
		fid, _ := d["id"].(string)
		if m, ok := d["menu"].(map[string]interface{}); ok && m != nil {
			base := defaultMenu()
			applyEntityMenuDefaults(base, category, entity, fid)
			mergeShallow(base, m)
			applyEntityMenuDefaults(base, category, entity, fid)
			d["menu"] = base
			if _, ok2 := d["menuSource"]; !ok2 {
				d["menuSource"] = "default"
			}
			continue
		}
		base := defaultMenu()
		applyEntityMenuDefaults(base, category, entity, fid)
		d["menu"] = base
		d["menuSource"] = "default"
	}

	out := make([]map[string]interface{}, 0, len(byID))
	for _, d := range byID {
		if category != "" {
			if c, _ := d["category"].(string); strings.TrimSpace(c) != category {
				continue
			}
		}
		out = append(out, d)
	}

	sort.Slice(out, func(i, j int) bool {
		ai, _ := out[i]["id"].(string)
		aj, _ := out[j]["id"].(string)
		return ai < aj
	})
	return out, nil
}

func inferCategory(fid string) string {
	if fid == "" {
		return ""
	}
	if idx := strings.Index(fid, "."); idx > 0 {
		return fid[:idx]
	}
	return ""
}

func defaultParamsSchema() map[string]interface{} {
	return map[string]interface{}{"type": "object", "properties": map[string]interface{}{}}
}

func defaultMenu() map[string]interface{} {
	return map[string]interface{}{
		"nodes":  []string{},
		"path":   "",
		"order":  100,
		"hidden": false,
	}
}

func mergeShallow(dst map[string]interface{}, src map[string]interface{}) {
	if dst == nil || src == nil {
		return
	}
	for k, v := range src {
		dst[k] = v
	}
}

func extractOperationRequestSchema(op *openapi3.Operation) map[string]interface{} {
	if op == nil || op.RequestBody == nil || op.RequestBody.Value == nil {
		return nil
	}
	content := op.RequestBody.Value.Content
	if len(content) == 0 {
		return nil
	}

	var media *openapi3.MediaType
	if mt, ok := content["application/json"]; ok && mt != nil {
		media = mt
	} else {
		for _, mt := range content {
			if mt != nil {
				media = mt
				break
			}
		}
	}
	if media == nil || media.Schema == nil {
		return nil
	}
	if media.Schema.Value != nil {
		var out map[string]interface{}
		b, err := json.Marshal(media.Schema.Value)
		if err != nil {
			return nil
		}
		if err := json.Unmarshal(b, &out); err != nil {
			return nil
		}
		return out
	}
	if strings.TrimSpace(media.Schema.Ref) != "" {
		return map[string]interface{}{"$ref": media.Schema.Ref}
	}
	return nil
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func sanitizeNodeKey(raw string) string {
	raw = strings.TrimSpace(strings.ToLower(raw))
	if raw == "" {
		return ""
	}
	var b strings.Builder
	for _, r := range raw {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' || r == '-' || r == '.' {
			b.WriteRune(r)
			continue
		}
		if r == ' ' || r == '/' || r == ':' {
			b.WriteRune('_')
		}
	}
	s := b.String()
	s = strings.Trim(s, "._-")
	s = nodeSeparatorDupRE.ReplaceAllString(s, "_")
	return s
}

func inferMenuNodes(category, entity, fid string) []string {
	nodes := make([]string, 0, 2)
	cat := sanitizeNodeKey(category)
	ent := sanitizeNodeKey(entity)
	if cat == "" {
		cat = sanitizeNodeKey(inferCategory(fid))
	}
	if ent == "" {
		inferredEntity, _ := inferEntityOperationFromID(fid)
		ent = sanitizeNodeKey(inferredEntity)
	}
	if cat != "" {
		nodes = append(nodes, cat)
	}
	if ent != "" {
		nodes = append(nodes, ent)
	}
	if len(nodes) == 0 {
		fallback := sanitizeNodeKey(fid)
		if fallback != "" {
			nodes = append(nodes, fallback)
		}
	}
	if len(nodes) == 0 {
		nodes = append(nodes, "general")
	}
	return nodes
}

func defaultFunctionPath(entity, fid string) string {
	entity = sanitizeNodeKey(entity)
	if entity == "" {
		inferredEntity, _ := inferEntityOperationFromID(fid)
		entity = sanitizeNodeKey(inferredEntity)
	}
	if entity != "" {
		return "/game/entities/" + entity
	}
	return "/game/functions/invoke?fid=" + fid
}

func applyEntityMenuDefaults(menu map[string]interface{}, category, entity, fid string) {
	if menu == nil {
		return
	}
	if rawNodes, ok := menu["nodes"]; ok {
		if arr, ok := rawNodes.([]string); ok {
			clean := make([]string, 0, len(arr))
			for _, n := range arr {
				if s := sanitizeNodeKey(n); s != "" {
					clean = append(clean, s)
				}
			}
			if len(clean) > 0 {
				menu["nodes"] = clean
			}
		}
		if arr, ok := rawNodes.([]interface{}); ok {
			clean := make([]string, 0, len(arr))
			for _, n := range arr {
				if s, ok := n.(string); ok {
					if normalized := sanitizeNodeKey(s); normalized != "" {
						clean = append(clean, normalized)
					}
				}
			}
			if len(clean) > 0 {
				menu["nodes"] = clean
			}
		}
	}
	nodes, _ := menu["nodes"].([]string)
	if len(nodes) == 0 {
		menu["nodes"] = inferMenuNodes(category, entity, fid)
	}
	if path, _ := menu["path"].(string); strings.TrimSpace(path) == "" {
		menu["path"] = defaultFunctionPath(entity, fid)
	}
}

func normalizeTerm(aliasMap map[string]map[string]string, domain, raw string) string {
	domain = strings.TrimSpace(strings.ToLower(domain))
	value := strings.TrimSpace(strings.ToLower(raw))
	if domain == "" || value == "" {
		return value
	}
	if m, ok := aliasMap[domain]; ok {
		if canonical, ok := m[value]; ok && canonical != "" {
			return canonical
		}
	}
	return value
}

func inferEntityOperationFromID(fid string) (string, string) {
	fid = strings.TrimSpace(strings.ToLower(fid))
	if fid == "" {
		return "", ""
	}
	parts := strings.FieldsFunc(fid, func(r rune) bool { return r == '.' || r == '_' || r == '-' || r == '/' })
	if len(parts) == 0 {
		return "", ""
	}
	actionSet := map[string]struct{}{
		"create": {}, "add": {}, "new": {}, "get": {}, "list": {}, "query": {}, "search": {}, "read": {}, "detail": {},
		"update": {}, "edit": {}, "modify": {}, "patch": {}, "delete": {}, "remove": {}, "ban": {}, "unban": {}, "mute": {},
		"invoke": {}, "execute": {}, "run": {},
	}
	operation := ""
	for i := len(parts) - 1; i >= 0; i-- {
		if _, ok := actionSet[parts[i]]; ok {
			operation = parts[i]
			break
		}
	}
	entity := ""
	stopwords := map[string]struct{}{
		"packs": {}, "pack": {}, "functions": {}, "function": {}, "examples": {}, "api": {}, "ops": {}, "gm": {},
	}
	for i := len(parts) - 1; i >= 0; i-- {
		p := parts[i]
		if p == operation {
			continue
		}
		if _, ok := actionSet[p]; ok {
			continue
		}
		if _, ok := stopwords[p]; ok {
			continue
		}
		entity = p
		break
	}
	return entity, operation
}

func loadTermDisplayMap(ctx context.Context, termModel *model.TermDictionaryModel) (map[string]map[string]map[string]string, error) {
	items, err := termModel.List(ctx, "")
	if err != nil {
		return nil, err
	}
	out := map[string]map[string]map[string]string{}
	for _, it := range items {
		domain := strings.TrimSpace(strings.ToLower(it.Domain))
		key := strings.TrimSpace(strings.ToLower(it.TermKey))
		if domain == "" || key == "" {
			continue
		}
		if _, ok := out[domain]; !ok {
			out[domain] = map[string]map[string]string{}
		}
		if _, ok := out[domain][key]; !ok {
			out[domain][key] = map[string]string{}
		}
		if zh := strings.TrimSpace(it.DisplayZh); zh != "" {
			out[domain][key]["zh"] = zh
		}
		if en := strings.TrimSpace(it.DisplayEn); en != "" {
			out[domain][key]["en"] = en
		}
	}
	return out, nil
}

func termDisplay(displayMap map[string]map[string]map[string]string, domain, key string) map[string]string {
	domain = strings.TrimSpace(strings.ToLower(domain))
	key = strings.TrimSpace(strings.ToLower(key))
	if domain == "" || key == "" {
		return nil
	}
	dm, ok := displayMap[domain]
	if !ok {
		return nil
	}
	disp, ok := dm[key]
	if !ok || len(disp) == 0 {
		return nil
	}
	out := map[string]string{}
	if zh := strings.TrimSpace(disp["zh"]); zh != "" {
		out["zh"] = zh
	}
	if en := strings.TrimSpace(disp["en"]); en != "" {
		out["en"] = en
	}
	return out
}
