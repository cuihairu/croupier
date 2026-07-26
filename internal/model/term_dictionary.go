package model

import (
	"time"

	"gorm.io/gorm"
)

// TermDictionary stores normalized game terminology entries used for aliasing and display hints.
type TermDictionary struct {
	ID        uint           `gorm:"primaryKey"`
	Domain    string         `gorm:"size:32;not null;index:idx_term_domain_key,priority:1;uniqueIndex:uidx_term_domain_alias,priority:1"`
	TermKey   string         `gorm:"size:64;not null;index:idx_term_domain_key,priority:2"`
	Alias     string         `gorm:"size:128;not null;uniqueIndex:uidx_term_domain_alias,priority:2"`
	DisplayZh string         `gorm:"size:128"`
	DisplayEn string         `gorm:"size:128"`
	SortOrder int            `gorm:"default:100"`
	CreatedAt time.Time      `gorm:"autoCreateTime"`
	UpdatedAt time.Time      `gorm:"autoUpdateTime"`
	DeletedAt gorm.DeletedAt `gorm:"index"`
}

func (TermDictionary) TableName() string {
	return "term_dictionary"
}
