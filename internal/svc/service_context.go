package svc

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"log/slog"

	"github.com/cuihairu/croupier/core/transport"
	"github.com/cuihairu/croupier/internal/audit"
	"github.com/cuihairu/croupier/internal/cache"
	"github.com/cuihairu/croupier/internal/cluster"
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
	"github.com/cuihairu/croupier/internal/ops/deploy"
	"github.com/cuihairu/croupier/internal/ops/provision"
	alertrule "github.com/cuihairu/croupier/internal/platform/alertrule"
	"github.com/cuihairu/croupier/internal/platform/approvals"
	"github.com/cuihairu/croupier/internal/platform/configsource"
	dispatch "github.com/cuihairu/croupier/internal/platform/dispatch"
	"github.com/cuihairu/croupier/internal/platform/executionlog"
	objstore "github.com/cuihairu/croupier/internal/platform/objstore"
	"github.com/cuihairu/croupier/internal/platform/outlet"
	reg "github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/cuihairu/croupier/internal/platform/settings"
	"github.com/cuihairu/croupier/internal/platform/tlsutil"
	"github.com/cuihairu/croupier/internal/platform/vault"
	policymgr "github.com/cuihairu/croupier/internal/policy"
	"github.com/cuihairu/croupier/internal/powerops"
	extensiongorm "github.com/cuihairu/croupier/internal/repo/gorm/extension"
	jwtutil "github.com/cuihairu/croupier/internal/security/jwtutil"
	notify "github.com/cuihairu/croupier/internal/service/notify"
	"github.com/cuihairu/croupier/internal/service/permission"
	scheduler "github.com/cuihairu/croupier/internal/tasks/scheduler"
	"github.com/cuihairu/croupier/internal/telemetry"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// AgentSessionResolver 解析在线 Agent 的会话调用器（ops 代理调用入口）。
