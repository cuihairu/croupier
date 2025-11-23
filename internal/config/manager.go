package config

import (
	"context"
	"fmt"
	"sync"
	"time"

	ctxmanager "github.com/cuihairu/croupier/internal/context"
	"github.com/cuihairu/croupier/internal/errors"
)

// Manager 配置管理器
type Manager struct {
	loader        Loader
	mu            sync.RWMutex
	ctx           context.Context
	cancel        context.CancelFunc
	watchCancel   context.CancelFunc
	errorFactory  *errors.ErrorFactory
	contextMgr    *ctxmanager.Manager
	currentConfig *Config
	restartChan   chan struct{}
}

// ManagerOption 管理器选项
type ManagerOption func(*Manager)

// WithErrorFactory 设置错误工厂
func WithErrorFactory(factory *errors.ErrorFactory) ManagerOption {
	return func(m *Manager) {
		m.errorFactory = factory
	}
}

// WithContextManager 设置上下文管理器
func WithContextManager(ctxMgr *ctxmanager.Manager) ManagerOption {
	return func(m *Manager) {
		m.contextMgr = ctxMgr
	}
}

// NewManager 创建配置管理器
func NewManager(ctx context.Context, options ...ManagerOption) (*Manager, error) {
	managerCtx, cancel := context.WithCancel(ctx)

	mgr := &Manager{
		ctx:          managerCtx,
		cancel:       cancel,
		restartChan:  make(chan struct{}, 1),
		errorFactory: errors.NewErrorFactory("config-manager"),
	}

	// 应用选项
	for _, opt := range options {
		opt(mgr)
	}

	// 创建配置加载器
	loaderOptions := []Option{
		WithDefaultTimeout(30 * time.Second),
		WithRetryCount(3),
		WithRetryDelay(1 * time.Second),
	}

	mgr.loader = NewLoader(managerCtx, loaderOptions...)

	return mgr, nil
}

// LoadFromFile 从文件加载配置
func (m *Manager) LoadFromFile(path string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if err := m.loader.FromFile(path); err != nil {
		return m.wrapError("从文件加载配置失败", err, "LoadFromFile")
	}

	// 验证配置
	if err := m.loader.Validate(); err != nil {
		return m.wrapError("配置验证失败", err, "LoadFromFile")
	}

	// 更新当前配置
	m.currentConfig = m.loader.Get()

	return nil
}

// LoadFromMultiple 从多个源加载配置
func (m *Manager) LoadFromMultiple(sources []*ConfigSource) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var lastError error

	// 按顺序加载配置源
	for _, source := range sources {
		var err error

		switch source.Type {
		case SourceTypeFile:
			err = m.loader.FromFile(source.Path)
		case SourceTypeJSON:
			err = m.loader.FromJSON([]byte(source.Content))
		case SourceTypeYAML:
			err = m.loader.FromYAML([]byte(source.Content))
		case SourceTypeEnv:
			err = m.loader.FromEnv(source.Prefix)
		case SourceTypeRemote:
			err = m.loader.FromRemote(source.URL, source.Headers)
		default:
			err = fmt.Errorf("不支持的配置源类型: %s", source.Type)
		}

		if err != nil {
			if source.Required {
				return m.wrapError(fmt.Sprintf("必需的配置源加载失败: %s", source.Path), err, "LoadFromMultiple")
			}
			lastError = err
			continue
		}
	}

	// 验证最终配置
	if err := m.loader.Validate(); err != nil {
		return m.wrapError("最终配置验证失败", err, "LoadFromMultiple")
	}

	// 更新当前配置
	m.currentConfig = m.loader.Get()

	// 如果有非致命错误，记录但不阻塞
	if lastError != nil {
		_ = m.wrapError("部分配置源加载失败", lastError, "LoadFromMultiple")
	}

	return nil
}

// GetConfig 获取当前配置
func (m *Manager) GetConfig() *Config {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.currentConfig == nil {
		return &Config{}
	}

	// 返回配置的副本
	return m.loader.Get()
}

// GetAppConfig 获取应用配置
func (m *Manager) GetAppConfig() *AppConfig {
	config := m.GetConfig()
	return &config.App
}

// GetNetworkConfig 获取网络配置
func (m *Manager) GetNetworkConfig() *NetworkConfig {
	config := m.GetConfig()
	return &config.Network
}

// GetDatabaseConfig 获取数据库配置
func (m *Manager) GetDatabaseConfig() *DatabaseConfig {
	config := m.GetConfig()
	return &config.Database
}

// GetSecurityConfig 获取安全配置
func (m *Manager) GetSecurityConfig() *SecurityConfig {
	config := m.GetConfig()
	return &config.Security
}

// GetObservabilityConfig 获取可观测性配置
func (m *Manager) GetObservabilityConfig() *ObservabilityConfig {
	config := m.GetConfig()
	return &config.Observability
}

// UpdateConfig 更新配置
func (m *Manager) UpdateConfig(updater func(*Config) error) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 获取当前配置副本
	config := m.loader.Get()

	// 应用更新
	if err := updater(config); err != nil {
		return m.wrapError("配置更新失败", err, "UpdateConfig")
	}

	// 验证更新后的配置
	tempLoader := NewLoader(m.ctx)
	if err := tempLoader.FromYAML([]byte("{}")); err != nil {
		return m.wrapError("创建临时加载器失败", err, "UpdateConfig")
	}

	// 这里应该有更好的验证方式，暂时简化
	if err := m.validateConfig(config); err != nil {
		return m.wrapError("更新后的配置验证失败", err, "UpdateConfig")
	}

	// 更新当前配置
	m.currentConfig = config
	if l, ok := m.loader.(*loader); ok {
		l.mu.Lock()
		l.config = config
		l.mu.Unlock()
	}

	// 触发重启信号（如果需要）
	select {
	case m.restartChan <- struct{}{}:
	default:
	}

	return nil
}

