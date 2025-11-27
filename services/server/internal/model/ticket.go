package model

import (
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// Ticket represents the primary ticketing model for /tickets API.
type Ticket struct {
	gorm.Model
	Title    string         `gorm:"size:255"`
	Content  string         `gorm:"type:text"`
	Category string         `gorm:"size:64;index"`
	Priority string         `gorm:"size:16;index"`
	Status   string         `gorm:"size:32;index"`
	Assignee string         `gorm:"size:64"`
	Tags     datatypes.JSON `gorm:"type:json"`
	PlayerID string         `gorm:"size:64"`
	Contact  string         `gorm:"size:128"`
	GameID   string         `gorm:"size:64;index"`
	Env      string         `gorm:"size:64"`
	Source   string         `gorm:"size:32"`
	DueAt    *time.Time
}

// TicketComment stores comments under /tickets module.
type TicketComment struct {
	gorm.Model
	TicketID uint   `gorm:"index"`
	Author   string `gorm:"size:64"`
	Content  string `gorm:"type:text"`
}

func (Ticket) TableName() string {
	return "tickets"
}

func (TicketComment) TableName() string {
	return "ticket_comments"
}
