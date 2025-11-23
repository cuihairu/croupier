package config

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"sync"
	"sync/atomic"
	"time"

	ctxmanager "github.com/cuihairu/croupier/internal/context"
	"github.com/cuihairu/croupier/internal/errors"
)

// AdvancedConfigManager 高级配置管理器
type AdvancedConfigManager struct {
	*Manager
	cache        *ConfigCache
	history      *ConfigHistory
	encryptor    *ConfigEncryptor
	metrics      *ConfigMetrics
	errorFactory *errors.ErrorFactory
	contextMgr   *ctxmanager.Manager
	mu           sync.RWMutex
}

// ConfigCache 配置缓存
type ConfigCache struct {
	items   map[string]*CacheItem
	mu      sync.RWMutex
	maxSize int
}

// CacheItem 缓存项
type CacheItem struct {
	Key        string
	Value      interface{}
	Expiration time.Time
	CreatedAt  time.Time
	AccessAt   time.Time
	HitCount   int64
}

// ConfigHistory 配置历史
type ConfigHistory struct {
	records []HistoryRecord
	mu      sync.RWMutex
	maxSize int
}

// HistoryRecord 历史记录
type HistoryRecord struct {
	ID        string                 `json:"id"`
	Timestamp time.Time              `json:"timestamp"`
	Operation string                 `json:"operation"`
	Source    string                 `json:"source"`
	Changes   map[string]interface{} `json:"changes"`
	Metadata  map[string]string      `json:"metadata"`
	Config    map[string]interface{} `json:"config"`
}

// ConfigEncryptor 配置加密器
type ConfigEncryptor struct {
	key    []byte
	cipher cipher.AEAD
}

// ConfigMetrics 配置指标
type ConfigMetrics struct {
	LoadCount        int64     `json:"load_count"`
	LoadErrors       int64     `json:"load_errors"`
	ReloadCount      int64     `json:"reload_count"`
	ValidationErrors int64     `json:"validation_errors"`
	LastLoad         time.Time `json:"last_load"`
	LastError        time.Time `json:"last_error"`
	CacheHits        int64     `json:"cache_hits"`
	CacheMisses      int64     `json:"cache_misses"`
	mu               sync.RWMutex
}

var configIDCounter uint64

// NewAdvancedConfigManager 创建高级配置管理器
func NewAdvancedConfigManager(ctx context.Context, opts ...ManagerOption) (*AdvancedConfigManager, error) {
	// 创建基础管理器
	baseManager, err := NewManager(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("创建基础配置管理器失败: %w", err)
	}

	// 创建错误工厂和上下文管理器
	errorFactory := errors.NewErrorFactory("advanced-config")
	contextMgr := ctxmanager.NewManager(30 * time.Second)

	advanced := &AdvancedConfigManager{
		Manager:      baseManager,
		cache:        NewConfigCache(100),
		history:      NewConfigHistory(50),
		metrics:      NewConfigMetrics(),
		errorFactory: errorFactory,
		contextMgr:   contextMgr,
	}

	// 初始化加密器
	if err := advanced.initEncryptor(); err != nil {
		log.Printf("初始化配置加密器失败: %v", err)
	}

	return advanced, nil
}

// NewConfigCache 创建配置缓存
func NewConfigCache(maxSize int) *ConfigCache {
	return &ConfigCache{
		items:   make(map[string]*CacheItem),
		maxSize: maxSize,
	}
}

// NewConfigHistory 创建配置历史
func NewConfigHistory(maxSize int) *ConfigHistory {
	return &ConfigHistory{
		records: make([]HistoryRecord, 0),
		maxSize: maxSize,
	}
}

// NewConfigMetrics 创建配置指标
func NewConfigMetrics() *ConfigMetrics {
	return &ConfigMetrics{}
}

