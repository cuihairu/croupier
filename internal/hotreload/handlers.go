package hotreload

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"path/filepath"
	"plugin"
	"strings"

	"gopkg.in/yaml.v3"
)

// === Go语言特定的热更新处理器 ===

// GoPluginHandler Go插件热更新处理器
type GoPluginHandler struct {
	logger  *slog.Logger
	plugins map[string]*plugin.Plugin
}

// NewGoPluginHandler 创建Go插件处理器
func NewGoPluginHandler(logger *slog.Logger) *GoPluginHandler {
	return &GoPluginHandler{
		logger:  logger,
		plugins: make(map[string]*plugin.Plugin),
	}
}

// Handle 处理Go插件热更新
func (h *GoPluginHandler) Handle(ctx context.Context, event ReloadEvent) error {
	if event.Type != ReloadTypePlugin {
		return nil
	}

	// 检查是否是.so文件（Go插件）
	if !strings.HasSuffix(event.Path, ".so") {
		return nil
	}

	h.logger.Info("Reloading Go plugin", "path", event.Path)

	// 关闭旧插件（如果存在）
	if oldPlugin, exists := h.plugins[event.Path]; exists {
		// Go插件无法真正卸载，只能覆盖引用
		_ = oldPlugin
	}

	// 加载新插件
	newPlugin, err := plugin.Open(event.Path)
	if err != nil {
		return fmt.Errorf("failed to load plugin %s: %w", event.Path, err)
	}

	h.plugins[event.Path] = newPlugin
	h.logger.Info("Go plugin reloaded successfully", "path", event.Path)

	return nil
}

// GetPlugin 获取已加载的插件
func (h *GoPluginHandler) GetPlugin(path string) (*plugin.Plugin, bool) {
	plugin, exists := h.plugins[path]
	return plugin, exists
}

// === 配置文件热更新处理器 ===

// ConfigHandler 配置文件热更新处理器
type ConfigHandler struct {
	logger      *slog.Logger
	configStore map[string]interface{}
	callbacks   map[string][]ConfigChangeCallback
}

// ConfigChangeCallback 配置变更回调
type ConfigChangeCallback func(oldConfig, newConfig interface{}) error

// NewConfigHandler 创建配置处理器
func NewConfigHandler(logger *slog.Logger) *ConfigHandler {
	return &ConfigHandler{
		logger:      logger,
		configStore: make(map[string]interface{}),
		callbacks:   make(map[string][]ConfigChangeCallback),
	}
}

// Handle 处理配置文件热更新
func (h *ConfigHandler) Handle(ctx context.Context, event ReloadEvent) error {
	if event.Type != ReloadTypeConfig {
		return nil
	}

	h.logger.Info("Reloading config", "path", event.Path)

	// 保存旧配置
	oldConfig := h.configStore[event.Path]

	// 解析新配置
	var newConfig interface{}
	ext := filepath.Ext(event.Path)

	switch ext {
	case ".json":
		if err := json.Unmarshal(event.Content, &newConfig); err != nil {
			return fmt.Errorf("failed to parse JSON config %s: %w", event.Path, err)
		}
	case ".yaml", ".yml":
		if err := yaml.Unmarshal(event.Content, &newConfig); err != nil {
			return fmt.Errorf("failed to parse YAML config %s: %w", event.Path, err)
		}
	default:
		return fmt.Errorf("unsupported config format: %s", ext)
	}

	// 更新配置存储
	h.configStore[event.Path] = newConfig

	// 调用配置变更回调
	if callbacks, exists := h.callbacks[event.Path]; exists {
		for _, callback := range callbacks {
			if err := callback(oldConfig, newConfig); err != nil {
				h.logger.Error("Config change callback failed",
					"path", event.Path, "error", err)
			}
		}
	}

	h.logger.Info("Config reloaded successfully", "path", event.Path)
	return nil
}

// GetConfig 获取配置
func (h *ConfigHandler) GetConfig(path string) (interface{}, bool) {
	config, exists := h.configStore[path]
	return config, exists
}

// RegisterCallback 注册配置变更回调
func (h *ConfigHandler) RegisterCallback(path string, callback ConfigChangeCallback) {
	h.callbacks[path] = append(h.callbacks[path], callback)
}

