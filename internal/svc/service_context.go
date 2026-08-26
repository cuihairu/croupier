package svc

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/cuihairu/croupier/internal/audit"
	"github.com/cuihairu/croupier/internal/cache"
	"github.com/cuihairu/croupier/internal/config"
	extensioncatalog "github.com/cuihairu/croupier/internal/core/extension/catalog"
	extensioninstallation "github.com/cuihairu/croupier/internal/core/extension/installation"
	extensionmanifest "github.com/cuihairu/croupier/internal/core/extension/manifest"
	extensionruntime "github.com/cuihairu/croupier/internal/core/extension/runtime"
	extensionsync "github.com/cuihairu/croupier/internal/core/extension/sync"
	"github.com/cuihairu/croupier/internal/db/dbctx"
	"github.com/cuihairu/croupier/internal/db/migrate"
	"github.com/cuihairu/croupier/internal/db/router"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/platform/approvals"
	dispatch "github.com/cuihairu/croupier/internal/platform/dispatch"
	objstore "github.com/cuihairu/croupier/internal/platform/objstore"
	reg "github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/cuihairu/croupier/internal/platform/tlsutil"
	policymgr "github.com/cuihairu/croupier/internal/policy"
	extensiongorm "github.com/cuihairu/croupier/internal/repo/gorm/extension"
	"github.com/cuihairu/croupier/internal/runtime"
	jwtutil "github.com/cuihairu/croupier/internal/security/jwtutil"
	"github.com/cuihairu/croupier/internal/service/permission"
	"github.com/cuihairu/croupier/internal/telemetry"
	"github.com/gin-gonic/gin"
	"gorm.io/datatypes"
	"gorm.io/gorm"
	"log/slog"
)

type ServiceContext struct {
	Config        config.Config
	Authority     gin.HandlerFunc
	AdminManager  *AdminManager
	OpsStateStore *OpsStateStore
	DB            *gorm.DB
	// Router manages per-game databases when Config.Database.MultiGame is
	// enabled. It is nil in legacy single-database mode.
	Router            *router.Router
	PermissionService *permission.PermissionService
	RegistryStore     *reg.Store
	Dispatcher        *dispatch.Dispatcher
	Cache             cache.CacheStore
	CacheHelper       *cache.CacheHelper

	AnalyticsFiltersLock *sync.RWMutex

	ApprovalsStore approvals.Store
	PolicyManager  *policymgr.Manager
	AuditService   *audit.AuditService

	ObjectStore objstore.Store
	Telemetry   *telemetry.GameTelemetryService

	// Agent Ops support
	MetricsStore         *reg.MetricsStore
	SystemInfoCache      *reg.SystemInfoCache
	AgentSessionResolver dispatch.AgentSessionResolver

	AdminModel         *model.AdminModel
	AlertModel         *model.AlertModel
	BehaviorModel      *model.BehaviorModel
	RetentionModel     *model.RetentionModel
	PaymentsModel      *model.PaymentsModel
	BackupModel        *model.BackupModel
	FAQModel           *model.FAQModel
	FeedbackModel      *model.FeedbackModel
	GameModel          *model.GameModel
	PlayerModel        *model.PlayerModel
	ProfileModel       *model.ProfileModel
	FunctionModel      *model.FunctionModel
	TermDictModel      *model.TermDictionaryModel
	RoleModel          *model.RoleModel
	NodeModel          *model.NodeModel
	PermissionModel    *model.PermissionModel
	RateLimitModel     *model.RateLimitModel
	SupportModel       *model.SupportModel
	TicketModel        *model.TicketModel
	BugModel           *model.BugModel
	ToolModel          *model.ToolLinkModel
	ReleaseModel       *model.GameReleaseModel
	HotpatchModel      *model.HotpatchModel
	MessageModel       *model.MessageModel
	CertificateModel   *model.CertificateModel
	ConfigVersionModel *model.ConfigVersionModel

	// Page Spec models
	PageSpecModel             *model.PageSpecModel
	PublishedPageSpecModel    *model.PublishedPageSpecModel
	PageVersionModel          *model.PageVersionModel
	OpenAPISourceModel        *model.OpenAPISourceModel
	OpenAPISourceBindingModel *model.OpenAPISourceBindingModel

	// Agent Session 持久化
	AgentSessionModel *reg.AgentSessionModel

	// 版本信息（由 build 脚本通过 ldflags 注入）
	ServerVersion   string
	ServerGitCommit string
	ServerBuildTime string

	// StartTime 记录服务器启动时间
	StartTime time.Time

	Extensions *ExtensionServices
}

type ExtensionServices struct {
	Catalog      *extensioncatalog.Service
	Manifest     *extensionmanifest.Service
	Installation *extensioninstallation.Service
	Runtime      *extensionruntime.Service
	Sync         *extensionsync.Service
}

