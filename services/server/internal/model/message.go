package model

import (
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// Message represents system/user notifications.
type Message struct {
	gorm.Model
	To      string         `gorm:"size:255;not null;index"`
	Type    string         `gorm:"size:64;not null;index"`
	Title   string         `gorm:"size:255"`
	Content string         `gorm:"type:text"`
	Data    datatypes.JSON `gorm:"type:json"`
	Status  string         `gorm:"size:32;default:'unread';index"`
	ReadAt  *time.Time     `gorm:"index"`
}

// TableName implements gorm's tabler interface.
func (Message) TableName() string {
	return "messages"
}
