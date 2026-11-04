package model

import (
	"context"
	"errors"
	"strings"

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
