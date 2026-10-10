// Package incident 实现事故报表体系批 1 的事故登记与类别管理 API
// （docs/design/incident-reports.md §2/§3/§4）。类别存库可配（枚举校验=
// 配置表合法值），incident 按 category_id 关联（改名不断链），删除类别
// 级联归并到内建「未分类」行。
package incident

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
	"gorm.io/gorm"
)

// categorySlugPattern 类别 slug 闭式约束（小写字母/数字/连字符）。
var categorySlugPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,63}$`)

// Service implements the incident registry API.
type Service struct {
	svcCtx *svc.ServiceContext
}

// NewService creates an incident service.
func NewService(svcCtx *svc.ServiceContext) *Service {
	return &Service{svcCtx: svcCtx}
}

// ---- 类别管理 ----

// ListCategories 全量类别（含停用行；enabledOnly 供登记选择器）。
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

// CreateCategory 新建类别（slug 校验 + 唯一性）。
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
	dto := toCategoryDTO(row)
	return &dto, nil
}

// UpdateCategory 更新类别（slug 不可改；未分类行名称可改但不可删）。
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
	dto := toCategoryDTO(row)
	return &dto, nil
}

// CategoryUsage 删除前的级联提示计数。
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

// DeleteCategory 删除类别：事故与 bug 归并到内建「未分类」行后物理删行
// （设计 §2.1 两段删除——调用方应先 GET usage 级联提示）。builtin 行拒绝删除。
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
		return tx.Delete(&model.IncidentCategory{}, id).Error
	})
}

// ---- 事故登记 ----

// CreateIncident 登记事故：类别必选且须为启用行，子类按类别白名单校验；
// incidentKey 非空时幂等（同键返回已存在行）。
func (s *Service) CreateIncident(ctx context.Context, req *IncidentCreateRequest, createdBy string) (*IncidentDTO, bool, error) {
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
	dto := s.toIncidentDTO(ctx, row)
	return &dto, true, nil
}

// GetIncident 单条查询（带类别冗余字段）。
func (s *Service) GetIncident(ctx context.Context, id uint) (*IncidentDTO, error) {
	row, err := model.NewIncidentModel(s.svcCtx.DB).Get(ctx, id)
	if err != nil {
		return nil, err
	}
	dto := s.toIncidentDTO(ctx, row)
	return &dto, nil
}

// ListIncidents 分页 + 过滤。
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

// UpdateIncident 更新可编辑字段；类别/子类变更重新校验白名单。
func (s *Service) UpdateIncident(ctx context.Context, id uint, req *IncidentUpdateRequest) (*IncidentDTO, error) {
	m := model.NewIncidentModel(s.svcCtx.DB)
	row, err := m.Get(ctx, id)
	if err != nil {
		return nil, err
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
	if req.Severity != nil {
		if _, ok := model.ValidIncidentSeverities[*req.Severity]; !ok {
			return nil, errorx.NewBadRequest("无效的严重度: " + *req.Severity)
		}
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
		updates["details"] = req.Details
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
	dto := s.toIncidentDTO(ctx, row)
	return &dto, nil
}

// TransitionIncident 状态流转：open→acknowledged→resolved；resolved 可
// 重开（清 ResolvedAt）；同态重复流转幂等放行。
func (s *Service) TransitionIncident(ctx context.Context, id uint, req *IncidentStatusRequest) (*IncidentDTO, error) {
	if _, ok := model.ValidIncidentStatuses[req.Status]; !ok {
		return nil, errorx.NewBadRequest("无效的事故状态: " + req.Status)
	}
	m := model.NewIncidentModel(s.svcCtx.DB)
	row, err := m.Get(ctx, id)
	if err != nil {
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
		// 重开/回退清恢复时间（复发统计按 resolved 历史另表，批 2）
		updates["resolved_at"] = nil
	}
	if err := m.Update(ctx, id, updates); err != nil {
		return nil, err
	}
	row, err = m.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	dto := s.toIncidentDTO(ctx, row)
	return &dto, nil
}

// ---- 内部辅助 ----

// validateCategoryAndSubcategory 类别须为启用行；子类按该类白名单校验
// （未配置白名单=不限，≤64 字符）。
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

// sourceOf 推导事故来源（无显式来源字段时按 createdBy/调用面；批 1 由
// handler 决定 manual，外部 API 批次覆写 external）。
func (s *Service) sourceOf(req *IncidentCreateRequest) string {
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

// countBugsByCategory 某类别下的 bug 数（删除级联提示；Bug 侧只有列，
// 无独立模型方法，直查）。
func (s *Service) countBugsByCategory(ctx context.Context, categoryID uint) (int64, error) {
	var count int64
	err := s.svcCtx.DB.WithContext(ctx).Model(&model.Bug{}).
		Where("category_id = ?", categoryID).Count(&count).Error
	return count, err
}

// isUniqueViolation 跨方言唯一约束冲突判定（sqlite/mysql/postgres）。
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
