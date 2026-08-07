package router

import (
	"context"

	adminapi "github.com/cuihairu/croupier/internal/api/admin"
	"github.com/cuihairu/croupier/internal/api/auth"
	configapi "github.com/cuihairu/croupier/internal/api/config"
	consoleapi "github.com/cuihairu/croupier/internal/api/console"
	extensionapi "github.com/cuihairu/croupier/internal/api/extension"
	functionapi "github.com/cuihairu/croupier/internal/api/function"
	gameapi "github.com/cuihairu/croupier/internal/api/game"
	monitoringapi "github.com/cuihairu/croupier/internal/api/monitoring"
	openapiapi "github.com/cuihairu/croupier/internal/api/openapi"
	opsapi "github.com/cuihairu/croupier/internal/api/ops"
	pageapi "github.com/cuihairu/croupier/internal/api/page"
	permissionapi "github.com/cuihairu/croupier/internal/api/permission"
	playerapi "github.com/cuihairu/croupier/internal/api/player"
	policyapi "github.com/cuihairu/croupier/internal/api/policy"
	"github.com/cuihairu/croupier/internal/api/profile"
	providerapi "github.com/cuihairu/croupier/internal/api/provider"
	registryapi "github.com/cuihairu/croupier/internal/api/registry"
	resourceapi "github.com/cuihairu/croupier/internal/api/resource"
	resourcecatalog "github.com/cuihairu/croupier/internal/api/resourcecatalog"
	roleapi "github.com/cuihairu/croupier/internal/api/role"
	taskapi "github.com/cuihairu/croupier/internal/api/task"
	"github.com/cuihairu/croupier/internal/config"
	extensioncatalog "github.com/cuihairu/croupier/internal/core/extension/catalog"
	extensioninstallation "github.com/cuihairu/croupier/internal/core/extension/installation"
	extensionmanifest "github.com/cuihairu/croupier/internal/core/extension/manifest"
	extensionruntime "github.com/cuihairu/croupier/internal/core/extension/runtime"
	extensionsync "github.com/cuihairu/croupier/internal/core/extension/sync"
	"github.com/cuihairu/croupier/internal/db"
	"github.com/cuihairu/croupier/internal/middleware"
	"github.com/cuihairu/croupier/internal/model"
	dispatch "github.com/cuihairu/croupier/internal/platform/dispatch"
	reg "github.com/cuihairu/croupier/internal/platform/registry"
	extensiongorm "github.com/cuihairu/croupier/internal/repo/gorm/extension"
	"github.com/cuihairu/croupier/internal/security/jwtutil"
	"github.com/cuihairu/croupier/internal/service"
	permissionservice "github.com/cuihairu/croupier/internal/service/permission"
	versioning "github.com/cuihairu/croupier/internal/service/versioning"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"log/slog"
)

// RegisterRoutes 注册所有路由
func RegisterRoutes(r *gin.Engine, cfg *config.Config) error {
	// 初始化数据库
	db, err := initDatabase(cfg)
	if err != nil {
		return err
	}

	// 创建 API 路由组
	api := r.Group("/api/v1")

	// 注册公开路由（无需认证）
	registerPublicRoutes(api, db, cfg)

	// 注册需要认证的路由
	registerAuthenticatedRoutes(api, db, cfg)

	return nil
}

