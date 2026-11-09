package page

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/cuihairu/croupier/internal/audit"
	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/dashboard/freshness"
	"github.com/cuihairu/croupier/internal/dashboard/generator"
	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/model"
	contractsvc "github.com/cuihairu/croupier/internal/service"
	"gorm.io/gorm"
)

// pageInvolvedResources 计算一个页面涉及的资源集合（OPEN-ISSUES #30）：
// resourceKey 列 ∪ 各 binding 函数契约的 resourceKey，去重升序。关联在读时
// 从 SpecJSON 现算、不落列——page_specs 量级为每 scope 几十行，且加列要走
// goose 编号迁移（见 CLAUDE.md 迁移契约），收益不抵风险；SpecJSON 解析失败
// 时退化为仅 resourceKey 列（不阻断列表）。
func (s *Service) pageInvolvedResources(p model.PageSpec, functionResources map[string]string) []string {
	seen := map[string]struct{}{}
	add := func(key string) {
		if key = strings.TrimSpace(key); key != "" {
			seen[key] = struct{}{}
		}
	}
	add(p.ResourceKey)
	var pageSpec spec.PageSpec
	if err := json.Unmarshal([]byte(p.SpecJSON), &pageSpec); err == nil {
		for _, binding := range pageSpec.Bindings {
			add(functionResources[strings.TrimSpace(binding.FunctionID)])
		}
	}
	if len(seen) == 0 {
		return nil
	}
	out := make([]string, 0, len(seen))
	for key := range seen {
		out = append(out, key)
	}
	sort.Strings(out)
	return out
}

// functionResourceIndex 建 functionID → resourceKey 映射（空资源不入表）。
// 函数契约表按 scope 一次读取，供关联计算复用。
func (s *Service) functionResourceIndex(ctx context.Context, gameID, env string) map[string]string {
	index := map[string]string{}
	if s == nil || s.svcCtx == nil || s.svcCtx.DB == nil {
		return index
	}
	contracts, err := model.NewFunctionContractModel(s.svcCtx.DB).ListByScope(ctx, gameID, env)
	if err != nil {
		return index
	}
	for _, contract := range contracts {
		if key := strings.TrimSpace(contract.ResourceKey); key != "" {
			index[contract.FunctionID] = key
		}
	}
	return index
}

