package model

import (
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// Alert captures runtime system alerts for ops center.
type Alert struct {
	gorm.Model
	AlertID string `gorm:"size:64;uniqueIndex"`
	Type    string `gorm:"size:64"`
	Level   string `gorm:"size:32"`
	Message string `gorm:"type:text"`
	Source  string `gorm:"size:64"`
	Status  string `gorm:"size:32;index"`
	// HandleStatus 风险事件工作流状态（人工处理面）。独立于 Status 列——
	// Status 被 AlertManager 入站 lifecycle 语义占用（firing/resolved），
	// 人工处理状态混进去会互相覆盖。取值 pending 待处理 / acknowledged
	// 已确认 / ignored 已忽略 / escalated 已升级；缺省 pending。
	HandleStatus  string `gorm:"size:32;index;default:pending"`
	HandledBy     string `gorm:"size:64"`
	HandledRemark string `gorm:"size:255"`
	HandledAt     *time.Time
	Details       datatypes.JSONMap `gorm:"type:json"`
	Metadata      datatypes.JSONMap `gorm:"type:json"`
	CreatedBy     string            `gorm:"size:64"`
}

// HandleStatus 工作流状态取值（风险事件扩展，risk-control-panel-design.md §3.3）。
const (
	AlertHandlePending      = "pending"
	AlertHandleAcknowledged = "acknowledged"
	AlertHandleIgnored      = "ignored"
	AlertHandleEscalated    = "escalated"
)

// AlertSilence stores silence windows for alerts.
type AlertSilence struct {
	gorm.Model
	AlertID        uint      `gorm:"index"`
	Reason         string    `gorm:"size:255"`
	DurationMinute int       `gorm:"default:0"`
	ExpiresAt      time.Time `gorm:"index"`
	CreatedBy      string    `gorm:"size:64"`
}

func (Alert) TableName() string {
	return "alerts"
}

func (AlertSilence) TableName() string {
	return "alert_silences"
}
