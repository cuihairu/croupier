package model

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"
)

// EntityModel 提供实体数据访问方法
type EntityModel struct {
	db *gorm.DB
}

// NewEntityModel 创建实体模型实例
func NewEntityModel(db *gorm.DB) *EntityModel {
	return &EntityModel{db: db}
}

// ListEntitiesOptions 控制实体列表查询的分页和过滤选项
type ListEntitiesOptions struct {
	Page       int
	PageSize   int
	Type       string
	ProviderID string
	Status     *int
}

// Create 插入新实体
func (m *EntityModel) Create(ctx context.Context, entity *Entity) error {
	return m.db.WithContext(ctx).Create(entity).Error
}

// FindOne 根据 ID 获取实体
func (m *EntityModel) FindOne(ctx context.Context, id uint) (*Entity, error) {
	var entity Entity
	if err := m.db.WithContext(ctx).First(&entity, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("entity not found")
		}
		return nil, err
	}
	return &entity, nil
}

// Update 更新实体
func (m *EntityModel) Update(ctx context.Context, id uint, updates map[string]interface{}) error {
	return m.db.WithContext(ctx).Model(&Entity{}).Where("id = ?", id).Updates(updates).Error
}

// Delete 删除实体
func (m *EntityModel) Delete(ctx context.Context, id uint) error {
	return m.db.WithContext(ctx).Delete(&Entity{}, id).Error
}

// List 分页获取实体列表
func (m *EntityModel) List(ctx context.Context, opts ListEntitiesOptions) ([]Entity, int64, error) {
	if opts.Page <= 0 {
		opts.Page = 1
	}
	if opts.PageSize <= 0 {
		opts.PageSize = 20
	}

	var (
		entities []Entity
		total    int64
	)

	query := m.db.WithContext(ctx).Model(&Entity{})

	if opts.Type != "" {
		query = query.Where("type = ?", opts.Type)
	}

	if opts.ProviderID != "" {
		query = query.Where("provider_id = ?", opts.ProviderID)
	}

	if opts.Status != nil {
		query = query.Where("status = ?", *opts.Status)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (opts.Page - 1) * opts.PageSize
	if err := query.Offset(offset).Limit(opts.PageSize).Find(&entities).Error; err != nil {
		return nil, 0, err
	}

	return entities, total, nil
}

// ValidateEntityData 验证实体数据格式
func (m *EntityModel) ValidateEntityData(entityType string, data interface{}) error {
	// 这里可以根据 entityType 实现不同的验证逻辑
	// 目前只是基本的非空检查
	if entityType == "" {
		return fmt.Errorf("entity type cannot be empty")
	}
	return nil
}
