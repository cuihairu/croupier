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

	// AUD-053 幂等键：客户端重试/双击携带同一 Idempotency-Key 时，
	// pending/succeeded 的同键备份只创建一次（后到方复用已存在行）。
	// 指针列——不带键的行存 NULL，多行 NULL 在 pg/mysql/sqlite 均不冲突；
	// 唯一索引兜并发双写的竞争窗口（冲突方回落到已存在行）。
	IdempotencyKey *string `gorm:"size:128;uniqueIndex"`
}

func (Backup) TableName() string {
	return "backups"
}
