package config

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

// Loader 配置加载器接口
type Loader interface {
	FromFile(path string) error
	FromJSON(data []byte) error
	FromYAML(data []byte) error
	FromEnv(prefix string) error
	FromRemote(url string, headers map[string]string) error
	Validate() error
	Get() *Config
	Reload() error
	Watch(callback func(*Config)) error
	GetSources() []Source
	Export(format string) ([]byte, error)
}

// Option 配置加载选项
type Option func(*loader)

// WithDefaultTimeout 设置默认超时时间
func WithDefaultTimeout(timeout time.Duration) Option {
	return func(l *loader) {
		l.httpTimeout = timeout
	}
}

// WithRetryCount 设置重试次数
func WithRetryCount(count int) Option {
	return func(l *loader) {
		l.retryCount = count
	}
}

// WithRetryDelay 设置重试延迟
func WithRetryDelay(delay time.Duration) Option {
	return func(l *loader) {
		l.retryDelay = delay
	}
}

// WithSecretsProvider 设置密钥提供者
func WithSecretsProvider(provider SecretsProvider) Option {
	return func(l *loader) {
		l.secretsProvider = provider
	}
}

// WithValidator 设置验证器
func WithValidator(validator Validator) Option {
	return func(l *loader) {
		l.validator = validator
	}
}

// loader 配置加载器实现
type loader struct {
	config          *Config
	mu              sync.RWMutex
	httpTimeout     time.Duration
	retryCount      int
	retryDelay      time.Duration
	secretsProvider SecretsProvider
	validator       Validator
	sources         []Source
	watchers        []func(*Config)
	httpClient      *http.Client
}

// SecretsProvider 密钥提供者接口
type SecretsProvider interface {
	GetSecret(ctx context.Context, key string) (string, error)
	SetSecret(ctx context.Context, key, value string) error
	DeleteSecret(ctx context.Context, key string) error
}

// Validator 配置验证器接口
type Validator interface {
	Validate(config *Config) error
}

// Source 配置源信息
type Source struct {
	Type    string            `json:"type"` // file, env, remote, memory
	Path    string            `json:"path"` // 路径或URL
	Hash    string            `json:"hash"`
	Loaded  time.Time         `json:"loaded"`
	Prefix  string            `json:"prefix,omitempty"`
	URL     string            `json:"url,omitempty"`
	Headers map[string]string `json:"-"`
	Content []byte            `json:"-"`
}

// NewLoader 创建配置加载器
func NewLoader(ctx context.Context, options ...Option) Loader {
	l := &loader{
		config:      &Config{},
		httpTimeout: 30 * time.Second,
		retryCount:  3,
		retryDelay:  1 * time.Second,
		httpClient:  &http.Client{Timeout: 30 * time.Second},
		sources:     make([]Source, 0),
		watchers:    make([]func(*Config), 0),
	}

	// 应用选项
	for _, opt := range options {
		opt(l)
	}

	return l
}

func (l *loader) decodeConfig(data []byte, format string) (*Config, error) {
	tempConfig := &Config{}

	switch format {
	case "json":
		if err := json.Unmarshal(data, tempConfig); err != nil {
			return nil, fmt.Errorf("解析JSON配置失败: %w", err)
		}
	case "yaml":
		if err := yaml.Unmarshal(data, tempConfig); err != nil {
			return nil, fmt.Errorf("解析YAML配置失败: %w", err)
		}
	default:
		return nil, fmt.Errorf("不支持的配置格式: %s", format)
	}

	return tempConfig, nil
}

// FromFile 从文件加载配置
func (l *loader) FromFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("读取配置文件失败 %s: %w", path, err)
	}

	// 根据文件扩展名选择解析方式
	ext := strings.ToLower(filepath.Ext(path))
	var format string
	switch ext {
	case ".json":
		format = "json"
	case ".yaml", ".yml":
		format = "yaml"
	default:
		return fmt.Errorf("不支持的配置文件格式: %s", ext)
	}

	tempConfig, err := l.decodeConfig(data, format)
	if err != nil {
		return err
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	l.config = l.mergeConfigs(l.config, tempConfig)
	l.sources = append(l.sources, Source{
		Type:   SourceTypeFile,
		Path:   path,
		Hash:   l.calculateHash(data),
		Loaded: time.Now(),
	})

	return nil
}

