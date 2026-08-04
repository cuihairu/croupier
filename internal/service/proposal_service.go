package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/db/dbctx"
	logicutils "github.com/cuihairu/croupier/internal/logic/utils"
	"github.com/cuihairu/croupier/internal/model"
	"gorm.io/gorm"
)

// ProposalService manages page proposals.
type ProposalService struct {
	db            *gorm.DB
	proposalModel *model.PageProposalModel
	pageModel     *model.PageSpecModel
	versionModel  *model.PageVersionModel
}

// NewProposalService creates the service.
func NewProposalService(db *gorm.DB) *ProposalService {
	return &ProposalService{
		db:            db,
		proposalModel: model.NewPageProposalModel(db),
		pageModel:     model.NewPageSpecModel(db),
		versionModel:  model.NewPageVersionModel(db),
	}
}

// ListProposals lists all proposals in a scope.
func (s *ProposalService) ListProposals(ctx context.Context, gameID, env string) ([]*model.PageProposal, error) {
	return s.proposalModel.ListByScope(ctx, gameID, env)
}

// ListProposalsByStatus lists proposals by status.
func (s *ProposalService) ListProposalsByStatus(ctx context.Context, gameID, env, status string) ([]*model.PageProposal, error) {
	return s.proposalModel.ListByStatus(ctx, gameID, env, status)
}

// GetProposal gets a proposal by key.
func (s *ProposalService) GetProposal(ctx context.Context, gameID, env, proposalKey string) (*model.PageProposal, error) {
	return s.proposalModel.FindByScopeAndKey(ctx, gameID, env, proposalKey)
}

// AcceptProposal accepts a proposal and materializes it as a PageSpec draft.
// Proposals with error-level diagnostics cannot be accepted.
func (s *ProposalService) AcceptProposal(ctx context.Context, gameID, env, proposalKey string) error {
	proposal, err := s.proposalModel.FindByScopeAndKey(ctx, gameID, env, proposalKey)
	if err != nil {
		return fmt.Errorf("proposal not found: %w", err)
	}

	if proposal.Status != "pending" {
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

		draft := &model.PageSpec{
			GameID:        gameID,
			Env:           env,
			PageKey:       strings.TrimSpace(pageSpec.PageKey),
			Type:          string(pageSpec.Type),
			ResourceKey:   strings.TrimSpace(pageSpec.ResourceKey),
			CategoryKey:   strings.TrimSpace(pageSpec.Category.Key),
			CategoryOrder: pageSpec.Category.Order,
			Order:         pageSpec.Order,
			Icon:          strings.TrimSpace(pageSpec.Icon),
			SpecJSON:      specJSON,
			Status:        "draft",
			DraftRevision: 1,
			UpdatedBy:     actor,
			CreatedAt:     now,
			UpdatedAt:     now,
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

		proposal.Status = "accepted"
		proposal.UpdatedBy = actor
		proposal.UpdatedAt = now
		return proposalModel.UpsertProposal(txCtx, proposal)
	})
}

// RejectProposal rejects a proposal.
func (s *ProposalService) RejectProposal(ctx context.Context, gameID, env, proposalKey string) error {
	proposal, err := s.proposalModel.FindByScopeAndKey(ctx, gameID, env, proposalKey)
	if err != nil {
		return fmt.Errorf("proposal not found: %w", err)
	}

	if proposal.Status != "pending" {
		return fmt.Errorf("proposal is not pending")
	}

	// Update status to rejected
	proposal.Status = "rejected"
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

func isValidProposalPageType(pageType spec.PageType) bool {
	switch pageType {
	case spec.PageTypeResource, spec.PageTypeOperation, spec.PageTypeTask, spec.PageTypeReport:
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
	default:
		return false
	}
}

func isValidBindingUsage(usage spec.PageBindingUsage) bool {
	switch usage {
	case spec.BindingUsageQuery, spec.BindingUsageDetail, spec.BindingUsageAction, spec.BindingUsageTask, spec.BindingUsageReport:
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

func actorFromContext(ctx context.Context) string {
	actor, err := logicutils.CurrentUsername(ctx)
	if err != nil {
		return "system"
	}
	return actor
}
