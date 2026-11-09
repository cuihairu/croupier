package handler

import (
	"github.com/cuihairu/croupier/internal/api/auth"
	"github.com/cuihairu/croupier/internal/api/component"
	"github.com/cuihairu/croupier/internal/api/monitoring"
	settingsapi "github.com/cuihairu/croupier/internal/api/sitesettings"
	configpkg "github.com/cuihairu/croupier/internal/config"
	"github.com/cuihairu/croupier/internal/middleware/reqinfo"
	"github.com/cuihairu/croupier/internal/model"
	settings "github.com/cuihairu/croupier/internal/platform/settings"
	"github.com/cuihairu/croupier/internal/security/jwtutil"
	"github.com/cuihairu/croupier/internal/service"
	notify "github.com/cuihairu/croupier/internal/service/notify"
	permissionservice "github.com/cuihairu/croupier/internal/service/permission"
	"github.com/cuihairu/croupier/internal/svc"

	"log/slog"

	"github.com/gin-gonic/gin"
)

func RegisterHandlers(r *gin.Engine, serverCtx *svc.ServiceContext) {
	// 客户端身份（IP/UA）注入 request context，供审计等非 handler 层读取
	//（audit.AuditService.Log 自动补 WithIPAddress）。
	r.Use(reqinfo.Middleware())
	registerUploadStaticRoute(r, serverCtx)
	v1 := r.Group("/api/v1")

	// 网站配置 handler 必须先于 auth 路由创建：registerAuthRoutes 要把
	// 「登录方式热刷新」回调注入进来（此前声明在使用之后，回调从未挂上）
	siteSettingsHandler := settingsapi.NewHandler(settings.Current(), serverCtx.PlatformSettingModel)
	siteSettingsHandlerRef = siteSettingsHandler

	// 公开路由（无认证）
	registerAuthRoutes(v1.Group("/auth"), serverCtx)
	registerMetaRoutes(v1, serverCtx)
	registerMonitoringPublicRoutes(r, v1.Group("/monitoring"), serverCtx)
	// Prometheus exposition（telemetry.prometheus.enabled 开启时挂载；
	// 免认证——抓取器不带 JWT，路径已动态加入 AuthMiddleware 白名单）
	if serverCtx.Config.Telemetry.Prometheus.Enabled {
		r.GET(serverCtx.Config.Telemetry.Prometheus.PrometheusPath(),
			gin.WrapH(monitoring.NewPrometheusHandler(serverCtx)))
	}
	registerRegistryRoutes(v1.Group("/registry"), serverCtx) // 公开访问
	registerOpenAPIReadRoutes(v1, serverCtx)
	registerPublicReleaseRoutes(v1, serverCtx) // 客户端检查更新(公开)
	registerCicdWebhookRoute(v1, serverCtx)    // 外部 CI 构建状态回写（公开端点，令牌校验在 handler）
	registerAlertInboundRoute(v1, serverCtx)   // 第三方告警推入（公开端点，HMAC 签名校验在 handler）
	registerPublicConfigRoutes(v1, serverCtx)  // 客户端配置拉取(公开只读)
	if serverCtx.Config.FeatureFlags.Enabled(configpkg.FlagSupport) {
		playerSupport := v1.Group("/", newSoftFeatureGuard(settings.Current()).guard(configpkg.FlagSupport))
		registerPlayerSupportRoutes(playerSupport, serverCtx) // 玩家侧客服(游戏内 SDK)
	}
	// 需要认证的路由（使用 Authority 中间件）
	protected := v1.Group("/")
	protected.Use(serverCtx.Authority)

	// 网站配置：公开快照 + admin 写入（L3 覆盖层，handler 已提前创建）
	// 通知服务：依赖 Layered（渠道配置）与消息模型（站内信）。
	serverCtx.NotifyService = notify.New(settings.Current(), serverCtx.MessageModel)
	settingsapi.RegisterPublic(v1.Group("/public"), siteSettingsHandler)
	// 组件模板（V4 三层组合）：CRUD
	componentHandler := component.NewHandler(model.NewComponentTemplateModel(serverCtx.DB), model.NewFunctionContractModel(serverCtx.DB))
	componentHandler.Register(protected.Group("/component-templates"))
	// T2/D1：契约落库/变更后自动重建组件模板（手动 regenerate 退化为兜底）。
	// 包级注入：ContractService 在各 api 服务内按需构造，无法逐处 setter。
	if regen := buildContractTemplateRegenerator(serverCtx, componentHandler); regen != nil {
		service.SetContractTemplateRegenerator(regen)
	}
	// RegisterAdmin 路径已带 /site 前缀（PUT/DELETE /site/:key、GET /site/features 等），
	// 这里必须挂根组；挂 "/site" 会变成 /api/v1/site/site/... 全线 404。
	siteSettingsHandler.RegisterAdmin(protected.Group("/"))
	// 文件侧外部服务清单（herald/alertInbound/serverStatus 只读呈现，
	// server-status-provider 批 S3；与 L3 分层设置无关，纯 L2 快照）。
	settingsapi.NewExternalServicesHandler(serverCtx.Config).Register(protected.Group("/"))
	// 登录方式（外部身份源 L3 运行时配置——Harbor 模式：保存即热生效）
	authSite := protected.Group("/site")
	authSite.GET("/auth", siteSettingsHandler.GetAuth)
	authSite.POST("/auth/test", siteSettingsHandler.TestAuthConnection)

	// Scope-dependent 路由：需要 X-Game-ID/X-Env 的接口。
	// GameDBMiddleware 仅挂在此组，解析 scope 并注入 context。
	scoped := protected.Group("/")
	scoped.Use(svc.GameDBMiddleware(serverCtx))

	// 功能开关两层语义（docs/architecture/config-layering.md §4）：
	//   L2（featureFlags yaml）= 部署级物理裁剪，显式 false 的域不注册
	//     API 路由（404），重启生效；
	//   L3（platform_settings features.*）= 运营级软开关，路由照常注册，
	//     middleware 运行时拦截（403 feature_disabled），保存即生效，
	//     且只能关闭 L2 已启用的域（合成 L2 ∧ L3）。
	flags := serverCtx.Config.FeatureFlags
	softFlags := newSoftFeatureGuard(settings.Current())

	// Scope-independent 路由：不需要 game/env scope。
	{
		registerAdminRoutes(protected.Group("/admin"), serverCtx)
		registerGameRoutes(protected.Group("/games"), serverCtx)
		registerNodeRoutes(protected.Group("/nodes"), serverCtx)
		registerStorageRoutes(protected.Group("/storage"), serverCtx)
		registerAgentRoutes(protected.Group("/agent"), serverCtx)
		if flags.Enabled(configpkg.FlagOps) {
			opsSoft := protected.Group("/", softFlags.guard(configpkg.FlagOps))
			registerDBMonRoutes(opsSoft.Group("/dbmon"), serverCtx)
			registerAlertRoutes(opsSoft.Group("/alerts"), serverCtx)
			registerIncidentCategoryRoutes(opsSoft.Group("/incident-categories"), serverCtx)
			registerIncidentRoutes(opsSoft.Group("/incidents"), serverCtx)
			registerIncidentReportRoutes(opsSoft.Group("/incident-reports"), serverCtx)
			// 对外事故/bug REST（公开端点，Bearer token 校验在 handler）+
			// 外部令牌管理（JWT + admin 角色）；事故域整体随 ops L2 开关
			// 物理裁剪，外部 API 与面板路由同进退。
			registerExternalIncidentRoutes(v1, opsSoft, serverCtx)
			registerBackupRoutes(opsSoft.Group("/backups"), serverCtx)
			registerCertificateRoutes(opsSoft.Group("/certificates"), serverCtx)
		}
		if flags.Enabled(configpkg.FlagExtensions) {
			extSoft := protected.Group("/", softFlags.guard(configpkg.FlagExtensions))
			registerExtensionRoutes(extSoft.Group("/extensions"), serverCtx)
			registerAgentExtensionRoutes(extSoft.Group("/agents"), serverCtx)
			registerPlatformRoutes(extSoft.Group("/platforms"), serverCtx)
		}
		if flags.Enabled(configpkg.FlagSupport) {
			supportSoft := protected.Group("/", softFlags.guard(configpkg.FlagSupport))
			registerFAQRoutes(supportSoft.Group("/faqs"), serverCtx)
		}
		registerMessageRoutes(protected.Group("/messages"), serverCtx)
		registerAnnouncementRoutes(protected.Group("/admin"), protected.Group("/"), serverCtx)
		registerMonitoringProtectedRoutes(protected.Group("/monitoring"), serverCtx)
		registerPermissionRoutes(protected.Group("/permissions"), serverCtx)
		registerProfileRoutes(protected.Group("/profile"), serverCtx)
		registerProviderRoutes(protected.Group("/providers"), serverCtx)
		registerRateLimitRoutes(protected.Group("/rate-limits"), serverCtx)
		registerRoleRoutes(protected.Group("/roles"), serverCtx)
		registerSchemaRoutes(protected.Group("/schemas"), serverCtx)
		registerTermsRoutes(protected.Group("/terms"), serverCtx)
		if flags.Enabled(configpkg.FlagDev) {
			devSoft := protected.Group("/", softFlags.guard(configpkg.FlagDev))
			registerBugRoutes(devSoft.Group("/bugs"), serverCtx)
			registerToolRoutes(devSoft.Group("/tools"), serverCtx)
			registerReleaseRoutes(devSoft.Group("/releases"), serverCtx)
			registerHotpatchRoutes(devSoft.Group("/hotpatches"), serverCtx)
			registerCicdRoutes(devSoft.Group("/cicd"), serverCtx)
		}
		registerRegistryShortcutRoutes(protected, serverCtx)
		registerAuditRoutes(protected, serverCtx)
	}

	// Scope-dependent 路由
	{
		registerConsoleRoutes(scoped.Group("/console"), serverCtx)
		// Proposals are materialized per game/environment. They must use the
		// same resolved scope as registration, pages, and console execution.
		registerProposalRoutes(scoped.Group("/proposals"), serverCtx)
		registerFunctionRoutes(scoped.Group("/functions"), serverCtx)
		registerFunctionCallRoutes(scoped.Group("/function-calls"), serverCtx)
		registerFunctionMetadataRoutes(scoped.Group("/metadata"), serverCtx)
		if flags.Enabled(configpkg.FlagOps) {
			registerOpsRoutes(scoped.Group("/ops", softFlags.guard(configpkg.FlagOps)), serverCtx)
		}
		registerPageRoutes(scoped.Group("/pages"), serverCtx)
		registerMenuRoutes(scoped.Group("/menus"), serverCtx)
		if flags.Enabled(configpkg.FlagAnalytics) {
			registerAnalyticsRoutes(scoped.Group("/analytics", softFlags.guard(configpkg.FlagAnalytics)), serverCtx)
		}
		registerApprovalRoutes(scoped.Group("/approvals"), serverCtx)
		registerExecutionLogRoutes(scoped.Group("/execution-logs"), serverCtx)
		registerAssignmentRoutes(scoped.Group("/assignments"), serverCtx)
		registerConfigRoutes(scoped.Group("/configs"), serverCtx)
		// 在线配置浏览器：只读浏览各配置中心（git/redis/nacos/db/croupier）+
		// 可写源应急编辑（docs/research/config-workflows-analysis.md）
		registerConfigExplorerRoutes(scoped.Group("/config-explorer"), serverCtx)
		registerResourceRoutes(scoped.Group("/resources"), serverCtx)
		registerResourceCatalogRoutes(scoped.Group("/resource-catalog"), serverCtx)
		registerProviderSdkStatsRoutes(scoped.Group("/providers"), serverCtx)
		registerVersioningRoutes(scoped.Group("/versioning"), serverCtx)
		if flags.Enabled(configpkg.FlagSupport) {
			registerFeedbackRoutes(scoped.Group("/feedback"), serverCtx)
			// #21：工单是 game-scoped 数据（game_id/env 行隔离），迁入 scoped 组
			// 由 GameDBMiddleware 按顶栏 scope 鉴权+注入（页面不再自带游戏/环境过滤）；
			// L3 软开关守卫随迁保留。
			ticketSoft := scoped.Group("/", softFlags.guard(configpkg.FlagSupport))
			registerTicketRoutes(ticketSoft.Group("/tickets"), serverCtx)
		}
		registerPlayerRoutes(scoped.Group("/players"), serverCtx)
		registerTaskRoutes(scoped.Group("/tasks"), serverCtx)
		registerScheduleRoutes(scoped.Group("/schedules"), serverCtx)
		registerOpenAPISourceRoutes(scoped.Group("/openapi"), serverCtx)
	}
}

