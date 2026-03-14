package model

import (
	"time"

	"gorm.io/gorm"
)

// SupportTicket captures support ticket data.
type SupportTicket struct {
	gorm.Model
	Title    string `gorm:"size:255"`
	Content  string `gorm:"type:text"`
	Category string `gorm:"size:64"`
	Priority string `gorm:"size:16"`
	Status   string `gorm:"size:32;index"`
	Assignee string `gorm:"size:64"`
	Tags     string `gorm:"size:255"`
	PlayerID string `gorm:"size:64"`
	Contact  string `gorm:"size:128"`
	GameID   string `gorm:"size:64;index"`
	Env      string `gorm:"size:64"`
	Source   string `gorm:"size:32"`
	DueAt    *time.Time
}

// SupportComment stores comments for support tickets.
type SupportComment struct {
	gorm.Model
	TicketID uint   `gorm:"index"`
	Author   string `gorm:"size:64"`
	Content  string `gorm:"type:text"`
	Attach   string `gorm:"type:text"`
}

// SupportFAQ mirrors FAQ entries in the support context.
type SupportFAQ struct {
	gorm.Model
	Question string `gorm:"type:text"`
	Answer   string `gorm:"type:text"`
	Category string `gorm:"size:64"`
	Tags     string `gorm:"size:255"`
	Visible  bool   `gorm:"default:true"`
	Sort     int    `gorm:"default:0"`
}

// SupportFeedback stores support feedback.
type SupportFeedback struct {
	gorm.Model
	PlayerID string `gorm:"size:64"`
	Contact  string `gorm:"size:128"`
	Content  string `gorm:"type:text"`
	Category string `gorm:"size:64"`
	Priority string `gorm:"size:16"`
	Status   string `gorm:"size:32"`
	Attach   string `gorm:"type:text"`
	GameID   string `gorm:"size:64"`
	Env      string `gorm:"size:64"`
}

func (SupportTicket) TableName() string {
	return "support_tickets"
}

func (SupportComment) TableName() string {
	return "support_ticket_comments"
}

func (SupportFAQ) TableName() string {
	return "support_faqs"
}

func (SupportFeedback) TableName() string {
	return "support_feedback"
}
