package model

import "gorm.io/gorm"

// Feedback represents player feedback submission.
type Feedback struct {
	gorm.Model
	PlayerID string `gorm:"size:64;index"`
	Contact  string `gorm:"size:128"`
	Content  string `gorm:"type:text"`
	Category string `gorm:"size:64;index"`
	Priority string `gorm:"size:16;index"`
	Status   string `gorm:"size:16;index"`
	Rating   int    `gorm:"default:0;index"`
	Reply    string `gorm:"type:text"`
	Attach   string `gorm:"type:text"`
	GameID   string `gorm:"size:64;index:idx_feedback_game_env,priority:1"`
	Env      string `gorm:"size:64;index:idx_feedback_game_env,priority:2"`
}

func (Feedback) TableName() string {
	return "feedbacks"
}
