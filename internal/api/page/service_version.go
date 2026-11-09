package page

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/cuihairu/croupier/internal/audit"
	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/model"
	versioningservice "github.com/cuihairu/croupier/internal/service/versioning"
)

// Page-version history pagination bounds.
const (
	pageVersionDefaultLimit = 5
	pageVersionMaxLimit     = 100
)

func (s *Service) Versions(ctx context.Context, req *PageVersionsRequest) (*PageVersionsResponse, error) {
	if err := s.requirePageRead(ctx); err != nil {
		return nil, err
	}
	gameID, env, err := requireScope(ctx)
	if err != nil {
		return nil, err
	}
	p, _ := s.svcCtx.PageSpecModel.FindByScopeAndPageKey(ctx, gameID, env, req.PageKey)
	// 版本随编辑/重发布无上限增长，历史必须分页拉取
	limit := req.Limit
	if limit <= 0 {
		limit = pageVersionDefaultLimit
	}
	if limit > pageVersionMaxLimit {
		limit = pageVersionMaxLimit
	}
	if req.Offset < 0 {
		req.Offset = 0
	}
	versions, total, err := s.svcCtx.PageVersionModel.ListByScopeAndPageKeyPaged(ctx, gameID, env, req.PageKey, limit, req.Offset)
	if err != nil {
		return nil, err
	}
	items := make([]spec.PageVersionItem, 0, len(versions))
	for _, v := range versions {
		items = append(items, spec.PageVersionItem{
			Version:            v.Version,
			Status:             v.Status,
			Message:            v.Message,
			IsCurrentDraft:     p != nil && v.Version == p.DraftRevision,
			IsCurrentPublished: p != nil && v.Version == p.PublishedVersion && p.PublishedVersion > 0,
			CreatedAt:          v.CreatedAt.Format(time.RFC3339),
			CreatedBy:          v.CreatedBy,
		})
	}
	resp := &PageVersionsResponse{Items: items, Total: total}
	if p != nil {
		resp.CurrentDraftRevision = p.DraftRevision
		resp.CurrentPublishedVersion = p.PublishedVersion
	}
	return resp, nil
}

func (s *Service) VersionDetail(ctx context.Context, req *PageVersionDetailRequest) (*PageVersionDetailResponse, error) {
	if err := s.requirePageRead(ctx); err != nil {
		return nil, err
	}
	gameID, env, err := requireScope(ctx)
	if err != nil {
		return nil, err
	}
	versions, err := s.svcCtx.PageVersionModel.ListByScopeAndPageKey(ctx, gameID, env, req.PageKey)
	if err != nil {
		return nil, err
	}
	for _, v := range versions {
		if strconv.Itoa(v.Version) != req.VersionID {
			continue
		}
		var pageSpec spec.PageSpec
		if err := json.Unmarshal([]byte(v.SpecJSON), &pageSpec); err != nil {
			return nil, fmt.Errorf("decode page version: %w", err)
		}
		return &PageVersionDetailResponse{
			Version:   v.Version,
			Status:    v.Status,
			Message:   v.Message,
			CreatedAt: v.CreatedAt.Format(time.RFC3339),
			CreatedBy: v.CreatedBy,
			Page:      pageSpec,
		}, nil
	}
	return nil, errorx.NewNotFound("page version not found")
}

func (s *Service) Rollback(ctx context.Context, req *PageRollbackRequest) (*PageRollbackResponse, error) {
	if err := s.requirePageRollback(ctx); err != nil {
		return nil, err
	}
	gameID, env, err := requireScope(ctx)
	if err != nil {
		return nil, err
	}
	actor, err := currentUsername(ctx)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(req.VersionID) == "" {
		return nil, errorx.NewBadRequest("versionId is required")
	}
	if req.ExpectedDraftRevision == nil {
		return nil, errorx.NewBadRequest("expectedDraftRevision is required")
	}
	versions, err := s.svcCtx.PageVersionModel.ListByScopeAndPageKey(ctx, gameID, env, req.PageKey)
	if err != nil {
		return nil, err
	}
	var target *model.PageVersion
	for _, v := range versions {
		if strconv.Itoa(v.Version) == req.VersionID {
			cp := v
			target = &cp
			break
		}
	}
	if target == nil {
		return nil, errorx.NewNotFound("page version not found")
	}
	// 注：ExpectedDraftRevision 的 nil 检查在函数入口（上方）已完成且 req
	// 在两处之间未被重新赋值，原此处的重复防御分支为死代码，已删（A 类）。
	var rolledBack spec.PageSpec
	if err := json.Unmarshal([]byte(target.SpecJSON), &rolledBack); err != nil {
		return nil, fmt.Errorf("decode page version: %w", err)
	}
	nextRevision := 0
	err = s.withPageTransaction(ctx, func(txCtx context.Context, pageModel *model.PageSpecModel, _ *model.PublishedPageSpecModel, versionModel *model.PageVersionModel) error {
		p, err := pageModel.FindByScopeAndPageKey(txCtx, gameID, env, req.PageKey)
		if err != nil {
			return ErrPageNotFound(req.PageKey)
		}
		if p.DraftRevision != *req.ExpectedDraftRevision {
			return errorx.NewConflictWithDetails("草稿版本冲突：页面已被其他修改更新，请刷新草稿后重试", map[string]any{
				"expected": *req.ExpectedDraftRevision,
				"current":  p.DraftRevision,
			})
		}
		if err := applyPageSpecToModel(p, rolledBack); err != nil {
			return err
		}
		p.GameID = gameID
		p.Env = env
		p.DraftRevision++
		p.Status = "draft"
		p.UpdatedAt = time.Now()
		p.BaseProposalKey = ""
		p.BaseProposalVersion = 0
		specJSON, err := buildPageSpecJSON(p)
		if err != nil {
			return err
		}
		if err := pageModel.Upsert(txCtx, p); err != nil {
			return err
		}
		if err := versionModel.UpsertByScopePageKeyVersion(txCtx, &model.PageVersion{
			GameID:    gameID,
			Env:       env,
			PageKey:   req.PageKey,
			Version:   p.DraftRevision,
			SpecJSON:  specJSON,
			Status:    "draft",
			Message:   "rollback to version " + strconv.Itoa(target.Version),
			CreatedBy: actor,
			CreatedAt: p.UpdatedAt,
		}); err != nil {
			return err
		}
		nextRevision = p.DraftRevision
		return nil
	})
	if err != nil {
		return nil, err
	}
	s.auditPageEvent(ctx, audit.EventPageRollback, gameID, env, req.PageKey, map[string]interface{}{
		"from_version":       target.Version,
		"new_draft_revision": nextRevision,
	})
	return &PageRollbackResponse{PageKey: req.PageKey, DraftRevision: nextRevision}, nil
}

