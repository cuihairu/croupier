package model

import "gorm.io/gorm"

// Role represents RBAC roles.
type Role struct {
	gorm.Model
	Name        string `gorm:"uniqueIndex;size:64;not null"`
	Description string `gorm:"size:256"`
	Category    string `gorm:"size:64"`
}

// TableName implements gorm's tabler interface.
func (Role) TableName() string {
	return "roles"
}
