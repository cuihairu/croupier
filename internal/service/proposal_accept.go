package service

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/config"
	"github.com/cuihairu/croupier/internal/db/dbctx"
	"github.com/cuihairu/croupier/internal/dbenum"
	"github.com/cuihairu/croupier/internal/model"
	"gorm.io/gorm"
)

// AcceptProposal accepts a proposal and materializes it as a PageSpec draft.
// Proposals with error-level diagnostics cannot be accepted.
// 发布分级（T10 扩展）：auto env 下落 draft 成功后自动接续发布；发布
// 失败不回滚 accept——draft 已在，PublishError 带回前端走人工链。
func (s *ProposalService) AcceptProposal(ctx context.Context, gameID, env, proposalKey string) (ProposalAcceptOutcome, error) {
	proposal, err := s.proposalModel.FindByScopeAndKey(ctx, gameID, env, proposalKey)
	if err != nil {
		return ProposalAcceptOutcome{}, fmt.Errorf("proposal not found: %w", err)
	}

	if proposal.Status != dbenum.ProposalStatusPending {
		return ProposalAcceptOutcome{}, fmt.Errorf("proposal is not pending")
	}

	// Block acceptance if there are error-level diagnostics
	if hasBlockingDiagnostics(proposal.Diagnostics) {
		return ProposalAcceptOutcome{}, fmt.Errorf("proposal has blocking diagnostics; resolve errors before accepting")
	}

	pageSpec, specJSON, err := pageSpecFromProposal(proposal)
	if err != nil {
		return ProposalAcceptOutcome{}, err
	}
	if err := validateAcceptedPageSpec(gameID, env, proposal, pageSpec); err != nil {
		return ProposalAcceptOutcome{}, err
	}

	actor := actorFromContext(ctx)
	now := time.Now()
	db := dbctx.Resolve(ctx, s.db)
	if err = db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
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
		// SetTitle 恒返回 nil（model 层实现为 `b, _ := json.Marshal(
		// map[string]string)`，无出错路径），原 err 检查为死分支，已删。
		_ = draft.SetTitle(normalizeLocalizedText(pageSpec.Title))
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
	}); err != nil {
		return ProposalAcceptOutcome{}, err
	}
	return s.autoPublishAfterAccept(ctx, gameID, env, proposalKey)
}

// autoPublishAfterAccept 在 accept 事务提交后按发布分级接续发布。
// 未注入钩子或策略非 auto 时维持「只落 draft」现状；auto 发布失败不回滚
// accept（镜像 versioning.CreateCompositePage 的降级语义）。
func (s *ProposalService) autoPublishAfterAccept(ctx context.Context, gameID, env, proposalKey string) (ProposalAcceptOutcome, error) {
	if s.publishReviewPolicy == nil || s.publishReviewPolicy(env) != config.PublishReviewAuto || s.autoPublish == nil {
		return ProposalAcceptOutcome{}, nil
	}
	if err := s.autoPublish(ctx, gameID, env, proposalKey); err != nil {
		slog.Warn("auto publish after accept failed; draft kept for manual publish",
			"gameId", gameID, "env", env, "proposalKey", proposalKey, "error", err)
		return ProposalAcceptOutcome{PublishError: err.Error()}, nil
	}
	return ProposalAcceptOutcome{Published: true}, nil
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
	actor, err := s.publishPermission(ctx)
	if err != nil {
		return ProposalPublishResult{}, err
	}
	if err := s.validateDirectPublishPageSpec(ctx, gameID, env, proposal, pageSpec); err != nil {
		return ProposalPublishResult{}, err
	}
	if details := s.publishValidationDiagnostics(ctx, gameID, env, pageSpec); len(details) > 0 {
		return ProposalPublishResult{}, errorx.NewValidationErrorWithDetails("page validation failed", details)
	}
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
		versionModel := model.NewPageVersionModel(tx)
		proposalModel := model.NewPageProposalModel(tx)
		proposalVersionModel := model.NewPageProposalVersionModel(tx)
		contractModel := model.NewFunctionContractModel(tx)

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
		proposal.FunctionDigest, err = currentFunctionDigest(txCtx, contractModel, gameID, env, pageSpec)
		if err != nil {
			return err
		}

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
			Status:              "draft",
			DraftRevision:       nextVersion,
			BaseProposalKey:     strings.TrimSpace(proposal.ProposalKey),
			BaseProposalVersion: proposalSnapshotVersion,
			UpdatedBy:           actor,
			CreatedAt:           now,
			UpdatedAt:           now,
		}
		// 同 AcceptProposal：SetTitle 恒返回 nil，err 检查为死分支，已删。
		_ = draft.SetTitle(normalizeLocalizedText(pageSpec.Title))
		if err := pageModel.Upsert(txCtx, draft); err != nil {
			return fmt.Errorf("create page draft from proposal: %w", err)
		}
		if err := versionModel.UpsertByScopePageKeyVersion(txCtx, &model.PageVersion{
			GameID:    gameID,
			Env:       env,
			PageKey:   draft.PageKey,
			Version:   nextVersion,
			SpecJSON:  specJSON,
			Status:    "draft",
			Message:   "accept generated proposal " + strings.TrimSpace(proposal.ProposalKey),
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
	revision := result.DraftRevision
	if s.publishDraft == nil {
		return ProposalPublishResult{}, fmt.Errorf("canonical proposal publisher is not configured")
	}
	if err := s.publishDraft.PublishProposalDraft(ctx, gameID, env, actor, pageSpec.PageKey, revision, proposalKey); err != nil {
		return ProposalPublishResult{}, err
	}
	result.PublishedVersion = revision
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

// ProposalPublishResult is returned when a proposal is directly published.
type ProposalPublishResult struct {
	PageKey          string `json:"pageKey"`
	DraftRevision    int    `json:"draftRevision"`
	PublishedVersion int    `json:"publishedVersion"`
}
