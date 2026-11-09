package handler

import (
	"github.com/cuihairu/croupier/internal/api/announcement"
	"github.com/cuihairu/croupier/internal/api/approval"
	"github.com/cuihairu/croupier/internal/api/assignment"
	"github.com/cuihairu/croupier/internal/api/backup"
	"github.com/cuihairu/croupier/internal/api/certificate"
	"github.com/cuihairu/croupier/internal/api/config"
	"github.com/cuihairu/croupier/internal/api/configexplorer"
	executionlogapi "github.com/cuihairu/croupier/internal/api/executionlog"
	"github.com/cuihairu/croupier/internal/api/faq"
	"github.com/cuihairu/croupier/internal/api/feedback"
	"github.com/cuihairu/croupier/internal/api/hotpatch"
	"github.com/cuihairu/croupier/internal/api/message"
	"github.com/cuihairu/croupier/internal/api/permission"
	"github.com/cuihairu/croupier/internal/api/platform"
	"github.com/cuihairu/croupier/internal/api/profile"
	"github.com/cuihairu/croupier/internal/api/rate_limit"
	"github.com/cuihairu/croupier/internal/api/role"
	"github.com/cuihairu/croupier/internal/api/schema"
	supportapi "github.com/cuihairu/croupier/internal/api/support"
	"github.com/cuihairu/croupier/internal/api/terms"
	"github.com/cuihairu/croupier/internal/api/ticket"
	"github.com/cuihairu/croupier/internal/svc"

	"github.com/gin-gonic/gin"
)

// ============================================================================
// Approval 路由注册
// ============================================================================
func registerExecutionLogRoutes(g *gin.RouterGroup, ctx *svc.ServiceContext) {
	execLogHandler := executionlogapi.NewHandler(executionlogapi.NewService(ctx))
	g.GET("/", execLogHandler.List)
	// #33：操作人聚合选项（静态段，与 /:id 共存）
	g.GET("/operator-options", execLogHandler.OperatorOptions)
	g.GET("/:id", execLogHandler.Get)
}

func registerApprovalRoutes(g *gin.RouterGroup, ctx *svc.ServiceContext) {
	approvalSvc := approval.NewService(ctx)
	approvalHandler := approval.NewHandler(approvalSvc)
	g.GET("/", approvalHandler.List)
	g.GET("/:id", approvalHandler.Get)
	g.POST("/:id/approve", approvalHandler.Approve)
	g.POST("/:id/reject", approvalHandler.Reject)
}

// ============================================================================
// Assignment 路由注册
// ============================================================================
func registerAssignmentRoutes(g *gin.RouterGroup, ctx *svc.ServiceContext) {
	assignmentSvc := assignment.NewService(ctx)
	assignmentHandler := assignment.NewHandler(assignmentSvc)
	g.GET("", assignmentHandler.List)
	g.GET("/", assignmentHandler.List)
	g.GET("/history", assignmentHandler.History)
	g.PUT("", assignmentHandler.Update)
	g.PUT("/", assignmentHandler.Update)
}

// ============================================================================
// Backup 路由注册
// ============================================================================
func registerBackupRoutes(g *gin.RouterGroup, ctx *svc.ServiceContext) {
	backupSvc := backup.NewService(ctx)
	backupHandler := backup.NewHandler(backupSvc)
	g.GET("", backupHandler.List)
	g.GET("/", backupHandler.List)
	g.POST("", backupHandler.Create)
	g.POST("/", backupHandler.Create)
	g.DELETE("/:id", backupHandler.Delete)
	g.GET("/:id/download", backupHandler.Download)
}

// ============================================================================
// Certificate 路由注册
// ============================================================================
func registerCertificateRoutes(g *gin.RouterGroup, ctx *svc.ServiceContext) {
	certificateSvc := certificate.NewService(ctx)
	certificateHandler := certificate.NewHandler(certificateSvc)
	g.GET("", certificateHandler.List)
	g.GET("/", certificateHandler.List)
	g.POST("", certificateHandler.Add)
	g.POST("/", certificateHandler.Add)
	g.GET("/:id", certificateHandler.Get)
	g.POST("/:id/check", certificateHandler.Check)
	g.DELETE("/:id", certificateHandler.Delete)
	g.GET("/stats", certificateHandler.Stats)
	g.POST("/alerts", certificateHandler.AddAlert)
	g.GET("/alerts", certificateHandler.AlertsList)
	g.POST("/check-all", certificateHandler.CheckAll)
	g.GET("/domain-info", certificateHandler.DomainInfo)
	g.GET("/expiring", certificateHandler.Expiring)
}

