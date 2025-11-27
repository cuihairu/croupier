package model

import "gorm.io/gorm"

// AdminGameScope limits admins to particular games.
type AdminGameScope struct {
	gorm.Model
	AdminID uint `gorm:"index;not null"`
	GameID  uint `gorm:"index;not null"`
}

// TableName implements gorm's tabler interface.
func (AdminGameScope) TableName() string {
	return "admin_game_scopes"
}

// AdminGameEnvScope limits admins to particular game environments.
type AdminGameEnvScope struct {
	gorm.Model
	AdminID uint   `gorm:"index;not null"`
	GameID  uint   `gorm:"index;not null"`
	Env     string `gorm:"index;size:64;not null"`
}

// TableName implements gorm's tabler interface.
func (AdminGameEnvScope) TableName() string {
	return "admin_game_env_scopes"
}
