package config

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	ctxmanager "github.com/cuihairu/croupier/internal/context"
	"github.com/cuihairu/croupier/internal/database"
	"github.com/cuihairu/croupier/internal/errors"
	"github.com/zeromicro/go-zero/rest"
	"github.com/zeromicro/go-zero/rest/httpx"
)

// IntegratedConfigManager 集成配置管理器
type IntegratedConfigManager struct {
	*Manager
	dbManager    *database.Manager
	errorFactory *errors.ErrorFactory
	contextMgr   *ctxmanager.Manager
	apiServer    *rest.Server
}

// NewIntegratedConfigManager 创建集成配置管理器
func NewIntegratedConfigManager(ctx context.Context) (*IntegratedConfigManager, error) {
	// 创建错误工厂
	errorFactory := errors.NewErrorFactory("config-integration")

	// 创建上下文管理器
	contextMgr := ctxmanager.NewManager(30 * time.Second)

	// 创建基础配置管理器
	baseManager, err := NewManager(ctx,
		WithErrorFactory(errorFactory),
		WithContextManager(contextMgr),
	)
	if err != nil {
		return nil, fmt.Errorf("创建基础配置管理器失败: %w", err)
	}

	return &IntegratedConfigManager{
		Manager:      baseManager,
		errorFactory: errorFactory,
		contextMgr:   contextMgr,
	}, nil
}

// Initialize 初始化集成配置管理器
func (icm *IntegratedConfigManager) Initialize(ctx context.Context) error {
	// 1. 加载配置
	err := icm.loadConfiguration(ctx)
	if err != nil {
		return fmt.Errorf("加载配置失败: %w", err)
	}

	// 2. 初始化数据库连接
	err = icm.initializeDatabase(ctx)
	if err != nil {
		return fmt.Errorf("初始化数据库失败: %w", err)
	}

	// 3. 启动配置HTTP API
	err = icm.startConfigurationAPI(ctx)
	if err != nil {
		return fmt.Errorf("启动配置API失败: %w", err)
	}

	// 4. 启动配置监听
	err = icm.startConfigurationWatcher(ctx)
	if err != nil {
		return fmt.Errorf("启动配置监听失败: %w", err)
	}

	log.Println("集成配置管理器初始化完成")
	return nil
}

// loadConfiguration 加载配置
func (icm *IntegratedConfigManager) loadConfiguration(ctx context.Context) error {
	// 根据环境决定配置源
	if err := icm.LoadFromFile("configs/config.example.yaml"); err != nil {
		log.Printf("加载默认配置失败，使用环境变量: %v", err)
	}

	// 从环境变量加载覆盖配置
	envSource := NewEnvConfigSource("CROUPIER_", false)
	sources := []*ConfigSource{envSource}

	return icm.LoadFromMultiple(sources)
}

// initializeDatabase 初始化数据库连接
func (icm *IntegratedConfigManager) initializeDatabase(ctx context.Context) error {
	dbConfig := icm.GetDatabaseConfig()

	if !dbConfig.Enabled {
		log.Println("数据库未启用，跳过数据库初始化")
		return nil
	}

	// 构建数据库连接字符串
	dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		dbConfig.Primary.Host,
		dbConfig.Primary.Port,
		dbConfig.Primary.Username,
		dbConfig.Primary.Password,
		dbConfig.Primary.Database,
		dbConfig.Primary.SSLMode,
	)

	// 创建数据库配置
	databaseConfig := &database.Config{
		MaxOpenConns:        dbConfig.ConnectionPool.MaxOpenConns,
		MaxIdleConns:        dbConfig.ConnectionPool.MaxIdleConns,
		ConnMaxLifetime:     dbConfig.ConnectionPool.ConnMaxLifetime,
		HealthCheckInterval: 30 * time.Second,
		MetricsEnabled:      true,
	}

	// 创建数据库管理器
	dbManager, err := database.NewManager(dsn, databaseConfig)
	if err != nil {
		return icm.wrapError("创建数据库管理器失败", err, "initializeDatabase")
	}

	icm.dbManager = dbManager

	// 测试数据库连接
	err = icm.dbManager.Ping(ctx)
	if err != nil {
		return icm.wrapError("数据库连接测试失败", err, "initializeDatabase")
	}

	log.Printf("数据库连接成功: %s:%d/%s",
		dbConfig.Primary.Host, dbConfig.Primary.Port, dbConfig.Primary.Database)

	return nil
}