func NewServiceContext(c config.Config, opts ...Option) *ServiceContext {
	// 初始化数据库连接
	slog.Default().Info("Connecting to database", "driver", c.Database.Driver, "dataSource", c.Database.DataSource)
	db, err := openDatabase(c)
	if err != nil {
		panic(fmt.Sprintf("Failed to connect to database: %v", err))
	}
	slog.Default().Info("Database connected successfully")

	// 自动迁移数据库模型
	slog.Default().Info("Starting database migration")
	multiGame := c.Database.MultiGame
	migrateCtx := context.Background()
	if multiGame {
		if _, err := migrateMeta(migrateCtx, db); err != nil {
			panic(fmt.Sprintf("Failed to migrate meta database: %v", err))
		}
	} else {
		if _, err := migrate.EnsureUpToDate(migrateCtx, db, migrate.ScopeSingle, autoMigrate); err != nil {
			panic(fmt.Sprintf("Failed to migrate database: %v", err))
		}
	}
	slog.Default().Info("Database migration completed successfully", "multiGame", multiGame)

	// 创建 per-game 数据库路由（当启用 multiGame 时）
	var dbRouter *router.Router
	if multiGame {
		dbRouter = newGameRouter(c, db)
		slog.Default().Info("Database-per-game router initialized")
	}

	// 创建服务
	permissionService := permission.NewPermissionService(db)

	// 模型实例（保持在同一处构建，便于逻辑层复用）
	adminModel := model.NewAdminModel(db)
	alertModel := model.NewAlertModel(db)
	behaviorModel := model.NewBehaviorModel(db)
	retentionModel := model.NewRetentionModel(db)
	paymentsModel := model.NewPaymentsModel(db)
	backupModel := model.NewBackupModel(db)
	faqModel := model.NewFAQModel(db)
	feedbackModel := model.NewFeedbackModel(db)
	gameModel := model.NewGameModel(db)
	playerModel := model.NewPlayerModel(db)
	profileModel := model.NewProfileModel(db)
	functionModel := model.NewFunctionModel(db)
	termDictModel := model.NewTermDictionaryModel(db)
	roleModel := model.NewRoleModel(db)
	nodeModel := model.NewNodeModel(db)
	permissionModel := model.NewPermissionModel(db)
	rateLimitModel := model.NewRateLimitModel(db)
	supportModel := model.NewSupportModel(db)
	ticketModel := model.NewTicketModel(db)
	bugModel := model.NewBugModel(db)
	toolModel := model.NewToolLinkModel(db)
	releaseModel := model.NewGameReleaseModel(db)
	hotpatchModel := model.NewHotpatchModel(db)
	messageModel := model.NewMessageModel(db)
	certificateModel := model.NewCertificateModel(db)
	configVersionModel := model.NewConfigVersionModel(db)

	// game_envs is the authoritative scope and routing registry. Older
	// deployments stored environments only in games.envs JSON, so backfill
	// missing rows before any authenticated request can resolve a scope.
	databaseNameFor := router.DefaultGameDBName
	if dbRouter != nil {
		databaseNameFor = dbRouter.NameForGame
	}
	if created, err := gameModel.BackfillEnvBindings(context.Background(), databaseNameFor); err != nil {
		panic(fmt.Sprintf("failed to backfill game environment bindings: %v", err))
	} else if created > 0 {
		slog.Default().Info("backfilled game environment bindings", "created", created)
	}

	// Page Spec models
	pageSpecModel := model.NewPageSpecModel(db)
	publishedPageSpecModel := model.NewPublishedPageSpecModel(db)
	pageVersionModel := model.NewPageVersionModel(db)
	openAPISourceModel := model.NewOpenAPISourceModel(db)
	openAPISourceBindingModel := model.NewOpenAPISourceBindingModel(db)

	// Agent Session Model for database persistence
	agentSessionModel := reg.NewAgentSessionModel(db)

	// 创建管理员管理器（基于JSON文件）
	configDir := resolveBootstrapAuthDir(c)
	slog.Default().Info("Initializing AdminManager", "configDir", configDir)
	adminManager := NewAdminManager(configDir)
	if err := adminManager.Initialize(); err != nil {
		// 如果初始化失败，记录错误但不停止服务
		slog.Default().Error("Failed to initialize AdminManager", "error", err)
		// 这样可以让服务启动，但登录功能可能受限
		// 在生产环境中应该更严格地处理这个错误
	} else {
		// 记录加载的管理员数量
		admins := adminManager.ListAdmins()
		slog.Default().Info("AdminManager initialized", "loadedAdmins", len(admins), "configDir", configDir)
		if len(admins) == 0 {
			slog.Default().Warn("No admins loaded from config, admin accounts will not be created")
		}
	}

	opsStateStore := NewOpsStateStore(resolveBootstrapBaseDir(c))

	objectStore, err := initObjectStore(context.Background(), c.Storage)
	if err != nil {
		panic(fmt.Sprintf("Failed to initialize object store: %v", err))
	}

	approvalsStore := approvals.NewMemStore()

	// 初始化审计服务
	auditStore, err := audit.NewSQLAuditStore(db)
	if err != nil {
		slog.Default().Error("Failed to initialize audit store", "error", err)
		// Continue without audit service
		auditStore = nil
	}
	var auditSvc *audit.AuditService
	if auditStore != nil {
		auditSvc = audit.NewAuditService(auditStore, nil)
		slog.Default().Info("AuditService initialized")
	}

	// 初始化策略管理器
	policyConfigPath := filepath.Join(resolveBootstrapBaseDir(c), "default-policies.yaml")
	policyManager, err := policymgr.NewManager(db, policyConfigPath)
	if err != nil {
		slog.Default().Error("Failed to initialize PolicyManager", "error", err)
		// Continue without policy manager - system will use defaults
		policyManager = nil
	} else {
		slog.Default().Info("PolicyManager initialized", "configPath", policyConfigPath)
	}

	// 初始化缓存
	cacheStore, err := cache.NewCacheStore(c.Cache)
	if err != nil {
		slog.Default().Error("Failed to initialize cache, using NullCache", "error", err)
		cacheStore = cache.NewNullCache()
	}
	cacheHelper := cache.NewCacheHelper(cacheStore)
	extensionRepos := extensiongorm.NewBundle(db)
	extensionManifestSvc := extensionmanifest.NewService()
	extensionCatalogSvc := extensioncatalog.NewService(extensionRepos.Catalog, extensionRepos.Release)
	extensionInstallationSvc := extensioninstallation.NewService(extensionRepos.Installation, extensionRepos.Event, extensionRepos.Binding)
	extensionRuntimeSvc := extensionruntime.NewService(extensionRepos.Installation, extensionRepos.Binding, extensionRepos.Event)
	extensionSyncSvc := extensionsync.NewService(extensionRepos.Installation, extensionRepos.Binding)

	ctx := &ServiceContext{
		Config:            c,
		AdminManager:      adminManager,
		OpsStateStore:     opsStateStore,
		DB:                db,
		Router:            dbRouter,
		PermissionService: permissionService,
		Cache:             cacheStore,
		CacheHelper:       cacheHelper,

		AdminModel:                adminModel,
		AlertModel:                alertModel,
		BehaviorModel:             behaviorModel,
		RetentionModel:            retentionModel,
		PaymentsModel:             paymentsModel,
		BackupModel:               backupModel,
		FAQModel:                  faqModel,
		FeedbackModel:             feedbackModel,
		GameModel:                 gameModel,
		PlayerModel:               playerModel,
		ProfileModel:              profileModel,
		FunctionModel:             functionModel,
		TermDictModel:             termDictModel,
		RoleModel:                 roleModel,
		NodeModel:                 nodeModel,
		PermissionModel:           permissionModel,
		RateLimitModel:            rateLimitModel,
		SupportModel:              supportModel,
		TicketModel:               ticketModel,
		BugModel:                  bugModel,
		ToolModel:                 toolModel,
		ReleaseModel:              releaseModel,
		HotpatchModel:             hotpatchModel,
		MessageModel:              messageModel,
		CertificateModel:          certificateModel,
		ConfigVersionModel:        configVersionModel,
		PageSpecModel:             pageSpecModel,
		PublishedPageSpecModel:    publishedPageSpecModel,
		PageVersionModel:          pageVersionModel,
		OpenAPISourceModel:        openAPISourceModel,
		OpenAPISourceBindingModel: openAPISourceBindingModel,
		AgentSessionModel:         agentSessionModel,

		// 版本信息（从 version.go 读取，ldflags 注入后会更新）
		ServerVersion:   ServerVersion,
		ServerGitCommit: ServerGitCommit,
		ServerBuildTime: ServerBuildTime,

		// 记录启动时间
		StartTime:            time.Now(),
		AnalyticsFiltersLock: &sync.RWMutex{},

		ObjectStore:    objectStore,
		ApprovalsStore: approvalsStore,
		PolicyManager:  policyManager,
		AuditService:   auditSvc,
		Extensions: &ExtensionServices{
			Catalog:      extensionCatalogSvc,
			Manifest:     extensionManifestSvc,
			Installation: extensionInstallationSvc,
			Runtime:      extensionRuntimeSvc,
			Sync:         extensionSyncSvc,
		},
	}

	for _, opt := range opts {
		if opt != nil {
			opt(ctx)
		}
	}

	if ctx.RegistryStore == nil {
		// Use NewStoreWithDB to enable database dual-write and recovery
		ctx.RegistryStore = reg.NewStoreWithDB(ctx.DB)
	}
	ctx.RegistryStore.SetScopeContextResolver(ctx.scopeContextForBackgroundRegistration)
	if ctx.Dispatcher == nil {
		var taskStore dispatch.TaskRoutingStore
		taskRoutingDir := resolveTaskRoutingDir(ctx.Config)
		if taskRoutingDir != "" {
			store, err := dispatch.NewFileTaskRoutingStore(taskRoutingDir)
			if err != nil {
				slog.Default().Error("failed to init task routing store", "dir", taskRoutingDir, "error", err)
			} else {
				taskStore = store
			}
		}
		ctx.Dispatcher = dispatch.NewDispatcherWithTaskStore(ctx.RegistryStore, taskStore, nil)

		// 设置 TaskRunWriter 以便在调度任务时持久化 task_runs 记录
		taskRunModel := model.NewTaskRunModel(ctx.DB)
		taskRunWriter := dispatch.NewTaskRunWriterAdapter(taskRunModel)
		ctx.Dispatcher.SetTaskRunWriter(taskRunWriter)

		if ttlStr := strings.TrimSpace(ctx.Config.AgentDispatch.TaskRoutingTTL); ttlStr != "" {
			if ttl, err := time.ParseDuration(ttlStr); err != nil {
				slog.Default().Error("invalid dispatch.task_routing_ttl", "value", ttlStr, "error", err)
			} else if ttl > 0 {
				if err := ctx.Dispatcher.CleanupOldTasks(ttl); err != nil {
					slog.Default().Error("failed to cleanup old tasks", "error", err)
				}
			}
		}

		if ctx.Config.AgentDispatch.ToAgentTLS.Enabled {
			ctx.Dispatcher.SetTLSConfig(&tlsutil.ClientTLSConfig{
				CertFile:           strings.TrimSpace(ctx.Config.AgentDispatch.ToAgentTLS.CertFile),
				KeyFile:            strings.TrimSpace(ctx.Config.AgentDispatch.ToAgentTLS.KeyFile),
				CAFile:             strings.TrimSpace(ctx.Config.AgentDispatch.ToAgentTLS.CAFile),
				ServerName:         strings.TrimSpace(ctx.Config.AgentDispatch.ToAgentTLS.ServerName),
				InsecureSkipVerify: ctx.Config.AgentDispatch.ToAgentTLS.InsecureSkipVerify,
			})
		} else {
			ctx.Dispatcher.SetTLSConfig(nil)
		}
	}

	if err := seedBootstrapPermissions(ctx); err != nil {
		slog.Default().Error("failed to seed bootstrap permissions", "error", err)
	}
	if err := seedBootstrapExtensionCatalog(ctx); err != nil {
		slog.Default().Error("failed to seed bootstrap extension catalog", "error", err)
	}
	if err := seedBootstrapRoles(ctx); err != nil {
		slog.Default().Error("failed to seed bootstrap roles", "error", err)
	}
	if err := seedBootstrapAdmins(ctx); err != nil {
		slog.Default().Error("failed to seed bootstrap admins", "error", err)
	}
	if err := seedBootstrapGames(ctx); err != nil {
		slog.Default().Error("failed to seed bootstrap games", "error", err)
	}
	if err := seedBootstrapTermDictionary(ctx); err != nil {
		slog.Default().Error("failed to seed term dictionary", "error", err)
	}

	// Initialize agent ops stores
	if ctx.MetricsStore == nil {
		ctx.MetricsStore = reg.NewMetricsStore()
	}
	if ctx.SystemInfoCache == nil {
		ctx.SystemInfoCache = reg.NewSystemInfoCache()
	}

	// 初始化 JWT 密钥（从配置文件读取）
	secret, err := jwtutil.ResolveSecret(ctx.Config)
	if err != nil {
		// JWT secret 未配置且不在开发模式，这是一个严重配置错误
		slog.Default().Error("JWT secret configuration error", "error", err)
		// 在生产环境下应该启动失败
		if !isDevelopmentConfig(ctx.Config) {
			panic(fmt.Sprintf("JWT secret not configured: %v", err))
		}
		// 开发模式使用默认密钥
		secret = jwtutil.DevSecret()
		slog.Default().Warn("Using development JWT secret - do not use in production", "mode", ctx.Config.Server.Mode)
	}
	jwtutil.InitGlobalSecret(secret)

	// 设置认证中间件
	ctx.Authority = NewAuthMiddleware(ctx)

	return ctx
}

