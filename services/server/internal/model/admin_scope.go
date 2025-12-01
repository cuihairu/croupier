package model

import "gorm.io/gorm"

// AdminGameScope limits admins to particular games.
type AdminGameScope struct {
	gorm.Model
	AdminID uint `gorm:"index:idx_admin_game_scopes_admin_id;not null"`
	GameID  uint `gorm:"index:idx_admin_game_scopes_game_id;not null"`
}

// TableName implements gorm's tabler interface.
func (AdminGameScope) TableName() string {
	return "admin_game_scopes"
}

// AdminGameEnvScope limits admins to particular game environments.
type AdminGameEnvScope struct {
	gorm.Model
	AdminID uint   `gorm:"index:idx_admin_game_env_scopes_admin_id;not null"`
	GameID  uint   `gorm:"index:idx_admin_game_env_scopes_game_id;not null"`
	Env     string `gorm:"index:idx_admin_game_env_scopes_env;size:64;not null"`
}

// TableName implements gorm's tabler interface.
func (AdminGameEnvScope) TableName() string {
	return "admin_game_env_scopes"
}
