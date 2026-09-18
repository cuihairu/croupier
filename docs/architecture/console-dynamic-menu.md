---
title: 运行控制台动态菜单
icon: menu
order: 9
category:
  - 系统架构
tag:
  - Console
  - 动态菜单
  - menu_items
---

# 运行控制台动态菜单

> **状态**：Current — 运行控制台菜单由菜单管理（`menu_items`）**唯一驱动**：菜单节点构成导航树，挂载到菜单且已发布的页面才出现在控制台。详细模型见 [Dashboard Resource/Page 模型](./dashboard-page-model.md)，操作向导（建菜单→发布→挂载→控制台出现）见 [运行控制台导航与页面挂载](/guide/concepts/console-navigation.md)。实现索引：菜单生成 `internal/api/console/`（`generateMenuFromMenuItems`，`GET /api/v1/console/menu`）、菜单树与权限过滤 `internal/api/menu/`（`AccessibleTree`）、菜单模型 `internal/model/menu.go`、挂载字段 `internal/model/page_spec.go`（`MenuID`）、前端路由 `web/config/routes.ts`、侧边菜单组装 `web/src/utils/consoleMenu.ts`（`buildMenuFromConsoleSpec`）、canonical 仲裁 `resolveConsolePageCanonicalPath`。

## 结论

运行控制台左侧菜单不是静态路由配置，不是函数目录的直接投影，也不是 PageSpec `category` 的聚合。

菜单来源只有一个：

```text
menu_items（菜单树结构） + page_specs.menu_id（挂载映射） + 最新已发布快照（页面内容）
    -> ConsoleMenuSpec
```

前端不得为动态分类修改 `web/src/locales/*/menu.ts`。菜单标题来自菜单系统的 `menu_items.labels`（菜单管理页维护），页面标题来自已发布 PageSpec 的 `title`。

## 挂载与过滤规则

页面出现在运行控制台由两个事实共同决定：

1. **挂载**：draft 表 `page_specs.menu_id` 指向同 scope 的菜单。挂载关系读 draft 表——改挂载即时生效，无需重发页面。
2. **已发布**：该 `pageKey` 存在 active 发布快照（`published_page_specs`）。draft-only 页面不上控制台。

组装规则：

- 菜单树结构（层级、排序、图标、可见性、权限）完全由 `menu_items` 决定：`menu.AccessibleTree` 按当前用户权限剪枝（不可见剪掉、菜单 permission 不持有剪掉、父级不可达剪整枝、孤儿提升为根）。
- 挂载页面作为菜单节点的 children 与子菜单混排，排序 `order → 本地化标题 → key`（组内同级比较）。
- 页面内容（title/icon/order）取最新发布快照；未挂载的已发布页面不出现在控制台导航，但直达 URL 仍可渲染。
- 无任何菜单时 `items` 为空，控制台首页展示「去菜单管理」引导空态。

## PageSpec.category 的现状

PageSpec 的 `category.key` 保留为页面侧元数据（页面工作台分组展示），**不再驱动运行控制台菜单**。其推导规则（显式声明 > `resourceKey` 前缀 > 主 binding `functionId` 前缀 > `pageKey` 前缀；详见 [ProComponents 页面生成与运行时](./ui-generation.md)）仍用于新页面草稿的默认值，但只影响工作台侧分组，与控制台导航无关。

## 多语言

动态菜单显示名分两个事实源：

```json
// menu_items（菜单管理页维护，菜单标题唯一事实）
{
  "menuKey": "support",
  "labels": {
    "zh-CN": "客服",
    "en-US": "Support"
  }
}
```

```json
// PublishedPageSpec（页面侧只携带自身标题）
{
  "title": {
    "zh-CN": "封禁玩家",
    "en-US": "Ban Player"
  }
}
```

规则：