func (ctx *ServiceContext) scopeContextForBackgroundRegistration(gameID, env string) context.Context {
	base := WithGameScope(context.Background(), GameScope{
		GameID: strings.TrimSpace(gameID),
		Env:    strings.TrimSpace(env),
	})
	if ctx == nil || ctx.Router == nil || strings.TrimSpace(gameID) == "" {
		return base
	}
	gameDB, err := ctx.Router.GameDB(base, strings.TrimSpace(gameID), strings.TrimSpace(env))
	if err != nil {
		slog.Default().Error("failed to resolve game database for function registration",
			"gameId", strings.TrimSpace(gameID), "env", strings.TrimSpace(env), "error", err)
		return base
	}
	return dbctx.WithDB(base, gameDB)
}

func NewTelemetryService(c config.Config, serviceName string, logger *slog.Logger) (*telemetry.GameTelemetryService, error) {
	cfg := telemetry.MergeEnv(telemetry.TelemetryConfig{
		Enabled:        c.Telemetry.Enabled,
		ServiceName:    firstNonEmpty(c.Telemetry.ServiceName, serviceName),
		ServiceVersion: c.Telemetry.ServiceVersion,
		Environment:    firstNonEmpty(c.Telemetry.Environment, c.Server.Mode),
		CollectorURL:   c.Telemetry.CollectorURL,
		GameID:         c.Telemetry.GameID,
		EnableTracing:  c.Telemetry.EnableTracing,
		EnableMetrics:  c.Telemetry.EnableMetrics,
		SamplingRatio:  c.Telemetry.SamplingRatio,
		UseTLS:         c.Telemetry.UseTLS,
		Headers:        c.Telemetry.Headers,
		Analytics: telemetry.AnalyticsBridgeConfig{
			Enabled:        c.Telemetry.Analytics.Enabled,
			RedisAddr:      c.Telemetry.Analytics.RedisAddr,
			RedisPassword:  c.Telemetry.Analytics.RedisPassword,
			RedisDB:        c.Telemetry.Analytics.RedisDB,
			TopicPrefix:    c.Telemetry.Analytics.TopicPrefix,
			RetentionHours: c.Telemetry.Analytics.RetentionHours,
			BatchSize:      c.Telemetry.Analytics.BatchSize,
			FlushInterval:  parseTelemetryDuration(c.Telemetry.Analytics.FlushInterval),
		},
	})
	if !cfg.Enabled {
		return nil, nil
	}
	if logger == nil {
		logger = slog.Default()
	}
	return telemetry.NewGameTelemetryService(cfg, logger)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func parseTelemetryDuration(value string) time.Duration {
	if strings.TrimSpace(value) == "" {
		return 0
	}
	duration, err := time.ParseDuration(strings.TrimSpace(value))
	if err != nil {
		return 0
	}
	return duration
}

// autoMigrate runs all necessary database migrations
func autoMigrate(db *gorm.DB) error {
	// Import all model packages and run their AutoMigrate functions
	if err := model.AutoMigrate(db); err != nil {
		return fmt.Errorf("failed to migrate server models: %w", err)
	}

	if err := migrateAuxModels(db); err != nil {
		return err
	}

	// Migrate Agent Session model
	if err := reg.MigrateAgentSessions(db); err != nil {
		return fmt.Errorf("failed to migrate agent sessions: %w", err)
	}

	return nil
}

// migrateAuxModels bootstraps tables owned by packages that must not depend
// on internal/model (audit, registry metrics). It runs as part of the
// migration baseline only — constructors of these stores never run DDL.
func migrateAuxModels(db *gorm.DB) error {
	if err := db.AutoMigrate(&audit.AuditModel{}, &reg.AgentMetricsHistory{}); err != nil {
		return fmt.Errorf("failed to migrate aux models: %w", err)
	}
	return nil
}

// migrateMeta brings the meta database to the latest schema version via the
// versioned migration executor. The legacy AutoMigrate path only runs as the
// one-time baseline bridge on fresh/pre-versioning databases
// (docs/architecture/database-migration-strategy.md).
func migrateMeta(ctx context.Context, db *gorm.DB) (int64, error) {
	return migrate.EnsureUpToDate(ctx, db, migrate.ScopeMeta, func(db *gorm.DB) error {
		return autoMigrateMeta(db)
	})
}

// autoMigrateMeta migrates only meta-level models into the meta database
// (database-per-game architecture).
func autoMigrateMeta(db *gorm.DB) error {
	if err := model.AutoMigrateMeta(db); err != nil {
		return fmt.Errorf("failed to migrate meta models: %w", err)
	}
	if err := migrateAuxModels(db); err != nil {
		return err
	}
	if err := reg.MigrateAgentSessions(db); err != nil {
		return fmt.Errorf("failed to migrate agent sessions: %w", err)
	}
	return nil
}

// newGameRouter builds a database-per-game router wired to the resolved
// driver/DSN. The router auto-creates and migrates each game database on
// first use.
func newGameRouter(c config.Config, metaDB *gorm.DB) *router.Router {
	driver, dsn := resolveDriverAndDSN(c)
	dsnForDB := func(metaDSN, dbName string) string {
		return DSNForDatabase(driver, metaDSN, dbName)
	}
	cfg := router.Config{
		Driver:         driver,
		MetaDSN:        dsn,
		DSNForDatabase: dsnForDB,
		EnsureDatabase: EnsureGameDatabase,
		Open:           OpenGormForRouter,
		MigrateGame: func(db *gorm.DB) error {
			if _, err := migrate.EnsureUpToDate(context.Background(), db, migrate.ScopeGame, model.AutoMigrateGame); err != nil {
				return err
			}
			return nil
		},
	}
	if prefix := strings.TrimSpace(c.Database.GameDBPrefix); prefix != "" {
		cfg.NameForGame = func(gameID, env string) string {
			return prefix + sanitizeGameDBComponent(gameID) + "_" + sanitizeGameDBComponent(env)
		}
	}
	return router.New(cfg, metaDB)
}

// sanitizeGameDBComponent lowercases and whitelists a game/env identifier for
// use inside a physical database name.
func sanitizeGameDBComponent(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" {
		s = "default"
	}
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '_', r == '-':
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
	}
	return b.String()
}

