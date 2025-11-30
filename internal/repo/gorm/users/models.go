//go:build legacy_repo
// +build legacy_repo

package usersgorm

import (
	"gorm.io/gorm"
	"time"
)

// GORM models (IDs as uint via gorm.Model)

// AdminRecord represents an administrator user in the system
type AdminRecord struct {
	gorm.Model
	Username     string `gorm:"uniqueIndex;size:64;not null"`
	DisplayName  string `gorm:"size:128"`
	Email        string `gorm:"size:256"`
	Phone        string `gorm:"size:32"`
	PasswordHash string `gorm:"size:255"`  // bcrypt hash
	Status       int    `gorm:"default:1"` // 1:active 0:disabled
	OTPSecret    string `gorm:"size:64"`
	LastLoginAt  *time.Time
	CreatedBy    uint // creator admin ID
	UpdatedBy    uint // updater admin ID
}

// TableName returns the table name for AdminRecord model
func (AdminRecord) TableName() string {
	return "admin_records"
}

// PermissionRecord represents a system permission
type PermissionRecord struct {
	gorm.Model
	ID          string `gorm:"primaryKey;size:64;not null"`
	Name        string `gorm:"size:128;not null"`
	Description string `gorm:"type:text"`
	Resource    string `gorm:"size:128;not null"` // Resource: admin, role, function, game, etc.
	Action      string `gorm:"size:64;not null"`  // Action: create, read, update, delete
	Category    string `gorm:"size:64;not null"`  // Category: system, game, player, etc.
}

// TableName returns the table name for PermissionRecord model
func (PermissionRecord) TableName() string {
	return "permission_records"
}

type RoleRecord struct {
	gorm.Model
	Name        string `gorm:"uniqueIndex;size:64;not null"`
	Description string `gorm:"size:256"`
	Category    string `gorm:"size:64"` // Category for role classification
}

// TableName returns the table name for RoleRecord model
func (RoleRecord) TableName() string {
	return "role_records"
}

type AdminRoleRecord struct {
	gorm.Model
	AdminID uint `gorm:"index;not null"`
	RoleID  uint `gorm:"index;not null"`
}

// TableName returns the table name for AdminRoleRecord model
func (AdminRoleRecord) TableName() string {
	return "admin_role_records"
}

type RolePermRecord struct {
	gorm.Model
	RoleID       uint   `gorm:"index;not null"`
	PermissionID string `gorm:"index;size:64;not null"`
}

// TableName returns the table name for RolePermRecord model
func (RolePermRecord) TableName() string {
	return "role_perm_records"
}

// AdminGameScope links an admin to allowed game IDs (scope control at game level)
type AdminGameScope struct {
	gorm.Model
	AdminID uint `gorm:"index;not null"`
	GameID  uint `gorm:"index;not null"`
}

func (AdminGameScope) TableName() string { return "admin_game_scopes" }

// AdminGameEnvScope links an admin to allowed envs under a specific game.
type AdminGameEnvScope struct {
	gorm.Model
	AdminID uint   `gorm:"index;not null"`
	GameID  uint   `gorm:"index;not null"`
	Env     string `gorm:"index;size:64;not null"`
}

func (AdminGameEnvScope) TableName() string { return "admin_game_env_scopes" }

func AutoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&AdminRecord{},
		&PermissionRecord{},
		&RoleRecord{},
		&AdminRoleRecord{},
		&RolePermRecord{},
		&AdminGameScope{},
		&AdminGameEnvScope{},
	)
}

// TableName returns table name for AdminRecord migration
func (AdminRecord) TableName() string {
	return "admin_records_migration"
}

// TableName returns table name for PermissionRecord migration
func (PermissionRecord) TableName() string {
	return "permission_records_migration"
}

// TableName returns table name for RoleRecord migration
func (RoleRecord) TableName() string {
	return "role_records_migration"
}

// TableName returns table name for AdminRoleRecord migration
func (AdminRoleRecord) TableName() string {
	return "admin_role_records_migration"
}

// TableName returns table name for RolePermRecord migration
func (RolePermRecord) TableName() string {
	return "role_perm_records_migration"
}

// TableName returns table name for AdminGameScope migration
func (AdminGameScope) TableName() string {
	return "admin_game_scopes_migration"
}

// TableName returns table name for AdminGameEnvScope migration
func (AdminGameEnvScope) TableName() string {
	return "admin_game_env_scopes_migration"
}

// Helpers to stamp time manually if needed
func now() time.Time { return time.Now().UTC() }
