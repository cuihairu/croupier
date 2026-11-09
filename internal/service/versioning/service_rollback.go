package versioning

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/db/dbctx"
	"github.com/cuihairu/croupier/internal/model"
	"gorm.io/gorm"
)

// RollbackRequest is the request for rollback.
type RollbackRequest struct {
	GameID                string `json:"-"`
	Env                   string `json:"-"`
	PageKey               string `uri:"pageKey" json:"-"`
	ExpectedDraftRevision int    `json:"expectedDraftRevision"`
	Version               int    `json:"version"`
	Reason                string `json:"reason,omitempty"`
}

// RollbackResponse is the response for rollback.
type RollbackResponse struct {
	PageKey       string `json:"pageKey"`
	DraftRevision int    `json:"draftRevision,omitempty"`
	Version       int    `json:"version,omitempty"`
	Message       string `json:"message"`
}

// RollbackDraft rolls back to a previous draft version.
func (s *Service) RollbackDraft(ctx context.Context, req *RollbackRequest) (*RollbackResponse, error) {
	if req.Version <= 0 {
		return nil, errorx.NewBadRequest("version is required")
	}
	if req.ExpectedDraftRevision <= 0 {
		return nil, errorx.NewBadRequest("expectedDraftRevision is required")
	}
	pageKey := strings.TrimSpace(req.PageKey)
	if pageKey == "" {
		return nil, errorx.NewBadRequest("pageKey is required")
	}
	target, err := s.findPageVersion(ctx, req.GameID, req.Env, pageKey, req.Version)
	if err != nil {
		return nil, err
	}
	var pageSpec spec.PageSpec
	if err := json.Unmarshal([]byte(target.SpecJSON), &pageSpec); err != nil {
		return nil, fmt.Errorf("decode page version: %w", err)
	}
	pageSpec = normalizePageSpec(pageSpec)
	actor := actorFromContext(ctx)
	now := time.Now()
	var nextRevision int

	err = s.withTransaction(ctx, func(txCtx context.Context) error {
		pageModel := model.NewPageSpecModel(dbctx.Resolve(txCtx, s.db))
		versionModel := model.NewPageVersionModel(dbctx.Resolve(txCtx, s.db))

		page, err := pageModel.FindByScopeAndPageKey(txCtx, req.GameID, req.Env, pageKey)
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if page == nil {
			return errorx.NewNotFound("page draft not found")
		}
		if page.DraftRevision != req.ExpectedDraftRevision {
			return draftRevisionConflict(req.ExpectedDraftRevision, page.DraftRevision)
		}
		nextRevision, err = versionModel.GetNextVersion(txCtx, req.GameID, req.Env, pageKey)
		if err != nil {
			return err
		}
		// applyPageSpecToModel/marshalPageSpec 对 Unmarshal 产物的 pageSpec
		// 恒成功（见 persistMergedDraft 处论证），err 检查为死分支，已删。
		_ = applyPageSpecToModel(page, pageSpec)
		page.GameID = req.GameID
		page.Env = req.Env
		page.Status = "draft"
		page.DraftRevision = nextRevision
		page.UpdatedBy = actor
		page.UpdatedAt = now
		specJSON, _ := marshalPageSpec(pageSpec)
		if err := pageModel.Upsert(txCtx, page); err != nil {
			return err
		}
		return versionModel.UpsertByScopePageKeyVersion(txCtx, &model.PageVersion{
			GameID:    req.GameID,
			Env:       req.Env,
			PageKey:   pageKey,
			Version:   nextRevision,
			SpecJSON:  specJSON,
			Status:    "draft",
			Message:   firstNonEmpty(req.Reason, "rollback draft to version "+strconv.Itoa(req.Version)),
			CreatedBy: actor,
			CreatedAt: now,
		})
	})
	if err != nil {
		return nil, err
	}
	return &RollbackResponse{
		PageKey:       pageKey,
		DraftRevision: nextRevision,
		Message:       fmt.Sprintf("rolled back draft %s to version %d", pageKey, req.Version),
	}, nil
}

