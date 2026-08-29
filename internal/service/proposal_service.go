package service

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/dashboard/freshness"
	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/db/dbctx"
	"github.com/cuihairu/croupier/internal/dbenum"
	logicutils "github.com/cuihairu/croupier/internal/logic/utils"
	"github.com/cuihairu/croupier/internal/model"
	"gorm.io/gorm"
)

// ProposalService manages page proposals.
type ProposalService struct {
	db             *gorm.DB
	proposalModel  *model.PageProposalModel
	pageModel      *model.PageSpecModel
	versionModel   *model.PageVersionModel
	contractModel  *model.FunctionContractModel
	publishedModel *model.PublishedPageSpecModel
	blockedModel   *model.BlockedProposalIssueModel
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

// NewProposalService creates the service.
func NewProposalService(db *gorm.DB) *ProposalService {
	return &ProposalService{
		db:             db,
		proposalModel:  model.NewPageProposalModel(db),
		pageModel:      model.NewPageSpecModel(db),
		versionModel:   model.NewPageVersionModel(db),
		contractModel:  model.NewFunctionContractModel(db),
		publishedModel: model.NewPublishedPageSpecModel(db),
		blockedModel:   model.NewBlockedProposalIssueModel(db),
	}
}

// ListProposals lists all proposals in a scope.
func (s *ProposalService) ListProposals(ctx context.Context, gameID, env string) ([]*model.PageProposal, error) {
	return s.proposalModel.ListByScope(ctx, gameID, env)
}

// ListProposalDTOs lists proposals using stable API DTOs.
func (s *ProposalService) ListProposalDTOs(ctx context.Context, gameID, env string, filter ProposalListFilter) ([]ProposalDTO, error) {
	resourceKey := strings.TrimSpace(filter.ResourceKey)
	status := dbenum.ProposalStatus(-1)
	if raw := strings.TrimSpace(filter.Status); raw != "" {
		if parsed, err := dbenum.ParseProposalStatus(strings.ToLower(raw)); err == nil {
			status = parsed
		}
	}

	var proposals []*model.PageProposal
	var err error
	switch {
	case status >= 0 && resourceKey != "":
		proposals, err = s.proposalModel.ListByScopeStatusAndResourceKey(ctx, gameID, env, status, resourceKey)
	case status >= 0:
		proposals, err = s.proposalModel.ListByStatus(ctx, gameID, env, status)
	case resourceKey != "":
		proposals, err = s.proposalModel.ListByScopeAndResourceKey(ctx, gameID, env, resourceKey)
	default:
		proposals, err = s.proposalModel.ListByScope(ctx, gameID, env)
	}
	if err != nil {
		return nil, err
	}

	items := make([]ProposalDTO, 0, len(proposals))
	for _, proposal := range proposals {
		item, err := proposalDTOFromModel(proposal)
		if err != nil {
			return nil, err
		}
		// 页面是否已物化：Inbox 需要据此把「发布」换成「去 Page Studio」。
		if _, err := s.pageModel.FindByScopeAndPageKey(ctx, gameID, env, proposal.PageKey); err == nil {
			item.PageExists = true
		}
		items = append(items, item)
	}
	return items, nil
}

// Inbox returns the canonical Page Studio queues:
// - publishable ready/basic proposals
// - needs_review proposals
// - blocked generation issues
// - stale draft/published pages caused by contract drift
func (s *ProposalService) Inbox(ctx context.Context, gameID, env string, filter ProposalListFilter) (ProposalInboxResponse, error) {
	proposals, err := s.ListProposalDTOs(ctx, gameID, env, filter)
	if err != nil {
		return ProposalInboxResponse{}, err
	}

	resp := ProposalInboxResponse{
		Publishable:     []ProposalDTO{},
		NeedsReview:     []ProposalDTO{},
		BlockedIssues:   []BlockedProposalIssueDTO{},
		ContractChanges: []ContractChangeDTO{},
	}
	for _, proposal := range proposals {
		if proposal.Status != "pending" {
			continue
		}
		if proposal.Quality == "ready" || proposal.Quality == "basic" {
			if !hasDiagnosticSeverity(proposal.Diagnostics, spec.SeverityError) {
				resp.Publishable = append(resp.Publishable, proposal)
				continue
			}
		}
		resp.NeedsReview = append(resp.NeedsReview, proposal)
	}

	blocked, err := s.listBlockedIssueDTOs(ctx, gameID, env, filter.ResourceKey)
	if err != nil {
		return ProposalInboxResponse{}, err
	}
	resp.BlockedIssues = blocked

	contractChanges, err := s.listContractChanges(ctx, gameID, env, filter.ResourceKey)
	if err != nil {
		return ProposalInboxResponse{}, err
	}
	resp.ContractChanges = contractChanges
	resp.Summary = ProposalInboxSummary{
		Publishable:     len(resp.Publishable),
		NeedsReview:     len(resp.NeedsReview),
		BlockedIssues:   len(resp.BlockedIssues),
		ContractChanges: len(resp.ContractChanges),
	}
	return resp, nil
}

// ListProposalsByStatus lists proposals by status.
func (s *ProposalService) ListProposalsByStatus(ctx context.Context, gameID, env string, status dbenum.ProposalStatus) ([]*model.PageProposal, error) {
	return s.proposalModel.ListByStatus(ctx, gameID, env, status)
}

// GetProposal gets a proposal by key.
func (s *ProposalService) GetProposal(ctx context.Context, gameID, env, proposalKey string) (*model.PageProposal, error) {
	return s.proposalModel.FindByScopeAndKey(ctx, gameID, env, proposalKey)
}

// GetProposalDTO gets a proposal by key using the stable API DTO.
func (s *ProposalService) GetProposalDTO(ctx context.Context, gameID, env, proposalKey string) (ProposalDTO, error) {
	proposal, err := s.proposalModel.FindByScopeAndKey(ctx, gameID, env, proposalKey)
	if err != nil {
		return ProposalDTO{}, err
	}
	return proposalDTOFromModel(proposal)
}

// AcceptProposal accepts a proposal and materializes it as a PageSpec draft.
// Proposals with error-level diagnostics cannot be accepted.
func (s *ProposalService) AcceptProposal(ctx context.Context, gameID, env, proposalKey string) error {
	proposal, err := s.proposalModel.FindByScopeAndKey(ctx, gameID, env, proposalKey)
	if err != nil {
		return fmt.Errorf("proposal not found: %w", err)
	}

	if proposal.Status != dbenum.ProposalStatusPending {
		return fmt.Errorf("proposal is not pending")
	}

	// Block acceptance if there are error-level diagnostics
	if hasBlockingDiagnostics(proposal.Diagnostics) {
		return fmt.Errorf("proposal has blocking diagnostics; resolve errors before accepting")
	}

	pageSpec, specJSON, err := pageSpecFromProposal(proposal)
	if err != nil {
		return err
	}
	if err := validateAcceptedPageSpec(gameID, env, proposal, pageSpec); err != nil {
		return err
	}

	actor := actorFromContext(ctx)
	now := time.Now()
	db := dbctx.Resolve(ctx, s.db)
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		txCtx := dbctx.WithDB(ctx, tx)
		pageModel := model.NewPageSpecModel(tx)
		versionModel := model.NewPageVersionModel(tx)
		proposalModel := model.NewPageProposalModel(tx)

		if _, err := pageModel.FindByScopeAndPageKey(txCtx, gameID, env, pageSpec.PageKey); err == nil {
			return errorx.NewConflict("page draft already exists; edit the draft or reject/regenerate the proposal")
		} else if err != nil && err != gorm.ErrRecordNotFound {
			return err
		}

		proposalSnapshotVersion, err := createProposalVersionSnapshot(txCtx, model.NewPageProposalVersionModel(tx), proposal, "accept generated proposal", actor)
		if err != nil {
			return err
		}
		draft := &model.PageSpec{
			GameID:              gameID,
			Env:                 env,
			PageKey:             strings.TrimSpace(pageSpec.PageKey),
			Type:                string(pageSpec.Type),
			ResourceKey:         strings.TrimSpace(pageSpec.ResourceKey),
			CategoryKey:         strings.TrimSpace(pageSpec.Category.Key),
			CategoryOrder:       pageSpec.Category.Order,
			Order:               pageSpec.Order,
			Icon:                strings.TrimSpace(pageSpec.Icon),
			SpecJSON:            specJSON,
			Status:              "draft",
			DraftRevision:       1,
			BaseProposalKey:     strings.TrimSpace(proposal.ProposalKey),
			BaseProposalVersion: proposalSnapshotVersion,
			UpdatedBy:           actor,
			CreatedAt:           now,
			UpdatedAt:           now,
		}
		if err := draft.SetTitle(normalizeLocalizedText(pageSpec.Title)); err != nil {
			return err
		}
		if err := draft.SetCategoryLabels(normalizeLocalizedText(pageSpec.Category.Labels)); err != nil {
			return err
		}
		if err := pageModel.Upsert(txCtx, draft); err != nil {
			return fmt.Errorf("create page draft from proposal: %w", err)
		}
		if err := versionModel.UpsertByScopePageKeyVersion(txCtx, &model.PageVersion{
			GameID:    gameID,
			Env:       env,
			PageKey:   draft.PageKey,
			Version:   draft.DraftRevision,
			SpecJSON:  specJSON,
			Status:    "draft",
			Message:   "accept generated proposal " + strings.TrimSpace(proposal.ProposalKey),
			CreatedBy: actor,
			CreatedAt: now,
		}); err != nil {
			return fmt.Errorf("create page draft version from proposal: %w", err)
		}
		proposal.Status = dbenum.ProposalStatusAccepted
		proposal.UpdatedBy = actor
		proposal.UpdatedAt = now
		return proposalModel.UpsertProposal(txCtx, proposal)
	})
}

