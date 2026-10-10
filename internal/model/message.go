package model

import (
	"time"

	"github.com/cuihairu/croupier/internal/dbenum"
	"gorm.io/gorm"
)

// Message represents system/user notifications.
type Message struct {
	gorm.Model
	To      string               `gorm:"column:recipient;size:255;not null;index:idx_messages_to_status,priority:1;index:idx_messages_to_created"`
	Type    string               `gorm:"size:64;not null;index"`
	Title   string               `gorm:"size:255"`
	Content string               `gorm:"type:text"`
	Data    JSON                 `gorm:"type:json"`
	Status  dbenum.MessageStatus `gorm:"index:idx_messages_to_status,priority:2;index:idx_messages_status_created"`
	ReadAt  *time.Time           `gorm:"index"`
	// 以下五列为 0041 站内通知强化（incident-reports.md §2.5 + 插件机制
	// §5.1）：紧急度分级、来源/关联追溯、可见范围。历史行经迁移回填。
	Level   string `gorm:"size:16;index;default:info"` // info|warn|critical
	Source  string `gorm:"size:64"`                    // 产生方，如 incident_report|alert|supervisor
	RefType string `gorm:"size:32"`                    // 关联类型，如 incident|report|server
	RefID   string `gorm:"size:128;index"`             // 关联对象 ID
	Scope   JSON   `gorm:"type:json"`                  // 可见范围 {categories:[slug...]}；null=全量
}

// 通知分级常量（level 列闭集）。
const (
	MessageLevelInfo     = "info"
	MessageLevelWarn     = "warn"
	MessageLevelCritical = "critical"
)

// TableName implements gorm's tabler interface.
func (Message) TableName() string {
	return "messages"
}