// FromJSON 从JSON数据加载配置
func (l *loader) FromJSON(data []byte) error {
	tempConfig, err := l.decodeConfig(data, "json")
	if err != nil {
		return err
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	l.config = l.mergeConfigs(l.config, tempConfig)
	l.sources = append(l.sources, Source{
		Type:    SourceTypeJSON,
		Hash:    l.calculateHash(data),
		Loaded:  time.Now(),
		Content: append([]byte(nil), data...),
	})

	return nil
}

// FromYAML 从YAML数据加载配置
func (l *loader) FromYAML(data []byte) error {
	tempConfig, err := l.decodeConfig(data, "yaml")
	if err != nil {
		return err
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	l.config = l.mergeConfigs(l.config, tempConfig)
	l.sources = append(l.sources, Source{
		Type:    SourceTypeYAML,
		Hash:    l.calculateHash(data),
		Loaded:  time.Now(),
		Content: append([]byte(nil), data...),
	})

	return nil
}

// FromEnv 从环境变量加载配置
func (l *loader) FromEnv(prefix string) error {
	// 创建环境变量映射
	envMap := make(map[string]interface{})

	for _, env := range os.Environ() {
		if strings.HasPrefix(env, prefix) {
			parts := strings.SplitN(env, "=", 2)
			if len(parts) == 2 {
				key := strings.TrimPrefix(parts[0], prefix)
				key = strings.ToLower(strings.ReplaceAll(key, "_", "."))

				// 尝试转换值的类型
				value := l.parseEnvValue(parts[1])
				l.setNestedValue(envMap, key, value)
			}
		}
	}

	// 将环境变量映射转换为配置
	configBytes, err := json.Marshal(envMap)
	if err != nil {
		return fmt.Errorf("转换环境变量配置失败: %w", err)
	}

	tempConfig := &Config{}
	if err := json.Unmarshal(configBytes, tempConfig); err != nil {
		return fmt.Errorf("解析环境变量配置失败: %w", err)
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	// 深度合并配置
	l.config = l.mergeConfigs(l.config, tempConfig)

	// 记录配置源
	l.sources = append(l.sources, Source{
		Type:   SourceTypeEnv,
		Hash:   l.calculateHash([]byte(prefix)),
		Loaded: time.Now(),
		Prefix: prefix,
	})

	return nil
}

// FromRemote 从远程URL加载配置
func (l *loader) FromRemote(url string, headers map[string]string) error {
	ctx, cancel := context.WithTimeout(context.Background(), l.httpTimeout)
	defer cancel()

	// 重试机制
	var lastErr error
	for attempt := 0; attempt < l.retryCount; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return fmt.Errorf("创建远程配置请求失败: %w", err)
		}

		for key, value := range headers {
			req.Header.Set(key, value)
		}

		if attempt > 0 {
			time.Sleep(l.retryDelay * time.Duration(attempt))
		}

		resp, err := l.httpClient.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		body, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()

		if readErr != nil {
			lastErr = fmt.Errorf("读取远程配置响应失败: %w", readErr)
			continue
		}

		if resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("远程配置请求失败，状态码: %d", resp.StatusCode)
			continue
		}

		format := "yaml"
		if strings.Contains(strings.ToLower(resp.Header.Get("Content-Type")), "json") {
			format = "json"
		}

		tempConfig, err := l.decodeConfig(body, format)
		if err != nil {
			return err
		}

		l.mu.Lock()
		l.config = l.mergeConfigs(l.config, tempConfig)
		l.sources = append(l.sources, Source{
			Type:    SourceTypeRemote,
			Path:    url,
			URL:     url,
			Hash:    l.calculateHash(body),
			Loaded:  time.Now(),
			Headers: cloneHeaders(headers),
		})
		l.mu.Unlock()

		return nil
	}

	return fmt.Errorf("加载远程配置失败，已达最大重试次数: %w", lastErr)
}

// Validate 验证配置
func (l *loader) Validate() error {
	l.mu.RLock()
	defer l.mu.RUnlock()

	if l.validator != nil {
		return l.validator.Validate(l.config)
	}

	// 默认验证逻辑
	return l.validateDefault()
}

// Get 获取配置
func (l *loader) Get() *Config {
	l.mu.RLock()
	defer l.mu.RUnlock()

	// 返回配置的深拷贝以避免并发修改
	return l.cloneConfig(l.config)
}

