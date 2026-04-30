// Package api provides REST API handlers for function metadata management.
package api

import (
	"context"
	"errors"
	"fmt"

	"github.com/cuihairu/croupier/internal/function/openapi"
	"github.com/cuihairu/croupier/internal/function/registry"
	functionv1 "github.com/cuihairu/croupier/pkg/pb/croupier/function/v1"
)

var (
	// ErrNotFound is returned when a function is not found.
	ErrNotFound = errors.New("function not found")
)

// Service provides business logic for function metadata management.
type Service struct {
	store *registry.Store
}

// NewService creates a new function metadata service.
func NewService(store *registry.Store) *Service {
	return &Service{
		store: store,
	}
}

// ListOptions defines filtering options for listing functions.
type ListOptions struct {
	Category  string
	Tag       string
	RiskLevel string
	Mode      string
}

// ListResult contains the result of listing functions.
type ListResult struct {
	Functions []*functionv1.FunctionMetadata
	Total     int
}

// List returns all functions matching the given filters.
func (s *Service) List(ctx context.Context, opts *ListOptions) (*ListResult, error) {
	// Build filter
	filter := &functionv1.FunctionFilter{}
	if opts.Category != "" {
		filter.Category = opts.Category
	}
	if opts.Tag != "" {
		filter.Tags = []string{opts.Tag}
	}
	if opts.RiskLevel != "" {
		filter.RiskLevel = normalizeRiskLevelForStore(opts.RiskLevel)
	}
	if opts.Mode != "" {
		filter.Mode = normalizeModeForStore(opts.Mode)
	}

	functions, err := s.store.Filter(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("filter functions: %w", err)
	}

	return &ListResult{
		Functions: functions,
		Total:     len(functions),
	}, nil
}

// Get returns a function by ID.
func (s *Service) Get(ctx context.Context, id string) (*functionv1.FunctionMetadata, error) {
	metadata, err := s.store.Get(ctx, id)
	if err != nil {
		return nil, ErrNotFound
	}
	return metadata, nil
}

// Register registers a new function.
func (s *Service) Register(ctx context.Context, metadata *functionv1.FunctionMetadata) error {
	return s.store.Register(ctx, metadata)
}

// RegisterBatch registers multiple functions.
func (s *Service) RegisterBatch(ctx context.Context, metadatas []*functionv1.FunctionMetadata) error {
	return s.store.RegisterBatch(ctx, metadatas)
}

// Update updates an existing function.
func (s *Service) Update(ctx context.Context, id string, metadata *functionv1.FunctionMetadata) error {
	// Ensure ID matches
	if metadata.Id != id {
		return fmt.Errorf("ID mismatch: %s != %s", metadata.Id, id)
	}

	// Check if exists
	if !s.store.Exists(ctx, id) {
		return ErrNotFound
	}

	return s.store.Register(ctx, metadata)
}

// Delete removes a function.
func (s *Service) Delete(ctx context.Context, id string) error {
	if !s.store.Exists(ctx, id) {
		return ErrNotFound
	}
	return s.store.Unregister(ctx, id)
}

// ImportFromOpenAPI imports functions from an OpenAPI specification.
func (s *Service) ImportFromOpenAPI(ctx context.Context, specData []byte, opts *ImportOptions) ([]*functionv1.FunctionMetadata, error) {
	// Use OpenAPI converter
	converter := openapi.NewConverter()

	var importOpts *openapi.ImportOptions
	if opts != nil {
		importOpts = &openapi.ImportOptions{
			CategoryPrefix:   opts.CategoryPrefix,
			TagPrefix:        opts.TagPrefix,
			DefaultTimeoutMs: opts.DefaultTimeoutMs,
			ContinueOnError:  opts.ContinueOnError,
		}
	}

	metadatas, err := converter.ImportFromSpecData(specData, importOpts)
	if err != nil {
		return nil, fmt.Errorf("import OpenAPI spec: %w", err)
	}

	// Register all functions
	for _, metadata := range metadatas {
		if err := s.store.Register(ctx, metadata); err != nil {
			return nil, fmt.Errorf("register function %s: %w", metadata.Id, err)
		}
	}

	return metadatas, nil
}

// GetCategories returns all unique categories.
func (s *Service) GetCategories(ctx context.Context) []string {
	return s.store.GetCategories(ctx)
}

// GetTags returns all unique tags.
func (s *Service) GetTags(ctx context.Context) []string {
	return s.store.GetTags(ctx)
}

// Helper functions for value normalization

func normalizeRiskLevelForStore(level string) string {
	switch level {
	case "low", "LOW", "RISK_LOW", "RISK_LEVEL_LOW":
		return "low"
	case "medium", "MEDIUM", "RISK_MEDIUM", "RISK_LEVEL_MEDIUM":
		return "medium"
	case "high", "HIGH", "RISK_HIGH", "RISK_LEVEL_HIGH":
		return "high"
	case "danger", "DANGER", "RISK_DANGER", "RISK_LEVEL_DANGER":
		return "danger"
	default:
		return level
	}
}

func normalizeModeForStore(mode string) string {
	switch mode {
	case "query", "QUERY", "MODE_QUERY":
		return "query"
	case "command", "COMMAND", "MODE_COMMAND":
		return "command"
	default:
		return mode
	}
}
