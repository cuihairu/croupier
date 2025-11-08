package support

import (
    "time"
    "gorm.io/gorm"
)

// Ticket represents a customer support ticket.
type Ticket struct {
    gorm.Model
    Title     string    `gorm:"size:255"`
    Content   string    `gorm:"type:text"`
    Category  string    `gorm:"size:64"`
    Priority  string    `gorm:"size:16"`  // low|normal|high|urgent
    Status    string    `gorm:"size:16"`  // open|in_progress|resolved|closed
    Assignee  string    `gorm:"size:64"`  // username or id
    Tags      string    `gorm:"size:255"` // comma-separated
    PlayerID  string    `gorm:"size:64"`
    Contact   string    `gorm:"size:128"`
    GameID    string    `gorm:"size:64"`
    Env       string    `gorm:"size:64"`
    Source    string    `gorm:"size:64"`  // web|sdk|email|other
    DueAt     *time.Time
}

// TicketComment represents a comment on a ticket.
type TicketComment struct {
    gorm.Model
    TicketID uint   `gorm:"index"`
    Author   string `gorm:"size:64"` // username
    Content  string `gorm:"type:text"`
    Attach   string `gorm:"type:text"` // optional JSON of attachments
}

// FAQ represents a knowledge base entry.
type FAQ struct {
    gorm.Model
    Question string `gorm:"type:text"`
    Answer   string `gorm:"type:text"`
    Category string `gorm:"size:64"`
    Tags     string `gorm:"size:255"`
    Visible  bool   `gorm:"default:true"`
    Sort     int    `gorm:"default:0"`
}

// Feedback represents player feedback submission.
type Feedback struct {
    gorm.Model
    PlayerID string `gorm:"size:64"`
    Contact  string `gorm:"size:128"`
    Content  string `gorm:"type:text"`
    Category string `gorm:"size:64"`
    Priority string `gorm:"size:16"` // low|normal|high
    Status   string `gorm:"size:16"` // new|triaged|closed
    Attach   string `gorm:"type:text"` // optional JSON of attachments
    GameID   string `gorm:"size:64"`
    Env      string `gorm:"size:64"`
}

func AutoMigrate(db *gorm.DB) error {
    return db.AutoMigrate(&Ticket{}, &TicketComment{}, &FAQ{}, &Feedback{})
}
