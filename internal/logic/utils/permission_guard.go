package utils

import (
	"context"
	"math"
	"strings"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/security/rbac"
	"github.com/cuihairu/croupier/internal/svc"
)

// RequireAnyPermission checks whether current admin has any of the required permission IDs.
// It loads admin roles from DB (not trusting JWT-embedded roles), expands them into permission IDs,
// and grants access if the admin has an admin-level role or wildcard permission "*".
func RequireAnyPermission(ctx context.Context, svcCtx *svc.ServiceContext, message string, required ...string) ([]model.Role, []string, error) {
	admin, roles, err := LoadCurrentAdmin(ctx, svcCtx)
	if err != nil {
		return nil, nil, err
	}

	roleNames := RoleNamesFromModels(roles)
	permIDs, err := PermissionIDsFromRoles(ctx, svcCtx, roles)
	if err != nil {
		return roles, nil, err
	}

	if HasAdminRole(roleNames) {
		permIDs = appendPermissionIDs(permIDs, "admin:all", "*")
	}
	allowed, err := rbac.EnforceAnyPermission(admin.Username, permIDs, required...)
	if err != nil {
		return roles, permIDs, errorx.NewInternalError("权限校验失败")
	}
	if allowed {
		return roles, permIDs, nil
	}

	if strings.TrimSpace(message) == "" {
		message = "无权执行该操作"
	}
	return roles, permIDs, errorx.NewForbidden(message)
}

func appendPermissionIDs(permissionIDs []string, values ...string) []string {
	if len(values) == 0 {
		return permissionIDs
	}

	// 检查内存分配是否会溢出
	lenA := len(permissionIDs)
	lenB := len(values)
	if lenA > math.MaxInt-lenB {
		// 溢出，返回原切片
		return permissionIDs
	}
	// lgtm[go/allocation-size-overflow] — overflow guarded above
	totalLen := lenA + lenB

	seen := make(map[string]struct{}, totalLen)
	out := make([]string, 0, totalLen)
	for _, permissionID := range permissionIDs {
		key := strings.ToLower(strings.TrimSpace(permissionID))
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, permissionID)
	}
	for _, permissionID := range values {
		key := strings.ToLower(strings.TrimSpace(permissionID))
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, permissionID)
	}
	return out
}
