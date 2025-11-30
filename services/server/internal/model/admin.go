package model

import (
	"time"

	"gorm.io/gorm"
)

// Admin represents administrator accounts.
type Admin struct {
	gorm.Model
	Username     string `gorm:"uniqueIndex;size:64;not null"`
	Nickname     string `gorm:"size:128"`
	Email        string `gorm:"size:256"`
	Phone        string `gorm:"size:32"`
	Avatar       string `gorm:"size:512"`
	PasswordHash string `gorm:"size:255"`
	Status       int    `gorm:"default:1"` // 1:active 0:disabled
	OTPSecret    string `gorm:"size:64"`
	LastLoginAt  *time.Time
	CreatedBy    uint
	UpdatedBy    uint
}

// TableName implements gorm's tabler interface.
func (Admin) TableName() string {
	return "admins"
}