func (s *Service) ListDrafts(ctx context.Context, req *PageDraftListRequest) (*PageDraftListResponse, error) {
	if err := s.requirePageRead(ctx); err != nil {
		return nil, err
	}
	gameID, env, err := requireScope(ctx)
	if err != nil {
		return nil, err
	}

	// 条件下推：status 直接走 SQL（ListByScopeAndStatus）；资源关联与关键词
	// 依赖 SpecJSON/函数契约，在 scope 候选集上过滤（OPEN-ISSUES #30）。
	var pages []model.PageSpec
	if strings.TrimSpace(req.Status) != "" {
		pages, err = s.svcCtx.PageSpecModel.ListByScopeAndStatus(ctx, gameID, env, strings.TrimSpace(req.Status))
	} else {
		pages, err = s.svcCtx.PageSpecModel.ListByScope(ctx, gameID, env)
	}
	if err != nil {
		return nil, err
	}

	resourceFilter := strings.TrimSpace(req.ResourceKey)
	keyword := strings.ToLower(strings.TrimSpace(req.Keyword))
	functionResources := s.functionResourceIndex(ctx, gameID, env)

	items := make([]spec.PageSpecDraftSummary, 0, len(pages))
	for _, p := range pages {
		resources := s.pageInvolvedResources(p, functionResources)
		if resourceFilter != "" && !containsString(resources, resourceFilter) {
			continue
		}
		if keyword != "" && !draftMatchesKeyword(p, resources, keyword) {
			continue
		}
		items = append(items, spec.PageSpecDraftSummary{
			GameID:      p.GameID,
			Env:         p.Env,
			PageKey:     p.PageKey,
			Type:        spec.PageType(p.Type),
			ResourceKey: p.ResourceKey,
			Resources:   resources,
			Title:       p.GetTitle(),
			Category: spec.PageCategorySpec{
				Key:   p.CategoryKey,
				Order: p.CategoryOrder,
			},
			MenuID:           menuIDToDTO(p.MenuID),
			Status:           spec.PageDraftStatus(p.Status),
			DraftRevision:    p.DraftRevision,
			PublishedVersion: p.PublishedVersion,
			UpdatedAt:        p.UpdatedAt.Format(time.RFC3339),
			UpdatedBy:        p.UpdatedBy,
		})
	}

	total := len(items)
	page, pageSize := normalizePageDraftPaging(req.Page, req.PageSize)
	start := (page - 1) * pageSize
	if start > total {
		start = total
	}
	end := start + pageSize
	if end > total {
		end = total
	}
	return &PageDraftListResponse{
		Items:    items[start:end],
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}

// normalizePageDraftPaging 收敛非法分页参数：page<1→1；pageSize 缺省 20、上限 200。
// 非法值钳制而非 400——过滤语义不受影响，只是窗口大小。
func normalizePageDraftPaging(page, pageSize int) (int, int) {
	if page < 1 {
		page = 1
	}
	switch {
	case pageSize <= 0:
		pageSize = defaultPageDraftPageSize
	case pageSize > maxPageDraftPageSize:
		pageSize = maxPageDraftPageSize
	}
	return page, pageSize
}

func containsString(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}

// draftMatchesKeyword 大小写不敏感子串匹配 pageKey / 各 locale 标题 / 涉及资源。
func draftMatchesKeyword(p model.PageSpec, resources []string, keyword string) bool {
	if strings.Contains(strings.ToLower(p.PageKey), keyword) {
		return true
	}
	for _, title := range p.GetTitle() {
		if strings.Contains(strings.ToLower(title), keyword) {
			return true
		}
	}
	for _, resource := range resources {
		if strings.Contains(strings.ToLower(resource), keyword) {
			return true
		}
	}
	return false
}

// Resources aggregates the resources involved in the scope's pages
// (GET /api/v1/pages/resources). Options must come from the server: deriving
// them from the current (possibly filtered) list collapses choices — same
// failure mode as resource-catalog categories (OPEN-ISSUES #14/#13).
func (s *Service) Resources(ctx context.Context) (*PageResourcesResponse, error) {
	if err := s.requirePageRead(ctx); err != nil {
		return nil, err
	}
	gameID, env, err := requireScope(ctx)
	if err != nil {
		return nil, err
	}

	pages, err := s.svcCtx.PageSpecModel.ListByScope(ctx, gameID, env)
	if err != nil {
		return nil, err
	}

	// 聚合口径与列表过滤同源（#30）：按页面涉及资源展开计数——多资源页在
	// 每个涉及资源下计一次（一页多资源是常态），空关联页面不成项。
	functionResources := s.functionResourceIndex(ctx, gameID, env)
	counts := make(map[string]int)
	for i := range pages {
		for _, key := range s.pageInvolvedResources(pages[i], functionResources) {
			counts[key]++
		}
	}
	items := make([]PageResourceOption, 0, len(counts))
	for key, count := range counts {
		items = append(items, PageResourceOption{ResourceKey: key, PageCount: count})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ResourceKey < items[j].ResourceKey })
	return &PageResourcesResponse{Items: items}, nil
}

func (s *Service) GetDraft(ctx context.Context, req *PageDraftRequest) (*PageDraftResponse, error) {
	if err := s.requirePageRead(ctx); err != nil {
		return nil, err
	}
	p, err := s.findDraft(ctx, req.PageKey)
	if err != nil {
		return nil, err
	}
	resp, err := pageDraftResponseFromModel(p)
	if err != nil {
		return nil, err
	}
	resp.BindingFreshness = s.bindingFreshnessForPublishedDraft(ctx, p)
	return resp, nil
}

// SetPageMenu mounts/unmounts a page under a menu (PUT /pages/:pageKey/menu).
// menuId nil or 0 unmounts; otherwise the menu must exist in the same scope.
func (s *Service) SetPageMenu(ctx context.Context, req *PageMenuUpdateRequest) (*PageMenuResponse, error) {
	if err := s.requirePageEdit(ctx); err != nil {
		return nil, err
	}
	gameID, env, err := requireScope(ctx)
	if err != nil {
		return nil, err
	}

	var menuID *uint
	if req.MenuID != nil && *req.MenuID != 0 {
		if s.svcCtx.MenuModel == nil {
			return nil, errorx.NewInternalError("菜单模型未初始化")
		}
		menu, err := s.svcCtx.MenuModel.FindByID(ctx, gameID, env, uint(*req.MenuID))
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, errorx.NewNotFound("菜单不存在")
			}
			return nil, err
		}
		menuID = &menu.ID
	}

	p, err := s.findDraft(ctx, req.PageKey)
	if err != nil {
		return nil, err
	}
	if err := s.svcCtx.PageSpecModel.UpdateMenuID(ctx, gameID, env, p.PageKey, menuID); err != nil {
		return nil, err
	}
	return &PageMenuResponse{
		PageKey: req.PageKey,
		MenuID:  menuIDToDTO(menuID),
	}, nil
}

