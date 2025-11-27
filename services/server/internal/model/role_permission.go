package model

import "gorm.io/gorm"

// RolePermission links roles and permissions.
type RolePermission struct {
	gorm.Model
	RoleID       uint   `gorm:"index;not null"`
	PermissionID string `gorm:"index;size:64;not null"`
}

// TableName implements gorm's tabler interface.
func (RolePermission) TableName() string {
	return "role_permissions"
}
