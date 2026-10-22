# Menu Management System Design Document

> The current menu-driven model and scope rules are governed by [the console dynamic menu](../architecture/console-dynamic-menu.md); the seven classes of default game menus and suggested business pages settled by the user on 2026-10-17 are in [default game menus](default-game-menus.md), and `configs/default-menus.json` has been updated accordingly. This file keeps the historical modeling background.

> Status: Proposal
> Author: Cui Haitao
> Date: 2026-09-17
> Related: the Croupier Dashboard page-taxonomy consistency problem

---

## 1. Problem Background

### 1.1 The Current Design

Croupier pages (PageSpecs) are classified through the `category` field:

```json
{
  "key": "resource--player",
  "category": {
    "key": "resource",
    "labels": {
      "zh-CN": "资源管理",
      "en-US": "Resource Management"
    }
  }
}
```

### 1.2 The Problems

1. **Classification labels are scattered**: every page stores `category.labels`, which must stay identical across all pages of the same classification.
2. **Changes are expensive**: renaming a classification requires synchronously updating every page under it.
3. **Consistency validation is strict**: editing a page errors out if the labels do not match.
4. **No menu hierarchy**: multi-level menu nesting is impossible.
5. **Coarse permission granularity**: access can only be controlled per classification, not per menu item.

---

## 2. Design Goals

1. **Menus managed independently**: classifications/menus become standalone entities, fully decoupled from pages.
2. **Pages choose menus**: the page editor selects which menu a page hangs under.
3. **Internationalization**: menu names support multiple languages.
4. **Hierarchical structure**: multi-level menu nesting is supported.
5. **Permissions**: visibility and operation permissions are controlled at the menu level.
6. **No backward compatibility**: `category.labels` is deprecated outright in favor of the menu system.

---

## 3. Data Model

### 3.1 The Menu Table (menu_items)

```sql
CREATE TABLE menu_items (
    id              SERIAL PRIMARY KEY,
    parent_id       INTEGER REFERENCES menu_items(id) ON DELETE CASCADE,
    menu_key        VARCHAR(64) NOT NULL UNIQUE,
    labels          JSONB NOT NULL,
    icon            VARCHAR(64),
    sort_order      INTEGER DEFAULT 0,
    permission      VARCHAR(128),
    is_visible      BOOLEAN DEFAULT true,
    created_at      TIMESTAMP DEFAULT NOW(),
    updated_at      TIMESTAMP DEFAULT NOW()
);
```

### 3.2 Page Table Changes

```sql
-- 删除旧字段
ALTER TABLE pages DROP COLUMN IF EXISTS category_key;
ALTER TABLE pages DROP COLUMN IF EXISTS category_labels;

-- 添加新字段
ALTER TABLE pages ADD COLUMN menu_id INTEGER REFERENCES menu_items(id);
```

### 3.3 Sample Menu Data

```sql
INSERT INTO menu_items (menu_key, labels, icon, sort_order) VALUES
('resource', '{"zh-CN": "资源管理", "en-US": "Resource"}', 'DatabaseOutlined', 1),
('operation', '{"zh-CN": "运营工具", "en-US": "Operations"}', 'ToolOutlined', 2),
('analytics', '{"zh-CN": "数据分析", "en-US": "Analytics"}', 'BarChartOutlined', 3);

INSERT INTO menu_items (parent_id, menu_key, labels, icon, sort_order) VALUES
(1, 'player', '{"zh-CN": "玩家管理", "en-US": "Player"}', 'UserOutlined', 1),
(1, 'order', '{"zh-CN": "订单管理", "en-US": "Order"}', 'FileTextOutlined', 2),
(1, 'leaderboard', '{"zh-CN": "排行榜", "en-US": "Leaderboard"}', 'TrophyOutlined', 3);
```

---

## 4. API Design

### 4.1 Menu Management APIs

| Method | Path                     | Description       |
| ------ | ------------------------ | ----------------- |
| GET    | `/api/v1/menus`          | Get the menu tree |
| POST   | `/api/v1/menus`          | Create a menu     |
| PUT    | `/api/v1/menus/:id`      | Update a menu     |
| DELETE | `/api/v1/menus/:id`      | Delete a menu     |
| PUT    | `/api/v1/menus/:id/sort` | Update ordering   |