// RollbackPublish rolls back to a previous published version.
func (s *Service) RollbackPublish(ctx context.Context, req *RollbackRequest) (*RollbackResponse, error) {
	if err := s.requirePublishPermission(ctx); err != nil {
		return nil, err
	}
	if req.Version <= 0 {
		return nil, errorx.NewBadRequest("version is required")
	}
	if req.ExpectedDraftRevision <= 0 {
		return nil, errorx.NewBadRequest("expectedDraftRevision is required")
	}
	pageKey := strings.TrimSpace(req.PageKey)
	if pageKey == "" {
		return nil, errorx.NewBadRequest("pageKey is required")
	}
	if s.publishRollback != nil {
		if err := s.publishRollback.ValidateVersioningRollbackTarget(ctx, req.GameID, req.Env, pageKey, req.Version); err != nil {
			return nil, err
		}
	}
	target, err := s.publishedModel.FindByScopePageKeyAndVersion(ctx, req.GameID, req.Env, pageKey, req.Version)
	if err != nil {
		return nil, errorx.NewNotFound("published page version not found")
	}
	var pageSpec spec.PageSpec
	if err := json.Unmarshal([]byte(target.SpecJSON), &pageSpec); err != nil {
		return nil, fmt.Errorf("decode published page spec: %w", err)
	}
	pageSpec = normalizePageSpec(pageSpec)
	actor := actorFromContext(ctx)
	now := time.Now()
	var nextVersion int

	err = s.withTransaction(ctx, func(txCtx context.Context) error {
		pageModel := model.NewPageSpecModel(dbctx.Resolve(txCtx, s.db))
		publishedModel := model.NewPublishedPageSpecModel(dbctx.Resolve(txCtx, s.db))
		versionModel := model.NewPageVersionModel(dbctx.Resolve(txCtx, s.db))

		var err error
		nextVersion, err = versionModel.GetNextVersion(txCtx, req.GameID, req.Env, pageKey)
		if err != nil {
			return err
		}
		page, err := pageModel.FindByScopeAndPageKey(txCtx, req.GameID, req.Env, pageKey)
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if page == nil {
			return errorx.NewNotFound("page draft not found")
		}
		if page.DraftRevision != req.ExpectedDraftRevision {
			return draftRevisionConflict(req.ExpectedDraftRevision, page.DraftRevision)
		}
		// applyPageSpecToModel/marshalPageSpec 对 Unmarshal 产物的 pageSpec
		// 恒成功（见 persistMergedDraft 处论证），err 检查为死分支，已删。
		_ = applyPageSpecToModel(page, pageSpec)
		page.GameID = req.GameID
		page.Env = req.Env
		page.Status = "published"
		page.PublishedActive = true
		page.PublishedVersion = nextVersion
		page.DraftRevision = nextVersion
		page.UpdatedBy = actor
		page.UpdatedAt = now

		specJSON, _ := marshalPageSpec(pageSpec)
		if err := publishedModel.DeactivatePage(txCtx, req.GameID, req.Env, pageKey, now); err != nil {
			return err
		}
		if err := publishedModel.Create(txCtx, &model.PublishedPageSpec{
			GameID:                req.GameID,
			Env:                   req.Env,
			PageKey:               pageKey,
			Version:               nextVersion,
			SpecJSON:              specJSON,
			BindingContractsJSON:  target.BindingContractsJSON,
			RendererSchemaVersion: target.RendererSchemaVersion,
			BaseProposalKey:       target.BaseProposalKey,
			BaseProposalVersion:   target.BaseProposalVersion,
			FunctionDigest:        target.FunctionDigest,
			SemanticsDigest:       target.SemanticsDigest,
			GeneratorVersion:      target.GeneratorVersion,
			Active:                true,
			PublishedAt:           now,
			PublishedBy:           actor,
		}); err != nil {
			return err
		}
		if err := pageModel.Upsert(txCtx, page); err != nil {
			return err
		}
		return versionModel.UpsertByScopePageKeyVersion(txCtx, &model.PageVersion{
			GameID:    req.GameID,
			Env:       req.Env,
			PageKey:   pageKey,
			Version:   nextVersion,
			SpecJSON:  specJSON,
			Status:    "published",
			Message:   firstNonEmpty(req.Reason, "rollback publish to version "+strconv.Itoa(req.Version)),
			CreatedBy: actor,
			CreatedAt: now,
		})
	})
	if err != nil {
		return nil, err
	}
	return &RollbackResponse{
		PageKey:       pageKey,
		DraftRevision: nextVersion,
		Version:       nextVersion,
		Message:       fmt.Sprintf("rolled back published page %s to version %d", pageKey, req.Version),
	}, nil
}

func (s *Service) findPageVersion(ctx context.Context, gameID, env, pageKey string, version int) (*model.PageVersion, error) {
	versions, err := s.pageVersionModel.ListByScopeAndPageKey(ctx, gameID, env, pageKey)
	if err != nil {
		return nil, err
	}
	for _, item := range versions {
		if item.Version == version {
			return &item, nil
		}
	}
	return nil, errorx.NewNotFound("page version not found")
}