type AgentSessionResolver interface {
	ResolveSessionCaller(agentID string) (transport.SessionCaller, bool)
}

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
	// AgentSessions 解析在线 Agent 会话（ops 代理调用：cron/服务列表）。
	AgentSessions AgentSessionResolver
	Cache         cache.CacheStore
	CacheHelper   *cache.CacheHelper

	AnalyticsFiltersLock *sync.RWMutex

	ApprovalsStore approvals.Store
	PolicyManager  *policymgr.Manager
	AuditService   *audit.AuditService

	ObjectStore objstore.Store
	// SMSRegistry 持有短信服务商实例；默认未接入（NewSMSRegistry），
	// 供个人中心如实上报「短信通道未接入」而非假装已开启。
	SMSRegistry *approvals.SMSRegistry
	Telemetry   *telemetry.GameTelemetryService

	// Agent Ops support
	MetricsStore         *reg.MetricsStore
	LogStore             *reg.LogStore
	SystemInfoCache      *reg.SystemInfoCache
	AgentSessionResolver dispatch.AgentSessionResolver

	// ServerProbes server 本机探针面（R3 server 侧内核装配）：空段不装配
	//（nil），手动诊断端点据此如实报「探测面未配置」。
	ServerProbes *ServerProbeManager

	// Provision 是云资源供给引擎（OPS-001）：provision.enabled=false 时不
	// 装配（nil），/ops/provision 端点据此如实报 503 provision_not_configured。
	Provision *provision.Orchestrator

	// Powerops 是远程电源管理编排（OPS 电源管理，docs/design/ops-power-management.md）：
	// powerops.enabled=false 时不装配（nil），/ops/power 端点据此如实报
	// 503 powerops_not_configured。
	Powerops *powerops.Orchestrator

	// Deployments 是运维部署发版编排（OPS-004）：包仓库登记 + 发版单分批 + 健康门禁 + 回滚。
	// 装配于 DB 与 ObjectStore 就绪后；nil 时 /ops/deploy 端点如实报 503。
	Deployments        *deploy.Orchestrator
	DeployPackageModel *model.DeployPackageModel
	DeploymentModel    *model.DeploymentModel

	AdminModel             *model.AdminModel
	AdminOTPRecoveryModel  *model.AdminOTPRecoveryCodeModel
	EmailVerificationModel *model.EmailVerificationModel
	AlertModel             *model.AlertModel
	BehaviorModel          *model.BehaviorModel
	RetentionModel         *model.RetentionModel
	PaymentsModel          *model.PaymentsModel
	BackupModel            *model.BackupModel
	AlertRuleModel         *model.AlertRuleModel
	FAQModel               *model.FAQModel
	FeedbackModel          *model.FeedbackModel
	GameModel              *model.GameModel
	PlayerModel            *model.PlayerModel
	ProfileModel           *model.ProfileModel
	FunctionModel          *model.FunctionModel
	TermDictModel          *model.TermDictionaryModel
	RoleModel              *model.RoleModel
	NodeModel              *model.NodeModel
	PermissionModel        *model.PermissionModel
	RateLimitModel         *model.RateLimitModel
	SupportModel           *model.SupportModel
	TicketModel            *model.TicketModel
	BugModel               *model.BugModel
	ToolModel              *model.ToolLinkModel
	CicdIntegrationModel   *model.CicdIntegrationModel
	CicdBuildModel         *model.CicdBuildModel
	ReleaseModel           *model.GameReleaseModel
	HotpatchModel          *model.HotpatchModel
	DBSourceModel          *model.DBSourceModel
	PlatformSettingModel   *model.PlatformSettingModel
	MessageModel           *model.MessageModel
	// NotifyService 分发审批/告警事件到已配置渠道（站内信/钉钉/webhook/邮件）。
	// 在 handler 装配时注入（依赖 settings.Layered 单例）。
	NotifyService *notify.Service
	// ExecutionLogWriter 执行留痕异步写入器（R1）；Run() 由 server 启动，
	// enabled=false 时为 nil。
	ExecutionLogWriter *executionlog.Writer
	ExecutionLogModel  *model.ExecutionLogModel
	// LogRetention 留痕保留期清理器（R3 + #54 手动清理入口）；
	// ExecutionLog 关闭时为 nil。
	LogRetention *executionlog.Retention
	// retentionCancel 取消 LogRetention 的后台 sweep 循环（Close 收尾用）；
	// ExecutionLog 关闭时为 nil。
	retentionCancel context.CancelFunc
	// Scheduler 是 cron 定时任务调度循环（StartScheduler 启动）。
	Scheduler *scheduler.Manager
	// ReportRunner 是 server-local 报表执行面（Kind=incident_report 的调度
	// 注入 scheduler.LocalRunner）；cmd/server 启动时经 SetReportRunner 注入。
	ReportRunner scheduler.LocalRunner
	// OutletManager 是告警出口链管理器（registerOutlets 构造；MetricsStore
	// 缺失时为 nil）。报表分发经此派发到站内/外部出口。
	OutletManager *outlet.Manager
	// StationSink 是站内通知落库面（registerOutlets 构造的 messageSink）。
	StationSink outlet.MessageSink
	// Vault 是密码箱（vault-provider 设计 §4；wireVault 构造）。凭据统一
	// 存取面——enabled=false 时 nil，接入点迁移（todo 4251）逐点接入。
	Vault vault.Provider
	// Secrets 是凭据值内引用（vault://）展开函数（NewServiceContext 装配，
	// 恒非 nil）：密码箱启用时引用经 provider 展开，未启用时引用 fail-closed
	// 报错、内联值直通（存量部署零行为变化）。
	Secrets vault.Resolver
	// Cluster 是多实例 HA 运行时（未启用时 nil）。
	Cluster *ClusterRuntime
	// ClusterReadiness 是集群就绪面（ARCH-004）：cluster.enabled 时
	// 创建（即使启动期降级），/readyz 判定与 /healthz 集群组件观测用；
	// 未启用集群时 nil。
	ClusterReadiness         *cluster.Readiness
	CertificateModel         *model.CertificateModel
	ConfigVersionModel       *model.ConfigVersionModel
	ConfigSourceBindingModel *model.ConfigSourceBindingModel
	TaskScheduleModel        *model.TaskScheduleModel

	// Page Spec models
	PageSpecModel             *model.PageSpecModel
	PublishedPageSpecModel    *model.PublishedPageSpecModel
	PageVersionModel          *model.PageVersionModel
	OpenAPISourceModel        *model.OpenAPISourceModel
	OpenAPISourceBindingModel *model.OpenAPISourceBindingModel
	MenuModel                 *model.MenuItemModel
	// MenuSeeder 默认菜单惰性种子（T-M10）：菜单读路径首访空 scope 时导入
	// bootstrap 目录 default-menus.json 骨架。文件缺失/空 → Disabled no-op。
	MenuSeeder *MenuSeeder

	// Agent Session 持久化
	AgentSessionModel *reg.AgentSessionModel

	// 版本信息（由 build 脚本通过 ldflags 注入）
	ServerVersion   string
	ServerGitCommit string
	ServerBuildTime string

	// StartTime 记录服务器启动时间
	StartTime time.Time

	Extensions *ExtensionServices

	// closeOnce guards Close()（ARCH-017：冲刷 + 池释放幂等收口）。
	closeOnce sync.Once
}

