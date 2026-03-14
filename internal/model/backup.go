package model

import (
	"gorm.io/gorm"
)

// Backup stores backup metadata.
type Backup struct {
	gorm.Model
	BackupID string `gorm:"size:64;uniqueIndex"`
	Name     string `gorm:"size:128"`
	Size     int64
	Type     string `gorm:"size:32"`
	Status   string `gorm:"size:32"`
	Location string `gorm:"size:255"`
	Checksum string `gorm:"size:64"`
}

func (Backup) TableName() string {
	return "backups"
}
