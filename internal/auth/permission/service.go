package permission

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"gorm.io/gorm"
	"github.com/cuihairu/croupier/internal/repo/gorm/users"
)

var (
	ErrPermissionDenied = errors.New("permission denied")
	ErrAdminNotFound    = errors.New("admin not found")
	ErrInvalidResource  = errors.New("invalid resource")
	ErrInvalidAction    = errors.New("invalid action")
)

// PermissionService handles permission checking logic
type PermissionService struct {
	db *gorm.DB
}

// NewPermissionService creates a new permission service
func NewPermissionService(db *gorm.DB) *PermissionService {
	return &PermissionService{db: db}
}

// CheckPermission checks if admin has permission for specific resource and action
func (s *PermissionService) CheckPermission(ctx context.Context, adminID uint, resource, action string) (bool, error) {
	if adminID == 0 {
		return false, ErrAdminNotFound
	}

	if !isValidResource(resource) {
		return false, ErrInvalidResource
	}

	if !isValidAction(action) {
		return false, ErrInvalidAction
	}

	// Get admin's roles
	var roles []users.RoleRecord
	err := s.db.Table("role_records").
		Joins("INNER JOIN admin_role_records ON role_records.id = admin_role_records.role_id").
		Where("admin_role_records.admin_id = ?", adminID).
		Find(&roles).Error
	if err != nil {
		return false, fmt.Errorf("failed to get admin roles: %w", err)
	}

	if len(roles) == 0 {
		return false, ErrPermissionDenied
	}

	// Get permissions for these roles
	var permissions []users.PermissionRecord
	err = s.db.Table("permission_records").
		Joins("INNER JOIN role_perm_records ON permission_records.id = role_perm_records.permission_id").
		Where("role_perm_records.role_id IN ?", s.getRoleIDs(roles)).
		Where("permission_records.resource = ?", resource).
		Where("permission_records.action = ?", action).
		Find(&permissions).Error
	if err != nil {
		return false, fmt.Errorf("failed to get permissions: %w", err)
	}

	return len(permissions) > 0, nil
}

// CheckGameScope checks if admin has access to specific game
func (s *PermissionService) CheckGameScope(ctx context.Context, adminID uint, gameID uint) (bool, error) {
	if adminID == 0 {
		return false, ErrAdminNotFound
	}

	var count int64
	err := s.db.Table("admin_game_scopes").
		Where("admin_id = ? AND game_id = ?", adminID, gameID).
		Count(&count).Error
	if err != nil {
		return false, fmt.Errorf("failed to check game scope: %w", err)
	}

	return count > 0, nil
}

// CheckGameEnvScope checks if admin has access to specific game environment
func (s *PermissionService) CheckGameEnvScope(ctx context.Context, adminID uint, gameID uint, env string) (bool, error) {
	if adminID == 0 {
		return false, ErrAdminNotFound
	}

	var count int64
	err := s.db.Table("admin_game_env_scopes").
		Where("admin_id = ? AND game_id = ? AND env = ?", adminID, gameID, env).
		Count(&count).Error
	if err != nil {
		return false, fmt.Errorf("failed to check game env scope: %w", err)
	}

	return count > 0, nil
}

// GetAdminPermissions returns all permissions for an admin
func (s *PermissionService) GetAdminPermissions(ctx context.Context, adminID uint) ([]users.PermissionRecord, error) {
	var permissions []users.PermissionRecord

	err := s.db.Table("permission_records").
		Joins("INNER JOIN role_perm_records ON permission_records.id = role_perm_records.permission_id").
		Joins("INNER JOIN admin_role_records ON role_perm_records.role_id = admin_role_records.role_id").
		Where("admin_role_records.admin_id = ?", adminID).
		Distinct("permission_records.*").
		Find(&permissions).Error

	if err != nil {
		return nil, fmt.Errorf("failed to get admin permissions: %w", err)
	}

	return permissions, nil
}

// GetAdminRoles returns all roles for an admin
func (s *PermissionService) GetAdminRoles(ctx context.Context, adminID uint) ([]users.RoleRecord, error) {
	var roles []users.RoleRecord

	err := s.db.Table("role_records").
		Joins("INNER JOIN admin_role_records ON role_records.id = admin_role_records.role_id").
		Where("admin_role_records.admin_id = ?", adminID).
		Find(&roles).Error

	if err != nil {
		return nil, fmt.Errorf("failed to get admin roles: %w", err)
	}

	return roles, nil
}

// Helper functions
func (s *PermissionService) getRoleIDs(roles []users.RoleRecord) []uint {
	ids := make([]uint, len(roles))
	for i, role := range roles {
		ids[i] = role.ID
	}
	return ids
}

func isValidResource(resource string) bool {
	validResources := []string{
		"admin", "role", "permission", "game", "player",
		"function", "component", "certificate", "backup",
		"analytics", "audit", "message", "ticket",
		"config", "schema", "provider", "pack",
	}

	for _, valid := range validResources {
		if strings.EqualFold(resource, valid) {
			return true
		}
	}
	return false
}

func isValidAction(action string) bool {
	validActions := []string{
		"create", "read", "update", "delete", "execute",
		"publish", "install", "uninstall", "enable", "disable",
		"start", "stop", "restart", "approve", "reject",
	}

	for _, valid := range validActions {
		if strings.EqualFold(action, valid) {
			return true
		}
	}
	return false
}