// ============================================================================
// Config 路由注册
// ============================================================================
func registerConfigRoutes(g *gin.RouterGroup, ctx *svc.ServiceContext) {
	configSvc := config.NewService(ctx)
	configHandler := config.NewHandler(configSvc)
	g.GET("", configHandler.List)
	g.GET("/", configHandler.List)
	// Excel 在线配置：上传 .xlsx / Univer 快照编译 → ConfigVersion(ns=gameplay)
	g.POST("/excel/import", configHandler.ImportExcel)
	g.POST("/excel/compile", configHandler.CompileExcelSnapshot)
	g.POST("", configHandler.Upsert)
	g.POST("/", configHandler.Upsert)
	g.GET("/version", configHandler.GetVersion)
	g.GET("/versions", configHandler.ListVersions)
	g.GET("/:id", configHandler.Get)
	g.PUT("/:id", configHandler.Save)
	g.POST("/:id/validate", configHandler.Validate)
	g.GET("/:id/versions", configHandler.ListVersionsByID)
	g.GET("/:id/versions/:version", configHandler.GetVersionByID)
	// SSE 配置变更订阅（通知只带版本号，数据走拉取）
	g.GET("/watch", config.NewWatchHandler(config.NewWatchService(ctx)).Watch)
}

// ============================================================================
// Config Explorer 路由注册
// ============================================================================
func registerConfigExplorerRoutes(g *gin.RouterGroup, ctx *svc.ServiceContext) {
	explorerSvc := configexplorer.NewService(ctx)
	explorerHandler := configexplorer.NewHandler(explorerSvc)
	g.GET("/sources", explorerHandler.ListBindings)
	g.POST("/sources", explorerHandler.UpsertBinding)
	g.DELETE("/sources/:id", explorerHandler.DeleteBinding)
	g.GET("/tree", explorerHandler.List)
	g.GET("/file", explorerHandler.Read)
	g.PUT("/file", explorerHandler.Write)
}

// ============================================================================
// FAQ 路由注册
// ============================================================================
func registerFAQRoutes(g *gin.RouterGroup, ctx *svc.ServiceContext) {
	faqSvc := faq.NewService(ctx)
	faqHandler := faq.NewHandler(faqSvc)
	g.GET("", faqHandler.List)
	g.GET("/", faqHandler.List)
	g.POST("", faqHandler.Create)
	g.POST("/", faqHandler.Create)
	g.PUT("/:id", faqHandler.Update)
	g.DELETE("/:id", faqHandler.Delete)
	// 玩家反馈投票（有用/无用），驱动 FAQ 内容治理队列
	g.POST("/:id/vote", faqHandler.Vote)
	g.GET("/categories", faqHandler.Categories)
}

// ============================================================================
// Feedback 路由注册
// ============================================================================
func registerFeedbackRoutes(g *gin.RouterGroup, ctx *svc.ServiceContext) {
	feedbackSvc := feedback.NewService(ctx)
	feedbackHandler := feedback.NewHandler(feedbackSvc)
	g.GET("", feedbackHandler.List)
	g.GET("/", feedbackHandler.List)
	g.POST("", feedbackHandler.Create)
	g.POST("/", feedbackHandler.Create)
	g.PUT("/:id", feedbackHandler.Update)
	g.DELETE("/:id", feedbackHandler.Delete)
	g.GET("/stats", feedbackHandler.Stats)
	// 反馈一键升级为工单（携带玩家上下文，幂等：重复转化返回原工单）
	g.POST("/:id/convert", feedbackHandler.ConvertToTicket)
}

// ============================================================================
// Announcement 路由注册
// ============================================================================
func registerAnnouncementRoutes(adminGroup, userGroup *gin.RouterGroup, ctx *svc.ServiceContext) {
	announcementSvc := announcement.NewService(ctx)
	announcementHandler := announcement.NewHandler(announcementSvc)
	// 管理端
	ag := adminGroup.Group("/announcements")
	{
		ag.GET("", announcementHandler.List)
		ag.GET("/", announcementHandler.List)
		ag.POST("", announcementHandler.Create)
		ag.POST("/", announcementHandler.Create)
		ag.PUT("/:id", announcementHandler.Update)
		ag.DELETE("/:id", announcementHandler.Delete)
	}
	// 用户侧
	ug := userGroup.Group("/announcements")
	{
		ug.GET("/active", announcementHandler.Active)
		ug.POST("/:id/dismiss", announcementHandler.Dismiss)
	}
}