// siteSettingsHandlerRef 让 registerAuthRoutes 能给站点设置注入
// 登录方式热刷新回调（装配顺序：settings 先于 auth 创建）。
var siteSettingsHandlerRef *settingsapi.Handler

func registerAuthRoutes(g *gin.RouterGroup, ctx *svc.ServiceContext) {
	jwtSecret, err := jwtutil.ResolveSecret(ctx.Config)
	if err != nil {
		// Log error but continue with fallback for compatibility
		slog.Default().Warn("JWT secret resolution warning, using fallback", "error", err)
		// Use fallback for backward compatibility
		if jwtSecret == "" {
			jwtSecret = jwtutil.DevSecret()
		}
	}
	authSvc := auth.NewService(ctx.AdminModel, permissionservice.NewPermissionService(ctx.DB), jwtSecret, ctx.OpsStateStore).
		WithGameModel(ctx.GameModel).
		WithAuditService(ctx.AuditService).
		WithRoleModel(model.NewRoleModel(ctx.DB)).
		WithOTPRecoveryModel(ctx.AdminOTPRecoveryModel).
		WithVerificationModel(ctx.EmailVerificationModel).
		WithRecoveryDB(ctx.DB).
		WithLoginLockout(ctx.Config.Auth.LoginLockout)
	// 初始装配从分层设置读取（yaml 初始值 + database L3 覆盖）：
	// 只读 ctx.Config 会导致 UI 保存的登录方式重启后静默失效
	providers, err := auth.BuildIdentityProviders(settings.Current().AuthProviderConfig())
	if err != nil {
		slog.Default().Error("identity providers config invalid", "error", err)
	} else {
		providers.Attach(authSvc)
	}
	// 登录方式热刷新：站点设置保存 auth.* 后即时重建身份提供方
	//（L3 覆盖 yaml；无效配置被 PutKey 拒绝并回滚，见 sitesettings handler）
	if siteSettingsHandlerRef != nil {
		siteSettingsHandlerRef.SetAuthChangeCallback(authSvc.RefreshIdentityProviders)
	}
	authHandler := auth.NewHandler(authSvc)
	g.POST("/login", authHandler.Login)
	g.POST("/logout", authHandler.Logout)
	g.GET("/providers", authHandler.Providers)
	g.POST("/register", authHandler.Register)
	// 注册邮箱验证（#51c 第二批，均匿名）：令牌即凭据的验证回调 +
	// 用户名+邮箱重发（service 层防枚举静默）。
	g.GET("/verify-email", authHandler.VerifyEmail)
	g.POST("/resend-verification", authHandler.ResendVerification)
	g.GET("/oidc/login", authHandler.OIDCLogin)
	g.GET("/oidc/callback", authHandler.OIDCCallback)
	g.GET("/github/login", authHandler.GitHubLogin)
	g.GET("/github/callback", authHandler.GitHubCallback)
	// 微信扫码 + 自定义 OAuth2（#51 第三批，均匿名，语义同 oidc/github）。
	g.GET("/wechat/login", authHandler.WeChatLogin)
	g.GET("/wechat/callback", authHandler.WeChatCallback)
	g.GET("/generic/login", authHandler.GenericOAuthLogin)
	g.GET("/generic/callback", authHandler.GenericOAuthCallback)
	g.GET("/mfa/status", ctx.Authority, authHandler.MFAStatus)
	g.POST("/mfa/setup", ctx.Authority, authHandler.MFASetup)
	g.POST("/mfa/confirm", ctx.Authority, authHandler.MFAConfirm)
	g.POST("/mfa/disable", ctx.Authority, authHandler.MFADisable)
	g.POST("/check", ctx.Authority, authHandler.Check)
	g.POST("/check/batch", ctx.Authority, authHandler.BatchCheck)
}