// registerPublicRoutes 注册公开路由
func registerPublicRoutes(api *gin.RouterGroup, db *gorm.DB, cfg *config.Config) {
	// Auth 模块
	jwtSecret, err := jwtutil.ResolveSecret(*cfg)
	if err != nil {
		// Log error but continue with fallback for compatibility
		slog.Default().Warn("JWT secret resolution warning, using fallback", "error", err)
		// Use fallback for backward compatibility
		if jwtSecret == "" {
			jwtSecret = jwtutil.DevSecret()
		}
	}
	authHandler := auth.NewHandler(auth.NewService(
		model.NewAdminModel(db),
		permissionservice.NewPermissionService(db),
		jwtSecret,
	))
	jwtutil.InitGlobalSecret(jwtSecret)
	authGroup := api.Group("/auth")
	{
		authGroup.POST("/login", authHandler.Login)
		authGroup.POST("/logout", authHandler.Logout)
	}

	// Health check
	api.GET("/monitoring/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})
}

// registerAuthenticatedRoutes 注册需要认证的路由
func registerAuthenticatedRoutes(api *gin.RouterGroup, db *gorm.DB, cfg *config.Config) {
	// 应用认证中间件
	authenticated := api.Group("")
	authenticated.Use(middleware.Auth())

	registryStore := reg.NewStoreWithDB(db)
	extensionRepos := extensiongorm.NewBundle(db)

	// Wire ContractService into registry store so gRPC function registration
	// automatically persists FunctionContracts.
	contractService := service.NewContractService(db)
	registryStore.SetContractService(contractService)
	registryStore.SetScopeContextResolver(func(gameID, env string) context.Context {
		return svc.WithGameScope(context.Background(), svc.GameScope{GameID: gameID, Env: env})
	})

	svcCtx := &svc.ServiceContext{
		DB:                        db,
		RegistryStore:             registryStore,
		Dispatcher:                dispatch.NewDispatcherWithTaskStore(registryStore, nil, nil),
		OpsStateStore:             svc.NewOpsStateStore("."),
		AdminModel:                model.NewAdminModel(db),
		GameModel:                 model.NewGameModel(db),
		PlayerModel:               model.NewPlayerModel(db),
		FunctionModel:             model.NewFunctionModel(db),
		RoleModel:                 model.NewRoleModel(db),
		PermissionModel:           model.NewPermissionModel(db),
		ConfigVersionModel:        model.NewConfigVersionModel(db),
		PageSpecModel:             model.NewPageSpecModel(db),
		PublishedPageSpecModel:    model.NewPublishedPageSpecModel(db),
		PageVersionModel:          model.NewPageVersionModel(db),
		OpenAPISourceModel:        model.NewOpenAPISourceModel(db),
		OpenAPISourceBindingModel: model.NewOpenAPISourceBindingModel(db),
		Extensions: &svc.ExtensionServices{
			Catalog:      extensioncatalog.NewService(extensionRepos.Catalog, extensionRepos.Release),
			Manifest:     extensionmanifest.NewService(),
			Installation: extensioninstallation.NewService(extensionRepos.Installation, extensionRepos.Event, extensionRepos.Binding),
			Runtime:      extensionruntime.NewService(extensionRepos.Installation, extensionRepos.Binding, extensionRepos.Event),
			Sync:         extensionsync.NewService(extensionRepos.Installation, extensionRepos.Binding),
		},
	}

	// 设置 TaskRunWriter 以便在调度任务时持久化 task_runs 记录
	taskRunModel := model.NewTaskRunModel(db)
	taskRunWriter := dispatch.NewTaskRunWriterAdapter(taskRunModel)
	svcCtx.Dispatcher.SetTaskRunWriter(taskRunWriter)

	if cfg != nil {
		svcCtx.Config = *cfg
	}

	// Profile 模块
	profileHandler := profile.NewHandler(profile.NewService(
		svcCtx.AdminModel,
		svcCtx.GameModel,
		svcCtx.RoleModel,
	))
	profileGroup := authenticated.Group("/profile")
	{
		profileGroup.GET("", profileHandler.GetProfile)
		profileGroup.GET("/games", profileHandler.GetGames)
		profileGroup.PUT("", profileHandler.UpdateProfile)
		profileGroup.PUT("/password", profileHandler.ChangePassword)
	}

	registerRoleRoutes(authenticated, svcCtx)
	registerPermissionRoutes(authenticated, svcCtx)
	registerGameRoutes(authenticated, svcCtx)
	registerPolicyRoutes(authenticated, svcCtx)
	registerAdminRoutes(authenticated, svcCtx)
	registerPlayerRoutes(authenticated, svcCtx)
	registerTaskRoutes(authenticated, svcCtx)
	registerRegistryRoutes(authenticated, svcCtx)
	registerFunctionRoutes(authenticated, svcCtx)
	registerConfigRoutes(authenticated, svcCtx)
	registerExtensionRoutes(authenticated, svcCtx)
	registerOpsRoutes(authenticated, svcCtx)
	registerMonitoringRoutes(authenticated, svcCtx)
	registerOpenAPIRoutes(authenticated, svcCtx)
	registerProviderRoutes(authenticated, svcCtx)
	registerResourceRoutes(authenticated, svcCtx)
	registerConsoleRoutes(authenticated, svcCtx)
	registerPageRoutes(authenticated, svcCtx)
	registerResourceCatalogRoutes(authenticated, svcCtx)
	registerVersioningRoutes(authenticated, svcCtx)
	registerContractRoutes(authenticated, svcCtx, contractService)
	registerProposalRoutes(authenticated, svcCtx)
	registerExportRoutes(authenticated, svcCtx)
}

func registerRoleRoutes(authenticated *gin.RouterGroup, svcCtx *svc.ServiceContext) {
	handler := roleapi.NewHandler(roleapi.NewService(svcCtx))
	group := authenticated.Group("/roles")
	{
		group.GET("", handler.RolesList)
		group.GET("/:id", handler.RoleDetail)
		group.POST("", handler.RoleCreate)
		group.PUT("/:id", handler.RoleUpdate)
		group.DELETE("/:id", handler.RoleDelete)
	}
}

func registerPermissionRoutes(authenticated *gin.RouterGroup, svcCtx *svc.ServiceContext) {
	handler := permissionapi.NewHandler(permissionapi.NewService(svcCtx))
	group := authenticated.Group("/permissions")
	{
		group.GET("", handler.List)
		group.GET("/:id", handler.Detail)
	}
}

func registerGameRoutes(authenticated *gin.RouterGroup, svcCtx *svc.ServiceContext) {
	handler := gameapi.NewHandler(gameapi.NewService(svcCtx))
	group := authenticated.Group("/games")
	{
		group.GET("", handler.List)
		group.POST("", handler.Create)
		group.GET("/:id", handler.Detail)
		group.PUT("/:id", handler.Update)
		group.DELETE("/:id", handler.Delete)
		group.GET("/:id/envs", handler.EnvsList)
		group.POST("/:id/envs", handler.EnvAdd)
		group.PUT("/:id/envs/:envId", handler.EnvUpdate)
		group.DELETE("/:id/envs/:envId", handler.EnvDelete)
	}
}

func registerAdminRoutes(authenticated *gin.RouterGroup, svcCtx *svc.ServiceContext) {
	handler := adminapi.NewHandler(adminapi.NewService(svcCtx))
	group := authenticated.Group("/admins")
	{
		group.GET("", handler.List)
		group.POST("", handler.Create)
		group.GET("/:id", handler.Get)
		group.PUT("/:id", handler.Update)
		group.DELETE("/:id", handler.Delete)
		group.POST("/:id/password/reset", handler.PasswordReset)
		group.GET("/:id/games", handler.GetGames)
		group.PUT("/:id/games", handler.UpdateGames)
	}
}

func registerPlayerRoutes(authenticated *gin.RouterGroup, svcCtx *svc.ServiceContext) {
	handler := playerapi.NewHandler(playerapi.NewService(svcCtx))
	group := authenticated.Group("/players")
	{
		group.GET("", handler.List)
		group.POST("", handler.Create)
		group.GET("/:id", handler.Detail)
		group.PUT("/:id", handler.Update)
		group.DELETE("/:id", handler.Delete)
		group.POST("/:id/balance", handler.Balance)
	}
}

func registerTaskRoutes(authenticated *gin.RouterGroup, svcCtx *svc.ServiceContext) {
	handler := taskapi.NewHandler(taskapi.NewService(svcCtx))
	group := authenticated.Group("/tasks")
	{
		group.GET("", handler.List)
		group.POST("", handler.Start)
		group.POST("/cancel", handler.CancelByBody)
		group.GET("/:id", handler.Detail)
		group.GET("/:id/events", handler.Events)
		group.POST("/:id/cancel", handler.Cancel)
	}
}

func registerRegistryRoutes(authenticated *gin.RouterGroup, svcCtx *svc.ServiceContext) {
	handler := registryapi.NewHandler(registryapi.NewService(svcCtx))
	group := authenticated.Group("/registry")
	{
		group.GET("", handler.GetRegistry)
		group.POST("", handler.GetRegistry)
	}
}

func registerFunctionRoutes(authenticated *gin.RouterGroup, svcCtx *svc.ServiceContext) {
	handler := functionapi.NewHandler(functionapi.NewService(svcCtx))
	policyHandler := policyapi.NewHandler(svcCtx.PolicyManager)
	group := authenticated.Group("/functions")
	{
		group.GET("", handler.List)
		group.GET("/pending", handler.Pending)
		group.GET("/instances/all", handler.InstancesAll)
		group.GET("/warnings", handler.Warnings)
		group.POST("/batch/copy", handler.BatchCopyFunctions)
		group.POST("/batch/delete", handler.BatchDeleteFunctions)
		group.POST("/batch/update", handler.BatchUpdateFunctions)
		group.GET("/:id", handler.Detail)
		group.GET("/:id/history", handler.History)
		group.GET("/:id/analytics", handler.Analytics)
		group.POST("/:id/invoke", handler.Invoke)
		group.POST("/:id/publish", handler.Publish)
		group.GET("/:id/instances", handler.Instances)
		group.GET("/:id/permissions", handler.Permissions)
		group.POST("/:id/permissions", handler.PermissionsUpdate)
		// Policy routes for individual functions
		group.GET("/:id/policy", policyHandler.GetPolicy)
		group.PUT("/:id/policy", policyHandler.SetPolicy)
		group.DELETE("/:id/policy", policyHandler.DeletePolicy)
	}
}

func registerConfigRoutes(authenticated *gin.RouterGroup, svcCtx *svc.ServiceContext) {
	handler := configapi.NewHandler(configapi.NewService(svcCtx))
	group := authenticated.Group("/configs")
	{
		group.GET("", handler.List)
		group.GET("/:id", handler.Get)
		group.PUT("/:id", handler.Save)
		group.POST("/:id/validate", handler.Validate)
		group.GET("/:id/versions", handler.ListVersionsByID)
		group.GET("/:id/versions/:version", handler.GetVersionByID)
	}
}

func registerExtensionRoutes(authenticated *gin.RouterGroup, svcCtx *svc.ServiceContext) {
	handler := extensionapi.NewHandler(extensionapi.NewService(svcCtx))
	group := authenticated.Group("/extensions")
	{
		group.GET("/catalog", handler.CatalogList)
		group.GET("/catalog/:id", handler.CatalogDetail)
		group.GET("/catalog/:id/releases", handler.CatalogReleases)
		group.GET("/installations", handler.InstallationList)
		group.POST("/installations", handler.Install)
		group.GET("/installations/:id", handler.InstallationDetail)
		group.GET("/installations/:id/config", handler.Config)
		group.GET("/installations/:id/config/schema", handler.ConfigSchema)
		group.GET("/installations/:id/events", handler.Events)
		group.GET("/installations/:id/capabilities", handler.Capabilities)
		group.GET("/installations/:id/pages", handler.Pages)
	}
}

func registerOpsRoutes(authenticated *gin.RouterGroup, svcCtx *svc.ServiceContext) {
	handler := opsapi.NewHandler(opsapi.NewService(svcCtx))
	group := authenticated.Group("/ops")
	{
		group.GET("/agents", handler.AgentsList)
		group.GET("/alerts", handler.Alerts)
		group.POST("/alerts/silence", handler.AlertSilence)
		group.GET("/backups", handler.BackupsList)
		group.POST("/backups", handler.BackupCreate)
		group.POST("/backup/delete", handler.BackupDelete)
		group.POST("/backup/download", handler.BackupDownload)
		group.GET("/config", handler.Config)
		group.GET("/functions", handler.Functions)
		group.GET("/health", handler.HealthGet)
		group.POST("/health/run", handler.HealthRun)
		group.POST("/health/update", handler.HealthUpdate)
		group.GET("/maintenance", handler.MaintenanceGet)
		group.POST("/maintenance/update", handler.MaintenanceUpdate)
		group.GET("/metrics", handler.Metrics)
		group.GET("/mq", handler.MQ)
		group.GET("/nodes", handler.Nodes)
		group.POST("/node/commands", handler.NodeCommands)
		group.POST("/node/drain", handler.NodeDrain)
		group.GET("/node/meta", handler.NodeMeta)
		group.POST("/node/restart", handler.NodeRestart)
		group.POST("/node/undrain", handler.NodeUndrain)
		group.GET("/notifications", handler.NotificationsGet)
		group.POST("/notifications/update", handler.NotificationsUpdate)
		group.GET("/services", handler.Services)
		group.GET("/silences", handler.Silences)
		group.GET("/agent/meta", handler.AgentMeta)
		group.GET("/agent/metrics", handler.AgentMetrics)
		group.GET("/agent/metrics/history", handler.AgentMetricsHistory)
		group.GET("/agent/processes", handler.AgentProcesses)
		group.GET("/agent/system-info", handler.AgentSystemInfo)
		group.POST("/agent/exec", handler.AgentExecCommand)
		group.POST("/agent/process/start", handler.AgentProcessStart)
		group.POST("/agent/process/stop", handler.AgentProcessStop)
		group.POST("/agent/process/restart", handler.AgentProcessRestart)
	}
}

func registerMonitoringRoutes(authenticated *gin.RouterGroup, svcCtx *svc.ServiceContext) {
	handler := monitoringapi.NewHandler(monitoringapi.NewService(svcCtx))
	group := authenticated.Group("/monitoring")
	{
		group.GET("/healthz", handler.Healthz)
		group.GET("/metrics", handler.Metrics)
		group.GET("/status", handler.Status)
	}
}

func registerOpenAPIRoutes(authenticated *gin.RouterGroup, svcCtx *svc.ServiceContext) {
	handler := openapiapi.NewHandler(openapiapi.NewService(svcCtx))
	group := authenticated.Group("/openapi")
	{
		group.GET("/document", handler.GetDocument)
		group.POST("/batch/spec", handler.BatchGetSpec)
		group.GET("/functions/:id/spec", handler.GetSpec)
		group.GET("/sources", handler.ListSources)
		group.POST("/sources", handler.CreateSource)
		group.GET("/sources/:sourceId", handler.GetSource)
		group.PUT("/sources/:sourceId", handler.UpdateSource)
		group.GET("/sources/:sourceId/diagnostics", handler.SourceDiagnostics)
		group.POST("/sources/:sourceId/bindings", handler.CreateBinding)
		group.DELETE("/sources/:sourceId/bindings/:bindingId", handler.DeleteBinding)
	}
}

func registerProviderRoutes(authenticated *gin.RouterGroup, svcCtx *svc.ServiceContext) {
	handler := providerapi.NewHandler(providerapi.NewService(svcCtx))
	group := authenticated.Group("/providers")
	{
		group.GET("", handler.List)
		group.GET("/capabilities", handler.Capabilities)
		group.GET("/descriptors", handler.Descriptors)
		group.GET("/:id", handler.Get)
		group.GET("/:id/resources", handler.Resources)
		group.DELETE("/:id", handler.Delete)
		group.POST("/:id/reload", handler.Reload)
	}
}

func registerResourceRoutes(authenticated *gin.RouterGroup, svcCtx *svc.ServiceContext) {
	handler := resourceapi.NewHandler(resourceapi.NewService(svcCtx))
	group := authenticated.Group("/resources")
	{
		group.GET("", handler.List)
		group.GET("/:resourceKey", handler.Detail)
		group.GET("/:resourceKey/operations", handler.Operations)
	}
}

func registerConsoleRoutes(authenticated *gin.RouterGroup, svcCtx *svc.ServiceContext) {
	handler := consoleapi.NewHandler(consoleapi.NewService(svcCtx))
	group := authenticated.Group("/console")
	{
		group.GET("/menu", handler.Menu)
		group.GET("/pages", handler.Pages)
		group.GET("/pages/:pageKey", handler.Page)
		group.POST("/pages/:pageKey/bindings/:bindingId/execute", handler.ExecuteBinding)
	}
}

func registerPageRoutes(authenticated *gin.RouterGroup, svcCtx *svc.ServiceContext) {
	handler := pageapi.NewHandler(pageapi.NewService(svcCtx))
	group := authenticated.Group("/pages")
	{
		group.GET("", handler.ListDrafts)
		group.GET("/:pageKey", handler.GetDraft)
		group.PUT("/:pageKey", handler.SaveDraft)
		group.POST("/:pageKey/regenerate", handler.RegenerateDraft)
		group.POST("/:pageKey/validate", handler.Validate)
		group.POST("/:pageKey/preview", handler.Preview)
		group.POST("/:pageKey/publish", handler.Publish)
		group.POST("/:pageKey/unpublish", handler.Unpublish)
		group.GET("/:pageKey/versions", handler.Versions)
		group.GET("/:pageKey/versions/:versionId", handler.VersionDetail)
		group.POST("/:pageKey/rollback", handler.Rollback)
	}
}

func registerPolicyRoutes(authenticated *gin.RouterGroup, svcCtx *svc.ServiceContext) {
	handler := policyapi.NewHandler(svcCtx.PolicyManager)
	policiesGroup := authenticated.Group("/policies")
	{
		policiesGroup.GET("/defaults", handler.GetDefaultPolicies)
		policiesGroup.GET("/overrides", handler.ListOverrides)
		policiesGroup.POST("/reload", handler.ReloadConfig)
	}
}

func registerResourceCatalogRoutes(authenticated *gin.RouterGroup, svcCtx *svc.ServiceContext) {
	service := resourcecatalog.NewService(svcCtx.DB, svcCtx.AuditService)
	handler := resourcecatalog.NewHandler(service)
	group := authenticated.Group("/resource-catalog")
	{
		group.GET("", handler.List)
		group.GET("/:resourceKey", handler.Detail)
		group.PUT("/:resourceKey/semantics", handler.UpdateSemantics)
		group.GET("/:resourceKey/semantics/versions", handler.ListSemanticVersions)
		group.GET("/:resourceKey/conflicts", handler.ListConflicts)
		group.POST("/:resourceKey/conflicts/:field/resolve", handler.ResolveConflict)
	}
}

func registerVersioningRoutes(authenticated *gin.RouterGroup, svcCtx *svc.ServiceContext) {
	service := versioning.NewService(svcCtx.DB)
	handler := versioning.NewHandler(service)
	group := authenticated.Group("/versioning")
	{
		group.GET("/pages/:pageKey/chain", handler.GetChangeChain)
		group.GET("/pages/:pageKey/diff", handler.Diff)
		group.POST("/pages/:pageKey/merge", handler.Merge)
		group.POST("/pages/:pageKey/rollback-draft", handler.RollbackDraft)
		group.POST("/pages/:pageKey/rollback-publish", handler.RollbackPublish)
		group.POST("/pages/:pageKey/regenerate", handler.RegenerateProposal)
		group.POST("/pages/:pageKey/republish", handler.Republish)
	}
}

func registerContractRoutes(authenticated *gin.RouterGroup, svcCtx *svc.ServiceContext, contractService *service.ContractService) {
	handler := service.NewContractHandler(contractService)
	group := authenticated.Group("/contracts")
	{
		group.GET("", handler.ListContracts)
		group.GET("/:functionId", handler.GetContract)
	}
	capGroup := authenticated.Group("/resource-capabilities")
	{
		capGroup.GET("", handler.ListResourceCapabilities)
	}
}

func registerProposalRoutes(authenticated *gin.RouterGroup, svcCtx *svc.ServiceContext) {
	proposalService := service.NewProposalService(svcCtx.DB)
	handler := service.NewProposalHandler(proposalService)
	group := authenticated.Group("/proposals")
	{
		group.GET("", handler.ListProposals)
		group.GET("/inbox", handler.Inbox)
		group.GET("/:proposalKey", handler.GetProposal)
		group.POST("/:proposalKey/accept", handler.AcceptProposal)
		group.POST("/:proposalKey/accept-and-publish", handler.AcceptAndPublishProposal)
		group.POST("/:proposalKey/reject", handler.RejectProposal)
	}
}

func registerExportRoutes(authenticated *gin.RouterGroup, svcCtx *svc.ServiceContext) {
	exportService := service.NewDataExportService(svcCtx.DB)
	handler := service.NewExportHandler(exportService)
	group := authenticated.Group("/export")
	{
		group.GET("/pages", handler.ExportPages)
	}
}

// initDatabase 初始化数据库连接
func initDatabase(cfg *config.Config) (*gorm.DB, error) {
	if cfg == nil {
		return nil, gorm.ErrInvalidDB
	}

	return db.Open(cfg.Database.DataSource)
}
