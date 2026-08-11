package model

import (
	"time"

	"gorm.io/gorm"
)

// Admin represents administrator accounts.
type Admin struct {
	gorm.Model
	Username     string     `gorm:"uniqueIndex;size:64;not null"`
	Nickname     string     `gorm:"size:128;index"`
	Email        string     `gorm:"size:256;index"`
	Phone        string     `gorm:"size:32;index"`
	Avatar       string     `gorm:"size:512"`
	PasswordHash string     `gorm:"size:255"`
	Status       int        `gorm:"default:1;index"` // 1:active 0:disabled
	OTPSecret    string     `gorm:"size:64"`
	LastLoginAt  *time.Time `gorm:"index"`
	LastGameID   string     `gorm:"size:64;index"` // 上次选择的游戏 ID（业务标识）
	LastEnv      string     `gorm:"size:64"`       // 上次选择的环境
	CreatedBy    uint       `gorm:"index"`
	UpdatedBy    uint
}

// TableName implements gorm's tabler interface.
func (Admin) TableName() string {
	return "admins"
}