func (s *Service) SaveDraft(ctx context.Context, req *PageSaveRequest) (*PageSaveResponse, error) {
	if err := s.requirePageEdit(ctx); err != nil {
		return nil, err
	}
	gameID, env, err := requireScope(ctx)
	if err != nil {
		return nil, err
	}
	actor, err := currentUsername(ctx)
	if err != nil {
		return nil, err
	}
	if req.DraftRevision == nil {
		return nil, errorx.NewBadRequest("draftRevision is required")
	}
	if strings.TrimSpace(req.PageKey) == "" {
		return nil, errorx.NewBadRequest("pageKey is required")
	}
	pageSpec := spec.PageSpec{
		PageKey:     strings.TrimSpace(req.PageKey),
		Type:        req.Type,
		ResourceKey: strings.TrimSpace(req.ResourceKey),
		Navigation:  req.Navigation,
		Resource:    req.Resource,
		Operation:   req.Operation,
		Task:        req.Task,
		Report:      req.Report,
		Bindings:    req.Bindings,
	}
	if !isValidPageType(req.Type) {
		return nil, errorx.NewBadRequest("type must be resource, operation, task, or report")
	}
	categoryKey := strings.TrimSpace(req.Category.Key)
	if categoryKey == "" {
		return nil, errorx.NewBadRequest("category.key is required")
	}

	title := normalizeLocaleKeys(req.Title)
	if !hasDefaultLocale(title) {
		return nil, errorx.NewBadRequest("title must include a non-empty value in at least one locale")
	}
	pageSpec.Title = title
	pageSpec.Description = normalizeLocaleKeys(req.Description)
	// 分类名称（category.labels）已由菜单系统接管（T-M8）：请求里的
	// labels 一律丢弃，仅保留 category.key 作为分组定位键。
	pageSpec.Category = spec.PageCategorySpec{
		Key:   categoryKey,
		Order: req.Category.Order,
	}
	pageSpec.Order = req.Order
	pageSpec.Icon = strings.TrimSpace(req.Icon)

	specJSON, err := marshalPageSpec(pageSpec)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	var nextRevision int
	err = s.withPageTransaction(ctx, func(txCtx context.Context, pageModel *model.PageSpecModel, _ *model.PublishedPageSpecModel, versionModel *model.PageVersionModel) error {
		existing, err := pageModel.FindByScopeAndPageKey(txCtx, gameID, env, req.PageKey)
		if err != nil && err != gorm.ErrRecordNotFound {
			return err
		}
		if existing == nil && *req.DraftRevision != 0 {
			return errorx.NewConflictWithDetails("草稿版本冲突：页面已被其他修改更新，请刷新草稿后重试", map[string]any{
				"expected": 0,
				"current":  0,
			})
		}
		if existing != nil && existing.DraftRevision != *req.DraftRevision {
			return errorx.NewConflictWithDetails("草稿版本冲突：页面已被其他修改更新，请刷新草稿后重试", map[string]any{
				"expected": existing.DraftRevision,
				"current":  existing.DraftRevision,
				"provided": *req.DraftRevision,
			})
		}

		ps := &model.PageSpec{
			GameID:        gameID,
			Env:           env,
			PageKey:       strings.TrimSpace(req.PageKey),
			Type:          string(req.Type),
			ResourceKey:   strings.TrimSpace(req.ResourceKey),
			CategoryKey:   categoryKey,
			CategoryOrder: req.Category.Order,
			Order:         req.Order,
			Icon:          strings.TrimSpace(req.Icon),
			SpecJSON:      specJSON,
			Status:        "draft",
			UpdatedBy:     actor,
			UpdatedAt:     now,
		}
		// SetTitle 恒返回 nil（model 层实现为 `b, _ := json.Marshal(
		// map[string]string)`，无出错路径），err 检查已删。
		_ = ps.SetTitle(title)

		if existing != nil {
			ps.ID = existing.ID
			ps.CreatedAt = existing.CreatedAt
			ps.PublishedVersion = existing.PublishedVersion
			ps.PublishedActive = existing.PublishedActive
			ps.BaseProposalKey = existing.BaseProposalKey
			ps.BaseProposalVersion = existing.BaseProposalVersion
			ps.DraftRevision = existing.DraftRevision + 1
			// 菜单挂载是独立 API 维护的运营属性，SaveDraft 全列覆盖时必须
			// 原样保留，否则每次保存都会静默解除页面的菜单关联。
			ps.MenuID = existing.MenuID
		} else {
			ps.CreatedAt = now
			ps.DraftRevision = 1
		}
		nextRevision = ps.DraftRevision

		if err := pageModel.Upsert(txCtx, ps); err != nil {
			return fmt.Errorf("save page draft: %w", err)
		}
		return versionModel.UpsertByScopePageKeyVersion(txCtx, &model.PageVersion{
			GameID:    gameID,
			Env:       env,
			PageKey:   ps.PageKey,
			Version:   ps.DraftRevision,
			SpecJSON:  specJSON,
			Status:    "draft",
			Message:   "save draft",
			CreatedBy: ps.UpdatedBy,
			CreatedAt: now,
		})
	})
	if err != nil {
		return nil, err
	}
	s.auditPageEvent(ctx, audit.EventPageDraftSave, gameID, env, req.PageKey, map[string]interface{}{
		"draft_revision": nextRevision,
		"type":           string(req.Type),
		"resource_key":   strings.TrimSpace(req.ResourceKey),
		"category_key":   categoryKey,
		"binding_count":  len(req.Bindings),
	})
	return &PageSaveResponse{PageKey: req.PageKey, DraftRevision: nextRevision}, nil
}

