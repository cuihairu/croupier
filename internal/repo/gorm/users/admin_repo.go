//go:build legacy_repo
// +build legacy_repo

package usersgorm

import (
	"context"
	"errors"
	"fmt"
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// AdminRepository handles admin-related database operations
type AdminRepository struct {
	db *gorm.DB
}

// NewAdminRepository creates a new admin repository
func NewAdminRepository(db *gorm.DB) *AdminRepository {
	return &AdminRepository{db: db}
}

// CreateAdmin creates a new admin
func (r *AdminRepository) CreateAdmin(ctx context.Context, admin *AdminRecord, password string) error {
	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}
	admin.PasswordHash = string(hashedPassword)

	return r.db.WithContext(ctx).Create(admin).Error
}

// GetAdminByID gets admin by ID
func (r *AdminRepository) GetAdminByID(ctx context.Context, id uint) (*AdminRecord, error) {
	var admin AdminRecord
	err := r.db.WithContext(ctx).First(&admin, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("admin not found")
		}
		return nil, err
	}
	return &admin, nil
}

// GetAdminByUsername gets admin by username
func (r *AdminRepository) GetAdminByUsername(ctx context.Context, username string) (*AdminRecord, error) {
	var admin AdminRecord
	err := r.db.WithContext(ctx).Where("username = ?", username).First(&admin).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("admin not found")
		}
		return nil, err
	}
	return &admin, nil
}

// UpdateAdmin updates admin information
func (r *AdminRepository) UpdateAdmin(ctx context.Context, id uint, updates map[string]interface{}) error {
	return r.db.WithContext(ctx).Model(&AdminRecord{}).Where("id = ?", id).Updates(updates).Error
}

// DeleteAdmin deletes an admin
func (r *AdminRepository) DeleteAdmin(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&AdminRecord{}, id).Error
}

// ListAdmins gets paginated list of admins
func (r *AdminRepository) ListAdmins(ctx context.Context, page, pageSize int, search, role string, status *int) ([]AdminRecord, int64, error) {
	var admins []AdminRecord
	var total int64

	query := r.db.WithContext(ctx).Model(&AdminRecord{})

	// Apply search filter
	if search != "" {
		query = query.Where("username LIKE ? OR display_name LIKE ? OR email LIKE ?",
			"%"+search+"%", "%"+search+"%", "%"+search+"%")
	}

	// Apply role filter
	if role != "" {
		query = query.Joins("INNER JOIN admin_role_records ON admin_records.id = admin_role_records.admin_id").
			Joins("INNER JOIN role_records ON admin_role_records.role_id = role_records.id").
			Where("role_records.name = ?", role)
	}

	// Apply status filter
	if status != nil {
		query = query.Where("status = ?", *status)
	}

	// Count total records
	err := query.Count(&total).Error
	if err != nil {
		return nil, 0, err
	}

	// Apply pagination
	offset := (page - 1) * pageSize
	err = query.Offset(offset).Limit(pageSize).Find(&admins).Error
	if err != nil {
		return nil, 0, err
	}

	return admins, total, nil
}

// ValidatePassword validates admin password
func (r *AdminRepository) ValidatePassword(ctx context.Context, username, password string) (*AdminRecord, error) {
	admin, err := r.GetAdminByUsername(ctx, username)
	if err != nil {
		return nil, err
	}

	err = bcrypt.CompareHashAndPassword([]byte(admin.PasswordHash), []byte(password))
	if err != nil {
		return nil, fmt.Errorf("invalid password")
	}

	// Update last login time
	now := time.Now().UTC()
	err = r.UpdateAdmin(ctx, admin.ID, map[string]interface{}{
		"last_login_at": now,
	})
	if err != nil {
		// Log error but don't fail login
		fmt.Printf("Failed to update last login time: %v\n", err)
	}
	admin.LastLoginAt = &now

	return admin, nil
}

// UpdatePassword updates admin password
func (r *AdminRepository) UpdatePassword(ctx context.Context, id uint, newPassword string) error {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	return r.db.WithContext(ctx).Model(&AdminRecord{}).Where("id = ?", id).Update("password_hash", hashedPassword).Error
}

// AssignRole assigns a role to an admin
func (r *AdminRepository) AssignRole(ctx context.Context, adminID, roleID uint) error {
	adminRole := &AdminRoleRecord{
		AdminID: adminID,
		RoleID:  roleID,
	}
	return r.db.WithContext(ctx).Create(adminRole).Error
}

// RemoveRole removes a role from an admin
func (r *AdminRepository) RemoveRole(ctx context.Context, adminID, roleID uint) error {
	return r.db.WithContext(ctx).
		Where("admin_id = ? AND role_id = ?", adminID, roleID).
		Delete(&AdminRoleRecord{}).Error
}

// GetAdminRoles gets all roles for an admin
func (r *AdminRepository) GetAdminRoles(ctx context.Context, adminID uint) ([]RoleRecord, error) {
	var roles []RoleRecord
	err := r.db.WithContext(ctx).
		Joins("INNER JOIN admin_role_records ON role_records.id = admin_role_records.role_id").
		Where("admin_role_records.admin_id = ?", adminID).
		Find(&roles).Error
	return roles, err
}

// SetGameScope sets game scope for an admin
func (r *AdminRepository) SetGameScope(ctx context.Context, adminID, gameID uint) error {
	scope := &AdminGameScope{
		AdminID: adminID,
		GameID:  gameID,
	}
	return r.db.WithContext(ctx).Create(scope).Error
}

// RemoveGameScope removes game scope for an admin
func (r *AdminRepository) RemoveGameScope(ctx context.Context, adminID, gameID uint) error {
	return r.db.WithContext(ctx).
		Where("admin_id = ? AND game_id = ?", adminID, gameID).
		Delete(&AdminGameScope{}).Error
}

// SetGameEnvScope sets game environment scope for an admin
func (r *AdminRepository) SetGameEnvScope(ctx context.Context, adminID, gameID uint, env string) error {
	scope := &AdminGameEnvScope{
		AdminID: adminID,
		GameID:  gameID,
		Env:     env,
	}
	return r.db.WithContext(ctx).Create(scope).Error
}

// RemoveGameEnvScope removes game environment scope for an admin
func (r *AdminRepository) RemoveGameEnvScope(ctx context.Context, adminID, gameID uint, env string) error {
	return r.db.WithContext(ctx).
		Where("admin_id = ? AND game_id = ? AND env = ?", adminID, gameID, env).
		Delete(&AdminGameEnvScope{}).Error
}