// gameDBNameFor resolves the physical database name for a game scope using
// the configured prefix (mirrors the router wiring in newGameRouter).
func gameDBNameFor(c config.Config, gameID, env string) string {
	if prefix := strings.TrimSpace(c.Database.GameDBPrefix); prefix != "" {
		return prefix + sanitizeGameDBComponent(gameID) + "_" + sanitizeGameDBComponent(env)
	}
	return router.DefaultGameDBName(gameID, env)
}

// isDevelopmentConfig checks if the current configuration is in development mode.
func isDevelopmentConfig(cfg config.Config) bool {
	// Check server mode configuration
	if strings.EqualFold(cfg.Server.Mode, "dev") || strings.EqualFold(cfg.Server.Mode, "development") || strings.EqualFold(cfg.Server.Mode, "debug") {
		return true
	}
	// Check environment variable
	if env := strings.TrimSpace(os.Getenv("CROUPIER_ENV")); env != "" {
		if strings.EqualFold(env, "dev") || strings.EqualFold(env, "development") {
			return true
		}
	}
	// Default to development if not explicitly set to production
	if strings.EqualFold(os.Getenv("CROUPIER_MODE"), "prod") || strings.EqualFold(os.Getenv("CROUPIER_MODE"), "production") {
		return false
	}
	return true
}

