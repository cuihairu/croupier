package model

import "gorm.io/gorm"

// AdminRole links admins and roles.
type AdminRole struct {
	gorm.Model
	AdminID uint `gorm:"index;not null"`
	RoleID  uint `gorm:"index;not null"`
}

// TableName implements gorm's tabler interface.
func (AdminRole) TableName() string {
	return "admin_roles"
}
