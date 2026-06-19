package model

import (
	"encoding/json"
	"strings"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// Game 游戏结构体
type Game struct {
	gorm.Model
	// GameID is the stable business identifier used for cross-game routing
	// (e.g. "demo", "rpg"). It is the canonical key referenced by
	// Player.GameID, Function.GameID and the database-per-game router.
	GameID      string `gorm:"size:64;uniqueIndex;not null" json:"gameId"`
	Name        string `gorm:"size:128;not null;index" json:"name"`
	Icon        string `gorm:"size:255" json:"icon"`
	Description string `gorm:"type:text" json:"description"`
	Enabled     bool   `gorm:"default:true;index" json:"enabled"`
	AliasName   string `gorm:"size:64;uniqueIndex" json:"aliasName"`
	Homepage    string `gorm:"size:255" json:"homepage"`
	Status      string `gorm:"size:32;default:'dev';index" json:"status"` // dev, test, running, online, offline, maintenance
	GameType    string `gorm:"size:64;index" json:"gameType"`
	GenreCode   string `gorm:"size:64;index" json:"genreCode"`
	Config      string `gorm:"type:text" json:"config"` // 游戏配置 JSON
	Color       string `gorm:"size:32" json:"color"`
	// Envs is kept for backward-compatible UI metadata. The authoritative
	// per-env routing data (including database_name) lives in GameEnvBinding.
	Envs datatypes.JSON `gorm:"type:json" json:"envs"`
}

// TableName 实现 GORM 的表名接口
func (Game) TableName() string {
	return "games"
}

// BeforeCreate auto-fills GameID from Name when left empty, so callers that
// only set Name (e.g. legacy tests) still get a valid unique business key.
func (g *Game) BeforeCreate(tx *gorm.DB) error {
	if strings.TrimSpace(g.GameID) == "" {
		g.GameID = deriveGameIDFromName(g.Name)
	}
	return nil
}

// deriveGameIDFromName produces a safe lowercase identifier from a display
// name: spaces/underscores collapse, non-alphanumeric characters are removed.
func deriveGameIDFromName(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	if name == "" {
		return "game"
	}
	var b strings.Builder
	prev := false
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			prev = false
		case r == '_' || r == '-' || r == ' ':
			if !prev && b.Len() > 0 {
				b.WriteRune('_')
				prev = true
			}
		}
	}
	result := strings.TrimSuffix(b.String(), "_")
	if result == "" {
		return "game"
	}
	return result
}

// GameEnv 游戏环境项
type GameEnv struct {
	Env         string `json:"env"`
	Description string `json:"description,omitempty"`
	Color       string `json:"color,omitempty"`
}

// GetEnvs 获取游戏环境列表
func (g *Game) GetEnvs() ([]GameEnv, error) {
	var envs []GameEnv
	if len(g.Envs) == 0 {
		return envs, nil
	}
	err := json.Unmarshal(g.Envs, &envs)
	return envs, err
}

// SetEnvs 设置游戏环境列表
func (g *Game) SetEnvs(envs []GameEnv) error {
	data, err := json.Marshal(envs)
	if err != nil {
		return err
	}
	g.Envs = data
	return nil
}

// GetConfig 获取游戏配置
func (g *Game) GetConfig(dest interface{}) error {
	if g.Config == "" {
		return nil
	}
	return json.Unmarshal([]byte(g.Config), dest)
}

// SetConfig 设置游戏配置
func (g *Game) SetConfig(config interface{}) error {
	data, err := json.Marshal(config)
	if err != nil {
		return err
	}
	g.Config = string(data)
	return nil
}

// GameEnvBinding is the authoritative per-game environment record stored in
// the meta database's game_envs table. Each (GameID, Env) pair maps to a
// physical database name used by the database-per-game router.
type GameEnvBinding struct {
	gorm.Model
	GameID       string `gorm:"size:64;index:idx_game_envs_game_env,unique;not null" json:"gameId"`
	Env          string `gorm:"size:64;index:idx_game_envs_game_env,unique;not null" json:"env"`
	DatabaseName string `gorm:"size:128;not null" json:"databaseName"`
	Description  string `gorm:"type:text" json:"description,omitempty"`
	Color        string `gorm:"size:16" json:"color,omitempty"`
}

// TableName implements gorm.Tabler.
func (GameEnvBinding) TableName() string { return "game_envs" }
