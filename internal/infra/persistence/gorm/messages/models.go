package messagesgorm

import (
    "time"
    "gorm.io/gorm"
)

// MessageRecord represents an internal inbox message for a user.
// Minimal MVP: per-user direct messages with read timestamp.
type MessageRecord struct {
    gorm.Model
    ToUserID   uint      `gorm:"index;not null"`
    FromUserID *uint     `gorm:"index"`
    Title      string    `gorm:"size:200"`
    Content    string    `gorm:"type:text"`
    Type       string    `gorm:"size:32"` // info|warning|task
    ReadAt     *time.Time
}

func AutoMigrate(db *gorm.DB) error {
    if err := db.AutoMigrate(&MessageRecord{}); err != nil { return err }
    if err := AutoMigrateBroadcast(db); err != nil { return err }
    return nil
}
