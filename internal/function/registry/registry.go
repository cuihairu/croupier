package registry

import (
	"context"
	"fmt"
	"log/slog"

	functionv1 "github.com/cuihairu/croupier/pkg/pb/croupier/function/v1"
)

// Registry provides high-level function registration and query API.
type Registry struct {
	store *Store
}

// New creates a new function registry.
func New() *Registry {
	return &Registry{
		store: NewStore(),
	}
}

// NewWithStore creates a new function registry with a custom store.
func NewWithStore(store *Store) *Registry {
	return &Registry{
		store: store,
	}
}

// Register registers a single function metadata.
func (r *Registry) Register(ctx context.Context, metadata *functionv1.FunctionMetadata) error {
	if err := r.validateMetadata(metadata); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}
	return r.store.Register(ctx, metadata)
}

// RegisterBatch registers multiple function metadatas in a single transaction.
func (r *Registry) RegisterBatch(ctx context.Context, list *functionv1.FunctionMetadataList) error {
	if list == nil {
		return fmt.Errorf("metadata list is required")
	}
	for _, metadata := range list.Functions {
		if err := r.validateMetadata(metadata); err != nil {
			return fmt.Errorf("validation failed for %s: %w", metadata.Id, err)
		}
	}
	return r.store.RegisterBatch(ctx, list.Functions)
}

// Get retrieves a function by ID.
func (r *Registry) Get(ctx context.Context, id string) (*functionv1.FunctionMetadata, error) {
	return r.store.Get(ctx, id)
}

// List retrieves all functions.
func (r *Registry) List(ctx context.Context) ([]*functionv1.FunctionMetadata, error) {
	return r.store.List(ctx)
}

// ListByCategory retrieves functions by category.
func (r *Registry) ListByCategory(ctx context.Context, category string) ([]*functionv1.FunctionMetadata, error) {
	return r.store.ListByCategory(ctx, category)
}

// ListByTag retrieves functions by tag.
func (r *Registry) ListByTag(ctx context.Context, tag string) ([]*functionv1.FunctionMetadata, error) {
	return r.store.ListByTag(ctx, tag)
}

// ListByRiskLevel retrieves functions by risk level.
func (r *Registry) ListByRiskLevel(ctx context.Context, riskLevel string) ([]*functionv1.FunctionMetadata, error) {
	return r.store.ListByRiskLevel(ctx, riskLevel)
}

// ListByMode retrieves functions by execution mode.
func (r *Registry) ListByMode(ctx context.Context, mode functionv1.FunctionBehavior_Mode) ([]*functionv1.FunctionMetadata, error) {
	return r.store.ListByMode(ctx, mode)
}

// Filter retrieves functions by filter criteria.
func (r *Registry) Filter(ctx context.Context, filter *functionv1.FunctionFilter) ([]*functionv1.FunctionMetadata, string, error) {
	metadatas, err := r.store.Filter(ctx, filter)
	if err != nil {
		return nil, "", err
	}

	// Simple pagination - in production, use cursor-based pagination
	nextPageToken := ""
	if filter != nil && filter.PageSize > 0 && len(metadatas) == int(filter.PageSize) {
		nextPageToken = fmt.Sprintf("page_%d", 1)
	}

	return metadatas, nextPageToken, nil
}

// Unregister removes a function from the registry.
func (r *Registry) Unregister(ctx context.Context, id string) error {
	return r.store.Unregister(ctx, id)
}

// Exists checks if a function is registered.
func (r *Registry) Exists(ctx context.Context, id string) bool {
	return r.store.Exists(ctx, id)
}

// Count returns the total number of registered functions.
func (r *Registry) Count(ctx context.Context) int {
	return r.store.Count(ctx)
}

// GetCategories returns all unique categories.
func (r *Registry) GetCategories(ctx context.Context) []string {
	return r.store.GetCategories(ctx)
}

// GetTags returns all unique tags.
func (r *Registry) GetTags(ctx context.Context) []string {
	return r.store.GetTags(ctx)
}

// validateMetadata validates function metadata before registration.
func (r *Registry) validateMetadata(metadata *functionv1.FunctionMetadata) error {
	if metadata == nil {
		return fmt.Errorf("metadata is required")
	}

	if metadata.Id == "" {
		return fmt.Errorf("function ID is required")
	}

	// Validate ID format: <domain>.<entity>.<action>
	// Allow flexible formats but require at least 2 parts
	parts := splitID(metadata.Id)
	if len(parts) < 2 {
		return fmt.Errorf("invalid function ID format: %s (expected <domain>.<entity>[.<action>])", metadata.Id)
	}

	// Validate version if provided
	if metadata.Version != "" && !isValidSemVer(metadata.Version) {
		slog.Warn("Function version may not be valid semver", "function_id", metadata.Id, "version", metadata.Version)
	}

	// Validate security
	if metadata.Security == nil {
		return fmt.Errorf("security configuration is required")
	}

	// Validate behavior
	if metadata.Behavior == nil {
		return fmt.Errorf("behavior configuration is required")
	}

	return nil
}

// splitID splits a function ID into parts.
func splitID(id string) []string {
	return splitAndTrim(id, ".")
}

// splitAndTrim splits a string by separator and trims each part.
func splitAndTrim(s, sep string) []string {
	parts := make([]string, 0)
	for _, part := range splitString(s, sep) {
		trimmed := trimSpace(part)
		if trimmed != "" {
			parts = append(parts, trimmed)
		}
	}
	return parts
}

func splitString(s, sep string) []string {
	if s == "" {
		return []string{}
	}
	result := make([]string, 0)
	current := ""
	for i := 0; i < len(s); i++ {
		if i+len(sep) <= len(s) && s[i:i+len(sep)] == sep {
			result = append(result, current)
			current = ""
			i += len(sep) - 1
		} else {
			current += string(s[i])
		}
	}
	result = append(result, current)
	return result
}

func trimSpace(s string) string {
	start := 0
	end := len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t' || s[start] == '\n' || s[start] == '\r') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t' || s[end-1] == '\n' || s[end-1] == '\r') {
		end--
	}
	return s[start:end]
}

// isValidSemVer checks if a version string is valid semver.
func isValidSemVer(version string) bool {
	// Basic semver validation: major.minor.patch
	// For simplicity, just check format has at least major.minor
	parts := splitAndTrim(version, ".")
	if len(parts) < 2 {
		return false
	}
	// Check if first parts are numeric
	for i := 0; i < 2 && i < len(parts); i++ {
		if !isNumeric(parts[i]) {
			return false
		}
	}
	return true
}

func isNumeric(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// GetStore returns the underlying store for advanced operations.
func (r *Registry) GetStore() *Store {
	return r.store
}
