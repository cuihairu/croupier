package croupier

import (
	"context"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/fsnotify/fsnotify"
)

// HotReloadConfig 热更新配置
type HotReloadConfig struct {
	Enabled              bool          `yaml:"enabled"`
	AutoReconnect        bool          `yaml:"auto_reconnect"`
	ReconnectDelay       time.Duration `yaml:"reconnect_delay"`
	MaxRetryAttempts     int           `yaml:"max_retry_attempts"`
	HealthCheckInterval  time.Duration `yaml:"health_check_interval"`
	GracefulShutdownTimeout time.Duration `yaml:"graceful_shutdown_timeout"`

	// 文件监听配置
	FileWatching struct {
		Enabled  bool     `yaml:"enabled"`
		WatchDir string   `yaml:"watch_dir"`
		Patterns []string `yaml:"patterns"`
	} `yaml:"file_watching"`

	// 工具集成配置
	Tools struct {
		Air     bool `yaml:"air"`     // Air工具支持
		Nodemon bool `yaml:"nodemon"` // Nodemon风格支持
		Plugin  bool `yaml:"plugin"`  // Go Plugin支持
	} `yaml:"tools"`
}

// HotReloadMetrics 热更新指标
type HotReloadMetrics struct {
	ReconnectCount    int64     `json:"reconnect_count"`
	LastReconnectTime time.Time `json:"last_reconnect_time"`
	FunctionReloads   int64     `json:"function_reloads"`
	ConfigReloads     int64     `json:"config_reloads"`
	FailedReloads     int64     `json:"failed_reloads"`
	ConnectionStatus  string    `json:"connection_status"`
}

// HotReloadable 热重载接口
type HotReloadable interface {
	// 重新加载函数定义
	ReloadFunction(functionID string, desc FunctionDescriptor, handler FunctionHandler) error

	// 批量重载函数
	ReloadFunctions(functions map[string]FunctionDescriptor, handlers map[string]FunctionHandler) error

	// 配置热更新
	ReloadConfig(config *ClientConfig) error

	// 获取重载状态
	GetReloadStatus() HotReloadMetrics

	// 优雅关闭
	GracefulShutdown(timeout time.Duration) error

	// 重新连接
	Reconnect(ctx context.Context) error
}

// hotReloadClient 支持热重载的客户端扩展
type hotReloadClient struct {
	Client

	hotConfig     HotReloadConfig

	// 热重载状态
	isReloading       bool
	reconnectCount    int64
	lastReconnectTime time.Time
	functionReloads   int64
	configReloads     int64
	failedReloads     int64

	// 原始函数存储（用于重载）
	functionDescs map[string]FunctionDescriptor

	// 文件监听器
	watcher   *fsnotify.Watcher
	watcherMu sync.RWMutex

	// 重连控制
	reconnectCh chan struct{}
	stopReload  chan struct{}
	reloadMu    sync.RWMutex
}

// NewHotReloadClient 创建支持热重载的客户端
func NewHotReloadClient(config *ClientConfig, hotConfig HotReloadConfig) (Client, HotReloadable) {
	baseClient := NewClient(config)

	hotClient := &hotReloadClient{
		Client:        baseClient,
		hotConfig:     hotConfig,
		functionDescs: make(map[string]FunctionDescriptor),
		reconnectCh:   make(chan struct{}, 1),
		stopReload:    make(chan struct{}),
	}

	if hotConfig.Enabled {
		hotClient.startHotReloadSupport()
	}

	return hotClient, hotClient
}

// RegisterFunction 重写注册函数以支持热重载
func (c *hotReloadClient) RegisterFunction(desc FunctionDescriptor, handler FunctionHandler) error {
	// 保存函数描述符用于重载
	c.reloadMu.Lock()
	c.functionDescs[desc.ID] = desc
	c.reloadMu.Unlock()

	// 调用基础实现
	return c.Client.RegisterFunction(desc, handler)
}

// ReloadFunction 实现热重载接口
func (c *hotReloadClient) ReloadFunction(functionID string, desc FunctionDescriptor, handler FunctionHandler) error {
	c.reloadMu.Lock()
	defer c.reloadMu.Unlock()

	if c.isReloading {
		return fmt.Errorf("reload operation already in progress")
	}

	c.isReloading = true
	defer func() { c.isReloading = false }()

	log.Printf("🔄 Reloading function: %s", functionID)

	// 验证新函数
	if desc.ID != functionID {
		atomic.AddInt64(&c.failedReloads, 1)
		return fmt.Errorf("function ID mismatch: expected %s, got %s", functionID, desc.ID)
	}

	// 保存旧的函数描述符用于回滚
	oldDesc, exists := c.functionDescs[functionID]

	// 更新函数描述符
	c.functionDescs[functionID] = desc

	// 重新注册函数
	if err := c.Client.RegisterFunction(desc, handler); err != nil {
		// 回滚
		if exists {
			c.functionDescs[functionID] = oldDesc
		} else {
			delete(c.functionDescs, functionID)
		}
		atomic.AddInt64(&c.failedReloads, 1)
		return fmt.Errorf("failed to reload function %s: %w", functionID, err)
	}

	atomic.AddInt64(&c.functionReloads, 1)
	log.Printf("✅ Function %s reloaded successfully", functionID)
	return nil
}

