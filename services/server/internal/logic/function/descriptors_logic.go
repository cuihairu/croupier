// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package function

import (
	"context"
	"sort"
	"strings"

	"github.com/cuihairu/croupier/services/server/internal/common/errorx"
	"github.com/cuihairu/croupier/services/server/internal/logic/utils"
	"github.com/cuihairu/croupier/services/server/internal/model"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type DescriptorsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取函数描述符列表
func NewDescriptorsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DescriptorsLogic {
	return &DescriptorsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DescriptorsLogic) Descriptors(req *types.DescriptorsRequest) ([]map[string]interface{}, error) {
	category := strings.TrimSpace(req.Type)

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

	// 1) DB descriptor templates (may include params schema)
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
		// If template schema wraps params, unwrap common shapes.
		if m, ok := params.(map[string]interface{}); ok {
			if v, ok := m["params"]; ok {
				params = v
			} else if v, ok := m["input"]; ok {
				params = v
			} else if v, ok := m["schema"]; ok {
				params = v
			}
		}
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
						"id":       fid,
						"version":  "",
						"category": inferCategory(fid),
						"params":   defaultParamsSchema(),
						"outputs":  nil,
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

		// 3) Extract metadata from OpenAPI operations
		operations := store.ListOpenAPIOperations()
		for fid, d := range byID {
			// Merge OpenAPI operation metadata
			if op, ok := operations[fid]; ok {
				// Extract x-category, x-risk, x-entity, x-operation from extensions
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
							d["entity"] = entityStr
						}
					}
					if operation, exists := op.Extensions["x-operation"]; exists {
						if opStr, ok := operation.(string); ok {
							d["operation"] = opStr
						}
					}
				}
				// Merge summary and description
				if op.Summary != "" {
					d["description"] = op.Summary
				}
				if op.Description != "" {
					d["description"] = op.Description
				}
			}
			// TODO: Load UI overrides from configs/ui/functions.json (removed in Stage 2)
			if _, ok := d["menu"]; !ok {
				d["menu"] = defaultMenu()
			}
		}
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
		"section": "Game",
		"group":   "Functions",
		"path":    "/game/functions/invoke",
		"order":   100,
		"hidden":  false,
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

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
