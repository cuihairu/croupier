package model

import (
	"encoding/json"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// Game 游戏结构体
type Game struct {
	gorm.Model
	Name        string         `gorm:"size:128;not null;index"`
	Icon        string         `gorm:"size:255"`
	Description string         `gorm:"type:text"`
	Enabled     bool           `gorm:"default:true;index"`
	AliasName   string         `gorm:"size:64;uniqueIndex"`
	Homepage    string         `gorm:"size:255"`
	Status      string         `gorm:"size:32;default:'dev';index"` // dev, test, running, online, offline, maintenance
	GameType    string         `gorm:"size:64;index"`
	GenreCode   string         `gorm:"size:64;index"`
	Config      string         `gorm:"type:text"` // 游戏配置 JSON
	Color       string         `gorm:"size:32"`
	Envs        datatypes.JSON `gorm:"type:json"` // 环境列表 JSON
}

// TableName 实现 GORM 的表名接口
func (Game) TableName() string {
	return "games"
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
