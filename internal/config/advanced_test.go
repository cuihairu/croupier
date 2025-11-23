package config

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAdvancedConfigManager_NewAdvancedConfigManager(t *testing.T) {
	ctx := context.Background()

	// 测试创建高级配置管理器
	advancedMgr, err := NewAdvancedConfigManager(ctx)
	require.NoError(t, err)
	require.NotNil(t, advancedMgr)
	require.NotNil(t, advancedMgr.cache)
	require.NotNil(t, advancedMgr.history)
	require.NotNil(t, advancedMgr.metrics)
	require.NotNil(t, advancedMgr.Manager)

	defer advancedMgr.Close()
}

func TestConfigCache_SetAndGet(t *testing.T) {
	cache := NewConfigCache(10)
	require.NotNil(t, cache)

	// 测试设置和获取
	testValue := "test-value"
	cache.Set("test-key", testValue, 1*time.Minute)

	item := cache.Get("test-key")
	require.NotNil(t, item)
	assert.Equal(t, "test-key", item.Key)
	assert.Equal(t, testValue, item.Value)
	assert.Equal(t, int64(1), item.HitCount)
}

func TestConfigCache_Expiration(t *testing.T) {
	cache := NewConfigCache(10)

	// 设置过期时间很短的缓存项
	cache.Set("expire-key", "expire-value", 10*time.Millisecond)

	// 等待过期
	time.Sleep(50 * time.Millisecond)

	// 检查是否已过期
	item := cache.Get("expire-key")
	assert.Nil(t, item)
}

func TestConfigCache_LRUEviction(t *testing.T) {
	cache := NewConfigCache(2) // 最大容量为2

	// 添加3个项，应该触发LRU淘汰
	cache.Set("key1", "value1", 1*time.Hour)
	cache.Set("key2", "value2", 1*time.Hour)
	cache.Set("key3", "value3", 1*time.Hour) // 这应该淘汰key1

	// key1应该被淘汰
	item1 := cache.Get("key1")
	assert.Nil(t, item1)

	// key2和key3应该仍然存在
	item2 := cache.Get("key2")
	assert.NotNil(t, item2)
	assert.Equal(t, "value2", item2.Value)

	item3 := cache.Get("key3")
	assert.NotNil(t, item3)
	assert.Equal(t, "value3", item3.Value)
}

func TestConfigHistory_RecordHistory(t *testing.T) {
	history := NewConfigHistory(5) // 最大容量为5

	// 添加一些历史记录
	history.mu.Lock()
	history.records = append(history.records, HistoryRecord{
		ID:        "1",
		Timestamp: time.Now().Add(-2 * time.Hour),
		Operation: "load",
		Source:    "file",
	})
	history.mu.Unlock()

	// 通过recordHistory添加新记录
	advancedMgr := &AdvancedConfigManager{
		history: history,
	}

	advancedMgr.recordHistory("update", "test-source", map[string]interface{}{
		"field": "value",
	})

	records := advancedMgr.GetHistory(10)
	assert.True(t, len(records) >= 1)

	// 检查最新记录
	latestRecord := records[len(records)-1]
	assert.Equal(t, "update", latestRecord.Operation)
	assert.Equal(t, "test-source", latestRecord.Source)
	assert.Contains(t, latestRecord.Changes, "field")
}

func TestConfigMetrics_Updates(t *testing.T) {
	metrics := NewConfigMetrics()

	// 测试指标更新
	metrics.mu.Lock()
	metrics.LoadCount = 10
	metrics.LoadErrors = 2
	metrics.CacheHits = 50
	metrics.CacheMisses = 25
	metrics.LastLoad = time.Now()
	metrics.mu.Unlock()

	// 获取指标副本
	currentMetrics := &ConfigMetrics{
		LoadCount:   metrics.LoadCount,
		LoadErrors:  metrics.LoadErrors,
		CacheHits:   metrics.CacheHits,
		CacheMisses: metrics.CacheMisses,
		LastLoad:    metrics.LastLoad,
	}

	assert.Equal(t, int64(10), currentMetrics.LoadCount)
	assert.Equal(t, int64(2), currentMetrics.LoadErrors)
	assert.Equal(t, int64(50), currentMetrics.CacheHits)
	assert.Equal(t, int64(25), currentMetrics.CacheMisses)
	assert.True(t, currentMetrics.LastLoad.After(time.Time{}))
}

