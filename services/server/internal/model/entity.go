package model

import (
	"encoding/json"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// Entity 实体结构体，用于动态内容管理
type Entity struct {
	gorm.Model
	Type       string         `gorm:"size:64;index;not null"` // 实体类型: player, item, quest 等
	Data       datatypes.JSON `gorm:"type:json"`              // JSON 数据
	ProviderID string         `gorm:"size:128;index"`         // 提供者 ID
	Status     int            `gorm:"default:1"`              // 0:禁用 1:启用
}

// TableName 实现 GORM 的表名接口
func (Entity) TableName() string {
	return "entities"
}

// GetData 解析 JSON 数据到指定结构
func (e *Entity) GetData(dest interface{}) error {
	if len(e.Data) == 0 {
		return nil
	}
	return json.Unmarshal(e.Data, dest)
}

// SetData 设置 JSON 数据
func (e *Entity) SetData(data interface{}) error {
	b, err := json.Marshal(data)
	if err != nil {
		return err
	}
	e.Data = b
	return nil
}
