// Package agent provides provider integration for Agent.
package agent

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	agentlocal "github.com/cuihairu/croupier/internal/platform/agentlocal"
	"github.com/cuihairu/croupier/internal/platform/openapi"
	"github.com/cuihairu/croupier/internal/platform/provider"
	sdkv1 "github.com/cuihairu/croupier/pkg/pb/croupier/sdk/v1"
	"gopkg.in/yaml.v3"
)

// ProviderManager manages platform providers for the Agent.
// It loads provider configurations and registers their methods as Functions.
type ProviderManager struct {
	mu               sync.RWMutex
	providers        map[string]provider.Provider // provider name -> Provider
	providerIDs      map[string]struct{}
	extensionNames   map[string]struct{}
	staticNames      map[string]struct{}
	extensionOnly    bool
	overrideStatic   bool
	store            *agentlocal.LocalStore
	logger           *slog.Logger
	configDir        string
	heartbeatStarted bool
}

// NewProviderManager creates a new provider manager.
func NewProviderManager(store *agentlocal.LocalStore, configDir string, logger *slog.Logger) *ProviderManager {
	if logger == nil {
		logger = slog.Default()
	}
	return &ProviderManager{
		providers:      make(map[string]provider.Provider),
		providerIDs:    make(map[string]struct{}),
		extensionNames: make(map[string]struct{}),
		staticNames:    make(map[string]struct{}),
		extensionOnly:  envBool("CROUPIER_EXTENSION_PROVIDERS_ONLY"),
		overrideStatic: envBool("CROUPIER_EXTENSION_PROVIDER_OVERRIDE_STATIC"),
		store:          store,
		logger:         logger,
		configDir:      configDir,
	}
}

// Load loads and initializes provider from configuration.
func (m *ProviderManager) Load(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.extensionOnly {
		m.logger.Info("skip loading static providers.yaml in extension-only mode")
		return nil
	}

	configPath := filepath.Join(m.configDir, "providers.yaml")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		m.logger.Debug("provider config not found, skipping", "path", configPath)
		return nil
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to read provider config: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("failed to parse provider config: %w", err)
	}

	// Initialize enabled providers
	for name, entry := range config.Providers {
		if !entry.Enabled {
			m.logger.Debug("provider disabled", "name", name)
			continue
		}

		if err := m.initProvider(ctx, name, entry); err != nil {
			m.logger.Error("failed to init provider", "name", name, "error", err)
			continue
		}
		m.staticNames[name] = struct{}{}
	}

	m.startProviderHeartbeat(ctx)
	return nil
}

// initProvider initializes a single provider.
func (m *ProviderManager) initProvider(ctx context.Context, name string, entry ProviderEntry) error {
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
	funcs := make([]*sdkv1.LocalFunctionDescriptor, 0, len(methods))

	// Try to get method details from OpenAPI provider (if available)
	var methodDetails map[string]*openapi.MethodDetails
	if openapiProvider, ok := p.(*openapi.Provider); ok {
		methodDetails = openapiProvider.GetMethodDetails()
	}

	for _, method := range methods {
		// Create function ID: provider.method
		funcID := fmt.Sprintf("%s.%s", name, method)

		// Create LocalFunctionDescriptor with OpenAPI-compatible fields
		desc := &sdkv1.LocalFunctionDescriptor{
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
				desc.Risk = details.Risk
				desc.Resource = details.Resource
				desc.Operation = details.Operation
				desc.Capability = details.Capability
				desc.Execution = details.Execution
			}
		}

		funcs = append(funcs, desc)
		m.logger.Debug("registering provider method", "function", funcID, "tags", desc.Tags, "summary", desc.Summary)
	}

	// Register all methods for this provider
	// Use "provider:" prefix in serviceID to identify provider functions
	serviceID := "provider:" + name
	if m.providerIDs != nil {
		m.providerIDs[serviceID] = struct{}{}
	}

	// Collect function IDs for logging
	functionIDs := make([]string, 0, len(funcs))
	for _, fn := range funcs {
		if fn != nil && fn.Id != "" {
			functionIDs = append(functionIDs, fn.Id)
		}
	}

	// Log provider session info (for debugging)
	m.logger.Debug("provider session",
		"provider_id", serviceID,
		"game_id", entry.GameID,
		"env", entry.Env,
		"functions", len(functionIDs))

	// 注册：providerID=serviceID, serviceID=serviceID, addr=""（临时）
	m.store.Register(serviceID, serviceID, "", "1.0.0", funcs, nil)

	m.logger.Info("provider loaded", "name", name, "methods", len(methods))
	return nil
}

