package model

import (
	"context"
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"
)

// BackupModel provides CRUD helpers for backups.
type BackupModel struct {
	db *gorm.DB
}

// NewBackupModel creates a new helper.
func NewBackupModel(db *gorm.DB) *BackupModel {
	return &BackupModel{db: db}
}

// ListBackupsOptions controls list filtering. GameID/Env 非空时按 OPS-008
// 归属列过滤（游戏域控制台只看本 scope 的备份；平台级清单不带 scope 全量）。
type ListBackupsOptions struct {
	PaginationOptions
	Type   string
	GameID string
	Env    string
}

// Create inserts a new backup entry.
func (m *BackupModel) Create(ctx context.Context, backup *Backup) error {
	return m.db.WithContext(ctx).Create(backup).Error
}

// FindByID fetches a backup.
func (m *BackupModel) FindByID(ctx context.Context, id uint) (*Backup, error) {
	var backup Backup
	if err := m.db.WithContext(ctx).First(&backup, id).Error; err != nil {
		return nil, err
	}
	return &backup, nil
}

// FindByBackupID fetches a backup via its stable backup_id.
func (m *BackupModel) FindByBackupID(ctx context.Context, backupID string) (*Backup, error) {
	trimmed := strings.TrimSpace(backupID)
	if trimmed == "" {
		return nil, errors.New("backup id is required")
	}

	var backup Backup
	if err := m.db.WithContext(ctx).
		Where("backup_id = ?", trimmed).
		First(&backup).Error; err != nil {
		return nil, err
	}
	return &backup, nil
}

// UpdateByBackupID updates fields by backup_id.
func (m *BackupModel) UpdateByBackupID(ctx context.Context, backupID string, updates map[string]interface{}) error {
	return m.db.WithContext(ctx).
		Model(&Backup{}).
		Where("backup_id = ?", strings.TrimSpace(backupID)).
		Updates(updates).Error
}

// Delete removes a backup.
func (m *BackupModel) Delete(ctx context.Context, id uint) error {
	return m.db.WithContext(ctx).Delete(&Backup{}, id).Error
}

// DeleteByBackupID removes a backup using backup_id.
func (m *BackupModel) DeleteByBackupID(ctx context.Context, backupID string) error {
	trimmed := strings.TrimSpace(backupID)
	if trimmed == "" {
		return errors.New("backup id is required")
	}

	return m.db.WithContext(ctx).Where("backup_id = ?", trimmed).Delete(&Backup{}).Error
}

// FindActiveByIdempotencyKey 按幂等键取活跃备份（AUD-053）：pending/
// succeeded 算活跃（重试复用）；failed 不算（失败后同键重试应创建新备份）。
// 同键活跃行最多一条（幂等键唯一索引 + Create 侧冲突回落保证）。
func (m *BackupModel) FindActiveByIdempotencyKey(ctx context.Context, key string) (*Backup, error) {
	trimmed := strings.TrimSpace(key)
	if trimmed == "" {
		return nil, errors.New("idempotency key is required")
	}
	var backup Backup
	if err := m.db.WithContext(ctx).
		Where("idempotency_key = ? AND status IN ?", trimmed, []string{"pending", "succeeded"}).
		Order("created_at DESC").
		First(&backup).Error; err != nil {
		return nil, err
	}
	return &backup, nil
}

// ReleaseIdempotencyKey 把同键 failed 行的幂等键让位（AUD-053）：失败后
// 同键重试要建新行，唯一索引不得挡路——先释放旧行的键再插入。只动 failed
// 行；并发双释放无害（第二次是 no-op）。
func (m *BackupModel) ReleaseIdempotencyKey(ctx context.Context, key string) error {
	return m.db.WithContext(ctx).Model(&Backup{}).
		Where("idempotency_key = ? AND status = ?", strings.TrimSpace(key), "failed").
		Update("idempotency_key", nil).Error
}

// FindRecentDuplicateByNameType 找时间窗内同名同类型的活跃备份（AUD-053
// 无客户端幂等键时的双击兜底：连续两次提交同名 full 备份只创建一次）。
func (m *BackupModel) FindRecentDuplicateByNameType(ctx context.Context, name, backupType string, window time.Duration) (*Backup, error) {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return nil, errors.New("backup name is required")
	}
	var backup Backup
	if err := m.db.WithContext(ctx).
		Where("name = ? AND type = ? AND status IN ? AND created_at >= ?",
			trimmed, backupType, []string{"pending", "succeeded"}, time.Now().Add(-window)).
		Order("created_at DESC").
		First(&backup).Error; err != nil {
		return nil, err
	}
	return &backup, nil
}

// List returns paginated backups.
func (m *BackupModel) List(ctx context.Context, opts ListBackupsOptions) ([]Backup, int64, error) {
	opts.Normalize()

	var (
		backups []Backup
		total   int64
	)

	query := m.db.WithContext(ctx).Model(&Backup{})
	if opts.Type != "" {
		query = query.Where("type = ?", opts.Type)
	}
	if opts.GameID != "" {
		query = query.Where("game_id = ?", opts.GameID)
	}
	if opts.Env != "" {
		query = query.Where("env = ?", opts.Env)
	}
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := query.Order("created_at DESC").
		Offset(opts.Offset()).
		Limit(opts.PageSize).
		Find(&backups).Error; err != nil {
		return nil, 0, err
	}
	return backups, total, nil
}
