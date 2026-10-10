package model

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"time"

	"gorm.io/gorm"
)

// HashExternalToken 把明文令牌转成 sha256 十六进制（64 字符定长），存库与
// 校验共用。明文只在创建时返回一次。
func HashExternalToken(plain string) string {
	sum := sha256.Sum256([]byte(plain))
	return hex.EncodeToString(sum[:])
}

// ExternalTokenModel 提供 external_tokens 的数据访问（incident-reports §9.2）。
type ExternalTokenModel struct {
	db *gorm.DB
}

// NewExternalTokenModel 创建外部令牌模型。
func NewExternalTokenModel(db *gorm.DB) *ExternalTokenModel {
	return &ExternalTokenModel{db: db}
}

// Create 插入新令牌。
func (m *ExternalTokenModel) Create(ctx context.Context, row *ExternalToken) error {
	return m.db.WithContext(ctx).Create(row).Error
}

// FindByHash 按 sha256 哈希取令牌（Bearer 校验路径）。
func (m *ExternalTokenModel) FindByHash(ctx context.Context, hash string) (*ExternalToken, error) {
	var row ExternalToken
	err := m.db.WithContext(ctx).Where("token_hash = ?", hash).First(&row).Error
	if err != nil {
		return nil, err
	}
	return &row, nil
}

// ListTokensOptions 分页/过滤参数。
type ListTokensOptions struct {
	Page     int
	PageSize int
}

// List 分页返回全部外部令牌（不含 plaintext，仅元数据）。
func (m *ExternalTokenModel) List(ctx context.Context, opts ListTokensOptions) ([]ExternalToken, int64, error) {
	if opts.Page <= 0 {
		opts.Page = 1
	}
	if opts.PageSize <= 0 {
		opts.PageSize = 20
	}
	var total int64
	if err := m.db.WithContext(ctx).Model(&ExternalToken{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []ExternalToken
	offset := (opts.Page - 1) * opts.PageSize
	err := m.db.WithContext(ctx).Order("id DESC").Offset(offset).Limit(opts.PageSize).Find(&rows).Error
	return rows, total, err
}

// Update 更新令牌元数据（scope/enabled/name）。
func (m *ExternalTokenModel) Update(ctx context.Context, id uint, updates map[string]interface{}) error {
	return m.db.WithContext(ctx).Model(&ExternalToken{}).Where("id = ?", id).Updates(updates).Error
}

// Delete 吊销/删除令牌。
func (m *ExternalTokenModel) Delete(ctx context.Context, id uint) error {
	return m.db.WithContext(ctx).Delete(&ExternalToken{}, id).Error
}

// TouchUsed 记录最近使用时间（成功校验后的非阻塞更新）。
func (m *ExternalTokenModel) TouchUsed(ctx context.Context, id uint, at time.Time) error {
	return m.db.WithContext(ctx).Model(&ExternalToken{}).Where("id = ?", id).Update("last_used_at", at).Error
}

// ExternalTokenScope 解析 scope JSON 中的 categories 列表（null 或解析失败视为全量）。
func (t *ExternalToken) ScopeCategories() []string {
	if len(t.Scope) == 0 {
		return nil
	}
	var sc struct {
		Categories []string `json:"categories"`
	}
	if err := json.Unmarshal(t.Scope, &sc); err != nil {
		return nil
	}
	return sc.Categories
}