// === 脚本热更新处理器 ===

// ScriptHandler 脚本热更新处理器
type ScriptHandler struct {
	logger      *slog.Logger
	scripts     map[string][]byte
	interpreters map[string]ScriptInterpreter
}

// ScriptInterpreter 脚本解释器接口
type ScriptInterpreter interface {
	Execute(script []byte, context map[string]interface{}) (interface{}, error)
	Validate(script []byte) error
}

// NewScriptHandler 创建脚本处理器
func NewScriptHandler(logger *slog.Logger) *ScriptHandler {
	return &ScriptHandler{
		logger:       logger,
		scripts:      make(map[string][]byte),
		interpreters: make(map[string]ScriptInterpreter),
	}
}

// Handle 处理脚本热更新
func (h *ScriptHandler) Handle(ctx context.Context, event ReloadEvent) error {
	if event.Type != ReloadTypeScript {
		return nil
	}

	h.logger.Info("Reloading script", "path", event.Path)

	// 获取脚本类型
	ext := filepath.Ext(event.Path)
	interpreter, exists := h.interpreters[ext]
	if !exists {
		return fmt.Errorf("no interpreter registered for %s", ext)
	}

	// 验证脚本语法
	if err := interpreter.Validate(event.Content); err != nil {
		return fmt.Errorf("script validation failed for %s: %w", event.Path, err)
	}

	// 更新脚本存储
	h.scripts[event.Path] = event.Content

	h.logger.Info("Script reloaded successfully", "path", event.Path)
	return nil
}

// RegisterInterpreter 注册脚本解释器
func (h *ScriptHandler) RegisterInterpreter(ext string, interpreter ScriptInterpreter) {
	h.interpreters[ext] = interpreter
}

// ExecuteScript 执行脚本
func (h *ScriptHandler) ExecuteScript(path string, context map[string]interface{}) (interface{}, error) {
	script, exists := h.scripts[path]
	if !exists {
		return nil, fmt.Errorf("script not found: %s", path)
	}

	ext := filepath.Ext(path)
	interpreter, exists := h.interpreters[ext]
	if !exists {
		return nil, fmt.Errorf("no interpreter for %s", ext)
	}

	return interpreter.Execute(script, context)
}

// === 资源文件热更新处理器 ===

// AssetHandler 资源文件热更新处理器
type AssetHandler struct {
	logger     *slog.Logger
	assets     map[string][]byte
	assetHooks map[string][]AssetReloadHook
}

// AssetReloadHook 资源重载钩子
type AssetReloadHook func(path string, content []byte) error

// NewAssetHandler 创建资源处理器
func NewAssetHandler(logger *slog.Logger) *AssetHandler {
	return &AssetHandler{
		logger:     logger,
		assets:     make(map[string][]byte),
		assetHooks: make(map[string][]AssetReloadHook),
	}
}

// Handle 处理资源文件热更新
func (h *AssetHandler) Handle(ctx context.Context, event ReloadEvent) error {
	if event.Type != ReloadTypeAsset {
		return nil
	}

	h.logger.Info("Reloading asset", "path", event.Path)

	// 更新资源存储
	h.assets[event.Path] = event.Content

	// 调用资源重载钩子
	if hooks, exists := h.assetHooks[event.Path]; exists {
		for _, hook := range hooks {
			if err := hook(event.Path, event.Content); err != nil {
				h.logger.Error("Asset reload hook failed",
					"path", event.Path, "error", err)
			}
		}
	}

	h.logger.Info("Asset reloaded successfully", "path", event.Path)
	return nil
}

// GetAsset 获取资源
func (h *AssetHandler) GetAsset(path string) ([]byte, bool) {
	asset, exists := h.assets[path]
	return asset, exists
}

// RegisterHook 注册资源重载钩子
func (h *AssetHandler) RegisterHook(path string, hook AssetReloadHook) {
	h.assetHooks[path] = append(h.assetHooks[path], hook)
}

// === 复合处理器管理器 ===

// HandlerManager 处理器管理器
type HandlerManager struct {
	logger        *slog.Logger
	pluginHandler *GoPluginHandler
	configHandler *ConfigHandler
	scriptHandler *ScriptHandler
	assetHandler  *AssetHandler
}