func (s *Service) RegenerateDraft(ctx context.Context, req *PageRegenerateRequest) (*PageRegenerateResponse, error) {
	if err := s.requirePageEdit(ctx); err != nil {
		return nil, err
	}
	return s.regenerateDraft(ctx, req)
}

// regenerateDraft 是 RegenerateDraft 的核心逻辑，不做权限检查——单页入口
// 已校验 pages:edit；批量入口（BulkRepublish）以自身的 pages:publish 门禁
// 覆盖整个操作（与 BulkPublish→AcceptAndPublishProposal 同一先例）。
func (s *Service) regenerateDraft(ctx context.Context, req *PageRegenerateRequest) (*PageRegenerateResponse, error) {
	gameID, env, err := requireScope(ctx)
	if err != nil {
		return nil, err
	}
	actor, err := currentUsername(ctx)
	if err != nil {
		return nil, err
	}
	if req.DraftRevision == nil {
		return nil, errorx.NewBadRequest("draftRevision is required")
	}

	p, err := s.svcCtx.PageSpecModel.FindByScopeAndPageKey(ctx, gameID, env, req.PageKey)
	if err != nil {
		return nil, ErrPageNotFound(req.PageKey)
	}
	if p.DraftRevision != *req.DraftRevision {
		return nil, errorx.NewConflictWithDetails("草稿版本冲突：页面已被其他修改更新，请刷新草稿后重试", map[string]any{
			"expected": p.DraftRevision,
			"current":  p.DraftRevision,
			"provided": *req.DraftRevision,
		})
	}

	replacement, err := s.proposalReplacementForDraft(ctx, gameID, env, p)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	if err := applyPageSpecToModel(p, replacement.PageSpec); err != nil {
		return nil, err
	}
	p.GameID = gameID
	p.Env = env
	p.Status = "draft"
	p.UpdatedBy = actor
	p.UpdatedAt = now
	p.DraftRevision++
	p.BaseProposalKey = replacement.ProposalKey
	p.BaseProposalVersion = replacement.ProposalVersion

	specJSON, err := buildPageSpecJSONFn(p)
	if err != nil {
		return nil, err
	}
	err = s.withPageTransaction(ctx, func(txCtx context.Context, pageModel *model.PageSpecModel, _ *model.PublishedPageSpecModel, versionModel *model.PageVersionModel) error {
		if err := pageModel.Upsert(txCtx, p); err != nil {
			return err
		}
		return versionModel.UpsertByScopePageKeyVersion(txCtx, &model.PageVersion{
			GameID:    gameID,
			Env:       env,
			PageKey:   p.PageKey,
			Version:   p.DraftRevision,
			SpecJSON:  specJSON,
			Status:    "draft",
			Message:   "regenerate default page from latest function contracts",
			CreatedBy: actor,
			CreatedAt: now,
		})
	})
	if err != nil {
		return nil, err
	}

	s.auditPageEvent(ctx, audit.EventPageDraftSave, gameID, env, req.PageKey, map[string]interface{}{
		"action":            "regenerate_default",
		"source":            "page_proposal",
		"draft_revision":    p.DraftRevision,
		"previous_revision": *req.DraftRevision,
		"quality":           string(replacement.Quality),
		"diagnostic_count":  len(replacement.Diagnostics),
		"diagnostic_errors": countErrors(replacement.Diagnostics),
		"binding_count":     len(replacement.Bindings),
		"resource_key":      replacement.ResourceKey,
		"proposal_key":      replacement.ProposalKey,
		"proposal_version":  replacement.ProposalVersion,
		"proposal_page_key": replacement.PageKey,
	})

	pageSpec, err := pageSpecFromModelFn(p)
	if err != nil {
		return nil, err
	}
	return &PageRegenerateResponse{
		PageKey:       p.PageKey,
		DraftRevision: p.DraftRevision,
		Page:          pageSpec,
		Diagnostics:   replacement.Diagnostics,
		Quality:       replacement.Quality,
	}, nil
}

