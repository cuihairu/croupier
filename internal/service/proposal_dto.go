package service

import (
	"time"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/model"
)

// ProposalAcceptOutcome accept 结果（auto env 下含接续发布结果）。
type ProposalAcceptOutcome struct {
	// Published auto 策略下 accept 落 draft 后已直接发布。
	Published bool
	// PublishError 非空 = auto 发布被质量门槛拒绝或失败（accept 本身已
	// 成功不回滚，draft 保留，可走手动发布重试）。
	PublishError string
}

// ProposalListFilter controls proposal query filters exposed by HTTP API.
type ProposalListFilter struct {
	Status      string
	ResourceKey string
}

// ProposalInboxResponse is the Page Studio entry data model. It keeps stale
// and blocked states outside PageProposal.quality.
type ProposalInboxResponse struct {
	Publishable     []ProposalDTO             `json:"publishable"`
	NeedsReview     []ProposalDTO             `json:"needsReview"`
	BlockedIssues   []BlockedProposalIssueDTO `json:"blockedIssues"`
	ContractChanges []ContractChangeDTO       `json:"contractChanges"`
	Summary         ProposalInboxSummary      `json:"summary"`
}

// ProposalInboxSummary exposes queue counts without forcing clients to infer
// product state from mixed rows.
type ProposalInboxSummary struct {
	Publishable     int `json:"publishable"`
	NeedsReview     int `json:"needsReview"`
	BlockedIssues   int `json:"blockedIssues"`
	ContractChanges int `json:"contractChanges"`
}

// BlockedProposalIssueDTO is a non-materialized generation issue. It has no
// PageSpec by design.
type BlockedProposalIssueDTO struct {
	ID            uint               `json:"id"`
	GameID        string             `json:"gameId,omitempty"`
	Env           string             `json:"env,omitempty"`
	ResourceKey   string             `json:"resourceKey,omitempty"`
	FunctionID    string             `json:"functionId,omitempty"`
	SourceDigests []string           `json:"sourceDigests,omitempty"`
	Diagnostics   []spec.Diagnostic  `json:"diagnostics,omitempty"`
	RepairHint    spec.LocalizedText `json:"repairHint,omitempty"`
	Status        string             `json:"status"`
	UpdatedAt     string             `json:"updatedAt"`
	UpdatedBy     string             `json:"updatedBy,omitempty"`
}

// ContractChangeDTO represents a stale draft or published page that requires
// explicit review and re-publication.
type ContractChangeDTO struct {
	PageKey          string                            `json:"pageKey"`
	PageType         spec.PageType                     `json:"pageType"`
	ResourceKey      string                            `json:"resourceKey,omitempty"`
	Title            spec.LocalizedText                `json:"title,omitempty"`
	CategoryKey      string                            `json:"categoryKey,omitempty"`
	Kind             string                            `json:"kind"` // draft|published
	Status           string                            `json:"status,omitempty"`
	DraftRevision    int                               `json:"draftRevision,omitempty"`
	PublishedVersion int                               `json:"publishedVersion,omitempty"`
	BindingFreshness []spec.BindingFreshnessDiagnostic `json:"bindingFreshness,omitempty"`
	UpdatedAt        string                            `json:"updatedAt,omitempty"`
	UpdatedBy        string                            `json:"updatedBy,omitempty"`
}

// ProposalDTO is the stable HTTP representation of PageProposal.
type ProposalDTO struct {
	ID               uint               `json:"id"`
	GameID           string             `json:"gameId,omitempty"`
	Env              string             `json:"env,omitempty"`
	ProposalKey      string             `json:"proposalKey"`
	PageKey          string             `json:"pageKey"`
	PageType         spec.PageType      `json:"pageType"`
	ResourceKey      string             `json:"resourceKey,omitempty"`
	Quality          string             `json:"quality"`
	GeneratorVersion string             `json:"generatorVersion"`
	FunctionDigest   string             `json:"functionDigest,omitempty"`
	SemanticsDigest  string             `json:"semanticsDigest,omitempty"`
	Title            spec.LocalizedText `json:"title"`
	Description      spec.LocalizedText `json:"description,omitempty"`
	CategoryKey      string             `json:"categoryKey,omitempty"`
	PageSpec         spec.PageSpec      `json:"pageSpec"`
	Diagnostics      []spec.Diagnostic  `json:"diagnostics,omitempty"`
	Status           string             `json:"status"`
	// PageExists marks that the proposal's target page has already been
	// materialized (draft or published). Inbox UIs must offer the Page Studio
	// edit/republish flow instead of accept-and-publish, which 409s on
	// existing pages by design.
	PageExists bool   `json:"pageExists"`
	UpdatedAt  string `json:"updatedAt"`
	UpdatedBy  string `json:"updatedBy,omitempty"`
}

func proposalDTOFromModel(proposal *model.PageProposal) (ProposalDTO, error) {
	if proposal == nil {
		return ProposalDTO{}, errorx.NewBadRequest("proposal is required")
	}
	pageSpec, _, err := pageSpecFromProposal(proposal)
	if err != nil {
		return ProposalDTO{}, err
	}
	return ProposalDTO{
		ID:               proposal.ID,
		GameID:           proposal.GameID,
		Env:              proposal.Env,
		ProposalKey:      proposal.ProposalKey,
		PageKey:          proposal.PageKey,
		PageType:         spec.PageType(proposal.PageType),
		ResourceKey:      proposal.ResourceKey,
		Quality:          proposal.Quality,
		GeneratorVersion: proposal.GeneratorVersion,
		FunctionDigest:   proposal.FunctionDigest,
		SemanticsDigest:  proposal.SemanticsDigest,
		Title:            normalizeJSONMap(proposal.Title),
		Description:      normalizeJSONMap(proposal.Description),
		CategoryKey:      proposal.CategoryKey,
		PageSpec:         pageSpec,
		Diagnostics:      diagnosticsFromJSON(proposal.Diagnostics),
		Status:           proposal.Status.String(),
		UpdatedAt:        proposal.UpdatedAt.Format(time.RFC3339),
		UpdatedBy:        proposal.UpdatedBy,
	}, nil
}
