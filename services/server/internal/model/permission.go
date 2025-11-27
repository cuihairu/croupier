package model

import (
	"time"

	"gorm.io/gorm"
)

// Permission represents RBAC permissions.
type Permission struct {
	ID          string         `gorm:"primaryKey;size:64;not null"`
	Name        string         `gorm:"size:128;not null"`
	Description string         `gorm:"type:text"`
	Resource    string         `gorm:"size:128;not null"`
	Action      string         `gorm:"size:64;not null"`
	Category    string         `gorm:"size:64;not null"`
	CreatedAt   time.Time      `gorm:"autoCreateTime"`
	UpdatedAt   time.Time      `gorm:"autoUpdateTime"`
	DeletedAt   gorm.DeletedAt `gorm:"index"`
}

// TableName implements gorm's tabler interface.
func (Permission) TableName() string {
	return "permissions"
}
