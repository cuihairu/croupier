package model

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// Hotpatch tracks one server-side hot-patch rollout through the platform
// channel (docs/research/hot-patch-design.md §3.2). The platform owns
// hosting/approval/rollout/audit; framework-specific reload happens in the
// agent adapter and is only referenced here.
type Hotpatch struct {
	gorm.Model
	GameID string `gorm:"size:64;index:idx_hotpatch_scope,priority:1"`
	Env    string `gorm:"size:64;index:idx_hotpatch_scope,priority:2"`

	// Framework: skynet | kbengine | jvm | nodejs | custom
	Framework string `gorm:"size:32;index"`
	// TargetSelector: server-id / node-label list (JSON array). Empty = all.
	TargetSelector JSON `gorm:"type:json"`
	// EntrySpec carries adapter parameters (e.g. skynet entry lua file,
	// jvm classFilter). Framework-white-listed by the adapter; the platform
	// only stores it.
	EntrySpec datatypes.JSONMap `gorm:"type:json"`

	// Package hosted in objstore; checksum is mandatory end-to-end.
	PackageKey       string `gorm:"size:512"`
	Size             int64
	Checksum         string `gorm:"size:64"` // sha256 hex
	FrameworkVersion string `gorm:"size:64"` // e.g. skynet 1.7

	// Status flow: draft → approved → rolling → applied / failed / rolled_back
	Status string `gorm:"size:32;index"`

	// Rollout: node-level gray (bucket by node id, mirrors releases).
	RolloutPercent int    `gorm:"default:0"`
	RolloutSeed    string `gorm:"size:32"`

	// Linked defect (fix traceability, mandatory per design §3.4).
	BugID uint `gorm:"index"`

	// Per-agent results recorded as they come back.
	Results JSON `gorm:"type:json"` // [{agentId,node,status,log,at}]

	ApprovalID string     `gorm:"size:64"` // two-person rule reference
	CreatedBy  string     `gorm:"size:64"`
	DueAt      *time.Time `gorm:"index"`
}

func (Hotpatch) TableName() string { return "hotpatches" }

// Hotpatch statuses.
const (
	HotpatchStatusDraft      = "draft"
	HotpatchStatusApproved   = "approved"
	HotpatchStatusRolling    = "rolling"
	HotpatchStatusApplied    = "applied"
	HotpatchStatusFailed     = "failed"
	HotpatchStatusRolledBack = "rolled_back"
)

// Hotpatch frameworks (closed set; agent adapter registry depends on it).
const (
	HotpatchFrameworkSkynet = "skynet"
	HotpatchFrameworkKBE    = "kbengine"
	HotpatchFrameworkJVM    = "jvm"
	HotpatchFrameworkNodeJS = "nodejs"
	HotpatchFrameworkCustom = "custom"
)

// ValidHotpatchFrameworks is the closed set of frameworks.
var ValidHotpatchFrameworks = map[string]struct{}{
	HotpatchFrameworkSkynet: {}, HotpatchFrameworkKBE: {}, HotpatchFrameworkJVM: {},
	HotpatchFrameworkNodeJS: {}, HotpatchFrameworkCustom: {},
}

// ValidHotpatchTransitions lists legal state moves. rolling→rolling updates
// only the rollout percent (non-decreasing, rollback semantics otherwise).
var ValidHotpatchTransitions = map[string][]string{
	HotpatchStatusDraft:    {HotpatchStatusApproved, HotpatchStatusFailed},
	HotpatchStatusApproved: {HotpatchStatusRolling, HotpatchStatusFailed},
	HotpatchStatusRolling:  {HotpatchStatusRolling, HotpatchStatusApplied, HotpatchStatusFailed, HotpatchStatusRolledBack},
	HotpatchStatusApplied:  {HotpatchStatusRolledBack},
	HotpatchStatusFailed:   {HotpatchStatusRolledBack},
}

// CanHotpatchTransition reports whether from → to is legal.
func CanHotpatchTransition(from, to string) bool {
	for _, next := range ValidHotpatchTransitions[from] {
		if next == to {
			return true
		}
	}
	return false
}

// HotpatchResult is one agent's execution outcome.
type HotpatchResult struct {
	AgentID string `json:"agentId"`
	Node    string `json:"node,omitempty"`
	Status  string `json:"status"` // ok | failed | rolled_back
	Log     string `json:"log,omitempty"`
	At      string `json:"at"`
}

// HotpatchModel provides CRUD for hotpatches.
type HotpatchModel struct {
	db *gorm.DB
}

// NewHotpatchModel creates a helper.
func NewHotpatchModel(db *gorm.DB) *HotpatchModel {
	return &HotpatchModel{db: db}
}

// HotpatchQueryOptions controls listing.
type HotpatchQueryOptions struct {
	PaginationOptions
	GameID    string
	Env       string
	Framework string
	Status    string
}

