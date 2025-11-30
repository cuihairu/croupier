package model

import "gorm.io/gorm"

// Feedback represents player feedback submission.
type Feedback struct {
	gorm.Model
	PlayerID string `gorm:"size:64;index"`
	Contact  string `gorm:"size:128"`
	Content  string `gorm:"type:text"`
	Category string `gorm:"size:64"`
	Priority string `gorm:"size:16"`
	Status   string `gorm:"size:16"`
	Rating   int    `gorm:"default:0"`
	Reply    string `gorm:"type:text"`
	Attach   string `gorm:"type:text"`
	GameID   string `gorm:"size:64;index"`
	Env      string `gorm:"size:64"`
}

func (Feedback) TableName() string {
	return "feedbacks"
}
