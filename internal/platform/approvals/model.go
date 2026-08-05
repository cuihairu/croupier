package approvals

import (
	"time"

	"gorm.io/gorm"
)

// ApprovalModel is the GORM model for approvals table
type ApprovalModel struct {
	ID              string         `gorm:"primaryKey;type:varchar(255)" json:"id"`
	State           string         `gorm:"type:varchar(50);not null;index" json:"state"`
	FunctionID      string         `gorm:"type:varchar(255);not null;index" json:"function_id"`
	GameID          string         `gorm:"type:varchar(255);not null;index" json:"game_id"`
	Env             string         `gorm:"type:varchar(100);not null;index" json:"env"`
	Actor           string         `gorm:"type:varchar(255);not null;index" json:"actor"`
	Mode            string         `gorm:"type:varchar(50);default:invoke" json:"mode"`
	IdempotencyKey  string         `gorm:"type:varchar(255);index" json:"idempotency_key"`
	Route           string         `gorm:"type:varchar(500)" json:"route"`
	TargetServiceID string         `gorm:"type:varchar(255);index" json:"target_service_id"`
	HashKey         string         `gorm:"type:varchar(255);index" json:"hash_key"`
	Payload         []byte         `gorm:"type:blob" json:"payload"`
	Reason          string         `gorm:"type:text" json:"reason"`
	ResultKind      string         `gorm:"type:varchar(50)" json:"result_kind"`
	TaskID          string         `gorm:"type:varchar(255);index" json:"task_id"`
	Result          []byte         `gorm:"type:blob" json:"result"`
	CreatedAt       time.Time      `gorm:"not null;index" json:"created_at"`
	UpdatedAt       time.Time      `gorm:"not null" json:"updated_at"`
	DeletedAt       gorm.DeletedAt `gorm:"index" json:"deleted_at"`
}

// TableName returns the table name for ApprovalModel
func (ApprovalModel) TableName() string {
	return "approvals"
}

// ToApproval converts ApprovalModel to Approval
func (a *ApprovalModel) ToApproval() *Approval {
	if a == nil {
		return nil
	}
	return &Approval{
		ID:              a.ID,
		State:           a.State,
		FunctionID:      a.FunctionID,
		GameID:          a.GameID,
		Env:             a.Env,
		Actor:           a.Actor,
		Mode:            a.Mode,
		IdempotencyKey:  a.IdempotencyKey,
		Route:           a.Route,
		TargetServiceID: a.TargetServiceID,
		HashKey:         a.HashKey,
		Payload:         a.Payload,
		Reason:          a.Reason,
		ResultKind:      a.ResultKind,
		TaskID:          a.TaskID,
		Result:          a.Result,
		CreatedAt:       a.CreatedAt,
		UpdatedAt:       a.UpdatedAt,
	}
}

// FromApproval creates ApprovalModel from Approval
func FromApproval(a *Approval) *ApprovalModel {
	if a == nil {
		return nil
	}
	return &ApprovalModel{
		ID:              a.ID,
		State:           a.State,
		FunctionID:      a.FunctionID,
		GameID:          a.GameID,
		Env:             a.Env,
		Actor:           a.Actor,
		Mode:            a.Mode,
		IdempotencyKey:  a.IdempotencyKey,
		Route:           a.Route,
		TargetServiceID: a.TargetServiceID,
		HashKey:         a.HashKey,
		Payload:         a.Payload,
		Reason:          a.Reason,
		ResultKind:      a.ResultKind,
		TaskID:          a.TaskID,
		Result:          a.Result,
		CreatedAt:       a.CreatedAt,
		UpdatedAt:       a.UpdatedAt,
	}
}

// AutoMigrate runs auto migration for ApprovalModel
func AutoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(&ApprovalModel{})
}
