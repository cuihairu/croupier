package resourcecatalog

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/cuihairu/croupier/internal/audit"
	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/model"
	contractsvc "github.com/cuihairu/croupier/internal/service"
	"github.com/cuihairu/croupier/internal/svc"
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

	// 分类聚合的进程内缓存（#14）：下拉选项要展示**全量** distinct 分类，
	// 不能从（已按分类过滤的）列表数据推导。按 (game, env) 键缓存，短 TTL
	// + 语义审核写路径主动失效。Service 在路由注册期构造一次，缓存生命周期
	// 与进程一致；HA 多实例各自持有副本，下拉选项短时陈旧可接受。
	categoryCacheMu sync.Mutex
	categoryCache   map[string]cachedCategories
}

// categoryCacheTTL bounds staleness of the aggregated category options.
const categoryCacheTTL = 30 * time.Second

type cachedCategories struct {
	items    []CategoryOption
	expireAt time.Time
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
		categoryCache:   make(map[string]cachedCategories),
	}
}

// Categories returns the distinct category keys for a scope with resource
// counts, served from a short-lived in-memory cache. The dropdown must offer
// the full category set regardless of the currently applied category filter
// (deriving options client-side from the filtered list collapses them to one).
func (s *Service) Categories(ctx context.Context, gameID, env string) (*CategoriesResponse, error) {
	gameID = svc.ResolveGameID(ctx, gameID)
	env = svc.ResolveEnv(ctx, env)
	cacheKey := gameID + "\x00" + env

	s.categoryCacheMu.Lock()
	if cached, ok := s.categoryCache[cacheKey]; ok && time.Now().Before(cached.expireAt) {
		items := cached.items
		s.categoryCacheMu.Unlock()
		return &CategoriesResponse{Items: items}, nil
	}
	s.categoryCacheMu.Unlock()

	capabilities, err := s.capabilityModel.ListByScope(ctx, gameID, env)
	if err != nil {
		return nil, fmt.Errorf("list resource capabilities for categories: %w", err)
	}

	counts := make(map[string]int)
	for _, cap := range capabilities {
		counts[categoryKeyForResource(cap.ResourceKey, cap.CategoryKey)]++
	}
	items := make([]CategoryOption, 0, len(counts))
	for key, count := range counts {
		items = append(items, CategoryOption{CategoryKey: key, Count: count})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].CategoryKey < items[j].CategoryKey })

	s.categoryCacheMu.Lock()
	s.categoryCache[cacheKey] = cachedCategories{items: items, expireAt: time.Now().Add(categoryCacheTTL)}
	s.categoryCacheMu.Unlock()

	return &CategoriesResponse{Items: items}, nil
}

// invalidateCategoryCache drops cached category options for a scope; called on
// write paths that may change a resource's category (semantic review). Callers
// pass the same resolved (gameID, env) used for the cache key.
func (s *Service) invalidateCategoryCache(gameID, env string) {
	s.categoryCacheMu.Lock()
	delete(s.categoryCache, gameID+"\x00"+env)
	s.categoryCacheMu.Unlock()
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
		// Apply category filter (compare against derived key to match dropdown options)
		resolvedCategory := categoryKeyForResource(cap.ResourceKey, cap.CategoryKey)
		if req.Category != "" && resolvedCategory != req.Category {
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
		// 唯一调用方均在 req.XXX > 0 守卫内传入 req.XXX，value 恒非 0，
		// 零值守卫分支为死代码，已删除。
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
		// actions 为纯 string/结构体字段组成的 []spec.ActionSemantic，Marshal 恒成功，
		// err 分支为死代码，已删除。
		raw, _ := json.Marshal(actions)
		semantics.Actions = model.JSON(raw)
		changedFields = append(changedFields, "actions")
		provenance["actions"] = provenanceRecord("actions", spec.SemanticSourcePlatformReview, sourceDigest, json.RawMessage(raw), "high", "effective", actor)
	}
	if req.Tasks != nil {
		tasks, err := s.validateTaskSemantics(ctx, svc.ResolveGameID(ctx, req.GameID), svc.ResolveEnv(ctx, req.Env), req.ResourceKey, req.Tasks)
		if err != nil {
			return nil, err
		}
		// tasks 为纯 string/结构体字段组成的 []spec.TaskSemantic，Marshal 恒成功，
		// err 分支为死代码，已删除。
		raw, _ := json.Marshal(tasks)
		semantics.Tasks = model.JSON(raw)
		changedFields = append(changedFields, "tasks")
		provenance["tasks"] = provenanceRecord("tasks", spec.SemanticSourcePlatformReview, sourceDigest, json.RawMessage(raw), "high", "effective", actor)
	}
	if req.Reports != nil {
		reports, err := s.validateReportSemantics(ctx, svc.ResolveGameID(ctx, req.GameID), svc.ResolveEnv(ctx, req.Env), req.ResourceKey, req.Reports)
		if err != nil {
			return nil, err
		}
		// reports 为纯 string 字段组成的 []spec.ReportSemantic，Marshal 恒成功，
		// err 分支为死代码，已删除。
		raw, _ := json.Marshal(reports)
		semantics.Reports = model.JSON(raw)
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
		// provenance 的 RawMessage 值要么来自 Unmarshal 产物（已验证合法 JSON），
		// 要么由 rawJSONString/rawJSONUint（strconv.Quote/FormatUint）构造，
		// 均为合法 JSON 片段，Marshal 恒成功，err 分支为死代码，已删除。
		raw, _ := json.Marshal(provenance)
		semantics.Provenance = raw
	}

	// Save semantics
	// 人工编辑路径：显式保存总视为一次变更（零改动提交也要留版本历史）。
	if _, err := s.semanticsModel.UpsertSemantics(ctx, semantics); err != nil {
		return nil, fmt.Errorf("upsert semantics: %w", err)
	}
	if err := s.createSemanticVersion(ctx, semantics, req.ChangeReason, actor); err != nil {
		return nil, err
	}
	if err := s.rebuildProposals(ctx, svc.ResolveGameID(ctx, req.GameID), svc.ResolveEnv(ctx, req.Env), req.ResourceKey); err != nil {
		return nil, err
	}

	// 审核可改写 categoryKey：失效该 scope 的分类聚合缓存，避免下拉 30s 内
	// 仍展示旧分类。
	s.invalidateCategoryCache(svc.ResolveGameID(ctx, req.GameID), svc.ResolveEnv(ctx, req.Env))

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
			// 副作用（语义更新）已完成：呈现缺口而非静默，也不返回 5xx。
			s.auditService.ReportWriteGap(ctx, audit.EventSemanticUpdate, err)
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
