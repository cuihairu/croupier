package model

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"time"

	"gorm.io/gorm"
)

// HashRefreshToken 把明文刷新令牌转成 sha256 十六进制（64 字符定长），存库
// 与兑换校验共用。明文只在签发/轮换响应里出现一次，落库即哈希。
func HashRefreshToken(plain string) string {
	sum := sha256.Sum256([]byte(plain))
	return hex.EncodeToString(sum[:])
}

// AdminRefreshToken 是刷新令牌轮换的持久状态（AUD-052）：一行对应一个已
// 签发的 refresh JWT。UsedAt 非空表示本令牌已被兑换过一次——刷新令牌是
// 一次性的，兑换即轮换出新对；已兑换/已吊销的令牌再次出现即为重放，触发
// 该账号名下全部刷新令牌吊销（reuse detection 需要服务端持久状态，纯 JWT
// 无状态方案做不到）。
type AdminRefreshToken struct {
	gorm.Model
	// AdminID 归属账号（重放时按账号整族吊销）。
	AdminID uint `gorm:"index"`
	// TokenHash = sha256 hex(refresh JWT 原文)，唯一索引支撑兑换/重放判定。
	TokenHash string `gorm:"size:64;uniqueIndex"`
	// RotatedFromID 是兑换链上游令牌 ID（0 = 登录直发），审计回溯用。
	RotatedFromID uint `gorm:"index"`
	ExpiresAt     time.Time
	UsedAt        *time.Time
	Revoked       bool
	// 签发上下文（审计回溯；不参与兑换判定）。
	ClientIP  string `gorm:"size:64"`
	UserAgent string `gorm:"size:255"`
}

func (AdminRefreshToken) TableName() string { return "admin_refresh_tokens" }

// AdminRefreshTokenModel 提供 admin_refresh_tokens 的数据访问。
type AdminRefreshTokenModel struct {
	db *gorm.DB
}

// NewAdminRefreshTokenModel 创建刷新令牌模型。
func NewAdminRefreshTokenModel(db *gorm.DB) *AdminRefreshTokenModel {
	return &AdminRefreshTokenModel{db: db}
}

// Create 插入新刷新令牌行。
func (m *AdminRefreshTokenModel) Create(ctx context.Context, row *AdminRefreshToken) error {
	return m.db.WithContext(ctx).Create(row).Error
}

// FindByHash 按哈希取令牌行（兑换/重放判定路径）。
func (m *AdminRefreshTokenModel) FindByHash(ctx context.Context, hash string) (*AdminRefreshToken, error) {
	var row AdminRefreshToken
	err := m.db.WithContext(ctx).Where("token_hash = ?", hash).First(&row).Error
	if err != nil {
		return nil, err
	}
	return &row, nil
}

// MarkUsed 标记兑换（轮换）：一次写入，令牌自此作废，重放即被识别。
func (m *AdminRefreshTokenModel) MarkUsed(ctx context.Context, id uint, usedAt time.Time) error {
	return m.db.WithContext(ctx).Model(&AdminRefreshToken{}).Where("id = ?", id).
		Update("used_at", usedAt).Error
}

// RevokeAllForAdmin 吊销该账号名下全部未吊销刷新令牌（重放响应/登出全端）。
func (m *AdminRefreshTokenModel) RevokeAllForAdmin(ctx context.Context, adminID uint) error {
	return m.db.WithContext(ctx).Model(&AdminRefreshToken{}).
		Where("admin_id = ? AND revoked = ?", adminID, false).
		Update("revoked", true).Error
}
