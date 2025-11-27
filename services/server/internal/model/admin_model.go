package model

import (
	"context"
	"errors"
	"fmt"
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// AdminModel exposes CRUD helpers backed by gorm.
type AdminModel struct {
	db *gorm.DB
}

// NewAdminModel creates a new AdminModel.
func NewAdminModel(db *gorm.DB) *AdminModel {
	return &AdminModel{db: db}
}

// ListAdminsOptions controls pagination and filtering for listing admins.
type ListAdminsOptions struct {
	Page     int
	PageSize int
	Search   string
	Role     string
	Status   *int
}

// Create inserts an admin with the given password (hashed internally).
func (m *AdminModel) Create(ctx context.Context, admin *Admin, password string) error {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}
	admin.PasswordHash = string(hashedPassword)
	return m.db.WithContext(ctx).Create(admin).Error
}

// FindOne fetches an admin by ID.
func (m *AdminModel) FindOne(ctx context.Context, id uint) (*Admin, error) {
	var admin Admin
	if err := m.db.WithContext(ctx).First(&admin, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("admin not found")
		}
		return nil, err
	}
	return &admin, nil
}

// FindByUsername fetches an admin by username.
func (m *AdminModel) FindByUsername(ctx context.Context, username string) (*Admin, error) {
	var admin Admin
	if err := m.db.WithContext(ctx).Where("username = ?", username).First(&admin).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("admin not found")
		}
		return nil, err
	}
	return &admin, nil
}

// Update updates the admin with arbitrary fields.
func (m *AdminModel) Update(ctx context.Context, id uint, updates map[string]interface{}) error {
	return m.db.WithContext(ctx).Model(&Admin{}).Where("id = ?", id).Updates(updates).Error
}

// Delete deletes an admin by ID.
func (m *AdminModel) Delete(ctx context.Context, id uint) error {
	return m.db.WithContext(ctx).Delete(&Admin{}, id).Error
}

// List returns paginated admins plus total count.
func (m *AdminModel) List(ctx context.Context, opts ListAdminsOptions) ([]Admin, int64, error) {
	if opts.Page <= 0 {
		opts.Page = 1
	}
	if opts.PageSize <= 0 {
		opts.PageSize = 20
	}

	var (
		admins []Admin
		total  int64
	)

	query := m.db.WithContext(ctx).Model(&Admin{})

	if opts.Search != "" {
		query = query.Where("username LIKE ? OR nickname LIKE ? OR email LIKE ?",
			"%"+opts.Search+"%", "%"+opts.Search+"%", "%"+opts.Search+"%")
	}

	if opts.Role != "" {
		query = query.Joins("INNER JOIN admin_roles ON admins.id = admin_roles.admin_id").
			Joins("INNER JOIN roles ON admin_roles.role_id = roles.id").
			Where("roles.name = ?", opts.Role)
	}

	if opts.Status != nil {
		query = query.Where("status = ?", *opts.Status)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (opts.Page - 1) * opts.PageSize
	if err := query.Offset(offset).Limit(opts.PageSize).Find(&admins).Error; err != nil {
		return nil, 0, err
	}

	return admins, total, nil
}

// ValidatePassword validates credentials and updates last login timestamp.
func (m *AdminModel) ValidatePassword(ctx context.Context, username, password string) (*Admin, error) {
	admin, err := m.FindByUsername(ctx, username)
	if err != nil {
		return nil, err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(admin.PasswordHash), []byte(password)); err != nil {
		return nil, fmt.Errorf("invalid password")
	}

	now := time.Now().UTC()
	if err := m.Update(ctx, admin.ID, map[string]interface{}{
		"last_login_at": now,
	}); err != nil {
		// best-effort update
		fmt.Printf("failed to update last login time: %v\n", err)
	}
	admin.LastLoginAt = &now
	return admin, nil
}

// UpdatePassword updates an admin password.
func (m *AdminModel) UpdatePassword(ctx context.Context, id uint, newPassword string) error {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}
	return m.db.WithContext(ctx).
		Model(&Admin{}).
		Where("id = ?", id).
		Update("password_hash", string(hashedPassword)).Error
}

// AssignRole attaches a role to an admin.
func (m *AdminModel) AssignRole(ctx context.Context, adminID, roleID uint) error {
	adminRole := &AdminRole{
		AdminID: adminID,
		RoleID:  roleID,
	}
	return m.db.WithContext(ctx).Create(adminRole).Error
}

// RemoveRole detaches a role from an admin.
func (m *AdminModel) RemoveRole(ctx context.Context, adminID, roleID uint) error {
	return m.db.WithContext(ctx).
		Where("admin_id = ? AND role_id = ?", adminID, roleID).
		Delete(&AdminRole{}).Error
}

// GetAdminRoles returns roles for an admin.
func (m *AdminModel) GetAdminRoles(ctx context.Context, adminID uint) ([]Role, error) {
	var roles []Role
	err := m.db.WithContext(ctx).
		Joins("INNER JOIN admin_roles ON roles.id = admin_roles.role_id").
		Where("admin_roles.admin_id = ?", adminID).
		Find(&roles).Error
	return roles, err
}

// SetGameScope assigns a game scope to an admin.
func (m *AdminModel) SetGameScope(ctx context.Context, adminID, gameID uint) error {
	scope := &AdminGameScope{
		AdminID: adminID,
		GameID:  gameID,
	}
	return m.db.WithContext(ctx).Create(scope).Error
}

// RemoveGameScope removes a game scope entry.
func (m *AdminModel) RemoveGameScope(ctx context.Context, adminID, gameID uint) error {
	return m.db.WithContext(ctx).
		Where("admin_id = ? AND game_id = ?", adminID, gameID).
		Delete(&AdminGameScope{}).Error
}

// SetGameEnvScope assigns a game env scope to an admin.
func (m *AdminModel) SetGameEnvScope(ctx context.Context, adminID, gameID uint, env string) error {
	scope := &AdminGameEnvScope{
		AdminID: adminID,
		GameID:  gameID,
		Env:     env,
	}
	return m.db.WithContext(ctx).Create(scope).Error
}

// RemoveGameEnvScope removes a game env scope entry.
func (m *AdminModel) RemoveGameEnvScope(ctx context.Context, adminID, gameID uint, env string) error {
	return m.db.WithContext(ctx).
		Where("admin_id = ? AND game_id = ? AND env = ?", adminID, gameID, env).
		Delete(&AdminGameEnvScope{}).Error
}
