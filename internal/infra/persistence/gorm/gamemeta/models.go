package gamemetagorm

import "gorm.io/gorm"

type GameEnvRecord struct {
	gorm.Model
	GameID string `gorm:"column:game_id;size:64;uniqueIndex:uniq_game_env,priority:1;not null"`
	Env    string `gorm:"column:env;size:64;uniqueIndex:uniq_game_env,priority:2;not null"`
}

func (GameEnvRecord) TableName() string { return "game_envs" }
