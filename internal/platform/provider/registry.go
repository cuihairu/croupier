package provider

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
)

// Registry manages provider instances.
// It is safe for concurrent use.
type Registry struct {
	mu        sync.RWMutex
	providers map[string]Provider
	logger    *slog.Logger
}

// NewRegistry creates a new empty provider registry.
func NewRegistry(logger *slog.Logger) *Registry {
	if logger == nil {
		logger = slog.Default()
	}
	return &Registry{
		providers: make(map[string]Provider),
		logger:    logger,
	}
}

// Register registers a new provider.
// If a provider with the same name already exists, it returns an error.
func (r *Registry) Register(ctx context.Context, p Provider, config ProviderConfig) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	name := p.Name()
	if _, exists := r.providers[name]; exists {
		return fmt.Errorf("provider %q already registered", name)
	}

	// Initialize the provider
	if err := p.Init(ctx, config); err != nil {
		return fmt.Errorf("failed to initialize provider %q: %w", name, err)
	}

	r.providers[name] = p
	r.logger.Info("provider registered", "name", name, "enabled", config.Enabled, "methods", p.SupportedMethods())
	return nil
}

// Unregister removes a provider from the registry.
// It also calls Close() on the provider.
func (r *Registry) Unregister(ctx context.Context, name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	p, exists := r.providers[name]
	if !exists {
		return &ProviderNotFoundError{Name: name}
	}

	if err := p.Close(); err != nil {
		r.logger.Warn("error closing provider", "name", name, "error", err)
	}

	delete(r.providers, name)
	r.logger.Info("provider unregistered", "name", name)
	return nil
}

// Get returns a provider by name.
// Returns the provider and true if found, nil and false otherwise.
func (r *Registry) Get(name string) (Provider, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	p, exists := r.providers[name]
	return p, exists
}

// List returns all registered providers.
func (r *Registry) List() []Provider {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]Provider, 0, len(r.providers))
	for _, p := range r.providers {
		result = append(result, p)
	}
	return result
}

// ListNames returns the names of all registered providers.
func (r *Registry) ListNames() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]string, 0, len(r.providers))
	for name := range r.providers {
		result = append(result, name)
	}
	return result
}

// Call invokes a method on a named provider.
// It returns an error if the provider doesn't exist, is disabled, or doesn't support the method.
func (r *Registry) Call(ctx context.Context, platform, method string, request []byte) ([]byte, error) {
	p, exists := r.Get(platform)
	if !exists {
		return nil, &ProviderNotFoundError{Name: platform}
	}

	if !p.IsEnabled() {
		return nil, &ProviderDisabledError{Name: platform}
	}

	// Check if method is supported
	supported := false
	for _, m := range p.SupportedMethods() {
		if m == method {
			supported = true
			break
		}
	}
	if !supported {
		return nil, &MethodNotSupportedError{Provider: platform, Method: method}
	}

	return p.Call(ctx, method, request)
}

// Close closes all registered providers.
func (r *Registry) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	var firstErr error
	for name, p := range r.providers {
		if err := p.Close(); err != nil {
			r.logger.Warn("error closing provider", "name", name, "error", err)
			if firstErr == nil {
				firstErr = err
			}
		}
	}

	r.providers = make(map[string]Provider)
	return firstErr
}