// NewHandlerManager 创建处理器管理器
func NewHandlerManager(logger *slog.Logger) *HandlerManager {
	return &HandlerManager{
		logger:        logger,
		pluginHandler: NewGoPluginHandler(logger),
		configHandler: NewConfigHandler(logger),
		scriptHandler: NewScriptHandler(logger),
		assetHandler:  NewAssetHandler(logger),
	}
}

// Handle 处理所有类型的热更新
func (m *HandlerManager) Handle(ctx context.Context, event ReloadEvent) error {
	m.logger.Info("Processing reload event",
		"type", event.Type,
		"path", event.Path,
		"version", event.Version)

	var errors []error

	// 根据类型分发到相应处理器
	switch event.Type {
	case ReloadTypePlugin:
		if err := m.pluginHandler.Handle(ctx, event); err != nil {
			errors = append(errors, err)
		}
	case ReloadTypeConfig:
		if err := m.configHandler.Handle(ctx, event); err != nil {
			errors = append(errors, err)
		}
	case ReloadTypeScript:
		if err := m.scriptHandler.Handle(ctx, event); err != nil {
			errors = append(errors, err)
		}
	case ReloadTypeAsset:
		if err := m.assetHandler.Handle(ctx, event); err != nil {
			errors = append(errors, err)
		}
	default:
		m.logger.Warn("Unknown reload type", "type", event.Type)
	}

	// 汇总错误
	if len(errors) > 0 {
		return fmt.Errorf("reload handler errors: %v", errors)
	}

	return nil
}

// GetPluginHandler 获取插件处理器
func (m *HandlerManager) GetPluginHandler() *GoPluginHandler {
	return m.pluginHandler
}

// GetConfigHandler 获取配置处理器
func (m *HandlerManager) GetConfigHandler() *ConfigHandler {
	return m.configHandler
}

// GetScriptHandler 获取脚本处理器
func (m *HandlerManager) GetScriptHandler() *ScriptHandler {
	return m.scriptHandler
}

// GetAssetHandler 获取资源处理器
func (m *HandlerManager) GetAssetHandler() *AssetHandler {
	return m.assetHandler
}

// === 游戏特定的便捷函数 ===

// GameConfig 游戏配置结构
type GameConfig struct {
	Balance GameBalance `json:"balance" yaml:"balance"`
	Rules   GameRules   `json:"rules" yaml:"rules"`
}

type GameBalance struct {
	PlayerHP     int     `json:"player_hp" yaml:"player_hp"`
	EnemyDamage  int     `json:"enemy_damage" yaml:"enemy_damage"`
	CoinDropRate float64 `json:"coin_drop_rate" yaml:"coin_drop_rate"`
}

type GameRules struct {
	MaxPlayers   int  `json:"max_players" yaml:"max_players"`
	EnablePVP    bool `json:"enable_pvp" yaml:"enable_pvp"`
	MatchTimeout int  `json:"match_timeout" yaml:"match_timeout"`
}

// RegisterGameConfigHandler 注册游戏配置处理器
func RegisterGameConfigHandler(hr HotReloader, configPath string, onChange func(*GameConfig) error) error {
	return hr.RegisterHandler(configPath, func(ctx context.Context, event ReloadEvent) error {
		if event.Type != ReloadTypeConfig {
			return nil
		}

		var config GameConfig
		ext := filepath.Ext(event.Path)

		switch ext {
		case ".json":
			if err := json.Unmarshal(event.Content, &config); err != nil {
				return err
			}
		case ".yaml", ".yml":
			if err := yaml.Unmarshal(event.Content, &config); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unsupported config format: %s", ext)
		}

		return onChange(&config)
	})
}

// WatchGameAssets 监听游戏资源目录
func WatchGameAssets(hr HotReloader, assetsDir string, onAssetChange func(string, []byte) error) error {
	pattern := filepath.Join(assetsDir, "*")

	return hr.RegisterHandler(pattern, func(ctx context.Context, event ReloadEvent) error {
		if event.Type != ReloadTypeAsset {
			return nil
		}

		return onAssetChange(event.Path, event.Content)
	})
}
