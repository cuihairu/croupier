package model

import (
	"gorm.io/gorm"
	"time"
)

// TaskSchedule 是定时任务（cron 调度）的定义。
//
// 生命周期：
//
//	active ⇄ paused
//	  └──────► dead_letter（连续触发失败达到 MaxFailedRuns）
//
// 每次到期触发生成一条普通 TaskRun（function_id + payload 复用既有
// 异步任务链路），ScheduleRunLog 记录触发历史（含幂等窗口）。
type TaskSchedule struct {
	gorm.Model
	Name          string `gorm:"size:128;index"`               // 展示名
	CronExpr      string `gorm:"size:64;not null"`             // 五字段 cron（分 时 日 月 周）
	GameID        string `gorm:"size:64;not null;index"`       // 作用域
	Env           string `gorm:"size:64;not null;index"`       // 作用域
	FunctionID    string `gorm:"size:128;not null;index"`      // 目标函数
	Payload       JSON   `gorm:"type:json"`                    // 调用参数
	Metadata      JSON   `gorm:"type:json"`                    // 附带 metadata（如 route）
	Status        string `gorm:"size:32;index;default:active"` // active | paused | dead_letter
	Timezone      string `gorm:"size:64"`                      // IANA 时区，空 = UTC
	MaxFailedRuns int    `gorm:"default:5"`                    // 连续失败上限，达到后进 dead_letter
	// ConsecutiveFailures 连续触发失败计数（成功后清零）。
	ConsecutiveFailures int `gorm:"default:0"`
	// LastTriggeredAt 上次到期触发时间（幂等窗口判重 + 下次触发推算基线）。
	LastTriggeredAt *time.Time
	// NextTriggeredAt 预计算的下次触发时间（调度循环扫描条件）。
	NextTriggeredAt *time.Time `gorm:"index"`
	LastRunID       string     `gorm:"size:64"` // 最近一次触发的 TaskRun ID
	Actor           string     `gorm:"size:128;index"`
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

func (TaskSchedule) TableName() string { return "task_schedules" }

// Schedule 状态常量。
const (
	ScheduleStatusActive     = "active"
	ScheduleStatusPaused     = "paused"
	ScheduleStatusDeadLetter = "dead_letter"
)

// TaskScheduleRunLog 是一次到期触发的审计记录（幂等窗口内不重复触发）。
// schedule_id+slot 唯一索引是多实例并发触发的兜底防线。
type TaskScheduleRunLog struct {
	gorm.Model
	ScheduleID uint      `gorm:"not null;uniqueIndex:uidx_schedule_run_slot,priority:1"`
	Slot       time.Time `gorm:"not null;uniqueIndex:uidx_schedule_run_slot,priority:2"`
	TaskRunID  string    `gorm:"size:64"`
	Status     string    `gorm:"size:32"` // dispatched | skipped | failed
	Message    string    `gorm:"size:255"`
	CreatedAt  time.Time
}

func (TaskScheduleRunLog) TableName() string { return "task_schedule_run_logs" }