func resolveBootstrapAuthDir(c config.Config) string {
	baseDir := resolveBootstrapBaseDir(c)
	if baseDir == "" {
		return runtime.DefaultBootstrapDataDir()
	}
	return baseDir
}

func resolveBootstrapBaseDir(c config.Config) string {
	if dir := strings.TrimSpace(c.BootstrapData.BaseDir); dir != "" {
		return toAbs(dir)
	}

	if c.Auth.UsersConfig != "" {
		return filepath.Dir(toAbs(c.Auth.UsersConfig))
	}

	return runtime.DefaultBootstrapDataDir()
}

func resolveTaskRoutingDir(c config.Config) string {
	if dir := strings.TrimSpace(c.AgentDispatch.TaskRoutingDir); dir != "" {
		return toAbs(dir)
	}

	return "data"
}

func toAbs(p string) string {
	if p == "" {
		return ""
	}
	if filepath.IsAbs(p) {
		return filepath.Clean(p)
	}
	if abs, err := filepath.Abs(p); err == nil {
		return abs
	}
	return p
}

func seedBootstrapAdmins(ctx *ServiceContext) error {
	if ctx == nil || ctx.AdminManager == nil || ctx.AdminModel == nil {
		slog.Default().Warn("seedBootstrapAdmins: missing dependencies", "ctx", ctx == nil, "adminManager", ctx == nil || ctx.AdminManager == nil, "adminModel", ctx == nil || ctx.AdminModel == nil)
		return nil
	}

	admins := ctx.AdminManager.ListAdmins()
	slog.Default().Info("seedBootstrapAdmins: starting", "adminCount", len(admins))
	if len(admins) == 0 {
		slog.Default().Warn("seedBootstrapAdmins: no admins to seed from AdminManager")
		return nil
	}

	bg := context.Background()
	for _, admin := range admins {
		username := strings.TrimSpace(admin.Username)
		if username == "" || strings.TrimSpace(admin.Password) == "" {
			continue
		}
		// Warn about plaintext passwords in bootstrap config
		if !admin.IsHashedPassword() {
			slog.Default().Warn("Bootstrap admin uses plaintext password - consider using bcrypt hash", "username", username)
		}

		bootstrapStatus := 1
		if admin.Status == 1 {
			bootstrapStatus = 1
		}

		var dbAdmin model.Admin
		err := ctx.DB.WithContext(bg).
			Where("username = ?", username).
			First(&dbAdmin).Error

		if errors.Is(err, gorm.ErrRecordNotFound) {
			newAdmin := &model.Admin{
				Username: username,
				Nickname: strings.TrimSpace(admin.Nickname),
				Email:    strings.TrimSpace(admin.Email),
				Phone:    strings.TrimSpace(admin.Phone),
				Status:   bootstrapStatus,
			}

			if err := ctx.AdminModel.Create(bg, newAdmin, admin.Password); err != nil {
				slog.Default().Error("创建引导管理员失败", "username", username, "error", err)
				continue
			}

			dbAdmin = *newAdmin
			slog.Default().Info("已创建引导管理员账号", "username", username)
		} else if err != nil {
			slog.Default().Error("查询管理员失败", "username", username, "error", err)
			continue
		} else if dbAdmin.Status != bootstrapStatus {
			if err := ctx.AdminModel.Update(bg, dbAdmin.ID, map[string]interface{}{"status": bootstrapStatus}); err != nil {
				slog.Default().Error("同步引导管理员状态失败", "username", username, "error", err)
			} else {
				dbAdmin.Status = bootstrapStatus
			}
		}

		for _, roleName := range admin.Roles {
			trimmed := strings.TrimSpace(roleName)
			if trimmed == "" {
				continue
			}
			var role model.Role
			if err := ctx.DB.WithContext(bg).Where("LOWER(name) = ?", strings.ToLower(trimmed)).First(&role).Error; err != nil {
				slog.Default().Error("为管理员查询角色失败", "username", username, "role", trimmed, "error", err)
				continue
			}
			var relCount int64
			if err := ctx.DB.WithContext(bg).
				Model(&model.AdminRole{}).
				Where("admin_id = ? AND role_id = ?", dbAdmin.ID, role.ID).
				Count(&relCount).Error; err != nil {
				slog.Default().Error("检查管理员是否已分配角色失败", "username", username, "role", trimmed, "error", err)
				continue
			}
			if relCount > 0 {
				continue
			}
			if err := ctx.AdminModel.AssignRole(bg, dbAdmin.ID, role.ID); err != nil {
				slog.Default().Error("为管理员分配角色失败", "username", username, "role", trimmed, "error", err)
			}
		}
		ctx.InvalidateAdminCache(bg, dbAdmin.ID, username)
	}

	return nil
}