// Reload 重新加载配置
func (l *loader) Reload() error {
	l.mu.RLock()
	if len(l.sources) == 0 {
		l.mu.RUnlock()
		return fmt.Errorf("没有可重新加载的配置源")
	}
	sources := append([]Source(nil), l.sources...)
	l.mu.RUnlock()

	reloadLoader := l.newReloadLoader()

	for _, source := range sources {
		var err error

		switch source.Type {
		case SourceTypeFile:
			err = reloadLoader.FromFile(source.Path)
		case SourceTypeJSON:
			err = reloadLoader.FromJSON(source.Content)
		case SourceTypeYAML:
			err = reloadLoader.FromYAML(source.Content)
		case SourceTypeEnv:
			err = reloadLoader.FromEnv(source.Prefix)
		case SourceTypeRemote:
			err = reloadLoader.FromRemote(source.URL, source.Headers)
		default:
			err = fmt.Errorf("不支持的配置源类型: %s", source.Type)
		}

		if err != nil {
			return fmt.Errorf("重新加载配置源失败 %s: %w", source.Type, err)
		}
	}

	if err := reloadLoader.Validate(); err != nil {
		return fmt.Errorf("重新加载的配置验证失败: %w", err)
	}

	l.mu.Lock()
	l.config = reloadLoader.config
	l.sources = reloadLoader.sources
	l.mu.Unlock()

	l.notifyWatchers(reloadLoader.cloneConfig(reloadLoader.config))

	return nil
}

func (l *loader) newReloadLoader() *loader {
	return &loader{
		config:          &Config{},
		httpTimeout:     l.httpTimeout,
		retryCount:      l.retryCount,
		retryDelay:      l.retryDelay,
		secretsProvider: l.secretsProvider,
		validator:       l.validator,
		httpClient:      l.httpClient,
		sources:         make([]Source, 0),
	}
}

func (l *loader) notifyWatchers(config *Config) {
	l.mu.RLock()
	watchers := append([]func(*Config){}, l.watchers...)
	l.mu.RUnlock()

	for _, watcher := range watchers {
		watcher(config)
	}
}

// Watch 监听配置变化
func (l *loader) Watch(callback func(*Config)) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.watchers = append(l.watchers, callback)
	return nil
}

// mergeConfigs 深度合并配置
func (l *loader) mergeConfigs(base, override *Config) *Config {
	if base == nil {
		return l.cloneConfig(override)
	}
	if override == nil {
		return l.cloneConfig(base)
	}

	result := l.cloneConfig(base)

	// 使用反射进行深度合并
	baseVal := reflect.ValueOf(result).Elem()
	overrideVal := reflect.ValueOf(override).Elem()

	l.mergeValue(baseVal, overrideVal)

	return result
}

// mergeValue 递归合并结构体字段
func (l *loader) mergeValue(base, override reflect.Value) {
	if !base.IsValid() || !override.IsValid() {
		return
	}

	switch base.Kind() {
	case reflect.Struct:
		for i := 0; i < base.NumField(); i++ {
			baseField := base.Field(i)
			overrideField := override.Field(i)

			if overrideField.IsValid() && !overrideField.IsZero() {
				if baseField.CanSet() {
					switch baseField.Kind() {
					case reflect.Struct:
						if baseField.Type() != overrideField.Type() {
							baseField.Set(overrideField)
						} else {
							l.mergeValue(baseField, overrideField)
						}
					case reflect.Map:
						l.mergeMap(baseField, overrideField)
					case reflect.Slice:
						if !overrideField.IsNil() {
							baseField.Set(overrideField)
						}
					default:
						baseField.Set(overrideField)
					}
				}
			}
		}
	case reflect.Map:
		l.mergeMap(base, override)
	case reflect.Slice:
		if !override.IsNil() {
			base.Set(override)
		}
	default:
		if !override.IsZero() {
			base.Set(override)
		}
	}
}

// mergeMap 合并map类型
func (l *loader) mergeMap(base, override reflect.Value) {
	if base.Kind() != reflect.Map || override.Kind() != reflect.Map {
		return
	}

	if base.IsNil() {
		base.Set(reflect.MakeMap(base.Type()))
	}

	iter := override.MapRange()
	for iter.Next() {
		key := iter.Key()
		value := iter.Value()
		base.SetMapIndex(key, value)
	}
}

