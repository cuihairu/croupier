package model

import (
	"gorm.io/gorm"
)

// ProfilePermission stores cached permission info per admin.
type ProfilePermission struct {
	gorm.Model
	AdminID  uint   `gorm:"index"`
	Resource string `gorm:"size:64"`
	GameID   string `gorm:"size:64"`
	Env      string `gorm:"size:32"`
	Actions  JSON   `gorm:"type:json"`
}

// ProfileGame stores admin game access summary.
type ProfileGame struct {
	gorm.Model
	AdminID     uint   `gorm:"index"`
	GameID      string `gorm:"size:64"`
	GameName    string `gorm:"size:128"`
	Color       string `gorm:"size:32"`
	Envs        JSON   `gorm:"type:json"`
	Permissions JSON   `gorm:"type:json"`
}

func (ProfilePermission) TableName() string {
	return "profile_permissions"
}

func (ProfileGame) TableName() string {
	return "profile_games"
}