### 4.2 Page Association APIs

| Method | Path                       | Description                 |
| ------ | -------------------------- | --------------------------- |
| PUT    | `/api/v1/pages/:key/menu`  | Set the page's menu         |
| GET    | `/api/v1/menus/:key/pages` | List the pages under a menu |

### 4.3 API Request/Response Examples

**Create a menu**

```json
POST /api/v1/menus
{
  "menu_key": "resource",
  "labels": {
    "zh-CN": "资源管理",
    "en-US": "Resource Management"
  },
  "icon": "DatabaseOutlined",
  "sort_order": 1,
  "permission": "resource:read"
}
```

**Response**

```json
{
  "id": 1,
  "menu_key": "resource",
  "labels": {
    "zh-CN": "资源管理",
    "en-US": "Resource Management"
  },
  "icon": "DatabaseOutlined",
  "sort_order": 1,
  "children": [
    { "id": 2, "menu_key": "player", "labels": { "zh-CN": "玩家管理" } },
    { "id": 3, "menu_key": "order", "labels": { "zh-CN": "订单管理" } }
  ]
}
```

---

## 5. Frontend UI Design

### 5.1 The Icon Approach

Use the **SVG components** from `@ant-design/icons`, consistent with the existing project:

```tsx
import {
  DatabaseOutlined,
  ToolOutlined,
  BarChartOutlined,
} from "@ant-design/icons";

// 菜单配置
const menuConfig = {
  icon: <DatabaseOutlined />, // SVG 组件
};
```

**Icon mapping table** (the icon name is stored; the frontend renders it dynamically):

| icon field         | Component | Purpose             |
| ------------------ | --------- | ------------------- |
| `DatabaseOutlined` | 📂        | Resource management |
| `ToolOutlined`     | 🛠️        | Operations tools    |
| `BarChartOutlined` | 📊        | Data analytics      |
| `UserOutlined`     | 👤        | Player management   |
| `FileTextOutlined` | 📋        | Order management    |
| `TrophyOutlined`   | 🏆        | Leaderboard         |
| `SettingOutlined`  | ⚙️        | System settings     |
| `SafetyOutlined`   | 🛡️        | Security management |

### 5.2 The Menu Management Page

```
┌─────────────────────────────────────────────────┐
│ 菜单管理                              [+ 新建菜单] │
├─────────────────────────────────────────────────┤
│ ▼ 📂 资源管理 (resource)                    [编辑] [删除] │
│   ├─ 👤 玩家管理 (player)                   [编辑] [删除] │
│   ├─ 📋 订单管理 (order)                    [编辑] [删除] │
│   └─ 📊 排行榜 (leaderboard)                [编辑] [删除] │
│ ▼ 🛠️ 运营工具 (operation)                   [编辑] [删除] │
│   ├─ 📢 公告管理                            [编辑] [删除] │
│   └─ 💬 客服管理                            [编辑] [删除] │
│ ▼ 📈 数据分析 (analytics)                   [编辑] [删除] │
│   ├─ 📊 数据概览                            [编辑] [删除] │
│   └─ 📉 报表中心                            [编辑] [删除] │
└─────────────────────────────────────────────────┘
```

### 5.3 The Menu Edit Modal

```
┌─────────────────────────────────────────┐
│ 编辑菜单                                 │
├─────────────────────────────────────────┤
│ 菜单标识: resource                       │
│                                         │
│ 名称（多语言）:                          │
│   中文: 资源管理                         │
│   English: Resource Management           │
│                                         │
│ 图标: [DatabaseOutlined ▼]              │
│ 排序: 1                                 │
│ 权限标识: resource:read                  │
│ 父菜单: (无)                            │
│                                         │
│        [取消]  [保存]                    │
└─────────────────────────────────────────┘
```

### 5.4 Choosing a Menu While Editing a Page

