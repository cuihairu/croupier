package versioning

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/db/dbctx"
	logicutils "github.com/cuihairu/croupier/internal/logic/utils"
	"github.com/cuihairu/croupier/internal/model"
	"gorm.io/gorm"
)

// Service provides versioning and change management operations.
type Service struct {
	db                   *gorm.DB
	contractModel        *model.FunctionContractModel
	semanticsModel       *model.CapabilitySemanticsModel
	proposalModel        *model.PageProposalModel
	proposalVersionModel *model.PageProposalVersionModel
	pageModel            *model.PageSpecModel
	publishedModel       *model.PublishedPageSpecModel
	pageVersionModel     *model.PageVersionModel
	// T10 发布分级：策略与 auto 发布回调由 routes 装配时注入（回调形式
	// 保持依赖单向——versioning 不 import api/page）。缺省时保存链维持
	// 「只建提案」现状。
	publishReviewPolicy func(env string) string
	autoPublish         func(ctx context.Context, gameID, env, proposalKey string) error
	publishDraft        VersionDraftPublisher
	publishRollback     VersionRollbackValidator
	permissionGuard     func(context.Context, string) error
}

// VersionDraftPublisher and VersionRollbackValidator route version-owned
// publish semantics through the canonical PageSpec core without importing it.
type VersionDraftPublisher interface {
	PublishVersioningDraft(ctx context.Context, gameID, env, actor, pageKey string, expectedDraftRevision int, reason string) (int, error)
}

type VersionRollbackValidator interface {
	ValidateVersioningRollbackTarget(ctx context.Context, gameID, env, pageKey string, targetVersion int) error
}

// NewService creates the service.
func NewService(db *gorm.DB) *Service {
	return &Service{
		db:                   db,
		contractModel:        model.NewFunctionContractModel(db),
		semanticsModel:       model.NewCapabilitySemanticsModel(db),
		proposalModel:        model.NewPageProposalModel(db),
		proposalVersionModel: model.NewPageProposalVersionModel(db),
		pageModel:            model.NewPageSpecModel(db),
		publishedModel:       model.NewPublishedPageSpecModel(db),
		pageVersionModel:     model.NewPageVersionModel(db),
	}
}

// SetPublishReviewHooks 注入发布分级策略与 auto 发布回调（T10/D5）。
// policy 返回 config.PublishReviewAuto/Required；autoPublish 由 routes
// 绑定 page service（AutoPublishComposite）。
func (s *Service) SetPublishReviewHooks(
	policy func(env string) string,
	autoPublish func(ctx context.Context, gameID, env, proposalKey string) error,
) {
	s.publishReviewPolicy = policy
	s.autoPublish = autoPublish
}

func (s *Service) SetCanonicalPublishers(
	publishDraft VersionDraftPublisher,
	publishRollback VersionRollbackValidator,
) {
	s.publishDraft = publishDraft
	s.publishRollback = publishRollback
}

func (s *Service) SetPermissionGuard(guard func(context.Context, string) error) {
	s.permissionGuard = guard
}

func (s *Service) requirePublishPermission(ctx context.Context) error {
	if s.permissionGuard == nil {
		return nil
	}
	return s.permissionGuard(ctx, actorFromContext(ctx))
}

func actorFromContext(ctx context.Context) string {
	actor, err := logicutils.CurrentUsername(ctx)
	if err != nil {
		return "system"
	}
	return actor
}

func (s *Service) withTransaction(ctx context.Context, fn func(context.Context) error) error {
	db := dbctx.Resolve(ctx, s.db)
	// 文件型 sqlite 高负载下，deferred 事务读→写升级会撞上并发写锁：
	// 升级死锁规避路径不经过 busy_handler，SQLITE_BUSY 立即返回（E2E
	// safe-auto-merge 在 CI 上实证 0.1ms 内 500）。事务回滚后 fn 的全部
	// 副作用作废，短暂退避重试安全。
	const maxAttempts = 5
	for attempt := 1; ; attempt++ {
		err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			return fn(dbctx.WithDB(ctx, tx))
		})
		if err == nil || !isSQLiteBusyErr(err) || attempt >= maxAttempts {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Duration(attempt) * 200 * time.Millisecond):
		}
	}
}

func isSQLiteBusyErr(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "SQLITE_BUSY") || strings.Contains(msg, "database is locked")
}

// DeletePage deletes a page's draft, published versions and pending proposals
// (cleanup of obsolete page schemas).
func (s *Service) DeletePage(ctx context.Context, gameID, env, pageKey string) error {
	pageKey = strings.TrimSpace(pageKey)
	if pageKey == "" {
		return errorx.NewBadRequest("pageKey is required")
	}
	if err := s.pageModel.Delete(ctx, gameID, env, pageKey); err != nil {
		return fmt.Errorf("delete page draft: %w", err)
	}
	if err := s.publishedModel.DeleteByScopeAndPageKey(ctx, gameID, env, pageKey); err != nil {
		return fmt.Errorf("delete published page: %w", err)
	}
	if err := s.proposalModel.DeleteByScopeAndPageKey(ctx, gameID, env, pageKey); err != nil {
		return fmt.Errorf("delete page proposals: %w", err)
	}
	return nil
}