// identityFieldLookup 返回 resourceKey → identity 字段名的查询闭包
// （S2：sync-selectors required 补齐的语义命中数据源，页面级缓存一次）。
// 查询失败/记录缺失一律容错为未命中——语义缺失只意味着回落 form 同名
// 路径（现状），不应阻断同步。
func (s *Service) identityFieldLookup(ctx context.Context, gameID, env string) func(string) (string, bool) {
	cache := map[string]string{}
	return func(resourceKey string) (string, bool) {
		resourceKey = strings.TrimSpace(resourceKey)
		if resourceKey == "" {
			return "", false
		}
		if field, cached := cache[resourceKey]; cached {
			return field, field != ""
		}
		field := ""
		if sem, err := model.NewCapabilitySemanticsModel(s.svcCtx.DB).
			FindByScopeAndResourceKey(ctx, gameID, env, resourceKey); err == nil && sem != nil {
			field = strings.TrimSpace(sem.IdentityField)
		}
		cache[resourceKey] = field
		return field, field != ""
	}
}

// SyncSelectors 一键同步 stale selector：对草稿逐 binding 跑
// spec.PlanBindingSelectorSync（dry-run 与 apply 共用同一 planner），
// 只修受影响的 assignment、保留全部未受影响定制。dryRun=true 只出报告
// 不落库；apply 走 SaveDraft 同款乐观锁（事务内 revision 重查）+ PageVersion。
// 同步不自动 publish（pages:publish 权限与审批语义分离），同步后的发布级
// 校验结果放在 remainingDiagnostics 由调用方呈现。
func (s *Service) SyncSelectors(ctx context.Context, req *PageSyncSelectorsRequest) (*PageSyncSelectorsResponse, error) {
	if err := s.requirePageEdit(ctx); err != nil {
		return nil, err
	}
	gameID, env, err := requireScope(ctx)
	if err != nil {
		return nil, err
	}
	actor, err := currentUsername(ctx)
	if err != nil {
		return nil, err
	}
	if req.DraftRevision == nil {
		return nil, errorx.NewBadRequest("draftRevision is required")
	}

	p, err := s.findDraft(ctx, req.PageKey)
	if err != nil {
		return nil, err
	}
	if p.DraftRevision != *req.DraftRevision {
		return nil, errorx.NewConflictWithDetails("草稿版本冲突：页面已被其他修改更新，请刷新草稿后重试", map[string]any{
			"expected": p.DraftRevision,
			"current":  p.DraftRevision,
			"provided": *req.DraftRevision,
		})
	}
	resp, err := s.syncSelectorsCore(ctx, gameID, env, actor, p, req.BindingIDs, req.DryRun, *req.DraftRevision)
	if err == nil && !req.DryRun {
		s.autoPublishAfterSync(ctx, resp, gameID, env, actor, int(p.PublishedVersion), resp.DraftRevision)
	}
	return resp, err
}

