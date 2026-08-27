package model

import (
	"time"

	"github.com/cuihairu/croupier/internal/dbenum"
	"gorm.io/gorm"
)

// Ticket represents the primary ticketing model for /tickets API.
type Ticket struct {
	gorm.Model
	Title    string              `gorm:"size:255"`
	Content  string              `gorm:"type:text"`
	Category string              `gorm:"size:64;index:idx_ticket_category_status"`
	Priority string              `gorm:"size:16;index"`
	Status   dbenum.TicketStatus `gorm:"index:idx_ticket_status;index:idx_ticket_category_status"`
	Assignee string              `gorm:"size:64;index"`
	Tags     JSON                `gorm:"type:json"`
	PlayerID string              `gorm:"size:64;index"`
	Contact  string              `gorm:"size:128"`
	GameID   string              `gorm:"size:64;index:idx_ticket_game_env,priority:1"`
	Env      string              `gorm:"size:64;index:idx_ticket_game_env,priority:2"`
	Source   string              `gorm:"size:32;index"`
	DueAt    *time.Time          `gorm:"index"`

	// Player context attached at creation (game-support P1; see
	// docs/research/game-support-systems.md). Structured columns cover the
	// common GM triage dimensions; anything else goes into Extra.
	ServerID    string `gorm:"size:64;index"`
	PlayerLevel int    `gorm:"default:0"`
	DeviceOS    string `gorm:"size:32"`
	DeviceModel string `gorm:"size:128"`
	Language    string `gorm:"size:16"`
	Extra       JSON   `gorm:"type:json"`

	// CSAT: satisfaction rating submitted when the ticket closes (1-5,
	// 0 = unrated). game-support P2.
	Rating  int    `gorm:"default:0;index"`
	RatedBy string `gorm:"size:64"`
	RatedAt *time.Time
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