// AcceptAndPublishProposal materializes a ready/basic proposal as a draft and
// immediately publishes the same immutable PageSpec snapshot.
func (s *ProposalService) AcceptAndPublishProposal(ctx context.Context, gameID, env, proposalKey string) (ProposalPublishResult, error) {
	proposal, err := s.proposalModel.FindByScopeAndKey(ctx, gameID, env, proposalKey)
	if err != nil {
		return ProposalPublishResult{}, fmt.Errorf("proposal not found: %w", err)
	}
	if proposal.Status != dbenum.ProposalStatusPending {
		return ProposalPublishResult{}, fmt.Errorf("proposal is not pending")
	}
	if proposal.Quality != "ready" && proposal.Quality != "basic" {
		return ProposalPublishResult{}, errorx.NewValidationError("only ready/basic proposals can be published directly")
	}
	if hasBlockingDiagnostics(proposal.Diagnostics) {
		return ProposalPublishResult{}, fmt.Errorf("proposal has blocking diagnostics; resolve errors before publishing")
	}

	pageSpec, specJSON, err := pageSpecFromProposal(proposal)
	if err != nil {
		return ProposalPublishResult{}, err
	}
	if err := s.validateDirectPublishPageSpec(ctx, gameID, env, proposal, pageSpec); err != nil {
		return ProposalPublishResult{}, err
	}
	contracts, err := s.buildBindingContracts(ctx, gameID, env, pageSpec.Bindings)
	if err != nil {
		return ProposalPublishResult{}, err
	}
	contractsJSON, err := json.Marshal(contracts)
	if err != nil {
		return ProposalPublishResult{}, err
	}

	actor := actorFromContext(ctx)
	now := time.Now()
	result := ProposalPublishResult{
		PageKey:          pageSpec.PageKey,
		DraftRevision:    1,
		PublishedVersion: 1,
	}
	db := dbctx.Resolve(ctx, s.db)
	err = db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		txCtx := dbctx.WithDB(ctx, tx)
		pageModel := model.NewPageSpecModel(tx)
		publishedModel := model.NewPublishedPageSpecModel(tx)
		versionModel := model.NewPageVersionModel(tx)
		proposalModel := model.NewPageProposalModel(tx)
		proposalVersionModel := model.NewPageProposalVersionModel(tx)

		existing, err := pageModel.FindByScopeAndPageKey(txCtx, gameID, env, pageSpec.PageKey)
		if err != nil && err != gorm.ErrRecordNotFound {
			return err
		}
		if existing != nil {
			return errorx.NewConflict("page already exists; use Page Studio edit/publish flow")
		}
		nextVersion, err := versionModel.GetNextVersion(txCtx, gameID, env, pageSpec.PageKey)
		if err != nil {
			return err
		}
		result.DraftRevision = nextVersion
		result.PublishedVersion = nextVersion

		proposalSnapshotVersion, err := createProposalVersionSnapshot(txCtx, proposalVersionModel, proposal, "accept and publish", actor)
		if err != nil {
			return err
		}
		draft := &model.PageSpec{
			GameID:              gameID,
			Env:                 env,
			PageKey:             strings.TrimSpace(pageSpec.PageKey),
			Type:                string(pageSpec.Type),
			ResourceKey:         strings.TrimSpace(pageSpec.ResourceKey),
			CategoryKey:         strings.TrimSpace(pageSpec.Category.Key),
			CategoryOrder:       pageSpec.Category.Order,
			Order:               pageSpec.Order,
			Icon:                strings.TrimSpace(pageSpec.Icon),
			SpecJSON:            specJSON,
			Status:              "published",
			PublishedActive:     true,
			DraftRevision:       nextVersion,
			PublishedVersion:    nextVersion,
			BaseProposalKey:     strings.TrimSpace(proposal.ProposalKey),
			BaseProposalVersion: proposalSnapshotVersion,
			UpdatedBy:           actor,
			CreatedAt:           now,
			UpdatedAt:           now,
		}
		if err := draft.SetTitle(normalizeLocalizedText(pageSpec.Title)); err != nil {
			return err
		}
		if err := draft.SetCategoryLabels(normalizeLocalizedText(pageSpec.Category.Labels)); err != nil {
			return err
		}
		if err := publishedModel.DeactivatePage(txCtx, gameID, env, pageSpec.PageKey, now); err != nil {
			return err
		}
		if err := publishedModel.Create(txCtx, &model.PublishedPageSpec{
			GameID:                gameID,
			Env:                   env,
			PageKey:               pageSpec.PageKey,
			Version:               nextVersion,
			SpecJSON:              specJSON,
			BindingContractsJSON:  string(contractsJSON),
			RendererSchemaVersion: rendererSchemaVersion,
			BaseProposalKey:       strings.TrimSpace(proposal.ProposalKey),
			BaseProposalVersion:   proposalSnapshotVersion,
			FunctionDigest:        strings.TrimSpace(proposal.FunctionDigest),
			SemanticsDigest:       strings.TrimSpace(proposal.SemanticsDigest),
			GeneratorVersion:      strings.TrimSpace(proposal.GeneratorVersion),
			Active:                true,
			PublishedAt:           now,
			PublishedBy:           actor,
		}); err != nil {
			return err
		}
		if err := pageModel.Upsert(txCtx, draft); err != nil {
			return fmt.Errorf("create page draft from proposal: %w", err)
		}
		if err := versionModel.UpsertByScopePageKeyVersion(txCtx, &model.PageVersion{
			GameID:    gameID,
			Env:       env,
			PageKey:   draft.PageKey,
			Version:   nextVersion,
			SpecJSON:  specJSON,
			Status:    "published",
			Message:   "accept and publish generated proposal " + strings.TrimSpace(proposal.ProposalKey),
			CreatedBy: actor,
			CreatedAt: now,
		}); err != nil {
			return fmt.Errorf("create page published version from proposal: %w", err)
		}
		proposal.Status = dbenum.ProposalStatusAccepted
		proposal.UpdatedBy = actor
		proposal.UpdatedAt = now
		return proposalModel.UpsertProposal(txCtx, proposal)
	})
	if err != nil {
		return ProposalPublishResult{}, err
	}
	return result, nil
}

