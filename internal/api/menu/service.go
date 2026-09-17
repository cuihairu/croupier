package menu

import (
	"context"
	"errors"
	"regexp"
	"strings"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/logic/utils"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
	"gorm.io/gorm"
)

// requireScope is the package-local test seam mirror of the page service:
// production always uses the real implementation.
var requireScope = func(ctx context.Context) (string, string, error) {
	scope := svc.GameScopeFromContext(ctx)
	gameID := strings.TrimSpace(scope.GameID)
	env := strings.TrimSpace(scope.Env)
	if gameID == "" {
		return "", "", errorx.NewBadRequest("X-Game-ID is required")
	}
	if env == "" {
		return "", "", errorx.NewBadRequest("X-Env is required")
	}
	return gameID, env, nil
}

var menuKeyPattern = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_-]{0,63}$`)

type Service struct {
	svcCtx *svc.ServiceContext
}

func NewService(svcCtx *svc.ServiceContext) *Service {
	return &Service{svcCtx: svcCtx}
}

func (s *Service) menuModel() *model.MenuItemModel {
	return s.svcCtx.MenuModel
}

// List returns the full menu tree of the current scope.
func (s *Service) List(ctx context.Context) (*MenuListResponse, error) {
	if _, _, err := utils.RequireAnyPermission(ctx, s.svcCtx, "无权查看菜单", "admin:all", "menu:read", "menu:update", "menu:create", "menu:delete"); err != nil {
		return nil, err
	}
	gameID, env, err := requireScope(ctx)
	if err != nil {
		return nil, err
	}
	items, err := s.menuModel().ListByScope(ctx, gameID, env)
	if err != nil {
		return nil, err
	}
	return &MenuListResponse{Items: buildMenuTree(items)}, nil
}

// Create adds a new menu node in the current scope.
func (s *Service) Create(ctx context.Context, req *CreateMenuRequest) (*MenuDTO, error) {
	if _, _, err := utils.RequireAnyPermission(ctx, s.svcCtx, "无权创建菜单", "admin:all", "menu:create"); err != nil {
		return nil, err
	}
	gameID, env, err := requireScope(ctx)
	if err != nil {
		return nil, err
	}

	menuKey := strings.TrimSpace(req.MenuKey)
	if !menuKeyPattern.MatchString(menuKey) {
		return nil, errorx.NewBadRequestWithDetails("菜单标识无效", map[string]any{
			"menuKey": "仅允许字母开头的字母/数字/中划线/下划线，长度 1-64",
		})
	}
	if err := validateLabels(req.Labels); err != nil {
		return nil, err
	}

	if _, err := s.menuModel().FindByScopeAndKey(ctx, gameID, env, menuKey); err == nil {
		return nil, errorx.NewConflict("菜单标识已存在: " + menuKey)
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	parentID, err := s.resolveParentID(ctx, gameID, env, req.ParentID, 0)
	if err != nil {
		return nil, err
	}

	item := &model.MenuItem{
		GameID:     gameID,
		Env:        env,
		ParentID:   parentID,
		MenuKey:    menuKey,
		Icon:       strings.TrimSpace(req.Icon),
		Permission: strings.TrimSpace(req.Permission),
		IsVisible:  req.IsVisible == nil || *req.IsVisible,
	}
	if req.SortOrder != nil {
		item.SortOrder = *req.SortOrder
	}
	if err := item.SetLabels(req.Labels); err != nil {
		return nil, err
	}
	if err := s.menuModel().Create(ctx, item); err != nil {
		return nil, err
	}
	return menuToDTO(item), nil
}

// Update applies partial updates to a scoped menu node.
func (s *Service) Update(ctx context.Context, req *UpdateMenuRequest) (*MenuDTO, error) {
	if _, _, err := utils.RequireAnyPermission(ctx, s.svcCtx, "无权编辑菜单", "admin:all", "menu:update"); err != nil {
		return nil, err
	}
	gameID, env, err := requireScope(ctx)
	if err != nil {
		return nil, err
	}
	id, err := utils.ParseUintID(req.ID, "菜单 ID")
	if err != nil {
		return nil, err
	}

	item, err := s.menuModel().FindByID(ctx, gameID, env, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errorx.NewNotFound("菜单不存在")
		}
		return nil, err
	}

	if req.MenuKey != nil {
		menuKey := strings.TrimSpace(*req.MenuKey)
		if !menuKeyPattern.MatchString(menuKey) {
			return nil, errorx.NewBadRequestWithDetails("菜单标识无效", map[string]any{
				"menuKey": "仅允许字母开头的字母/数字/中划线/下划线，长度 1-64",
			})
		}
		if menuKey != item.MenuKey {
			if _, err := s.menuModel().FindByScopeAndKey(ctx, gameID, env, menuKey); err == nil {
				return nil, errorx.NewConflict("菜单标识已存在: " + menuKey)
			} else if !errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, err
			}
		}
		item.MenuKey = menuKey
	}

	if req.ParentID != nil {
		parentID, err := s.resolveParentID(ctx, gameID, env, req.ParentID, item.ID)
		if err != nil {
			return nil, err
		}
		if err := s.ensureNoCycle(ctx, gameID, env, item.ID, parentID); err != nil {
			return nil, err
		}
		item.ParentID = parentID
	}

	if req.Labels != nil {
		if err := validateLabels(*req.Labels); err != nil {
			return nil, err
		}
		if err := item.SetLabels(*req.Labels); err != nil {
			return nil, err
		}
	}
	if req.Icon != nil {
		item.Icon = strings.TrimSpace(*req.Icon)
	}
	if req.Permission != nil {
		item.Permission = strings.TrimSpace(*req.Permission)
	}
	if req.SortOrder != nil {
		item.SortOrder = *req.SortOrder
	}
	if req.IsVisible != nil {
		item.IsVisible = *req.IsVisible
	}

	if err := s.menuModel().Save(ctx, item); err != nil {
		return nil, err
	}
	return menuToDTO(item), nil
}

// Delete removes a scoped menu node together with all its descendants
// (ON DELETE CASCADE semantics from the design).
func (s *Service) Delete(ctx context.Context, req *UpdateMenuRequest) error {
	if _, _, err := utils.RequireAnyPermission(ctx, s.svcCtx, "无权删除菜单", "admin:all", "menu:delete"); err != nil {
		return err
	}
	gameID, env, err := requireScope(ctx)
	if err != nil {
		return err
	}
	id, err := utils.ParseUintID(req.ID, "菜单 ID")
	if err != nil {
		return err
	}
	if _, err := s.menuModel().FindByID(ctx, gameID, env, id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errorx.NewNotFound("菜单不存在")
		}
		return err
	}

	items, err := s.menuModel().ListByScope(ctx, gameID, env)
	if err != nil {
		return err
	}
	children := make(map[uint][]uint, len(items))
	for i := range items {
		if items[i].ParentID != nil {
			children[*items[i].ParentID] = append(children[*items[i].ParentID], items[i].ID)
		}
	}
	toDelete := collectDescendants(id, children)
	if err := s.menuModel().Delete(ctx, gameID, env, id); err != nil {
		return err
	}
	for _, childID := range toDelete {
		if err := s.menuModel().Delete(ctx, gameID, env, childID); err != nil {
			return err
		}
	}
	return nil
}

// UpdateSort changes only the sort order of a scoped menu node.
func (s *Service) UpdateSort(ctx context.Context, req *SortMenuRequest) (*MenuDTO, error) {
	if _, _, err := utils.RequireAnyPermission(ctx, s.svcCtx, "无权调整菜单排序", "admin:all", "menu:sort", "menu:update"); err != nil {
		return nil, err
	}
	gameID, env, err := requireScope(ctx)
	if err != nil {
		return nil, err
	}
	id, err := utils.ParseUintID(req.ID, "菜单 ID")
	if err != nil {
		return nil, err
	}
	if _, err := s.menuModel().FindByID(ctx, gameID, env, id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errorx.NewNotFound("菜单不存在")
		}
		return nil, err
	}
	if err := s.menuModel().UpdateSortOrder(ctx, gameID, env, id, req.SortOrder); err != nil {
		return nil, err
	}
	item, err := s.menuModel().FindByID(ctx, gameID, env, id)
	if err != nil {
		return nil, err
	}
	return menuToDTO(item), nil
}

// Accessible returns the permission-filtered menu tree for the current user.
// No menu:read gate here: this is the login-time endpoint for every admin.
func (s *Service) Accessible(ctx context.Context) (*MenuListResponse, error) {
	gameID, env, err := requireScope(ctx)
	if err != nil {
		return nil, err
	}
	_, roles, err := utils.LoadCurrentAdmin(ctx, s.svcCtx)
	if err != nil {
		return nil, err
	}
	permIDs, err := utils.PermissionIDsFromRoles(ctx, s.svcCtx, roles)
	if err != nil {
		return nil, err
	}
	if utils.HasAdminRole(utils.RoleNamesFromModels(roles)) {
		permIDs = append(permIDs, "admin:all", "*")
	}
	items, err := s.menuModel().ListByScope(ctx, gameID, env)
	if err != nil {
		return nil, err
	}
	return &MenuListResponse{Items: filterAccessibleTree(items, permIDs)}, nil
}

// filterAccessibleTree builds the user-visible menu tree. Rules follow the
// design (docs/design/menu-management.md §7): invisible menus are pruned,
// a menu-level permission must be held by the user, and requirements cascade
// from parents to children (an inaccessible parent hides its whole branch).
// Orphan nodes (parent record missing entirely) are promoted to roots; a
// parent that exists but is filtered out hides its children.
func filterAccessibleTree(items []model.MenuItem, permIDs []string) []*MenuDTO {
	allowed := make(map[string]struct{}, len(permIDs))
	for _, id := range permIDs {
		allowed[strings.ToLower(strings.TrimSpace(id))] = struct{}{}
	}
	_, hasWildcard := allowed["*"]
	_, hasAdminAll := allowed["admin:all"]
	hasAll := hasWildcard || hasAdminAll

	byID := make(map[uint]*model.MenuItem, len(items))
	for i := range items {
		byID[items[i].ID] = &items[i]
	}

	accessible := make(map[uint]bool, len(items))
	var check func(item *model.MenuItem) bool
	check = func(item *model.MenuItem) bool {
		if visible, seen := accessible[item.ID]; seen {
			return visible
		}
		ok := item.IsVisible
		if ok && item.Permission != "" && !hasAll {
			if _, found := allowed[strings.ToLower(strings.TrimSpace(item.Permission))]; !found {
				ok = false
			}
		}
		// 先落 false 再递归父级：脏数据成环时读到的 false 直接终止递归
		accessible[item.ID] = false
		if ok && item.ParentID != nil {
			if parent, exists := byID[*item.ParentID]; exists {
				ok = check(parent)
			}
		}
		accessible[item.ID] = ok
		return ok
	}

	dtos := make([]*MenuDTO, 0, len(items))
	byDTO := make(map[uint]*MenuDTO, len(items))
	for i := range items {
		dto := menuToDTO(&items[i])
		dtos = append(dtos, dto)
		byDTO[items[i].ID] = dto
	}

	roots := make([]*MenuDTO, 0)
	for i := range items {
		item := &items[i]
		if !check(item) {
			continue
		}
		if item.ParentID == nil {
			roots = append(roots, byDTO[item.ID])
			continue
		}
		parent, exists := byID[*item.ParentID]
		if !exists {
			roots = append(roots, byDTO[item.ID])
			continue
		}
		if !check(parent) {
			continue
		}
		parentDTO := byDTO[parent.ID]
		parentDTO.Children = append(parentDTO.Children, byDTO[item.ID])
	}
	return roots
}

// resolveParentID validates the requested parent: 0/nil means root; otherwise
// the parent must exist in the same scope and cannot be the node itself.
func (s *Service) resolveParentID(ctx context.Context, gameID, env string, parentID *int64, selfID uint) (*uint, error) {
	if parentID == nil || *parentID == 0 {
		return nil, nil
	}
	parent, err := s.menuModel().FindByID(ctx, gameID, env, uint(*parentID))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errorx.NewNotFound("父菜单不存在")
		}
		return nil, err
	}
	if selfID != 0 && parent.ID == selfID {
		return nil, errorx.NewBadRequest("父菜单不能是自身")
	}
	return &parent.ID, nil
}

// ensureNoCycle walks up from newParent; reaching selfID means the move would
// create a cycle.
func (s *Service) ensureNoCycle(ctx context.Context, gameID, env string, selfID uint, newParentID *uint) error {
	current := newParentID
	for current != nil {
		if *current == selfID {
			return errorx.NewBadRequest("不能将菜单移动到自己的子级")
		}
		parent, err := s.menuModel().FindByID(ctx, gameID, env, *current)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil
			}
			return err
		}
		if parent.ParentID == nil {
			return nil
		}
		current = parent.ParentID
	}
	return nil
}

func collectDescendants(root uint, children map[uint][]uint) []uint {
	var out []uint
	stack := []uint{root}
	for len(stack) > 0 {
		id := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		for _, child := range children[id] {
			out = append(out, child)
			stack = append(stack, child)
		}
	}
	return out
}

func validateLabels(labels spec.LocalizedText) error {
	for _, value := range labels {
		if strings.TrimSpace(value) != "" {
			return nil
		}
	}
	return errorx.NewBadRequestWithDetails("菜单名称不能为空", map[string]any{
		"labels": "至少需要一个语言的名称",
	})
}

func menuToDTO(item *model.MenuItem) *MenuDTO {
	var parentID *int64
	if item.ParentID != nil {
		v := int64(*item.ParentID)
		parentID = &v
	}
	labels := item.GetLabels()
	if labels == nil {
		labels = map[string]string{}
	}
	return &MenuDTO{
		ID:         int64(item.ID),
		ParentID:   parentID,
		MenuKey:    item.MenuKey,
		Labels:     spec.LocalizedText(labels),
		Icon:       item.Icon,
		SortOrder:  item.SortOrder,
		Permission: item.Permission,
		IsVisible:  item.IsVisible,
		Children:   []*MenuDTO{},
	}
}

// buildMenuTree assembles an ordered flat list into a tree. Orphan nodes
// (parent missing or soft-deleted) are promoted to roots so no node is lost.
func buildMenuTree(items []model.MenuItem) []*MenuDTO {
	byID := make(map[uint]*MenuDTO, len(items))
	dtos := make([]*MenuDTO, 0, len(items))
	for i := range items {
		dto := menuToDTO(&items[i])
		dtos = append(dtos, dto)
		byID[items[i].ID] = dto
	}
	roots := make([]*MenuDTO, 0)
	for i := range items {
		item := &items[i]
		if item.ParentID == nil {
			roots = append(roots, dtos[i])
			continue
		}
		parent, ok := byID[*item.ParentID]
		if !ok {
			roots = append(roots, dtos[i])
			continue
		}
		parent.Children = append(parent.Children, dtos[i])
	}
	return roots
}
