package model

import "time"

// PowerRequest is one approval-gated power operation over a snapshot of
// target machines (OPS power management, docs/design/ops-power-management.md
// §6). State machine: pending → executing → verifying → done/partial/failed,
// with pending → rejected when the approval is denied.
type PowerRequest struct {
	ID uint `gorm:"primaryKey"`
	// RequestID is the stable public id.
	RequestID string `gorm:"size:64;uniqueIndex;not null"`
	// Action is the power action (power_on/power_off/force_off/reboot/status).
	Action string `gorm:"size:32;not null"`
	// Reason is the operator-supplied cause, kept for the audit trail.
	Reason string `gorm:"size:255"`
	// RequestedBy is the authenticated requester (approval actor).
	RequestedBy string `gorm:"size:64"`
	// ApprovalID links the approvals record; empty only if gate creation failed
	// (request stays pending and refuses resume).
	ApprovalID string `gorm:"size:64;index"`
	// State is the request state (pending/executing/verifying/done/partial/failed/rejected).
	State string `gorm:"size:16;index;not null"`
	// TargetsJSON snapshots the resolved machines at submit time — the
	// approved scope cannot silently change afterwards.
	TargetsJSON string `gorm:"type:text"`
	// Total / Succeeded / Failed are per-machine counters for batch aggregation.
	Total     int `gorm:"not null;default:0"`
	Succeeded int `gorm:"not null;default:0"`
	Failed    int `gorm:"not null;default:0"`
	// ErrorMsg carries the orchestrator-level failure (e.g. approval gate error).
	ErrorMsg  string `gorm:"size:512"`
	GameID    string `gorm:"size:64;index"`
	Env       string `gorm:"size:64;index"`
	StartedAt *time.Time
	DoneAt    *time.Time
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (PowerRequest) TableName() string { return "power_requests" }

// PowerRequestTarget is one machine's outcome inside a power request.
type PowerRequestTarget struct {
	ID uint `gorm:"primaryKey"`
	// RequestID references PowerRequest.RequestID.
	RequestID string `gorm:"size:64;index:idx_power_target_request;not null"`
	NodeID    string `gorm:"size:64;index:idx_power_target_request;not null"`
	// Provider is the machine's provider kind (ipmi/wol/cloud/agent).
	Provider string `gorm:"size:16"`
	// Success reports the action execution; Verified reports the post-action
	// state confirmation (power_on on, power_off off, agent reconnect).
	Success  bool `gorm:"not null;default:false"`
	Verified bool `gorm:"not null;default:false"`
	// State is the last observed power state (on/off/unknown).
	State string `gorm:"size:16"`
	// Output is the non-sensitive provider output tail.
	Output string `gorm:"size:512"`
	// ErrorMsg is the failure reason (unsupported action, unreachable, ...).
	ErrorMsg   string `gorm:"size:512"`
	AttemptAt  *time.Time
	VerifiedAt *time.Time
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

func (PowerRequestTarget) TableName() string { return "power_request_targets" }
