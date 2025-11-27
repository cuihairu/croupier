package model

import (
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// FAQ represents a knowledge base entry.
type FAQ struct {
	gorm.Model
	Question string         `gorm:"type:text"`
	Answer   string         `gorm:"type:text"`
	Category string         `gorm:"size:64;index"`
	Tags     datatypes.JSON `gorm:"type:json"`
	Visible  bool           `gorm:"default:true"`
	Sort     int            `gorm:"default:0"`
	Views    int            `gorm:"default:0"`
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
