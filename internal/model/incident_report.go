package model

import (
	"context"
	"errors"
	"log/slog"
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

// 播种的报表调度（docs/design/incident-reports.md §6）：周报每周一 09:00、
// 月报每月 1 日 09:00；GameID/Env="*" 表示跨作用域；FunctionID 固定
// "incident-report"（Kind=incident_report 时调度器不进函数派发链）。
const (
	ScheduleNameIncidentWeek  = "incident-report-week"
	ScheduleNameIncidentMonth = "incident-report-month"
	FunctionIDIncidentReport  = "incident-report"
)

// SeedIncidentReportSchedules 幂等播种周期报表调度（按 name 存在即跳过），
// 创建后经 nextAfter 回调立即算出 next_triggered_at（ListDue 扫描条件，
// 否则永不触发）。返回新播种条数。HasTable 守卫内建（multiGame 模式 meta
// 库无 task_schedules 表时静默跳过）。nextAfter 由调用方注入（svc 侧经
// scheduler.ParseCron 实现——model 不 import scheduler，防环）。
func SeedIncidentReportSchedules(ctx context.Context, m *TaskScheduleModel, nextAfter func(cronExpr string) (time.Time, bool)) (int, error) {
	if m == nil || m.db == nil || !m.db.Migrator().HasTable(&TaskSchedule{}) {
		return 0, nil
	}
	seeds := []struct {
		Name     string
		CronExpr string
		Payload  JSON
	}{
		{Name: ScheduleNameIncidentWeek, CronExpr: "0 9 * * 1", Payload: JSON(`{"period":"week"}`)},
		{Name: ScheduleNameIncidentMonth, CronExpr: "0 9 1 * *", Payload: JSON(`{"period":"month"}`)},
	}
	count := 0
	for _, sd := range seeds {
		var existing TaskSchedule
		err := m.db.WithContext(ctx).Where("name = ?", sd.Name).First(&existing).Error
		if err == nil {
			continue
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return count, err
		}
		created, err := m.Create(ctx, CreateScheduleInput{
			Name:       sd.Name,
			CronExpr:   sd.CronExpr,
			GameID:     "*",
			Env:        "*",
			FunctionID: FunctionIDIncidentReport,
			Payload:    sd.Payload,
			Kind:       ScheduleKindIncidentReport,
			Actor:      "system",
		})
		if err != nil {
			return count, err
		}
		if nextAfter != nil {
			if next, ok := nextAfter(sd.CronExpr); ok {
				if uerr := m.UpdateSchedule(ctx, created.ID, map[string]interface{}{"next_triggered_at": next}); uerr != nil {
					slog.WarnContext(ctx, "seed incident report schedule: set next trigger failed", "schedule", sd.Name, "error", uerr)
				}
			}
		}
		count++
	}
	return count, nil
}