// syncSelectorsCore 是单页 selector 同步的核心（planner 循环 + 发布级
// 校验 + apply 事务）。外壳（权限/scope/乐观锁前置校验）由调用方负责：
// 单页 SyncSelectors 要求调用方提供 draftRevision；BulkSyncSelectors 由
// 服务端读取当前草稿 revision 后调用（并发冲突在核心的事务内重查处
// 409，调用方按页计失败继续）。
func (s *Service) syncSelectorsCore(ctx context.Context, gameID, env, actor string, p *model.PageSpec, bindingIDs []string, dryRun bool, providedRevision int) (*PageSyncSelectorsResponse, error) {
	plan, err := s.planSelectorSync(ctx, gameID, env, p, bindingIDs)
	if err != nil {
		return nil, err
	}
	synced := plan.synced
	reports := plan.reports
	remaining := plan.remaining

	if dryRun {
		return &PageSyncSelectorsResponse{
			PageKey:              p.PageKey,
			DryRun:               true,
			Applied:              false,
			DraftRevision:        p.DraftRevision,
			SyncedBindings:       reports,
			RemainingDiagnostics: remaining,
		}, nil
	}

	// 幂等护栏（仅系统愈合循环）：无 selector 修复且无 manual 项时不落
	// 草稿——后台循环每 5 分钟一轮，不能让无变化页面反复 bump revision。
	// 用户/bulk 路径保持「同步必落草稿」旧契约（覆盖写语义与错误路径不变）。
	if actor == staleHealActor && countChangedBindings(reports) == 0 && countManualEntries(reports) == 0 {
		return &PageSyncSelectorsResponse{
			PageKey:        p.PageKey,
			DryRun:         dryRun,
			Applied:        false,
			DraftRevision:  p.DraftRevision,
			SyncedBindings: reports,
		}, nil
	}

	now := time.Now()
	if err := applyPageSpecToModel(p, synced); err != nil {
		return nil, err
	}
	p.GameID = gameID
	p.Env = env
	p.Status = "draft"
	p.UpdatedBy = actor
	p.UpdatedAt = now
	p.DraftRevision++
	specJSON, err := buildPageSpecJSONFn(p)
	if err != nil {
		return nil, err
	}
	err = s.withPageTransaction(ctx, func(txCtx context.Context, pageModel *model.PageSpecModel, _ *model.PublishedPageSpecModel, versionModel *model.PageVersionModel) error {
		// 事务内重查 revision（SaveDraft 同款乐观锁）：planner 运行期间
		// 草稿可能被并发保存，直接落库会静默冲掉他人定制。
		current, err := pageModel.FindByScopeAndPageKey(txCtx, gameID, env, p.PageKey)
		if err != nil {
			return err
		}
		if current.DraftRevision != providedRevision {
			return errorx.NewConflictWithDetails("草稿版本冲突：页面已被其他修改更新，请刷新草稿后重试", map[string]any{
				"expected": current.DraftRevision,
				"current":  current.DraftRevision,
				"provided": providedRevision,
			})
		}
		if err := pageModel.Upsert(txCtx, p); err != nil {
			return err
		}
		return versionModel.UpsertByScopePageKeyVersion(txCtx, &model.PageVersion{
			GameID:    gameID,
			Env:       env,
			PageKey:   p.PageKey,
			Version:   p.DraftRevision,
			SpecJSON:  specJSON,
			Status:    "draft",
			Message:   "sync selectors from latest function contracts",
			CreatedBy: actor,
			CreatedAt: now,
		})
	})
	if err != nil {
		return nil, err
	}

	s.auditPageEvent(ctx, audit.EventPageDraftSave, gameID, env, p.PageKey, map[string]interface{}{
		"action":            "sync_selectors",
		"draft_revision":    p.DraftRevision,
		"previous_revision": providedRevision,
		"binding_count":     len(reports),
		"changed_bindings":  countChangedBindings(reports),
		"manual_count":      countManualEntries(reports),
	})

	return &PageSyncSelectorsResponse{
		PageKey:              p.PageKey,
		DryRun:               false,
		Applied:              true,
		DraftRevision:        p.DraftRevision,
		SyncedBindings:       reports,
		RemainingDiagnostics: remaining,
	}, nil
}