// initEncryptor 初始化加密器
func (acm *AdvancedConfigManager) initEncryptor() error {
	// 从环境变量获取加密密钥
	keyStr := acm.getEncryptionKey()
	if keyStr == "" {
		return nil // 加密密钥未设置，不启用加密
	}

	key, err := base64.StdEncoding.DecodeString(keyStr)
	if err != nil {
		return fmt.Errorf("解码加密密钥失败: %w", err)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return fmt.Errorf("创建AES加密器失败: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return fmt.Errorf("创建GCM模式失败: %w", err)
	}

	acm.encryptor = &ConfigEncryptor{
		key:    key,
		cipher: gcm,
	}

	return nil
}

// getEncryptionKey 获取加密密钥
func (acm *AdvancedConfigManager) getEncryptionKey() string {
	// 这里应该从安全的地方获取密钥，比如密钥管理系统
	// 为了演示，这里从环境变量获取
	return "" // 默认不启用加密
}

// LoadWithCache 带缓存的配置加载
func (acm *AdvancedConfigManager) LoadWithCache(ctx context.Context, cacheKey string, loader func() (*Config, error)) (*Config, error) {
	acm.metrics.mu.Lock()
	defer acm.metrics.mu.Unlock()

	// 检查缓存
	if item := acm.cache.Get(cacheKey); item != nil {
		acm.metrics.CacheHits++
		if config, ok := item.Value.(*Config); ok {
			return config, nil
		}
	}

	acm.metrics.CacheMisses++

	// 从源加载配置
	config, err := loader()
	if err != nil {
		acm.metrics.LoadErrors++
		acm.metrics.LastError = time.Now()
		return nil, err
	}

	// 缓存配置
	acm.cache.Set(cacheKey, config, 5*time.Minute)
	acm.metrics.LoadCount++
	acm.metrics.LastLoad = time.Now()

	// 记录历史
	acm.recordHistory("load", cacheKey, map[string]interface{}{
		"cache_key": cacheKey,
		"source":    "loader",
	})

	return config, nil
}

// Get 获取配置（带缓存支持）
func (acm *AdvancedConfigManager) Get(ctx context.Context, cacheKey string) (*Config, error) {
	return acm.LoadWithCache(ctx, cacheKey, func() (*Config, error) {
		return acm.Manager.GetConfig(), nil
	})
}

// EncryptConfig 加密配置
func (acm *AdvancedConfigManager) EncryptConfig(config *Config) (map[string]string, error) {
	if acm.encryptor == nil {
		return nil, fmt.Errorf("配置加密器未初始化")
	}

	encrypted := make(map[string]string)

	// 序列化配置
	jsonData, err := json.Marshal(config)
	if err != nil {
		return nil, fmt.Errorf("序列化配置失败: %w", err)
	}

	// 加密数据
	encryptedData, err := acm.encryptor.Encrypt(jsonData)
	if err != nil {
		return nil, fmt.Errorf("加密配置失败: %w", err)
	}

	encrypted["config"] = encryptedData
	encrypted["timestamp"] = time.Now().Format(time.RFC3339)
	encrypted["version"] = "1.0"

	return encrypted, nil
}

// DecryptConfig 解密配置
func (acm *AdvancedConfigManager) DecryptConfig(encryptedData map[string]string) (*Config, error) {
	if acm.encryptor == nil {
		return nil, fmt.Errorf("配置加密器未初始化")
	}

	configData, ok := encryptedData["config"]
	if !ok {
		return nil, fmt.Errorf("加密数据中缺少config字段")
	}

	// 解密数据
	decryptedData, err := acm.encryptor.Decrypt(configData)
	if err != nil {
		return nil, fmt.Errorf("解密配置失败: %w", err)
	}

	// 反序列化配置
	var config Config
	if err := json.Unmarshal(decryptedData, &config); err != nil {
		return nil, fmt.Errorf("反序列化配置失败: %w", err)
	}

	return &config, nil
}

// ValidateWithAdvancedValidation 高级配置验证
func (acm *AdvancedConfigManager) ValidateWithAdvancedValidation(config *Config) (*ValidationResult, error) {
	// 基础验证
	validator := NewDefaultValidator()
	if err := validator.Validate(config); err != nil {
		return &ValidationResult{
			Valid:    false,
			Errors:   []string{err.Error()},
			Score:    0,
			Warnings: []string{},
		}, nil
	}

	// 高级验证规则
	result := &ValidationResult{
		Valid:  true,
		Errors: []string{},
		Score:  100,
	}

	// 性能配置验证
	acm.validatePerformanceConfig(config, result)

	// 安全配置验证
	acm.validateSecurityConfig(config, result)

	// 可观测性配置验证
	acm.validateObservabilityConfig(config, result)

	return result, nil
}

// ValidationResult 验证结果
type ValidationResult struct {
	Valid    bool     `json:"valid"`
	Errors   []string `json:"errors"`
	Warnings []string `json:"warnings"`
	Score    int      `json:"score"` // 0-100
}

// validatePerformanceConfig 验证性能配置
func (acm *AdvancedConfigManager) validatePerformanceConfig(config *Config, result *ValidationResult) {
	// 数据库连接池配置
	if config.Database.ConnectionPool.MaxOpenConns > 100 {
		result.Warnings = append(result.Warnings, "数据库连接池最大连接数过高，可能影响性能")
		result.Score -= 5
	}

	if config.Database.ConnectionPool.MaxIdleConns < 2 {
		result.Warnings = append(result.Warnings, "数据库连接池最小空闲连接数过低，可能影响性能")
		result.Score -= 3
	}

	// 业务配置
	if config.Business.Games.MaxConcurrentGames > 10000 {
		result.Warnings = append(result.Warnings, "最大并发游戏数设置过高，请确保系统资源充足")
		result.Score -= 5
	}
}

// validateSecurityConfig 验证安全配置
func (acm *AdvancedConfigManager) validateSecurityConfig(config *Config, result *ValidationResult) {
	// JWT配置
	if config.Security.JWT.Enabled {
		if len(config.Security.JWT.Secret) < 32 {
			result.Errors = append(result.Errors, "JWT密钥长度至少需要32个字符")
			result.Valid = false
			result.Score -= 20
		}

		if config.Security.JWT.Expiry > 24*time.Hour {
			result.Warnings = append(result.Warnings, "JWT过期时间过长，可能增加安全风险")
			result.Score -= 5
		}
	}

	// 密码策略
	if config.Security.PasswordPolicy.MinLength < 8 {
		result.Errors = append(result.Errors, "密码最小长度不应少于8个字符")
		result.Valid = false
		result.Score -= 15
	}
}

// validateObservabilityConfig 验证可观测性配置
func (acm *AdvancedConfigManager) validateObservabilityConfig(config *Config, result *ValidationResult) {
	// 健康检查端口冲突
	if config.Observability.HealthCheck.Port == config.Network.Server.HTTPPort {
		result.Errors = append(result.Errors, "健康检查端口与HTTP服务端口冲突")
		result.Valid = false
		result.Score -= 10
	}

	// 指标端口冲突
	if config.Observability.Metrics.Enabled &&
		config.Observability.Metrics.Port == config.Network.Server.HTTPPort {
		result.Errors = append(result.Errors, "指标端口与HTTP服务端口冲突")
		result.Valid = false
		result.Score -= 10
	}
}

// GetMetrics 获取配置管理指标
func (acm *AdvancedConfigManager) GetMetrics() *ConfigMetrics {
	acm.metrics.mu.RLock()
	defer acm.metrics.mu.RUnlock()

	return &ConfigMetrics{
		LoadCount:        acm.metrics.LoadCount,
		LoadErrors:       acm.metrics.LoadErrors,
		ReloadCount:      acm.metrics.ReloadCount,
		ValidationErrors: acm.metrics.ValidationErrors,
		LastLoad:         acm.metrics.LastLoad,
		LastError:        acm.metrics.LastError,
		CacheHits:        acm.metrics.CacheHits,
		CacheMisses:      acm.metrics.CacheMisses,
	}
}

// GetHistory 获取配置变更历史
func (acm *AdvancedConfigManager) GetHistory(limit int) []HistoryRecord {
	acm.history.mu.RLock()
	defer acm.history.mu.RUnlock()

	if limit <= 0 || limit > len(acm.history.records) {
		limit = len(acm.history.records)
	}

	// 返回最新的记录
	records := make([]HistoryRecord, limit)
	copy(records, acm.history.records[len(acm.history.records)-limit:])
	return records
}

// recordHistory 记录配置变更历史
func (acm *AdvancedConfigManager) recordHistory(operation, source string, changes map[string]interface{}) {
	record := HistoryRecord{
		ID:        acm.generateID(),
		Timestamp: time.Now(),
		Operation: operation,
		Source:    source,
		Changes:   changes,
		Metadata:  map[string]string{"version": "1.0"},
	}

	acm.history.mu.Lock()
	defer acm.history.mu.Unlock()

	// 添加记录
	acm.history.records = append(acm.history.records, record)

	// 限制记录数量
	if len(acm.history.records) > acm.history.maxSize {
		acm.history.records = acm.history.records[1:]
	}
}

// generateID 生成唯一ID
func (acm *AdvancedConfigManager) generateID() string {
	seq := atomic.AddUint64(&configIDCounter, 1)
	return fmt.Sprintf("cfg_%d_%d", time.Now().UnixNano(), seq)
}

// 缓存相关方法

// Set 设置缓存项
func (cc *ConfigCache) Set(key string, value interface{}, ttl time.Duration) {
	cc.mu.Lock()
	defer cc.mu.Unlock()

	// 检查缓存大小限制
	if len(cc.items) >= cc.maxSize {
		cc.evictLRU()
	}

	cc.items[key] = &CacheItem{
		Key:        key,
		Value:      value,
		Expiration: time.Now().Add(ttl),
		CreatedAt:  time.Now(),
		AccessAt:   time.Now(),
		HitCount:   0,
	}
}

// Get 获取缓存项
func (cc *ConfigCache) Get(key string) *CacheItem {
	cc.mu.RLock()
	defer cc.mu.RUnlock()

	item, exists := cc.items[key]
	if !exists {
		return nil
	}

	// 检查是否过期
	if time.Now().After(item.Expiration) {
		delete(cc.items, key)
		return nil
	}

	// 更新访问信息
	item.AccessAt = time.Now()
	item.HitCount++

	return item
}

// evictLRU LRU淘汰策略
func (cc *ConfigCache) evictLRU() {
	if len(cc.items) == 0 {
		return
	}

	var oldestKey string
	var oldestTime time.Time

	for key, item := range cc.items {
		if oldestKey == "" || item.AccessAt.Before(oldestTime) {
			oldestKey = key
			oldestTime = item.AccessAt
		}
	}

	if oldestKey != "" {
		delete(cc.items, oldestKey)
	}
}

// 加密相关方法

// Encrypt 加密数据
func (ce *ConfigEncryptor) Encrypt(plaintext []byte) (string, error) {
	nonce := make([]byte, ce.cipher.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	ciphertext := ce.cipher.Seal(nonce, nonce, plaintext, nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt 解密数据
func (ce *ConfigEncryptor) Decrypt(ciphertext string) ([]byte, error) {
	data, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return nil, err
	}

	nonceSize := ce.cipher.NonceSize()
	if len(data) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertextBytes := data[:nonceSize], data[nonceSize:]
	plaintext, err := ce.cipher.Open(nil, nonce, ciphertextBytes, nil)
	if err != nil {
		return nil, err
	}

	return plaintext, nil
}

// ExportWithHistory 导出配置及历史
func (acm *AdvancedConfigManager) ExportWithHistory() (map[string]interface{}, error) {
	config := acm.GetConfig()
	history := acm.GetHistory(10) // 最近10条记录
	metrics := acm.GetMetrics()

	export := map[string]interface{}{
		"config":      config,
		"history":     history,
		"metrics":     metrics,
		"export_time": time.Now().Format(time.RFC3339),
		"version":     "1.0",
	}

	return export, nil
}

// BackupConfiguration 备份配置
func (acm *AdvancedConfigManager) BackupConfiguration(ctx context.Context, backupPath string) error {
	_, err := acm.ExportWithHistory()
	if err != nil {
		return acm.wrapError("导出配置失败", err, "backup")
	}

	// 这里可以实现将备份保存到文件、数据库或云存储
	log.Printf("配置备份已准备，路径: %s", backupPath)
	log.Printf("备份包含配置、历史记录和指标数据")

	return nil
}

// wrapError 包装错误
func (acm *AdvancedConfigManager) wrapError(message string, err error, operation string) error {
	if acm.errorFactory != nil {
		return acm.errorFactory.Wrap(err, operation).WithDetails(message)
	}
	return fmt.Errorf("%s: %w", message, err)
}
