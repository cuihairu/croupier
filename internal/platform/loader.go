// Package platform provides third-party platform integration support.
package platform

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sync"

	"github.com/cuihairu/croupier/internal/platform/openapi"
	"github.com/cuihairu/croupier/internal/platform/provider"
	"github.com/cuihairu/croupier/internal/platform/quicksdk"
	"gopkg.in/yaml.v3"
)

// Loader loads and manages platform providers from configuration.
type Loader struct {
	registry   *provider.Registry
	providers  map[string]provider.Provider
	configPath string
	logger     *slog.Logger
	mu         sync.RWMutex
}

// Config represents the platforms configuration file structure.
type Config struct {
	Platforms map[string]ProviderEntry `yaml:"platforms"`
}

// ProviderEntry represents a single provider entry in the config.
type ProviderEntry struct {
	Enabled bool                   `yaml:"enabled"`
	Type    string                 `yaml:"type"`
	Config  map[string]interface{} `yaml:"config"`
}

// NewLoader creates a new platform loader.
func NewLoader(configPath string, logger *slog.Logger) *Loader {
	if logger == nil {
		logger = slog.Default()
	}
	return &Loader{
		registry:   provider.NewRegistry(logger),
		providers:  make(map[string]provider.Provider),
		configPath: configPath,
		logger:     logger,
	}
}

// Load loads providers from the configuration file.
func (l *Loader) Load(ctx context.Context) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	// Read config file
	data, err := os.ReadFile(l.configPath)
	if err != nil {
		if os.IsNotExist(err) {
			l.logger.Warn("platform config file not found, using defaults", "path", l.configPath)
			return nil
		}
		return fmt.Errorf("failed to read config file: %w", err)
	}

	// Parse config
	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("failed to parse config: %w", err)
	}

	// Load providers
	for name, entry := range config.Platforms {
		if !entry.Enabled {
			l.logger.Debug("provider disabled in config", "name", name)
			continue
		}

		// Create provider instance based on type
		var p provider.Provider
		switch entry.Type {
		case "quicksdk":
			p = quicksdk.NewProvider(l.logger)
		case "openapi":
			p = openapi.NewProvider()
		default:
			l.logger.Warn("unknown provider type", "name", name, "type", entry.Type)
			continue
		}

		// Build provider config
		providerConfig := provider.ProviderConfig{
			Enabled: entry.Enabled,
			Type:    entry.Type,
			Config:  entry.Config,
		}

		// Expand environment variables in config
		providerConfig.Config = l.expandEnvVars(providerConfig.Config)

		// Register provider
		if err := l.registry.Register(ctx, p, providerConfig); err != nil {
			l.logger.Error("failed to register provider", "name", name, "error", err)
			continue
		}

		l.providers[name] = p
		l.logger.Info("provider loaded", "name", name, "type", entry.Type)
	}

	return nil
}

// expandEnvVars expands environment variables in config values.
func (l *Loader) expandEnvVars(config map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	for k, v := range config {
		switch val := v.(type) {
		case string:
			result[k] = l.expandEnvString(val)
		case map[string]interface{}:
			result[k] = l.expandEnvVars(val)
		case []interface{}:
			// Handle arrays if needed
			result[k] = val
		default:
			result[k] = val
		}
	}
	return result
}

// expandEnvString expands ${VAR} style environment variables.
func (l *Loader) expandEnvString(s string) string {
	if len(s) > 4 && s[0:2] == "${" && s[len(s)-1] == '}' {
		envVar := s[2 : len(s)-1]
		if val := os.Getenv(envVar); val != "" {
			return val
		}
	}
	return os.ExpandEnv(s)
}

// Registry returns the provider registry.
func (l *Loader) Registry() *provider.Registry {
	return l.registry
}

// GetProvider returns a provider by name.
func (l *Loader) GetProvider(name string) (provider.Provider, bool) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	p, exists := l.providers[name]
	return p, exists
}

// ListProviders returns all loaded providers.
func (l *Loader) ListProviders() []provider.Provider {
	l.mu.RLock()
	defer l.mu.RUnlock()
	result := make([]provider.Provider, 0, len(l.providers))
	for _, p := range l.providers {
		result = append(result, p)
	}
	return result
}

// Reload reloads the configuration.
func (l *Loader) Reload(ctx context.Context) error {
	// Unregister existing providers
	l.mu.Lock()
	for name := range l.providers {
		if err := l.registry.Unregister(ctx, name); err != nil {
			l.logger.Warn("failed to unregister provider during reload", "name", name, "error", err)
		}
		delete(l.providers, name)
	}
	l.mu.Unlock()

	// Reload config
	return l.Load(ctx)
}

// Close closes all providers.
func (l *Loader) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.registry.Close()
}
