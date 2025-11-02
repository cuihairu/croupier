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
    GameID uint   `gorm:"uniqueIndex:uniq_game_env,priority:1;not null"`
    Env    string `gorm:"size:64;uniqueIndex:uniq_game_env,priority:2;not null"`
}
