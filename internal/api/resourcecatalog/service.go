package resourcecatalog

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/cuihairu/croupier/internal/model"
	"gorm.io/gorm"
)

// Service provides Resource Catalog operations reading from persistent storage.
type Service struct {
	db              *gorm.DB
	contractModel   *model.FunctionContractModel
	capabilityModel *model.ResourceCapabilityModel
	semanticsModel  *model.CapabilitySemanticsModel
}

// NewService creates the service.
func NewService(db *gorm.DB) *Service {
	return &Service{
		db:              db,
		contractModel:   model.NewFunctionContractModel(db),
		capabilityModel: model.NewResourceCapabilityModel(db),
		semanticsModel:  model.NewCapabilitySemanticsModel(db),
	}
}

// ResourceCatalogItem represents a resource in the catalog.
type ResourceCatalogItem struct {
	ResourceKey string            `json:"resourceKey"`
	Labels      map[string]string `json:"labels"`
	Description map[string]string `json:"description,omitempty"`
	CategoryKey string            `json:"categoryKey,omitempty"`
	Status      string            `json:"status"` // identified|pending|conflict|not_executable
	Functions   []FunctionInfo    `json:"functions"`
	Semantics   *SemanticsInfo    `json:"semantics,omitempty"`
	Diagnostics []DiagnosticInfo  `json:"diagnostics,omitempty"`
}

// FunctionInfo represents a function in the catalog.
type FunctionInfo struct {
	FunctionID string `json:"functionId"`
	Version    string `json:"version"`
	Capability string `json:"capability"`
	Execution  string `json:"execution"`
	Risk       string `json:"risk"`
	Enabled    bool   `json:"enabled"`
	Source     string `json:"source"`
}

// SemanticsInfo represents capability semantics summary.
type SemanticsInfo struct {
	Version        int    `json:"version"`
	HasIdentity    bool   `json:"hasIdentity"`
	HasCollection  bool   `json:"hasCollection"`
	HasCreate      bool   `json:"hasCreate"`
	HasUpdate      bool   `json:"hasUpdate"`
	HasDelete      bool   `json:"hasDelete"`
	HasActions     bool   `json:"hasActions"`
	HasTasks       bool   `json:"hasTasks"`
	HasReports     bool   `json:"hasReports"`
	Source         string `json:"source"`
}

// DiagnosticInfo represents a diagnostic message.
type DiagnosticInfo struct {
	Code     string `json:"code"`
	Severity string `json:"severity"`
	Message  string `json:"message"`
}

// ListRequest is the request for listing resources.
type ListRequest struct {
	GameID   string
	Env      string
	Category string
	Query    string
}

// ListResponse is the response for listing resources.
type ListResponse struct {
	Items []ResourceCatalogItem `json:"items"`
	Total int                   `json:"total"`
}

// List returns resource catalog items from persistent storage.
func (s *Service) List(ctx context.Context, req *ListRequest) (*ListResponse, error) {
	// Get all resource capabilities
	capabilities, err := s.capabilityModel.ListByScope(ctx, req.GameID, req.Env)
	if err != nil {
		return nil, fmt.Errorf("list resource capabilities: %w", err)
	}

	items := make([]ResourceCatalogItem, 0, len(capabilities))
	for _, cap := range capabilities {
		// Apply category filter
		if req.Category != "" && cap.CategoryKey != req.Category {
			continue
		}

		// Get contracts for this resource
		contracts, err := s.contractModel.ListByResourceKey(ctx, req.GameID, req.Env, cap.ResourceKey)
		if err != nil {
			continue
		}

		// Get semantics
		semantics, _ := s.semanticsModel.FindByScopeAndResourceKey(ctx, req.GameID, req.Env, cap.ResourceKey)

		// Build item
		item := ResourceCatalogItem{
			ResourceKey: cap.ResourceKey,
			Labels:      toStringMap(cap.Labels),
			Description: toStringMap(cap.Description),
			CategoryKey: cap.CategoryKey,
			Status:      determineStatus(contracts, semantics),
			Functions:   buildFunctionInfos(contracts),
			Semantics:   buildSemanticsInfo(semantics),
		}

		// Apply search filter
		if req.Query != "" {
			if !matchesQuery(item, req.Query) {
				continue
			}
		}

		items = append(items, item)
	}

	// Sort by category, then resource key
	sort.Slice(items, func(i, j int) bool {
		if items[i].CategoryKey != items[j].CategoryKey {
			return items[i].CategoryKey < items[j].CategoryKey
		}
		return items[i].ResourceKey < items[j].ResourceKey
	})

	return &ListResponse{
		Items: items,
		Total: len(items),
	}, nil
}

