package resourcecatalog

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/cuihairu/croupier/internal/audit"
	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/model"
	"gorm.io/gorm"
)

// Service provides Resource Catalog operations reading from persistent storage.
type Service struct {
	db              *gorm.DB
	contractModel   *model.FunctionContractModel
	capabilityModel *model.ResourceCapabilityModel
	semanticsModel  *model.CapabilitySemanticsModel
	auditService    *audit.AuditService
}

// NewService creates the service.
func NewService(db *gorm.DB, auditSvc *audit.AuditService) *Service {
	return &Service{
		db:              db,
		contractModel:   model.NewFunctionContractModel(db),
		capabilityModel: model.NewResourceCapabilityModel(db),
		semanticsModel:  model.NewCapabilitySemanticsModel(db),
		auditService:    auditSvc,
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
	Version       int    `json:"version"`
	HasIdentity   bool   `json:"hasIdentity"`
	HasCollection bool   `json:"hasCollection"`
	HasCreate     bool   `json:"hasCreate"`
	HasUpdate     bool   `json:"hasUpdate"`
	HasDelete     bool   `json:"hasDelete"`
	HasActions    bool   `json:"hasActions"`
	HasTasks      bool   `json:"hasTasks"`
	HasReports    bool   `json:"hasReports"`
	Source        string `json:"source"`
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

// UpdateSemanticsRequest is the request for updating resource semantics.
type UpdateSemanticsRequest struct {
	GameID      string `json:"-"`
	Env         string `json:"-"`
	ResourceKey string `json:"-"`

	// Identity configuration
	IdentityField     string `json:"identityField,omitempty"`
	IdentityFieldType string `json:"identityFieldType,omitempty"` // string|number|integer
	IdentityPath      string `json:"identityPath,omitempty"`

	// Collection configuration
	CollectionQueryID uint   `json:"collectionQueryId,omitempty"`
	CollectionPath    string `json:"collectionPath,omitempty"`
	PageFieldName     string `json:"pageFieldName,omitempty"`
	PageSizeFieldName string `json:"pageSizeFieldName,omitempty"`
	ItemsFieldName    string `json:"itemsFieldName,omitempty"`
	TotalFieldName    string `json:"totalFieldName,omitempty"`

	// Item query configuration
	ItemQueryID uint   `json:"itemQueryId,omitempty"`
	ItemPath    string `json:"itemPath,omitempty"`

	// Lifecycle configuration
	CreateID uint `json:"createId,omitempty"`
	UpdateID uint `json:"updateId,omitempty"`
	DeleteID uint `json:"deleteId,omitempty"`

	// Change reason for audit
	ChangeReason string `json:"changeReason,omitempty"`
}

// UpdateSemanticsResponse is the response for updating semantics.
type UpdateSemanticsResponse struct {
	Version int    `json:"version"`
	Source  string `json:"source"`
	Message string `json:"message"`
}

// UpdateSemantics updates resource capability semantics.
// This allows admins to supplement semantics that cannot be auto-detected.
func (s *Service) UpdateSemantics(ctx context.Context, req *UpdateSemanticsRequest) (*UpdateSemanticsResponse, error) {
	// Get or create semantics
	semantics, err := s.semanticsModel.FindByScopeAndResourceKey(ctx, req.GameID, req.Env, req.ResourceKey)
	if err != nil {
		// Create new semantics
		semantics = &model.CapabilitySemantics{
			GameID:      req.GameID,
			Env:         req.Env,
			ResourceKey: req.ResourceKey,
			Source:      "platform_review",
		}
	}

	// Validate function IDs exist
	if req.CollectionQueryID > 0 {
		if err := s.validateFunctionID(ctx, req.GameID, req.Env, req.CollectionQueryID); err != nil {
			return nil, fmt.Errorf("invalid collectionQueryId: %w", err)
		}
		semantics.CollectionQueryID = req.CollectionQueryID
	}
	if req.ItemQueryID > 0 {
		if err := s.validateFunctionID(ctx, req.GameID, req.Env, req.ItemQueryID); err != nil {
			return nil, fmt.Errorf("invalid itemQueryId: %w", err)
		}
		semantics.ItemQueryID = req.ItemQueryID
	}
	if req.CreateID > 0 {
		if err := s.validateFunctionID(ctx, req.GameID, req.Env, req.CreateID); err != nil {
			return nil, fmt.Errorf("invalid createId: %w", err)
		}
		semantics.CreateID = req.CreateID
	}
	if req.UpdateID > 0 {
		if err := s.validateFunctionID(ctx, req.GameID, req.Env, req.UpdateID); err != nil {
			return nil, fmt.Errorf("invalid updateId: %w", err)
		}
		semantics.UpdateID = req.UpdateID
	}
	if req.DeleteID > 0 {
		if err := s.validateFunctionID(ctx, req.GameID, req.Env, req.DeleteID); err != nil {
			return nil, fmt.Errorf("invalid deleteId: %w", err)
		}
		semantics.DeleteID = req.DeleteID
	}

	// Update identity if provided
	if req.IdentityField != "" {
		semantics.IdentityField = req.IdentityField
		semantics.IdentityFieldType = req.IdentityFieldType
		if semantics.IdentityFieldType == "" {
			semantics.IdentityFieldType = "string"
		}
		semantics.IdentityPath = req.IdentityPath
	}

	// Update collection path if provided
	if req.CollectionPath != "" {
		semantics.CollectionPath = req.CollectionPath
	}
	if req.PageFieldName != "" {
		semantics.PageFieldName = req.PageFieldName
	}
	if req.PageSizeFieldName != "" {
		semantics.PageSizeFieldName = req.PageSizeFieldName
	}
	if req.ItemsFieldName != "" {
		semantics.ItemsFieldName = req.ItemsFieldName
	}
	if req.TotalFieldName != "" {
		semantics.TotalFieldName = req.TotalFieldName
	}

	// Update item path if provided
	if req.ItemPath != "" {
		semantics.ItemPath = req.ItemPath
	}

	// Set source to platform_review for manual updates
	semantics.Source = "platform_review"

	// Save semantics
	if err := s.semanticsModel.UpsertSemantics(ctx, semantics); err != nil {
		return nil, fmt.Errorf("upsert semantics: %w", err)
	}

	// Audit log
	if s.auditService != nil {
		actor := "admin" // TODO: get from context
		if _, err := s.auditService.Log(ctx, audit.EventSemanticUpdate,
			audit.WithActorID(actor, "user", actor),
			audit.WithResourceID("resource_catalog", req.ResourceKey),
			audit.WithDetails(map[string]interface{}{
				"change_reason": req.ChangeReason,
			}),
		); err != nil {
			slog.ErrorContext(ctx, "failed to write semantic update audit event",
				"resourceKey", req.ResourceKey,
				"error", err,
			)
		}
	}

	return &UpdateSemanticsResponse{
		Version: semantics.Version,
		Source:  semantics.Source,
		Message: "semantics updated successfully",
	}, nil
}

// validateFunctionID checks if a function ID exists in the scope.
func (s *Service) validateFunctionID(ctx context.Context, gameID, env string, functionID uint) error {
	_, err := s.contractModel.FindByScopeAndFunctionID(ctx, gameID, env, fmt.Sprintf("%d", functionID))
	return err
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

// ---------------------------------------------------------------------------
// Conflict Management API
// ---------------------------------------------------------------------------

// ConflictInfo represents a semantic conflict for API response.
type ConflictInfo struct {
	Field      string            `json:"field"`
	Values     map[string]string `json:"values"` // source -> value
	Resolution string            `json:"resolution,omitempty"`
	ResolvedAt string            `json:"resolvedAt,omitempty"`
	ResolvedBy string            `json:"resolvedBy,omitempty"`
}

// ProvenanceInfo represents field-level provenance for API response.
type ProvenanceInfo struct {
	Field        string `json:"field"`
	Source       string `json:"source"`
	Confidence   string `json:"confidence"`
	Status       string `json:"status"`
	Value        string `json:"value,omitempty"`
	UpdatedAt    string `json:"updatedAt"`
	UpdatedBy    string `json:"updatedBy"`
}

// ListConflictsRequest is the request for listing conflicts.
type ListConflictsRequest struct {
	GameID      string `json:"-"`
	Env         string `json:"-"`
	ResourceKey string `json:"-"`
}

// ListConflictsResponse is the response for listing conflicts.
type ListConflictsResponse struct {
	Conflicts []ConflictInfo  `json:"conflicts"`
	Provenance []ProvenanceInfo `json:"provenance"`
}

// ListConflicts returns conflicts and provenance for a resource.
func (s *Service) ListConflicts(ctx context.Context, req *ListConflictsRequest) (*ListConflictsResponse, error) {
	semantics, err := s.semanticsModel.FindByScopeAndResourceKey(ctx, req.GameID, req.Env, req.ResourceKey)
	if err != nil {
		return nil, fmt.Errorf("find semantics: %w", err)
	}

	response := &ListConflictsResponse{
		Conflicts:  make([]ConflictInfo, 0),
		Provenance: make([]ProvenanceInfo, 0),
	}

	// Parse conflicts
	if len(semantics.Conflicts) > 0 {
		var conflicts []spec.SemanticConflict
		if err := json.Unmarshal(semantics.Conflicts, &conflicts); err == nil {
			for _, c := range conflicts {
				values := make(map[string]string)
				for source, val := range c.Values {
					values[string(source)] = string(val)
				}
				response.Conflicts = append(response.Conflicts, ConflictInfo{
					Field:      c.Field,
					Values:     values,
					Resolution: string(c.Resolution),
					ResolvedAt: c.ResolvedAt,
					ResolvedBy: c.ResolvedBy,
				})
			}
		}
	}

	// Parse provenance
	if len(semantics.Provenance) > 0 {
		var provenance map[string]*spec.SemanticProvenance
		if err := json.Unmarshal(semantics.Provenance, &provenance); err == nil {
			for _, p := range provenance {
				response.Provenance = append(response.Provenance, ProvenanceInfo{
					Field:      p.Field,
					Source:     string(p.Source),
					Confidence: p.Confidence,
					Status:     p.Status,
					Value:      string(p.Value),
					UpdatedAt:  p.UpdatedAt,
					UpdatedBy:  p.UpdatedBy,
				})
			}
		}
	}

	return response, nil
}

// ResolveConflictRequest is the request for resolving a conflict.
type ResolveConflictRequest struct {
	GameID      string `json:"-"`
	Env         string `json:"-"`
	ResourceKey string `json:"-"`
	Field       string `json:"field"`
	ChosenSource string `json:"chosenSource"` // platform_review|sdk_explicit|openapi_rest
	Reason      string `json:"reason,omitempty"`
}

// ResolveConflictResponse is the response for resolving a conflict.
type ResolveConflictResponse struct {
	Message string `json:"message"`
}

// ResolveConflict resolves a semantic conflict by choosing a source.
func (s *Service) ResolveConflict(ctx context.Context, req *ResolveConflictRequest) (*ResolveConflictResponse, error) {
	semantics, err := s.semanticsModel.FindByScopeAndResourceKey(ctx, req.GameID, req.Env, req.ResourceKey)
	if err != nil {
		return nil, fmt.Errorf("find semantics: %w", err)
	}

	// Parse existing conflicts
	var conflicts []spec.SemanticConflict
	if len(semantics.Conflicts) > 0 {
		if err := json.Unmarshal(semantics.Conflicts, &conflicts); err != nil {
			return nil, fmt.Errorf("parse conflicts: %w", err)
		}
	}

	// Find the conflict
	found := false
	for i, c := range conflicts {
		if c.Field == req.Field {
			// Validate chosen source exists in conflict
			chosenSource := spec.SemanticSource(req.ChosenSource)
			if _, ok := c.Values[chosenSource]; !ok {
				return nil, fmt.Errorf("source %s not found in conflict values", req.ChosenSource)
			}

			// Resolve conflict
			conflicts[i].Resolution = chosenSource
			conflicts[i].ResolvedAt = time.Now().UTC().Format(time.RFC3339)
			conflicts[i].ResolvedBy = "admin" // TODO: get from context

			// Update provenance
			var provenance map[string]*spec.SemanticProvenance
			if len(semantics.Provenance) > 0 {
				json.Unmarshal(semantics.Provenance, &provenance)
			}
			if provenance == nil {
				provenance = make(map[string]*spec.SemanticProvenance)
			}

			if prov, exists := provenance[req.Field]; exists {
				prov.Value = c.Values[chosenSource]
				prov.Source = chosenSource
				prov.Confidence = confidenceForSource(chosenSource)
				prov.Status = "effective"
				prov.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
				prov.UpdatedBy = "admin"
			}

			// Save provenance
			provenanceJSON, _ := json.Marshal(provenance)
			semantics.Provenance = provenanceJSON

			found = true
			break
		}
	}

	if !found {
		return nil, fmt.Errorf("conflict not found for field %s", req.Field)
	}

	// Save conflicts
	conflictsJSON, _ := json.Marshal(conflicts)
	semantics.Conflicts = conflictsJSON

	// Update semantics
	if err := s.semanticsModel.Update(ctx, semantics); err != nil {
		return nil, fmt.Errorf("update semantics: %w", err)
	}

	// Audit log
	if s.auditService != nil {
		actor := "admin" // TODO: get from context
		if _, err := s.auditService.Log(ctx, audit.EventSemanticConflictResolve,
			audit.WithActorID(actor, "user", actor),
			audit.WithResourceID("resource_catalog", req.ResourceKey),
			audit.WithDetails(map[string]interface{}{
				"field":          req.Field,
				"chosen_source":  req.ChosenSource,
				"reason":         req.Reason,
			}),
		); err != nil {
			slog.ErrorContext(ctx, "failed to write semantic conflict resolve audit event",
				"resourceKey", req.ResourceKey,
				"field", req.Field,
				"error", err,
			)
		}
	}

	return &ResolveConflictResponse{
		Message: fmt.Sprintf("Conflict resolved for field %s", req.Field),
	}, nil
}

func confidenceForSource(source spec.SemanticSource) string {
	switch source {
	case spec.SemanticSourcePlatformReview, spec.SemanticSourceSDKExplicit:
		return "high"
	case spec.SemanticSourceOpenAPIRest:
		return "low"
	default:
		return "low"
	}
}