// planSelectorSync 只规划不改库，dry-run、草稿同步与系统愈合共享同一
// selector/governance 决策，避免不同入口产生不同步的修复语义。
func (s *Service) planSelectorSync(ctx context.Context, gameID, env string, p *model.PageSpec, bindingIDs []string) (*selectorSyncPlan, error) {
	pageSpec, err := pageSpecFromModel(p)
	if err != nil {
		return nil, err
	}
	functions := s.functionsForScope(ctx, gameID, env)
	contracts := s.publishedContractsForSync(ctx, p)
	onlyBindings := map[string]struct{}{}
	for _, id := range bindingIDs {
		if id = strings.TrimSpace(id); id != "" {
			onlyBindings[id] = struct{}{}
		}
	}

	synced := pageSpec
	reports := make([]spec.BindingSelectorSyncReport, 0, len(pageSpec.Bindings))
	syncOpts := spec.SelectorSyncOptions{
		RecomputeDefaults: generator.RecomputeDefaultOutputs,
		IdentityOf:        s.identityFieldLookup(ctx, gameID, env),
	}
	for i, binding := range pageSpec.Bindings {
		if len(onlyBindings) > 0 {
			if _, ok := onlyBindings[strings.TrimSpace(binding.ID)]; !ok {
				continue
			}
		}
		fn, ok := functions[strings.TrimSpace(binding.FunctionID)]
		if !ok {
			reports = append(reports, spec.BindingSelectorSyncReport{
				BindingID:  binding.ID,
				FunctionID: binding.FunctionID,
				Manual: []spec.Diagnostic{diagnostic(
					"binding_function_missing",
					spec.SeverityError,
					"bound function no longer exists; remove or rebind it before publishing",
					"bindings."+binding.ID,
				)},
			})
			continue
		}
		contract, hasContract := contracts[strings.TrimSpace(binding.ID)]
		if hasContract {
			syncOpts.PrevTrusted = contract.InputSchemaDigest != "" &&
				contract.OutputSchemaDigest != "" &&
				freshness.MatchesDigest(fn.PreviousInputSchema, contract.InputSchemaDigest) &&
				freshness.MatchesDigest(fn.PreviousOutputSchema, contract.OutputSchemaDigest)
		} else {
			syncOpts.PrevTrusted = false
		}
		report, syncedBinding := spec.PlanBindingSelectorSync(pageSpec, binding, fn, syncOpts)
		if hasContract {
			for _, stale := range freshness.EvaluateBinding(syncedBinding, contract, functions) {
				switch stale.Status {
				case spec.BindingFreshnessGovernanceStale, spec.BindingFreshnessFunctionVersionStale:
					stale.Diagnostic.Field = "bindings." + binding.ID
					report.Manual = append(report.Manual, stale.Diagnostic)
				}
			}
		}
		reports = append(reports, report)
		synced.Bindings[i] = syncedBinding
	}

	return &selectorSyncPlan{
		pageSpec:  pageSpec,
		synced:    synced,
		reports:   reports,
		remaining: s.ValidatePageSpecInScope(ctx, gameID, env, synced, true),
	}, nil
}

// autoPublishAfterSync 是「自动化收口」（2026-09）：单页同步成功且
// selector 全部适配（无 manual 遗留）、页面已发布时，自动接续发布，
// 刷新契约快照并恢复控制台执行——用户不再需要「同步 → 发布」两步。
// 发布权限缺失或发布失败降级为 AutoPublishError 提示，草稿保持已同步。
func (s *Service) autoPublishAfterSync(ctx context.Context, resp *PageSyncSelectorsResponse, gameID, env, actor string, publishedVersion int, draftRevision int) {
	if resp == nil || !resp.Applied || publishedVersion == 0 || countManualEntries(resp.SyncedBindings) > 0 {
		return
	}
	if err := s.requirePagePublish(ctx); err != nil {
		resp.AutoPublishError = "publish_permission_required"
		return
	}
	rev := draftRevision
	if _, err := s.publishFromEntry(ctx, pagePublishEntry{
		Caller:           pagePublishCallerAPI,
		Actor:            actor,
		AllowsAutoPolicy: false,
		RequiresPublish:  false,
		GameID:           gameID,
		Env:              env,
	}, &PagePublishRequest{PageKey: resp.PageKey, DraftRevision: &rev}); err != nil {
		resp.AutoPublishError = err.Error()
		return
	}
	resp.AutoPublished = true
}

type selectorSyncPlan struct {
	pageSpec  spec.PageSpec
	synced    spec.PageSpec
	reports   []spec.BindingSelectorSyncReport
	remaining []spec.Diagnostic
}

// publishedContractsForSync 取最新发布快照的 binding contract（prev
// 精确性判定与不可修项透传都需要）；无发布历史的页返回空 map。
func (s *Service) publishedContractsForSync(ctx context.Context, p *model.PageSpec) map[string]spec.BindingContractSnapshot {
	out := map[string]spec.BindingContractSnapshot{}
	if s == nil || s.svcCtx == nil || s.svcCtx.PublishedPageSpecModel == nil || p == nil || p.PublishedVersion == 0 {
		return out
	}
	published, err := s.svcCtx.PublishedPageSpecModel.FindLatestByScopeAndPageKey(ctx, p.GameID, p.Env, p.PageKey)
	if err != nil {
		return out
	}
	_, contracts := parsePublishedPageForFreshness(*published)
	for _, contract := range contracts {
		if id := strings.TrimSpace(contract.BindingID); id != "" {
			out[id] = contract
		}
	}
	return out
}

func countChangedBindings(reports []spec.BindingSelectorSyncReport) int {
	n := 0
	for _, report := range reports {
		if report.Changed {
			n++
		}
	}
	return n
}

func countManualEntries(reports []spec.BindingSelectorSyncReport) int {
	n := 0
	for _, report := range reports {
		for _, entry := range report.Input {
			if entry.Action == spec.SelectorSyncManual {
				n++
			}
		}
		for _, entry := range report.Output {
			if entry.Action == spec.SelectorSyncManual {
				n++
			}
		}
		n += len(report.Manual)
	}
	return n
}