// parseEnvValue 解析环境变量值的类型
func (l *loader) parseEnvValue(value string) interface{} {
	// 尝试解析为布尔值
	if strings.ToLower(value) == "true" {
		return true
	}
	if strings.ToLower(value) == "false" {
		return false
	}

	// 尝试解析为整数
	if intVal, err := strconv.Atoi(value); err == nil {
		return intVal
	}

	// 尝试解析为浮点数
	if floatVal, err := strconv.ParseFloat(value, 64); err == nil {
		return floatVal
	}

	// 尝试解析为JSON数组或对象
	var jsonVal interface{}
	if err := json.Unmarshal([]byte(value), &jsonVal); err == nil {
		return jsonVal
	}

	// 默认作为字符串
	return value
}

// setNestedValue 设置嵌套结构值
func (l *loader) setNestedValue(m map[string]interface{}, key string, value interface{}) {
	parts := strings.Split(key, ".")
	current := m

	for i, part := range parts {
		if i == len(parts)-1 {
			// 最后一级，设置值
			current[part] = value
			return
		}

		// 确保中间级别是map
		if _, ok := current[part]; !ok {
			current[part] = make(map[string]interface{})
		}

		if next, ok := current[part].(map[string]interface{}); ok {
			current = next
		} else {
			// 路径冲突，创建新的子map
			current[part] = make(map[string]interface{})
			current = current[part].(map[string]interface{})
		}
	}
}

func cloneHeaders(headers map[string]string) map[string]string {
	if len(headers) == 0 {
		return nil
	}

	result := make(map[string]string, len(headers))
	for k, v := range headers {
		result[k] = v
	}

	return result
}

// cloneConfig 深拷贝配置
func (l *loader) cloneConfig(config *Config) *Config {
	if config == nil {
		return nil
	}

	data, err := json.Marshal(config)
	if err != nil {
		return &Config{}
	}

	clone := &Config{}
	json.Unmarshal(data, clone)
	return clone
}

// calculateHash 计算配置哈希值
func (l *loader) calculateHash(data []byte) string {
	// 简单的哈希实现，实际应该使用crypto/sha256
	return fmt.Sprintf("%x", len(data))
}

// validateDefault 默认配置验证
func (l *loader) validateDefault() error {
	config := l.config

	// 验证应用配置
	if config.App.Name == "" {
		return fmt.Errorf("应用名称不能为空")
	}
	if config.App.Version == "" {
		return fmt.Errorf("应用版本不能为空")
	}

	// 验证网络配置
	if config.Network.Server.HTTPPort <= 0 || config.Network.Server.HTTPPort > 65535 {
		return fmt.Errorf("HTTP端口无效: %d", config.Network.Server.HTTPPort)
	}
	if config.Network.Server.GRPCPort <= 0 || config.Network.Server.GRPCPort > 65535 {
		return fmt.Errorf("gRPC端口无效: %d", config.Network.Server.GRPCPort)
	}

	// 验证数据库配置
	if config.Database.Enabled {
		if config.Database.Primary.Host == "" {
			return fmt.Errorf("数据库主机不能为空")
		}
		if config.Database.Primary.Port <= 0 || config.Database.Primary.Port > 65535 {
			return fmt.Errorf("数据库端口无效: %d", config.Database.Primary.Port)
		}
		if config.Database.Primary.Database == "" {
			return fmt.Errorf("数据库名称不能为空")
		}
	}

	// 验证安全配置
	if config.Security.JWT.Enabled {
		if config.Security.JWT.Secret == "" {
			return fmt.Errorf("JWT密钥不能为空")
		}
		if len(config.Security.JWT.Secret) < 32 {
			return fmt.Errorf("JWT密钥长度不能少于32个字符")
		}
	}

	return nil
}

// GetSources 获取配置源信息
func (l *loader) GetSources() []Source {
	l.mu.RLock()
	defer l.mu.RUnlock()

	result := make([]Source, len(l.sources))
	for i, source := range l.sources {
		result[i] = source
		if source.Content != nil {
			result[i].Content = append([]byte(nil), source.Content...)
		}
		if len(source.Headers) > 0 {
			result[i].Headers = cloneHeaders(source.Headers)
		}
	}

	return result
}

// Export 导出配置到指定格式
func (l *loader) Export(format string) ([]byte, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	switch strings.ToLower(format) {
	case "json":
		return json.MarshalIndent(l.config, "", "  ")
	case "yaml":
		return yaml.Marshal(l.config)
	default:
		return nil, fmt.Errorf("不支持的导出格式: %s", format)
	}
}
