package model

import (
	"context"
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// PageProposal represents a generated page suggestion before user confirmation.
// It is the output of the generator from FunctionContract/CapabilitySemantics.
type PageProposal struct {
	gorm.Model
	GameID           string `gorm:"size:64;uniqueIndex:idx_page_proposal_scope"`
	Env              string `gorm:"size:64;uniqueIndex:idx_page_proposal_scope"`
	ProposalKey      string `gorm:"size:128;uniqueIndex:idx_page_proposal_scope"` // resourceKey or functionId
	PageKey          string `gorm:"size:128;index"`
	PageType         string `gorm:"size:32"` // resource|operation|task|report
	ResourceKey      string `gorm:"size:64;index"`
	Quality          string `gorm:"size:32"` // ready|basic|needs_review
	GeneratorVersion string `gorm:"size:32"`

	// Source tracking
	FunctionDigest  string `gorm:"size:64"` // digest of FunctionContract
	SemanticsDigest string `gorm:"size:64"` // digest of CapabilitySemantics

	// Generated content
	Title       datatypes.JSONMap `gorm:"type:json"` // LocalizedText
	Description datatypes.JSONMap `gorm:"type:json"` // LocalizedText
	CategoryKey string            `gorm:"size:64"`
	PageSpec    datatypes.JSON    `gorm:"type:json"` // PageSpec JSON

	// Diagnostics
	Diagnostics datatypes.JSON `gorm:"type:json"` // Diagnostic array

	// Status
	Status string `gorm:"size:32;default:'pending'"` // pending|accepted|rejected|expired

	UpdatedAt time.Time
	UpdatedBy string `gorm:"size:64"`
}

// PageProposalVersion stores version history of proposals.
type PageProposalVersion struct {
	gorm.Model
	ProposalID      uint `gorm:"index"`
	Version         int
	Proposal        datatypes.JSON `gorm:"type:json"` // Snapshot of PageProposal
	FunctionDigest  string         `gorm:"size:64"`
	SemanticsDigest string         `gorm:"size:64"`
	ChangeReason    string         `gorm:"size:256"`
	CreatedBy       string         `gorm:"size:64"`
}

// BlockedProposalIssue represents a proposal that cannot be materialized.
// It only contains diagnostics and repair hints, not a spec.
type BlockedProposalIssue struct {
	gorm.Model
	GameID      string `gorm:"size:64;uniqueIndex:idx_blocked_proposal_scope"`
	Env         string `gorm:"size:64;uniqueIndex:idx_blocked_proposal_scope"`
	ResourceKey string `gorm:"size:64;index"`
	FunctionID  string `gorm:"size:128;index"`

	// Source tracking
	SourceDigests datatypes.JSON `gorm:"type:json"` // SourceDigest array

	// Diagnostics explains why the proposal cannot be materialized
	Diagnostics datatypes.JSON `gorm:"type:json"` // Diagnostic array

	// RepairHint provides guidance on how to resolve the issue
	RepairHint datatypes.JSONMap `gorm:"type:json"` // LocalizedText

	// Status tracks whether this issue has been addressed
	Status string `gorm:"size:32;default:'open'"` // open|resolved|dismissed

	UpdatedAt time.Time
	UpdatedBy string `gorm:"size:64"`
}

// BlockedProposalIssueModel provides data access for blocked proposal issues.
type BlockedProposalIssueModel struct {
	db *gorm.DB
}

// NewBlockedProposalIssueModel creates a new model.
func NewBlockedProposalIssueModel(db *gorm.DB) *BlockedProposalIssueModel {
	return &BlockedProposalIssueModel{db: db}
}

// Create creates a new blocked proposal issue.
func (m *BlockedProposalIssueModel) Create(ctx context.Context, issue *BlockedProposalIssue) error {
	return m.db.WithContext(ctx).Create(issue).Error
}

// FindByScopeAndResourceKey finds a blocked issue by scope and resource key.
func (m *BlockedProposalIssueModel) FindByScopeAndResourceKey(ctx context.Context, gameID, env, resourceKey string) (*BlockedProposalIssue, error) {
	var issue BlockedProposalIssue
	if err := m.db.WithContext(ctx).
		Where("game_id = ? AND env = ? AND resource_key = ? AND status = ?", gameID, env, resourceKey, "open").
		First(&issue).Error; err != nil {
		return nil, err
	}
	return &issue, nil
}

// ListByScope lists all blocked issues in a scope.
func (m *BlockedProposalIssueModel) ListByScope(ctx context.Context, gameID, env string) ([]*BlockedProposalIssue, error) {
	var issues []*BlockedProposalIssue
	if err := m.db.WithContext(ctx).
		Where("game_id = ? AND env = ? AND status = ?", gameID, env, "open").
		Order("created_at DESC").
		Find(&issues).Error; err != nil {
		return nil, err
	}
	return issues, nil
}

// UpdateStatus updates the status of a blocked issue.
func (m *BlockedProposalIssueModel) UpdateStatus(ctx context.Context, id uint, status, updatedBy string) error {
	return m.db.WithContext(ctx).
		Model(&BlockedProposalIssue{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"status":     status,
			"updated_by": updatedBy,
		}).Error
}
