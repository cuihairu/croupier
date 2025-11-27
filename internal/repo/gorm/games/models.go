package games

import (
	"encoding/json"
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// GameRecord is the DB model for a game definition.
type GameRecord struct {
	gorm.Model
	Name        string `gorm:"size:128;not null"`
	Icon        string `gorm:"size:255"`
	Description string `gorm:"type:text"`
	Enabled     bool   `gorm:"default:true"`
	// Additional metadata fields
	AliasName string `gorm:"size:64"`
	Homepage  string `gorm:"size:255"`
	// Lifecycle status: dev | test | running | online | offline | maintenance
	Status    string `gorm:"size:32;default:dev"`
	GameType  string `gorm:"size:64"`
	GenreCode string `gorm:"size:64"`
	Config    string `gorm:"type:text"` // Game specific configuration
	// Envs stores the list of env names this game supports (JSON array of strings)
	Envs datatypes.JSON `gorm:"type:json"`
}

// TableName returns the table name for GameRecord model
func (GameRecord) TableName() string {
	return "game_records"
}

// GameEnvRecord expresses an allowed environment for a game.
type GameEnvRecord struct {
	// Global env definition; unique across system
	Env         string `gorm:"primaryKey;size:50;not null"`
	Description string `gorm:"type:text"`
	Color       string `gorm:"size:16"`
}

func (GameEnvRecord) TableName() string { return "game_envs" }

// GameAgentRecord represents an agent registration for a game
type GameAgentRecord struct {
	gorm.Model
	GameID     string `gorm:"size:64;index;not null"`
	AgentID    string `gorm:"size:128;not null"`
	Env        string `gorm:"size:64;index;not null"`
	Status       string     `gorm:"size:32;default:active"` // active, inactive, error
	LastHeartbeat *time.Time
	Metadata     string     `gorm:"type:json"` // Additional agent metadata
}

// TableName returns the table name for GameAgentRecord model
func (GameAgentRecord) TableName() string {
	return "game_agent_records"
}

// Helpers to encode/decode GameRecord.Envs
func (g *GameRecord) GetEnvList() []string {
	var arr []string
	if len(g.Envs) == 0 {
		return arr
	}
	_ = json.Unmarshal(g.Envs, &arr)
	return arr
}
func (g *GameRecord) SetEnvList(envs []string) {
	b, _ := json.Marshal(envs)
	g.Envs = b
}

func AutoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&GameRecord{},
		&GameEnvRecord{},
		&GameAgentRecord{},
	)
}

// TableName returns the table name for GameRecord migration
func (GameRecord) TableName() string {
	return "game_records_migration"
}
