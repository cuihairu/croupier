package model

import (
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// BehaviorEvent stores raw behavior events for analytics.
type BehaviorEvent struct {
	gorm.Model
	GameID     string            `gorm:"size:64;index"`
	Env        string            `gorm:"size:32;index"`
	EventType  string            `gorm:"size:64;index"`
	UserID     string            `gorm:"size:64;index"`
	Data       datatypes.JSONMap `gorm:"type:json"`
	OccurredAt time.Time         `gorm:"index"`
}

// FeatureAdoption stores feature adoption metrics snapshots.
type FeatureAdoption struct {
	gorm.Model
	GameID       string  `gorm:"size:64;index"`
	Env          string  `gorm:"size:32;index"`
	Feature      string  `gorm:"size:64;index:idx_feature_unique,unique"`
	Users        int     `gorm:"default:0"`
	AdoptionRate float64 `gorm:"default:0"`
	Frequency    float64 `gorm:"default:0"`
	WindowStart  time.Time
	WindowEnd    time.Time
}

func (FeatureAdoption) TableName() string {
	return "feature_adoptions"
}
