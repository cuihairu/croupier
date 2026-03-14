package model

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"
)

// PermissionModel exposes query helpers for permissions.
type PermissionModel struct {
	db *gorm.DB
}

// NewPermissionModel constructs a PermissionModel.
func NewPermissionModel(db *gorm.DB) *PermissionModel {
	return &PermissionModel{db: db}
}

// ListPermissionsOptions controls pagination/filtering.
type ListPermissionsOptions struct {
	Page     int
	PageSize int
	Category string
	Resource string
}

// List fetches permissions using filters.
func (m *PermissionModel) List(ctx context.Context, opts ListPermissionsOptions) ([]Permission, int64, error) {
	if opts.Page <= 0 {
		opts.Page = 1
	}
	if opts.PageSize <= 0 {
		opts.PageSize = 20
	}

	query := m.db.WithContext(ctx).Model(&Permission{})
	if opts.Category != "" {
		query = query.Where("category = ?", opts.Category)
	}
	if opts.Resource != "" {
		query = query.Where("resource = ?", opts.Resource)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var permissions []Permission
	offset := (opts.Page - 1) * opts.PageSize
	if err := query.
		Order("id ASC").
		Offset(offset).
		Limit(opts.PageSize).
		Find(&permissions).Error; err != nil {
		return nil, 0, err
	}

	return permissions, total, nil
}

// FindOne fetches a permission by ID.
func (m *PermissionModel) FindOne(ctx context.Context, id string) (*Permission, error) {
	var perm Permission
	if err := m.db.WithContext(ctx).First(&perm, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("permission not found")
		}
		return nil, err
	}
	return &perm, nil
}