func TestAdvancedConfigManager_LoadWithCache(t *testing.T) {
	ctx := context.Background()
	advancedMgr, err := NewAdvancedConfigManager(ctx)
	require.NoError(t, err)
	defer advancedMgr.Close()

	// 第一次加载（应该从源加载）
	loadCount := 0
	config, err := advancedMgr.LoadWithCache(ctx, "test-cache-key", func() (*Config, error) {
		loadCount++
		return &Config{
			App: AppConfig{
				Name:    "test-app",
				Version: "1.0.0",
				Env:     "test",
			},
		}, nil
	})
	require.NoError(t, err)
	require.NotNil(t, config)
	assert.Equal(t, 1, loadCount)

	// 第二次加载（应该从缓存加载）
	config2, err := advancedMgr.LoadWithCache(ctx, "test-cache-key", func() (*Config, error) {
		loadCount++
		return &Config{}, nil
	})
	require.NoError(t, err)
	require.NotNil(t, config2)
	assert.Equal(t, 1, loadCount) // 加载次数不应该增加
	assert.Equal(t, config.App.Name, config2.App.Name)

	// 检查缓存指标
	metrics := advancedMgr.GetMetrics()
	assert.Greater(t, metrics.CacheHits, int64(0))
}

func TestAdvancedConfigManager_ValidateWithAdvancedValidation(t *testing.T) {
	ctx := context.Background()
	advancedMgr, err := NewAdvancedConfigManager(ctx)
	require.NoError(t, err)
	defer advancedMgr.Close()

	// 测试有效配置
	validConfig := &Config{
		App: AppConfig{
			Name:    "valid-app",
			Version: "1.0.0",
			Env:     "production",
		},
		Network: NetworkConfig{
			Server: ServerConfig{
				HTTPPort: 8080,
				GRPCPort: 9090,
			},
		},
		Security: SecurityConfig{
			JWT: JWTConfig{
				Enabled:       true,
				Secret:        "this-is-a-very-long-jwt-secret-key-that-meets-requirements",
				Expiry:        1 * time.Hour,
				RefreshExpiry: 24 * time.Hour,
			},
			PasswordPolicy: PasswordPolicyConfig{
				MinLength:        12,
				RequireUppercase: true,
				RequireLowercase: true,
				RequireNumbers:   true,
				RequireSymbols:   true,
			},
		},
		Observability: ObservabilityConfig{
			HealthCheck: HealthCheckConfig{
				Enabled: true,
				Port:    8081, // 不同的端口
			},
			Metrics: MetricsConfig{
				Enabled: true,
				Port:    9090, // 不同的端口
				Path:    "/metrics",
			},
		},
		Database: DatabaseConfig{
			Enabled: true,
			Primary: DatabaseInstance{
				Host:     "localhost",
				Port:     5432,
				Database: "testdb",
				Username: "testuser",
				Password: "testpass",
				SSLMode:  "disable",
			},
			ConnectionPool: ConnectionPoolConfig{
				MaxOpenConns:    25,
				MaxIdleConns:    5,
				ConnMaxLifetime: 5 * time.Minute,
			},
		},
		Business: BusinessConfig{
			Games: GamesConfig{
				MaxConcurrentGames: 100,
				MaxPlayersPerGame:  10,
				DefaultGameTimeout: 1 * time.Hour,
			},
			Functions: FunctionsConfig{
				Registry: RegistryConfig{
					MaxSize: 1000,
				},
				Execution: ExecutionConfig{
					DefaultTimeout: 30 * time.Second,
					MaxTimeout:     5 * time.Minute,
				},
			},
			Jobs: JobsConfig{
				Queue: QueueConfig{
					MaxSize: 1000,
				},
				Retry: RetryConfig{
					MaxAttempts:  3,
					InitialDelay: 1 * time.Second,
				},
			},
		},
	}

	result, err := advancedMgr.ValidateWithAdvancedValidation(validConfig)
	require.NoError(t, err)
	assert.True(t, result.Valid)
	assert.GreaterOrEqual(t, result.Score, 80) // 应该有较高的分数

	// 测试无效配置
	invalidConfig := &Config{
		App: AppConfig{
			Name:    "invalid-app",
			Version: "1.0.0",
			Env:     "production",
		},
		Network: NetworkConfig{
			Server: ServerConfig{
				HTTPPort: 8080,
				GRPCPort: 8080, // 端口冲突
			},
		},
		Security: SecurityConfig{
			JWT: JWTConfig{
				Enabled: true,
				Secret:  "short", // 密钥太短
				Expiry:  1 * time.Hour,
			},
			PasswordPolicy: PasswordPolicyConfig{
				MinLength: 6, // 太短
			},
		},
	}

	result, err = advancedMgr.ValidateWithAdvancedValidation(invalidConfig)
	require.NoError(t, err)
	assert.False(t, result.Valid)
	assert.Less(t, result.Score, 50)
	assert.True(t, len(result.Errors) > 0)
}