// ============================================================================
// Message 路由注册
// ============================================================================
func registerMessageRoutes(g *gin.RouterGroup, ctx *svc.ServiceContext) {
	messageSvc := message.NewService(ctx)
	messageHandler := message.NewHandler(messageSvc, ctx.Config.SSE)
	// 管理员群发（handler 内校验 admin 角色——/admin/:id wildcard 与静态
	// 段同方法冲突，故挂本组静态路径）
	g.POST("/broadcast", messageHandler.Broadcast)
	g.GET("", messageHandler.List)
	g.GET("/", messageHandler.List)
	g.POST("", messageHandler.Send)
	g.POST("/", messageHandler.Send)
	g.GET("/:id", messageHandler.Get)
	g.POST("/:id/read", messageHandler.Read)
	g.GET("/unread-count", messageHandler.UnreadCount)
	g.GET("/stream", messageHandler.Stream)
}

func registerPermissionRoutes(g *gin.RouterGroup, ctx *svc.ServiceContext) {
	permissionSvc := permission.NewService(ctx)
	permissionHandler := permission.NewHandler(permissionSvc)
	g.GET("", permissionHandler.List)
	g.GET("/", permissionHandler.List)
	g.GET("/:id", permissionHandler.Detail)
}

// ============================================================================
// Platform 路由注册
// ============================================================================
func registerPlatformRoutes(g *gin.RouterGroup, ctx *svc.ServiceContext) {
	platformSvc := platform.NewService(ctx)
	platformHandler := platform.NewHandler(platformSvc)
	g.POST("/call", platformHandler.Call)
	g.GET("", platformHandler.List)
	g.GET("/", platformHandler.List)
	g.GET("/:platform/methods", platformHandler.Methods)
}

// ============================================================================
// Profile 路由注册
// ============================================================================
func registerProfileRoutes(g *gin.RouterGroup, ctx *svc.ServiceContext) {
	profileSvc := profile.NewService(ctx.AdminModel, ctx.GameModel, ctx.RoleModel, ctx.OpsStateStore).
		WithDB(ctx.DB).
		// 头像对象 key → 当前可访问 URL；未接对象存储时退化为 /uploads/ 相对路径。
		WithObjectStore(ctx.ObjectStore).
		// 资料更新后失效 admin 缓存（缓存整行，含 Avatar）。
		WithCacheInvalidator(ctx.InvalidateAdminCache).
		// 短信通道可用性（未接入时如实报 unavailable，前端据此禁用开关）
		WithSMSRegistry(ctx.SMSRegistry)
	profileHandler := profile.NewHandler(profileSvc)
	g.GET("", profileHandler.GetProfile)     // /api/v1/profile
	g.GET("/", profileHandler.GetProfile)    // /api/v1/profile/
	g.PUT("", profileHandler.UpdateProfile)  // /api/v1/profile
	g.PUT("/", profileHandler.UpdateProfile) // /api/v1/profile/
	g.PUT("/password", profileHandler.ChangePassword)
	g.GET("/permissions", profileHandler.GetPermissions)
	g.GET("/notification-channels", profileHandler.GetNotificationChannels)
	g.GET("/games", profileHandler.GetGames)
	g.PATCH("/scope", profileHandler.UpdateScope)
}

func registerRoleRoutes(g *gin.RouterGroup, ctx *svc.ServiceContext) {
	roleSvc := role.NewService(ctx)
	roleHandler := role.NewHandler(roleSvc)
	g.GET("", roleHandler.RolesList)
	g.GET("/", roleHandler.RolesList)
	g.POST("", roleHandler.RoleCreate)
	g.POST("/", roleHandler.RoleCreate)
	g.GET("/:id", roleHandler.RoleDetail)
	g.PUT("/:id", roleHandler.RoleUpdate)
	g.DELETE("/:id", roleHandler.RoleDelete)
	g.PUT("/:id/permissions", roleHandler.RoleUpdate)
}

// ============================================================================
// RateLimit 路由注册
// ============================================================================
func registerRateLimitRoutes(g *gin.RouterGroup, ctx *svc.ServiceContext) {
	rateLimitSvc := rate_limit.NewService(ctx)
	rateLimitHandler := rate_limit.NewHandler(rateLimitSvc)
	g.GET("", rateLimitHandler.List)
	g.GET("/", rateLimitHandler.List)
	g.GET("/:id", rateLimitHandler.Get)
	g.PUT("", rateLimitHandler.Upsert)
	g.PUT("/", rateLimitHandler.Upsert)
	g.DELETE("/:id", rateLimitHandler.Delete)
	g.POST("/preview", rateLimitHandler.Preview)
}

