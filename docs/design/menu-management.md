# 菜单管理系统设计文档

> 状态：提案
> 作者：崔海涛
> 日期：2026-09-17
> 关联：Croupier Dashboard 页面分类一致性问题

---

## 1. 问题背景

### 1.1 当前设计

Croupier 的页面（PageSpec）通过 `category` 字段进行分类：

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

### 1.2 存在的问题

1. **分类标签分散**：每个页面都存储 `category.labels`，同分类下必须一致
2. **修改成本高**：改分类名称需要同步修改该分类下所有页面
3. **一致性校验严格**：编辑页面时如果 labels 不匹配会报错
4. **不支持菜单层级**：无法实现多级菜单嵌套
5. **权限控制粒度粗**：只能按分类控制，不能按菜单项控制

---

## 2. 设计目标

1. **菜单独立管理**：分类/菜单作为独立实体，与页面完全解耦
2. **页面选择菜单**：页面编辑时选择挂在哪个菜单下
3. **支持国际化**：菜单名称支持多语言
4. **支持层级结构**：支持多级菜单嵌套
5. **权限控制**：菜单级别控制可见性和操作权限
6. **不向后兼容**：直接废弃 `category.labels`，统一使用菜单系统

---

## 3. 数据模型

### 3.1 菜单表（menu_items）

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

### 3.2 页面表修改

```sql
-- 删除旧字段
ALTER TABLE pages DROP COLUMN IF EXISTS category_key;
ALTER TABLE pages DROP COLUMN IF EXISTS category_labels;

-- 添加新字段
ALTER TABLE pages ADD COLUMN menu_id INTEGER REFERENCES menu_items(id);
```

### 3.3 菜单示例数据

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

## 4. API 设计

### 4.1 菜单管理 API

| 方法   | 路径                     | 说明       |
| ------ | ------------------------ | ---------- |
| GET    | `/api/v1/menus`          | 获取菜单树 |
| POST   | `/api/v1/menus`          | 创建菜单   |
| PUT    | `/api/v1/menus/:id`      | 更新菜单   |
| DELETE | `/api/v1/menus/:id`      | 删除菜单   |
| PUT    | `/api/v1/menus/:id/sort` | 更新排序   |

### 4.2 页面关联 API

| 方法 | 路径                       | 说明                 |
| ---- | -------------------------- | -------------------- |
| PUT  | `/api/v1/pages/:key/menu`  | 设置页面所属菜单     |
| GET  | `/api/v1/menus/:key/pages` | 获取菜单下的页面列表 |

### 4.3 API 请求/响应示例

**创建菜单**

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

**响应**

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

## 5. 前端 UI 设计

### 5.1 图标方案

使用 `@ant-design/icons` 的 **SVG 组件**，与现有项目保持一致：

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

**图标映射表**（存储 icon name，前端动态渲染）：

| icon 字段          | 组件 | 用途     |
| ------------------ | ---- | -------- |
| `DatabaseOutlined` | 📂   | 资源管理 |
| `ToolOutlined`     | 🛠️   | 运营工具 |
| `BarChartOutlined` | 📊   | 数据分析 |
| `UserOutlined`     | 👤   | 玩家管理 |
| `FileTextOutlined` | 📋   | 订单管理 |
| `TrophyOutlined`   | 🏆   | 排行榜   |
| `SettingOutlined`  | ⚙️   | 系统设置 |
| `SafetyOutlined`   | 🛡️   | 安全管理 |

### 5.2 菜单管理页面

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

### 5.3 菜单编辑弹窗

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

### 5.4 页面编辑时选择菜单

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

## 6. 迁移方案

### 6.1 数据迁移

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

### 6.2 代码迁移

1. **Phase 1**：创建菜单管理 API + UI + 数据库迁移
2. **Phase 2**：页面编辑器增加菜单选择器
3. **Phase 3**：前端侧边栏改用菜单数据
4. **Phase 4**：删除所有 `category.labels` 相关代码

---

## 7. 权限设计

### 7.1 菜单权限

| 权限          | 说明         |
| ------------- | ------------ |
| `menu:create` | 创建菜单     |
| `menu:read`   | 查看菜单     |
| `menu:update` | 编辑菜单     |
| `menu:delete` | 删除菜单     |
| `menu:sort`   | 调整菜单排序 |

### 7.2 菜单项权限

菜单项的 `permission` 字段控制该菜单下所有页面的访问权限。

---

## 8. 待讨论

1. **菜单模板**：是否需要预设菜单模板（如标准游戏运营菜单）？
2. **菜单权限继承**：子菜单是否继承父菜单的权限？
3. **菜单缓存**：菜单数据变更后如何通知前端刷新？

## 7. 权限设计

### 7.1 权限继承规则

**核心原则**：子菜单继承父菜单的权限，无权限则整个分支不显示。

```
资源管理 (resource:read)
  ├─ 玩家管理 (player:read)      ← 继承 resource:read
  ├─ 订单管理 (order:read)       ← 继承 resource:read
  └─ 排行榜 (leaderboard:read)   ← 继承 resource:read

运营工具 (operation:read)
  ├─ 公告管理 (announce:read)
  └─ 客服管理 (support:read)
```

### 7.2 权限检查逻辑

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

### 7.3 前端过滤逻辑

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

### 7.4 数据库设计

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

### 7.5 权限继承示例

| 菜单        | permission         | 用户A权限 | 用户B权限 | 用户C权限 |
| ----------- | ------------------ | --------- | --------- | --------- |
| 资源管理    | `resource:read`    | ✅ 有     | ✅ 有     | ❌ 无     |
| ├─ 玩家管理 | `player:read`      | ✅ 有     | ❌ 无     | -         |
| ├─ 订单管理 | `order:read`       | ✅ 有     | ✅ 有     | -         |
| └─ 排行榜   | `leaderboard:read` | ✅ 有     | ✅ 有     | -         |

**用户A**：看到全部菜单
**用户B**：看到资源管理、订单管理、排行榜（玩家管理被隐藏）
**用户C**：看不到资源管理（整个分支被隐藏）

### 7.6 与角色系统的集成

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

## 9. 前端数据加载策略

### 9.1 登录时拉取（推荐方案）

```
用户登录
  ↓
POST /api/v1/auth/login → 返回 token + 用户信息
  ↓
GET /api/v1/menus/accessible → 返回用户可访问的菜单树
  ↓
前端渲染侧边栏 + 路由守卫
```

### 9.2 API 设计

**获取可访问菜单**

```
GET /api/v1/menus/accessible
Authorization: Bearer <token>

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

### 9.3 前端存储

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

### 9.4 权限变更处理

- **默认**：权限变更后用户重新登录生效
- **可选**： WebSocket 推送权限变更事件，前端刷新菜单
