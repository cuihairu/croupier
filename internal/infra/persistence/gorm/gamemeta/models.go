package gamemetagorm

import "gorm.io/gorm"

type GameEnvRecord struct {
    gorm.Model
    GameID string `gorm:"column:game_id;size:64;index:uniq_game_env,unique;not null"`
    Env    string `gorm:"column:env;size:64;index:uniq_game_env,unique;not null"`
}

func (GameEnvRecord) TableName() string { return "game_envs" }

