package model

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"gorm.io/gorm"
)

// RoleModel exposes CRUD helpers for Role + Permission relationships.
type RoleModel struct {
	db *gorm.DB
}

// NewRoleModel constructs a RoleModel.
func NewRoleModel(db *gorm.DB) *RoleModel {
	return &RoleModel{db: db}
}

// ListRolesOptions controls pagination/filtering when listing roles.
type ListRolesOptions struct {
	Page     int
	PageSize int
	Category string
	Search   string
}

// Create inserts a new role.
func (m *RoleModel) Create(ctx context.Context, role *Role) error {
	return m.db.WithContext(ctx).Create(role).Error
}

// FindOne fetches a role by ID.
func (m *RoleModel) FindOne(ctx context.Context, id uint) (*Role, error) {
	var role Role
	if err := m.db.WithContext(ctx).First(&role, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("role not found")
		}
		return nil, err
	}
	return &role, nil
}

// Update applies partial updates to a role.
func (m *RoleModel) Update(ctx context.Context, id uint, updates map[string]interface{}) error {
	if len(updates) == 0 {
		return nil
	}
	return m.db.WithContext(ctx).Model(&Role{}).Where("id = ?", id).Updates(updates).Error
}

// Delete removes a role.
func (m *RoleModel) Delete(ctx context.Context, id uint) error {
	return m.db.WithContext(ctx).Delete(&Role{}, id).Error
}

// List returns paginated roles plus total count.
func (m *RoleModel) List(ctx context.Context, opts ListRolesOptions) ([]Role, int64, error) {
	if opts.Page <= 0 {
		opts.Page = 1
	}
	if opts.PageSize <= 0 {
		opts.PageSize = 20
	}

	query := m.db.WithContext(ctx).Model(&Role{})
	if opts.Category != "" {
		query = query.Where("category = ?", opts.Category)
	}
	if opts.Search != "" {
		like := "%" + opts.Search + "%"
		query = query.Where("name LIKE ? OR description LIKE ?", like, like)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var roles []Role
	offset := (opts.Page - 1) * opts.PageSize
	if err := query.Order("id DESC").Offset(offset).Limit(opts.PageSize).Find(&roles).Error; err != nil {
		return nil, 0, err
	}
	return roles, total, nil
}

// ReplacePermissions rewrites role_permissions entries for the given role.
func (m *RoleModel) ReplacePermissions(ctx context.Context, roleID uint, permissionIDs []string) error {
	tx := m.db.WithContext(ctx)
	if err := tx.Where("role_id = ?", roleID).Delete(&RolePermission{}).Error; err != nil {
		return err
	}

	if len(permissionIDs) == 0 {
		return nil
	}

	records := make([]RolePermission, 0, len(permissionIDs))
	for _, pid := range permissionIDs {
		records = append(records, RolePermission{
			RoleID:       roleID,
			PermissionID: pid,
		})
	}
	return tx.Create(&records).Error
}

// GetRolePermissionIDs returns permission IDs attached to the role.
func (m *RoleModel) GetRolePermissionIDs(ctx context.Context, roleID uint) ([]string, error) {
	var entries []RolePermission
	if err := m.db.WithContext(ctx).
		Where("role_id = ?", roleID).
		Find(&entries).Error; err != nil {
		return nil, err
	}

	ids := make([]string, 0, len(entries))
	for _, entry := range entries {
		ids = append(ids, entry.PermissionID)
	}
	return ids, nil
}

// GetRolesPermissionIDs fetches permission IDs for multiple roles.
func (m *RoleModel) GetRolesPermissionIDs(ctx context.Context, roleIDs []uint) (map[uint][]string, error) {
	if len(roleIDs) == 0 {
		return map[uint][]string{}, nil
	}

	type pair struct {
		RoleID       uint
		PermissionID string
	}
	var rows []pair
	if err := m.db.WithContext(ctx).
		Table("role_permissions").
		Select("role_id, permission_id").
		Where("role_id IN ?", roleIDs).
		Order("role_id").
		Scan(&rows).Error; err != nil {
		return nil, err
	}

	result := make(map[uint][]string, len(roleIDs))
	for _, row := range rows {
		result[row.RoleID] = append(result[row.RoleID], row.PermissionID)
	}
	return result, nil
}

// ValidatePermissionIDs ensures all provided permission IDs exist and returns deduplicated IDs.
func (m *RoleModel) ValidatePermissionIDs(ctx context.Context, permissionIDs []string) ([]string, error) {
	ids := uniqueStrings(permissionIDs)
	if len(ids) == 0 {
		return nil, nil
	}

	var found []Permission
	if err := m.db.WithContext(ctx).
		Select("id").
		Where("id IN ?", ids).
		Find(&found).Error; err != nil {
		return nil, err
	}

	set := make(map[string]struct{}, len(found))
	for _, perm := range found {
		set[strings.ToLower(perm.ID)] = struct{}{}
	}

	var missing []string
	normalized := make([]string, 0, len(ids))
	for _, id := range ids {
		lower := strings.ToLower(id)
		if _, ok := set[lower]; !ok {
			missing = append(missing, id)
			continue
		}
		normalized = append(normalized, id)
	}

	if len(missing) > 0 {
		return nil, fmt.Errorf("permissions not found: %s", strings.Join(missing, ", "))
	}
	return normalized, nil
}

func uniqueStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}

	ordered := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, val := range values {
		if strings.TrimSpace(val) == "" {
			continue
		}
		key := strings.TrimSpace(val)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		ordered = append(ordered, key)
	}
	return ordered
}
