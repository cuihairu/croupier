package database

import (
	"context"
	"fmt"
	"math"

	"gorm.io/gorm"
)

// ListOptions 分页查询选项
type ListOptions struct {
	Page     int    `form:"page" binding:"min=1" default:"1"`
	PageSize int    `form:"page_size" binding:"min=1,max=100" default:"20"`
	Sort     string `form:"sort" default:"id"`
	Order    string `form:"order" default:"asc"`
	Search   string `form:"search"`
}

// Validate 验证分页选项
func (opts *ListOptions) Validate() error {
	if opts.Page < 1 {
		return fmt.Errorf("page must be greater than 0")
	}
	if opts.PageSize < 1 || opts.PageSize > 100 {
		return fmt.Errorf("page_size must be between 1 and 100")
	}
	if opts.Order != "asc" && opts.Order != "desc" {
		return fmt.Errorf("order must be 'asc' or 'desc'")
	}
	return nil
}

// GetOffset 计算偏移量
func (opts *ListOptions) GetOffset() int {
	return (opts.Page - 1) * opts.PageSize
}

// GetOrderClause 获取排序子句
func (opts *ListOptions) GetOrderClause() string {
	return fmt.Sprintf("%s %s", opts.Sort, opts.Order)
}

// PaginatedResult 分页结果
type PaginatedResult[T any] struct {
	Items      []T        `json:"items"`
	Pagination Pagination `json:"pagination"`
}

// Pagination 分页信息
type Pagination struct {
	Page       int   `json:"page"`
	PageSize   int   `json:"page_size"`
	Total      int64 `json:"total"`
	TotalPages int   `json:"total_pages"`
	HasNext    bool  `json:"has_next"`
	HasPrev    bool  `json:"has_prev"`
}

// NewPaginatedResult 创建新的分页结果
func NewPaginatedResult[T any](items []T, opts *ListOptions, total int64) *PaginatedResult[T] {
	totalPages := int(math.Ceil(float64(total) / float64(opts.PageSize)))

	return &PaginatedResult[T]{
		Items: items,
		Pagination: Pagination{
			Page:       opts.Page,
			PageSize:   opts.PageSize,
			Total:      total,
			TotalPages: totalPages,
			HasNext:    opts.Page < totalPages,
			HasPrev:    opts.Page > 1,
		},
	}
}

// QueryBuilder 查询构建器
type QueryBuilder struct {
	db *gorm.DB
}

// NewQueryBuilder 创建新的查询构建器
func NewQueryBuilder(db *gorm.DB) *QueryBuilder {
	return &QueryBuilder{db: db}
}

// List 执行分页查询
func (qb *QueryBuilder) List(ctx any, model any, opts *ListOptions) (*PaginatedResult[any], error) {
	if err := opts.Validate(); err != nil {
		return nil, fmt.Errorf("invalid list options: %w", err)
	}

	// 构建查询
	query := qb.db.WithContext(ctx.(context.Context)).Model(model)

	// 添加搜索条件
	if opts.Search != "" {
		// 这里应该根据模型的具体字段构建搜索条件
		// 为了简化，我们假设有一个可搜索的字段
		query = query.Where("name LIKE ?", "%"+opts.Search+"%")
	}

	// 获取总数
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, fmt.Errorf("failed to count records: %w", err)
	}

	// 执行分页查询
	var items []any
	offset := opts.GetOffset()
	orderClause := opts.GetOrderClause()

	if err := query.Order(orderClause).Offset(offset).Limit(opts.PageSize).Find(&items).Error; err != nil {
		return nil, fmt.Errorf("failed to fetch records: %w", err)
	}

	return NewPaginatedResult(items, opts, total), nil
}

// ListGeneric 通用分页查询
func ListGeneric[T any](db *gorm.DB, ctx any, opts *ListOptions, searchCondition func(*gorm.DB) *gorm.DB) (*PaginatedResult[T], error) {
	if err := opts.Validate(); err != nil {
		return nil, fmt.Errorf("invalid list options: %w", err)
	}

	// 构建查询
	query := db.WithContext(ctx.(context.Context))

	// 应用搜索条件
	if searchCondition != nil {
		query = searchCondition(query)
	}

	// 获取总数
	var total int64
	if err := query.Model(new(T)).Count(&total).Error; err != nil {
		return nil, fmt.Errorf("failed to count records: %w", err)
	}

	// 执行分页查询
	var items []T
	offset := opts.GetOffset()
	orderClause := opts.GetOrderClause()

	if err := query.Model(new(T)).Order(orderClause).Offset(offset).Limit(opts.PageSize).Find(&items).Error; err != nil {
		return nil, fmt.Errorf("failed to fetch records: %w", err)
	}

	return NewPaginatedResult(items, opts, total), nil
}