```
┌─────────────────────────────────────────┐
│ 编辑页面 resource--player                │
├─────────────────────────────────────────┤
│ ...                                     │
│ 所属菜单: [资源管理 ▼]                    │
│           ├─ 📂 资源管理                  │
│           ├─   👤 玩家管理                │
│           ├─   📋 订单管理                │
│           ├─ 🛠️ 运营工具                  │
│           └─ 📈 数据分析                  │
│ ...                                     │
└─────────────────────────────────────────┘
```

---

## 6. Migration Plan

### 6.1 Data Migration

```sql
-- 1. 从现有页面中提取唯一的分类，创建菜单
INSERT INTO menu_items (menu_key, labels, sort_order)
SELECT DISTINCT category_key, category_labels, 0
FROM pages
WHERE category_key IS NOT NULL
ON CONFLICT (menu_key) DO NOTHING;

-- 2. 关联页面到菜单
UPDATE pages p
SET menu_id = m.id
FROM menu_items m
WHERE p.category_key = m.menu_key;

-- 3. 删除旧字段
ALTER TABLE pages DROP COLUMN category_key;
ALTER TABLE pages DROP COLUMN category_labels;

-- 4. 验证迁移结果
SELECT menu_key, COUNT(*) as page_count
FROM pages p JOIN menu_items m ON p.menu_id = m.id
GROUP BY menu_key;
```

### 6.2 Code Migration

1. **Phase 1**: build the menu management API + UI + the database migration.
2. **Phase 2**: add the menu selector to the page editor.
3. **Phase 3**: switch the frontend sidebar to menu data.
4. **Phase 4**: delete all `category.labels`-related code.

---

## 7. Permission Design

### 7.1 Menu Permissions

| Permission    | Description   |
| ------------- | ------------- |
| `menu:create` | Create menus  |
| `menu:read`   | View menus    |
| `menu:update` | Edit menus    |
| `menu:delete` | Delete menus  |
| `menu:sort`   | Reorder menus |

### 7.2 Menu Item Permissions

A menu item's `permission` field governs access to all pages under that menu.

---

## 8. Open Questions

1. **Menu templates**: do we need preset menu templates (e.g. a standard game-operations menu)?
2. **Menu permission inheritance**: do child menus inherit the parent menu's permissions?
3. **Menu caching**: after menu data changes, how is the frontend notified to refresh?

## 7. Permission Design (Inheritance)

### 7.1 The Inheritance Rule

**Core principle**: a child menu inherits its parent menu's permissions; without the permission the whole branch is hidden.

```
资源管理 (resource:read)
  ├─ 玩家管理 (player:read)      ← 继承 resource:read
  ├─ 订单管理 (order:read)       ← 继承 resource:read
  └─ 排行榜 (leaderboard:read)   ← 继承 resource:read

运营工具 (operation:read)
  ├─ 公告管理 (announce:read)
  └─ 客服管理 (support:read)
```

### 7.2 The Permission-Check Logic

```go
// 伪代码：检查用户是否有权访问某个菜单
func canAccessMenu(user *User, menu *MenuItem) bool {
    // 1. 检查菜单本身是否可见
    if !menu.IsVisible {
        return false
    }

    // 2. 检查菜单权限（如果有）
    if menu.Permission != "" {
        if !user.HasPermission(menu.Permission) {
            return false
        }
    }

    // 3. 递归检查父菜单权限
    if menu.ParentID != nil {
        parent := GetMenuByID(*menu.ParentID)
        if !canAccessMenu(user, parent) {
            return false
        }
    }

    return true
}
```

### 7.3 The Frontend Filtering Logic

```tsx
// 前端：过滤用户无权限的菜单
function filterMenus(menus: MenuItem[], userPermissions: string[]): MenuItem[] {
  return menus
    .filter((menu) => {
      // 没有权限字段 = 所有人可见
      if (!menu.permission) return true;
      // 有权限字段 = 检查用户是否有该权限
      return userPermissions.includes(menu.permission);
    })
    .map((menu) => ({
      ...menu,
      children: filterMenus(menu.children || [], userPermissions),
    }))
    .filter((menu) => menu.children?.length > 0 || !menu.permission);
}
```

### 7.4 Database Design

