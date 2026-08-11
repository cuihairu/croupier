package utils

import (
	"context"
	"math"
	"strconv"
	"strings"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/helper"
	"github.com/cuihairu/croupier/internal/model"
)

// ParseRoleID converts a path param into uint ID.
func ParseRoleID(id string) (uint, error) {
	if strings.TrimSpace(id) == "" {
		return 0, errorx.NewBadRequest("角色ID不能为空")
	}
	value, err := strconv.ParseUint(id, 10, 64)
	if err != nil {
		return 0, errorx.NewBadRequest("无效的角色ID")
	}
	if value == 0 {
		return 0, errorx.NewBadRequest("角色ID必须大于0")
	}
	if value > math.MaxUint {
		return 0, errorx.NewBadRequest("角色ID超出范围")
	}
	return uint(value), nil
}

// BuildRole converts GORM model + permission IDs into API payload.
func BuildRole(role *model.Role, permissionIDs []string) Role {
	return Role{
		Id:          int64(role.ID),
		Name:        role.Name,
		Description: role.Description,
		Category:    role.Category,
		Permissions: permissionIDs,
		CreatedAt:   helper.FormatTimestamp(role.CreatedAt),
		UpdatedAt:   helper.FormatTimestamp(role.UpdatedAt),
	}
}

// Local types for backward compatibility
type Role struct {
	Id          int64    `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Category    string   `json:"category"`
	Permissions []string `json:"permissions"`
	CreatedAt   string   `json:"createdAt"`
	UpdatedAt   string   `json:"updatedAt"`
}

// EnsurePermissionIDs validates provided permission IDs via RoleModel.
func EnsurePermissionIDs(ctx context.Context, roleModel *model.RoleModel, permissionIDs []string) ([]string, error) {
	if roleModel == nil {
		return nil, errorx.NewInternalError("role model is not initialized")
	}
	return roleModel.ValidatePermissionIDs(ctx, permissionIDs)
}

// RoleNamesFromModels extracts role names from model slice.
func RoleNamesFromModels(roles []model.Role) []string {
	if len(roles) == 0 {
		return []string{}
	}
	names := make([]string, 0, len(roles))
	for _, role := range roles {
		names = append(names, role.Name)
	}
	return names
}

// HasRole reports whether roleNames includes the given role (case-insensitive).
func HasRole(roleNames []string, role string) bool {
	want := strings.ToLower(strings.TrimSpace(role))
	if want == "" {
		return false
	}
	for _, r := range roleNames {
		if strings.ToLower(strings.TrimSpace(r)) == want {
			return true
		}
	}
	return false
}
