package permissions

import (
	"encoding/json"
	"gorm.io/gorm"
	"time"
)

// Permission represents a permission definition
type Permission struct {
	ID          uint   `gorm:"primaryKey"`
	Code        string `gorm:"column:code;uniqueIndex;size:100"`
	Name        string `gorm:"column:name;size:200"`
	Description string `gorm:"column:description;type:text"`
	Category    string `gorm:"column:category;index;size:50"`
	Module      string `gorm:"column:module;index;size:50"`
	Enabled     bool   `gorm:"column:enabled;default:true"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (Permission) TableName() string {
	return "permissions"
}

// Role represents a role definition
type Role struct {
	ID          uint   `gorm:"primaryKey"`
	Code        string `gorm:"column:code;uniqueIndex;size:50"`
	Name        string `gorm:"column:name;size:100"`
	Description string `gorm:"column:description;type:text"`
	Level       int    `gorm:"column:level;default:0"` // Role hierarchy level
	Enabled     bool   `gorm:"column:enabled;default:true"`
	CreatedAt   time.Time
	UpdatedAt   time.Time

	// Many-to-many relationship
	Permissions []Permission `gorm:"many2many:role_permissions;"`
}

func (Role) TableName() string {
	return "roles"
}

// RolePermission represents the many-to-many relationship
type RolePermission struct {
	RoleID       uint `gorm:"primaryKey"`
	PermissionID uint `gorm:"primaryKey"`
	CreatedAt    time.Time
}

func (RolePermission) TableName() string {
	return "role_permissions"
}

// UserRole represents user role assignments
type UserRole struct {
	ID        uint   `gorm:"primaryKey"`
	UserID    string `gorm:"column:user_id;index;size:100"`
	RoleCode  string `gorm:"column:role_code;index;size:50"`
	GameID    string `gorm:"column:game_id;index;size:50"` // Scoped by game
	Env       string `gorm:"column:env;index;size:20"`     // Environment (prod/test/dev)
	CreatedBy string `gorm:"column:created_by;size:100"`
	CreatedAt time.Time
	UpdatedAt time.Time

	Role Role `gorm:"foreignKey:RoleCode;references:Code"`
}

func (UserRole) TableName() string {
	return "user_roles"
}

// Store handles permission and role management
type Store struct {
	db *gorm.DB
}

func NewStore(db *gorm.DB) *Store {
	return &Store{db: db}
}

// AutoMigrate creates all permission-related tables
func (s *Store) AutoMigrate() error {
	return s.db.AutoMigrate(&Permission{}, &Role{}, &RolePermission{}, &UserRole{})
}

// Permission operations
func (s *Store) CreatePermission(permission *Permission) error {
	return s.db.Create(permission).Error
}

func (s *Store) GetPermission(code string) (*Permission, error) {
	var permission Permission
	err := s.db.Where("code = ?", code).First(&permission).Error
	return &permission, err
}

func (s *Store) ListPermissions(category string) ([]Permission, error) {
	var permissions []Permission
	query := s.db
	if category != "" {
		query = query.Where("category = ?", category)
	}
	err := query.Where("enabled = ?", true).Find(&permissions).Error
	return permissions, err
}

// Role operations
func (s *Store) CreateRole(role *Role) error {
	return s.db.Create(role).Error
}

func (s *Store) GetRole(code string) (*Role, error) {
	var role Role
	err := s.db.Preload("Permissions").Where("code = ?", code).First(&role).Error
	return &role, err
}

func (s *Store) ListRoles() ([]Role, error) {
	var roles []Role
	err := s.db.Preload("Permissions").Where("enabled = ?", true).Find(&roles).Error
	return roles, err
}

func (s *Store) AssignPermissionToRole(roleCode, permissionCode string) error {
	var role Role
	var permission Permission

	if err := s.db.Where("code = ?", roleCode).First(&role).Error; err != nil {
		return err
	}
	if err := s.db.Where("code = ?", permissionCode).First(&permission).Error; err != nil {
		return err
	}

	return s.db.Model(&role).Association("Permissions").Append(&permission)
}

// User role operations
func (s *Store) AssignRoleToUser(userID, roleCode, gameID, env, createdBy string) error {
	userRole := &UserRole{
		UserID:    userID,
		RoleCode:  roleCode,
		GameID:    gameID,
		Env:       env,
		CreatedBy: createdBy,
	}
	return s.db.Create(userRole).Error
}

func (s *Store) GetUserRoles(userID, gameID, env string) ([]Role, error) {
	var roles []Role
	query := s.db.Table("roles").
		Joins("JOIN user_roles ON roles.code = user_roles.role_code").
		Where("user_roles.user_id = ?", userID)

	if gameID != "" {
		query = query.Where("user_roles.game_id = ?", gameID)
	}
	if env != "" {
		query = query.Where("user_roles.env = ?", env)
	}

	err := query.Preload("Permissions").Find(&roles).Error
	return roles, err
}

func (s *Store) GetUserPermissions(userID, gameID, env string) ([]string, error) {
	roles, err := s.GetUserRoles(userID, gameID, env)
	if err != nil {
		return nil, err
	}

	permissionSet := make(map[string]bool)
	for _, role := range roles {
		for _, permission := range role.Permissions {
			if permission.Enabled {
				permissionSet[permission.Code] = true
			}
		}
	}

	permissions := make([]string, 0, len(permissionSet))
	for code := range permissionSet {
		permissions = append(permissions, code)
	}

	return permissions, nil
}

// JSON import/export functions
type PermissionImport struct {
	Code        string `json:"code"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Category    string `json:"category"`
	Module      string `json:"module"`
}

type RoleImport struct {
	Code        string   `json:"code"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Level       int      `json:"level"`
	Permissions []string `json:"permissions"`
}

func (s *Store) ImportPermissionsFromJSON(data []byte) error {
	var imports []PermissionImport
	if err := json.Unmarshal(data, &imports); err != nil {
		return err
	}

	for _, imp := range imports {
		permission := &Permission{
			Code:        imp.Code,
			Name:        imp.Name,
			Description: imp.Description,
			Category:    imp.Category,
			Module:      imp.Module,
			Enabled:     true,
		}

		// Upsert - update if exists, create if not
		s.db.Where("code = ?", permission.Code).FirstOrCreate(permission)
	}

	return nil
}

func (s *Store) ImportRolesFromJSON(data []byte) error {
	var imports []RoleImport
	if err := json.Unmarshal(data, &imports); err != nil {
		return err
	}

	for _, imp := range imports {
		role := &Role{
			Code:        imp.Code,
			Name:        imp.Name,
			Description: imp.Description,
			Level:       imp.Level,
			Enabled:     true,
		}

		// Create or update role
		if err := s.db.Where("code = ?", role.Code).FirstOrCreate(role).Error; err != nil {
			continue
		}

		// Assign permissions
		for _, permCode := range imp.Permissions {
			s.AssignPermissionToRole(role.Code, permCode)
		}
	}

	return nil
}