type ExtensionServices struct {
	Catalog      *extensioncatalog.Service
	Manifest     *extensionmanifest.Service
	Installation *extensioninstallation.Service
	Runtime      *extensionruntime.Service
	Sync         *extensionsync.Service
}

func NewServiceContext(c config.Config, opts ...Option) *ServiceContext {
	// 密码箱先行（todo 4251）：DB DSN 与后续装配面凭据可能是 vault:// 引用，
	// provider/resolver 必须先于 openDatabase 就位；resolver 同时注入
	// settings 层（L3 凭据键读边界展开）。
	vaultProvider := newVaultProvider(c)
	secrets := vault.NewResolver(vaultProvider)
	settings.SetSecretResolver(secrets)

	// 初始化数据库连接。dataSource 日志不回显内联明文（密码箱红线）：
	// vault:// 引用原样打印（路径非密），内联形态只打 driver。
	if vault.IsVaultRef(c.Database.DataSource) {
		slog.Default().Info("Connecting to database", "driver", c.Database.Driver, "dataSource", c.Database.DataSource)
	} else {
		slog.Default().Info("Connecting to database", "driver", c.Database.Driver)
	}
	db, err := openDatabase(c, secrets)
	if err != nil {
		// panic-ok: NewServiceContext 启动装配期，DB 连不上属不可恢复
		// 启动失败（CR-FIX-22 豁免清单）。
		panic(fmt.Sprintf("Failed to connect to database: %v", err))
	}
	slog.Default().Info("Database connected successfully")

	// 自动迁移数据库模型
	slog.Default().Info("Starting database migration")
	multiGame := c.Database.MultiGame
	migrateCtx := context.Background()
	if multiGame {
		if _, err := migrateMeta(migrateCtx, db); err != nil {
			// panic-ok: 启动期迁移失败，库停旧版语义，不可恢复（CR-FIX-22）。
			panic(fmt.Sprintf("Failed to migrate meta database: %v", err))
		}
	} else {
		if _, err := migrate.EnsureUpToDate(migrateCtx, db, migrate.ScopeSingle, autoMigrate); err != nil {
			// panic-ok: 启动期迁移失败，库停旧版语义，不可恢复（CR-FIX-22）。
			panic(fmt.Sprintf("Failed to migrate database: %v", err))
		}
	}
	slog.Default().Info("Database migration completed successfully", "multiGame", multiGame)

	// 创建 per-game 数据库路由（当启用 multiGame 时）
	var dbRouter *router.Router
	if multiGame {
		dbRouter = newGameRouter(c, db, secrets)
		slog.Default().Info("Database-per-game router initialized")
	}

	// 创建服务
	permissionService := permission.NewPermissionService(db)

	// 模型实例（保持在同一处构建，便于逻辑层复用）
	adminModel := model.NewAdminModel(db)
	adminOTPRecoveryModel := model.NewAdminOTPRecoveryCodeModel(db)
	emailVerificationModel := model.NewEmailVerificationModel(db)
	alertModel := model.NewAlertModel(db)
	behaviorModel := model.NewBehaviorModel(db)
	retentionModel := model.NewRetentionModel(db)
	paymentsModel := model.NewPaymentsModel(db)
	backupModel := model.NewBackupModel(db)
	alertRuleModel := model.NewAlertRuleModel(db)
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
	cicdIntegrationModel := model.NewCicdIntegrationModel(db)
	cicdBuildModel := model.NewCicdBuildModel(db)
	releaseModel := model.NewGameReleaseModel(db)
	hotpatchModel := model.NewHotpatchModel(db)
	dbSourceModel := model.NewDBSourceModel(db)
	platformSettingModel := model.NewPlatformSettingModel(db)
	messageModel := model.NewMessageModel(db)
	certificateModel := model.NewCertificateModel(db)
	configVersionModel := model.NewConfigVersionModel(db)
	configSourceBindingModel := model.NewConfigSourceBindingModel(db)
	// 配置浏览器的 croupier 数据源适配器复用共享 ConfigVersionModel
	configsource.SetCroupierVersionModel(configVersionModel)
	taskScheduleModel := model.NewTaskScheduleModel(db)
	deployPackageModel := model.NewDeployPackageModel(db)
	deploymentModel := model.NewDeploymentModel(db)

	// game_envs is the authoritative scope and routing registry. Older
	// deployments stored environments only in games.envs JSON, so backfill
	// missing rows before any authenticated request can resolve a scope.
	databaseNameFor := router.DefaultGameDBName
	if dbRouter != nil {
		databaseNameFor = dbRouter.NameForGame
	}
	if created, err := gameModel.BackfillEnvBindings(context.Background(), databaseNameFor); err != nil {
		// panic-ok: 启动期回填失败意味着 game/env 绑定层不完整，带病启动
		// 会放大成路由错库（CR-FIX-22 豁免清单）。
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
	menuModel := model.NewMenuItemModel(db)

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

	// 默认菜单种子（T-M10）：bootstrap 目录 default-menus.json，缺失/空 →
	// 禁用；解析失败 Error 禁用（不阻断启动，菜单管理仍可手工创建）。
	menuSeeds, err := LoadSeedMenus(configDir)
	if err != nil {
		slog.Default().Error("Default menu seeds disabled: invalid file", "error", err)
	} else if len(menuSeeds) > 0 {
		slog.Default().Info("Default menu seeds enabled", "count", len(menuSeeds), "configDir", configDir)
	}

	opsStateStore := NewOpsStateStore(resolveBootstrapBaseDir(c))

	objectStore, err := initObjectStore(context.Background(), c.Storage, secrets)
	if err != nil {
		// panic-ok: 启动装配期对象存储初始化失败，属不可恢复配置错误
		//（CR-FIX-22 豁免清单）。
		panic(fmt.Sprintf("Failed to initialize object store: %v", err))
	}

	approvalsStore, err := approvals.NewSQLStore(db)
	if err != nil {
		// panic-ok: 启动装配期 approvals 存储初始化失败，审批面不可用
		// 不可恢复（CR-FIX-22 豁免清单）。
		panic(fmt.Sprintf("failed to initialize approvals SQL store: %v", err))
	}

	// 初始化审计服务。错误分支已删（原 if err != nil 降级为 nil）：
	// NewSQLAuditStore 仅对 nil db 报错，而此处 db 来自 openDatabase，
	// 其失败早在函数开头 panic，恒非 nil。
	auditStore, _ := audit.NewSQLAuditStore(db)
	var auditSvc *audit.AuditService
	if auditStore != nil {
		auditSvc = audit.NewAuditService(auditStore, nil)
		// ARCH-007 D4 write-failure policy: strict by default (fail-closed),
		// best_effort only on explicit config. SetWriteFailurePolicy maps
		// empty/unknown values back to strict, so an absent knob keeps the
		// default.
		if c.Audit.WriteFailurePolicy != "" {
			auditSvc.SetWriteFailurePolicy(c.Audit.WriteFailurePolicy)
		}
		slog.Default().Info("AuditService initialized",
			"writeFailurePolicy", auditSvc.WriteFailurePolicy())
	}

	// 执行留痕写入器（R1）：异步批量落库，失败不影响执行主路径
	var execLogWriter *executionlog.Writer
	// 留痕保留期清理器（R3/#54）：ExecutionLog 关闭时不启动、保持 nil
	var logRetention *executionlog.Retention
	// 留痕后台循环的 ctx 必须可取消：ServiceContext.Close 收停 Writer 与
	// Retention（此前挂 context.Background() 永不退出，测试 teardown 临时
	// 目录时在飞 sweep 仍经 router 懒开 sqlite，与 RemoveAll 竞态）。
	var retentionCancel context.CancelFunc
	if c.ExecutionLog.IsEnabled() && db != nil {
		// multiGame：按 entry 的 game+env 路由到对应物理库（否则读写错位）。
		// 注意 typed-nil：仅 multiGame 启用时才传入 router 接口。
		var execLogRouter executionlog.DBRouter
		if dbRouter != nil {
			execLogRouter = dbRouter
		}
		execLogWriter = executionlog.NewWriter(db, executionlog.Config{
			Enabled:         true,
			MaxPayloadBytes: c.ExecutionLog.MaxPayloadBytes,
			Router:          execLogRouter,
		})
		execLogWriter.Run(context.Background())
		slog.Default().Info("ExecutionLog writer started")

		retentionCtx, cancelRetention := context.WithCancel(context.Background())
		retentionCancel = cancelRetention
		// ResolveDays 每轮清理前读 L3 log.retentionDays 覆盖（#54，热生效）。
		execDays, taskDays := c.ExecutionLog.EffectiveRetentionDays(), c.TaskLog.EffectiveRetentionDays()
		logRetention = executionlog.NewRetention(db, executionlog.RetentionConfig{
			ExecutionLogDays: execDays,
			TaskLogDays:      taskDays,
			ResolveDays: func() (int, int) {
				ls := settings.Current()
				if ls == nil {
					return 0, 0
				}
				d := ls.LogsSettings().RetentionDays
				return d, d
			},
			Router:     execLogRouter,
			GameScopes: listGameEnvScopes(db),
		})
		logRetention.Run(retentionCtx)
		slog.Default().Info("Retention sweep loop started",
			"executionLogDays", execDays,
			"taskLogDays", taskDays)
	}

	// 初始化策略管理器
	policyConfigPath := filepath.Join(resolveBootstrapBaseDir(c), "default-policies.yaml")
	policyManager, err := policymgr.NewManager(db, policyConfigPath)
	if err != nil {
		// ARCH-025：配置文件损坏不得静默禁用策略治理。回退到内置默认档位
		// （high/danger 仍需审批），治理链保持强制；配置故障记录在案。
		slog.Default().Error("Failed to load policy config; falling back to built-in defaults (governance stays enforced)",
			"error", err, "configPath", policyConfigPath)
		policyManager = policymgr.NewManagerWithDefaults(db)
	} else {
		slog.Default().Info("PolicyManager initialized", "configPath", policyConfigPath)
	}

	// 初始化缓存。cache 密码支持 vault:// 引用（todo 4251）；解析失败按
	// 「未配置密码」降级（fail-closed，不把引用串当真凭据发往 redis）。
	cacheCfg := c.Cache
	if pw, rerr := secrets(context.Background(), cacheCfg.Password); rerr != nil {
		slog.Default().Error("Cache password resolve failed; continuing without password", "error", rerr)
		cacheCfg.Password = ""
	} else {
		cacheCfg.Password = pw
	}
	cacheStore, err := cache.NewCacheStore(cacheCfg)
	if err != nil {
		slog.Default().Error("Failed to initialize cache, using NullCache", "error", err)
		cacheStore = cache.NewNullCache()
	}
	cacheHelper := cache.NewCacheHelper(cacheStore)
	extensionRepos := extensiongorm.NewBundle(db)
	extensionManifestSvc := extensionmanifest.NewService()
	extensionCatalogSvc := extensioncatalog.NewService(extensionRepos.Catalog, extensionRepos.Release)
	extensionInstallationSvc := extensioninstallation.NewService(extensionRepos.Installation, extensionRepos.Event, extensionRepos.Binding)
	extensionRuntimeSvc := extensionruntime.NewService(extensionRepos.Installation, extensionRepos.Release, extensionRepos.Binding, extensionRepos.Event)
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
		AdminOTPRecoveryModel:     adminOTPRecoveryModel,
		EmailVerificationModel:    emailVerificationModel,
		AlertModel:                alertModel,
		BehaviorModel:             behaviorModel,
		RetentionModel:            retentionModel,
		PaymentsModel:             paymentsModel,
		BackupModel:               backupModel,
		AlertRuleModel:            alertRuleModel,
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
		CicdIntegrationModel:      cicdIntegrationModel,
		CicdBuildModel:            cicdBuildModel,
		ReleaseModel:              releaseModel,
		HotpatchModel:             hotpatchModel,
		DBSourceModel:             dbSourceModel,
		PlatformSettingModel:      platformSettingModel,
		MessageModel:              messageModel,
		ExecutionLogWriter:        execLogWriter,
		LogRetention:              logRetention,
		retentionCancel:           retentionCancel,
		ExecutionLogModel:         model.NewExecutionLogModel(db),
		NotifyService:             nil, // handler 装配时注入（依赖 Layered）
		CertificateModel:          certificateModel,
		ConfigVersionModel:        configVersionModel,
		ConfigSourceBindingModel:  configSourceBindingModel,
		TaskScheduleModel:         taskScheduleModel,
		DeployPackageModel:        deployPackageModel,
		DeploymentModel:           deploymentModel,
		PageSpecModel:             pageSpecModel,
		PublishedPageSpecModel:    publishedPageSpecModel,
		PageVersionModel:          pageVersionModel,
		OpenAPISourceModel:        openAPISourceModel,
		OpenAPISourceBindingModel: openAPISourceBindingModel,
		MenuModel:                 menuModel,
		MenuSeeder:                NewMenuSeeder(menuModel, platformSettingModel, menuSeeds),
		AgentSessionModel:         agentSessionModel,

		// 版本信息（从 version.go 读取，ldflags 注入后会更新）
		ServerVersion:   ServerVersion,
		ServerGitCommit: ServerGitCommit,
		ServerBuildTime: ServerBuildTime,

		// 记录启动时间
		StartTime:            time.Now(),
		AnalyticsFiltersLock: &sync.RWMutex{},

		ObjectStore:    objectStore,
		SMSRegistry:    approvals.NewSMSRegistry(),
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

		if ctx.Config.AgentDispatch.ToAgentTLS.EffectiveEnabled() {
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

	// 引导播种统一 fail-open：权限/角色/管理员三个 seed 的所有失败路径
	// 均降级为日志并恒返 nil，错误分支已删（返回 error 是演进预留）；
	// 扩展目录与词条字典可失败（读盘/反序列化），错误仅记录不中断启动；
	// games seed 的 count 失败是唯一可报错源，同样仅记录。
	_ = seedBootstrapPermissions(ctx)
	if err := seedBootstrapExtensionCatalog(ctx); err != nil {
		slog.Default().Error("failed to seed bootstrap extension catalog", "error", err)
	}
	_ = seedBootstrapRoles(ctx)
	_ = seedBootstrapAdmins(ctx)
	if err := seedBootstrapGames(ctx); err != nil {
		slog.Default().Error("failed to seed bootstrap games", "error", err)
	}
	// 演示审批种子（仅 dev 模式）：approvals 页面三态数据 + 两人复核演示，
	// 依赖 game_envs 绑定解析 scope，故置于 games seed 之后。
	seedDemoApprovals(ctx)
	if err := seedBootstrapTermDictionary(ctx); err != nil {
		slog.Default().Error("failed to seed term dictionary", "error", err)
	}

	// Initialize agent ops stores
	if ctx.MetricsStore == nil {
		ctx.MetricsStore = reg.NewMetricsStore()
	}
	// 指标告警规则评估：上报即评估（阈值命中→Alert+通知）。
	if ctx.AlertRuleModel != nil {
		evaluator := alertrule.New(ctx.AlertRuleModel, ctx.AlertModel, nil)
		ctx.MetricsStore.SetOnReport(evaluator.EvaluateAgent)
	}
	// 告警出口链（plugin-mechanism §5.1）：站内出口链首恒注册（默认必有、
	// 零外发）；herald 显式启用且凭证齐全时注册为外部主出口，否则注册
	// noop（零配置=零外发）。MetricsStore 缺失（裁剪部署）时不接线。
	registerOutlets(ctx)
	// 密码箱（vault-provider §4）：已在函数顶部装配（先于 openDatabase，
	// DSN 引用展开依赖）；此处接入上下文字段。
	ctx.Vault = vaultProvider
	ctx.Secrets = secrets
	if ctx.SystemInfoCache == nil {
		ctx.SystemInfoCache = reg.NewSystemInfoCache()
	}

	// 云资源供给引擎（OPS-001）：provision.enabled=false 不装配（端点如实报
	// 未配置）。RegistryStore/ApprovalsStore 已在上面就位；bootstrap token
	// 不入库、启动时不读取——提交请求时经 BootstrapTokenEnv 环境变量读取
	//（可轮换；env 缺失时该请求被拒，见 api/ops/provision.go 503 语义）。
	if ctx.Config.Provision.Enabled {
		runModel := model.NewProvisionRunModel(ctx.DB)
		orch := &provision.Orchestrator{
			Runner:     provision.NewTfexecRunner(ctx.Config.Provision.EffectiveBinaryPath()),
			Registry:   newProvisionAgentRegistry(ctx.RegistryStore),
			Approval:   newProvisionApprovalGate(ctx.ApprovalsStore),
			Store:      newProvisionDBStore(runModel),
			WorkDir:    ctx.Config.Provision.EffectiveWorkDir(),
			InstallURL: ctx.Config.Provision.AgentInstallURL,
			// #657：ticket 续行时 HubAddr/BootstrapToken 不入库（json:"-"），
			// 须从 server 配置/环境重新注入（token 可轮换），否则渲染出空
			// hubAddr/token 的 cloud-init，agent 引导必然失败。
			InjectResumeParams: func(req *provision.Request) error {
				req.HubAddr = strings.TrimSpace(ctx.Config.Provision.HubAddr)
				if req.HubAddr == "" {
					return fmt.Errorf("provision: hub address not configured")
				}
				req.BootstrapToken = strings.TrimSpace(os.Getenv(ctx.Config.Provision.EffectiveBootstrapTokenEnv()))
				if req.BootstrapToken == "" {
					return fmt.Errorf("provision: bootstrap token not configured")
				}
				return nil
			},
		}
		if t := ctx.Config.Provision.EffectiveRegisterTimeout(); t > 0 {
			orch.RegisterTimeout = t
		}
		ctx.Provision = orch
		slog.Default().Info("provision engine enabled", "binary", ctx.Config.Provision.EffectiveBinaryPath(), "workDir", ctx.Config.Provision.EffectiveWorkDir())
	}

	// 远程电源管理（OPS 电源管理）：powerops.enabled=false 不装配（端点如实
	// 报未配置）。provider 构建失败（如 ipmitool 缺失）只记日志跳过该 kind，
	// 提交时按 provider 缺失诚实失败；ops.power.* 风险声明注入 registry
	//（force_off 走 high+OTP step-up）。
	if ctx.Config.Powerops.Enabled {
		ctx.Powerops = newPoweropsOrchestrator(ctx)
		slog.Default().Info("powerops engine enabled", "verifyTimeoutMs", ctx.Config.Powerops.EffectiveVerifyTimeout(), "verifyIntervalMs", ctx.Config.Powerops.EffectiveVerifyInterval())
	}

	// 初始化 JWT 密钥（从配置文件读取）。值内支持 vault:// 引用（todo 4251）。
	// 原「非开发态 panic / 开发态兜底 DevSecret」的双分支已收敛为直接 panic：
	// ResolveSecret 报错 ⟺ secret 为空且 jwtutil.isDevelopmentMode 为 false，
	// 而该判定与本包 isDevelopmentConfig 对 mode/CROUPIER_ENV/CROUPIER_MODE
	// 逐条等价，err != nil 时 isDevelopmentConfig 必为 false——开发态兜底
	// 分支恒不可达（开发态下 ResolveSecret 自身已返回 DevSecret、err 为 nil）。
	// 引用解析失败走同一 panic 通道（fail-closed：不回落 DevSecret）；展开后
	// 明文写回 ctx.Config，防下游（routes 装配等）再读到原始引用。
	secret, err := resolveJWTSecret(ctx.Config, secrets)
	if err != nil {
		// JWT secret 未配置/引用不可解析且不在开发模式，这是一个严重配置错误
		// panic-ok: 启动期安全配置缺失，拒绝启动优于降级无签名 token
		//（CR-FIX-22 豁免清单）。
		panic(fmt.Sprintf("JWT secret not configured: %v", err))
	}
	ctx.Config.Auth.JWTSecret = secret
	jwtutil.InitGlobalSecret(secret)

	// 设置认证中间件
	ctx.Authority = NewAuthMiddleware(ctx)

	return ctx
}

// Close 收停 NewServiceContext 起动的后台 goroutine 并释放连接池：
// 留痕写入器冲刷队列后退出，保留期清理 cancel 后等待在飞的一轮 sweep 结束，
// 随后按序关闭 per-game Router 连接、缓存与元库连接池（ARCH-017 有界清理）。
// 进程内多实例（测试、备件装配）teardown 临时目录前必须先 Close——后台
// sweep 仍经 router 懒开 per-game sqlite 时 RemoveAll 会撞文件竞态。
//
// 调用方契约：优雅停机必须在 drain 完在途 HTTP 请求之后调用 Close——审计
// 与执行留痕的写路径是同步/队列式的，池一关就永远写不进了；顺序是
// 「drain → 停后台 → Close」，Close 自身不再等待任何在飞请求。
// 幂等，可安全重复调用。
func (ctx *ServiceContext) Close() {
	if ctx == nil {
		return
	}
	ctx.closeOnce.Do(func() {
		// 1. 执行留痕异步写入器：冲刷队列残余条目（写库须在池关闭前）。
		if ctx.ExecutionLogWriter != nil {
			ctx.ExecutionLogWriter.Stop()
		}
		// 2. 留痕保留期清理：cancel 后等在飞 sweep 收尾（同上依赖池）。
		if ctx.retentionCancel != nil {
			ctx.retentionCancel()
		}
		if ctx.LogRetention != nil {
			ctx.LogRetention.Wait()
		}
		// 3. 释放连接池：per-game Router 先关，缓存次之，元库最后——
		//    步骤 1/2 的落库路径直到此刻前都有活池可用。
		if ctx.Router != nil {
			if err := ctx.Router.Close(); err != nil {
				slog.Default().Error("close game database router failed", "error", err)
			}
		}
		if ctx.Cache != nil {
			if err := ctx.Cache.Close(); err != nil {
				slog.Default().Error("close cache failed", "error", err)
			}
		}
		if ctx.DB != nil {
			if sqlDB, err := ctx.DB.DB(); err == nil && sqlDB != nil {
				if err := sqlDB.Close(); err != nil {
					slog.Default().Error("close meta database pool failed", "error", err)
				}
			}
		}
	})
}

// GameScopeContext 解析 (gameId, env) 的游戏作用域 context（含 per-game DB
// 覆盖）。ops 供给销毁守卫按运行作用域查 task_runs（在途任务闸）经此取
// game 库连接。
func (ctx *ServiceContext) GameScopeContext(gameID, env string) context.Context {
	return ctx.scopeContextForBackgroundRegistration(gameID, env)
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