// startConfigurationAPI 启动配置HTTP API
func (icm *IntegratedConfigManager) startConfigurationAPI(ctx context.Context) error {
	networkConfig := icm.GetNetworkConfig()

	host := strings.TrimSpace(networkConfig.Server.Host)
	restConf := rest.RestConf{
		Host: host,
		Port: networkConfig.Server.HTTPPort,
	}
	restConf.Mode = "dev"
	restConf.Timeout = 30 * 1000 // ms
	server := rest.MustNewServer(restConf)
	icm.setupConfigurationRoutes(server)
	icm.apiServer = server

	go func() {
		addr := fmt.Sprintf("%s:%d", host, networkConfig.Server.HTTPPort)
		if host == "" {
			addr = fmt.Sprintf(":%d", networkConfig.Server.HTTPPort)
		}
		log.Printf("配置API服务器启动在: %s", addr)
		server.Start()
	}()

	return nil
}

// setupConfigurationRoutes 设置配置API路由
func (icm *IntegratedConfigManager) setupConfigurationRoutes(server *rest.Server) {
	server.AddRoutes(
		[]rest.Route{
			{Method: http.MethodGet, Path: "/", Handler: icm.handleGetCurrentConfig},
			{Method: http.MethodGet, Path: "/sources", Handler: icm.handleGetConfigSources},
			{Method: http.MethodPost, Path: "/reload", Handler: icm.handleReloadConfig},
			{Method: http.MethodPost, Path: "/validate", Handler: icm.handleValidateConfig},
			{Method: http.MethodGet, Path: "/env", Handler: icm.handleGetEnvInfo},
			{Method: http.MethodGet, Path: "/export/:format", Handler: icm.handleExportConfig},
			{Method: http.MethodGet, Path: "/health", Handler: icm.handleHealthCheck},
		},
		rest.WithPrefix("/api/v1/config"),
	)
}

// handleGetCurrentConfig 获取当前配置
func (icm *IntegratedConfigManager) handleGetCurrentConfig(w http.ResponseWriter, r *http.Request) {
	config := icm.GetConfig()

	// 移除敏感信息
	safeConfig := icm.sanitizeConfig(config)

	httpx.WriteJson(w, http.StatusOK, map[string]any{
		"success":   true,
		"data":      safeConfig,
		"timestamp": time.Now(),
	})
}

// handleGetConfigSources 获取配置源信息
func (icm *IntegratedConfigManager) handleGetConfigSources(w http.ResponseWriter, r *http.Request) {
	sources := icm.GetConfigSources()

	httpx.WriteJson(w, http.StatusOK, map[string]any{
		"success":   true,
		"data":      sources,
		"timestamp": time.Now(),
	})
}

// handleReloadConfig 重新加载配置
func (icm *IntegratedConfigManager) handleReloadConfig(w http.ResponseWriter, r *http.Request) {
	err := icm.Reload()
	if err != nil {
		icm.SendError(w, r, icm.errorFactory.InternalError("reload_config", err))
		return
	}

	httpx.WriteJson(w, http.StatusOK, map[string]any{
		"success":   true,
		"message":   "配置重新加载成功",
		"timestamp": time.Now(),
	})
}

// handleValidateConfig 验证配置
func (icm *IntegratedConfigManager) handleValidateConfig(w http.ResponseWriter, r *http.Request) {
	config := icm.GetConfig()
	validator := NewDefaultValidator()

	err := validator.Validate(config)
	if err != nil {
		httpx.WriteJson(w, http.StatusBadRequest, map[string]any{
			"success":   false,
			"error":     err.Error(),
			"timestamp": time.Now(),
		})
		return
	}

	httpx.WriteJson(w, http.StatusOK, map[string]any{
		"success":   true,
		"message":   "配置验证通过",
		"timestamp": time.Now(),
	})
}

// handleGetEnvInfo 获取环境变量信息
func (icm *IntegratedConfigManager) handleGetEnvInfo(w http.ResponseWriter, r *http.Request) {
	envManager := NewEnvManager("CROUPIER_")
	envInfo := envManager.GetEnvInfo()

	httpx.WriteJson(w, http.StatusOK, map[string]any{
		"success":   true,
		"data":      envInfo,
		"timestamp": time.Now(),
	})
}

