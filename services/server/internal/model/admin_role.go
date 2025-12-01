package model

import "gorm.io/gorm"

// AdminRole links admins and roles.
type AdminRole struct {
	gorm.Model
	AdminID uint `gorm:"index:idx_admin_roles_admin_id;not null"`
	RoleID  uint `gorm:"index:idx_admin_roles_role_id;not null"`
}

// TableName implements gorm's tabler interface.
func (AdminRole) TableName() string {
	return "admin_roles"
}
