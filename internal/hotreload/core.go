package hotreload

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"plugin"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// HotReloader Croupier热更新核心接口
type HotReloader interface {
	// 注册热更新处理器
	RegisterHandler(pattern string, handler ReloadHandler) error

	// 启动文件监听
	StartWatching(ctx context.Context) error

	// 手动触发重载
	Reload(path string) error

	// 获取当前版本信息
	GetVersion() *VersionInfo

	// 停止热更新
	Stop() error
}

// ReloadHandler 热更新回调函数
type ReloadHandler func(ctx context.Context, event ReloadEvent) error

// ReloadEvent 热更新事件
type ReloadEvent struct {
	Type      ReloadType `json:"type"`
	Path      string     `json:"path"`
	Content   []byte     `json:"content,omitempty"`
	Version   string     `json:"version"`
	Timestamp time.Time  `json:"timestamp"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// ReloadType 热更新类型
type ReloadType string

const (
	ReloadTypeConfig   ReloadType = "config"
	ReloadTypeScript   ReloadType = "script"
	ReloadTypePlugin   ReloadType = "plugin"
	ReloadTypeAsset    ReloadType = "asset"
	ReloadTypeFunction ReloadType = "function"
)

// VersionInfo 版本信息
type VersionInfo struct {
	Version   string            `json:"version"`
	BuildTime time.Time         `json:"build_time"`
	Files     map[string]string `json:"files"` // path -> hash
}

// Config 热更新配置
type Config struct {
	// 基础配置
	ServerURL    string `json:"server_url"`    // Croupier服务器地址
	GameID       string `json:"game_id"`       // 游戏ID
	Environment  string `json:"environment"`   // 环境：dev/test/prod

	// 监听配置
	WatchDirs    []string `json:"watch_dirs"`    // 监听目录
	WatchExts    []string `json:"watch_exts"`    // 监听扩展名
	IgnorePatterns []string `json:"ignore_patterns"` // 忽略模式

	// 行为配置
	PollInterval    time.Duration `json:"poll_interval"`     // 轮询间隔
	DebounceTime    time.Duration `json:"debounce_time"`     // 防抖时间
	MaxRetries      int           `json:"max_retries"`       // 最大重试次数
	EnableRemote    bool          `json:"enable_remote"`     // 启用远程更新
	AutoReload      bool          `json:"auto_reload"`       // 自动重载
	BackupEnabled   bool          `json:"backup_enabled"`    // 启用备份
}

// DefaultConfig 默认配置
func DefaultConfig() *Config {
	return &Config{
		Environment:     "development",
		WatchDirs:       []string{"./configs", "./scripts", "./plugins"},
		WatchExts:       []string{".json", ".yaml", ".yml", ".lua", ".js", ".so", ".dll"},
		IgnorePatterns:  []string{"*.tmp", "*.log", ".git/*"},
		PollInterval:    time.Second,
		DebounceTime:    500 * time.Millisecond,
		MaxRetries:      3,
		EnableRemote:    true,
		AutoReload:      true,
		BackupEnabled:   true,
	}
}

// croupierHotReloader Croupier热更新实现
type croupierHotReloader struct {
	config   *Config
	logger   *slog.Logger
	watcher  *fsnotify.Watcher
	handlers map[string]ReloadHandler
	plugins  map[string]*plugin.Plugin
	version  *VersionInfo

	// 运行时状态
	running    bool
	stopChan   chan struct{}
	debouncers map[string]*time.Timer
	mutex      sync.RWMutex

	// 远程同步
	httpClient *http.Client
}

// NewHotReloader 创建热更新实例
func NewHotReloader(config *Config, logger *slog.Logger) (HotReloader, error) {
	if config == nil {
		config = DefaultConfig()
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to create watcher: %w", err)
	}

	hr := &croupierHotReloader{
		config:     config,
		logger:     logger,
		watcher:    watcher,
		handlers:   make(map[string]ReloadHandler),
		plugins:    make(map[string]*plugin.Plugin),
		debouncers: make(map[string]*time.Timer),
		stopChan:   make(chan struct{}),
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}

	// 初始化版本信息
	hr.version = &VersionInfo{
		Version:   "1.0.0",
		BuildTime: time.Now(),
		Files:     make(map[string]string),
	}

	return hr, nil
}

// RegisterHandler 注册处理器
func (hr *croupierHotReloader) RegisterHandler(pattern string, handler ReloadHandler) error {
	hr.mutex.Lock()
	defer hr.mutex.Unlock()

	hr.handlers[pattern] = handler
	hr.logger.Info("Registered hot reload handler", "pattern", pattern)
	return nil
}

// StartWatching 开始监听
func (hr *croupierHotReloader) StartWatching(ctx context.Context) error {
	hr.mutex.Lock()
	if hr.running {
		hr.mutex.Unlock()
		return fmt.Errorf("hot reloader is already running")
	}
	hr.running = true
	hr.mutex.Unlock()

	// 添加监听目录
	for _, dir := range hr.config.WatchDirs {
		if err := hr.addWatchDir(dir); err != nil {
			hr.logger.Error("Failed to add watch dir", "dir", dir, "error", err)
		}
	}

	// 启动监听协程
	go hr.watchLoop(ctx)

	// 启动远程同步（如果启用）
	if hr.config.EnableRemote {
		go hr.remoteSyncLoop(ctx)
	}

	hr.logger.Info("Hot reloader started",
		"watch_dirs", hr.config.WatchDirs,
		"remote_enabled", hr.config.EnableRemote)

	return nil
}

// addWatchDir 添加监听目录
func (hr *croupierHotReloader) addWatchDir(dir string) error {
	return filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			return hr.watcher.Add(path)
		}
		return nil
	})
}

// watchLoop 监听循环
func (hr *croupierHotReloader) watchLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-hr.stopChan:
			return
		case event, ok := <-hr.watcher.Events:
			if !ok {
				return
			}
			hr.handleFileEvent(ctx, event)
		case err, ok := <-hr.watcher.Errors:
			if !ok {
				return
			}
			hr.logger.Error("File watcher error", "error", err)
		}
	}
}

// handleFileEvent 处理文件事件
func (hr *croupierHotReloader) handleFileEvent(ctx context.Context, event fsnotify.Event) {
	// 过滤不需要的事件
	if !hr.shouldHandleFile(event.Name) {
		return
	}

	hr.logger.Debug("File event", "event", event.Op.String(), "file", event.Name)

	// 防抖处理
	hr.debounceReload(ctx, event.Name, func() {
		if err := hr.reloadFile(ctx, event.Name); err != nil {
			hr.logger.Error("Failed to reload file", "file", event.Name, "error", err)
		}
	})
}

// debounceReload 防抖重载
func (hr *croupierHotReloader) debounceReload(ctx context.Context, path string, reloadFunc func()) {
	hr.mutex.Lock()
	defer hr.mutex.Unlock()

	// 取消之前的定时器
	if timer, exists := hr.debouncers[path]; exists {
		timer.Stop()
	}

	// 创建新的定时器
	hr.debouncers[path] = time.AfterFunc(hr.config.DebounceTime, reloadFunc)
}

// shouldHandleFile 判断是否需要处理文件
func (hr *croupierHotReloader) shouldHandleFile(path string) bool {
	// 检查扩展名
	ext := filepath.Ext(path)
	for _, allowedExt := range hr.config.WatchExts {
		if ext == allowedExt {
			// 检查是否在忽略列表中
			for _, pattern := range hr.config.IgnorePatterns {
				if matched, _ := filepath.Match(pattern, filepath.Base(path)); matched {
					return false
				}
			}
			return true
		}
	}
	return false
}

// reloadFile 重载文件
func (hr *croupierHotReloader) reloadFile(ctx context.Context, path string) error {
	// 读取文件内容
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	// 创建重载事件
	reloadEvent := ReloadEvent{
		Type:      hr.detectReloadType(path),
		Path:      path,
		Content:   content,
		Version:   hr.generateVersion(path, content),
		Timestamp: time.Now(),
		Metadata: map[string]interface{}{
			"size": len(content),
			"ext":  filepath.Ext(path),
		},
	}

	// 备份原文件（如果启用）
	if hr.config.BackupEnabled {
		if err := hr.backupFile(path); err != nil {
			hr.logger.Warn("Failed to backup file", "file", path, "error", err)
		}
	}

	// 调用匹配的处理器
	return hr.callHandlers(ctx, reloadEvent)
}

// detectReloadType 检测重载类型
func (hr *croupierHotReloader) detectReloadType(path string) ReloadType {
	ext := filepath.Ext(path)
	base := filepath.Base(path)

	switch {
	case ext == ".json" || ext == ".yaml" || ext == ".yml":
		return ReloadTypeConfig
	case ext == ".lua" || ext == ".js" || ext == ".py":
		return ReloadTypeScript
	case ext == ".so" || ext == ".dll":
		return ReloadTypePlugin
	case base == "assets" || filepath.Dir(path) == "assets":
		return ReloadTypeAsset
	default:
		return ReloadTypeFunction
	}
}

// generateVersion 生成版本号
func (hr *croupierHotReloader) generateVersion(path string, content []byte) string {
	// 简单的基于时间戳和内容哈希的版本号
	hash := fmt.Sprintf("%x", len(content)) // 简化的哈希
	timestamp := time.Now().Format("20060102150405")
	return fmt.Sprintf("%s_%s", timestamp, hash)
}

// backupFile 备份文件
func (hr *croupierHotReloader) backupFile(path string) error {
	backupDir := "./backups/hotreload"
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return err
	}

	timestamp := time.Now().Format("20060102_150405")
	backupPath := filepath.Join(backupDir, fmt.Sprintf("%s_%s",
		filepath.Base(path), timestamp))

	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	return os.WriteFile(backupPath, content, 0644)
}

// callHandlers 调用处理器
func (hr *croupierHotReloader) callHandlers(ctx context.Context, event ReloadEvent) error {
	hr.mutex.RLock()
	defer hr.mutex.RUnlock()

	var lastError error
	handlerCalled := false

	// 查找匹配的处理器
	for pattern, handler := range hr.handlers {
		matched, err := filepath.Match(pattern, event.Path)
		if err != nil {
			hr.logger.Error("Pattern match error", "pattern", pattern, "error", err)
			continue
		}

		if matched {
			handlerCalled = true
			hr.logger.Info("Calling reload handler", "pattern", pattern, "file", event.Path)

			if err := handler(ctx, event); err != nil {
				hr.logger.Error("Handler failed", "pattern", pattern, "file", event.Path, "error", err)
				lastError = err
			} else {
				hr.logger.Info("Handler succeeded", "pattern", pattern, "file", event.Path)
			}
		}
	}

	if !handlerCalled {
		hr.logger.Warn("No handler found for file", "file", event.Path)
	}

	return lastError
}

// Reload 手动触发重载
func (hr *croupierHotReloader) Reload(path string) error {
	ctx := context.Background()
	return hr.reloadFile(ctx, path)
}

// GetVersion 获取版本信息
func (hr *croupierHotReloader) GetVersion() *VersionInfo {
	hr.mutex.RLock()
	defer hr.mutex.RUnlock()

	// 深拷贝避免并发问题
	version := &VersionInfo{
		Version:   hr.version.Version,
		BuildTime: hr.version.BuildTime,
		Files:     make(map[string]string),
	}

	for k, v := range hr.version.Files {
		version.Files[k] = v
	}

	return version
}

// remoteSyncLoop 远程同步循环
func (hr *croupierHotReloader) remoteSyncLoop(ctx context.Context) {
	ticker := time.NewTicker(hr.config.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-hr.stopChan:
			return
		case <-ticker.C:
			if err := hr.checkRemoteUpdates(ctx); err != nil {
				hr.logger.Error("Remote sync failed", "error", err)
			}
		}
	}
}

// checkRemoteUpdates 检查远程更新
func (hr *croupierHotReloader) checkRemoteUpdates(ctx context.Context) error {
	if hr.config.ServerURL == "" || hr.config.GameID == "" {
		return nil // 未配置远程服务器
	}

	url := fmt.Sprintf("%s/api/hotreload/version?game_id=%s&env=%s",
		hr.config.ServerURL, hr.config.GameID, hr.config.Environment)

	resp, err := hr.httpClient.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server returned status %d", resp.StatusCode)
	}

	var remoteVersion VersionInfo
	if err := json.NewDecoder(resp.Body).Decode(&remoteVersion); err != nil {
		return err
	}

	// 比较版本并处理更新
	return hr.handleRemoteUpdate(ctx, &remoteVersion)
}

// handleRemoteUpdate 处理远程更新
func (hr *croupierHotReloader) handleRemoteUpdate(ctx context.Context, remoteVersion *VersionInfo) error {
	// 简化实现：比较版本号
	if remoteVersion.Version != hr.version.Version {
		hr.logger.Info("Remote version differs",
			"local", hr.version.Version,
			"remote", remoteVersion.Version)

		// 这里可以实现具体的更新逻辑
		// 例如：下载新文件、触发重载等
		return hr.downloadAndApplyUpdates(ctx, remoteVersion)
	}

	return nil
}

// downloadAndApplyUpdates 下载并应用更新
func (hr *croupierHotReloader) downloadAndApplyUpdates(ctx context.Context, version *VersionInfo) error {
	// 实现具体的下载和应用逻辑
	hr.logger.Info("Applying remote updates", "version", version.Version)

	// 更新本地版本信息
	hr.mutex.Lock()
	hr.version = version
	hr.mutex.Unlock()

	return nil
}

// Stop 停止热更新
func (hr *croupierHotReloader) Stop() error {
	hr.mutex.Lock()
	defer hr.mutex.Unlock()

	if !hr.running {
		return nil
	}

	hr.running = false
	close(hr.stopChan)

	// 取消所有防抖定时器
	for _, timer := range hr.debouncers {
		timer.Stop()
	}

	// 关闭文件监听器
	if hr.watcher != nil {
		hr.watcher.Close()
	}

	hr.logger.Info("Hot reloader stopped")
	return nil
}

// === 便捷的全局实例 ===

var globalHotReloader HotReloader

// InitGlobal 初始化全局热更新实例
func InitGlobal(config *Config, logger *slog.Logger) error {
	hr, err := NewHotReloader(config, logger)
	if err != nil {
		return err
	}
	globalHotReloader = hr
	return nil
}

// RegisterGlobalHandler 注册全局处理器
func RegisterGlobalHandler(pattern string, handler ReloadHandler) error {
	if globalHotReloader == nil {
		return fmt.Errorf("global hot reloader not initialized")
	}
	return globalHotReloader.RegisterHandler(pattern, handler)
}

// StartGlobalWatching 开始全局监听
func StartGlobalWatching(ctx context.Context) error {
	if globalHotReloader == nil {
		return fmt.Errorf("global hot reloader not initialized")
	}
	return globalHotReloader.StartWatching(ctx)
}

// StopGlobal 停止全局热更新
func StopGlobal() error {
	if globalHotReloader != nil {
		return globalHotReloader.Stop()
	}
	return nil
}