func seedBootstrapRoles(ctx *ServiceContext) error {
	if ctx == nil || ctx.AdminManager == nil || ctx.RoleModel == nil || ctx.DB == nil {
		return nil
	}

	roles := ctx.AdminManager.ListRoles()
	if len(roles) == 0 {
		return nil
	}

	bg := context.Background()
	for _, role := range roles {
		if role == nil {
			continue
		}
		code := strings.TrimSpace(role.Code)
		if code == "" {
			continue
		}

		var dbRole model.Role
		err := ctx.DB.WithContext(bg).
			Where("name = ?", code).
			First(&dbRole).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			dbRole = model.Role{
				Name:        code,
				Description: strings.TrimSpace(role.Description),
				Category:    strings.TrimSpace(role.Name),
			}
			if err := ctx.RoleModel.Create(bg, &dbRole); err != nil {
				slog.Default().Error("创建引导角色失败", "code", code, "error", err)
				continue
			}
			slog.Default().Info("已创建引导角色", "code", code)
		} else if err != nil {
			slog.Default().Error("检查引导角色是否存在失败", "code", code, "error", err)
			continue
		}

		if len(role.Permissions) == 0 {
			continue
		}
		normalized, err := ctx.RoleModel.ValidatePermissionIDs(bg, role.Permissions)
		if err != nil {
			slog.Default().Error("校验角色权限失败", "code", code, "error", err)
			continue
		}

		// 获取现有权限
		existingPerms, err := ctx.RoleModel.GetRolePermissionIDs(bg, dbRole.ID)
		if err != nil {
			slog.Default().Error("查询角色现有权限失败", "code", code, "error", err)
			continue
		}

		// 如果已有权限数量匹配且内容相同，跳过
		if len(existingPerms) == len(normalized) {
			existingPermMap := make(map[string]bool)
			for _, ep := range existingPerms {
				existingPermMap[ep] = true
			}
			allMatch := true
			for _, perm := range normalized {
				if !existingPermMap[perm] {
					allMatch = false
					break
				}
			}
			if allMatch {
				continue // 权限完全一致，跳过更新
			}
		}

		// 使用 ReplacePermissions 更新，它会先删除旧的再插入新的
		if err := ctx.RoleModel.ReplacePermissions(bg, dbRole.ID, normalized); err != nil {
			slog.Default().Error("更新角色权限失败", "code", code, "error", err)
		}
	}
	return nil
}

func seedBootstrapPermissions(ctx *ServiceContext) error {
	if ctx == nil || ctx.AdminManager == nil || ctx.PermissionModel == nil || ctx.DB == nil {
		return nil
	}

	perms := ctx.AdminManager.ListPermissions()
	if len(perms) == 0 {
		return nil
	}

	bg := context.Background()
	for _, perm := range perms {
		if perm == nil {
			continue
		}
		code := strings.TrimSpace(perm.Code)
		if code == "" {
			continue
		}

		var existing model.Permission
		err := ctx.DB.WithContext(bg).Where("id = ?", code).First(&existing).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			resource, action := derivePermissionResourceAction(code, perm.Module)
			category := strings.TrimSpace(perm.Category)
			if category == "" {
				category = resource
			}
			name := strings.TrimSpace(perm.Name)
			if name == "" {
				name = code
			}
			record := &model.Permission{
				ID:          code,
				Name:        name,
				Description: strings.TrimSpace(perm.Description),
				Resource:    resource,
				Action:      action,
				Category:    category,
			}
			if err := ctx.DB.WithContext(bg).Create(record).Error; err != nil {
				slog.Default().Error("创建引导权限失败", "code", code, "error", err)
			} else {
				slog.Default().Info("已创建引导权限", "code", code)
			}
		} else if err != nil {
			slog.Default().Error("检查权限是否存在失败", "code", code, "error", err)
		}
	}
	return nil
}

func derivePermissionResourceAction(code, module string) (string, string) {
	resource := strings.TrimSpace(module)
	action := "*"

	baseResource, baseAction := splitPermissionCode(code)
	if resource == "" {
		resource = baseResource
	}
	if baseAction != "" {
		action = baseAction
	}
	if resource == "" {
		resource = "global"
	}
	if action == "" {
		action = "*"
	}
	return resource, action
}

func splitPermissionCode(code string) (string, string) {
	code = strings.TrimSpace(code)
	if code == "" {
		return "", ""
	}
	parts := strings.SplitN(code, ":", 2)
	resource := strings.TrimSpace(parts[0])
	action := "*"
	if len(parts) == 2 && strings.TrimSpace(parts[1]) != "" {
		action = strings.TrimSpace(parts[1])
	}
	return resource, action
}

type bootstrapExtensionCatalogFile struct {
	Items []bootstrapExtensionCatalogItem `json:"items"`
}

type bootstrapExtensionCatalogItem struct {
	ExtensionID   string                            `json:"extensionId"`
	Name          string                            `json:"name"`
	DisplayName   string                            `json:"displayName"`
	Vendor        string                            `json:"vendor"`
	Kind          string                            `json:"kind"`
	Summary       string                            `json:"summary"`
	IconURL       string                            `json:"iconUrl"`
	HomepageURL   string                            `json:"homepageUrl"`
	Status        string                            `json:"status"`
	LatestVersion string                            `json:"latestVersion"`
	Releases      []bootstrapExtensionReleaseRecord `json:"releases"`
}

