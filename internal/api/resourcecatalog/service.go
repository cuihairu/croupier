package resourcecatalog

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/cuihairu/croupier/internal/audit"
	"github.com/cuihairu/croupier/internal/dashboard/freshness"
	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/dbenum"
	logicutils "github.com/cuihairu/croupier/internal/logic/utils"
	"github.com/cuihairu/croupier/internal/model"
	contractsvc "github.com/cuihairu/croupier/internal/service"
	"github.com/cuihairu/croupier/internal/svc"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// Service provides Resource Catalog operations reading from persistent storage.
type Service struct {
	db              *gorm.DB
	contractModel   *model.FunctionContractModel
	capabilityModel *model.ResourceCapabilityModel
	semanticsModel  *model.CapabilitySemanticsModel
	versionModel    *model.CapabilitySemanticVersionModel
	contractService *contractsvc.ContractService
	proposalModel   *model.PageProposalModel
	pageModel       *model.PageSpecModel
	publishedModel  *model.PublishedPageSpecModel
	auditService    *audit.AuditService
}

// NewService creates the service.
func NewService(db *gorm.DB, auditSvc *audit.AuditService) *Service {
	return &Service{
		db:              db,
		contractModel:   model.NewFunctionContractModel(db),
		capabilityModel: model.NewResourceCapabilityModel(db),
		semanticsModel:  model.NewCapabilitySemanticsModel(db),
		versionModel:    model.NewCapabilitySemanticVersionModel(db),
		contractService: contractsvc.NewContractService(db),
		proposalModel:   model.NewPageProposalModel(db),
		pageModel:       model.NewPageSpecModel(db),
		publishedModel:  model.NewPublishedPageSpecModel(db),
		auditService:    auditSvc,
	}
}

// ResourceCatalogItem represents a resource in the catalog.
type ResourceCatalogItem struct {
	ResourceKey   string             `json:"resourceKey"`
	Labels        map[string]string  `json:"labels"`
	Description   map[string]string  `json:"description,omitempty"`
	CategoryKey   string             `json:"categoryKey,omitempty"`
	Status        string             `json:"status"` // identified|pending|conflict|not_executable
	Functions     []FunctionInfo     `json:"functions"`
	Semantics     *SemanticsInfo     `json:"semantics,omitempty"`
	Diagnostics   []DiagnosticInfo   `json:"diagnostics,omitempty"`
	AffectedPages []AffectedPageInfo `json:"affectedPages,omitempty"`
}

// FunctionInfo represents a function in the catalog.
type FunctionInfo struct {
	ID         uint   `json:"id"`
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
	Version             int                   `json:"version"`
	HasIdentity         bool                  `json:"hasIdentity"`
	HasCollection       bool                  `json:"hasCollection"`
	HasCreate           bool                  `json:"hasCreate"`
	HasUpdate           bool                  `json:"hasUpdate"`
	HasDelete           bool                  `json:"hasDelete"`
	HasActions          bool                  `json:"hasActions"`
	HasTasks            bool                  `json:"hasTasks"`
	HasReports          bool                  `json:"hasReports"`
	Source              string                `json:"source"`
	SourceDigest        string                `json:"sourceDigest,omitempty"`
	IdentityField       string                `json:"identityField,omitempty"`
	IdentityFieldType   string                `json:"identityFieldType,omitempty"`
	IdentityPath        string                `json:"identityPath,omitempty"`
	CollectionQueryID   uint                  `json:"collectionQueryId,omitempty"`
	CollectionPath      string                `json:"collectionPath,omitempty"`
	PageFieldName       string                `json:"pageFieldName,omitempty"`
	PageSizeFieldName   string                `json:"pageSizeFieldName,omitempty"`
	ItemsFieldName      string                `json:"itemsFieldName,omitempty"`
	TotalFieldName      string                `json:"totalFieldName,omitempty"`
	ItemQueryID         uint                  `json:"itemQueryId,omitempty"`
	ItemPath            string                `json:"itemPath,omitempty"`
	CreateID            uint                  `json:"createId,omitempty"`
	UpdateID            uint                  `json:"updateId,omitempty"`
	DeleteID            uint                  `json:"deleteId,omitempty"`
	Actions             []ActionSemanticInfo  `json:"actions,omitempty"`
	Tasks               []spec.TaskSemantic   `json:"tasks,omitempty"`
	Reports             []spec.ReportSemantic `json:"reports,omitempty"`
	UnresolvedConflicts int                   `json:"unresolvedConflicts"`
}

// ActionSemanticInfo describes resource action capability semantics.
type ActionSemanticInfo struct {
	FunctionID    string `json:"functionId"`
	Subject       string `json:"subject"` // resource_item|resource_selection|none
	IdentityInput string `json:"identityInput,omitempty"`
}

// DiagnosticInfo represents a diagnostic message.
type DiagnosticInfo struct {
	Code       string `json:"code"`
	Severity   string `json:"severity"`
	Message    string `json:"message"`
	FunctionID string `json:"functionId,omitempty"`
	Field      string `json:"field,omitempty"`
}