// handleExportConfig 导出配置
func (icm *IntegratedConfigManager) handleExportConfig(w http.ResponseWriter, r *http.Request) {
	format := r.PathValue("format")

	data, err := icm.Export(format)
	if err != nil {
		icm.SendError(w, r, icm.errorFactory.InvalidInputError("export_config", "format", format, err))
		return
	}

	contentType := "application/yaml"
	if format == "json" {
		contentType = "application/json"
	}

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=config.%s", format))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

// handleHealthCheck 配置健康检查
func (icm *IntegratedConfigManager) handleHealthCheck(w http.ResponseWriter, r *http.Request) {
	status := map[string]interface{}{
		"config_loaded":      true,
		"database_connected": false,
		"last_reload":        time.Now(),
	}

	// 检查数据库连接
	if icm.dbManager != nil {
		ctx, cancel := icm.contextMgr.ForDatabase(r.Context())
		defer cancel()

		if err := icm.dbManager.Ping(ctx); err == nil {
			status["database_connected"] = true
		}
	}

	httpx.WriteJson(w, http.StatusOK, map[string]any{
		"status":    "healthy",
		"details":   status,
		"timestamp": time.Now(),
	})
}

// sanitizeConfig 清理配置中的敏感信息
func (icm *IntegratedConfigManager) sanitizeConfig(config *Config) *Config {
	// 创建配置副本
	safeConfig := icm.cloneConfig(config)

	// 清理敏感信息
	if safeConfig.Security.JWT.Secret != "" {
		safeConfig.Security.JWT.Secret = "***REDACTED***"
	}

	if safeConfig.Database.Primary.Password != "" {
		safeConfig.Database.Primary.Password = "***REDACTED***"
	}

	if safeConfig.Database.ReadOnly.Enabled {
		for i := range safeConfig.Database.ReadOnly.Replicas {
			if safeConfig.Database.ReadOnly.Replicas[i].Password != "" {
				safeConfig.Database.ReadOnly.Replicas[i].Password = "***REDACTED***"
			}
		}
	}

	return safeConfig
}

// startConfigurationWatcher 启动配置监听
func (icm *IntegratedConfigManager) startConfigurationWatcher(ctx context.Context) error {
	// 监听配置变更
	icm.WatchConfig(ctx, func(config *Config, err error) {
		if err != nil {
			log.Printf("配置监听错误: %v", err)
			return
		}

		log.Println("检测到配置变更，重新初始化组件...")

		// 重新初始化数据库连接
		if dbErr := icm.initializeDatabase(ctx); dbErr != nil {
			log.Printf("重新初始化数据库失败: %v", dbErr)
		}
	})

	return nil
}

// SendError 发送错误响应
func (icm *IntegratedConfigManager) SendError(w http.ResponseWriter, r *http.Request, err error) {
	if appErr, ok := err.(*errors.AppError); ok {
		for k, v := range appErr.HTTPHeaders {
			if strings.TrimSpace(k) == "" {
				continue
			}
			w.Header().Set(k, v)
		}
		httpx.WriteJson(w, int(appErr.HTTPStatusCode), map[string]any{
			"success": false,
			"error": map[string]any{
				"code":    appErr.Code,
				"message": appErr.Message,
				"details": appErr.Details,
			},
			"timestamp": time.Now(),
		})
		return
	}

	httpx.WriteJson(w, http.StatusInternalServerError, map[string]any{
		"success": false,
		"error": map[string]any{
			"code":    "INTERNAL_ERROR",
			"message": err.Error(),
		},
		"timestamp": time.Now(),
	})
}

// Shutdown 关闭集成配置管理器
func (icm *IntegratedConfigManager) Shutdown(ctx context.Context) error {
	log.Println("正在关闭集成配置管理器...")

	// 关闭HTTP服务器
	if icm.apiServer != nil {
		icm.apiServer.Stop()
	}

	// 关闭数据库连接
	if icm.dbManager != nil {
		if err := icm.dbManager.Close(); err != nil {
			log.Printf("关闭数据库连接失败: %v", err)
		}
	}

	// 关闭配置管理器
	if err := icm.Manager.Close(); err != nil {
		log.Printf("关闭配置管理器失败: %v", err)
	}

	log.Println("集成配置管理器已关闭")
	return nil
}

// wrapError 包装错误
func (icm *IntegratedConfigManager) wrapError(message string, err error, operation string) error {
	if icm.errorFactory != nil {
		return icm.errorFactory.Wrap(err, operation).WithDetails(message)
	}
	return fmt.Errorf("%s: %w", message, err)
}

// cloneConfig 克隆配置
func (icm *IntegratedConfigManager) cloneConfig(config *Config) *Config {
	// 简化的克隆实现，实际应该使用序列化
	if config == nil {
		return nil
	}

	return &Config{
		App:           config.App,
		Network:       config.Network,
		Database:      config.Database,
		Security:      config.Security,
		Observability: config.Observability,
		Business:      config.Business,
		Storage:       config.Storage,
	}
}

// ExampleIntegrationUsage 集成使用示例
func ExampleIntegrationUsage() {
	ctx := context.Background()

	// 创建集成配置管理器
	integratedMgr, err := NewIntegratedConfigManager(ctx)
	if err != nil {
		log.Fatal("创建集成配置管理器失败:", err)
	}

	// 初始化
	err = integratedMgr.Initialize(ctx)
	if err != nil {
		log.Fatal("初始化集成配置管理器失败:", err)
	}

	// 设置信号处理
	// 在实际应用中，这里应该处理SIGINT、SIGTERM等信号
	defer integratedMgr.Shutdown(ctx)

	log.Println("集成配置管理系统启动完成")
	log.Println("配置API: http://localhost:8080/api/v1/config")
	log.Println("健康检查: http://localhost:8080/api/v1/config/health")

	// 保持服务运行
	select {
	case <-ctx.Done():
		log.Println("收到停止信号，正在关闭...")
	}
}
