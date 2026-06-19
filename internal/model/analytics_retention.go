package model

import (
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// RetentionCohort stores cohort retention metrics.
type RetentionCohort struct {
	gorm.Model
	GameID      string         `gorm:"size:64;index"`
	Env         string         `gorm:"size:32;index"`
	ServerID    string         `gorm:"size:64;index"` // MMORPG multi-server support
	Cohort      string         `gorm:"size:32;index"`
	Users       int            `gorm:"default:0"`
	Retention   datatypes.JSON `gorm:"type:json"`
	WindowStart time.Time
	WindowEnd   time.Time
}

func (RetentionCohort) TableName() string {
	return "retention_cohorts"
}
