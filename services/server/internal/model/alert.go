package model

import (
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// Alert captures runtime system alerts for ops center.
type Alert struct {
	gorm.Model
	AlertID   string            `gorm:"size:64;uniqueIndex"`
	Type      string            `gorm:"size:64"`
	Level     string            `gorm:"size:32"`
	Message   string            `gorm:"type:text"`
	Source    string            `gorm:"size:64"`
	Status    string            `gorm:"size:32;index"`
	Details   datatypes.JSONMap `gorm:"type:json"`
	Metadata  datatypes.JSONMap `gorm:"type:json"`
	CreatedBy string            `gorm:"size:64"`
}

// AlertSilence stores silence windows for alerts.
type AlertSilence struct {
	gorm.Model
	AlertID        uint      `gorm:"index"`
	Reason         string    `gorm:"size:255"`
	DurationMinute int       `gorm:"default:0"`
	ExpiresAt      time.Time `gorm:"index"`
	CreatedBy      string    `gorm:"size:64"`
}

func (Alert) TableName() string {
	return "alerts"
}

func (AlertSilence) TableName() string {
	return "alert_silences"
}