func (s *Service) findDraft(ctx context.Context, pageKey string) (*model.PageSpec, error) {
	gameID, env, err := requireScope(ctx)
	if err != nil {
		return nil, err
	}
	p, err := s.svcCtx.PageSpecModel.FindByScopeAndPageKey(ctx, gameID, env, pageKey)
	if err != nil {
		return nil, ErrPageNotFound(pageKey)
	}
	return p, nil
}

// BulkSyncSelectors 批量同步 stale selector（契约变更队列的草稿侧收口）：
// 逐页跑与单页 SyncSelectors 同源的 planner/apply 核心。与单页的差异：
//   - pageKeys 为空时取审批收件箱 ContractChanges 的全部 pageKey
//     （published 与 draft 都取——同步只动草稿，上线仍走 bulk-republish）；
//   - revision 由服务端读取当前草稿版本（并发修改在核心事务内 409，
//     按页计入 Failed 继续，不中断其余页面）；
//   - 先 dry-run 判定：存在 Manual（governance/version 等不可由 selector
//     同步修复的漂移）的页面整体 Skipped 并透传诊断，不做半吊子同步；
//   - 严格只写 draft（published_page_specs 不动），不自动 publish。
func (s *Service) BulkSyncSelectors(ctx context.Context, req *PageBulkSyncSelectorsRequest) (*PageBulkSyncSelectorsResult, error) {
	if err := s.requirePageEdit(ctx); err != nil {
		return nil, err
	}
	gameID, env, err := requireScope(ctx)
	if err != nil {
		return nil, err
	}
	actor, err := currentUsername(ctx)
	if err != nil {
		return nil, err
	}

	// pageKey 去重：Inbox 的 ContractChanges 对同一页面可能同时挂
	// published 与 draft 两条变更；重复同步会因 revision 过期产生伪 409。
	pageKeys := make([]string, 0, len(req.PageKeys))
	seen := map[string]bool{}
	appendKey := func(key string) {
		if key = strings.TrimSpace(key); key == "" || seen[key] {
			return
		}
		seen[key] = true
		pageKeys = append(pageKeys, key)
	}
	for _, key := range req.PageKeys {
		appendKey(key)
	}
	if len(pageKeys) == 0 {
		proposalSvc := contractsvc.NewProposalService(s.svcCtx.DB)
		inbox, err := proposalSvc.Inbox(ctx, gameID, env, contractsvc.ProposalListFilter{})
		if err != nil {
			return nil, err
		}
		for _, change := range inbox.ContractChanges {
			appendKey(change.PageKey)
		}
	}

	result := &PageBulkSyncSelectorsResult{Total: len(pageKeys)}
	for _, pageKey := range pageKeys {
		p, err := s.svcCtx.PageSpecModel.FindByScopeAndPageKey(ctx, gameID, env, pageKey)
		if err != nil {
			result.Failed = append(result.Failed, map[string]string{
				"pageKey": pageKey,
				"error":   ErrPageNotFound(pageKey).Error(),
			})
			continue
		}
		// dry-run 判定：Manual 非空（不可由 selector 同步修复的漂移）→
		// 整页跳过并透传诊断，由人工决定重绑定/下线/重发布。
		dry, err := s.syncSelectorsCore(ctx, gameID, env, actor, p, nil, true, p.DraftRevision)
		if err != nil {
			result.Failed = append(result.Failed, map[string]string{
				"pageKey": pageKey,
				"error":   err.Error(),
			})
			continue
		}
		if manual := collectManualDiagnostics(dry.SyncedBindings); len(manual) > 0 {
			result.Skipped = append(result.Skipped, PageBulkSyncSelectorsSkipped{
				PageKey: pageKey,
				Reason:  "manual_required",
				Manual:  manual,
			})
			continue
		}
		if _, err := s.syncSelectorsCore(ctx, gameID, env, actor, p, nil, false, p.DraftRevision); err != nil {
			result.Failed = append(result.Failed, map[string]string{
				"pageKey": pageKey,
				"error":   err.Error(),
			})
			continue
		}
		result.Synced = append(result.Synced, pageKey)
	}
	return result, nil
}

// collectManualDiagnostics 汇总各 binding 报告里的 Manual 诊断（批量
// 同步的跳过原因透传）。
func collectManualDiagnostics(reports []spec.BindingSelectorSyncReport) []spec.Diagnostic {
	var manual []spec.Diagnostic
	for _, report := range reports {
		manual = append(manual, report.Manual...)
	}
	return manual
}
