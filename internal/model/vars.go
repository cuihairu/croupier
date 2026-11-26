package model

import (
	"errors"
	"time"

	"gorm.io/gorm"
)

// 常用错误定义
var (
	ErrNotFound            = gorm.ErrRecordNotFound
	ErrDuplicateKey        = errors.New("duplicate key error")
	ErrInvalidData         = errors.New("invalid data")
	ErrInvalidPassword     = errors.New("invalid password")
	ErrPermissionDenied    = errors.New("permission denied")
	ErrInsufficientBalance = errors.New("insufficient balance")
)

// 状态常量
const (
	StatusDisabled  = 0
	StatusEnabled   = 1
	StatusSuspended = 2
)

// 游戏状态常量
const (
	GameStatusDev         = "dev"
	GameStatusTest        = "test"
	GameStatusRunning     = "running"
	GameStatusOnline      = "online"
	GameStatusOffline     = "offline"
	GameStatusMaintenance = "maintenance"
)

// 玩家状态常量
const (
	PlayerStatusBanned    = 0
	PlayerStatusActive    = 1
	PlayerStatusSuspended = 2
)

// 通用分页选项
type PaginationOptions struct {
	Page     int
	PageSize int
}

// 标准化分页参数
func (opts *PaginationOptions) Normalize() {
	opts.normalizeWithLimit(maxPageSize)
}

// normalizeWithLimit applies the shared normalization with a caller-provided
// upper bound on page size.
func (opts *PaginationOptions) normalizeWithLimit(limit int) {
	if opts.Page <= 0 {
		opts.Page = 1
	}
	if opts.PageSize <= 0 {
		opts.PageSize = 20
	}
	if opts.PageSize > limit {
		opts.PageSize = limit
	}
}

// maxPageSize caps page size for interactive paginated endpoints. Aggregation
// callers that must scan a whole window page through offset windows via
// NewAggregationPagination instead, so their large windows are not truncated.
const maxPageSize = 100

// 计算偏移量
func (opts *PaginationOptions) Offset() int {
	return (opts.Page - 1) * opts.PageSize
}

// NewPagination returns normalized PaginationOptions. It clamps page >= 1 and
// 1 <= pageSize <= maxPageSize (default 20), and is the recommended
// constructor for interactive paged endpoints.
func NewPagination(page, pageSize int) PaginationOptions {
	opts := PaginationOptions{Page: page, PageSize: pageSize}
	opts.Normalize()
	return opts
}

// NewAggregationPagination returns normalized PaginationOptions whose page size
// is bounded by maxAggregationPageSize instead of the interactive cap, so
// window-wide aggregation scans can request larger pages.
func NewAggregationPagination(page, pageSize int) PaginationOptions {
	opts := PaginationOptions{Page: page, PageSize: pageSize}
	opts.normalizeWithLimit(maxAggregationPageSize)
	return opts
}

// maxAggregationPageSize bounds aggregation scan pages. It is a page-size
// guard only; window-wide aggregation still pages until the total is covered.
const maxAggregationPageSize = 5000

// 通用更新时间跟踪
type TimestampMixin struct {
	CreatedAt time.Time `gorm:"autoCreateTime"`
	UpdatedAt time.Time `gorm:"autoUpdateTime"`
}

// NowUTC 获取当前 UTC 时间
func NowUTC() time.Time {
	return time.Now().UTC()
}

// IsValidStatus 检查状态值是否有效
func IsValidStatus(status int) bool {
	return status >= 0 && status <= 2
}

// IsValidGameStatus 检查游戏状态值是否有效
func IsValidGameStatus(status string) bool {
	validStatuses := map[string]bool{
		GameStatusDev:         true,
		GameStatusTest:        true,
		GameStatusRunning:     true,
		GameStatusOnline:      true,
		GameStatusOffline:     true,
		GameStatusMaintenance: true,
	}
	return validStatuses[status]
}
