package model

import (
	"time"

	"gorm.io/gorm"
)

// Backup stores backup metadata.
type Backup struct {
	gorm.Model
	BackupID     string `gorm:"size:64;uniqueIndex"`
	Name         string `gorm:"size:128"`
	Size         int64
	Type         string `gorm:"size:32"`
	Status       string `gorm:"size:32"`
	Location     string `gorm:"size:255"`
	Checksum     string `gorm:"size:64"`
	ErrorMessage string `gorm:"type:text"`
	CompletedAt  *time.Time

	// OPS-008: 游戏库备份归属（经 capture 所见数据源登记）。平台级备份
	// （meta DB）三列空；SourceID 指向 db_sources.id（源删除后 SourceName
	// 兜底显示，列表不因源被删而丢失可读性）。
	GameID     string `gorm:"size:64;index:idx_backup_scope,priority:1"`
	Env        string `gorm:"size:64;index:idx_backup_scope,priority:2"`
	SourceID   uint   `gorm:"index:idx_backup_scope,priority:3"`
	SourceName string `gorm:"size:128"`
}

func (Backup) TableName() string {
	return "backups"
}