type bootstrapExtensionReleaseRecord struct {
	Version         string         `json:"version"`
	ReleaseChannel  string         `json:"releaseChannel"`
	MinCoreVersion  string         `json:"minCoreVersion"`
	PackageRef      string         `json:"packageRef"`
	Checksum        string         `json:"checksum"`
	Changelog       string         `json:"changelog"`
	PublishedAt     string         `json:"publishedAt"`
	PublishedAtUnix int64          `json:"publishedAtUnix"`
	Manifest        map[string]any `json:"manifest"`
}

func seedBootstrapExtensionCatalog(ctx *ServiceContext) error {
	if ctx == nil || ctx.DB == nil {
		return nil
	}

	seedPath := filepath.Join(resolveBootstrapAuthDir(ctx.Config), "extensions", "catalog.json")
	data, err := os.ReadFile(seedPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("read extension catalog seed failed: %w", err)
	}

	var payload bootstrapExtensionCatalogFile
	if err := json.Unmarshal(data, &payload); err != nil {
		return fmt.Errorf("parse extension catalog seed failed: %w", err)
	}
	if len(payload.Items) == 0 {
		return nil
	}

	bg := context.Background()
	for _, item := range payload.Items {
		extID := strings.TrimSpace(item.ExtensionID)
		if extID == "" {
			continue
		}
		record := model.ExtensionCatalog{
			ExtensionID:   extID,
			Name:          strings.TrimSpace(item.Name),
			DisplayName:   strings.TrimSpace(item.DisplayName),
			Vendor:        strings.TrimSpace(item.Vendor),
			Kind:          strings.TrimSpace(item.Kind),
			Summary:       strings.TrimSpace(item.Summary),
			IconURL:       strings.TrimSpace(item.IconURL),
			HomepageURL:   strings.TrimSpace(item.HomepageURL),
			Status:        strings.TrimSpace(item.Status),
			LatestVersion: strings.TrimSpace(item.LatestVersion),
		}
		if record.Name == "" {
			record.Name = extID
		}
		if record.DisplayName == "" {
			record.DisplayName = record.Name
		}
		if record.Vendor == "" {
			record.Vendor = "official"
		}
		if record.Kind == "" {
			record.Kind = "official"
		}
		if record.Status == "" {
			record.Status = "active"
		}

		var existing model.ExtensionCatalog
		err := ctx.DB.WithContext(bg).Where("extension_id = ?", extID).First(&existing).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			if err := ctx.DB.WithContext(bg).Create(&record).Error; err != nil {
				return fmt.Errorf("create extension catalog seed failed (%s): %w", extID, err)
			}
		} else if err != nil {
			return fmt.Errorf("query extension catalog seed failed (%s): %w", extID, err)
		} else {
			updates := map[string]any{
				"name":           record.Name,
				"display_name":   record.DisplayName,
				"vendor":         record.Vendor,
				"kind":           record.Kind,
				"summary":        record.Summary,
				"icon_url":       record.IconURL,
				"homepage_url":   record.HomepageURL,
				"status":         record.Status,
				"latest_version": record.LatestVersion,
			}
			if err := ctx.DB.WithContext(bg).Model(&existing).Updates(updates).Error; err != nil {
				return fmt.Errorf("update extension catalog seed failed (%s): %w", extID, err)
			}
		}

		for _, release := range item.Releases {
			version := strings.TrimSpace(release.Version)
			if version == "" {
				continue
			}
			manifest := release.Manifest
			if len(manifest) == 0 {
				manifest = map[string]any{
					"id":      extID,
					"name":    record.Name,
					"version": version,
				}
			}
			manifestJSON, _ := json.Marshal(manifest)
			publishedAtUnix := release.PublishedAtUnix
			if publishedAtUnix == 0 {
				if ts := strings.TrimSpace(release.PublishedAt); ts != "" {
					if parsed, parseErr := time.Parse(time.RFC3339, ts); parseErr == nil {
						publishedAtUnix = parsed.Unix()
					}
				}
			}
			if publishedAtUnix == 0 {
				publishedAtUnix = time.Now().Unix()
			}

			var existingRelease model.ExtensionRelease
			err := ctx.DB.WithContext(bg).
				Where("extension_id = ? AND version = ?", extID, version).
				First(&existingRelease).Error
			if errors.Is(err, gorm.ErrRecordNotFound) {
				newRelease := model.ExtensionRelease{
					ExtensionID:     extID,
					Version:         version,
					ReleaseChannel:  strings.TrimSpace(release.ReleaseChannel),
					ManifestJSON:    datatypes.JSON(manifestJSON),
					PackageRef:      strings.TrimSpace(release.PackageRef),
					Checksum:        strings.TrimSpace(release.Checksum),
					MinCoreVersion:  strings.TrimSpace(release.MinCoreVersion),
					Changelog:       strings.TrimSpace(release.Changelog),
					PublishedAtUnix: publishedAtUnix,
				}
				if newRelease.ReleaseChannel == "" {
					newRelease.ReleaseChannel = "stable"
				}
				if err := ctx.DB.WithContext(bg).Create(&newRelease).Error; err != nil {
					return fmt.Errorf("create extension release seed failed (%s@%s): %w", extID, version, err)
				}
			} else if err != nil {
				return fmt.Errorf("query extension release seed failed (%s@%s): %w", extID, version, err)
			}
		}
	}

	slog.Default().Info("extension catalog bootstrap seeded", "path", seedPath, "items", len(payload.Items))
	return nil
}

