package svc

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/cuihairu/croupier/internal/cache"
	"github.com/cuihairu/croupier/internal/config"
	extensioncatalog "github.com/cuihairu/croupier/internal/core/extension/catalog"
	extensioninstallation "github.com/cuihairu/croupier/internal/core/extension/installation"
	extensionmanifest "github.com/cuihairu/croupier/internal/core/extension/manifest"
	extensionruntime "github.com/cuihairu/croupier/internal/core/extension/runtime"
	extensionsync "github.com/cuihairu/croupier/internal/core/extension/sync"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/pkg2/jwt"
	plat "github.com/cuihairu/croupier/internal/platform"
	"github.com/cuihairu/croupier/internal/platform/approvals"
	dispatch "github.com/cuihairu/croupier/internal/platform/dispatch"
	"github.com/cuihairu/croupier/internal/platform/migrationflags"
	objstore "github.com/cuihairu/croupier/internal/platform/objstore"
	reg "github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/cuihairu/croupier/internal/platform/tlsutil"
	extensiongorm "github.com/cuihairu/croupier/internal/repo/gorm/extension"
	"github.com/cuihairu/croupier/internal/runtime"
	jwtutil2 "github.com/cuihairu/croupier/internal/security/jwtutil"
	"github.com/cuihairu/croupier/internal/service/permission"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"log/slog"
)

