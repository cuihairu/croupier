// Package agent provides platform integration for Agent.
package agent

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"

	agentlocal "github.com/cuihairu/croupier/internal/platform/agentlocal"
	"github.com/cuihairu/croupier/internal/platform/openapi"
	"github.com/cuihairu/croupier/internal/platform/provider"
	localv1 "github.com/cuihairu/croupier/pkg/pb/croupier/agent/local/v1"
	"gopkg.in/yaml.v3"
)

// PlatformManager manages platform providers for the Agent.
// It loads platform configurations and registers their methods as Functions.
type PlatformManager struct {
	mu        sync.RWMutex
	providers map[string]provider.Provider // platform name -> Provider
	store     *agentlocal.LocalStore
	logger    *slog.Logger
	configDir string
}

// NewPlatformManager creates a new platform manager.
func NewPlatformManager(store *agentlocal.LocalStore, configDir string, logger *slog.Logger) *PlatformManager {
	if logger == nil {
		logger = slog.Default()
	}
	return &PlatformManager{
		providers: make(map[string]provider.Provider),
		store:     store,
		logger:    logger,
		configDir: configDir,
	}
}

// Load loads and initializes platform providers from configuration.
func (m *PlatformManager) Load(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	configPath := filepath.Join(m.configDir, "platforms.yaml")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		m.logger.Debug("platform config not found, skipping", "path", configPath)
		return nil
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to read platform config: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("failed to parse platform config: %w", err)
	}

	// Initialize enabled providers
	for name, entry := range config.Platforms {
		if !entry.Enabled {
			m.logger.Debug("platform disabled", "name", name)
			continue
		}

		if err := m.initProvider(ctx, name, entry); err != nil {
			m.logger.Error("failed to init platform", "name", name, "error", err)
			continue
		}
	}

	return nil
}

// initProvider initializes a single platform provider.
func (m *PlatformManager) initProvider(ctx context.Context, name string, entry ProviderEntry) error {
	var p provider.Provider

	switch entry.Type {
	case "openapi":
		p = openapi.NewProvider()
	default:
		return fmt.Errorf("unsupported provider type: %s", entry.Type)
	}

	// Build provider config
	providerConfig := provider.ProviderConfig{
		Enabled: entry.Enabled,
		Type:    entry.Type,
		Config:  m.expandEnvVars(entry.Config),
	}

	// Initialize provider
	if err := p.Init(ctx, providerConfig); err != nil {
		return fmt.Errorf("provider init failed: %w", err)
	}

	m.providers[name] = p

	// Register methods as functions in LocalStore
	methods := p.SupportedMethods()
	funcs := make([]*localv1.LocalFunctionDescriptor, 0, len(methods))

	// Try to get method details from OpenAPI provider (if available)
	var methodDetails map[string]*openapi.MethodDetails
	if openapiProvider, ok := p.(*openapi.Provider); ok {
		methodDetails = openapiProvider.GetMethodDetails()
	}

	for _, method := range methods {
		// Create function ID: platform.method
		funcID := fmt.Sprintf("%s.%s", name, method)

		// Create LocalFunctionDescriptor with OpenAPI-compatible fields
		desc := &localv1.LocalFunctionDescriptor{
			Id:      funcID,
			Version: "1.0.0",
		}

		// Fill in details from OpenAPI provider if available
		if methodDetails != nil {
			if details, exists := methodDetails[method]; exists {
				desc.Summary = details.Summary
				desc.Description = details.Description
				desc.Tags = details.Tags
				desc.Deprecated = details.Deprecated
				desc.OperationId = details.OperationID
			}
		}

		funcs = append(funcs, desc)
		m.logger.Debug("registering platform method", "function", funcID, "tags", desc.Tags, "summary", desc.Summary)
	}

	// Register all methods for this platform
	// Use "platform:" prefix in serviceID to identify platform functions
	serviceID := "platform:" + name
	m.store.Register(serviceID, "", "1.0.0", funcs)

	m.logger.Info("platform loaded", "name", name, "methods", len(methods))
	return nil
}

// expandEnvVars expands environment variables in config values.
func (m *PlatformManager) expandEnvVars(config map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	for k, v := range config {
		switch val := v.(type) {
		case string:
			result[k] = m.expandEnvString(val)
		case map[string]interface{}:
			result[k] = m.expandEnvVars(val)
		case []interface{}:
			// Handle arrays
			result[k] = val
		default:
			result[k] = val
		}
	}
	return result
}

// expandEnvString expands ${VAR} style environment variables.
func (m *PlatformManager) expandEnvString(s string) string {
	if len(s) > 4 && s[0:2] == "${" && s[len(s)-1] == '}' {
		envVar := s[2 : len(s)-1]
		if val := os.Getenv(envVar); val != "" {
			return val
		}
	}
	return os.ExpandEnv(s)
}

// Call invokes a platform method.
// The functionID should be in format "platform_name.method_name".
func (m *PlatformManager) Call(ctx context.Context, functionID string, request []byte) ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Parse functionID: "platform_name.method_name"
	idx := strings.Index(functionID, ".")
	if idx <= 0 || idx >= len(functionID)-1 {
		return nil, fmt.Errorf("invalid platform function ID: %s", functionID)
	}
	platformName := functionID[:idx]
	methodName := functionID[idx+1:]

	p, exists := m.providers[platformName]
	if !exists {
		return nil, fmt.Errorf("platform not found: %s", platformName)
	}

	return p.Call(ctx, methodName, request)
}

// IsPlatformFunction checks if a function ID is a platform function.
func (m *PlatformManager) IsPlatformFunction(functionID string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Parse functionID: "platform_name.method_name"
	idx := strings.Index(functionID, ".")
	if idx <= 0 || idx >= len(functionID)-1 {
		return false
	}
	platformName := functionID[:idx]

	_, exists := m.providers[platformName]
	return exists
}

// Close closes all platform providers.
func (m *PlatformManager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var lastErr error
	for name, p := range m.providers {
		if err := p.Close(); err != nil {
			m.logger.Error("failed to close platform", "name", name, "error", err)
			lastErr = err
		}
	}
	m.providers = make(map[string]provider.Provider)
	return lastErr
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
