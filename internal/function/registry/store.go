package registry

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/cuihairu/croupier/internal/function/registrationguard"
	functionv1 "github.com/cuihairu/croupier/pkg/pb/croupier/function/v1"
	"google.golang.org/protobuf/proto"
)

// Store keeps function metadata in-memory with indexing.
type Store struct {
	mu        sync.RWMutex
	functions map[string]*functionv1.FunctionMetadata // id -> metadata

	// Indexes
	byResource map[string]map[string]struct{} // resource -> set of function ids
	byTag      map[string]map[string]struct{} // tag -> set of function ids
	byRisk     map[string]map[string]struct{} // risk_level -> set of function ids
	byMode     map[string]map[string]struct{} // mode -> set of function ids

	// Lifecycle tracking
	createdAt map[string]time.Time // id -> registration time
	updatedAt map[string]time.Time // id -> last update time
}

// NewStore creates a new function registry store.
func NewStore() *Store {
	return &Store{
		functions:  make(map[string]*functionv1.FunctionMetadata),
		byResource: make(map[string]map[string]struct{}),
		byTag:      make(map[string]map[string]struct{}),
		byRisk:     make(map[string]map[string]struct{}),
		byMode:     make(map[string]map[string]struct{}),
		createdAt:  make(map[string]time.Time),
		updatedAt:  make(map[string]time.Time),
	}
}

