package model

import (
	"gorm.io/gorm"
)

// FAQ represents a knowledge base entry.
// FAQ is the player-facing knowledge base entry (Q&A). See
// docs/research/game-support-systems.md for the AI+human friendly design.
type FAQ struct {
	gorm.Model
	Question string `gorm:"type:text"`
	Answer   string `gorm:"type:text"`
	Category string `gorm:"size:64;index"`
	Tags     JSON   `gorm:"type:json"`
	Visible  bool   `gorm:"default:true"`
	Sort     int    `gorm:"default:0"`
	Views    int    `gorm:"default:0"`
	// Slug is the stable public reference used by deep links and AI
	// citations. Empty means the entry predates slugs. Uniqueness among
	// non-empty slugs is enforced by the service layer (a DB unique index
	// would collide on the many empty strings of pre-slug rows).
	Slug string `gorm:"size:128;index"`
	// Summary is a short AI-oriented digest of the answer (RAG chunk title).
	Summary string `gorm:"size:512"`
	// Vote counters drive content governance (helpful ratio feeds the
	// "needs improvement" queue).
	HelpfulCount   int `gorm:"default:0"`
	UnhelpfulCount int `gorm:"default:0"`
}

// FAQCategory stores FAQ category metadata.
type FAQCategory struct {
	gorm.Model
	Name        string `gorm:"size:64;uniqueIndex"`
	Description string `gorm:"size:255"`
	Visible     bool   `gorm:"default:true"`
	Sort        int    `gorm:"default:0"`
}

func (FAQ) TableName() string {
	return "faqs"
}

func (FAQCategory) TableName() string {
	return "faq_categories"
}
