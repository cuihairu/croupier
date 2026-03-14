package model

import (
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// RateLimit represents throttling configurations.
type RateLimit struct {
	gorm.Model
	RateLimitID string `gorm:"size:64;uniqueIndex"`
	Name        string `gorm:"size:128"`
	Resource    string `gorm:"size:64"`
	Limit       int
	Window      int
	Action      string            `gorm:"size:32"`
	Rules       datatypes.JSONMap `gorm:"type:json"`
	Status      int               `gorm:"default:1"`
}

func (RateLimit) TableName() string {
	return "rate_limits"
}
