package utils

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/cuihairu/croupier/services/server/internal/model"
	"github.com/cuihairu/croupier/services/server/internal/types"
)

// ParseRoleID converts a path param into uint ID.
func ParseRoleID(id string) (uint, error) {
	if strings.TrimSpace(id) == "" {
		return 0, fmt.Errorf("角色ID不能为空")
	}
	value, err := strconv.ParseUint(id, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("无效的角色ID: %w", err)
	}
	if value == 0 {
		return 0, fmt.Errorf("角色ID必须大于0")
	}
	return uint(value), nil
}

// BuildRole converts GORM model + permission IDs into API payload.
func BuildRole(role *model.Role, permissionIDs []string) types.Role {
	return types.Role{
		Id:          int64(role.ID),
		Name:        role.Name,
		Description: role.Description,
		Category:    role.Category,
		Permissions: permissionIDs,
		CreatedAt:   FormatTimestamp(role.CreatedAt),
		UpdatedAt:   FormatTimestamp(role.UpdatedAt),
	}
}

// EnsurePermissionIDs validates provided permission IDs via RoleModel.
func EnsurePermissionIDs(ctx context.Context, roleModel *model.RoleModel, permissionIDs []string) ([]string, error) {
	if roleModel == nil {
		return nil, fmt.Errorf("role model is not initialized")
	}
	return roleModel.ValidatePermissionIDs(ctx, permissionIDs)
}

// RoleNamesFromModels extracts role names from model slice.
func RoleNamesFromModels(roles []model.Role) []string {
	if len(roles) == 0 {
		return nil
	}
	names := make([]string, 0, len(roles))
	for _, role := range roles {
		names = append(names, role.Name)
	}
	return names
}
