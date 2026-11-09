package handler

import (
	"context"

	"github.com/cuihairu/croupier/internal/api/console"
	"github.com/cuihairu/croupier/internal/api/menu"
	"github.com/cuihairu/croupier/internal/api/page"
	"github.com/cuihairu/croupier/internal/api/resource"
	"github.com/cuihairu/croupier/internal/api/resourcecatalog"
	logicutils "github.com/cuihairu/croupier/internal/logic/utils"
	"github.com/cuihairu/croupier/internal/service"
	versioningservice "github.com/cuihairu/croupier/internal/service/versioning"
	"github.com/cuihairu/croupier/internal/svc"

	"github.com/gin-gonic/gin"
)

// ============================================================================
// Resource 路由注册
// ============================================================================
func registerResourceRoutes(g *gin.RouterGroup, ctx *svc.ServiceContext) {
	resourceSvc := resource.NewService(ctx)
	resourceHandler := resource.NewHandler(resourceSvc)
	g.GET("", resourceHandler.List)
	g.GET("/", resourceHandler.List)
	g.GET("/:resourceKey", resourceHandler.Detail)
	g.GET("/:resourceKey/operations", resourceHandler.Operations)
}

// ============================================================================
// Resource Catalog 路由注册
// ============================================================================
func registerResourceCatalogRoutes(g *gin.RouterGroup, ctx *svc.ServiceContext) {
	service := resourcecatalog.NewService(ctx.DB, ctx.AuditService)
	handler := resourcecatalog.NewHandler(service)
	g.GET("", handler.List)
	// 静态段 /categories 优先于 /:resourceKey 匹配（gin 路由树静态优先）。
	g.GET("/categories", handler.Categories)
	g.GET("/:resourceKey", handler.Detail)
	g.PUT("/:resourceKey/semantics", handler.UpdateSemantics)
	g.GET("/:resourceKey/semantics/versions", handler.ListSemanticVersions)
	g.GET("/:resourceKey/conflicts", handler.ListConflicts)
	g.POST("/:resourceKey/conflicts/:field/resolve", handler.ResolveConflict)
}

// ============================================================================
// Page 路由注册
// ============================================================================
func registerPageRoutes(g *gin.RouterGroup, ctx *svc.ServiceContext) {
	page.RegisterDraftRoutes(g, ctx)
}

func registerMenuRoutes(g *gin.RouterGroup, ctx *svc.ServiceContext) {
	menu.RegisterMenuRoutes(g, ctx)
}

func registerProposalRoutes(g *gin.RouterGroup, ctx *svc.ServiceContext) {
	proposalSvc := service.NewProposalService(ctx.DB)
	// 发布分级（T10 扩展）：auto env 下 accept 落 draft 后自动接续发布。
	// 策略与发布回调在此装配注入（ProposalService 不 import api/page，
	// 依赖保持单向）；回调复用 versioning 同源的 page service
	// AutoPublishComposite——已存在草稿走「提案重建草稿 + 发布」。
	pageSvc := page.NewService(ctx)
	proposalSvc.SetProposalDraftPublisher(pageSvc.PublishProposalDraftForService())
	proposalSvc.SetPublishValidator(pageSvc)
	proposalSvc.SetPermissionGuard(func(c context.Context, actor string) (string, error) {
		if _, _, err := logicutils.RequireAnyPermission(
			c, ctx, "无权发布页面", "admin:all", "pages:publish",
		); err != nil {
			return "", err
		}
		return actor, nil
	})
	proposalSvc.SetPublishReviewHooks(
		func(env string) string { return ctx.Config.Pages.ResolvePublishReview(env) },
		pageSvc.AutoPublishComposite,
	)
	proposalHandler := service.NewProposalHandler(proposalSvc)
	g.GET("", proposalHandler.ListProposals)
	g.GET("/", proposalHandler.ListProposals)
	g.GET("/inbox", proposalHandler.Inbox)
	g.GET("/:proposalKey", proposalHandler.GetProposal)
	g.POST("/:proposalKey/accept", proposalHandler.AcceptProposal)
	g.POST("/:proposalKey/accept-and-publish", proposalHandler.AcceptAndPublishProposal)
	g.POST("/:proposalKey/reject", proposalHandler.RejectProposal)
}

func registerVersioningRoutes(g *gin.RouterGroup, ctx *svc.ServiceContext) {
	versioningSvc := versioningservice.NewService(ctx.DB)
	versioningSvc.SetPermissionGuard(func(requestCtx context.Context, actor string) error {
		_, _, err := logicutils.RequireAnyPermission(requestCtx, ctx, "无权发布页面", "admin:all", "pages:publish")
		return err
	})
	// 发布分级（T10）：pages.publishReview=auto 时 composite 保存后直接发布。
	// 策略与发布回调在此装配注入（versioning 不 import api/page，依赖保持
	// 单向）；republish/rollback-publish 统一要求 pages:publish。
	pageSvc := page.NewService(ctx)
	versioningSvc.SetCanonicalPublishers(
		pageSvc.PublishVersioningDraftForService(),
		pageSvc.ValidateVersioningRollbackTargetForService(),
	)
	versioningSvc.SetPublishReviewHooks(
		func(env string) string { return ctx.Config.Pages.ResolvePublishReview(env) },
		pageSvc.AutoPublishComposite,
	)
	versioningHandler := versioningservice.NewHandler(versioningSvc)
	g.GET("/pages/:pageKey/chain", versioningHandler.GetChangeChain)
	g.GET("/pages/:pageKey/diff", versioningHandler.Diff)
	g.POST("/pages/:pageKey/merge", versioningHandler.Merge)
	g.POST("/pages/:pageKey/rollback-draft", versioningHandler.RollbackDraft)
	g.POST("/pages/:pageKey/rollback-publish", versioningHandler.RollbackPublish)
	g.POST("/pages/:pageKey/regenerate", versioningHandler.RegenerateProposal)
	g.POST("/pages/:pageKey/republish", versioningHandler.Republish)
	// 组合页：手动聚合 2+ 资源生成 composite 提案（ProposalInbox 接受发布）
	g.POST("/pages/composite", versioningHandler.CreateCompositePage)
	// 组合页实时预览：同一生成链产出 PageSpec 但不落库（编辑器预览消费）。
	g.POST("/pages/composite/preview", versioningHandler.PreviewCompositePage)
	// 草稿预览执行：同一生成链重建草稿 spec 后走与发布页相同的执行门禁
	// （权限/审批/契约快照），编辑器「真实预览」经此入口执行，不走已发布
	// 快照，也不落库。执行核心复用 console 服务（快照由草稿侧构造）。
	g.POST("/pages/composite/preview/execute", console.NewHandler(console.NewService(ctx)).ExecuteDraftCompositeBinding)
	g.DELETE("/pages/:pageKey", versioningHandler.DeletePage)
}

// ============================================================================
// Console 路由注册
// ============================================================================
func registerConsoleRoutes(g *gin.RouterGroup, ctx *svc.ServiceContext) {
	consoleSvc := console.NewService(ctx)
	consoleHandler := console.NewHandler(consoleSvc)
	g.GET("/menu", consoleHandler.Menu)
	g.GET("/pages", consoleHandler.Pages)
	g.GET("/pages/:pageKey", consoleHandler.Page)
	g.POST("/pages/:pageKey/bindings/:bindingId/execute", consoleHandler.ExecuteBinding)
}