```sql
CREATE TABLE menu_items (
    id              SERIAL PRIMARY KEY,
    parent_id       INTEGER REFERENCES menu_items(id) ON DELETE CASCADE,
    menu_key        VARCHAR(64) NOT NULL UNIQUE,
    labels          JSONB NOT NULL,
    icon            VARCHAR(64),
    sort_order      INTEGER DEFAULT 0,
    permission      VARCHAR(128),  -- 空 = 所有人可见
    is_visible      BOOLEAN DEFAULT true,
    created_at      TIMESTAMP DEFAULT NOW(),
    updated_at      TIMESTAMP DEFAULT NOW()
);

-- 权限继承查询（递归 CTE）
WITH RECURSIVE menu_tree AS (
    SELECT id, parent_id, menu_key, permission, is_visible
    FROM menu_items
    WHERE parent_id IS NULL

    UNION ALL

    SELECT m.id, m.parent_id, m.menu_key, m.permission, m.is_visible
    FROM menu_items m
    INNER JOIN menu_tree t ON m.parent_id = t.id
    WHERE t.is_visible = true
)
SELECT * FROM menu_tree WHERE is_visible = true;
```

### 7.5 The Inheritance Example

| Menu        | permission         | User A | User B  | User C  |
| ----------- | ------------------ | ------ | ------- | ------- |
| 资源管理    | `resource:read`    | ✅ has | ✅ has  | ❌ none |
| ├─ 玩家管理 | `player:read`      | ✅ has | ❌ none | -       |
| ├─ 订单管理 | `order:read`       | ✅ has | ✅ has  | -       |
| └─ 排行榜   | `leaderboard:read` | ✅ has | ✅ has  | -       |

**User A**: sees every menu.
**User B**: sees Resource management, Order management, Leaderboard (Player management is hidden).
**User C**: does not see Resource management (the whole branch is hidden).

### 7.6 Integration with the Role System

```go
// 角色-菜单权限映射
type RoleMenuPermission struct {
    RoleID     int    `json:"role_id"`
    MenuID     int    `json:"menu_id"`
    Permission string `json:"permission"` // "read" / "write" / "delete"
}

// 查询用户可访问的菜单
func GetAccessibleMenus(userID int) []MenuItem {
    user := GetUserByID(userID)
    roles := GetUserRoles(userID)

    // 收集用户所有权限
    permissions := make(map[string]bool)
    for _, role := range roles {
        perms := GetRolePermissions(role.ID)
        for _, p := range perms {
            permissions[p] = true
        }
    }

    // 过滤菜单
    menus := GetAllMenus()
    return filterMenus(menus, permissions)
}
```

---

## 9. Frontend Data-Loading Strategy

### 9.1 Fetch at Login (The Recommended Approach)

```
用户登录
  ↓
POST /api/v1/auth/login → 返回 token + 用户信息
  ↓
GET /api/v1/menus/accessible → 返回用户可访问的菜单树
  ↓
前端渲染侧边栏 + 路由守卫
```

### 9.2 API Design

**Get the accessible menus**

```
GET /api/v1/menus/accessible
Authorization: Bearer ***

响应：
{
  "menus": [
    {
      "id": 1,
      "menu_key": "resource",
      "labels": {"zh-CN": "资源管理", "en-US": "Resource"},
      "icon": "DatabaseOutlined",
      "children": [
        {"id": 2, "menu_key": "player", "labels": {"zh-CN": "玩家管理"}},
        {"id": 3, "menu_key": "order", "labels": {"zh-CN": "订单管理"}}
      ]
    }
  ]
}
```

### 9.3 Frontend Storage

```tsx
// 登录后存储菜单到状态管理
const login = async (credentials) => {
  const { token, user } = await authApi.login(credentials);
  const { menus } = await menuApi.getAccessible();

  // 存储到 store
  store.commit("SET_TOKEN", token);
  store.commit("SET_USER", user);
  store.commit("SET_MENUS", menus);

  // 渲染侧边栏
  router.addRoutes(generateRoutes(menus));
};
```

### 9.4 Handling Permission Changes

- **Default**: a permission change takes effect when the user logs in again.
- **Optional**: push permission-change events over WebSocket and have the frontend refresh its menus.