func TestAdvancedConfigManager_ExportWithHistory(t *testing.T) {
	ctx := context.Background()
	advancedMgr, err := NewAdvancedConfigManager(ctx)
	require.NoError(t, err)
	defer advancedMgr.Close()

	// 添加一些历史记录
	advancedMgr.recordHistory("load", "test-source", map[string]interface{}{
		"cache_key": "test-key",
	})

	// 导出配置
	export, err := advancedMgr.ExportWithHistory()
	require.NoError(t, err)
	require.NotNil(t, export)

	// 检查导出的内容
	assert.Contains(t, export, "config")
	assert.Contains(t, export, "history")
	assert.Contains(t, export, "metrics")
	assert.Contains(t, export, "export_time")
	assert.Contains(t, export, "version")

	// 检查历史记录
	history := export["history"]
	assert.NotNil(t, history)
}

func TestConfigEncryptor_EncryptAndDecrypt(t *testing.T) {
	// 创建一个简单的加密器（仅用于测试）
	key := make([]byte, 32) // AES-256
	for i := range key {
		key[i] = byte(i % 256)
	}

	block, err := aes.NewCipher(key)
	require.NoError(t, err)

	gcm, err := cipher.NewGCM(block)
	require.NoError(t, err)

	encryptor := &ConfigEncryptor{
		key:    key,
		cipher: gcm,
	}

	// 测试数据
	plaintext := []byte(`{"name": "test-config", "version": "1.0.0"}`)

	// 加密
	encrypted, err := encryptor.Encrypt(plaintext)
	require.NoError(t, err)
	assert.NotEmpty(t, encrypted)
	assert.NotEqual(t, string(plaintext), encrypted)

	// 解密
	decrypted, err := encryptor.Decrypt(encrypted)
	require.NoError(t, err)
	assert.Equal(t, plaintext, decrypted)
}

func TestConfigEncryptor_InvalidDecrypt(t *testing.T) {
	// 创建加密器
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i % 256)
	}

	block, err := aes.NewCipher(key)
	require.NoError(t, err)

	gcm, err := cipher.NewGCM(block)
	require.NoError(t, err)

	encryptor := &ConfigEncryptor{
		key:    key,
		cipher: gcm,
	}

	// 尝试解密无效数据
	invalidData := "invalid-base64-data"
	_, err = encryptor.Decrypt(invalidData)
	assert.Error(t, err)

	// 尝试解密太短的数据
	shortData := "aW52YWxpZA==" // base64编码的"invalid"
	_, err = encryptor.Decrypt(shortData)
	assert.Error(t, err)
}

func TestAdvancedConfigManager_GenerateID(t *testing.T) {
	advancedMgr := &AdvancedConfigManager{}
	id1 := advancedMgr.generateID()
	id2 := advancedMgr.generateID()

	assert.NotEmpty(t, id1)
	assert.NotEmpty(t, id2)
	assert.NotEqual(t, id1, id2)
	assert.True(t, strings.HasPrefix(id1, "cfg_"))
	assert.True(t, strings.HasPrefix(id2, "cfg_"))
}

func TestAdvancedConfigManager_HistoryLimit(t *testing.T) {
	history := NewConfigHistory(3) // 最大容量为3
	advancedMgr := &AdvancedConfigManager{history: history}

	// 添加超过容量的记录
	for i := 0; i < 5; i++ {
		advancedMgr.recordHistory("test", "test-source", map[string]interface{}{
			"iteration": i,
		})
	}

	// 获取历史记录，应该最多只有3条
	records := advancedMgr.GetHistory(10)
	assert.Equal(t, 3, len(records))

	// 检查最新的记录是最后添加的
	latestRecord := records[len(records)-1]
	assert.Contains(t, latestRecord.Changes, "iteration")
}

// Benchmark tests
func BenchmarkConfigCache_Set(b *testing.B) {
	cache := NewConfigCache(1000)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		cache.Set(fmt.Sprintf("key-%d", i), fmt.Sprintf("value-%d", i), 1*time.Hour)
	}
}

func BenchmarkConfigCache_Get(b *testing.B) {
	cache := NewConfigCache(1000)

	// 预填充缓存
	for i := 0; i < 100; i++ {
		cache.Set(fmt.Sprintf("key-%d", i), fmt.Sprintf("value-%d", i), 1*time.Hour)
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		cache.Get(fmt.Sprintf("key-%d", i%100))
	}
}
