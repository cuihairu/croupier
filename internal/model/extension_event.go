package model

import (
	"gorm.io/gorm"
)

type ExtensionEvent struct {
	gorm.Model
	InstallationID uint   `gorm:"not null;index" json:"installationId"`
	EventType      string `gorm:"size:32;not null;index" json:"eventType"`
	Level          string `gorm:"size:16;not null;index" json:"level"`
	Message        string `gorm:"type:text;not null" json:"message"`
	PayloadJSON    JSON   `gorm:"type:json" json:"payloadJson"`
	CreatedBy      string `gorm:"size:128" json:"createdBy"`
}

func (ExtensionEvent) TableName() string {
	return "extension_events"
}
