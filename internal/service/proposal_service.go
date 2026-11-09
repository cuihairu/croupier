package service

import (
	"context"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
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
	// 发布分级（T10 扩展）：auto env 下 AcceptProposal 落 draft 后自动
	// 接续发布。策略与发布回调由 routes 装配时注入（回调形式保持依赖
	// 单向——ProposalService 不 import api/page）。缺省时 accept 维持
	// 「只落 draft」现状。
	publishReviewPolicy func(env string) string
	autoPublish         func(ctx context.Context, gameID, env, proposalKey string) error
	publishDraft        ProposalDraftPublisher
	publishValidator    PagePublishValidator
	permissionGuard     func(context.Context, string) (string, error)
}

// ProposalDraftPublisher isolates proposal acceptance from the dashboard
// service package while still delegating to the single page publish core.
type ProposalDraftPublisher interface {
	PublishProposalDraft(ctx context.Context, gameID, env, actor, pageKey string, expectedDraftRevision int, proposalKey string) error
}

type PagePublishValidator interface {
	ValidatePageSpecInScope(ctx context.Context, gameID, env string, page spec.PageSpec, publish bool) []spec.Diagnostic
}

func (s *ProposalService) SetProposalDraftPublisher(publisher ProposalDraftPublisher) {
	s.publishDraft = publisher
}

func (s *ProposalService) SetPublishValidator(validator PagePublishValidator) {
	s.publishValidator = validator
}

func (s *ProposalService) SetPermissionGuard(guard func(context.Context, string) (string, error)) {
	s.permissionGuard = guard
}

// SetPublishReviewHooks 注入发布分级策略与 auto 发布回调（镜像
// versioning.Service.SetPublishReviewHooks）。policy 返回
// config.PublishReviewAuto/Required；autoPublish 由 routes 绑定 page
// service（AutoPublishComposite）。
func (s *ProposalService) SetPublishReviewHooks(
	policy func(env string) string,
	autoPublish func(ctx context.Context, gameID, env, proposalKey string) error,
) {
	s.publishReviewPolicy = policy
	s.autoPublish = autoPublish
}

// NewProposalService creates the service.
func NewProposalService(db *gorm.DB) *ProposalService {
	service := &ProposalService{
		db:             db,
		proposalModel:  model.NewPageProposalModel(db),
		pageModel:      model.NewPageSpecModel(db),
		versionModel:   model.NewPageVersionModel(db),
		contractModel:  model.NewFunctionContractModel(db),
		publishedModel: model.NewPublishedPageSpecModel(db),
		blockedModel:   model.NewBlockedProposalIssueModel(db),
	}
	service.permissionGuard = serviceProposalPermissionGuard(service)
	return service
}

const rendererSchemaVersion = "page-spec:1"
