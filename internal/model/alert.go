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
	// GameID/Env 告警归属 scope（空 = 平台级，全局可见）。与 AlertRule 的
	// GameID/Env 列语义一致：空串表示平台级告警，非空表示 game/env 级告警。
	// 列表过滤：scope 非空时 (game_id=? AND env=?) OR (game_id='' AND env='')；
	// scope 为空时不过滤（全局视野，面板告警页）。
	// 打戳点：alertrule evaluator（rule.GameID/Env）、inbound generic
	// payload（scope.gameId/env）、control_handler capture 告警（req 字段）。
	// dbmon/pipeline_monitor 为基础设施告警，不打戳（平台级）。
	GameID string `gorm:"size:64;index"`
	Env    string `gorm:"size:64;index"`
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