// RejectProposal rejects a proposal.
func (s *ProposalService) RejectProposal(ctx context.Context, gameID, env, proposalKey string) error {
	proposal, err := s.proposalModel.FindByScopeAndKey(ctx, gameID, env, proposalKey)
	if err != nil {
		return fmt.Errorf("proposal not found: %w", err)
	}

	if proposal.Status != dbenum.ProposalStatusPending {
		return fmt.Errorf("proposal is not pending")
	}

	// Update status to rejected
	proposal.Status = dbenum.ProposalStatusRejected
	return s.proposalModel.UpsertProposal(ctx, proposal)
}

// hasBlockingDiagnostics checks if diagnostics contain error-level items.
func hasBlockingDiagnostics(diagnosticsJSON []byte) bool {
	if len(diagnosticsJSON) == 0 {
		return false
	}

	var diagnostics []struct {
		Severity string `json:"severity"`
	}
	if err := json.Unmarshal(diagnosticsJSON, &diagnostics); err != nil {
		return false
	}

	for _, d := range diagnostics {
		if d.Severity == "error" {
			return true
		}
	}
	return false
}

func createProposalVersionSnapshot(
	ctx context.Context,
	versionModel *model.PageProposalVersionModel,
	proposal *model.PageProposal,
	reason string,
	actor string,
) (int, error) {
	if proposal == nil || proposal.ID == 0 {
		return 0, errorx.NewValidationError("proposal must be persisted before snapshot")
	}
	nextVersion, err := versionModel.GetNextVersion(ctx, proposal.ID)
	if err != nil {
		return 0, err
	}
	proposalJSON, err := json.Marshal(proposal)
	if err != nil {
		return 0, err
	}
	if err := versionModel.CreateVersion(ctx, &model.PageProposalVersion{
		ProposalID:      proposal.ID,
		Version:         nextVersion,
		Proposal:        proposalJSON,
		FunctionDigest:  proposal.FunctionDigest,
		SemanticsDigest: proposal.SemanticsDigest,
		ChangeReason:    reason,
		CreatedBy:       actor,
	}); err != nil {
		return 0, err
	}
	return nextVersion, nil
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

func diagnosticsFromJSON(raw []byte) []spec.Diagnostic {
	if len(raw) == 0 {
		return nil
	}
	var diagnostics []spec.Diagnostic
	if err := json.Unmarshal(raw, &diagnostics); err != nil {
		return []spec.Diagnostic{{
			Code:     "proposal_diagnostics_invalid",
			Severity: spec.SeverityWarning,
			Message:  "proposal diagnostics payload is not readable",
		}}
	}
	return diagnostics
}

func hasDiagnosticSeverity(diagnostics []spec.Diagnostic, severity spec.DiagnosticSeverity) bool {
	for _, item := range diagnostics {
		if item.Severity == severity {
			return true
		}
	}
	return false
}

func (s *ProposalService) listBlockedIssueDTOs(ctx context.Context, gameID, env, resourceKey string) ([]BlockedProposalIssueDTO, error) {
	var issues []*model.BlockedProposalIssue
	var err error
	resourceKey = strings.TrimSpace(resourceKey)
	if resourceKey == "" {
		issues, err = s.blockedModel.ListByScope(ctx, gameID, env)
	} else {
		issues, err = s.blockedModel.ListByScopeAndResourceKey(ctx, gameID, env, resourceKey)
	}
	if err != nil {
		if isMissingTableErr(err) {
			return []BlockedProposalIssueDTO{}, nil
		}
		return nil, err
	}
	out := make([]BlockedProposalIssueDTO, 0, len(issues))
	for _, issue := range issues {
		if issue == nil {
			continue
		}
		out = append(out, blockedIssueDTOFromModel(issue))
	}
	return out, nil
}

func blockedIssueDTOFromModel(issue *model.BlockedProposalIssue) BlockedProposalIssueDTO {
	return BlockedProposalIssueDTO{
		ID:            issue.ID,
		GameID:        issue.GameID,
		Env:           issue.Env,
		ResourceKey:   strings.TrimSpace(issue.ResourceKey),
		FunctionID:    strings.TrimSpace(issue.FunctionID),
		SourceDigests: stringSliceFromJSON(issue.SourceDigests),
		Diagnostics:   diagnosticsFromJSON(issue.Diagnostics),
		RepairHint:    normalizeJSONMap(issue.RepairHint),
		Status:        issue.Status,
		UpdatedAt:     issue.UpdatedAt.Format(time.RFC3339),
		UpdatedBy:     issue.UpdatedBy,
	}
}

func (s *ProposalService) listContractChanges(ctx context.Context, gameID, env, resourceKey string) ([]ContractChangeDTO, error) {
	functions, err := s.functionSpecsByID(ctx, gameID, env)
	if err != nil {
		return nil, err
	}
	publishedChanges, err := s.publishedContractChanges(ctx, gameID, env, strings.TrimSpace(resourceKey), functions)
	if err != nil {
		return nil, err
	}
	draftChanges, err := s.draftContractChanges(ctx, gameID, env, strings.TrimSpace(resourceKey), publishedChanges, functions)
	if err != nil {
		return nil, err
	}
	out := append(publishedChanges, draftChanges...)
	sort.Slice(out, func(i, j int) bool {
		if out[i].UpdatedAt != out[j].UpdatedAt {
			return out[i].UpdatedAt > out[j].UpdatedAt
		}
		if out[i].Kind != out[j].Kind {
			return out[i].Kind < out[j].Kind
		}
		return out[i].PageKey < out[j].PageKey
	})
	return out, nil
}

func (s *ProposalService) publishedContractChanges(
	ctx context.Context,
	gameID string,
	env string,
	resourceKey string,
	functions map[string]spec.FunctionSpec,
) ([]ContractChangeDTO, error) {
	published, err := s.publishedModel.ListLatestActiveByScope(ctx, gameID, env)
	if err != nil {
		if isMissingTableErr(err) {
			return []ContractChangeDTO{}, nil
		}
		return nil, err
	}
	out := make([]ContractChangeDTO, 0)
	for _, item := range published {
		page, contracts := parsePublishedSnapshot(item)
		if page.PageKey == "" {
			continue
		}
		if resourceKey != "" && strings.TrimSpace(page.ResourceKey) != resourceKey {
			continue
		}
		diags := freshness.EvaluatePublishedBindings(page.Bindings, contracts, functions)
		if len(diags) == 0 {
			continue
		}
		out = append(out, ContractChangeDTO{
			PageKey:          item.PageKey,
			PageType:         page.Type,
			ResourceKey:      strings.TrimSpace(page.ResourceKey),
			Title:            page.Title,
			CategoryKey:      strings.TrimSpace(page.Category.Key),
			Kind:             "published",
			Status:           "stale",
			PublishedVersion: item.Version,
			BindingFreshness: diags,
			UpdatedAt:        item.PublishedAt.Format(time.RFC3339),
			UpdatedBy:        item.PublishedBy,
		})
	}
	return out, nil
}

func (s *ProposalService) draftContractChanges(
	ctx context.Context,
	gameID string,
	env string,
	resourceKey string,
	publishedChanges []ContractChangeDTO,
	functions map[string]spec.FunctionSpec,
) ([]ContractChangeDTO, error) {
	drafts, err := s.pageModel.ListByScope(ctx, gameID, env)
	if err != nil {
		if isMissingTableErr(err) {
			return []ContractChangeDTO{}, nil
		}
		return nil, err
	}
	publishedStale := make(map[string][]spec.BindingFreshnessDiagnostic, len(publishedChanges))
	for _, change := range publishedChanges {
		publishedStale[change.PageKey] = change.BindingFreshness
	}
	out := make([]ContractChangeDTO, 0)
	for _, draft := range drafts {
		if resourceKey != "" && strings.TrimSpace(draft.ResourceKey) != resourceKey {
			continue
		}
		page := parseDraftPageSpec(draft)
		diags := staleDiagnosticsForDraft(page, functions)
		if len(diags) == 0 && draft.PublishedVersion > 0 {
			diags = publishedStale[draft.PageKey]
		}
		if len(diags) == 0 {
			continue
		}
		out = append(out, ContractChangeDTO{
			PageKey:          draft.PageKey,
			PageType:         spec.PageType(draft.Type),
			ResourceKey:      strings.TrimSpace(draft.ResourceKey),
			Title:            draft.GetTitle(),
			CategoryKey:      strings.TrimSpace(draft.CategoryKey),
			Kind:             "draft",
			Status:           draft.Status,
			DraftRevision:    draft.DraftRevision,
			PublishedVersion: draft.PublishedVersion,
			BindingFreshness: diags,
			UpdatedAt:        draft.UpdatedAt.Format(time.RFC3339),
			UpdatedBy:        draft.UpdatedBy,
		})
	}
	return out, nil
}

func staleDiagnosticsForDraft(page spec.PageSpec, functions map[string]spec.FunctionSpec) []spec.BindingFreshnessDiagnostic {
	var out []spec.BindingFreshnessDiagnostic
	for _, binding := range page.Bindings {
		functionID := strings.TrimSpace(binding.FunctionID)
		fn, ok := functions[functionID]
		if !ok {
			out = append(out, spec.BindingFreshnessDiagnostic{
				BindingID:  strings.TrimSpace(binding.ID),
				FunctionID: functionID,
				Status:     spec.BindingFreshnessFunctionMissing,
				Diagnostic: spec.Diagnostic{
					Code:       "binding_function_missing",
					Severity:   spec.SeverityError,
					Message:    "bound function no longer exists; update the draft binding before publishing",
					FunctionID: functionID,
					Field:      "bindings." + strings.TrimSpace(binding.ID),
				},
			})
			continue
		}
		if executionModeForFunctionSpec(fn) != binding.Execution.Mode {
			out = append(out, spec.BindingFreshnessDiagnostic{
				BindingID:  strings.TrimSpace(binding.ID),
				FunctionID: functionID,
				Status:     spec.BindingFreshnessExecutionModeStale,
				Diagnostic: spec.Diagnostic{
					Code:       "draft_binding_execution_mode_stale",
					Severity:   spec.SeverityError,
					Message:    "draft binding execution mode no longer matches the latest function contract",
					FunctionID: functionID,
					Field:      "bindings." + strings.TrimSpace(binding.ID) + ".execution.mode",
				},
			})
		}
	}
	return out
}

func parsePublishedSnapshot(item model.PublishedPageSpec) (spec.PageSpec, []spec.BindingContractSnapshot) {
	var page spec.PageSpec
	if strings.TrimSpace(item.SpecJSON) != "" {
		_ = json.Unmarshal([]byte(item.SpecJSON), &page)
	}
	if strings.TrimSpace(page.PageKey) == "" {
		page.PageKey = item.PageKey
	}
	var contracts []spec.BindingContractSnapshot
	if strings.TrimSpace(item.BindingContractsJSON) != "" {
		_ = json.Unmarshal([]byte(item.BindingContractsJSON), &contracts)
	}
	return page, contracts
}

func parseDraftPageSpec(item model.PageSpec) spec.PageSpec {
	var page spec.PageSpec
	if strings.TrimSpace(item.SpecJSON) != "" {
		_ = json.Unmarshal([]byte(item.SpecJSON), &page)
	}
	if strings.TrimSpace(page.PageKey) == "" {
		page.PageKey = item.PageKey
		page.Type = spec.PageType(item.Type)
		page.ResourceKey = item.ResourceKey
		page.Title = item.GetTitle()
		page.Category = spec.PageCategorySpec{
			Key:    item.CategoryKey,
			Labels: item.GetCategoryLabels(),
			Order:  item.CategoryOrder,
		}
	}
	return page
}

func stringSliceFromJSON(raw []byte) []string {
	if len(raw) == 0 {
		return nil
	}
	var values []string
	if err := json.Unmarshal(raw, &values); err != nil {
		return nil
	}
	out := values[:0]
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			out = append(out, value)
		}
	}
	return out
}

