package utils

import (
	"context"
	"strings"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
)

// RequireAnyPermission checks whether current admin has any of the required permission IDs.
// It loads admin roles from DB (not trusting JWT-embedded roles), expands them into permission IDs,
// and grants access if the admin has an admin-level role or wildcard permission "*".
func RequireAnyPermission(ctx context.Context, svcCtx *svc.ServiceContext, message string, required ...string) ([]model.Role, []string, error) {
	_, roles, err := LoadCurrentAdmin(ctx, svcCtx)
	if err != nil {
		return nil, nil, err
	}

	roleNames := RoleNamesFromModels(roles)
	permIDs, err := PermissionIDsFromRoles(ctx, svcCtx, roles)
	if err != nil {
		return roles, nil, err
	}

	if HasAdminRole(roleNames) || HasPermissionID(permIDs, "*") {
		return roles, permIDs, nil
	}

	for _, want := range required {
		if HasPermissionID(permIDs, want) {
			return roles, permIDs, nil
		}
	}

	if strings.TrimSpace(message) == "" {
		message = "无权执行该操作"
	}
	return roles, permIDs, errorx.NewForbidden(message)
}