// ============================================================================
// Schema 路由注册
// ============================================================================
func registerSchemaRoutes(g *gin.RouterGroup, ctx *svc.ServiceContext) {
	schemaSvc := schema.NewService(ctx)
	schemaHandler := schema.NewHandler(schemaSvc)
	g.GET("", schemaHandler.List)
	g.GET("/", schemaHandler.List)
	g.POST("", schemaHandler.Create)
	g.POST("/", schemaHandler.Create)
	g.GET("/:id", schemaHandler.Get)
	g.PUT("/:id", schemaHandler.Update)
	g.DELETE("/:id", schemaHandler.Delete)
	g.POST("/:id/validate", schemaHandler.Validate)
	g.POST("/raw-validate", schemaHandler.RawValidate)
	g.GET("/:id/ui-config", schemaHandler.GetUIConfig)
	g.PUT("/:id/ui-config", schemaHandler.UpdateUIConfig)
}

// ============================================================================
// Terms 路由注册
// ============================================================================
func registerTermsRoutes(g *gin.RouterGroup, ctx *svc.ServiceContext) {
	termsSvc := terms.NewService(ctx)
	termsHandler := terms.NewHandler(termsSvc)
	g.GET("", termsHandler.List)
	g.GET("/", termsHandler.List)
	g.PUT("", termsHandler.Upsert)
	g.PUT("/", termsHandler.Upsert)
	g.DELETE("", termsHandler.Delete)
	g.DELETE("/", termsHandler.Delete)
}

// ============================================================================
// Ticket 路由注册
// ============================================================================
func registerTicketRoutes(g *gin.RouterGroup, ctx *svc.ServiceContext) {
	ticketSvc := ticket.NewService(ctx)
	ticketHandler := ticket.NewHandler(ticketSvc)
	g.GET("", ticketHandler.List)
	g.GET("/", ticketHandler.List)
	// #21：分类/处理人过滤选项由服务端聚合提供（页面禁止客户端推导）
	g.GET("/filter-options", ticketHandler.FilterOptions)
	g.POST("", ticketHandler.Create)
	g.POST("/", ticketHandler.Create)
	g.GET("/:id", ticketHandler.Get)
	g.PUT("/:id", ticketHandler.Update)
	g.DELETE("/:id", ticketHandler.Delete)
	g.POST("/:id/transition", ticketHandler.Transition)
	// 满意度评价（CSAT，仅已解决/已关闭工单；重开自动清分）
	g.POST("/:id/rate", ticketHandler.Rate)
	// 升级为缺陷（bug-tracking P2：携带玩家上下文，source=ticket 反查来源）
	g.POST("/:id/convert-bug", ticketHandler.ConvertToBug)
	// bug↔工单多对多关联（#25）：工单侧列出/添加/解除 bug 关联
	g.GET("/:id/bugs", ticketHandler.ListBugs)
	g.POST("/:id/bugs", ticketHandler.LinkBug)
	g.DELETE("/:id/bugs/:bugId", ticketHandler.UnlinkBug)
	g.GET("/:id/comments", ticketHandler.GetComments)
	g.POST("/:id/comments", ticketHandler.CreateComment)
}

func registerPlayerSupportRoutes(g *gin.RouterGroup, ctx *svc.ServiceContext) {
	supportapi.RegisterPlayerRoutes(g, ctx)
}

func registerPublicConfigRoutes(g *gin.RouterGroup, ctx *svc.ServiceContext) {
	pub := config.NewPublicHandler(config.NewPublicService(ctx))
	g.GET("/public/configs", pub.List)
}

func registerHotpatchRoutes(g *gin.RouterGroup, ctx *svc.ServiceContext) {
	hpSvc := hotpatch.NewService(ctx)
	hpHandler := hotpatch.NewHandler(hpSvc)
	g.GET("", hpHandler.List)
	g.GET("/", hpHandler.List)
	g.POST("", hpHandler.Create)
	g.POST("/", hpHandler.Create)
	g.POST("/:id/package", hpHandler.UploadPackage)
	g.POST("/:id/transition", hpHandler.Transition)
	g.POST("/:id/results", hpHandler.ReportResult)
}
