package handler

import (
	"context"
	"fmt"

	"github.com/cuihairu/croupier/internal/api/bug"
	cicdapi "github.com/cuihairu/croupier/internal/api/cicd"
	"github.com/cuihairu/croupier/internal/api/component"
	dbmon "github.com/cuihairu/croupier/internal/api/dbmon"
	releaseapi "github.com/cuihairu/croupier/internal/api/release"
	"github.com/cuihairu/croupier/internal/api/tool"
	"github.com/cuihairu/croupier/internal/model"
	settings "github.com/cuihairu/croupier/internal/platform/settings"
	secguard "github.com/cuihairu/croupier/internal/security/secguard"
	"github.com/cuihairu/croupier/internal/service"
	"github.com/cuihairu/croupier/internal/svc"

	"github.com/gin-gonic/gin"
)

func registerReleaseRoutes(g *gin.RouterGroup, ctx *svc.ServiceContext) {
	releaseSvc := releaseapi.NewService(ctx)
	releaseHandler := releaseapi.NewHandler(releaseSvc)
	g.GET("", releaseHandler.List)
	g.GET("/", releaseHandler.List)
	g.POST("", releaseHandler.Create)
	g.POST("/", releaseHandler.Create)
	g.POST("/:id/transition", releaseHandler.Transition)
	g.POST("/:id/artifact", releaseHandler.UploadArtifact)
}

// 客户端检查更新是公开端点（游戏内 SDK 调用，无管理台 token）。
func registerPublicReleaseRoutes(g *gin.RouterGroup, ctx *svc.ServiceContext) {
	releaseSvc := releaseapi.NewService(ctx)
	releaseHandler := releaseapi.NewHandler(releaseSvc)
	g.POST("/releases/check", releaseHandler.CheckUpdate)
}

// buildContractTemplateRegenerator 组装「契约变更 → 自动重建组件模板」
// 的注入闭包：拉当前 scope 契约全量后复用组件模块的 regenerate 逻辑
// （与 POST /component-templates/regenerate 同一实现）。
func buildContractTemplateRegenerator(serverCtx *svc.ServiceContext, componentHandler *component.Handler) service.ContractTemplateRegenerator {
	if serverCtx == nil || serverCtx.DB == nil || componentHandler == nil {
		return nil
	}
	contractModel := model.NewFunctionContractModel(serverCtx.DB)
	return func(ctx context.Context, gameID, env string) error {
		contracts, err := contractModel.ListByScope(ctx, gameID, env)
		if err != nil {
			return fmt.Errorf("load contracts for template regen: %w", err)
		}
		return componentHandler.RegenerateFromContracts(ctx, contracts)
	}
}

func registerDBMonRoutes(g *gin.RouterGroup, ctx *svc.ServiceContext) {
	dbmonSvc := dbmon.NewService(ctx)
	dbmonHandler := dbmon.NewHandler(dbmonSvc)
	g.GET("/sources", dbmonHandler.ListSources)
	g.POST("/sources", dbmonHandler.CreateSource)
	g.PUT("/sources/:id", dbmonHandler.UpdateSource)
	g.DELETE("/sources/:id", dbmonHandler.DeleteSource)
	g.POST("/probe", dbmonHandler.ProbeAll)
}

func registerToolRoutes(g *gin.RouterGroup, ctx *svc.ServiceContext) {
	toolSvc := tool.NewService(ctx)
	toolHandler := tool.NewHandler(toolSvc)
	g.GET("", toolHandler.List)
	g.GET("/", toolHandler.List)
	g.POST("", toolHandler.Create)
	g.POST("/", toolHandler.Create)
	g.PUT("/:id", toolHandler.Update)
	g.DELETE("/:id", toolHandler.Delete)
}

// ============================================================================
// CI/CD 接入（可插拔 provider：jenkins/gitlab-ci/github-actions/generic #58）
// ============================================================================
func registerCicdRoutes(g *gin.RouterGroup, ctx *svc.ServiceContext) {
	cicdSvc := cicdapi.NewService(ctx)
	// 出站调用走 secguard 守卫客户端（SSRF/端口/域过滤与既有 webhook 探针同口径）
	cicdSvc = cicdSvc.WithHTTPClient(secguard.HTTPClient(secguard.Resolve(settings.Current()), nil))
	cicdHandler := cicdapi.NewHandler(cicdSvc)
	g.GET("/integrations", cicdHandler.List)
	g.POST("/integrations", cicdHandler.Create)
	g.PUT("/integrations/:id", cicdHandler.Update)
	g.DELETE("/integrations/:id", cicdHandler.Delete)
	g.POST("/integrations/:id/test", cicdHandler.Test)
	g.POST("/integrations/:id/trigger", cicdHandler.Trigger)
	g.GET("/builds", cicdHandler.Builds)
	g.POST("/builds/:id/refresh", cicdHandler.RefreshBuild)
}

// registerCicdWebhookRoute 挂公开构建状态回写端点（外部 CI 服务器无法携带
// JWT；鉴权 = integration.Token 的 X-CICD-Token 精确匹配，未配 Token 的
// 接入为开放端点——诚实边界）。
func registerCicdWebhookRoute(g *gin.RouterGroup, ctx *svc.ServiceContext) {
	cicdSvc := cicdapi.NewService(ctx)
	cicdHandler := cicdapi.NewHandler(cicdSvc)
	g.POST("/cicd/webhooks/:id", cicdHandler.Webhook)
}

// ============================================================================
// 缺陷追踪（Bug Tracker）路由注册
// ============================================================================
func registerBugRoutes(g *gin.RouterGroup, ctx *svc.ServiceContext) {
	bugSvc := bug.NewService(ctx)
	bugHandler := bug.NewHandler(bugSvc)
	g.GET("", bugHandler.List)
	g.GET("/", bugHandler.List)
	g.POST("", bugHandler.Create)
	g.POST("/", bugHandler.Create)
	// 崩溃聚合上报：同指纹堆栈聚合计数，首个报告开 triage 缺陷
	g.POST("/crash", bugHandler.ReportCrash)
	g.GET("/:id", bugHandler.Get)
	g.PUT("/:id", bugHandler.Update)
	g.DELETE("/:id", bugHandler.Delete)
	// bug↔工单多对多关联（#25）：缺陷侧列出/添加/解除客服工单关联
	g.GET("/:id/tickets", bugHandler.ListTickets)
	g.POST("/:id/tickets", bugHandler.LinkTicket)
	g.DELETE("/:id/tickets/:ticketId", bugHandler.UnlinkTicket)
}
