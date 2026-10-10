package model

import (
	"time"

	"gorm.io/gorm"
)

// IncidentReport 周期报表快照（docs/design/incident-reports.md §2.4）：报告
// 生成器 server-local 执行后落库的完整 payload，PushStatus 记录按类别的
// 分发结果。meta DB 存储（报表跨 game 聚合，与 incidents 同库）。
type IncidentReport struct {
	gorm.Model
	// PeriodType+PeriodStart 联合唯一：同档同期只留一行，重复触发生成即覆盖语义
	// 由调度幂等（run-log slot）保证，这里兜底防并发双写。
	PeriodType  string `gorm:"size:16;uniqueIndex:uidx_incident_reports_period,priority:1"` // week|month|quarter|year
	PeriodStart string `gorm:"size:16;uniqueIndex:uidx_incident_reports_period,priority:2"` // 2026-W41 | 2026-10 | 2026-Q4 | 2026
	ReportKind  string `gorm:"size:16"`                                                     // summary|responsibility|leaderboard|trend
	Payload     JSON   `gorm:"type:json"`                                                   // 完整报表 payload（§5 指标+对比+矩阵）
	GeneratedAt time.Time
	PushStatus  JSON `gorm:"type:json"` // [{slug,leader,channel,pushedAt,eventId,ok,error}]
}

func (IncidentReport) TableName() string { return "incident_reports" }

// 报表档位与形态常量（与 web 端 PeriodType 口径一致）。
const (
	ReportPeriodWeek    = "week"
	ReportPeriodMonth   = "month"
	ReportPeriodQuarter = "quarter"
	ReportPeriodYear    = "year"

	ReportKindSummary        = "summary"
	ReportKindResponsibility = "responsibility"
	ReportKindLeaderboard    = "leaderboard"
	ReportKindTrend          = "trend"
)

// ExternalToken 对外 REST API 调用令牌（§9）：sha256 哈希存库、明文只在
// 创建响应返回一次；scope 限可见面（categories 为空 = 全量），GET 过滤与
// webhook 事件推送共用同一收窄口径。
type ExternalToken struct {
	gorm.Model
	Name string `gorm:"size:64;not null;uniqueIndex"` // 调用方标识（审计/限流维度）
	// TokenHash sha256(token) 十六进制（64 字符定长），uniqueIndex 支持按哈希反查校验。
	TokenHash  string `gorm:"size:64;not null;uniqueIndex"`
	Scope      JSON   `gorm:"type:json"` // {categories:[slug...]}；null=全量
	Enabled    bool   `gorm:"default:true"`
	LastUsedAt *time.Time
	CreatedBy  string `gorm:"size:64"`
}

func (ExternalToken) TableName() string { return "external_tokens" }
