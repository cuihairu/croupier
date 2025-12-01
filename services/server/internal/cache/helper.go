package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/cuihairu/croupier/services/server/internal/config"
	"github.com/zeromicro/go-zero/core/logx"
)

// NewCacheStore 根据配置创建缓存实例
func NewCacheStore(cfg config.CacheConfig) (CacheStore, error) {
	if !cfg.Enabled {
		logx.Info("Cache is disabled, using NullCache")
		return NewNullCache(), nil
	}

	cacheType := strings.ToLower(strings.TrimSpace(cfg.Type))
	if cacheType == "" {
		cacheType = "local" // 默认使用本地缓存
	}

	// 解析默认 TTL
	var defaultTTL time.Duration
	if cfg.TTL != "" {
		ttl, err := time.ParseDuration(cfg.TTL)
		if err != nil {
			logx.Errorf("Invalid cache TTL %s, using default 5m: %v", cfg.TTL, err)
			defaultTTL = 5 * time.Minute
		} else {
			defaultTTL = ttl
		}
	} else {
		defaultTTL = 5 * time.Minute
	}

	switch cacheType {
	case "redis":
		return newRedisCache(cfg, defaultTTL)
	case "local", "memory":
		return newLocalCache(cfg, defaultTTL)
	default:
		return nil, fmt.Errorf("unsupported cache type: %s", cacheType)
	}
}

func newRedisCache(cfg config.CacheConfig, defaultTTL time.Duration) (CacheStore, error) {
	addr := cfg.Addr
	if addr == "" {
		addr = "localhost:6379"
	}

	poolSize := cfg.PoolSize
	if poolSize <= 0 {
		poolSize = 10
	}

	cache, err := NewRedisCache(addr, cfg.Password, cfg.DB, poolSize, defaultTTL)
	if err != nil {
		return nil, fmt.Errorf("failed to create redis cache: %w", err)
	}

	logx.Infof("Redis cache initialized: addr=%s db=%d pool=%d ttl=%s",
		addr, cfg.DB, poolSize, defaultTTL)

	return cache, nil
}

func newLocalCache(cfg config.CacheConfig, defaultTTL time.Duration) (CacheStore, error) {
	// 解析清理间隔
	var cleanupInterval time.Duration
	if cfg.EvictTTL != "" {
		interval, err := time.ParseDuration(cfg.EvictTTL)
		if err != nil {
			logx.Errorf("Invalid evict TTL %s, using default 10m: %v", cfg.EvictTTL, err)
			cleanupInterval = 10 * time.Minute
		} else {
			cleanupInterval = interval
		}
	} else {
		cleanupInterval = 10 * time.Minute
	}

	cache := NewLocalCache(defaultTTL, cleanupInterval)

	logx.Infof("Local cache initialized: ttl=%s cleanup=%s",
		defaultTTL, cleanupInterval)

	return cache, nil
}

// CacheHelper 缓存辅助函数
type CacheHelper struct {
	store CacheStore
}

func NewCacheHelper(store CacheStore) *CacheHelper {
	return &CacheHelper{store: store}
}

// GetJSON 从缓存获取 JSON 对象
func (h *CacheHelper) GetJSON(ctx context.Context, key string, dest interface{}) error {
	data, err := h.store.Get(ctx, key)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, dest)
}

// SetJSON 设置 JSON 对象到缓存
func (h *CacheHelper) SetJSON(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return h.store.Set(ctx, key, data, ttl)
}

// Remember 缓存模式：如果缓存存在则返回，否则执行 loader 并缓存结果
func (h *CacheHelper) Remember(ctx context.Context, key string, ttl time.Duration, dest interface{}, loader func() (interface{}, error)) error {
	// 尝试从缓存获取
	err := h.GetJSON(ctx, key, dest)
	if err == nil {
		return nil // 缓存命中
	}

	if !IsCacheMiss(err) {
		logx.Errorf("Cache error for key %s: %v", key, err)
	}

	// 缓存未命中，执行 loader
	value, err := loader()
	if err != nil {
		return err
	}

	// 保存到缓存
	if err := h.SetJSON(ctx, key, value, ttl); err != nil {
		logx.Errorf("Failed to cache key %s: %v", key, err)
	}

	// 将加载的值复制到 dest
	data, _ := json.Marshal(value)
	return json.Unmarshal(data, dest)
}

// CacheKey 生成缓存键的辅助函数
func CacheKey(parts ...string) string {
	return "croupier:" + strings.Join(parts, ":")
}

// AdminCacheKey 生成管理员缓存键
func AdminCacheKey(username string) string {
	return CacheKey("admin", "user", strings.ToLower(username))
}

// AdminIDCacheKey 生成基于ID的管理员缓存键
func AdminIDCacheKey(adminID uint) string {
	return CacheKey("admin", "id", fmt.Sprintf("%d", adminID))
}

// AdminRolesCacheKey 生成管理员角色缓存键
func AdminRolesCacheKey(adminID uint) string {
	return CacheKey("admin", "roles", fmt.Sprintf("%d", adminID))
}

// RoleCacheKey 生成角色缓存键
func RoleCacheKey(roleID uint) string {
	return CacheKey("role", fmt.Sprintf("%d", roleID))
}

// RolePermissionsCacheKey 生成角色权限缓存键
func RolePermissionsCacheKey(roleID uint) string {
	return CacheKey("role", "perms", fmt.Sprintf("%d", roleID))
}

// PermissionCacheKey 生成权限缓存键
func PermissionCacheKey(permissionID string) string {
	return CacheKey("permission", strings.ToLower(permissionID))
}

// GameCacheKey 生成游戏缓存键
func GameCacheKey(gameID uint) string {
	return CacheKey("game", fmt.Sprintf("%d", gameID))
}

// GamesCacheKey 生成游戏列表缓存键
func GamesCacheKey() string {
	return CacheKey("games", "all")
}
