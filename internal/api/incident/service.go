// Package incident implements the batch 1 incident registry and category
// management APIs of the incident report system
// (docs/design/incident-reports.md §2/§3/§4). Categories are stored in the
// database and configurable (enum validation = the allowed values of the
// config table); incidents link by category_id (renames do not break the
// chain), and deleting a category cascades its rows into the built-in
// "uncategorized" entry.
package incident

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/cuihairu/croupier/internal/audit"
	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/logic/utils"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
	"gorm.io/gorm"
)

// auditIncident is the unified audit trail for sensitive write paths in the
// incident domain (AUD-049): category changes, incident registration and state
// transitions share one event type; success/failure/denied differ only by
// outcome.
func (s *Service) auditIncident(ctx context.Context, event audit.AuditEventType, state, action, resourceID string, extra map[string]interface{}) {
	details := map[string]interface{}{"action": action}
	for k, v := range extra {
		details[k] = v
	}
	actor := ""
	if name, err := utils.CurrentUsername(ctx); err == nil {
		actor = name
	}
	audit.LogWrite(s.svcCtx.AuditService, ctx, event, state, actor, "incident", resourceID, details)
}

// categorySlugPattern is the closed-form constraint on a category slug
// (lowercase letters/digits/hyphens).
var categorySlugPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,63}$`)

// Service implements the incident registry API.
type Service struct {
	svcCtx *svc.ServiceContext
}

// NewService creates an incident service.
func NewService(svcCtx *svc.ServiceContext) *Service {
	return &Service{svcCtx: svcCtx}
}

// requireIncidentScope is row-level scope validation for a single incident row
// (aligned with requireTaskScope in the task domain): when the current request
// carries a game/env scope, the row's own attribution must match it or the
// request gets 403. The error does not echo the target row's game/env, so an
// unauthorized caller cannot enumerate resources in other scopes via the
// response. The external token surface (external.go) has no header scope and
// relies on token-based filtering; it does not pass through this guard.
func requireIncidentScope(ctx context.Context, row *model.Incident) error {
	scope := svc.GameScopeFromContext(ctx)
	if row == nil || ((scope.GameID != "" || scope.Env != "") && !svc.ScopeMatches(ctx, row.GameID, row.Env)) {
		return utils.NewPermissionDenied("无权访问该事故（当前游戏/环境与事故作用域不一致）", nil)
	}
	return nil
}

// ---- Category management ----

// ListCategories returns all categories (including disabled rows; enabledOnly
// feeds the registration selector).
func (s *Service) ListCategories(ctx context.Context, enabledOnly bool) (*CategoryListResponse, error) {
	rows, err := model.NewIncidentCategoryModel(s.svcCtx.DB).List(ctx, enabledOnly)
	if err != nil {
		return nil, err
	}
	items := make([]CategoryDTO, 0, len(rows))
	for i := range rows {
		items = append(items, toCategoryDTO(&rows[i]))
	}
	return &CategoryListResponse{Items: items, Total: int64(len(items))}, nil
}

// CreateCategory creates a category (slug validation + uniqueness).
func (s *Service) CreateCategory(ctx context.Context, req *CategoryUpsertRequest) (*CategoryDTO, error) {
	if req.Name == nil || strings.TrimSpace(*req.Name) == "" {
		return nil, errorx.NewBadRequest("类别名称不能为空")
	}
	name := strings.TrimSpace(*req.Name)
	slug := strings.TrimSpace(req.Slug)
	if !categorySlugPattern.MatchString(slug) {
		return nil, errorx.NewBadRequest("类别 slug 须为小写字母/数字/连字符（1-64 位）")
	}
	m := model.NewIncidentCategoryModel(s.svcCtx.DB)
	if _, err := m.GetBySlug(ctx, slug); err == nil {
		return nil, errorx.NewConflict("类别 slug 已存在: " + slug)
	}
	row := &model.IncidentCategory{Name: name, Slug: slug, Enabled: true}
	applyCategoryUpsert(row, req)
	if err := m.Create(ctx, row); err != nil {
		if isUniqueViolation(err) {
			return nil, errorx.NewConflict("类别名称或 slug 已存在")
		}
		return nil, err
	}
	s.auditIncident(ctx, audit.EventIncidentCategory, audit.StateSuccess, "category_create", fmt.Sprint(row.ID), map[string]interface{}{"slug": row.Slug, "name": row.Name})
	dto := toCategoryDTO(row)
	return &dto, nil
}

// UpdateCategory updates a category (the slug is immutable; the uncategorized
// row's name can be renamed but the row can never be deleted).
func (s *Service) UpdateCategory(ctx context.Context, id uint, req *CategoryUpsertRequest) (*CategoryDTO, error) {
	m := model.NewIncidentCategoryModel(s.svcCtx.DB)
	row, err := m.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if req.Slug != "" && strings.TrimSpace(req.Slug) != row.Slug {
		return nil, errorx.NewBadRequest("类别 slug 建行后不可改")
	}
	if req.Name != nil && strings.TrimSpace(*req.Name) == "" {
		return nil, errorx.NewBadRequest("类别名称不能为空")
	}
	updates := map[string]interface{}{}
	applyCategoryUpdates(updates, req)
	if err := m.Update(ctx, id, updates); err != nil {
		if isUniqueViolation(err) {
			return nil, errorx.NewConflict("类别名称已存在")
		}
		return nil, err
	}
	row, err = m.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	s.auditIncident(ctx, audit.EventIncidentCategory, audit.StateSuccess, "category_update", fmt.Sprint(id), map[string]interface{}{"slug": row.Slug})
	dto := toCategoryDTO(row)
	return &dto, nil
}

// CategoryUsage counts the cascade impact before deleting a category.
func (s *Service) CategoryUsage(ctx context.Context, id uint) (*CategoryUsageResponse, error) {
	if _, err := model.NewIncidentCategoryModel(s.svcCtx.DB).Get(ctx, id); err != nil {
		return nil, err
	}
	incidents, err := model.NewIncidentModel(s.svcCtx.DB).CountByCategory(ctx, id)
	if err != nil {
		return nil, err
	}
	bugs, err := s.countBugsByCategory(ctx, id)
	if err != nil {
		return nil, err
	}
	return &CategoryUsageResponse{Incidents: incidents, Bugs: bugs}, nil
}

// DeleteCategory deletes a category: incidents and bugs are first merged into
// the built-in "uncategorized" row, then the row is physically deleted
// (design §2.1 two-phase delete — callers should GET usage for the cascade
// prompt first). Builtin rows refuse deletion.
func (s *Service) DeleteCategory(ctx context.Context, id uint) error {
	m := model.NewIncidentCategoryModel(s.svcCtx.DB)
	row, err := m.Get(ctx, id)
	if err != nil {
		return err
	}
	if row.Builtin {
		return errorx.NewBadRequest("内建类别不可删除（可停用）")
	}
	uncategorizedID, err := m.UncategorizedID(ctx)
	if err != nil {
		return err
	}
	return s.svcCtx.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.Incident{}).Where("category_id = ?", id).
			Update("category_id", uncategorizedID).Error; err != nil {
			return err
		}
		if err := tx.Model(&model.Bug{}).Where("category_id = ?", id).
			Update("category_id", uncategorizedID).Error; err != nil {
			return err
		}
		if err := tx.Delete(&model.IncidentCategory{}, id).Error; err != nil {
			return err
		}
		s.auditIncident(ctx, audit.EventIncidentCategory, audit.StateSuccess, "category_delete", fmt.Sprint(id), map[string]interface{}{"slug": row.Slug})
		return nil
	})
}

// ---- Incident registration ----

// CreateIncident registers an incident: the category is required and must be an
// enabled row; subcategories are validated against the category whitelist; a
// non-empty incidentKey makes the call idempotent (the existing row for the
// same key is returned).
func (s *Service) CreateIncident(ctx context.Context, req *IncidentCreateRequest, createdBy string) (*IncidentDTO, bool, error) {
	// Attribution is forced to the current request scope: dashboard
	// registrations always land in this scope, and the game/env values in
	// the body are not the source of truth for attribution (the external
	// token surface has no header scope and is unaffected; its attribution
	// binding is governed separately by the external layer).
	if scope := svc.GameScopeFromContext(ctx); scope.GameID != "" {
		req.GameID = scope.GameID
	}
	if scope := svc.GameScopeFromContext(ctx); scope.Env != "" {
		req.Env = scope.Env
	}
	title := strings.TrimSpace(req.Title)
	if title == "" {
		return nil, false, errorx.NewBadRequest("事故标题不能为空")
	}
	if req.CategoryID == 0 {
		return nil, false, errorx.NewBadRequest("事故类别必选")
	}
	im := model.NewIncidentModel(s.svcCtx.DB)
	if req.IncidentKey != "" {
		if existing, err := im.FindByKey(ctx, req.IncidentKey); err == nil {
			dto := s.toIncidentDTO(ctx, existing)
			return &dto, false, nil
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, false, err
		}
	}
	if err := s.validateCategoryAndSubcategory(ctx, req.CategoryID, req.Subcategory); err != nil {
		return nil, false, err
	}
	if req.Severity == "" {
		req.Severity = model.IncidentSeverityInfo
	}
	if _, ok := model.ValidIncidentSeverities[req.Severity]; !ok {
		return nil, false, errorx.NewBadRequest("无效的严重度: " + req.Severity)
	}
	if _, ok := model.ValidIncidentSources[s.sourceOf(req)]; !ok {
		return nil, false, errorx.NewBadRequest("无效的事故来源")
	}
	if err := validateAttribution(req.ResponsibleType); err != nil {
		return nil, false, err
	}
	if err := validateRef(req.RefType, req.RefID); err != nil {
		return nil, false, err
	}
	detectedAt := time.Now()
	if req.DetectedAt != nil {
		detectedAt = *req.DetectedAt
	}
	respType := req.ResponsibleType
	if respType == "" {
		respType = model.IncidentRespUnknown
	}
	row := &model.Incident{
		Title:           title,
		CategoryID:      req.CategoryID,
		Subcategory:     strings.TrimSpace(req.Subcategory),
		Severity:        req.Severity,
		Status:          model.IncidentStatusOpen,
		Source:          s.sourceOf(req),
		ResponsibleType: respType,
		ResponsibleID:   strings.TrimSpace(req.ResponsibleID),
		DetectedAt:      detectedAt,
		GameID:          strings.TrimSpace(req.GameID),
		Env:             strings.TrimSpace(req.Env),
		RefType:         strings.TrimSpace(req.RefType),
		RefID:           strings.TrimSpace(req.RefID),
		CreatedBy:       createdBy,
	}
	if len(req.ExecLogIDs) > 0 {
		raw, _ := json.Marshal(req.ExecLogIDs)
		row.ExecLogIDs = model.JSON(raw)
	}
	if len(req.Details) > 0 {
		row.Details = req.Details
	}
	if req.IncidentKey != "" {
		key := strings.TrimSpace(req.IncidentKey)
		row.IncidentKey = &key
	}
	if err := im.Create(ctx, row); err != nil {
		if isUniqueViolation(err) {
			if existing, ferr := im.FindByKey(ctx, req.IncidentKey); ferr == nil {
				dto := s.toIncidentDTO(ctx, existing)
				return &dto, false, nil
			}
			return nil, false, errorx.NewConflict("事故幂等键已存在")
		}
		return nil, false, err
	}
	s.auditIncident(ctx, audit.EventIncidentCreate, audit.StateSuccess, "create", fmt.Sprint(row.ID),
		map[string]interface{}{"title": row.Title, "severity": row.Severity, "categoryId": row.CategoryID, "gameId": row.GameID, "env": row.Env})
	dto := s.toIncidentDTO(ctx, row)
	DispatchLifecycle(ctx, s.svcCtx, row, LifecycleIncidentCreated)
	return &dto, true, nil
}

// GetIncident fetches a single incident (with the denormalized category fields).
func (s *Service) GetIncident(ctx context.Context, id uint) (*IncidentDTO, error) {
	row, err := model.NewIncidentModel(s.svcCtx.DB).Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := requireIncidentScope(ctx, row); err != nil {
		return nil, err
	}
	dto := s.toIncidentDTO(ctx, row)
	return &dto, nil
}

// ListIncidents returns paginated and filtered incidents.
func (s *Service) ListIncidents(ctx context.Context, opts model.IncidentQueryOptions) (*IncidentListResponse, error) {
	rows, total, err := model.NewIncidentModel(s.svcCtx.DB).List(ctx, opts)
	if err != nil {
		return nil, err
	}
	catNames := s.categoryNames(ctx, rows)
	items := make([]IncidentDTO, 0, len(rows))
	for i := range rows {
		dto := toIncidentDTOBasic(&rows[i])
		if cat, ok := catNames[rows[i].CategoryID]; ok {
			dto.CategorySlug = cat.slug
			dto.CategoryName = cat.name
		}
		items = append(items, dto)
	}
	return &IncidentListResponse{Items: items, Total: total}, nil
}

// UpdateIncident updates the editable fields; category/subcategory changes are
// re-validated against the whitelist.
func (s *Service) UpdateIncident(ctx context.Context, id uint, req *IncidentUpdateRequest) (*IncidentDTO, error) {
	m := model.NewIncidentModel(s.svcCtx.DB)
	row, err := m.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := requireIncidentScope(ctx, row); err != nil {
		s.auditIncident(ctx, audit.EventIncidentUpdate, audit.StateDenied, "update", fmt.Sprint(id), nil)
		return nil, err
	}
	// Attribution migration may only land inside the current scope: moving an
	// incident to another game/env is equivalent to a cross-scope write
	// (migrating out of this scope is rejected too, so rows cannot be moved
	// out of the visible domain unnoticed).
	if req.GameID != nil || req.Env != nil {
		targetGame, targetEnv := row.GameID, row.Env
		if req.GameID != nil {
			targetGame = strings.TrimSpace(*req.GameID)
		}
		if req.Env != nil {
			targetEnv = strings.TrimSpace(*req.Env)
		}
		scope := svc.GameScopeFromContext(ctx)
		if (scope.GameID != "" || scope.Env != "") && !svc.ScopeMatches(ctx, targetGame, targetEnv) {
			return nil, utils.NewPermissionDenied("事故归属迁移只允许落在当前游戏/环境内", nil)
		}
	}
	updates := map[string]interface{}{}
	if req.Title != nil {
		title := strings.TrimSpace(*req.Title)
		if title == "" {
			return nil, errorx.NewBadRequest("事故标题不能为空")
		}
		updates["title"] = title
	}
	if req.CategoryID != nil && *req.CategoryID != 0 && *req.CategoryID != row.CategoryID {
		if err := s.validateCategoryAndSubcategory(ctx, *req.CategoryID, derefOrEmpty(req.Subcategory)); err != nil {
			return nil, err
		}
		updates["category_id"] = *req.CategoryID
	} else if req.Subcategory != nil {
		if err := s.validateCategoryAndSubcategory(ctx, row.CategoryID, *req.Subcategory); err != nil {
			return nil, err
		}
	}
	if req.Subcategory != nil {
		updates["subcategory"] = strings.TrimSpace(*req.Subcategory)
	}
	escalated := false
	if req.Severity != nil {
		if _, ok := model.ValidIncidentSeverities[*req.Severity]; !ok {
			return nil, errorx.NewBadRequest("无效的严重度: " + *req.Severity)
		}
		escalated = severityRank(*req.Severity) > severityRank(row.Severity)
		updates["severity"] = *req.Severity
	}
	if req.ResponsibleType != nil {
		if err := validateAttribution(*req.ResponsibleType); err != nil {
			return nil, err
		}
		updates["responsible_type"] = *req.ResponsibleType
	}
	if req.ResponsibleID != nil {
		updates["responsible_id"] = strings.TrimSpace(*req.ResponsibleID)
	}
	if req.GameID != nil {
		updates["game_id"] = strings.TrimSpace(*req.GameID)
	}
	if req.Env != nil {
		updates["env"] = strings.TrimSpace(*req.Env)
	}
	if req.ExecLogIDs != nil {
		raw, _ := json.Marshal(*req.ExecLogIDs)
		updates["exec_log_ids"] = model.JSON(raw)
	}
	if req.RefType != nil {
		if err := validateRef(*req.RefType, derefOrEmpty(req.RefID)); err != nil {
			return nil, err
		}
		updates["ref_type"] = strings.TrimSpace(*req.RefType)
	}
	if req.RefID != nil {
		updates["ref_id"] = strings.TrimSpace(*req.RefID)
	}
	if req.Details != nil {
		// AUD-020: the details column is a JSON type (model Details
		// datatypes.JSONMap); a raw map going into Updates cannot be
		// serialized (sql: unsupported type map[...]). Same pattern as
		// exec_log_ids: JSON-encode into model.JSON before writing.
		raw, err := json.Marshal(req.Details)
		if err != nil {
			return nil, errorx.NewBadRequest("无效的详情数据: " + err.Error())
		}
		updates["details"] = model.JSON(raw)
	}
	if len(updates) == 0 {
		dto := s.toIncidentDTO(ctx, row)
		return &dto, nil
	}
	if err := m.Update(ctx, id, updates); err != nil {
		return nil, err
	}
	row, err = m.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if escalated {
		DispatchLifecycle(ctx, s.svcCtx, row, LifecycleIncidentEscalated)
	}
	s.auditIncident(ctx, audit.EventIncidentUpdate, audit.StateSuccess, "update", fmt.Sprint(id), nil)
	dto := s.toIncidentDTO(ctx, row)
	return &dto, nil
}

// TransitionIncident performs the state transition open→acknowledged→resolved;
// resolved can be reopened (clearing ResolvedAt); repeating a transition to the
// same state is idempotent and passes through.
func (s *Service) TransitionIncident(ctx context.Context, id uint, req *IncidentStatusRequest) (*IncidentDTO, error) {
	if _, ok := model.ValidIncidentStatuses[req.Status]; !ok {
		return nil, errorx.NewBadRequest("无效的事故状态: " + req.Status)
	}
	m := model.NewIncidentModel(s.svcCtx.DB)
	row, err := m.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := requireIncidentScope(ctx, row); err != nil {
		s.auditIncident(ctx, audit.EventIncidentTransition, audit.StateDenied, "transition", fmt.Sprint(id), map[string]interface{}{"to": req.Status})
		return nil, err
	}
	updates := map[string]interface{}{"status": req.Status}
	switch req.Status {
	case model.IncidentStatusResolved:
		if row.ResolvedAt == nil {
			resolvedAt := time.Now()
			if req.ResolvedAt != nil {
				resolvedAt = *req.ResolvedAt
			}
			updates["resolved_at"] = resolvedAt
		}
	case model.IncidentStatusOpen, model.IncidentStatusAcked:
		// Reopening or rolling back clears the recovery time (recurrence
		// statistics use the resolved history in a separate table, batch 2)
		updates["resolved_at"] = nil
	}
	if err := m.Update(ctx, id, updates); err != nil {
		return nil, err
	}
	row, err = m.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if req.Status == model.IncidentStatusResolved {
		DispatchLifecycle(ctx, s.svcCtx, row, LifecycleIncidentResolved)
	}
	s.auditIncident(ctx, audit.EventIncidentTransition, audit.StateSuccess, "transition", fmt.Sprint(id),
		map[string]interface{}{"to": req.Status, "gameId": row.GameID, "env": row.Env})
	dto := s.toIncidentDTO(ctx, row)
	return &dto, nil
}

// ---- Internal helpers ----

// validateCategoryAndSubcategory checks that the category is an enabled row;
// the subcategory is validated against that category's whitelist (no configured
// whitelist = unrestricted, ≤64 chars).
func (s *Service) validateCategoryAndSubcategory(ctx context.Context, categoryID uint, subcategory string) error {
	row, err := model.NewIncidentCategoryModel(s.svcCtx.DB).Get(ctx, categoryID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errorx.NewBadRequest(fmt.Sprintf("类别不存在: %d", categoryID))
		}
		return err
	}
	if !row.Enabled {
		return errorx.NewBadRequest("类别已停用: " + row.Name)
	}
	sub := strings.TrimSpace(subcategory)
	if len(sub) > 64 {
		return errorx.NewBadRequest("子类长度不能超过 64")
	}
	whitelist := parseStringList(row.Subcategories)
	if len(whitelist) > 0 {
		if sub == "" {
			return errorx.NewBadRequest(fmt.Sprintf("类别 %s 要求必选子类", row.Name))
		}
		for _, w := range whitelist {
			if w == sub {
				return nil
			}
		}
		return errorx.NewBadRequest(fmt.Sprintf("子类不在类别 %s 的白名单内: %s", row.Name, sub))
	}
	return nil
}

// sourceOf derives the incident source (when no explicit source field is set,
// from the ref type; batch 1 lets the handler decide manual, and the external
// API batch overrides it with external).
func (s *Service) sourceOf(req *IncidentCreateRequest) string {
	if req.Source != "" {
		return req.Source
	}
	if req.RefType == model.IncidentRefAlert {
		return model.IncidentSourceAlert
	}
	if req.RefType == model.IncidentRefCicdBuild {
		return model.IncidentSourceCicd
	}
	if req.RefType == model.IncidentRefProbeWindow {
		return model.IncidentSourceProbe
	}
	if req.RefType == model.IncidentRefSupervisorEvent {
		return model.IncidentSourceSupervisor
	}
	return model.IncidentSourceManual
}

func validateAttribution(respType string) error {
	if respType == "" {
		return nil
	}
	if _, ok := model.ValidIncidentRespTypes[respType]; !ok {
		return errorx.NewBadRequest("无效的责任类型: " + respType)
	}
	return nil
}

func validateRef(refType, refID string) error {
	if refType == "" {
		if refID != "" {
			return errorx.NewBadRequest("refId 需与 refType 同时提供")
		}
		return nil
	}
	if _, ok := model.ValidIncidentRefTypes[refType]; !ok {
		return errorx.NewBadRequest("无效的关联对象类型: " + refType)
	}
	if strings.TrimSpace(refID) == "" {
		return errorx.NewBadRequest("refType 需搭配 refId")
	}
	return nil
}

func applyCategoryUpsert(row *model.IncidentCategory, req *CategoryUpsertRequest) {
	updates := map[string]interface{}{}
	applyCategoryUpdates(updates, req)
	if v, ok := updates["name"]; ok {
		row.Name = v.(string)
	}
	if v, ok := updates["sort"]; ok {
		row.Sort = v.(int)
	}
	if v, ok := updates["leader"]; ok {
		row.Leader = v.(string)
	}
	if v, ok := updates["subcategories"]; ok {
		row.Subcategories = v.(model.JSON)
	}
	if v, ok := updates["audience"]; ok {
		row.Audience = v.(model.JSON)
	}
	if v, ok := updates["enabled"]; ok {
		row.Enabled = v.(bool)
	}
}

func applyCategoryUpdates(updates map[string]interface{}, req *CategoryUpsertRequest) {
	if req.Name != nil {
		updates["name"] = strings.TrimSpace(*req.Name)
	}
	if req.Sort != nil {
		updates["sort"] = *req.Sort
	}
	if req.Leader != nil {
		updates["leader"] = strings.TrimSpace(*req.Leader)
	}
	if req.Subcategories != nil {
		if len(*req.Subcategories) == 0 {
			updates["subcategories"] = nil
		} else {
			raw, _ := json.Marshal(*req.Subcategories)
			updates["subcategories"] = model.JSON(raw)
		}
	}
	if req.Audience != nil {
		raw, _ := json.Marshal(*req.Audience)
		updates["audience"] = model.JSON(raw)
	}
	if req.Enabled != nil {
		updates["enabled"] = *req.Enabled
	}
}

func parseStringList(raw model.JSON) []string {
	if len(raw) == 0 {
		return nil
	}
	var out []string
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil
	}
	return out
}

func parseExecLogIDs(raw model.JSON) []int64 {
	if len(raw) == 0 {
		return nil
	}
	var out []int64
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil
	}
	return out
}

func parseAudience(raw model.JSON) *CategoryAudience {
	if len(raw) == 0 {
		return nil
	}
	var out CategoryAudience
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil
	}
	return &out
}

type categoryName struct {
	slug string
	name string
}

func (s *Service) categoryNames(ctx context.Context, rows []model.Incident) map[uint]categoryName {
	ids := make([]uint, 0, len(rows))
	seen := map[uint]bool{}
	for _, r := range rows {
		if r.CategoryID != 0 && !seen[r.CategoryID] {
			ids = append(ids, r.CategoryID)
			seen[r.CategoryID] = true
		}
	}
	out := map[uint]categoryName{}
	if len(ids) == 0 {
		return out
	}
	cats, err := model.NewIncidentCategoryModel(s.svcCtx.DB).List(ctx, false)
	if err != nil {
		return out
	}
	for _, c := range cats {
		if seen[c.ID] {
			out[c.ID] = categoryName{slug: c.Slug, name: c.Name}
		}
	}
	return out
}

func toCategoryDTO(row *model.IncidentCategory) CategoryDTO {
	return CategoryDTO{
		ID:            row.ID,
		Name:          row.Name,
		Slug:          row.Slug,
		Sort:          row.Sort,
		Leader:        row.Leader,
		Subcategories: parseStringList(row.Subcategories),
		Audience:      parseAudience(row.Audience),
		Enabled:       row.Enabled,
		Builtin:       row.Builtin,
		CreatedAt:     row.CreatedAt,
		UpdatedAt:     row.UpdatedAt,
	}
}

func toIncidentDTOBasic(row *model.Incident) IncidentDTO {
	dto := IncidentDTO{
		ID:              row.ID,
		Title:           row.Title,
		CategoryID:      row.CategoryID,
		Subcategory:     row.Subcategory,
		Severity:        row.Severity,
		Status:          row.Status,
		Source:          row.Source,
		ResponsibleType: row.ResponsibleType,
		ResponsibleID:   row.ResponsibleID,
		DetectedAt:      row.DetectedAt,
		ResolvedAt:      row.ResolvedAt,
		GameID:          row.GameID,
		Env:             row.Env,
		ExecLogIDs:      parseExecLogIDs(row.ExecLogIDs),
		RefType:         row.RefType,
		RefID:           row.RefID,
		Details:         row.Details,
		CreatedBy:       row.CreatedBy,
		CreatedAt:       row.CreatedAt,
		UpdatedAt:       row.UpdatedAt,
	}
	if row.IncidentKey != nil {
		dto.IncidentKey = *row.IncidentKey
	}
	return dto
}

func (s *Service) toIncidentDTO(ctx context.Context, row *model.Incident) IncidentDTO {
	dto := toIncidentDTOBasic(row)
	if cat, err := model.NewIncidentCategoryModel(s.svcCtx.DB).Get(ctx, row.CategoryID); err == nil {
		dto.CategorySlug = cat.Slug
		dto.CategoryName = cat.Name
	}
	return dto
}

func derefOrEmpty(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// countBugsByCategory counts bugs under a category (for the delete cascade
// prompt; the Bug side only has columns and no dedicated model method, so
// query it directly).
func (s *Service) countBugsByCategory(ctx context.Context, categoryID uint) (int64, error) {
	var count int64
	err := s.svcCtx.DB.WithContext(ctx).Model(&model.Bug{}).
		Where("category_id = ?", categoryID).Count(&count).Error
	return count, err
}

// isUniqueViolation detects unique-constraint conflicts across dialects
// (sqlite/mysql/postgres).
func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "UNIQUE constraint failed") ||
		strings.Contains(msg, "Duplicate entry") ||
		strings.Contains(msg, "duplicate key value") ||
		strings.Contains(msg, "1062")
}