// List returns hotpatches matching filters.
func (m *HotpatchModel) List(ctx context.Context, opts HotpatchQueryOptions) ([]Hotpatch, int64, error) {
	opts.PaginationOptions.Normalize()
	var (
		items []Hotpatch
		total int64
	)
	query := m.db.WithContext(ctx).Model(&Hotpatch{})
	if opts.GameID != "" {
		query = query.Where("game_id = ?", opts.GameID)
	}
	if opts.Env != "" {
		query = query.Where("env = ?", opts.Env)
	}
	if opts.Framework != "" {
		query = query.Where("framework = ?", opts.Framework)
	}
	if opts.Status != "" {
		query = query.Where("status = ?", opts.Status)
	}
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := query.Order("updated_at DESC").
		Offset(opts.Offset()).Limit(opts.PageSize).Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// FindOne returns a hotpatch by id.
func (m *HotpatchModel) FindOne(ctx context.Context, id uint) (*Hotpatch, error) {
	var hp Hotpatch
	if err := m.db.WithContext(ctx).First(&hp, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
		return nil, err
	}
	return &hp, nil
}

// Create inserts a hotpatch.
func (m *HotpatchModel) Create(ctx context.Context, hp *Hotpatch) error {
	return m.db.WithContext(ctx).Create(hp).Error
}

// Update applies a partial update map.
func (m *HotpatchModel) Update(ctx context.Context, id uint, updates map[string]interface{}) error {
	return m.db.WithContext(ctx).Model(&Hotpatch{}).Where("id = ?", id).Updates(updates).Error
}

// Transition moves the state machine forward inside a transaction.
func (m *HotpatchModel) Transition(ctx context.Context, id uint, to string, rolloutPercent *int) (*Hotpatch, error) {
	var result *Hotpatch
	err := m.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var hp Hotpatch
		if err := tx.First(&hp, id).Error; err != nil {
			return err
		}
		if !CanHotpatchTransition(hp.Status, to) {
			return fmt.Errorf("invalid transition %s -> %s", hp.Status, to)
		}
		if (to == HotpatchStatusApproved || to == HotpatchStatusRolling) && hp.PackageKey == "" {
			return errors.New("package must be uploaded before rollout")
		}
		if to == HotpatchStatusApproved && hp.BugID == 0 {
			return errors.New("hotpatch must reference a bug (traceability)")
		}
		updates := map[string]interface{}{"status": to}
		if to == HotpatchStatusRolling {
			pct := hp.RolloutPercent
			if rolloutPercent != nil {
				pct = *rolloutPercent
			}
			if pct < 0 || pct > 100 {
				return errors.New("rollout percent must be within 0-100")
			}
			if pct < hp.RolloutPercent {
				return errors.New("rollout percent can only increase; use rollback to reduce exposure")
			}
			updates["rollout_percent"] = pct
		}
		if err := tx.Model(&Hotpatch{}).Where("id = ?", id).Updates(updates).Error; err != nil {
			return err
		}
		result = &hp
		result.Status = to
		if pct, ok := updates["rollout_percent"].(int); ok {
			result.RolloutPercent = pct
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// AppendResult records one agent outcome (idempotent per agent: last wins).
func (m *HotpatchModel) AppendResult(ctx context.Context, id uint, r HotpatchResult) error {
	return m.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var hp Hotpatch
		if err := tx.First(&hp, id).Error; err != nil {
			return err
		}
		var results []HotpatchResult
		if len(hp.Results) > 0 {

		}
		replaced := false
		for i := range results {
			if results[i].AgentID == r.AgentID {
				results[i] = r
				replaced = true
				break
			}
		}
		if !replaced {
			results = append(results, r)
		}
		bytes, err := json.Marshal(results)
		if err != nil {
			return err
		}
		return tx.Model(&Hotpatch{}).Where("id = ?", id).
			Update("results", bytes).Error
	})
}

// BucketHit mirrors the release gray bucketing for node-level rollout.
func (h *Hotpatch) BucketHit(nodeID string) bool {
	if h.RolloutPercent >= 100 {
		return true
	}
	if h.RolloutPercent <= 0 {
		return false
	}
	sum := sha256.Sum256([]byte(nodeID + "|" + h.RolloutSeed))
	raw := sum[:4]
	value := (uint32(raw[0])<<24 | uint32(raw[1])<<16 | uint32(raw[2])<<8 | uint32(raw[3])) % 10000
	return int(value) < h.RolloutPercent*100
}

// HotpatchSeedHex generates a rollout seed.
func HotpatchSeedHex() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%x", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}

// NormalizeHotpatchFramework validates a framework value.
func NormalizeHotpatchFramework(f string) (string, bool) {
	f = strings.ToLower(strings.TrimSpace(f))
	if _, ok := ValidHotpatchFrameworks[f]; ok {
		return f, true
	}
	return "", false
}
