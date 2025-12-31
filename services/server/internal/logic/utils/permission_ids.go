package utils

import (
	"context"
	"strings"

	"github.com/cuihairu/croupier/services/server/internal/model"
	"github.com/cuihairu/croupier/services/server/internal/svc"
)

// PermissionIDsFromRoles expands role->permission IDs using role_permissions.
func PermissionIDsFromRoles(ctx context.Context, svcCtx *svc.ServiceContext, roles []model.Role) ([]string, error) {
	if svcCtx == nil {
		return nil, nil
	}
	if svcCtx.RoleModel == nil {
		return nil, nil
	}
	ids := make(map[string]struct{}, 64)
	for _, role := range roles {
		if role.ID == 0 {
			continue
		}
		list, err := svcCtx.GetRolePermissionIDsCached(ctx, role.ID)
		if err != nil {
			return nil, err
		}
		for _, id := range list {
			key := strings.ToLower(strings.TrimSpace(id))
			if key == "" {
				continue
			}
			ids[key] = struct{}{}
		}
	}
	out := make([]string, 0, len(ids))
	for id := range ids {
		out = append(out, id)
	}
	return out, nil
}

func HasPermissionID(permissionIDs []string, want string) bool {
	key := strings.ToLower(strings.TrimSpace(want))
	if key == "" {
		return false
	}
	for _, id := range permissionIDs {
		if strings.ToLower(strings.TrimSpace(id)) == key {
			return true
		}
	}
	return false
}
