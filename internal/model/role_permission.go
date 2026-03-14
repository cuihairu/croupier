package model

import "gorm.io/gorm"

// RolePermission links roles and permissions.
type RolePermission struct {
	gorm.Model
	RoleID       uint   `gorm:"index:idx_role_permissions_role_id;not null"`
	PermissionID string `gorm:"index:idx_role_permissions_permission_id;size:64;not null"`
}

// TableName implements gorm's tabler interface.
func (RolePermission) TableName() string {
	return "role_permissions"
}
