package model

import (
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type ExtensionEvent struct {
	gorm.Model
	InstallationID uint           `gorm:"not null;index" json:"installation_id"`
	EventType      string         `gorm:"size:32;not null;index" json:"event_type"`
	Level          string         `gorm:"size:16;not null;index" json:"level"`
	Message        string         `gorm:"type:text;not null" json:"message"`
	PayloadJSON    datatypes.JSON `gorm:"type:json" json:"payload_json"`
	CreatedBy      string         `gorm:"size:128" json:"created_by"`
}

func (ExtensionEvent) TableName() string {
	return "extension_events"
}