- `menu_items.labels` 用于菜单标题；labels 为空时回落显示 `menuKey`。
- 已发布 PageSpec 的 `title` 用于页面菜单标题。
- 静态 locale 只用于固定系统菜单，例如“运行控制台”。
- 动态菜单项必须设置 `locale: false`。
- 页面 `title` 缺少系统默认语言时发布失败（菜单 labels 不在此校验范围）。
- 菜单 labels 在菜单管理页维护，与页面发布链解耦；改菜单名不需要重发页面。

## 路由

运行控制台保留固定参数路由承载动态菜单：

```text
/console/home
/console/:categoryKey
/console/:categoryKey/:pageKey
```

`/console/:categoryKey` 的 `categoryKey` 段是**挂载菜单 key**（`menuKey`），展示该菜单下的挂载页面（含子菜单组）。

`/console/:categoryKey/:pageKey` 渲染具体 PageSpec。canonical 仲裁以菜单树为唯一事实：`resolveConsolePageCanonicalPath` 在 `ConsoleMenuSpec` 树中递归查找 `pageKey` 的挂载路径；URL 与规范路径不一致时前端跳转到规范路径。页面未挂任何菜单（canonical 为空串）时不重定向，直达 URL 正常渲染。URL 不是 scope：页面、菜单和执行都按全局 `game_id + env` context 查询；同一个 `pageKey` 可以存在于不同 scope。

## 默认菜单种子（T-M10）

菜单读路径（`GET /menus`、`GET /menus/accessible`，含控制台导航组装共用的 `menu.AccessibleTree`）在 scope 首次访问时触发惰性种子：该 `(gameId, env)` 无任何菜单 → 导入 bootstrap 目录（`bootstrapData.baseDir`，与 `admins.json` 同目录）`default-menus.json` 中的顶级菜单骨架。实现：`internal/svc/menu_seeder.go`（`MenuSeeder.EnsureSeeded`）。

设计边界（有意行为）：

- 文件存在且非空即启用，缺失/空数组即禁用（零配置开关）；任一条目非法（menuKey 不合规、labels 全空）整体禁用并 Error 日志，不留半套骨架。
- 只在 scope 菜单表为空时导入，**永不更新/覆盖**用户数据；进程内每 scope 只尝试一次（成败均标记），重启后重新评估——用户删光全部菜单并重启会重新种一次，彻底禁用需删种子文件。
- 种子是系统行为，不写用户审计（与 AdminManager 默认管理员同语义）。
- 默认骨架（`configs/default-menus.json`）：玩家管理 / 运营 / 支付订单 / 公告 / 审计五个空组，仅是挂载位置骨架——三要素（菜单+已发布+挂载）不变，空组在控制台渲染为指向 `/console/<menuKey>` 的单链接。

## 边界

禁止：

- 维护硬编码分类表。
- 为动态分类新增静态 i18n key。
- 从前端页面里重复实现菜单推断。
- 从函数目录或 PageSpec `category` 聚合直接生成运行控制台菜单。
- 把未发布 PageSpec（draft-only）或函数注册草稿展示到运行控制台。
- 在 PageSpec、前端组件或静态字典中引入菜单文案（菜单名称只属于菜单系统）。

允许：

- PageSpec 保存时按推导规则生成 `category.key` 默认值（仅工作台侧分组用途）。
- Server 根据函数能力契约生成 PageSpec 建议；建议不是菜单事实源。
- 用户在页面工作台中覆盖页面标题、图标和排序；在菜单管理中维护树结构、labels、图标、排序、权限与可见性。

## 验收规则

- 新增菜单不需要改前端代码。
- 切换语言后，菜单标题来自 `menu_items.labels`，页面标题来自已发布 PageSpec `title`。
- 已发布但未挂载的页面不出现在控制台导航；挂载后无需重发页面即出现。
- 挂载后下线（unpublish）的页面从控制台消失；解除挂载同样消失（菜单组保留为空组）。
- 无权限菜单（`menu_items.permission` 不持有或不可见）不出现在控制台。
- 没有 `title` 默认语言时页面发布失败。
- 函数目录、Page Studio 草稿和运行控制台菜单之间不存在第二套菜单逻辑。
- 切换全局 game/env 后，菜单只显示新 scope 的菜单树与挂载页面。