type ServiceContext struct {
	Config            config.Config
	Authority         gin.HandlerFunc
	AdminManager      *AdminManager
	OpsStateStore     *OpsStateStore
	DB                *gorm.DB
	PermissionService *permission.PermissionService
	RegistryStore     *reg.Store
	Dispatcher        *dispatch.Dispatcher
	Cache             cache.CacheStore
	CacheHelper       *cache.CacheHelper

	AnalyticsFiltersLock *sync.RWMutex

	ApprovalsStore approvals.Store

	ObjectStore objstore.Store

	PlatformLoader *plat.Loader

	// Agent Ops support
	MetricsStore    *reg.MetricsStore
	SystemInfoCache *reg.SystemInfoCache

	AdminModel           *model.AdminModel
	AlertModel           *model.AlertModel
	BehaviorModel        *model.BehaviorModel
	RetentionModel       *model.RetentionModel
	PaymentsModel        *model.PaymentsModel
	BackupModel          *model.BackupModel
	FAQModel             *model.FAQModel
	FeedbackModel        *model.FeedbackModel
	EntityModel          *model.EntityModel
	GameModel            *model.GameModel
	PlayerModel          *model.PlayerModel
	ProfileModel         *model.ProfileModel
	FunctionModel        *model.FunctionModel
	TermDictModel        *model.TermDictionaryModel
	RoleModel            *model.RoleModel
	NodeModel            *model.NodeModel
	PermissionModel      *model.PermissionModel
	RateLimitModel       *model.RateLimitModel
	SupportModel         *model.SupportModel
	TicketModel          *model.TicketModel
	MessageModel         *model.MessageModel
	CertificateModel     *model.CertificateModel
	ConfigVersionModel   *model.ConfigVersionModel
	WorkspaceConfigModel *model.WorkspaceConfigModel

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
	db, err := openDatabase(c)
	if err != nil {
		panic(fmt.Sprintf("Failed to connect to database: %v", err))
	}

	// 自动迁移数据库模型
	if err := autoMigrate(db); err != nil {
		panic(fmt.Sprintf("Failed to auto migrate database: %v", err))
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
	entityModel := model.NewEntityModel(db)
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
	messageModel := model.NewMessageModel(db)
	certificateModel := model.NewCertificateModel(db)
	configVersionModel := model.NewConfigVersionModel(db)
	workspaceConfigModel := model.NewWorkspaceConfigModel(db)

	// Agent Session Model for database persistence
	agentSessionModel := reg.NewAgentSessionModel(db)

	// 创建管理员管理器（基于JSON文件）
	configDir := resolveBootstrapAuthDir(c)
	adminManager := NewAdminManager(configDir)
	if err := adminManager.Initialize(); err != nil {
		// 如果初始化失败，记录错误但不停止服务
		// 这样可以让服务启动，但登录功能可能受限
		// 在生产环境中应该更严格地处理这个错误
	}

	opsStateStore := NewOpsStateStore(resolveBootstrapBaseDir(c))

	objectStore, err := initObjectStore(context.Background(), c.Storage)
	if err != nil {
		panic(fmt.Sprintf("Failed to initialize object store: %v", err))
	}

	approvalsStore := approvals.NewMemStore()

	// 初始化平台加载器
	platformLoader := initPlatformLoader(c)

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
		PermissionService: permissionService,
		Cache:             cacheStore,
		CacheHelper:       cacheHelper,

		AdminModel:           adminModel,
		AlertModel:           alertModel,
		BehaviorModel:        behaviorModel,
		RetentionModel:       retentionModel,
		PaymentsModel:        paymentsModel,
		BackupModel:          backupModel,
		FAQModel:             faqModel,
		FeedbackModel:        feedbackModel,
		EntityModel:          entityModel,
		GameModel:            gameModel,
		PlayerModel:          playerModel,
		ProfileModel:         profileModel,
		FunctionModel:        functionModel,
		TermDictModel:        termDictModel,
		RoleModel:            roleModel,
		NodeModel:            nodeModel,
		PermissionModel:      permissionModel,
		RateLimitModel:       rateLimitModel,
		SupportModel:         supportModel,
		TicketModel:          ticketModel,
		MessageModel:         messageModel,
		CertificateModel:     certificateModel,
		ConfigVersionModel:   configVersionModel,
		WorkspaceConfigModel: workspaceConfigModel,
		AgentSessionModel:    agentSessionModel,

		// 版本信息（从 version.go 读取，ldflags 注入后会更新）
		ServerVersion:   ServerVersion,
		ServerGitCommit: ServerGitCommit,
		ServerBuildTime: ServerBuildTime,

		// 记录启动时间
		StartTime:            time.Now(),
		AnalyticsFiltersLock: &sync.RWMutex{},

		ObjectStore:    objectStore,
		ApprovalsStore: approvalsStore,
		PlatformLoader: platformLoader,
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
	if ctx.Dispatcher == nil {
		var jobStore dispatch.JobRoutingStore
		jobRoutingDir := resolveJobRoutingDir(ctx.Config)
		if jobRoutingDir != "" {
			store, err := dispatch.NewFileJobRoutingStore(jobRoutingDir)
			if err != nil {
				slog.Default().Error("failed to init job routing store", "dir", jobRoutingDir, "error", err)
			} else {
				jobStore = store
			}
		}
		ctx.Dispatcher = dispatch.NewDispatcherWithJobStore(ctx.RegistryStore, jobStore)

		if ttlStr := strings.TrimSpace(ctx.Config.AgentDispatch.JobRoutingTTL); ttlStr != "" {
			if ttl, err := time.ParseDuration(ttlStr); err != nil {
				slog.Default().Error("invalid dispatch.job_routing_ttl", "value", ttlStr, "error", err)
			} else if ttl > 0 {
				if err := ctx.Dispatcher.CleanupOldJobs(ttl); err != nil {
					slog.Default().Error("failed to cleanup old jobs", "error", err)
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
	if err := seedBootstrapWorkspaces(ctx); err != nil {
		slog.Default().Error("failed to seed bootstrap workspaces", "error", err)
	}

	// Initialize agent ops stores
	if ctx.MetricsStore == nil {
		ctx.MetricsStore = reg.NewMetricsStore()
	}
	if ctx.SystemInfoCache == nil {
		ctx.SystemInfoCache = reg.NewSystemInfoCache()
	}

	// 初始化 JWT 密钥（从配置文件读取）
	secret, _ := jwtutil2.ResolveSecret(ctx.Config)
	jwt.SetSecret(secret)

	// 设置认证中间件
	ctx.Authority = NewAuthMiddleware(ctx)

	return ctx
}

// autoMigrate runs all necessary database migrations
func autoMigrate(db *gorm.DB) error {
	// Import all model packages and run their AutoMigrate functions
	if err := model.AutoMigrate(db); err != nil {
		return fmt.Errorf("failed to migrate server models: %w", err)
	}

	// Migrate Agent Session model
	if err := reg.MigrateAgentSessions(db); err != nil {
		return fmt.Errorf("failed to migrate agent sessions: %w", err)
	}

	return nil
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

func resolveJobRoutingDir(c config.Config) string {
	if dir := strings.TrimSpace(c.AgentDispatch.JobRoutingDir); dir != "" {
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
		return nil
	}

	admins := ctx.AdminManager.ListAdmins()
	if len(admins) == 0 {
		return nil
	}

	bg := context.Background()
	for _, admin := range admins {
		username := strings.TrimSpace(admin.Username)
		if username == "" || strings.TrimSpace(admin.Password) == "" {
			continue
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

// initPlatformLoader initializes the third-party platform loader
func initPlatformLoader(c config.Config) *plat.Loader {
	if migrationflags.IsLegacyDisabled() {
		slog.Default().Info("Third-party platform legacy loader disabled by env")
		return nil
	}
	if migrationflags.IsExtensionOnly() {
		slog.Default().Info("Third-party platform legacy loader skipped in extension-only mode")
		return nil
	}
	// Default platform config path
	configFile := "configs/platforms.yaml"
	if c.Platforms.ConfigFile != "" {
		configFile = c.Platforms.ConfigFile
	}

	// If platform integration is explicitly disabled, return nil
	if !c.Platforms.Enabled {
		slog.Default().Info("Third-party platform integration is disabled")
		return nil
	}
	slog.Default().Warn("Third-party platform legacy loader is enabled (deprecated path); prefer extension-first external-platform. Set CROUPIER_PLATFORM_LEGACY_DISABLED=true to disable legacy loader")

	loader := plat.NewLoader(configFile, nil)

	// Load platform configurations
	if err := loader.Load(context.Background()); err != nil {
		slog.Default().Error("Failed to load platform configurations", "error", err)
		// Return loader anyway so it can be used for runtime reload
		return loader
	}

	slog.Default().Info("Third-party platform loader initialized", "config", configFile)
	return loader
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
	case "obs":
		return objstore.OpenOBS(ctx, storeCfg)
	case "file":
		return objstore.OpenFile(ctx, storeCfg)
	default:
		return nil, fmt.Errorf("unsupported storage driver: %s", driver)
	}
}

// seedBootstrapWorkspaces creates default workspace configurations for registered functions.
// It groups functions by their prefix (e.g., "examples.player", "packs.prom") and creates
// a workspace for each group.
func seedBootstrapWorkspaces(ctx *ServiceContext) error {
	if ctx == nil || ctx.FunctionModel == nil || ctx.WorkspaceConfigModel == nil {
		return nil
	}

	bg := context.Background()
	functionIDs, err := collectWorkspaceBootstrapFunctionIDs(ctx, bg)
	if err != nil {
		return err
	}
	if len(functionIDs) == 0 {
		return nil
	}

	// Group functions by prefix (e.g., "examples.player" -> ["examples.player.get", "examples.player.create"])
	groups := make(map[string][]string)
	for _, functionID := range functionIDs {
		parts := strings.SplitN(functionID, ".", 3)
		if len(parts) < 2 {
			continue
		}
		prefix := parts[0] + "." + parts[1]
		groups[prefix] = append(groups[prefix], functionID)
	}

	// Create workspace for each group
	menuOrder := 0
	for prefix, functionIDs := range groups {
		// Check if workspace already exists
		_, err := ctx.WorkspaceConfigModel.FindByObjectKey(bg, prefix)
		if err == nil {
			continue // Already exists
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			slog.Default().Error("failed to check workspace", "prefix", prefix, "error", err)
			continue
		}

		// Create workspace config JSON
		layout := buildDefaultWorkspaceLayout(prefix, functionIDs)
		configJSON, err := json.Marshal(layout)
		if err != nil {
			slog.Default().Error("failed to marshal workspace config", "prefix", prefix, "error", err)
			continue
		}

		// Extract title from prefix (e.g., "examples.player" -> "Player")
		titleParts := strings.Split(prefix, ".")
		title := titleParts[len(titleParts)-1]
		title = strings.ToUpper(string(title[0])) + title[1:] // Capitalize first letter

		workspace := &model.WorkspaceConfig{
			ObjectKey: prefix,
			Title:     title,
			Published: false,
			MenuOrder: menuOrder,
			Config:    configJSON,
		}

		if err := ctx.WorkspaceConfigModel.Upsert(bg, workspace); err != nil {
			slog.Default().Error("failed to create workspace", "prefix", prefix, "error", err)
			continue
		}

		slog.Default().Info("created default workspace", "prefix", prefix, "functions", len(functionIDs))
		menuOrder++
	}

	return nil
}

// EnsureWorkspaceSeeded ensures bootstrap workspace configs exist based on current function catalog.
func (ctx *ServiceContext) EnsureWorkspaceSeeded() error {
	return seedBootstrapWorkspaces(ctx)
}

func collectWorkspaceBootstrapFunctionIDs(ctx *ServiceContext, bg context.Context) ([]string, error) {
	ids := make(map[string]struct{})

	// 1) Database-backed function catalog.
	functions, _, err := ctx.FunctionModel.List(bg, model.ListFunctionsOptions{
		PaginationOptions: model.PaginationOptions{
			Page:     1,
			PageSize: 10000,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list functions: %w", err)
	}
	for _, fn := range functions {
		fid := strings.TrimSpace(fn.FunctionID)
		if fid == "" {
			continue
		}
		ids[fid] = struct{}{}
	}

	// 2) Runtime registry functions (covers agent-registered functions not yet persisted in DB).
	if ctx.RegistryStore != nil {
		ctx.RegistryStore.Mu().RLock()
		for _, sess := range ctx.RegistryStore.AgentsUnsafe() {
			if sess == nil {
				continue
			}
			for fid := range sess.Functions {
				fid = strings.TrimSpace(fid)
				if fid == "" {
					continue
				}
				ids[fid] = struct{}{}
			}
		}
		ctx.RegistryStore.Mu().RUnlock()
	}

	out := make([]string, 0, len(ids))
	for fid := range ids {
		out = append(out, fid)
	}
	return out, nil
}

// buildDefaultWorkspaceLayout creates a default layout for a workspace
func buildDefaultWorkspaceLayout(objectKey string, functionIDs []string) map[string]interface{} {
	// Create tabs layout with one tab per function (up to 10)
	tabs := make([]map[string]interface{}, 0, min(len(functionIDs), 10))

	for i, fnID := range functionIDs {
		if i >= 10 {
			break
		}

		// Extract function name from ID (e.g., "examples.player.get" -> "get")
		parts := strings.Split(fnID, ".")
		fnName := parts[len(parts)-1]

		tab := map[string]interface{}{
			"key":       fnID,
			"title":     fnName,
			"functions": []string{fnID}, // 添加 functions 字段
			"layout": map[string]interface{}{
				"type":           "form",
				"submitFunction": fnID,
				"fields":         []map[string]interface{}{},
			},
		}
		tabs = append(tabs, tab)
	}

	return map[string]interface{}{
		"objectKey": objectKey,
		"title":     objectKey,
		"layout": map[string]interface{}{
			"type": "tabs",
			"tabs": tabs,
		},
		"published": false,
		"menuOrder": 0,
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
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
	svcCtx               *ServiceContext
	allowPaths           map[string]struct{}
	allowPref            []string
	publicReadPrefixes   []string
	publicReadExactPaths map[string]struct{}
}

// NewAuthMiddlewareImpl 创建认证中间件实例
func NewAuthMiddlewareImpl(svcCtx *ServiceContext) *AuthMiddleware {
	return &AuthMiddleware{
		svcCtx: svcCtx,
		allowPaths: map[string]struct{}{
			"/api/v1/auth/login":         {},
			"/api/v1/monitoring/health":  {},
			"/api/v1/monitoring/healthz": {},
		},
		allowPref: []string{
			"/api/v1/auth/login",
		},
		publicReadPrefixes: []string{
			"/api/v1/configs",
			"/api/v1/registry",              // 公开访问：注册中心（agents、functions）
			"/api/v1/functions/descriptors", // 公开访问：函数描述符列表
		},
		publicReadExactPaths: map[string]struct{}{
			"/api/v1/functions": {}, // 公开访问：函数列表（精确匹配，子路径需认证）
		},
	}
}

// Handle 处理认证中间件（Gin 风格）
func (m *AuthMiddleware) Handle(c *gin.Context) {
	if m.shouldBypassGin(c) {
		c.Next()
		return
	}

	// 获取 Authorization header
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		// 兼容 SSE 等无法自定义 header 的场景，支持 token 查询参数
		if token := strings.TrimSpace(c.Query("token")); token != "" {
			authHeader = "Bearer " + token
		}
	}
	if authHeader == "" {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing authorization header", "message": "未授权"})
		return
	}

	// 解析 Bearer token
	tokenParts := strings.SplitN(authHeader, " ", 2)
	if len(tokenParts) != 2 || tokenParts[0] != "Bearer" {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid authorization header format", "message": "授权头格式错误"})
		return
	}

	token := tokenParts[1]

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
	// 使用 JWT 包验证 token
	claims, err := jwt.ParseToken(token)
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
		// 精确路径匹配
		if _, ok := m.publicReadExactPaths[path]; ok {
			return true
		}
		// 前缀匹配
		for _, prefix := range m.publicReadPrefixes {
			if strings.HasPrefix(path, prefix) {
				return true
			}
		}
	}
	return false
}