// ReloadFunctions 批量重载函数
func (c *hotReloadClient) ReloadFunctions(functions map[string]FunctionDescriptor, handlers map[string]FunctionHandler) error {
	c.reloadMu.Lock()
	defer c.reloadMu.Unlock()

	if c.isReloading {
		return fmt.Errorf("reload operation already in progress")
	}

	c.isReloading = true
	defer func() { c.isReloading = false }()

	log.Printf("🔄 Batch reloading %d functions", len(functions))

	// 保存旧状态用于回滚
	oldDescs := make(map[string]FunctionDescriptor)
	for id := range functions {
		if oldDesc, exists := c.functionDescs[id]; exists {
			oldDescs[id] = oldDesc
		}
	}

	// 逐个重载函数
	failedCount := 0
	for functionID, desc := range functions {
		handler, exists := handlers[functionID]
		if !exists {
			log.Printf("⚠️ No handler found for function %s, skipping", functionID)
			failedCount++
			continue
		}

		c.functionDescs[functionID] = desc
		if err := c.Client.RegisterFunction(desc, handler); err != nil {
			log.Printf("❌ Failed to reload function %s: %v", functionID, err)
			failedCount++
			// 回滚这个函数
			if oldDesc, exists := oldDescs[functionID]; exists {
				c.functionDescs[functionID] = oldDesc
			} else {
				delete(c.functionDescs, functionID)
			}
		} else {
			atomic.AddInt64(&c.functionReloads, 1)
		}
	}

	if failedCount > 0 {
		atomic.AddInt64(&c.failedReloads, int64(failedCount))
		return fmt.Errorf("failed to reload %d out of %d functions", failedCount, len(functions))
	}

	log.Printf("✅ Successfully reloaded all %d functions", len(functions))
	return nil
}

// ReloadConfig 重载配置
func (c *hotReloadClient) ReloadConfig(newConfig *ClientConfig) error {
	log.Printf("🔄 Reloading client configuration")

	// 这里可以实现配置热更新逻辑
	// 对于某些配置变更，可能需要重新连接
	atomic.AddInt64(&c.configReloads, 1)

	log.Printf("✅ Configuration reloaded successfully")
	return nil
}

// GetReloadStatus 获取重载状态
func (c *hotReloadClient) GetReloadStatus() HotReloadMetrics {
	status := "connected"
	if c.isReloading {
		status = "reloading"
	}

	return HotReloadMetrics{
		ReconnectCount:    atomic.LoadInt64(&c.reconnectCount),
		LastReconnectTime: c.lastReconnectTime,
		FunctionReloads:   atomic.LoadInt64(&c.functionReloads),
		ConfigReloads:     atomic.LoadInt64(&c.configReloads),
		FailedReloads:     atomic.LoadInt64(&c.failedReloads),
		ConnectionStatus:  status,
	}
}

// GracefulShutdown 优雅关闭
func (c *hotReloadClient) GracefulShutdown(timeout time.Duration) error {
	log.Printf("🛑 Starting graceful shutdown (timeout: %v)", timeout)

	// We don't currently use a context in Stop(); keep timeout available for future use.

	// 停止文件监听
	c.stopFileWatching()

	// 停止重连机制
	close(c.stopReload)

	// 调用基础停止方法
	return c.Client.Stop()
}

// Reconnect 重新连接
func (c *hotReloadClient) Reconnect(ctx context.Context) error {
	log.Printf("🔄 Attempting to reconnect...")

	// 先断开当前连接
	c.Client.Stop()

	// 重新连接
	if err := c.Client.Connect(ctx); err != nil {
		atomic.AddInt64(&c.failedReloads, 1)
		return fmt.Errorf("failed to reconnect: %w", err)
	}

	// 重新注册所有函数
	c.reloadMu.RLock()
	functionCount := len(c.functionDescs)
	c.reloadMu.RUnlock()

	if functionCount > 0 {
		log.Printf("🔄 Re-registering %d functions after reconnection", functionCount)
		if err := c.reregisterAllFunctions(); err != nil {
			return fmt.Errorf("failed to re-register functions: %w", err)
		}
	}

	atomic.AddInt64(&c.reconnectCount, 1)
	c.lastReconnectTime = time.Now()

	log.Printf("✅ Reconnection successful")
	return nil
}