func initObjectStore(ctx context.Context, cfg config.StorageConfig) (objstore.Store, error) {
	driver := strings.TrimSpace(cfg.Driver)
	if driver == "" {
		return nil, nil
	}

	storeCfg := objstore.Config{
		Driver:         driver,
		Bucket:         strings.TrimSpace(cfg.Bucket),
		Region:         strings.TrimSpace(cfg.Region),
		Endpoint:       strings.TrimSpace(cfg.Endpoint),
		AccessKey:      strings.TrimSpace(cfg.AccessKey),
		SecretKey:      strings.TrimSpace(cfg.SecretKey),
		ForcePathStyle: cfg.ForcePathStyle,
		BaseDir:        toAbs(strings.TrimSpace(cfg.BaseDir)),
	}
	if ttl := strings.TrimSpace(cfg.SignedURLTTL); ttl != "" {
		if d, err := time.ParseDuration(ttl); err == nil {
			storeCfg.SignedURLTTL = d
		}
	}

	if err := objstore.Validate(storeCfg); err != nil {
		return nil, err
	}

	switch strings.ToLower(driver) {
	case "s3":
		return objstore.OpenS3(ctx, storeCfg)
	case "oss":
		return objstore.OpenOSS(ctx, storeCfg)
	case "cos":
		return objstore.OpenCOS(ctx, storeCfg)
	case "file":
		return objstore.OpenFile(ctx, storeCfg)
	default:
		return nil, fmt.Errorf("unsupported storage driver: %s", driver)
	}
}

// ============================================================================
// 认证中间件
// ============================================================================

// NewAuthMiddleware 创建认证中间件
func NewAuthMiddleware(svcCtx *ServiceContext) gin.HandlerFunc {
	return NewAuthMiddlewareImpl(svcCtx).Handle
}

// DBHealth 数据库健康检查
type DBHealth struct {
	svcCtx *ServiceContext
}

// NewDBHealth 创建数据库健康检查实例
func NewDBHealth(svcCtx *ServiceContext) *DBHealth {
	return &DBHealth{
		svcCtx: svcCtx,
	}
}

// Check 检查数据库连接
func (h *DBHealth) Check(ctx context.Context) error {
	if h.svcCtx == nil || h.svcCtx.AdminModel == nil {
		return fmt.Errorf("数据库模型未初始化")
	}

	// 执行简单的查询来检查数据库连接
	queryCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	// 尝试查询一个管理员（不关心结果）
	_, err := h.svcCtx.AdminModel.FindOne(queryCtx, 1)
	if err != nil && err != sql.ErrNoRows {
		slog.ErrorContext(queryCtx, "Database health check failed", "error", err)
		return fmt.Errorf("数据库连接检查失败")
	}

	return nil
}

// Ping 简单的 ping 检查
func (h *DBHealth) Ping() error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	return h.Check(ctx)
}

// AuthMiddleware 认证中间件实现
type AuthMiddleware struct {
	svcCtx             *ServiceContext
	allowPaths         map[string]struct{}
	allowPref          []string
	publicReadPrefixes []string
}

// NewAuthMiddlewareImpl 创建认证中间件实例
func NewAuthMiddlewareImpl(svcCtx *ServiceContext) *AuthMiddleware {
	return &AuthMiddleware{
		svcCtx: svcCtx,
		allowPaths: map[string]struct{}{
			"/healthz":                   {},
			"/api/v1/auth/login":         {},
			"/api/v1/monitoring/health":  {},
			"/api/v1/monitoring/healthz": {},
		},
		allowPref: []string{
			"/api/v1/auth/login",
		},
		publicReadPrefixes: []string{
			"/api/v1/registry", // 公开访问：注册中心（agents、functions）
			// 客户端公开端点（游戏内 SDK / 玩家侧，无管理台 token）
			"/api/v1/public/", // 配置拉取 + 玩家客服
			"/api/v1/releases/check",
		},
	}
}

// Handle 处理认证中间件（Gin 风格）
func (m *AuthMiddleware) Handle(c *gin.Context) {
	if m.shouldBypassGin(c) {
		c.Next()
		return
	}

	// 获取 Authorization header，fallback 到 ?token= query param
	// （EventSource 不支持自定义 headers，SSE 前端通过 ?token= 传递 JWT）
	var token string
	authHeader := c.GetHeader("Authorization")
	if authHeader != "" {
		tokenParts := strings.SplitN(authHeader, " ", 2)
		if len(tokenParts) != 2 || tokenParts[0] != "Bearer" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid authorization header format", "message": "授权头格式错误"})
			return
		}
		token = tokenParts[1]
	} else {
		token = c.Query("token")
	}

	if token == "" {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing authorization", "message": "未授权"})
		return
	}

	// 验证 JWT token
	username, roles, adminID, err := m.authenticate(c.Request.Context(), token)
	if err != nil {
		slog.ErrorContext(c.Request.Context(), "authentication failed", "error", err)
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "authentication_failed", "message": "认证失败"})
		return
	}

	ctx := context.WithValue(c.Request.Context(), "username", username)
	ctx = context.WithValue(ctx, "roles", roles)
	ctx = context.WithValue(ctx, "adminID", adminID)
	c.Request = c.Request.WithContext(ctx)
	c.Set("username", username)
	c.Set("roles", roles)
	c.Set("adminID", adminID)
	slog.InfoContext(c.Request.Context(), "Authenticated user", "username", username, "roles", roles)

	c.Next()
}

func (m *AuthMiddleware) shouldBypassGin(c *gin.Context) bool {
	return m.shouldBypass(c.Request)
}

func (m *AuthMiddleware) authenticate(ctx context.Context, token string) (string, []string, uint, error) {
	secret := jwtutil.GetGlobalSecret()
	if strings.TrimSpace(secret) == "" {
		return "", nil, 0, errors.New("jwt secret not initialized")
	}

	claims, err := jwtutil.Parse(token, secret)
	if err != nil {
		return "", nil, 0, fmt.Errorf("invalid token: %w", err)
	}

	return claims.Username, claims.Roles, claims.AdminID, nil
}

func (m *AuthMiddleware) shouldBypass(r *http.Request) bool {
	path := r.URL.Path
	if _, ok := m.allowPaths[path]; ok {
		return true
	}
	for _, prefix := range m.allowPref {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	if r.Method == http.MethodGet {
		// 前缀匹配
		for _, prefix := range m.publicReadPrefixes {
			if strings.HasPrefix(path, prefix) {
				return true
			}
		}
	}
	return false
}