// SyncExtensionProviders replaces extension-managed providers with the provided set.
// It does not touch providers loaded from static providers.yaml unless they are already extension-managed.
func (m *ProviderManager) SyncExtensionProviders(ctx context.Context, entries map[string]ProviderEntry) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	target := map[string]ProviderEntry{}
	for name, entry := range entries {
		key := strings.TrimSpace(name)
		if key == "" {
			continue
		}
		target[key] = entry
	}

	for name := range m.extensionNames {
		if _, keep := target[name]; keep {
			continue
		}
		if p, ok := m.providers[name]; ok {
			_ = p.Close()
			delete(m.providers, name)
		}
		serviceID := "provider:" + name
		delete(m.providerIDs, serviceID)
		// Cleanup previous function registrations for this provider.
		m.store.RemoveProvider(serviceID)
		delete(m.extensionNames, name)
	}

	for name, entry := range target {
		if !entry.Enabled {
			continue
		}
		if _, exists := m.providers[name]; exists {
			if _, managed := m.extensionNames[name]; managed {
				if p := m.providers[name]; p != nil {
					_ = p.Close()
				}
				delete(m.providers, name)
				delete(m.extensionNames, name)
				delete(m.staticNames, name)
			} else if m.overrideStatic {
				if p := m.providers[name]; p != nil {
					_ = p.Close()
				}
				delete(m.providers, name)
				delete(m.staticNames, name)
			} else {
				m.logger.Warn("skip extension provider due to name conflict with static provider", "name", name)
				continue
			}
		}
		if err := m.initProvider(ctx, name, entry); err != nil {
			m.logger.Error("failed to init extension provider", "name", name, "error", err)
			continue
		}
		m.extensionNames[name] = struct{}{}
	}
	return nil
}

func envBool(key string) bool {
	val := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
	return val == "1" || val == "true" || val == "yes" || val == "on"
}

func (m *ProviderManager) startProviderHeartbeat(ctx context.Context) {
	if m == nil || m.store == nil || m.heartbeatStarted || len(m.providerIDs) == 0 {
		return
	}
	m.heartbeatStarted = true
	interval := parseDurationEnv("CROUPIER_AGENTLOCAL_PROVIDER_HEARTBEAT", 30*time.Second)
	if interval <= 0 {
		interval = 30 * time.Second
	}
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				for id := range m.providerIDs {
					m.store.Heartbeat(id)
				}
			}
		}
	}()
}

// expandEnvVars expands environment variables in config values.
func (m *ProviderManager) expandEnvVars(config map[string]interface{}) map[string]interface{} {
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
func (m *ProviderManager) expandEnvString(s string) string {
	if len(s) > 4 && s[0:2] == "${" && s[len(s)-1] == '}' {
		envVar := s[2 : len(s)-1]
		if val := os.Getenv(envVar); val != "" {
			return val
		}
	}
	return os.ExpandEnv(s)
}

// Call invokes a provider method.
// The functionID should be in format "provider_name.method_name".
func (m *ProviderManager) Call(ctx context.Context, functionID string, request []byte) ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Parse functionID: "provider_name.method_name"
	idx := strings.Index(functionID, ".")
	if idx <= 0 || idx >= len(functionID)-1 {
		return nil, fmt.Errorf("invalid provider function ID: %s", functionID)
	}
	providerName := functionID[:idx]
	methodName := functionID[idx+1:]

	p, exists := m.providers[providerName]
	if !exists {
		return nil, fmt.Errorf("provider not found: %s", providerName)
	}

	return p.Call(ctx, methodName, request)
}

// IsPlatformFunction checks if a function ID is a provider function.
func (m *ProviderManager) IsPlatformFunction(functionID string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Parse functionID: "provider_name.method_name"
	idx := strings.Index(functionID, ".")
	if idx <= 0 || idx >= len(functionID)-1 {
		return false
	}
	providerName := functionID[:idx]

	_, exists := m.providers[providerName]
	return exists
}

// Close closes all provider.
func (m *ProviderManager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var lastErr error
	for name, p := range m.providers {
		if err := p.Close(); err != nil {
			m.logger.Error("failed to close provider", "name", name, "error", err)
			lastErr = err
		}
	}
	m.providers = make(map[string]provider.Provider)
	return lastErr
}

// Config represents the providers configuration file structure.
type Config struct {
	Providers map[string]ProviderEntry `yaml:"providers"`
}

// ProviderEntry represents a single provider entry in the config.
type ProviderEntry struct {
	Enabled bool                   `yaml:"enabled"`
	Type    string                 `yaml:"type"`
	GameID  string                 `yaml:"game_id"` // Game ID for game/environment scoping
	Env     string                 `yaml:"env"`     // Logical environment (prod/dev/staging)
	Config  map[string]interface{} `yaml:"config"`
}