// Register stores a function metadata in the registry.
// If a function with the same ID exists, it will be updated.
func (s *Store) Register(ctx context.Context, metadata *functionv1.FunctionMetadata) error {
	if metadata == nil {
		return fmt.Errorf("metadata is required")
	}
	if metadata.Id == "" {
		return fmt.Errorf("function ID is required")
	}
	if violation, ok := registrationguard.FindPresentationViolation(metadata.Extensions, metadata.InputSchema, metadata.OutputSchema); ok {
		return fmt.Errorf("function registration contains forbidden presentation field %q at %s", violation.Field, violation.Location)
	}

	// Clone the metadata to avoid external modifications
	cloned, err := cloneMetadata(metadata)
	if err != nil {
		return fmt.Errorf("clone metadata failed: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	existing := s.functions[metadata.Id]

	// Update indexes
	if existing != nil {
		s.removeFromIndexes(metadata.Id, existing)
	}

	s.functions[metadata.Id] = cloned
	s.addToIndexes(metadata.Id, cloned)

	if existing == nil {
		s.createdAt[metadata.Id] = now
	}
	s.updatedAt[metadata.Id] = now

	slog.Debug("Function registered",
		"function_id", metadata.Id,
		"version", metadata.Version,
		"resource", metadata.Resource,
		"action", map[bool]string{true: "updated", false: "created"}[existing != nil])

	return nil
}

// RegisterBatch stores multiple function metadatas in a single operation.
func (s *Store) RegisterBatch(ctx context.Context, metadatas []*functionv1.FunctionMetadata) error {
	for _, metadata := range metadatas {
		if err := s.Register(ctx, metadata); err != nil {
			return fmt.Errorf("register %s failed: %w", metadata.Id, err)
		}
	}
	return nil
}

// Get retrieves a function metadata by ID.
func (s *Store) Get(ctx context.Context, id string) (*functionv1.FunctionMetadata, error) {
	if id == "" {
		return nil, fmt.Errorf("function ID is required")
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	metadata, exists := s.functions[id]
	if !exists {
		return nil, fmt.Errorf("function not found: %s", id)
	}

	return cloneMetadata(metadata)
}

// List retrieves all function metadatas.
func (s *Store) List(ctx context.Context) ([]*functionv1.FunctionMetadata, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*functionv1.FunctionMetadata, 0, len(s.functions))
	for _, metadata := range s.functions {
		cloned, err := cloneMetadata(metadata)
		if err != nil {
			slog.Warn("Failed to clone metadata", "function_id", metadata.Id, "error", err)
			continue
		}
		result = append(result, cloned)
	}

	return result, nil
}

// ListByResource retrieves function metadatas by resource.
func (s *Store) ListByResource(ctx context.Context, resource string) ([]*functionv1.FunctionMetadata, error) {
	if resource == "" {
		return s.List(ctx)
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	ids, exists := s.byResource[resource]
	if !exists {
		return []*functionv1.FunctionMetadata{}, nil
	}

	result := make([]*functionv1.FunctionMetadata, 0, len(ids))
	for id := range ids {
		if metadata, ok := s.functions[id]; ok {
			cloned, err := cloneMetadata(metadata)
			if err != nil {
				continue
			}
			result = append(result, cloned)
		}
	}

	return result, nil
}

// ListByTag retrieves function metadatas by tag.
func (s *Store) ListByTag(ctx context.Context, tag string) ([]*functionv1.FunctionMetadata, error) {
	if tag == "" {
		return nil, fmt.Errorf("tag is required")
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	ids, exists := s.byTag[tag]
	if !exists {
		return []*functionv1.FunctionMetadata{}, nil
	}

	result := make([]*functionv1.FunctionMetadata, 0, len(ids))
	for id := range ids {
		if metadata, ok := s.functions[id]; ok {
			cloned, err := cloneMetadata(metadata)
			if err != nil {
				continue
			}
			result = append(result, cloned)
		}
	}

	return result, nil
}

// ListByRiskLevel retrieves function metadatas by risk level.
func (s *Store) ListByRiskLevel(ctx context.Context, riskLevel string) ([]*functionv1.FunctionMetadata, error) {
	if riskLevel == "" {
		return nil, fmt.Errorf("risk level is required")
	}

	// Normalize the input risk level to match index format
	normalizedLevel := normalizeEnumName(riskLevel)

	s.mu.RLock()
	defer s.mu.RUnlock()

	ids, exists := s.byRisk[normalizedLevel]
	if !exists {
		return []*functionv1.FunctionMetadata{}, nil
	}

	result := make([]*functionv1.FunctionMetadata, 0, len(ids))
	for id := range ids {
		if metadata, ok := s.functions[id]; ok {
			cloned, err := cloneMetadata(metadata)
			if err != nil {
				continue
			}
			result = append(result, cloned)
		}
	}

	return result, nil
}

// ListByMode retrieves function metadatas by execution mode.
func (s *Store) ListByMode(ctx context.Context, mode functionv1.FunctionBehavior_Mode) ([]*functionv1.FunctionMetadata, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	modeStr := normalizeEnumName(mode.String())
	ids, exists := s.byMode[modeStr]
	if !exists {
		return []*functionv1.FunctionMetadata{}, nil
	}

	result := make([]*functionv1.FunctionMetadata, 0, len(ids))
	for id := range ids {
		if metadata, ok := s.functions[id]; ok {
			cloned, err := cloneMetadata(metadata)
			if err != nil {
				continue
			}
			result = append(result, cloned)
		}
	}

	return result, nil
}

// Filter retrieves function metadatas by filter criteria.
func (s *Store) Filter(ctx context.Context, filter *functionv1.FunctionFilter) ([]*functionv1.FunctionMetadata, error) {
	if filter == nil {
		return s.List(ctx)
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	// Start with all functions, then filter
	candidates := make(map[string]struct{})
	for id := range s.functions {
		candidates[id] = struct{}{}
	}

	// Filter by resource
	if filter.Resource != "" {
		if ids, exists := s.byResource[filter.Resource]; exists {
			candidates = intersectStringSets(candidates, ids)
		} else {
			return []*functionv1.FunctionMetadata{}, nil
		}
	}

	// Filter by tags
	if len(filter.Tags) > 0 {
		for _, tag := range filter.Tags {
			if ids, exists := s.byTag[tag]; exists {
				candidates = intersectStringSets(candidates, ids)
			} else {
				return []*functionv1.FunctionMetadata{}, nil
			}
		}
	}

	// Filter by risk level
	if filter.RiskLevel != "" {
		if ids, exists := s.byRisk[filter.RiskLevel]; exists {
			candidates = intersectStringSets(candidates, ids)
		} else {
			return []*functionv1.FunctionMetadata{}, nil
		}
	}

	// Filter by mode
	if filter.Mode != "" {
		if ids, exists := s.byMode[filter.Mode]; exists {
			candidates = intersectStringSets(candidates, ids)
		} else {
			return []*functionv1.FunctionMetadata{}, nil
		}
	}

	// Build result
	result := make([]*functionv1.FunctionMetadata, 0, len(candidates))
	for id := range candidates {
		if metadata, ok := s.functions[id]; ok {
			cloned, err := cloneMetadata(metadata)
			if err != nil {
				continue
			}
			result = append(result, cloned)
		}
	}

	// Apply pagination
	if filter.PageSize > 0 && len(result) > int(filter.PageSize) {
		result = result[:filter.PageSize]
	}

	return result, nil
}

// Unregister removes a function metadata from the registry.
func (s *Store) Unregister(ctx context.Context, id string) error {
	if id == "" {
		return fmt.Errorf("function ID is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	metadata, exists := s.functions[id]
	if !exists {
		return fmt.Errorf("function not found: %s", id)
	}

	s.removeFromIndexes(id, metadata)
	delete(s.functions, id)
	delete(s.createdAt, id)
	delete(s.updatedAt, id)

	slog.Debug("Function unregistered", "function_id", id)
	return nil
}

// Exists checks if a function is registered.
func (s *Store) Exists(ctx context.Context, id string) bool {
	if id == "" {
		return false
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	_, exists := s.functions[id]
	return exists
}

// Count returns the number of registered functions.
func (s *Store) Count(ctx context.Context) int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.functions)
}

// GetResources returns all unique resources.
func (s *Store) GetResources(ctx context.Context) []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	resources := make([]string, 0, len(s.byResource))
	for resource := range s.byResource {
		resources = append(resources, resource)
	}
	return resources
}

// GetTags returns all unique tags.
func (s *Store) GetTags(ctx context.Context) []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	tags := make([]string, 0, len(s.byTag))
	for tag := range s.byTag {
		tags = append(tags, tag)
	}
	return tags
}

// GetCreatedAt returns the registration time of a function.
func (s *Store) GetCreatedAt(ctx context.Context, id string) (time.Time, error) {
	if id == "" {
		return time.Time{}, fmt.Errorf("function ID is required")
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	t, exists := s.createdAt[id]
	if !exists {
		return time.Time{}, fmt.Errorf("function not found: %s", id)
	}

	return t, nil
}

// GetUpdatedAt returns the last update time of a function.
func (s *Store) GetUpdatedAt(ctx context.Context, id string) (time.Time, error) {
	if id == "" {
		return time.Time{}, fmt.Errorf("function ID is required")
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	t, exists := s.updatedAt[id]
	if !exists {
		return time.Time{}, fmt.Errorf("function not found: %s", id)
	}

	return t, nil
}

// addToIndexes adds a function to all indexes.
func (s *Store) addToIndexes(id string, metadata *functionv1.FunctionMetadata) {
	// Resource index
	if metadata.Resource != "" {
		if s.byResource[metadata.Resource] == nil {
			s.byResource[metadata.Resource] = make(map[string]struct{})
		}
		s.byResource[metadata.Resource][id] = struct{}{}
	}

	// Tag index
	for _, tag := range metadata.Tags {
		if s.byTag[tag] == nil {
			s.byTag[tag] = make(map[string]struct{})
		}
		s.byTag[tag][id] = struct{}{}
	}

	// Risk level index - normalize enum names: RISK_LOW -> low
	if metadata.Security != nil {
		riskStr := normalizeEnumName(metadata.Security.RiskLevel.String())
		if s.byRisk[riskStr] == nil {
			s.byRisk[riskStr] = make(map[string]struct{})
		}
		s.byRisk[riskStr][id] = struct{}{}
	}

	// Mode index - normalize enum names: MODE_QUERY -> query, MODE_UNKNOWN -> unknown
	if metadata.Behavior != nil {
		modeStr := normalizeEnumName(metadata.Behavior.Mode.String())
		if s.byMode[modeStr] == nil {
			s.byMode[modeStr] = make(map[string]struct{})
		}
		s.byMode[modeStr][id] = struct{}{}
	}
}

// removeFromIndexes removes a function from all indexes.
func (s *Store) removeFromIndexes(id string, metadata *functionv1.FunctionMetadata) {
	// Resource index
	if metadata.Resource != "" {
		if ids, exists := s.byResource[metadata.Resource]; exists {
			delete(ids, id)
			if len(ids) == 0 {
				delete(s.byResource, metadata.Resource)
			}
		}
	}

	// Tag index
	for _, tag := range metadata.Tags {
		if ids, exists := s.byTag[tag]; exists {
			delete(ids, id)
			if len(ids) == 0 {
				delete(s.byTag, tag)
			}
		}
	}

	// Risk level index - normalize enum names
	if metadata.Security != nil {
		riskStr := normalizeEnumName(metadata.Security.RiskLevel.String())
		if ids, exists := s.byRisk[riskStr]; exists {
			delete(ids, id)
			if len(ids) == 0 {
				delete(s.byRisk, riskStr)
			}
		}
	}

	// Mode index - normalize enum names
	if metadata.Behavior != nil {
		modeStr := normalizeEnumName(metadata.Behavior.Mode.String())
		if ids, exists := s.byMode[modeStr]; exists {
			delete(ids, id)
			if len(ids) == 0 {
				delete(s.byMode, modeStr)
			}
		}
	}
}

// normalizeEnumName converts enum names like MODE_QUERY -> query, RISK_LEVEL_LOW -> low, route_strategy_lb -> lb.
func normalizeEnumName(s string) string {
	// First convert to lowercase for consistent handling
	s = strings.ToLower(s)

	// Remove common lowercase prefixes (longer prefixes first to avoid partial matches)
	prefixes := []string{"route_strategy_", "risk_level_", "approval_type_", "mode_", "risk_", "route_", "approval_"}
	for _, prefix := range prefixes {
		if strings.HasPrefix(s, prefix) {
			s = strings.TrimPrefix(s, prefix)
			break
		}
	}
	return s
}

// cloneMetadata creates a deep copy of FunctionMetadata.
func cloneMetadata(metadata *functionv1.FunctionMetadata) (*functionv1.FunctionMetadata, error) {
	if metadata == nil {
		return nil, fmt.Errorf("metadata is nil")
	}
	return proto.Clone(metadata).(*functionv1.FunctionMetadata), nil
}

// intersectStringSets returns the intersection of two string sets.
func intersectStringSets(a, b map[string]struct{}) map[string]struct{} {
	result := make(map[string]struct{})
	for k := range a {
		if _, ok := b[k]; ok {
			result[k] = struct{}{}
		}
	}
	return result
}