// AffectedPageInfo summarizes UI artifacts impacted by this resource.
type AffectedPageInfo struct {
	PageKey          string                            `json:"pageKey"`
	PageType         string                            `json:"pageType,omitempty"`
	Title            map[string]string                 `json:"title,omitempty"`
	Kind             string                            `json:"kind"` // draft|published|proposal
	Status           string                            `json:"status,omitempty"`
	DraftRevision    int                               `json:"draftRevision,omitempty"`
	PublishedVersion int                               `json:"publishedVersion,omitempty"`
	ProposalKey      string                            `json:"proposalKey,omitempty"`
	ProposalQuality  string                            `json:"proposalQuality,omitempty"`
	ProposalStatus   string                            `json:"proposalStatus,omitempty"`
	Stale            bool                              `json:"stale,omitempty"`
	BindingFreshness []spec.BindingFreshnessDiagnostic `json:"bindingFreshness,omitempty"`
	UpdatedAt        string                            `json:"updatedAt,omitempty"`
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
	capabilities, err := s.capabilityModel.ListByScope(ctx, svc.ResolveGameID(ctx, req.GameID), svc.ResolveEnv(ctx, req.Env))
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
		contracts, err := s.contractModel.ListByResourceKey(ctx, svc.ResolveGameID(ctx, req.GameID), svc.ResolveEnv(ctx, req.Env), cap.ResourceKey)
		if err != nil {
			return nil, fmt.Errorf("list contracts for resource %s: %w", cap.ResourceKey, err)
		}

		// Get semantics
		semantics, err := s.findSemanticsOptional(ctx, svc.ResolveGameID(ctx, req.GameID), svc.ResolveEnv(ctx, req.Env), cap.ResourceKey)
		if err != nil {
			return nil, err
		}

		// Build item
		item := ResourceCatalogItem{
			ResourceKey: cap.ResourceKey,
			Labels:      labelsForResource(cap.Labels, cap.ResourceKey),
			Description: toStringMap(cap.Description),
			CategoryKey: categoryKeyForResource(cap.ResourceKey, cap.CategoryKey),
			Status:      determineStatus(contracts, semantics),
			Functions:   buildFunctionInfos(contracts),
			Semantics:   buildSemanticsInfo(semantics),
			Diagnostics: buildDiagnostics(contracts, semantics),
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
	GameID      string `json:"-"`
	Env         string `json:"-"`
	ResourceKey string `uri:"resourceKey" binding:"required"`
}

// Detail returns a single resource catalog item.
func (s *Service) Detail(ctx context.Context, req *DetailRequest) (*ResourceCatalogItem, error) {
	// Get resource capability
	cap, err := s.capabilityModel.FindByScopeAndResourceKey(ctx, svc.ResolveGameID(ctx, req.GameID), svc.ResolveEnv(ctx, req.Env), req.ResourceKey)
	if err != nil {
		return nil, fmt.Errorf("resource not found: %w", err)
	}

	// Get contracts
	contracts, err := s.contractModel.ListByResourceKey(ctx, svc.ResolveGameID(ctx, req.GameID), svc.ResolveEnv(ctx, req.Env), req.ResourceKey)
	if err != nil {
		return nil, fmt.Errorf("list contracts: %w", err)
	}

	// Get semantics
	semantics, err := s.findSemanticsOptional(ctx, svc.ResolveGameID(ctx, req.GameID), svc.ResolveEnv(ctx, req.Env), req.ResourceKey)
	if err != nil {
		return nil, err
	}

	item := &ResourceCatalogItem{
		ResourceKey: cap.ResourceKey,
		Labels:      labelsForResource(cap.Labels, cap.ResourceKey),
		Description: toStringMap(cap.Description),
		CategoryKey: categoryKeyForResource(cap.ResourceKey, cap.CategoryKey),
		Status:      determineStatus(contracts, semantics),
		Functions:   buildFunctionInfos(contracts),
		Semantics:   buildSemanticsInfo(semantics),
		Diagnostics: buildDiagnostics(contracts, semantics),
	}
	affectedPages, err := s.buildAffectedPages(ctx, svc.ResolveGameID(ctx, req.GameID), svc.ResolveEnv(ctx, req.Env), req.ResourceKey)
	if err != nil {
		return nil, err
	}
	item.AffectedPages = affectedPages

	return item, nil
}

// UpdateSemanticsRequest is the request for updating resource semantics.
type UpdateSemanticsRequest struct {
	GameID      string `json:"-"`
	Env         string `json:"-"`
	ResourceKey string `json:"-" uri:"resourceKey" binding:"required"`

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

	// Resource action semantics. This is capability context, not button UI.
	Actions []ActionSemanticInfo `json:"actions,omitempty"`

	// Task/report semantics. These are capability data semantics, not page UI.
	Tasks   []spec.TaskSemantic   `json:"tasks,omitempty"`
	Reports []spec.ReportSemantic `json:"reports,omitempty"`

	// Change reason for audit
	ChangeReason string `json:"changeReason,omitempty"`
}

// UpdateSemanticsResponse is the response for updating semantics.
type UpdateSemanticsResponse struct {
	Version int    `json:"version"`
	Source  string `json:"source"`
	Message string `json:"message"`
}

// SemanticVersionInfo represents a stored CapabilitySemantics snapshot version.
type SemanticVersionInfo struct {
	Version      int    `json:"version"`
	SourceDigest string `json:"sourceDigest,omitempty"`
	ChangeReason string `json:"changeReason,omitempty"`
	CreatedAt    string `json:"createdAt"`
	CreatedBy    string `json:"createdBy,omitempty"`
}

// ListSemanticVersionsRequest is the request for semantic version history.
type ListSemanticVersionsRequest struct {
	GameID      string `json:"-"`
	Env         string `json:"-"`
	ResourceKey string `json:"-" uri:"resourceKey" binding:"required"`
	// Limit caps the page size; defaults to semanticVersionDefaultLimit when 0.
	Limit int `form:"limit"`
	// Offset skips older versions; newest first ordering is preserved.
	Offset int `form:"offset"`
}

const (
	semanticVersionDefaultLimit = 5
	semanticVersionMaxLimit     = 100
)

// ListSemanticVersionsResponse is the response for semantic version history.
type ListSemanticVersionsResponse struct {
	Items []SemanticVersionInfo `json:"items"`
	Total int64                 `json:"total"`
}

// UpdateSemantics updates resource capability semantics.
// This allows admins to supplement semantics that cannot be auto-detected.
func (s *Service) UpdateSemantics(ctx context.Context, req *UpdateSemanticsRequest) (*UpdateSemanticsResponse, error) {
	actor := actorFromContext(ctx)
	if err := s.requireResourceCapability(ctx, svc.ResolveGameID(ctx, req.GameID), svc.ResolveEnv(ctx, req.Env), req.ResourceKey); err != nil {
		return nil, err
	}
	// Get or create semantics
	semantics, err := s.semanticsModel.FindByScopeAndResourceKey(ctx, svc.ResolveGameID(ctx, req.GameID), svc.ResolveEnv(ctx, req.Env), req.ResourceKey)
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("find semantics: %w", err)
		}
		// Create new semantics
		semantics = &model.CapabilitySemantics{
			GameID:      svc.ResolveGameID(ctx, req.GameID),
			Env:         svc.ResolveEnv(ctx, req.Env),
			ResourceKey: req.ResourceKey,
			Source:      "platform_review",
			UpdatedBy:   actor,
		}
	}
	sourceDigest := semanticSourceDigest(ctx, s.contractModel, svc.ResolveGameID(ctx, req.GameID), svc.ResolveEnv(ctx, req.Env), req.ResourceKey)
	if sourceDigest != "" {
		semantics.SourceDigest = sourceDigest
	}
	provenance := parseProvenance(semantics.Provenance)
	changedFields := make([]string, 0)
	trackString := func(field string, value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		changedFields = append(changedFields, field)
		provenance[field] = provenanceRecord(field, spec.SemanticSourcePlatformReview, sourceDigest, rawJSONString(value), "high", "effective", actor)
	}
	trackUint := func(field string, value uint) {
		if value == 0 {
			return
		}
		changedFields = append(changedFields, field)
		provenance[field] = provenanceRecord(field, spec.SemanticSourcePlatformReview, sourceDigest, rawJSONUint(value), "high", "effective", actor)
	}

	// Validate function IDs exist
	if req.CollectionQueryID > 0 {
		if _, err := s.validateFunctionBinding(ctx, svc.ResolveGameID(ctx, req.GameID), svc.ResolveEnv(ctx, req.Env), req.ResourceKey, req.CollectionQueryID, "collection_query"); err != nil {
			return nil, fmt.Errorf("invalid collectionQueryId: %w", err)
		}
		semantics.CollectionQueryID = req.CollectionQueryID
		trackUint("collectionQueryID", req.CollectionQueryID)
	}
	if req.ItemQueryID > 0 {
		if _, err := s.validateFunctionBinding(ctx, svc.ResolveGameID(ctx, req.GameID), svc.ResolveEnv(ctx, req.Env), req.ResourceKey, req.ItemQueryID, "item_query"); err != nil {
			return nil, fmt.Errorf("invalid itemQueryId: %w", err)
		}
		semantics.ItemQueryID = req.ItemQueryID
		trackUint("itemQueryID", req.ItemQueryID)
	}
	if req.CreateID > 0 {
		if _, err := s.validateFunctionBinding(ctx, svc.ResolveGameID(ctx, req.GameID), svc.ResolveEnv(ctx, req.Env), req.ResourceKey, req.CreateID, "create"); err != nil {
			return nil, fmt.Errorf("invalid createId: %w", err)
		}
		semantics.CreateID = req.CreateID
		trackUint("createID", req.CreateID)
	}
	if req.UpdateID > 0 {
		if _, err := s.validateFunctionBinding(ctx, svc.ResolveGameID(ctx, req.GameID), svc.ResolveEnv(ctx, req.Env), req.ResourceKey, req.UpdateID, "update"); err != nil {
			return nil, fmt.Errorf("invalid updateId: %w", err)
		}
		semantics.UpdateID = req.UpdateID
		trackUint("updateID", req.UpdateID)
	}
	if req.DeleteID > 0 {
		if _, err := s.validateFunctionBinding(ctx, svc.ResolveGameID(ctx, req.GameID), svc.ResolveEnv(ctx, req.Env), req.ResourceKey, req.DeleteID, "delete"); err != nil {
			return nil, fmt.Errorf("invalid deleteId: %w", err)
		}
		semantics.DeleteID = req.DeleteID
		trackUint("deleteID", req.DeleteID)
	}
	if req.Actions != nil {
		actions, err := s.validateActionSemantics(ctx, svc.ResolveGameID(ctx, req.GameID), svc.ResolveEnv(ctx, req.Env), req.ResourceKey, req.Actions)
		if err != nil {
			return nil, err
		}
		raw, err := json.Marshal(actions)
		if err != nil {
			return nil, fmt.Errorf("marshal actions: %w", err)
		}
		semantics.Actions = datatypes.JSON(raw)
		changedFields = append(changedFields, "actions")
		provenance["actions"] = provenanceRecord("actions", spec.SemanticSourcePlatformReview, sourceDigest, json.RawMessage(raw), "high", "effective", actor)
	}
	if req.Tasks != nil {
		tasks, err := s.validateTaskSemantics(ctx, svc.ResolveGameID(ctx, req.GameID), svc.ResolveEnv(ctx, req.Env), req.ResourceKey, req.Tasks)
		if err != nil {
			return nil, err
		}
		raw, err := json.Marshal(tasks)
		if err != nil {
			return nil, fmt.Errorf("marshal tasks: %w", err)
		}
		semantics.Tasks = datatypes.JSON(raw)
		changedFields = append(changedFields, "tasks")
		provenance["tasks"] = provenanceRecord("tasks", spec.SemanticSourcePlatformReview, sourceDigest, json.RawMessage(raw), "high", "effective", actor)
	}
	if req.Reports != nil {
		reports, err := s.validateReportSemantics(ctx, svc.ResolveGameID(ctx, req.GameID), svc.ResolveEnv(ctx, req.Env), req.ResourceKey, req.Reports)
		if err != nil {
			return nil, err
		}
		raw, err := json.Marshal(reports)
		if err != nil {
			return nil, fmt.Errorf("marshal reports: %w", err)
		}
		semantics.Reports = datatypes.JSON(raw)
		changedFields = append(changedFields, "reports")
		provenance["reports"] = provenanceRecord("reports", spec.SemanticSourcePlatformReview, sourceDigest, json.RawMessage(raw), "high", "effective", actor)
	}

	// Update identity if provided
	if req.IdentityField != "" {
		semantics.IdentityField = req.IdentityField
		semantics.IdentityFieldType = req.IdentityFieldType
		if semantics.IdentityFieldType == "" {
			semantics.IdentityFieldType = "string"
		}
		semantics.IdentityPath = req.IdentityPath
		trackString("identityField", semantics.IdentityField)
		trackString("identityFieldType", semantics.IdentityFieldType)
		trackString("identityPath", semantics.IdentityPath)
	}

	// Update collection path if provided
	if req.CollectionPath != "" {
		semantics.CollectionPath = req.CollectionPath
		trackString("collectionPath", req.CollectionPath)
	}
	if req.PageFieldName != "" {
		semantics.PageFieldName = req.PageFieldName
		trackString("pageFieldName", req.PageFieldName)
	}
	if req.PageSizeFieldName != "" {
		semantics.PageSizeFieldName = req.PageSizeFieldName
		trackString("pageSizeFieldName", req.PageSizeFieldName)
	}
	if req.ItemsFieldName != "" {
		semantics.ItemsFieldName = req.ItemsFieldName
		trackString("itemsFieldName", req.ItemsFieldName)
	}
	if req.TotalFieldName != "" {
		semantics.TotalFieldName = req.TotalFieldName
		trackString("totalFieldName", req.TotalFieldName)
	}

	// Update item path if provided
	if req.ItemPath != "" {
		semantics.ItemPath = req.ItemPath
		trackString("itemPath", req.ItemPath)
	}

	// Set source to platform_review for manual updates
	semantics.Source = "platform_review"
	semantics.UpdatedBy = actor
	if len(provenance) > 0 {
		raw, err := json.Marshal(provenance)
		if err != nil {
			return nil, fmt.Errorf("marshal provenance: %w", err)
		}
		semantics.Provenance = raw
	}

	// Save semantics
	if err := s.semanticsModel.UpsertSemantics(ctx, semantics); err != nil {
		return nil, fmt.Errorf("upsert semantics: %w", err)
	}
	if err := s.createSemanticVersion(ctx, semantics, req.ChangeReason, actor); err != nil {
		return nil, err
	}
	if err := s.rebuildProposals(ctx, svc.ResolveGameID(ctx, req.GameID), svc.ResolveEnv(ctx, req.Env), req.ResourceKey); err != nil {
		return nil, err
	}

	// Audit log
	if s.auditService != nil {
		if _, err := s.auditService.Log(ctx, audit.EventSemanticUpdate,
			audit.WithActorID(actor, "user", actor),
			audit.WithResourceID("resource_catalog", req.ResourceKey),
			audit.WithDetails(map[string]interface{}{
				"change_reason":    req.ChangeReason,
				"changed_fields":   changedFields,
				"semantic_version": semantics.Version,
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

// ListSemanticVersions returns a paginated semantic version history for a
// resource, newest first. A resource can accumulate thousands of versions from
// automated re-registrations, so the endpoint never returns the full list.
func (s *Service) ListSemanticVersions(ctx context.Context, req *ListSemanticVersionsRequest) (*ListSemanticVersionsResponse, error) {
	semantics, err := s.findSemanticsOptional(ctx, svc.ResolveGameID(ctx, req.GameID), svc.ResolveEnv(ctx, req.Env), req.ResourceKey)
	if err != nil {
		return nil, err
	}
	if semantics == nil {
		return &ListSemanticVersionsResponse{
			Items: []SemanticVersionInfo{},
			Total: 0,
		}, nil
	}
	limit := req.Limit
	if limit <= 0 {
		limit = semanticVersionDefaultLimit
	}
	if limit > semanticVersionMaxLimit {
		limit = semanticVersionMaxLimit
	}
	if req.Offset < 0 {
		req.Offset = 0
	}
	versions, total, err := s.versionModel.ListBySemanticsIDPaged(ctx, semantics.ID, limit, req.Offset)
	if err != nil {
		return nil, fmt.Errorf("list semantic versions: %w", err)
	}
	items := make([]SemanticVersionInfo, 0, len(versions))
	for _, version := range versions {
		items = append(items, SemanticVersionInfo{
			Version:      version.Version,
			SourceDigest: version.SourceDigest,
			ChangeReason: version.ChangeReason,
			CreatedAt:    version.CreatedAt.Format(time.RFC3339),
			CreatedBy:    version.CreatedBy,
		})
	}
	return &ListSemanticVersionsResponse{
		Items: items,
		Total: total,
	}, nil
}

func (s *Service) requireResourceCapability(ctx context.Context, gameID, env, resourceKey string) error {
	if _, err := s.capabilityModel.FindByScopeAndResourceKey(ctx, gameID, env, resourceKey); err != nil {
		return fmt.Errorf("resource capability not found: %w", err)
	}
	return nil
}

// validateFunctionBinding checks if a function belongs to the resource and capability slot.
func (s *Service) validateFunctionBinding(ctx context.Context, gameID, env, resourceKey string, functionID uint, capability string) (*model.FunctionContract, error) {
	var contract model.FunctionContract
	if err := s.db.WithContext(ctx).
		Where("game_id = ? AND env = ? AND id = ?", gameID, env, functionID).
		First(&contract).Error; err != nil {
		return nil, err
	}
	if contract.ResourceKey != resourceKey {
		return nil, fmt.Errorf("function %s belongs to resource %s, not %s", contract.FunctionID, contract.ResourceKey, resourceKey)
	}
	if contract.Capability.String() != capability {
		return nil, fmt.Errorf("function %s capability is %s, not %s", contract.FunctionID, contract.Capability.String(), capability)
	}
	if !contract.Enabled {
		return nil, fmt.Errorf("function %s is disabled", contract.FunctionID)
	}
	return &contract, nil
}

// Helper functions

func determineStatus(contracts []*model.FunctionContract, semantics *model.CapabilitySemantics) string {
	if len(contracts) == 0 {
		return "not_executable"
	}
	if semantics == nil {
		return "pending"
	}
	if countUnresolvedConflicts(semantics.Conflicts) > 0 {
		return "conflict"
	}
	hasCollectionContract := false
	for _, contract := range contracts {
		if contract != nil && contract.Capability == dbenum.CapabilityCollectionQuery {
			hasCollectionContract = true
			break
		}
	}
	if hasCollectionContract && semantics.CollectionQueryID > 0 && strings.TrimSpace(semantics.IdentityField) != "" {
		return "identified"
	}
	return "pending"
}

func buildFunctionInfos(contracts []*model.FunctionContract) []FunctionInfo {
	infos := make([]FunctionInfo, 0, len(contracts))
	for _, c := range contracts {
		infos = append(infos, FunctionInfo{
			ID:         c.ID,
			FunctionID: c.FunctionID,
			Version:    c.Version,
			Capability: c.Capability.String(),
			Execution:  c.Execution,
			Risk:       c.Risk.String(),
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
		Version:             semantics.Version,
		HasIdentity:         semantics.IdentityField != "",
		HasCollection:       semantics.CollectionQueryID > 0,
		HasCreate:           semantics.CreateID > 0,
		HasUpdate:           semantics.UpdateID > 0,
		HasDelete:           semantics.DeleteID > 0,
		HasActions:          len(parseActionSemantics(semantics.Actions)) > 0,
		HasTasks:            len(parseTaskSemantics(semantics.Tasks)) > 0,
		HasReports:          len(parseReportSemantics(semantics.Reports)) > 0,
		Source:              semantics.Source,
		SourceDigest:        semantics.SourceDigest,
		IdentityField:       semantics.IdentityField,
		IdentityFieldType:   semantics.IdentityFieldType,
		IdentityPath:        semantics.IdentityPath,
		CollectionQueryID:   semantics.CollectionQueryID,
		CollectionPath:      semantics.CollectionPath,
		PageFieldName:       semantics.PageFieldName,
		PageSizeFieldName:   semantics.PageSizeFieldName,
		ItemsFieldName:      semantics.ItemsFieldName,
		TotalFieldName:      semantics.TotalFieldName,
		ItemQueryID:         semantics.ItemQueryID,
		ItemPath:            semantics.ItemPath,
		CreateID:            semantics.CreateID,
		UpdateID:            semantics.UpdateID,
		DeleteID:            semantics.DeleteID,
		Actions:             parseActionSemantics(semantics.Actions),
		Tasks:               parseTaskSemantics(semantics.Tasks),
		Reports:             parseReportSemantics(semantics.Reports),
		UnresolvedConflicts: countUnresolvedConflicts(semantics.Conflicts),
	}
}

func buildDiagnostics(contracts []*model.FunctionContract, semantics *model.CapabilitySemantics) []DiagnosticInfo {
	diagnostics := make([]DiagnosticInfo, 0)
	if semantics != nil {
		diagnostics = append(diagnostics, decodeDiagnostics(semantics.Diagnostics, "")...)
		if unresolved := countUnresolvedConflicts(semantics.Conflicts); unresolved > 0 {
			diagnostics = append(diagnostics, DiagnosticInfo{
				Code:     "semantic_conflict",
				Severity: "error",
				Message:  fmt.Sprintf("%d semantic conflict(s) require platform review", unresolved),
			})
		}
	}
	for _, contract := range contracts {
		if contract == nil {
			continue
		}
		diagnostics = append(diagnostics, decodeDiagnostics(contract.Diagnostics, contract.FunctionID)...)
	}
	return diagnostics
}

func (s *Service) validateActionSemantics(
	ctx context.Context,
	gameID string,
	env string,
	resourceKey string,
	actions []ActionSemanticInfo,
) ([]ActionSemanticInfo, error) {
	out := make([]ActionSemanticInfo, 0, len(actions))
	seen := map[string]struct{}{}
	for index, action := range actions {
		functionID := strings.TrimSpace(action.FunctionID)
		if functionID == "" {
			return nil, fmt.Errorf("invalid actions[%d]: functionId is required", index)
		}
		contract, err := s.contractModel.FindByScopeAndFunctionID(ctx, gameID, env, functionID)
		if err != nil {
			return nil, fmt.Errorf("invalid actions[%d].functionId: %w", index, err)
		}
		if strings.TrimSpace(contract.ResourceKey) != resourceKey {
			return nil, fmt.Errorf("invalid actions[%d].functionId: function does not belong to resource %s", index, resourceKey)
		}
		if contract.Capability != dbenum.CapabilityAction {
			return nil, fmt.Errorf("invalid actions[%d].functionId: function capability must be action", index)
		}
		subject := strings.TrimSpace(action.Subject)
		switch subject {
		case "resource_item", "resource_selection":
			if strings.TrimSpace(action.IdentityInput) == "" || !isJSONPointer(action.IdentityInput) {
				return nil, fmt.Errorf("invalid actions[%d].identityInput: must be a JSON Pointer", index)
			}
		case "none":
			action.IdentityInput = ""
		default:
			return nil, fmt.Errorf("invalid actions[%d].subject: must be resource_item, resource_selection, or none", index)
		}
		key := functionID + ":" + subject + ":" + strings.TrimSpace(action.IdentityInput)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, ActionSemanticInfo{
			FunctionID:    functionID,
			Subject:       subject,
			IdentityInput: strings.TrimSpace(action.IdentityInput),
		})
	}
	return out, nil
}

func (s *Service) validateTaskSemantics(
	ctx context.Context,
	gameID string,
	env string,
	resourceKey string,
	tasks []spec.TaskSemantic,
) ([]spec.TaskSemantic, error) {
	out := make([]spec.TaskSemantic, 0, len(tasks))
	seen := map[string]struct{}{}
	for index, task := range tasks {
		start, err := s.validateSemanticFunctionRef(ctx, gameID, env, resourceKey, task.Start, spec.CapabilityTask, fmt.Sprintf("tasks[%d].start", index))
		if err != nil {
			return nil, err
		}
		task.Start = start.ref
		if !isJSONPointer(task.TaskID.ResultPath) {
			return nil, fmt.Errorf("invalid tasks[%d].taskId.resultPath: must be a JSON Pointer", index)
		}
		if !spec.IsValidJsonScalarType(task.TaskID.ValueType) {
			return nil, fmt.Errorf("invalid tasks[%d].taskId.valueType: must be string, number, integer, or boolean", index)
		}
		if !schemaHasPointer(start.contract.OutputSchema, task.TaskID.ResultPath) {
			return nil, fmt.Errorf("invalid tasks[%d].taskId.resultPath: path not found in start output schema", index)
		}
		status, err := s.validateSemanticFunctionRef(ctx, gameID, env, resourceKey, task.Status.Function, "", fmt.Sprintf("tasks[%d].status.function", index))
		if err != nil {
			return nil, err
		}
		task.Status.Function = status.ref
		if err := validateTaskInputPointer(status.contract, task.Status.TaskIDInput, fmt.Sprintf("tasks[%d].status.taskIdInput", index)); err != nil {
			return nil, err
		}
		if !isJSONPointer(task.Status.StatePath) {
			return nil, fmt.Errorf("invalid tasks[%d].status.statePath: must be a JSON Pointer", index)
		}
		if !schemaHasPointer(status.contract.OutputSchema, task.Status.StatePath) {
			return nil, fmt.Errorf("invalid tasks[%d].status.statePath: path not found in status output schema", index)
		}
		if task.Events != nil {
			events, err := s.validateSemanticFunctionRef(ctx, gameID, env, resourceKey, task.Events.Function, "", fmt.Sprintf("tasks[%d].events.function", index))
			if err != nil {
				return nil, err
			}
			task.Events.Function = events.ref
			if err := validateTaskInputPointer(events.contract, task.Events.TaskIDInput, fmt.Sprintf("tasks[%d].events.taskIdInput", index)); err != nil {
				return nil, err
			}
			if !isJSONPointer(task.Events.EventsPath) {
				return nil, fmt.Errorf("invalid tasks[%d].events.eventsPath: must be a JSON Pointer", index)
			}
			if !schemaHasPointer(events.contract.OutputSchema, task.Events.EventsPath) {
				return nil, fmt.Errorf("invalid tasks[%d].events.eventsPath: path not found in events output schema", index)
			}
		}
		if task.Result != nil {
			result, err := s.validateSemanticFunctionRef(ctx, gameID, env, resourceKey, task.Result.Function, "", fmt.Sprintf("tasks[%d].result.function", index))
			if err != nil {
				return nil, err
			}
			task.Result.Function = result.ref
			if err := validateTaskInputPointer(result.contract, task.Result.TaskIDInput, fmt.Sprintf("tasks[%d].result.taskIdInput", index)); err != nil {
				return nil, err
			}
			if !isJSONPointer(task.Result.ResultPath) {
				return nil, fmt.Errorf("invalid tasks[%d].result.resultPath: must be a JSON Pointer", index)
			}
			if !schemaHasPointer(result.contract.OutputSchema, task.Result.ResultPath) {
				return nil, fmt.Errorf("invalid tasks[%d].result.resultPath: path not found in result output schema", index)
			}
		}
		if task.Cancel != nil {
			cancel, err := s.validateSemanticFunctionRef(ctx, gameID, env, resourceKey, task.Cancel.Function, "", fmt.Sprintf("tasks[%d].cancel.function", index))
			if err != nil {
				return nil, err
			}
			task.Cancel.Function = cancel.ref
			if err := validateTaskInputPointer(cancel.contract, task.Cancel.TaskIDInput, fmt.Sprintf("tasks[%d].cancel.taskIdInput", index)); err != nil {
				return nil, err
			}
		}
		if task.Retry != nil {
			return nil, fmt.Errorf("invalid tasks[%d].retry: retry runtime is not available", index)
		}
		key := strings.TrimSpace(task.Start.FunctionID)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, task)
	}
	return out, nil
}

func (s *Service) validateReportSemantics(
	ctx context.Context,
	gameID string,
	env string,
	resourceKey string,
	reports []spec.ReportSemantic,
) ([]spec.ReportSemantic, error) {
	out := make([]spec.ReportSemantic, 0, len(reports))
	seen := map[string]struct{}{}
	for index, report := range reports {
		query, err := s.validateSemanticFunctionRef(ctx, gameID, env, resourceKey, report.Query, spec.CapabilityReport, fmt.Sprintf("reports[%d].query", index))
		if err != nil {
			return nil, err
		}
		report.Query = query.ref
		if !isJSONPointer(report.DatasetPath) {
			return nil, fmt.Errorf("invalid reports[%d].datasetPath: must be a JSON Pointer", index)
		}
		datasetItemSchema, ok := arrayItemSchemaAtPointer(query.contract.OutputSchema, report.DatasetPath)
		if !ok {
			return nil, fmt.Errorf("invalid reports[%d].datasetPath: path must reference an array in query output schema", index)
		}
		if len(report.Dimensions) == 0 {
			return nil, fmt.Errorf("invalid reports[%d].dimensions: at least one dimension is required", index)
		}
		if len(report.Metrics) == 0 {
			return nil, fmt.Errorf("invalid reports[%d].metrics: at least one metric is required", index)
		}
		dimensions, err := validateJSONPointerList(report.Dimensions, fmt.Sprintf("reports[%d].dimensions", index))
		if err != nil {
			return nil, err
		}
		metrics, err := validateJSONPointerList(report.Metrics, fmt.Sprintf("reports[%d].metrics", index))
		if err != nil {
			return nil, err
		}
		report.Dimensions = dimensions
		report.Metrics = metrics
		for _, pointer := range report.Dimensions {
			if !strings.HasPrefix(pointer, "/") {
				return nil, fmt.Errorf("invalid reports[%d]: dataset field pointer %s must be relative to dataset item and start with /", index, pointer)
			}
			if !schemaObjectHasPointer(datasetItemSchema, pointer) {
				return nil, fmt.Errorf("invalid reports[%d].dimensions: pointer %s not found in dataset item schema", index, pointer)
			}
		}
		for _, pointer := range report.Metrics {
			if !strings.HasPrefix(pointer, "/") {
				return nil, fmt.Errorf("invalid reports[%d]: dataset field pointer %s must be relative to dataset item and start with /", index, pointer)
			}
			if !schemaObjectHasPointer(datasetItemSchema, pointer) {
				return nil, fmt.Errorf("invalid reports[%d].metrics: pointer %s not found in dataset item schema", index, pointer)
			}
		}
		key := strings.TrimSpace(report.Query.FunctionID)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, report)
	}
	return out, nil
}

type semanticFunctionRef struct {
	ref      spec.FunctionRef
	contract *model.FunctionContract
}

func (s *Service) validateSemanticFunctionRef(
	ctx context.Context,
	gameID string,
	env string,
	resourceKey string,
	ref spec.FunctionRef,
	requiredCapability spec.CapabilityKind,
	field string,
) (semanticFunctionRef, error) {
	functionID := strings.TrimSpace(ref.FunctionID)
	if functionID == "" {
		return semanticFunctionRef{}, fmt.Errorf("invalid %s.functionId: functionId is required", field)
	}
	contract, err := s.contractModel.FindByScopeAndFunctionID(ctx, gameID, env, functionID)
	if err != nil {
		return semanticFunctionRef{}, fmt.Errorf("invalid %s.functionId: %w", field, err)
	}
	if strings.TrimSpace(contract.ResourceKey) != resourceKey {
		return semanticFunctionRef{}, fmt.Errorf("invalid %s.functionId: function does not belong to resource %s", field, resourceKey)
	}
	if requiredCapability != "" && spec.CapabilityKind(contract.Capability.String()) != requiredCapability {
		return semanticFunctionRef{}, fmt.Errorf("invalid %s.functionId: function capability must be %s", field, requiredCapability)
	}
	if !contract.Enabled {
		return semanticFunctionRef{}, fmt.Errorf("invalid %s.functionId: function %s is disabled", field, contract.FunctionID)
	}
	return semanticFunctionRef{
		ref: spec.FunctionRef{
			FunctionID:         strings.TrimSpace(contract.FunctionID),
			ContractVersion:    strings.TrimSpace(contract.Version),
			InputSchemaDigest:  digestRawJSON(contract.InputSchema),
			OutputSchemaDigest: digestRawJSON(contract.OutputSchema),
		},
		contract: contract,
	}, nil
}

func validateTaskInputPointer(contract *model.FunctionContract, pointer string, field string) error {
	if !isJSONPointer(pointer) {
		return fmt.Errorf("invalid %s: must be a JSON Pointer", field)
	}
	if contract != nil && !schemaHasPointer(contract.InputSchema, pointer) {
		return fmt.Errorf("invalid %s: path not found in function input schema", field)
	}
	return nil
}

func validateJSONPointerList(values []string, field string) ([]string, error) {
	out := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for index, value := range values {
		pointer := strings.TrimSpace(value)
		if pointer == "" || !strings.HasPrefix(pointer, "/") || !isJSONPointer(pointer) {
			return nil, fmt.Errorf("invalid %s[%d]: must be a non-empty JSON Pointer", field, index)
		}
		if _, ok := seen[pointer]; ok {
			continue
		}
		seen[pointer] = struct{}{}
		out = append(out, pointer)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("invalid %s: at least one JSON Pointer is required", field)
	}
	return out, nil
}

func compactJSONPointers(values []string) []string {
	out := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for _, value := range values {
		pointer := strings.TrimSpace(value)
		if pointer == "" || !strings.HasPrefix(pointer, "/") || !isJSONPointer(pointer) {
			continue
		}
		if _, ok := seen[pointer]; ok {
			continue
		}
		seen[pointer] = struct{}{}
		out = append(out, pointer)
	}
	return out
}

func parseActionSemantics(raw []byte) []ActionSemanticInfo {
	if len(raw) == 0 {
		return nil
	}
	var actions []ActionSemanticInfo
	if err := json.Unmarshal(raw, &actions); err != nil {
		return nil
	}
	return compactActionSemantics(actions)
}

func parseTaskSemantics(raw []byte) []spec.TaskSemantic {
	if len(raw) == 0 {
		return nil
	}
	var tasks []spec.TaskSemantic
	if err := json.Unmarshal(raw, &tasks); err != nil {
		return nil
	}
	out := make([]spec.TaskSemantic, 0, len(tasks))
	for _, task := range tasks {
		if strings.TrimSpace(task.Start.FunctionID) == "" {
			continue
		}
		out = append(out, task)
	}
	return out
}

func parseReportSemantics(raw []byte) []spec.ReportSemantic {
	if len(raw) == 0 {
		return nil
	}
	var reports []spec.ReportSemantic
	if err := json.Unmarshal(raw, &reports); err != nil {
		return nil
	}
	out := make([]spec.ReportSemantic, 0, len(reports))
	for _, report := range reports {
		if strings.TrimSpace(report.Query.FunctionID) == "" {
			continue
		}
		report.Dimensions = compactJSONPointers(report.Dimensions)
		report.Metrics = compactJSONPointers(report.Metrics)
		out = append(out, report)
	}
	return out
}

func compactActionSemantics(actions []ActionSemanticInfo) []ActionSemanticInfo {
	out := make([]ActionSemanticInfo, 0, len(actions))
	for _, action := range actions {
		action.FunctionID = strings.TrimSpace(action.FunctionID)
		action.Subject = strings.TrimSpace(action.Subject)
		action.IdentityInput = strings.TrimSpace(action.IdentityInput)
		if action.FunctionID == "" || action.Subject == "" {
			continue
		}
		if action.Subject == "none" {
			action.IdentityInput = ""
		}
		out = append(out, action)
	}
	return out
}

func isJSONPointer(path string) bool {
	return path == "" || strings.HasPrefix(strings.TrimSpace(path), "/")
}

func schemaHasPointer(raw []byte, pointer string) bool {
	if len(raw) == 0 {
		return true
	}
	var root map[string]json.RawMessage
	if err := json.Unmarshal(raw, &root); err != nil {
		return true
	}
	return schemaObjectHasPointer(root, pointer)
}

func schemaObjectHasPointer(root map[string]json.RawMessage, pointer string) bool {
	if !isJSONPointer(pointer) {
		return false
	}
	if pointer == "" {
		return true
	}
	current := root
	for _, token := range jsonPointerTokens(pointer) {
		properties := parseRawObject(current["properties"])
		if len(properties) == 0 {
			return false
		}
		next := parseRawObject(properties[token])
		if len(next) == 0 {
			return false
		}
		current = next
	}
	return true
}

func arrayItemSchemaAtPointer(raw []byte, pointer string) (map[string]json.RawMessage, bool) {
	if len(raw) == 0 || !isJSONPointer(pointer) {
		return nil, false
	}
	var root map[string]json.RawMessage
	if err := json.Unmarshal(raw, &root); err != nil {
		return nil, false
	}
	node := root
	if pointer != "" {
		for _, token := range jsonPointerTokens(pointer) {
			properties := parseRawObject(node["properties"])
			if len(properties) == 0 {
				return nil, false
			}
			node = parseRawObject(properties[token])
			if len(node) == 0 {
				return nil, false
			}
		}
	}
	if schemaStringValue(node["type"]) != "array" {
		return nil, false
	}
	items := parseRawObject(node["items"])
	return items, len(items) > 0
}

func parseRawObject(raw json.RawMessage) map[string]json.RawMessage {
	if len(raw) == 0 {
		return nil
	}
	var out map[string]json.RawMessage
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil
	}
	return out
}

func schemaStringValue(raw json.RawMessage) string {
	var value string
	_ = json.Unmarshal(raw, &value)
	return value
}

func jsonPointerTokens(path string) []string {
	if path == "" {
		return nil
	}
	parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
	for i, part := range parts {
		parts[i] = strings.ReplaceAll(strings.ReplaceAll(part, "~1", "/"), "~0", "~")
	}
	return parts
}

func digestRawJSON(raw []byte) string {
	if len(raw) == 0 {
		return ""
	}
	return fmt.Sprintf("%x", sha256Bytes(raw))
}

func (s *Service) buildAffectedPages(ctx context.Context, gameID, env, resourceKey string) ([]AffectedPageInfo, error) {
	byKey := map[string]*AffectedPageInfo{}
	drafts, err := s.pageModel.ListByScope(ctx, gameID, env)
	if err != nil {
		if isMissingTableErr(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("list page drafts: %w", err)
	}
	for _, draft := range drafts {
		if strings.TrimSpace(draft.ResourceKey) != resourceKey {
			continue
		}
		byKey["draft:"+draft.PageKey] = &AffectedPageInfo{
			PageKey:          draft.PageKey,
			PageType:         draft.Type,
			Title:            draft.GetTitle(),
			Kind:             "draft",
			Status:           draft.Status,
			DraftRevision:    draft.DraftRevision,
			PublishedVersion: draft.PublishedVersion,
			UpdatedAt:        draft.UpdatedAt.Format(time.RFC3339),
		}
	}

	publishedPages, err := s.publishedModel.ListByScope(ctx, gameID, env)
	if err != nil {
		if isMissingTableErr(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("list published pages: %w", err)
	}
	functions := s.functionSpecsByID(ctx, gameID, env)
	for _, published := range publishedPages {
		pageSpec := parsePublishedPageSpec(published)
		if strings.TrimSpace(pageSpec.ResourceKey) != resourceKey {
			continue
		}
		bindingFreshness := freshness.EvaluatePublishedBindings(pageSpec.Bindings, parseBindingContracts(published.BindingContractsJSON), functions)
		byKey["published:"+published.PageKey] = &AffectedPageInfo{
			PageKey:          published.PageKey,
			PageType:         string(pageSpec.Type),
			Title:            pageSpec.Title,
			Kind:             "published",
			Status:           activeStatus(published.Active),
			PublishedVersion: published.Version,
			Stale:            len(bindingFreshness) > 0,
			BindingFreshness: bindingFreshness,
			UpdatedAt:        published.PublishedAt.Format(time.RFC3339),
		}
	}

	proposals, err := s.proposalModel.ListByScopeAndResourceKey(ctx, gameID, env, resourceKey)
	if err != nil {
		if isMissingTableErr(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("list proposals: %w", err)
	}
	for _, proposal := range proposals {
		byKey["proposal:"+proposal.ProposalKey] = &AffectedPageInfo{
			PageKey:         proposal.PageKey,
			PageType:        proposal.PageType,
			Title:           toStringMap(proposal.Title),
			Kind:            "proposal",
			ProposalKey:     proposal.ProposalKey,
			ProposalQuality: proposal.Quality,
			ProposalStatus:  proposal.Status.String(),
			Status:          proposal.Status.String(),
			UpdatedAt:       proposal.UpdatedAt.Format(time.RFC3339),
		}
	}

	items := make([]AffectedPageInfo, 0, len(byKey))
	for _, item := range byKey {
		items = append(items, *item)
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Kind != items[j].Kind {
			return affectedKindOrder(items[i].Kind) < affectedKindOrder(items[j].Kind)
		}
		return items[i].PageKey < items[j].PageKey
	})
	return items, nil
}

func (s *Service) functionSpecsByID(ctx context.Context, gameID, env string) map[string]spec.FunctionSpec {
	contracts, err := s.contractModel.ListByScope(ctx, gameID, env)
	if err != nil {
		return map[string]spec.FunctionSpec{}
	}
	out := make(map[string]spec.FunctionSpec, len(contracts))
	for _, contract := range contracts {
		if contract == nil || strings.TrimSpace(contract.FunctionID) == "" {
			continue
		}
		out[contract.FunctionID] = spec.FunctionSpec{
			ID:           contract.FunctionID,
			Version:      contract.Version,
			Enabled:      contract.Enabled,
			InputSchema:  spec.JSONSchema(contract.InputSchema),
			OutputSchema: spec.JSONSchema(contract.OutputSchema),
			Risk:         spec.RiskLevel(contract.Risk.String()),
			Permission:   strings.TrimSpace(contract.Permission),
		}
	}
	return out
}

func parsePublishedPageSpec(published model.PublishedPageSpec) spec.PageSpec {
	var pageSpec spec.PageSpec
	if strings.TrimSpace(published.SpecJSON) != "" {
		_ = json.Unmarshal([]byte(published.SpecJSON), &pageSpec)
	}
	if strings.TrimSpace(pageSpec.PageKey) == "" {
		pageSpec.PageKey = published.PageKey
	}
	return pageSpec
}

func parseBindingContracts(raw string) []spec.BindingContractSnapshot {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	var contracts []spec.BindingContractSnapshot
	_ = json.Unmarshal([]byte(raw), &contracts)
	return contracts
}

func activeStatus(active bool) string {
	if active {
		return "active"
	}
	return "inactive"
}

func affectedKindOrder(kind string) int {
	switch kind {
	case "published":
		return 0
	case "draft":
		return 1
	case "proposal":
		return 2
	default:
		return 3
	}
}

func isMissingTableErr(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "no such table") ||
		strings.Contains(message, "does not exist") ||
		strings.Contains(message, "undefined table")
}

func decodeDiagnostics(raw []byte, fallbackFunctionID string) []DiagnosticInfo {
	if len(raw) == 0 {
		return nil
	}
	var values []spec.Diagnostic
	if err := json.Unmarshal(raw, &values); err != nil {
		return []DiagnosticInfo{{
			Code:       "diagnostic_parse_failed",
			Severity:   string(spec.SeverityWarning),
			Message:    "diagnostics payload is not readable",
			FunctionID: fallbackFunctionID,
		}}
	}
	out := make([]DiagnosticInfo, 0, len(values))
	for _, value := range values {
		functionID := value.FunctionID
		if functionID == "" {
			functionID = fallbackFunctionID
		}
		out = append(out, DiagnosticInfo{
			Code:       value.Code,
			Severity:   string(value.Severity),
			Message:    value.Message,
			FunctionID: functionID,
			Field:      value.Field,
		})
	}
	return out
}

func countUnresolvedConflicts(raw []byte) int {
	if len(raw) == 0 {
		return 0
	}
	var conflicts []spec.SemanticConflict
	if err := json.Unmarshal(raw, &conflicts); err != nil {
		return 0
	}
	count := 0
	for _, conflict := range conflicts {
		if conflict.Resolution == "" {
			count++
		}
	}
	return count
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

// humanizeResourceKey turns "inventory" / "player_item" into "Inventory" /
// "Player Item" so the catalog still shows a readable name before platform
// review fills in proper localized labels.
func humanizeResourceKey(key string) string {
	key = strings.Trim(strings.TrimSpace(key), "._-")
	if key == "" {
		return ""
	}
	parts := strings.FieldsFunc(key, func(r rune) bool {
		return r == '.' || r == '_' || r == '-'
	})
	for i := range parts {
		if parts[i] == "" {
			continue
		}
		parts[i] = strings.ToUpper(parts[i][:1]) + parts[i][1:]
	}
	return strings.Join(parts, " ")
}

// categoryKeyForResource falls back to the key prefix ("mail.template" ->
// "mail") when no reviewed category exists, matching the page generator's
// InferCategoryFromKey behavior.
func categoryKeyForResource(resourceKey, reviewed string) string {
	if reviewed != "" {
		return reviewed
	}
	resourceKey = strings.TrimSpace(resourceKey)
	if idx := strings.Index(resourceKey, "."); idx > 0 {
		return resourceKey[:idx]
	}
	return resourceKey
}

// labelsForResource returns reviewed labels when present, otherwise a
// humanized fallback derived from the resource key. Derived values are
// display-only: the stored capability keeps empty labels until reviewed.
func labelsForResource(labels map[string]interface{}, resourceKey string) map[string]string {
	reviewed := toStringMap(labels)
	if len(reviewed) > 0 {
		return reviewed
	}
	if text := humanizeResourceKey(resourceKey); text != "" {
		return map[string]string{"zh-CN": text, "en-US": text}
	}
	return nil
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

func (s *Service) findSemanticsOptional(ctx context.Context, gameID, env, resourceKey string) (*model.CapabilitySemantics, error) {
	semantics, err := s.semanticsModel.FindByScopeAndResourceKey(ctx, gameID, env, resourceKey)
	if err == nil {
		return semantics, nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return nil, fmt.Errorf("find semantics for resource %s: %w", resourceKey, err)
}

func semanticSourceDigest(ctx context.Context, contractModel *model.FunctionContractModel, gameID, env, resourceKey string) string {
	if contractModel == nil {
		return ""
	}
	contracts, err := contractModel.ListByResourceKey(ctx, gameID, env, resourceKey)
	if err != nil {
		return ""
	}
	raw, err := json.Marshal(contracts)
	if err != nil {
		return ""
	}
	return fmt.Sprintf("%x", sha256Bytes(raw))
}

func sha256Bytes(raw []byte) []byte {
	sum := sha256.Sum256(raw)
	return sum[:]
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
	Field      string `json:"field"`
	Source     string `json:"source"`
	Confidence string `json:"confidence"`
	Status     string `json:"status"`
	Value      string `json:"value,omitempty"`
	UpdatedAt  string `json:"updatedAt"`
	UpdatedBy  string `json:"updatedBy"`
}

// ListConflictsRequest is the request for listing conflicts.
type ListConflictsRequest struct {
	GameID      string `json:"-"`
	Env         string `json:"-"`
	ResourceKey string `json:"-" uri:"resourceKey" binding:"required"`
}

// ListConflictsResponse is the response for listing conflicts.
type ListConflictsResponse struct {
	Conflicts  []ConflictInfo   `json:"conflicts"`
	Provenance []ProvenanceInfo `json:"provenance"`
}

// ListConflicts returns conflicts and provenance for a resource.
func (s *Service) ListConflicts(ctx context.Context, req *ListConflictsRequest) (*ListConflictsResponse, error) {
	semantics, err := s.semanticsModel.FindByScopeAndResourceKey(ctx, svc.ResolveGameID(ctx, req.GameID), svc.ResolveEnv(ctx, req.Env), req.ResourceKey)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return &ListConflictsResponse{
				Conflicts:  []ConflictInfo{},
				Provenance: []ProvenanceInfo{},
			}, nil
		}
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
	GameID       string `json:"-"`
	Env          string `json:"-"`
	ResourceKey  string `json:"-" uri:"resourceKey" binding:"required"`
	Field        string `json:"field" uri:"field" binding:"required"`
	ChosenSource string `json:"chosenSource"` // platform_review|sdk_explicit|openapi_rest
	Reason       string `json:"reason,omitempty"`
}

// ResolveConflictResponse is the response for resolving a conflict.
type ResolveConflictResponse struct {
	Message string `json:"message"`
}

// ResolveConflict resolves a semantic conflict by choosing a source.
func (s *Service) ResolveConflict(ctx context.Context, req *ResolveConflictRequest) (*ResolveConflictResponse, error) {
	actor := actorFromContext(ctx)
	chosenSource := spec.SemanticSource(strings.TrimSpace(req.ChosenSource))
	if !isValidSemanticSource(chosenSource) {
		return nil, fmt.Errorf("invalid chosenSource %s", req.ChosenSource)
	}
	semantics, err := s.semanticsModel.FindByScopeAndResourceKey(ctx, svc.ResolveGameID(ctx, req.GameID), svc.ResolveEnv(ctx, req.Env), req.ResourceKey)
	if err != nil {
		return nil, fmt.Errorf("find semantics: %w", err)
	}
	if sourceDigest := semanticSourceDigest(ctx, s.contractModel, svc.ResolveGameID(ctx, req.GameID), svc.ResolveEnv(ctx, req.Env), req.ResourceKey); sourceDigest != "" {
		semantics.SourceDigest = sourceDigest
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
			if _, ok := c.Values[chosenSource]; !ok {
				return nil, fmt.Errorf("source %s not found in conflict values", req.ChosenSource)
			}

			// Resolve conflict
			conflicts[i].Resolution = chosenSource
			conflicts[i].ResolvedAt = time.Now().UTC().Format(time.RFC3339)
			conflicts[i].ResolvedBy = actor
			if err := applySemanticFieldValue(semantics, req.Field, c.Values[chosenSource]); err != nil {
				return nil, err
			}

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
				prov.SourceDigest = semantics.SourceDigest
				prov.Confidence = confidenceForSource(chosenSource)
				prov.Status = "effective"
				prov.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
				prov.UpdatedBy = actor
			} else {
				provenance[req.Field] = provenanceRecord(req.Field, chosenSource, semantics.SourceDigest, c.Values[chosenSource], confidenceForSource(chosenSource), "effective", actor)
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
	semantics.UpdatedBy = actor

	// Update semantics
	if err := s.semanticsModel.UpsertSemantics(ctx, semantics); err != nil {
		return nil, fmt.Errorf("update semantics: %w", err)
	}
	if err := s.createSemanticVersion(ctx, semantics, req.Reason, actor); err != nil {
		return nil, err
	}
	if err := s.rebuildProposals(ctx, svc.ResolveGameID(ctx, req.GameID), svc.ResolveEnv(ctx, req.Env), req.ResourceKey); err != nil {
		return nil, err
	}

	// Audit log
	if s.auditService != nil {
		if _, err := s.auditService.Log(ctx, audit.EventSemanticConflictResolve,
			audit.WithActorID(actor, "user", actor),
			audit.WithResourceID("resource_catalog", req.ResourceKey),
			audit.WithDetails(map[string]interface{}{
				"field":         req.Field,
				"chosen_source": req.ChosenSource,
				"reason":        req.Reason,
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

func isValidSemanticSource(source spec.SemanticSource) bool {
	switch source {
	case spec.SemanticSourcePlatformReview, spec.SemanticSourceSDKExplicit, spec.SemanticSourceOpenAPIRest:
		return true
	default:
		return false
	}
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

func actorFromContext(ctx context.Context) string {
	actor, err := logicutils.CurrentUsername(ctx)
	if err != nil || strings.TrimSpace(actor) == "" {
		return "system"
	}
	return strings.TrimSpace(actor)
}

func provenanceRecord(field string, source spec.SemanticSource, sourceDigest string, value json.RawMessage, confidence string, status string, actor string) *spec.SemanticProvenance {
	return &spec.SemanticProvenance{
		Field:        field,
		Source:       source,
		SourceDigest: sourceDigest,
		Confidence:   confidence,
		Status:       status,
		Value:        value,
		UpdatedAt:    time.Now().UTC().Format(time.RFC3339),
		UpdatedBy:    actor,
	}
}

func parseProvenance(raw []byte) map[string]*spec.SemanticProvenance {
	provenance := map[string]*spec.SemanticProvenance{}
	if len(raw) == 0 {
		return provenance
	}
	_ = json.Unmarshal(raw, &provenance)
	if provenance == nil {
		return map[string]*spec.SemanticProvenance{}
	}
	return provenance
}

func rawJSONString(value string) json.RawMessage {
	return json.RawMessage(strconv.Quote(value))
}

func rawJSONUint(value uint) json.RawMessage {
	return json.RawMessage(strconv.FormatUint(uint64(value), 10))
}

func applySemanticFieldValue(semantics *model.CapabilitySemantics, field string, raw json.RawMessage) error {
	switch strings.TrimSpace(field) {
	case "identityField":
		return assignString(raw, &semantics.IdentityField)
	case "identityFieldType":
		return assignString(raw, &semantics.IdentityFieldType)
	case "identityPath":
		return assignString(raw, &semantics.IdentityPath)
	case "collectionQueryID", "collectionQueryId":
		return assignUint(raw, &semantics.CollectionQueryID)
	case "collectionPath":
		return assignString(raw, &semantics.CollectionPath)
	case "pageFieldName":
		return assignString(raw, &semantics.PageFieldName)
	case "pageSizeFieldName":
		return assignString(raw, &semantics.PageSizeFieldName)
	case "itemsFieldName":
		return assignString(raw, &semantics.ItemsFieldName)
	case "totalFieldName":
		return assignString(raw, &semantics.TotalFieldName)
	case "itemQueryID", "itemQueryId":
		return assignUint(raw, &semantics.ItemQueryID)
	case "itemPath":
		return assignString(raw, &semantics.ItemPath)
	case "createID", "createId":
		return assignUint(raw, &semantics.CreateID)
	case "updateID", "updateId":
		return assignUint(raw, &semantics.UpdateID)
	case "deleteID", "deleteId":
		return assignUint(raw, &semantics.DeleteID)
	case "actions":
		semantics.Actions = datatypes.JSON(raw)
	case "tasks":
		semantics.Tasks = datatypes.JSON(raw)
	case "reports":
		semantics.Reports = datatypes.JSON(raw)
	default:
		return fmt.Errorf("unsupported semantic conflict field %s", field)
	}
	return nil
}

func assignString(raw json.RawMessage, target *string) error {
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return fmt.Errorf("semantic field must be string: %w", err)
	}
	*target = strings.TrimSpace(value)
	return nil
}

func assignUint(raw json.RawMessage, target *uint) error {
	var value uint
	if err := json.Unmarshal(raw, &value); err != nil {
		return fmt.Errorf("semantic field must be unsigned integer: %w", err)
	}
	*target = value
	return nil
}

func (s *Service) createSemanticVersion(ctx context.Context, semantics *model.CapabilitySemantics, reason string, actor string) error {
	if s.versionModel == nil || semantics == nil {
		return nil
	}
	version := &model.CapabilitySemanticVersion{
		SemanticsID:  semantics.ID,
		Version:      semantics.Version,
		Semantics:    capabilitySemanticsJSON(semantics),
		SourceDigest: semantics.SourceDigest,
		ChangeReason: strings.TrimSpace(reason),
		CreatedBy:    actor,
	}
	if version.ChangeReason == "" {
		version.ChangeReason = "resource catalog semantic update"
	}
	if err := s.versionModel.CreateVersion(ctx, version); err != nil {
		return fmt.Errorf("create semantic version: %w", err)
	}
	return nil
}

func capabilitySemanticsJSON(value *model.CapabilitySemantics) datatypes.JSON {
	raw, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	return datatypes.JSON(raw)
}

func (s *Service) rebuildProposals(ctx context.Context, gameID string, env string, resourceKey string) error {
	if s.contractService == nil {
		return nil
	}
	if err := s.contractService.RebuildProposalsForResource(ctx, gameID, env, resourceKey); err != nil {
		return fmt.Errorf("rebuild proposals: %w", err)
	}
	return nil
}
