package model

import (
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// Message represents system/user notifications.
type Message struct {
	gorm.Model
	To      string         `gorm:"column:recipient;size:255;not null;index:idx_messages_to_status,priority:1;index:idx_messages_to_created"`
	Type    string         `gorm:"size:64;not null;index"`
	Title   string         `gorm:"size:255"`
	Content string         `gorm:"type:text"`
	Data    datatypes.JSON `gorm:"type:json"`
	Status  string         `gorm:"size:32;default:'unread';index:idx_messages_to_status,priority:2;index:idx_messages_status_created"`
	ReadAt  *time.Time     `gorm:"index"`
}

// TableName implements gorm's tabler interface.
func (Message) TableName() string {
	return "messages"
}