func normalizeJSONMap(values map[string]interface{}) spec.LocalizedText {
	if len(values) == 0 {
		return nil
	}
	out := make(spec.LocalizedText, len(values))
	for key, value := range values {
		if text, ok := value.(string); ok && strings.TrimSpace(text) != "" {
			out[key] = strings.TrimSpace(text)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// ProposalPublishResult is returned when a proposal is directly published.
type ProposalPublishResult struct {
	PageKey          string `json:"pageKey"`
	DraftRevision    int    `json:"draftRevision"`
	PublishedVersion int    `json:"publishedVersion"`
}

func pageSpecFromProposal(proposal *model.PageProposal) (spec.PageSpec, string, error) {
	if proposal == nil {
		return spec.PageSpec{}, "", errorx.NewBadRequest("proposal is required")
	}
	if len(proposal.PageSpec) == 0 {
		return spec.PageSpec{}, "", errorx.NewValidationError("proposal does not contain canonical PageSpec")
	}
	var pageSpec spec.PageSpec
	if err := json.Unmarshal(proposal.PageSpec, &pageSpec); err != nil {
		return spec.PageSpec{}, "", errorx.NewValidationError("proposal PageSpec is invalid JSON")
	}
	pageSpec.PageKey = strings.TrimSpace(pageSpec.PageKey)
	pageSpec.ResourceKey = strings.TrimSpace(pageSpec.ResourceKey)
	pageSpec.Icon = strings.TrimSpace(pageSpec.Icon)
	pageSpec.Title = normalizeLocalizedText(pageSpec.Title)
	pageSpec.Description = normalizeLocalizedText(pageSpec.Description)
	pageSpec.Category.Key = strings.TrimSpace(pageSpec.Category.Key)
	pageSpec.Category.Labels = normalizeLocalizedText(pageSpec.Category.Labels)
	for i := range pageSpec.Bindings {
		pageSpec.Bindings[i].ID = strings.TrimSpace(pageSpec.Bindings[i].ID)
		pageSpec.Bindings[i].FunctionID = strings.TrimSpace(pageSpec.Bindings[i].FunctionID)
	}
	raw, err := json.Marshal(pageSpec)
	if err != nil {
		return spec.PageSpec{}, "", err
	}
	return pageSpec, string(raw), nil
}

func validateAcceptedPageSpec(gameID, env string, proposal *model.PageProposal, page spec.PageSpec) error {
	details := map[string]string{}
	if strings.TrimSpace(gameID) == "" {
		details["gameId"] = "game scope is required"
	}
	if strings.TrimSpace(env) == "" {
		details["env"] = "environment scope is required"
	}
	if strings.TrimSpace(page.PageKey) == "" {
		details["pageKey"] = "pageKey is required"
	}
	if strings.TrimSpace(proposal.PageKey) != "" && strings.TrimSpace(proposal.PageKey) != strings.TrimSpace(page.PageKey) {
		details["pageKey"] = "proposal pageKey must match PageSpec pageKey"
	}
	if !isValidProposalPageType(page.Type) {
		details["type"] = "type must be resource, operation, task, or report"
	}
	if !hasDefaultLocale(page.Title) {
		details["title"] = "title must include zh-CN locale"
	}
	if strings.TrimSpace(page.Category.Key) == "" {
		details["category.key"] = "category.key is required"
	}
	if !hasDefaultLocale(page.Category.Labels) {
		details["category.labels"] = "category.labels must include zh-CN locale"
	}
	if len(page.Bindings) == 0 {
		details["bindings"] = "page must bind at least one function"
	}
	if !pageShapeMatchesType(page) {
		details["shape"] = "page type must include the matching page body"
	}
	for i, binding := range page.Bindings {
		prefix := fmt.Sprintf("bindings[%d]", i)
		if strings.TrimSpace(binding.ID) == "" {
			details[prefix+".id"] = "binding.id is required"
		}
		if strings.TrimSpace(binding.FunctionID) == "" {
			details[prefix+".functionId"] = "binding.functionId is required"
		}
		if !isValidBindingUsage(binding.Usage) {
			details[prefix+".usage"] = "binding.usage is invalid"
		}
		if !isValidExecutionMode(binding.Execution.Mode) {
			details[prefix+".execution.mode"] = "binding.execution.mode is invalid"
		}
	}
	if len(details) > 0 {
		return errorx.NewValidationErrorWithDetails("proposal PageSpec validation failed", details)
	}
	return nil
}

func (s *ProposalService) validateDirectPublishPageSpec(ctx context.Context, gameID, env string, proposal *model.PageProposal, page spec.PageSpec) error {
	if err := validateAcceptedPageSpec(gameID, env, proposal, page); err != nil {
		return err
	}
	details := map[string]string{}
	for _, diag := range spec.ValidatePublishablePageShape(page) {
		field := strings.TrimSpace(diag.Field)
		if field == "" {
			field = strings.TrimSpace(diag.Code)
		}
		details[field] = diag.Message
	}
	functions, err := s.functionSpecsByID(ctx, gameID, env)
	if err != nil {
		return err
	}
	for i, binding := range page.Bindings {
		field := fmt.Sprintf("bindings[%d]", i)
		functionID := strings.TrimSpace(binding.FunctionID)
		fn, ok := functions[functionID]
		if !ok {
			details[field+".functionId"] = "bound function contract does not exist"
			continue
		}
		if !fn.Enabled {
			details[field+".functionId"] = "bound function is disabled"
		}
		requiresInputSelectors := schemaHasFields(fn.InputSchema)
		requiresOutputSelectors := bindingRequiresOutputSelectors(binding, page)
		if requiresInputSelectors && binding.Selectors == nil {
			details[field+".selectors.input"] = "binding.selectors.input is required before publish"
			continue
		}
		if requiresOutputSelectors && binding.Selectors == nil {
			details[field+".selectors.output"] = "binding.selectors.output is required before publish"
			continue
		}
		if binding.Selectors == nil {
			continue
		}
		if requiresInputSelectors {
			inputResult := spec.ValidateSelector(binding.Selectors.Input, fn.InputSchema, spec.SelectorContextForBinding(page, binding))
			for _, item := range inputResult.Errors {
				details[field+".selectors.input."+item.Field] = item.Message
			}
		}
		if requiresOutputSelectors && len(binding.Selectors.Output) == 0 {
			details[field+".selectors.output"] = "binding.selectors.output is required before publish"
		}
		outputResult := spec.ValidateOutputAssignments(binding.Selectors.Output, fn.OutputSchema)
		for _, item := range outputResult.Errors {
			details[field+".selectors.output."+item.Field] = item.Message
		}
		for _, diag := range spec.ValidateRequiredOutputAssignments(binding, page) {
			details[field+".selectors.output"] = diag.Message
		}
	}
	if err := s.validateCategoryLabelConflict(ctx, gameID, env, page); err != nil {
		return err
	}
	if len(details) > 0 {
		return errorx.NewValidationErrorWithDetails("proposal PageSpec publish validation failed", details)
	}
	return nil
}

func (s *ProposalService) functionSpecsByID(ctx context.Context, gameID, env string) (map[string]spec.FunctionSpec, error) {
	contracts, err := s.contractModel.ListByScope(ctx, gameID, env)
	if err != nil {
		return nil, fmt.Errorf("list function contracts: %w", err)
	}
	return FunctionSpecsFromContracts(contracts), nil
}

func (s *ProposalService) validateCategoryLabelConflict(ctx context.Context, gameID, env string, page spec.PageSpec) error {
	if s.publishedModel == nil {
		return nil
	}
	categoryKey := strings.TrimSpace(page.Category.Key)
	if categoryKey == "" {
		return nil
	}
	published, err := s.publishedModel.ListLatestActiveByScope(ctx, gameID, env)
	if err != nil {
		return fmt.Errorf("list published pages: %w", err)
	}
	expected := normalizeLocalizedText(page.Category.Labels)
	for _, item := range published {
		if item.PageKey == page.PageKey {
			continue
		}
		var publishedPage spec.PageSpec
		if strings.TrimSpace(item.SpecJSON) != "" {
			_ = json.Unmarshal([]byte(item.SpecJSON), &publishedPage)
		}
		if strings.TrimSpace(publishedPage.Category.Key) != categoryKey {
			continue
		}
		if !localizedTextEqual(normalizeLocalizedText(publishedPage.Category.Labels), expected) {
			return errorx.NewValidationErrorWithDetails("category labels conflict", map[string]string{
				"category.labels": "category.labels must match existing published pages in the same category",
			})
		}
	}
	return nil
}

func isValidProposalPageType(pageType spec.PageType) bool {
	switch pageType {
	case spec.PageTypeResource, spec.PageTypeOperation, spec.PageTypeTask, spec.PageTypeReport,
		spec.PageTypeComposite:
		return true
	default:
		return false
	}
}

func pageShapeMatchesType(page spec.PageSpec) bool {
	switch page.Type {
	case spec.PageTypeResource:
		return page.Resource != nil
	case spec.PageTypeOperation:
		return page.Operation != nil
	case spec.PageTypeTask:
		return page.Task != nil
	case spec.PageTypeReport:
		return page.Report != nil
	case spec.PageTypeComposite:
		return page.Composite != nil && len(page.Composite.Sections) > 0
	default:
		return false
	}
}

func isValidBindingUsage(usage spec.PageBindingUsage) bool {
	switch usage {
	case spec.BindingUsageQuery,
		spec.BindingUsageDetail,
		spec.BindingUsageAction,
		spec.BindingUsageTask,
		spec.BindingUsageTaskStatus,
		spec.BindingUsageTaskEvents,
		spec.BindingUsageTaskResult,
		spec.BindingUsageTaskCancel,
		spec.BindingUsageTaskRetry,
		spec.BindingUsageReport:
		return true
	default:
		return false
	}
}

func isValidExecutionMode(mode spec.PageExecutionMode) bool {
	switch mode {
	case spec.PageExecutionModeSync, spec.PageExecutionModeTask:
		return true
	default:
		return false
	}
}

func executionModeForFunctionSpec(fn spec.FunctionSpec) spec.PageExecutionMode {
	if fn.Execution == spec.FunctionExecutionTask {
		return spec.PageExecutionModeTask
	}
	return spec.PageExecutionModeSync
}

func isMissingTableErr(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "no such table") ||
		strings.Contains(message, "does not exist") ||
		strings.Contains(message, "undefined table")
}

func normalizeLocalizedText(input map[string]string) map[string]string {
	if len(input) == 0 {
		return nil
	}
	out := make(map[string]string, len(input))
	for k, v := range input {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		switch strings.ToLower(strings.ReplaceAll(k, "_", "-")) {
		case "zh", "zh-cn":
			out["zh-CN"] = v
		case "en", "en-us":
			out["en-US"] = v
		default:
			out[k] = v
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func hasDefaultLocale(labels map[string]string) bool {
	return labels != nil && strings.TrimSpace(labels["zh-CN"]) != ""
}

func localizedTextEqual(left map[string]string, right map[string]string) bool {
	left = normalizeLocalizedText(left)
	right = normalizeLocalizedText(right)
	if len(left) != len(right) {
		return false
	}
	for key, leftValue := range left {
		if strings.TrimSpace(right[key]) != strings.TrimSpace(leftValue) {
			return false
		}
	}
	return true
}

func bindingRequiresOutputSelectors(binding spec.PageFunctionBinding, page spec.PageSpec) bool {
	switch binding.Usage {
	case spec.BindingUsageQuery:
		return page.Type == spec.PageTypeResource
	case spec.BindingUsageReport:
		return page.Type == spec.PageTypeReport
	case spec.BindingUsageTaskStatus, spec.BindingUsageTaskEvents, spec.BindingUsageTaskResult:
		return page.Type == spec.PageTypeTask
	default:
		return false
	}
}

func schemaHasFields(raw spec.JSONSchema) bool {
	if len(raw) == 0 {
		return false
	}
	var parsed map[string]json.RawMessage
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return true
	}
	var properties map[string]json.RawMessage
	if err := json.Unmarshal(parsed["properties"], &properties); err == nil && len(properties) > 0 {
		return true
	}
	var required []string
	if err := json.Unmarshal(parsed["required"], &required); err == nil && len(required) > 0 {
		return true
	}
	return false
}

func actorFromContext(ctx context.Context) string {
	actor, err := logicutils.CurrentUsername(ctx)
	if err != nil {
		return "system"
	}
	return actor
}

const rendererSchemaVersion = "page-spec:1"

func (s *ProposalService) buildBindingContracts(ctx context.Context, gameID, env string, bindings []spec.PageFunctionBinding) ([]spec.BindingContractSnapshot, error) {
	out := make([]spec.BindingContractSnapshot, 0, len(bindings))
	for _, binding := range bindings {
		functionID := strings.TrimSpace(binding.FunctionID)
		if functionID == "" {
			return nil, errorx.NewValidationError("binding.functionId is required")
		}
		contract, err := s.contractModel.FindByScopeAndFunctionID(ctx, gameID, env, functionID)
		if err != nil {
			return nil, errorx.NewValidationError("bound function contract does not exist: " + functionID)
		}
		if !contract.Enabled {
			return nil, errorx.NewValidationError("bound function is disabled: " + functionID)
		}
		out = append(out, spec.BindingContractSnapshot{
			BindingID:             strings.TrimSpace(binding.ID),
			FunctionID:            functionID,
			FunctionVersion:       strings.TrimSpace(contract.Version),
			InputSchemaDigest:     digestJSON(contract.InputSchema),
			OutputSchemaDigest:    digestJSON(contract.OutputSchema),
			Risk:                  spec.RiskLevel(contract.Risk.String()),
			Permission:            strings.TrimSpace(contract.Permission),
			Approval:              ApprovalPolicyFromJSONMap(contract.Approval),
			ExecutionMode:         binding.Execution.Mode,
			RendererSchemaVersion: rendererSchemaVersion,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].BindingID < out[j].BindingID })
	return out, nil
}

func digestJSON(raw model.JSON) string {
	if len(raw) == 0 {
		return ""
	}
	return freshness.CanonicalDigest(raw)
}
