package model

import (
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// PageProposal represents a generated page suggestion before user confirmation.
// It is the output of the generator from FunctionContract/CapabilitySemantics.
type PageProposal struct {
	gorm.Model
	GameID          string `gorm:"size:64;uniqueIndex:idx_page_proposal_scope"`
	Env             string `gorm:"size:64;uniqueIndex:idx_page_proposal_scope"`
	ProposalKey     string `gorm:"size:128;uniqueIndex:idx_page_proposal_scope"` // resourceKey or functionId
	PageKey         string `gorm:"size:128;index"`
	PageType        string `gorm:"size:32"` // resource|operation|task|report
	ResourceKey     string `gorm:"size:64;index"`
	Quality         string `gorm:"size:32"` // ready|basic|needs_review|blocked
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
	ProposalID      uint   `gorm:"index"`
	Version         int
	Proposal        datatypes.JSON `gorm:"type:json"` // Snapshot of PageProposal
	FunctionDigest  string         `gorm:"size:64"`
	SemanticsDigest string         `gorm:"size:64"`
	ChangeReason    string         `gorm:"size:256"`
	CreatedBy       string         `gorm:"size:64"`
}
