package model

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
)

// EmailVerification 承载注册邮箱验证令牌（OPEN-ISSUES #51c 第二批）。
//
// 设计要点：
//   - 只存 SHA-256 哈希不存明文：令牌经邮件链接外发，库泄时不可回放；
//   - 一次性语义靠 used_at 承载（与 admin_otp_recovery_codes 同款），
//     验证回调单条 UPDATE ... WHERE used_at IS NULL 天然防并发复用；
//   - 重发即作废：发新令牌前把该账号未用令牌全部标记 used，旧链接即刻失效。
type EmailVerification struct {
	ID        uint   `gorm:"primarykey"`
	AdminID   uint   `gorm:"not null;index"`
	TokenHash string `gorm:"size:64;not null;uniqueIndex"`
	// Purpose 预留用途区分（当前仅 register）；同表后续承载改邮箱/找回密码
	// 时避免再加列迁移。
	Purpose   string     `gorm:"size:32;not null;default:register;index"`
	ExpiresAt time.Time  `gorm:"index"`
	UsedAt    *time.Time `gorm:"index"`
	CreatedAt time.Time  `gorm:"autoCreateTime"`
}

func (EmailVerification) TableName() string { return "email_verifications" }

// EmailVerificationModel persists email verification tokens.
type EmailVerificationModel struct {
	db *gorm.DB
}

func NewEmailVerificationModel(db *gorm.DB) *EmailVerificationModel {
	return &EmailVerificationModel{db: db}
}

func (m *EmailVerificationModel) ensure() error {
	if m == nil || m.db == nil {
		return errors.New("email verification model 未初始化")
	}
	return nil
}

// CreateInvalidatingActive 在一个事务里作废该账号同用途的未用令牌并写入新令牌。
// 重发即作废：同一时刻每个账号至多一个可用令牌，旧邮件链接点击即报失效。
func (m *EmailVerificationModel) CreateInvalidatingActive(ctx context.Context, row *EmailVerification) error {
	if err := m.ensure(); err != nil {
		return err
	}
	return m.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		now := time.Now()
		if err := tx.Model(&EmailVerification{}).
			Where("admin_id = ? AND purpose = ? AND used_at IS NULL", row.AdminID, row.Purpose).
			Update("used_at", now).Error; err != nil {
			return err
		}
		return tx.Create(row).Error
	})
}

// FindValidByTokenHash 返回该哈希对应的未使用且未过期令牌；找不到（含已用/
// 已过期/不存在）统一返回 nil, nil——对外不区分原因，避免令牌有效性探测。
func (m *EmailVerificationModel) FindValidByTokenHash(ctx context.Context, tokenHash string) (*EmailVerification, error) {
	if err := m.ensure(); err != nil {
		return nil, err
	}
	var row EmailVerification
	err := m.db.WithContext(ctx).
		Where("token_hash = ? AND used_at IS NULL AND expires_at > ?", tokenHash, time.Now()).
		First(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &row, nil
}

// Consume 在一个事务里把令牌标记已用并将对应账号 email_verified 置真。
// 单条 UPDATE ... WHERE used_at IS NULL 防并发复用；返回 false 表示令牌
// 已被并发消费。
func (m *EmailVerificationModel) Consume(ctx context.Context, verification *EmailVerification) (bool, error) {
	if err := m.ensure(); err != nil {
		return false, err
	}
	now := time.Now()
	var consumed bool
	err := m.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		res := tx.Model(&EmailVerification{}).
			Where("id = ? AND used_at IS NULL", verification.ID).
			Update("used_at", now)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return nil
		}
		consumed = true
		return tx.Model(&Admin{}).Where("id = ?", verification.AdminID).
			Update("email_verified", true).Error
	})
	if err != nil {
		return false, err
	}
	return consumed, nil
}

// LatestCreatedAtFor 返回该账号同用途最近一次发令牌的时间（零值表示从未发过）。
// 重发频控用：间隔内的重发请求直接拒绝，防止公开端点被刷信。
func (m *EmailVerificationModel) LatestCreatedAtFor(ctx context.Context, adminID uint, purpose string) (time.Time, error) {
	if err := m.ensure(); err != nil {
		return time.Time{}, err
	}
	var row EmailVerification
	err := m.db.WithContext(ctx).
		Where("admin_id = ? AND purpose = ?", adminID, purpose).
		Order("id DESC").First(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return time.Time{}, nil
		}
		return time.Time{}, err
	}
	return row.CreatedAt, nil
}
