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
	if opts.Page <= 0 {
		opts.Page = 1
	}
	if opts.PageSize <= 0 {
		opts.PageSize = 20
	}
	if opts.PageSize > 100 {
		opts.PageSize = 100
	}
}

// 计算偏移量
func (opts *PaginationOptions) Offset() int {
	return (opts.Page - 1) * opts.PageSize
}

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
