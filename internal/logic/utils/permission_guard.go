package utils

import (
	"context"
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
		// 生产路径不可达：EnforceAnyPermission 的错误只可能来自其包内测试
		// 接缝（newLogicalModelFromString / newLogicalEnforcer，均为 rbac 包
		// 私有变量，跨包无法注入）；真实路径下 casbin 对编译期常量模型
		// （logicalPermissionModel）+ 纯内存 policy 的 Enforce/AddPolicy 均
		// 不会失败。保留该分支仅作为库行为变化时的兜底，不为其构造测试。
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

	// Capacity hints derive from a single slice length only: no addition of
	// two lengths, so the allocation size cannot overflow.
	seen := make(map[string]struct{}, len(permissionIDs))
	out := make([]string, 0, len(permissionIDs))
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