// DetailRequest is the request for resource detail.
type DetailRequest struct {
	GameID      string
	Env         string
	ResourceKey string
}

// Detail returns a single resource catalog item.
func (s *Service) Detail(ctx context.Context, req *DetailRequest) (*ResourceCatalogItem, error) {
	// Get resource capability
	cap, err := s.capabilityModel.FindByScopeAndResourceKey(ctx, req.GameID, req.Env, req.ResourceKey)
	if err != nil {
		return nil, fmt.Errorf("resource not found: %w", err)
	}

	// Get contracts
	contracts, err := s.contractModel.ListByResourceKey(ctx, req.GameID, req.Env, req.ResourceKey)
	if err != nil {
		return nil, fmt.Errorf("list contracts: %w", err)
	}

	// Get semantics
	semantics, _ := s.semanticsModel.FindByScopeAndResourceKey(ctx, req.GameID, req.Env, req.ResourceKey)

	item := &ResourceCatalogItem{
		ResourceKey: cap.ResourceKey,
		Labels:      toStringMap(cap.Labels),
		Description: toStringMap(cap.Description),
		CategoryKey: cap.CategoryKey,
		Status:      determineStatus(contracts, semantics),
		Functions:   buildFunctionInfos(contracts),
		Semantics:   buildSemanticsInfo(semantics),
	}

	return item, nil
}

// Helper functions

func determineStatus(contracts []*model.FunctionContract, semantics *model.CapabilitySemantics) string {
	if len(contracts) == 0 {
		return "not_executable"
	}
	if semantics == nil {
		return "pending"
	}
	// Check if we have a complete CRUD set
	hasQuery := false
	hasIdentity := false
	for _, c := range contracts {
		if c.Capability == "collection_query" {
			hasQuery = true
		}
		if c.Capability == "item_query" {
			hasIdentity = true
		}
	}
	if hasQuery && hasIdentity {
		return "identified"
	}
	return "pending"
}

func buildFunctionInfos(contracts []*model.FunctionContract) []FunctionInfo {
	infos := make([]FunctionInfo, 0, len(contracts))
	for _, c := range contracts {
		infos = append(infos, FunctionInfo{
			FunctionID: c.FunctionID,
			Version:    c.Version,
			Capability: c.Capability,
			Execution:  c.Execution,
			Risk:       c.Risk,
			Enabled:    c.Enabled,
			Source:     c.Source,
		})
	}
	return infos
}

func buildSemanticsInfo(semantics *model.CapabilitySemantics) *SemanticsInfo {
	if semantics == nil {
		return nil
	}
	return &SemanticsInfo{
		Version:       semantics.Version,
		HasIdentity:   semantics.IdentityField != "",
		HasCollection: semantics.CollectionQueryID > 0,
		HasCreate:     semantics.CreateID > 0,
		HasUpdate:     semantics.UpdateID > 0,
		HasDelete:     semantics.DeleteID > 0,
		HasActions:    len(semantics.Actions) > 0,
		HasTasks:      len(semantics.Tasks) > 0,
		HasReports:    len(semantics.Reports) > 0,
		Source:        semantics.Source,
	}
}

func toStringMap(m map[string]interface{}) map[string]string {
	if m == nil {
		return nil
	}
	result := make(map[string]string, len(m))
	for k, v := range m {
		if s, ok := v.(string); ok {
			result[k] = s
		}
	}
	return result
}

func matchesQuery(item ResourceCatalogItem, query string) bool {
	q := strings.ToLower(query)
	if strings.Contains(strings.ToLower(item.ResourceKey), q) {
		return true
	}
	for _, label := range item.Labels {
		if strings.Contains(strings.ToLower(label), q) {
			return true
		}
	}
	return false
}
