package model

import "time"

// PowerRequest is one approval-gated power operation over a snapshot of
// target machines (OPS power management, docs/design/ops-power-management.md
// §6). State machine: pending → executing → verifying → done/partial/failed,
// with pending → rejected when the approval is denied. JSON keys are the
// /ops/power contract surface (lowerCamelCase, 契约命名规则).
type PowerRequest struct {
	ID uint `gorm:"primaryKey" json:"id"`
	// RequestID is the stable public id.
	RequestID string `gorm:"size:64;uniqueIndex;not null" json:"requestId"`
	// Action is the power action (power_on/power_off/force_off/reboot/status).
	Action string `gorm:"size:32;not null" json:"action"`
	// Reason is the operator-supplied cause, kept for the audit trail.
	Reason string `gorm:"size:255" json:"reason"`
	// RequestedBy is the authenticated requester (approval actor).
	RequestedBy string `gorm:"size:64" json:"requestedBy"`
	// ApprovalID links the approvals record; empty only if gate creation failed
	// (request stays pending and refuses resume).
	ApprovalID string `gorm:"size:64;index" json:"approvalId"`
	// State is the request state (pending/executing/verifying/done/partial/failed/rejected).
	State string `gorm:"size:16;index;not null" json:"state"`
	// TargetsJSON snapshots the resolved machines at submit time — the
	// approved scope cannot silently change afterwards.
	TargetsJSON string `gorm:"type:text" json:"targetsJson"`
	// Total / Succeeded / Failed are per-machine counters for batch aggregation.
	Total     int `gorm:"not null;default:0" json:"total"`
	Succeeded int `gorm:"not null;default:0" json:"succeeded"`
	Failed    int `gorm:"not null;default:0" json:"failed"`
	// ErrorMsg carries the orchestrator-level failure (e.g. approval gate error).
	ErrorMsg  string     `gorm:"size:512" json:"errorMsg"`
	GameID    string     `gorm:"size:64;index" json:"gameId"`
	Env       string     `gorm:"size:64;index" json:"env"`
	StartedAt *time.Time `json:"startedAt"`
	DoneAt    *time.Time `json:"doneAt"`
	CreatedAt time.Time  `json:"createdAt"`
	UpdatedAt time.Time  `json:"updatedAt"`
}

func (PowerRequest) TableName() string { return "power_requests" }

// PowerRequestTarget is one machine's outcome inside a power request.
type PowerRequestTarget struct {
	ID        uint   `gorm:"primaryKey" json:"id"`
	RequestID string `gorm:"size:64;index:idx_power_target_request;not null" json:"requestId"`
	NodeID    string `gorm:"size:64;index:idx_power_target_request;not null" json:"nodeId"`
	// Provider is the machine's provider kind (ipmi/wol/cloud/agent).
	Provider string `gorm:"size:16" json:"provider"`
	// Success reports the action execution; Verified reports the post-action
	// state confirmation (power_on on, power_off off, agent reconnect).
	Success  bool `gorm:"not null;default:false" json:"success"`
	Verified bool `gorm:"not null;default:false" json:"verified"`
	// State is the last observed power state (on/off/unknown).
	State string `gorm:"size:16" json:"state"`
	// Output is the non-sensitive provider output tail.
	Output string `gorm:"size:512" json:"output"`
	// ErrorMsg is the failure reason (unsupported action, unreachable, ...).
	ErrorMsg   string     `gorm:"size:512" json:"errorMsg"`
	AttemptAt  *time.Time `json:"attemptAt"`
	VerifiedAt *time.Time `json:"verifiedAt"`
	CreatedAt  time.Time  `json:"createdAt"`
	UpdatedAt  time.Time  `json:"updatedAt"`
}

func (PowerRequestTarget) TableName() string { return "power_request_targets" }
