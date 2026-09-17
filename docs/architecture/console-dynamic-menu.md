---
title: 运行控制台动态菜单
icon: menu
order: 9
category:
  - 系统架构
tag:
  - Console
  - 动态菜单
  - PublishedPageSpec
---

# 运行控制台动态菜单

> **状态**：Current — 运行控制台菜单消费已发布 PageSpec 与菜单系统（`menu_items`）的并集。详细模型见 [Dashboard Resource/Page 模型](./dashboard-page-model.md)。实现索引：菜单生成 `internal/api/console/`（`GET /api/v1/console/menu`）、菜单模型 `internal/model/menu.go`、前端路由 `web/config/routes.ts`、菜单组装 `web/src/utils/consoleMenu.ts`（`buildConsoleMenuFromAccessibleMenus`）、存量迁移 `scripts/migrate-categories-to-menus.sql`。

## 结论

运行控制台左侧菜单不是静态路由配置，也不是函数目录的直接投影。

菜单来源只有一个：

```text
PublishedPageSpec[] -> ConsoleMenuSpec
```

前端不得为动态分类修改 `web/src/locales/*/menu.ts`。动态分类名称来自菜单系统（`menu_items.labels`，菜单管理页维护），页面标题来自已发布 PageSpec 的 `title`；PageSpec 侧只提供 `category.key`（分组定位键），不携带分类文案（T-M8）。

## 分类规则

分类 key 的确定规则只有一套：

1. PageSpec 显式声明 `category.key` 时，使用该值。
2. 生成器创建的 ResourcePage 必须显式写入由 `resourceKey` 第一个 `.` 前缀推导的分类。
3. 生成器创建的独立 Operation/Task/Report 页面必须显式写入由主 binding 原始 `functionId` 第一个 `.` 前缀推导的分类。
4. 仅供手工创建且缺少上述来源的 PageSpec 使用 `pageKey` 的第一个 `.` 前缀；没有 `.` 时使用完整 key。

其中第 2/3 条的生成器默认值定义见 [ProComponents 页面生成与运行时](./ui-generation.md)（唯一出处）；本节只保留菜单侧的仲裁与兜底规则。

示例：

| 输入                                             | 最终分类    |
| ------------------------------------------------ | ----------- |
| `category.key = support`, `resourceKey = player` | `support`   |
| `resourceKey = player.ban`                       | `player`    |
| `resourceKey = mail.send`                        | `mail`      |
| `resourceKey = mail`                             | `mail`      |
| `functionId = analytics.retention`               | `analytics` |
| `pageKey = custom.page`（手工创建）              | `custom`    |

## 分类仲裁

同一 `category.key` 可被多个 PageSpec 使用；T-M8 起页面规格不再携带分类文案，仲裁规则随之简化：

1. 分类名称的唯一事实是菜单系统：`menu_items` 以 `(game_id, env, menu_key)` 唯一索引承载分类多语言名称（`labels`），不存在多页面 labels 冲突问题；存量漂移由迁移脚本（`scripts/migrate-categories-to-menus.sql`）归位——同 key 下 labels 不一致时取最近更新页面的 labels 落为菜单名。
2. 分类 order 取该分类下所有已发布页面 `category.order` 的最小值；分类内页面按各自 `order` 排序。
3. 分类下最后一个页面下线时，页面驱动菜单中的该分类随之消失；菜单系统中同名菜单仍保留（可挂纯菜单分组或空分类），由菜单管理页独立管理。

## 确定时机

函数注册不提供运行菜单分类，也不提供分类多语言显示名。Server 可以根据 `resourceKey`、主 binding 原始 `functionId` 和契约分析给 Page Studio 提供分类建议，但最终分类必须在 PageSpec 保存或发布时确定。`operation--mail.send` 等生成 pageKey 不是分类推导来源。

运行控制台加载菜单时不再推断分类。页面侧只读取已经发布并通过校验的 `category.key` 与页面 `title`；分类显示名由菜单系统的 `menu_items.labels` 覆盖（前端 `buildConsoleMenuFromAccessibleMenus` 按 `menuKey = category.key` 匹配挂载）。

## 多语言

动态菜单显示名分两个事实源（T-M8）：分类标题来自菜单系统，页面标题来自 PageSpec：

```json
// menu_items（菜单管理页维护，分类标题唯一事实）
{
  "menuKey": "support",
  "labels": {
    "zh-CN": "客服",
    "en-US": "Support"
  }
}
```

```json
// PublishedPageSpec（页面侧只携带分组键与自身标题）
{
  "category": { "key": "support" },
  "title": {
    "zh-CN": "封禁玩家",
    "en-US": "Ban Player"
  }
}
```

规则：

- `menu_items.labels` 用于分类菜单标题；菜单缺失或 labels 为空时前端回落显示 `menuKey`。
- `title` 用于页面菜单标题。
- 静态 locale 只用于固定系统菜单，例如“运行控制台”。
- 动态菜单项必须设置 `locale: false`。
- 页面 `title` 缺少系统默认语言时发布失败（分类名称不在此校验范围——它不再随页面发布）。
- 菜单 labels 在菜单管理页维护，与页面发布链解耦；改分类名不需要重发页面。

## 路由

运行控制台保留固定参数路由承载动态菜单：

```text
/console/home
/console/:categoryKey
/console/:categoryKey/:pageKey
```

`/console/:categoryKey` 展示该分类下的已发布页面。

`/console/:categoryKey/:pageKey` 渲染具体 PageSpec。如果地址中的分类和 PageSpec 发布分类不一致，前端应跳转到规范路径。URL 不是 scope：页面、菜单和执行都按全局 `game_id + env` context 查询；同一个 `pageKey` 可以存在于不同 scope。

## 边界

禁止：

- 维护硬编码分类表。
- 为动态分类新增静态 i18n key。
- 从前端页面里重复实现分类推断。
- 从函数目录直接生成运行控制台菜单。
- 把未发布 PageSpec 或函数注册草稿展示到运行控制台。
- 在 PageSpec、前端组件或静态字典中重新引入分类文案（分类名称只属于菜单系统）。

允许：

- PageSpec 保存时根据规则生成分类 key。
- Server 根据函数能力契约生成 PageSpec 建议；建议不是菜单事实源。
- 用户在 Page Studio 中覆盖分类、标题、图标和排序。

## 验收规则

- 新增分类不需要改前端代码。
- 切换语言后，动态分类标题来自 `menu_items.labels`，页面标题来自 PageSpec `title`。
- 没有 PageSpec 发布时，运行控制台不展示对应页面菜单；空菜单（无子菜单无页面）可保留为分组入口，点击落到分类路由空态页。
- 没有 `title` 默认语言时页面发布失败。
- 函数目录、Page Studio 草稿和运行控制台菜单之间不存在第二套分类逻辑。
- 存量迁移后 `unmapped_categorized_pages` 必须为 0（迁移脚本自带校验输出）。
- 切换全局 game/env 后，菜单只显示新 scope 的 active PublishedPageSpec 与菜单项。