// Reload 重新加载配置
func (m *Manager) Reload() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if err := m.loader.Reload(); err != nil {
		return m.wrapError("重新加载配置失败", err, "Reload")
	}

	m.currentConfig = m.loader.Get()

	// 触发重启信号
	select {
	case m.restartChan <- struct{}{}:
	default:
	}

	return nil
}

// WatchConfig 监听配置变化
func (m *Manager) WatchConfig(ctx context.Context, callback func(*Config, error)) error {
	return m.loader.Watch(func(config *Config) {
		m.mu.Lock()
		m.currentConfig = config
		m.mu.Unlock()

		if callback != nil {
			callback(config, nil)
		}
	})
}

// RestartChan 返回重启信号通道
func (m *Manager) RestartChan() <-chan struct{} {
	return m.restartChan
}

// Export 导出配置
func (m *Manager) Export(format string) ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	data, err := m.loader.Export(format)
	if err != nil {
		return nil, m.wrapError("导出配置失败", err, "Export")
	}

	return data, nil
}

// Validate 验证当前配置
func (m *Manager) Validate() error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.loader.Validate()
}

// GetConfigSources 获取配置源信息
func (m *Manager) GetConfigSources() []Source {
	return m.loader.GetSources()
}

// CreateConfigContext 创建配置相关的上下文
func (m *Manager) CreateConfigContext(operation string) (context.Context, context.CancelFunc) {
	if m.contextMgr != nil {
		return m.contextMgr.ForBackground(operation)
	}
	return context.WithTimeout(m.ctx, 30*time.Second)
}

// wrapError 包装错误
func (m *Manager) wrapError(message string, err error, operation string) error {
	if m.errorFactory != nil {
		details := message
		if err != nil {
			details = fmt.Sprintf("%s: %v", message, err)
		}
		return m.errorFactory.Wrap(err, operation).WithDetails(details)
	}
	return fmt.Errorf("%s: %w", message, err)
}

// validateConfig 验证配置
func (m *Manager) validateConfig(config *Config) error {
	// 基本验证
	if config.App.Name == "" {
		return fmt.Errorf("应用名称不能为空")
	}

	// 端口验证
	if config.Network.Server.HTTPPort <= 0 || config.Network.Server.HTTPPort > 65535 {
		return fmt.Errorf("HTTP端口无效: %d", config.Network.Server.HTTPPort)
	}

	if config.Network.Server.GRPCPort <= 0 || config.Network.Server.GRPCPort > 65535 {
		return fmt.Errorf("gRPC端口无效: %d", config.Network.Server.GRPCPort)
	}

	// 数据库验证
	if config.Database.Enabled {
		if config.Database.Primary.Host == "" {
			return fmt.Errorf("数据库主机不能为空")
		}
		if config.Database.Primary.Database == "" {
			return fmt.Errorf("数据库名称不能为空")
		}
	}

	// 安全验证
	if config.Security.JWT.Enabled && config.Security.JWT.Secret == "" {
		return fmt.Errorf("启用JWT时密钥不能为空")
	}

	return nil
}

// Close 关闭管理器
func (m *Manager) Close() error {
	if m.cancel != nil {
		m.cancel()
	}

	if m.watchCancel != nil {
		m.watchCancel()
	}

	close(m.restartChan)

	return nil
}

// ConfigSource 配置源定义
type ConfigSource struct {
	Type     string            `json:"type"`     // file, json, yaml, env, remote
	Path     string            `json:"path"`     // 文件路径或URL
	Content  string            `json:"content"`  // 直接内容（适用于json/yaml类型）
	Prefix   string            `json:"prefix"`   // 环境变量前缀
	URL      string            `json:"url"`      // 远程URL
	Headers  map[string]string `json:"headers"`  // 请求头
	Required bool              `json:"required"` // 是否必需
}

// 配置源类型常量
const (
	SourceTypeFile   = "file"
	SourceTypeJSON   = "json"
	SourceTypeYAML   = "yaml"
	SourceTypeEnv    = "env"
	SourceTypeRemote = "remote"
)

// NewConfigSource 创建配置源
func NewConfigSource(sourceType, path string, required bool) *ConfigSource {
	return &ConfigSource{
		Type:     sourceType,
		Path:     path,
		Required: required,
	}
}

// NewRemoteConfigSource 创建远程配置源
func NewRemoteConfigSource(url string, headers map[string]string, required bool) *ConfigSource {
	return &ConfigSource{
		Type:     SourceTypeRemote,
		URL:      url,
		Headers:  headers,
		Required: required,
	}
}

// NewEnvConfigSource 创建环境变量配置源
func NewEnvConfigSource(prefix string, required bool) *ConfigSource {
	return &ConfigSource{
		Type:     SourceTypeEnv,
		Prefix:   prefix,
		Required: required,
	}
}

// NewJSONConfigSource 创建JSON配置源
func NewJSONConfigSource(content string, required bool) *ConfigSource {
	return &ConfigSource{
		Type:     SourceTypeJSON,
		Content:  content,
		Required: required,
	}
}

// NewYAMLConfigSource 创建YAML配置源
func NewYAMLConfigSource(content string, required bool) *ConfigSource {
	return &ConfigSource{
		Type:     SourceTypeYAML,
		Content:  content,
		Required: required,
	}
}
