package model

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
)

// AdminOTPRecoveryCode is a one-time recovery code issued together with TOTP
// enrollment.
//
// 设计要点：
//   - 只存哈希不存明文：绑定流程把明文展示一次，用户丢失验证器后可用任一码登录；
//   - 每码一行而非一行存 JSON 数组：恢复码天然是「一次性」的，需要逐条标记
//     used_at 才能防止复用，也便于统计剩余数量；
//   - 禁用两步验证时随 admins.otp_secret 一并清空。
type AdminOTPRecoveryCode struct {
	ID        uint           `gorm:"primarykey"`
	AdminID   uint           `gorm:"not null;index"`
	CodeHash  string         `gorm:"size:64;not null;index"`
	UsedAt    *time.Time     `gorm:"index"`
	CreatedAt time.Time      `gorm:"autoCreateTime"`
	UpdatedAt time.Time      `gorm:"autoUpdateTime"`
	DeletedAt gorm.DeletedAt `gorm:"index"`
}

func (AdminOTPRecoveryCode) TableName() string { return "admin_otp_recovery_codes" }

// IsUsable reports whether the code has not been spent yet.
func (c *AdminOTPRecoveryCode) IsUsable() bool { return c.UsedAt == nil }

// AdminOTPRecoveryCodeModel persists recovery codes.
type AdminOTPRecoveryCodeModel struct {
	db *gorm.DB
}

func NewAdminOTPRecoveryCodeModel(db *gorm.DB) *AdminOTPRecoveryCodeModel {
	return &AdminOTPRecoveryCodeModel{db: db}
}

// Replace deletes every recovery code of the admin and inserts the new set.
// Used at enrollment and at re-enrollment; the old set must not survive, or
// codes the user thought invalid would still work.
func (m *AdminOTPRecoveryCodeModel) Replace(ctx context.Context, adminID uint, hashes []string) error {
	if m == nil || m.db == nil {
		return errors.New("recovery code model 未初始化")
	}
	return m.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Unscoped().Where("admin_id = ?", adminID).Delete(&AdminOTPRecoveryCode{}).Error; err != nil {
			return err
		}
		if len(hashes) == 0 {
			return nil
		}
		rows := make([]AdminOTPRecoveryCode, 0, len(hashes))
		for _, h := range hashes {
			rows = append(rows, AdminOTPRecoveryCode{AdminID: adminID, CodeHash: h})
		}
		return tx.Create(&rows).Error
	})
}

// ListUsable returns the admin's unspent recovery codes.
func (m *AdminOTPRecoveryCodeModel) ListUsable(ctx context.Context, adminID uint) ([]AdminOTPRecoveryCode, error) {
	if m == nil || m.db == nil {
		return nil, errors.New("recovery code model 未初始化")
	}
	var out []AdminOTPRecoveryCode
	err := m.db.WithContext(ctx).
		Where("admin_id = ? AND used_at IS NULL", adminID).
		Order("id ASC").
		Find(&out).Error
	return out, err
}

// Consume marks the matching unused code as spent and reports whether one was
// actually consumed.
//
// 单条 UPDATE ... WHERE used_at IS NULL 既是「找到」也是「标记」，天然防并发
// 复用：两个并发请求只有一个能让受影响行数为 1。
func (m *AdminOTPRecoveryCodeModel) Consume(ctx context.Context, adminID uint, hash string) (bool, error) {
	if m == nil || m.db == nil {
		return false, errors.New("recovery code model 未初始化")
	}
	now := time.Now()
	res := m.db.WithContext(ctx).
		Model(&AdminOTPRecoveryCode{}).
		Where("admin_id = ? AND code_hash = ? AND used_at IS NULL", adminID, hash).
		Update("used_at", now)
	if res.Error != nil {
		return false, res.Error
	}
	return res.RowsAffected > 0, nil
}

// DeleteAll clears every recovery code of the admin (used when MFA is disabled).
func (m *AdminOTPRecoveryCodeModel) DeleteAll(ctx context.Context, adminID uint) error {
	if m == nil || m.db == nil {
		return errors.New("recovery code model 未初始化")
	}
	return m.db.WithContext(ctx).
		Unscoped().
		Where("admin_id = ?", adminID).
		Delete(&AdminOTPRecoveryCode{}).Error
}

// CountUsable returns how many recovery codes remain.
func (m *AdminOTPRecoveryCodeModel) CountUsable(ctx context.Context, adminID uint) (int64, error) {
	if m == nil || m.db == nil {
		return 0, errors.New("recovery code model 未初始化")
	}
	var n int64
	err := m.db.WithContext(ctx).
		Model(&AdminOTPRecoveryCode{}).
		Where("admin_id = ? AND used_at IS NULL", adminID).
		Count(&n).Error
	return n, err
}