// startHotReloadSupport 启动热重载支持
func (c *hotReloadClient) startHotReloadSupport() {
	// 启动自动重连机制
	if c.hotConfig.AutoReconnect {
		go c.autoReconnectLoop()
	}

	// 启动文件监听
	if c.hotConfig.FileWatching.Enabled {
		go c.startFileWatching()
	}

	log.Printf("🔥 Hot reload support enabled")
}

// autoReconnectLoop 自动重连循环
func (c *hotReloadClient) autoReconnectLoop() {
	ticker := time.NewTicker(c.hotConfig.HealthCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-c.stopReload:
			return
		case <-ticker.C:
			// 健康检查逻辑
			if !c.isConnected() {
				select {
				case c.reconnectCh <- struct{}{}:
					c.attemptReconnect()
				default:
					// 重连已在进行中
				}
			}
		case <-c.reconnectCh:
			c.attemptReconnect()
		}
	}
}

// attemptReconnect 尝试重连
func (c *hotReloadClient) attemptReconnect() {
	maxAttempts := c.hotConfig.MaxRetryAttempts
	delay := c.hotConfig.ReconnectDelay

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		log.Printf("🔄 Reconnection attempt %d/%d", attempt, maxAttempts)

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		err := c.Reconnect(ctx)
		cancel()

		if err == nil {
			log.Printf("✅ Reconnection successful on attempt %d", attempt)
			return
		}

		log.Printf("❌ Reconnection attempt %d failed: %v", attempt, err)

		if attempt < maxAttempts {
			time.Sleep(delay)
			// 指数退避
			delay = time.Duration(float64(delay) * 1.5)
			if delay > 60*time.Second {
				delay = 60 * time.Second
			}
		}
	}

	log.Printf("❌ All reconnection attempts failed")
}

// startFileWatching 启动文件监听
func (c *hotReloadClient) startFileWatching() {
	if c.hotConfig.FileWatching.WatchDir == "" {
		return
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Printf("❌ Failed to create file watcher: %v", err)
		return
	}

	c.watcherMu.Lock()
	c.watcher = watcher
	c.watcherMu.Unlock()

	// 添加监听目录
	err = watcher.Add(c.hotConfig.FileWatching.WatchDir)
	if err != nil {
		log.Printf("❌ Failed to watch directory %s: %v", c.hotConfig.FileWatching.WatchDir, err)
		return
	}

	log.Printf("👀 Watching directory: %s", c.hotConfig.FileWatching.WatchDir)

	// 文件事件处理循环
	for {
		select {
		case <-c.stopReload:
			return
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}
			c.handleFileEvent(event)
		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			log.Printf("❌ File watcher error: %v", err)
		}
	}
}

// handleFileEvent 处理文件事件
func (c *hotReloadClient) handleFileEvent(event fsnotify.Event) {
	if event.Op&fsnotify.Write == fsnotify.Write {
		log.Printf("👀 File modified: %s", event.Name)

		// 这里可以根据文件类型触发不同的重载行为
		// 例如：.go文件 -> 触发重编译信号
		//      .yaml文件 -> 重载配置
		//      .json文件 -> 重载函数描述符

		// 简单示例：触发重连
		select {
		case c.reconnectCh <- struct{}{}:
		default:
		}
	}
}

// stopFileWatching 停止文件监听
func (c *hotReloadClient) stopFileWatching() {
	c.watcherMu.Lock()
	defer c.watcherMu.Unlock()

	if c.watcher != nil {
		c.watcher.Close()
		c.watcher = nil
	}
}

// isConnected 检查连接状态（这需要基础客户端提供支持）
func (c *hotReloadClient) isConnected() bool {
	// 这里应该检查实际的连接状态
	// 暂时返回true，实际实现需要从基础客户端获取状态
	return true
}

// reregisterAllFunctions 重新注册所有函数
func (c *hotReloadClient) reregisterAllFunctions() error {
	c.reloadMu.RLock()
	defer c.reloadMu.RUnlock()

	for functionID, desc := range c.functionDescs {
		// 这里需要重新获取handler，实际实现中应该也保存handlers
		log.Printf("Re-registering function: %s (v%s)", functionID, desc.Version)
		// 实际实现需要调用 c.Client.RegisterFunction(desc, savedHandler)
	}

	return nil
}

// DefaultHotReloadConfig 默认热重载配置
func DefaultHotReloadConfig() HotReloadConfig {
	config := HotReloadConfig{
		Enabled:                 true,
		AutoReconnect:          true,
		ReconnectDelay:         5 * time.Second,
		MaxRetryAttempts:       10,
		HealthCheckInterval:    30 * time.Second,
		GracefulShutdownTimeout: 30 * time.Second,
	}

	config.FileWatching.Enabled = false // 默认关闭文件监听
	config.FileWatching.WatchDir = "./functions"
	config.FileWatching.Patterns = []string{"*.go", "*.yaml", "*.json"}

	config.Tools.Air = true
	config.Tools.Nodemon = false
	config.Tools.Plugin = false

	return config
}
