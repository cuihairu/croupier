package model

import (
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type TaskRun struct {
	gorm.Model
	TaskID            string `gorm:"size:64;uniqueIndex"`
	FunctionID        string `gorm:"size:128;index"`
	AgentID           string `gorm:"size:128;index"`
	ProviderID        string `gorm:"size:128;index"`
	GameID            string `gorm:"size:64;index"`
	Env               string `gorm:"size:64;index"`
	Status            string `gorm:"size:32;index"`
	Progress          int32
	Message           string         `gorm:"size:255"`
	InputPayload      datatypes.JSON `gorm:"type:json"`
	ResultPayload     datatypes.JSON `gorm:"type:json"`
	ErrorMessage      string         `gorm:"type:text"`
	StartedAt         *time.Time
	FinishedAt        *time.Time
	CancelRequestedAt *time.Time
	TraceID           string `gorm:"size:128;index"`
	IdempotencyKey    string `gorm:"size:128;index"`
}

func (TaskRun) TableName() string {
	return "task_runs"
}

type TaskEvent struct {
	gorm.Model
	TaskID    string `gorm:"size:64;index:idx_task_events_task_seq,priority:1"`
	Seq       int64  `gorm:"index:idx_task_events_task_seq,priority:2"`
	Type      string `gorm:"size:32;index"`
	Progress  int32
	Message   string         `gorm:"size:255"`
	Payload   datatypes.JSON `gorm:"type:json"`
	CreatedAt time.Time      `gorm:"index"`
}

func (TaskEvent) TableName() string {
	return "task_events"
}
