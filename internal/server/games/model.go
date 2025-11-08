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
    // Additional metadata fields
    AliasName   string `gorm:"size:64"`
    Homepage    string `gorm:"size:255"`
    // Lifecycle status: dev (开发) | test (测试) | running (运行中) | online (在线) | offline (下线) | maintenance (维护)
    Status      string `gorm:"size:32;default:dev"`
}

// GameEnv expresses an allowed environment for a game.
type GameEnv struct {
    gorm.Model
    GameID uint   `gorm:"uniqueIndex:uniq_game_env,priority:1;not null"`
    Env    string `gorm:"size:64;uniqueIndex:uniq_game_env,priority:2;not null"`
    Description string `gorm:"type:text"`
}
