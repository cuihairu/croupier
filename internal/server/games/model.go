package games

import "gorm.io/gorm"

// Game is the DB model for a game definition.
// Use gorm.Model.ID (uint) as the primary key.
type Game struct {
    gorm.Model
    Name        string
    Icon        string
    Description string `gorm:"type:text"`
    Enabled     bool   `gorm:"default:true"`
}

// GameEnv expresses an allowed environment for a game.
type GameEnv struct {
    gorm.Model
    GameID uint   `gorm:"index:uniq_game_env,unique;not null"`
    Env    string `gorm:"size:64;index:uniq_game_env,unique;not null"`
}

