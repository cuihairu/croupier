package console

import (
	"context"
	"net/url"
	"sort"
	"strings"

	"github.com/cuihairu/croupier/internal/api/menu"
	"github.com/cuihairu/croupier/internal/dashboard/spec"
)

// generateMenuFromMenuItems assembles the console navigation from the
// permission-filtered menu tree. The page→menu mount map is read from the
// draft table (page_specs.menu_id) so re-mounting takes effect without
// republishing; page content (title/icon/order) comes from the latest
// published snapshot — draft-only pages never reach the console.
func (s *Service) generateMenuFromMenuItems(ctx context.Context, gameID, env, lang string) (spec.ConsoleMenuSpec, error) {
	tree, err := menu.AccessibleTree(ctx, s.svcCtx)
	if err != nil {
		return spec.ConsoleMenuSpec{}, err
	}
	if len(tree) == 0 {
		return spec.ConsoleMenuSpec{Items: []spec.ConsoleMenuItem{}}, nil
	}
	specs, err := s.svcCtx.PageSpecModel.ListByScope(ctx, gameID, env)
	if err != nil {
		return spec.ConsoleMenuSpec{}, err
	}
	publishedPages, err := s.svcCtx.PublishedPageSpecModel.ListLatestActiveByScope(ctx, gameID, env)
	if err != nil {
		return spec.ConsoleMenuSpec{}, err
	}
	published := parsePublishedPages(publishedPages)
	publishedByKey := make(map[string]spec.PublishedPageSpec, len(published))
	for _, page := range published {
		publishedByKey[page.PageKey] = page
	}
	pagesByMenu := make(map[uint][]pageEntry)
	for i := range specs {
		menuID := specs[i].MenuID
		if menuID == nil {
			continue
		}
		page, ok := publishedByKey[specs[i].PageKey]
		if !ok {
			continue
		}
		pagesByMenu[*menuID] = append(pagesByMenu[*menuID], pageEntry{
			key:   page.PageKey,
			title: page.Title,
			icon:  page.Icon,
			order: page.Order,
		})
	}
	items := make([]spec.ConsoleMenuItem, 0, len(tree))
	for _, node := range tree {
		items = append(items, consoleMenuItemFromMenu(node, pagesByMenu, lang))
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Order != items[j].Order {
			return items[i].Order < items[j].Order
		}
		left := getLocalizedText(items[i].Title, lang, items[i].Key)
		right := getLocalizedText(items[j].Title, lang, items[j].Key)
		if left != right {
			return left < right
		}
		return items[i].Key < items[j].Key
	})
	return spec.ConsoleMenuSpec{Items: items}, nil
}

// consoleMenuItemFromMenu maps a menu node to a console menu item. Children
// merge submenus and mounted published pages, sorted by order → localized
// title → key (same rule the page-driven menu used).
func consoleMenuItemFromMenu(node *menu.MenuDTO, pagesByMenu map[uint][]pageEntry, lang string) spec.ConsoleMenuItem {
	// MenuDTO.ID 是 int64（API 契约），挂载映射按模型主键 uint 索引。
	menuID := uint(node.ID)
	children := make([]spec.ConsoleMenuItem, 0, len(node.Children)+len(pagesByMenu[menuID]))
	for _, child := range node.Children {
		children = append(children, consoleMenuItemFromMenu(child, pagesByMenu, lang))
	}
	for _, entry := range pagesByMenu[menuID] {
		children = append(children, spec.ConsoleMenuItem{
			Key:    entry.key,
			Path:   consolePagePath(node.MenuKey, entry.key),
			Title:  entry.title,
			Locale: false,
			Icon:   entry.icon,
			Order:  entry.order,
		})
	}
	sort.Slice(children, func(i, j int) bool {
		if children[i].Order != children[j].Order {
			return children[i].Order < children[j].Order
		}
		left := getLocalizedText(children[i].Title, lang, children[i].Key)
		right := getLocalizedText(children[j].Title, lang, children[j].Key)
		if left != right {
			return left < right
		}
		return children[i].Key < children[j].Key
	})
	item := spec.ConsoleMenuItem{
		Key:      node.MenuKey,
		Path:     consoleCategoryPath(node.MenuKey),
		Title:    node.Labels,
		Locale:   false,
		Order:    node.SortOrder,
		Children: children,
	}
	// 菜单未配 icon 时沿用页面驱动菜单的兜底：取组内第一个非空页面图标。
	for _, child := range children {
		if item.Icon == "" && strings.TrimSpace(child.Icon) != "" {
			item.Icon = child.Icon
			break
		}
	}
	return item
}

func consoleCategoryPath(categoryKey string) string {
	return "/console/" + url.PathEscape(categoryKey)
}

func consolePagePath(categoryKey string, pageKey string) string {
	return consoleCategoryPath(categoryKey) + "/" + url.PathEscape(pageKey)
}
