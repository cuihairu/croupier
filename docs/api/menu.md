---
title: 控制台菜单 API
icon: bars-staggered
order: 22
---

# 控制台菜单 API（Menus）

控制台动态菜单的管理面 CRUD 与当前用户可见树。实现以 `internal/api/menu/` 为准；
菜单模型与渲染链路见 [控制台动态菜单](../architecture/console-dynamic-menu.md)。

- **认证**：均需 JWT；挂在 scoped 组（`X-Game-ID`/`X-Env` 生效，菜单绑定 per-game）。
- **labels 为 `LocalizedText`**（BCP47 键，如 `{"zh-CN": "...", "en-US": "..."}`），
  契约见 [本地化文本契约](../architecture/localized-text-contract.md)。

## 端点

| 方法   | 路径                       | 说明                                       |
| ------ | -------------------------- | ------------------------------------------ |
| GET    | `/api/v1/menus`            | 全量菜单树（`items`）                      |
| GET    | `/api/v1/menus/accessible` | 当前用户可见菜单树（按权限过滤）           |
| POST   | `/api/v1/menus`            | 创建菜单项                                 |
| PUT    | `/api/v1/menus/:id`        | 更新（指针字段：提供 = 更新，省略 = 保持） |
| PUT    | `/api/v1/menus/:id/sort`   | 更新排序                                   |
| DELETE | `/api/v1/menus/:id`        | 删除                                       |

## 结构

```go
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

// POST 创建
type CreateMenuRequest struct {
	MenuKey    string             `json:"menuKey" binding:"required"`
	ParentID   *int64             `json:"parentId"`
	Labels     spec.LocalizedText `json:"labels"`
	Icon       string             `json:"icon"`
	SortOrder  *int               `json:"sortOrder"`
	Permission string             `json:"permission"`
	IsVisible  *bool              `json:"isVisible"`
}

// PUT /:id/sort
type SortMenuRequest struct {
	SortOrder int `json:"sortOrder"`
}
```

## 语义注记

- `PUT /menus/:id` 中 `parentId = 0` 表示移回根级；树由服务端按 `sortOrder` 组装。
- `permission` 填权限码时，`GET /menus/accessible` 按当前账号权限裁剪不可见项。
