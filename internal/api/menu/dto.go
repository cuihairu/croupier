package menu

import (
	"github.com/cuihairu/croupier/internal/dashboard/spec"
)

// MenuDTO is the API representation of a menu node. Contract keys follow the
// repo-wide lowerCamelCase convention.
type MenuDTO struct {
	ID         int64              `json:"id"`
	ParentID   *int64             `json:"parentId"`
	MenuKey    string             `json:"menuKey"`
	Labels     spec.LocalizedText `json:"labels"`
	Icon       string             `json:"icon,omitempty"`
	SortOrder  int                `json:"sortOrder"`
	Permission string             `json:"permission,omitempty"`
	IsVisible  bool               `json:"isVisible"`
	Children   []*MenuDTO         `json:"children"`
}

// CreateMenuRequest is the payload for POST /api/v1/menus.
type CreateMenuRequest struct {
	MenuKey    string             `json:"menuKey" binding:"required"`
	ParentID   *int64             `json:"parentId"`
	Labels     spec.LocalizedText `json:"labels"`
	Icon       string             `json:"icon"`
	SortOrder  *int               `json:"sortOrder"`
	Permission string             `json:"permission"`
	IsVisible  *bool              `json:"isVisible"`
}

// UpdateMenuRequest is the payload for PUT /api/v1/menus/:id. Pointer fields
// follow "provided = update, omitted = keep" semantics; ParentID = 0 moves the
// node back to root level.
type UpdateMenuRequest struct {
	ID         string              `uri:"id" binding:"required"`
	MenuKey    *string             `json:"menuKey"`
	ParentID   *int64              `json:"parentId"`
	Labels     *spec.LocalizedText `json:"labels"`
	Icon       *string             `json:"icon"`
	SortOrder  *int                `json:"sortOrder"`
	Permission *string             `json:"permission"`
	IsVisible  *bool               `json:"isVisible"`
}

// SortMenuRequest is the payload for PUT /api/v1/menus/:id/sort.
type SortMenuRequest struct {
	ID        string `uri:"id" binding:"required"`
	SortOrder int    `json:"sortOrder"`
}

// MenuListResponse wraps the menu tree for GET /api/v1/menus.
type MenuListResponse struct {
	Items []*MenuDTO `json:"items"`
}