// PublishVersioningDraft adapts versioning republish to the canonical draft
// publish core. It rejects a stale draft revision before writing a new version.
func (s *Service) PublishVersioningDraft(ctx context.Context, gameID, env, actor, pageKey string, expectedDraftRevision int, reason string) (int, error) {
	entry := pagePublishEntry{
		Caller: pagePublishCallerVersionRepublish,
		Actor:  actor,
		GameID: gameID,
		Env:    env,
	}
	revision := expectedDraftRevision
	if _, err := s.publishEntryCore(ctx, entry, &PagePublishRequest{PageKey: pageKey, DraftRevision: &revision}); err != nil {
		return 0, err
	}
	s.auditPageEvent(ctx, audit.EventPagePublish, gameID, env, pageKey, map[string]interface{}{
		"action":            "republish_current_draft",
		"caller":            pagePublishCallerVersionRepublish,
		"reason":            reason,
		"draft_revision":    expectedDraftRevision,
		"published_version": revision,
	})
	return revision, nil
}

// ValidateVersioningRollbackTarget applies the same PageSpec publish contract
// to the immutable rollback target before versioning creates the new revision.
func (s *Service) ValidateVersioningRollbackTarget(ctx context.Context, gameID, env, pageKey string, targetVersion int) error {
	published, err := s.svcCtx.PublishedPageSpecModel.FindByScopePageKeyAndVersion(ctx, gameID, env, pageKey, targetVersion)
	if err != nil {
		return errorx.NewNotFound("published page version not found")
	}
	pageSpec, err := pageSpecFromPublishedModel(*published)
	if err != nil {
		return err
	}
	diags := s.ValidatePageSpecInScope(ctx, gameID, env, pageSpec, true)
	if countErrors(diags) > 0 {
		return errorx.NewValidationErrorWithDetails("page validation failed", diagnosticsToDetails(diags))
	}
	contracts, err := buildBindingContracts(pageSpec.Bindings, s.functionsForScope(ctx, gameID, env))
	if err != nil {
		return err
	}
	if _, err := pageJSONMarshal(contracts); err != nil {
		return err
	}
	return nil
}

// PublishVersioningDraftForService is the cross-package adapter for versioning
// republish. It exposes only the canonical versioning entry.
func (s *Service) PublishVersioningDraftForService() versioningservice.VersionDraftPublisher {
	return versioningDraftPublisher{s: s}
}

// ValidateVersioningRollbackTargetForService is the cross-package adapter for
// versioning rollback preflight. It exposes only canonical snapshot validation.
func (s *Service) ValidateVersioningRollbackTargetForService() versioningservice.VersionRollbackValidator {
	return versioningRollbackValidator{s: s}
}

type versioningDraftPublisher struct {
	s *Service
}

func (p versioningDraftPublisher) PublishVersioningDraft(ctx context.Context, gameID, env, actor, pageKey string, expectedDraftRevision int, reason string) (int, error) {
	return p.s.PublishVersioningDraft(ctx, gameID, env, actor, pageKey, expectedDraftRevision, reason)
}

type versioningRollbackValidator struct {
	s *Service
}

func (p versioningRollbackValidator) ValidateVersioningRollbackTarget(ctx context.Context, gameID, env, pageKey string, targetVersion int) error {
	return p.s.ValidateVersioningRollbackTarget(ctx, gameID, env, pageKey, targetVersion)
}
