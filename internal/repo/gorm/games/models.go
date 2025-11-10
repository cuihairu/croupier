package games

import (
    "encoding/json"
    "gorm.io/datatypes"
    "gorm.io/gorm"
)

// Game is the DB model for a game definition.
// Use gorm.Model.ID (uint) as the primary key.
type Game struct {
    gorm.Model
    Name        string
    Icon        string
    Description string `gorm:"type:text"`
    Enabled     bool   `gorm:"default:true"`
    // Additional metadata fields
    AliasName string `gorm:"size:64"`
    Homepage  string `gorm:"size:255"`
    // Lifecycle status: dev (寮€鍙? | test (娴嬭瘯) | running (杩愯涓? | online (鍦ㄧ嚎) | offline (涓嬬嚎) | maintenance (缁存姢)
    Status string `gorm:"size:32;default:dev"`
    GameType string
    GenreCode string
    // Envs stores the list of env names this game supports (JSON array of strings)
    Envs datatypes.JSON `gorm:"type:json"`
}

// GameEnv expresses an allowed environment for a game.
type GameEnv struct {
    // Global env definition; unique across system
    Env         string `gorm:"primaryKey;size:50;not null"`
    Description string `gorm:"type:text"`
    Color       string `gorm:"size:16"`
}

func (GameEnv) TableName() string { return "game_envs" }

// Helpers to encode/decode Game.Envs
func (g *Game) GetEnvList() []string {
    var arr []string
    if len(g.Envs) == 0 { return arr }
    _ = json.Unmarshal(g.Envs, &arr)
    return arr
}
func (g *Game) SetEnvList(envs []string) {
    b, _ := json.Marshal(envs)
    g.Envs = b
}

