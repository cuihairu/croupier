---
title: 设计态 / 运行态 导航隔离方案设计
icon: columns-2
order: 20
category:
  - 系统架构
tag:
  - 导航
  - 设计提案
---

# 设计态 / 运行态 导航隔离方案设计

> **状态**：已评审 — 待决策（评审推荐方案 C）
>
> **评审结论摘要**：原提案（方案 B：双 Layout 物理隔离）方向成立，但存在 5 个阻塞级技术问题，
> 且其核心痛点（菜单冗长）可用成本更低的方案 C（单 Layout + 菜单分组）解决。
> 建议先落地方案 C，方案 B 作为未来演进方向保留（见附录 A，已附勘误）。

## 1. 背景与目标

Croupier 的后台管理系统包含两类功能：

- **设计态（Design）**：函数注册、页面设计、资源编排、权限配置等——面向管理员/技术运营
- **运行态（Runtime）**：控制台操作、数据分析、运维监控、客服支持——面向日常运营人员

当前两套功能混在同一个菜单体系中。目标是为两类人群提供清晰的导航边界。

**关键事实（影响方案选择）**：Umi access 插件已按权限过滤菜单（`web/src/access.ts`），
只有 `console:read` 的运营人员本来就看不到设计态菜单。因此"菜单冗长"的真实受害者主要是
**admin/全权限用户**——方案设计应以这个事实为前提。

## 2. 候选方案

### 2.1 方案 B：双 Layout 物理隔离（原提案）

新增 `/design/*` 与 `/runtime/*` 两套路由前缀，各自挂独立 Layout（独立侧边栏/头部），
Header 放置模式切换器，根路由按角色自动跳转。详细设计见附录 A。

### 2.2 方案 C：单 Layout + 菜单分组（评审新增）

URL、路由、Layout 全部保持不变，仅在现有 `menu.request` 返回前，把顶层菜单项按映射表
包进「设计」「运行」两个分组节点（ProLayout 中无 path 的 children 节点渲染为不可导航的子菜单标题）。
可选叠加：Welcome 页按角色调整默认引导、Header 加纯前端视图过滤开关。

## 3. 方案对比

| 维度              | 现状   | 方案 B（双 Layout）      | 方案 C（菜单分组） |
| ----------------- | ------ | ------------------------ | ------------------ |
| 解决核心痛点      | —      | 彻底                     | 解决               |
| URL 变更          | 无     | 全量变更 + redirect 层   | 无                 |
| 后端改动          | 无     | ConsoleMenuSpec 路径前缀 | 无                 |
| e2e / 文档 / 书签 | 无影响 | 全部需更新               | 无影响             |
| Layout 维护       | 1 份   | 2 份 ProLayout chrome    | 1 份               |
| 权限语义          | 不变   | 需新增 Layout 级派生权限 | 不变               |
| 工作量            | —      | 周级（含修复阻塞问题）   | 天级               |
| 回滚成本          | —      | 中（URL 已变更）         | 低（纯前端菜单层） |

## 4. 方案 B 评审发现

### 4.1 阻塞问题（实施前必须解决）

**B1. `layout: 'DesignLayout'` 是无效 API。**
@umijs/max 路由配置的 `layout` 属性只支持 `false`（禁用布局），不支持指定自定义 Layout。
当前全局启用了 plugin-layout（`web/config/config.ts:80`），正确做法是分支顶层 `layout: false`

- 自定义 wrapper 组件（渲染自己的 ProLayout + `<Outlet/>`）。
  更重要的是：`web/src/app.tsx:108` 的整个 `RunTimeLayoutConfig`（menu.request 动态菜单、
  avatarProps、actionsRender、childrenRender、SettingDrawer）如何拆分/复用到两个 Layout，
  原文档完全未覆盖——这才是方案 B 最大的工程量。

**B2. 示例代码 API 错误。**

- `useHistory()` 是 react-router v5 API，Umi 4 / @umijs/max 中不存在，
  应用 `import { history } from '@umijs/max'` 或 `useNavigate()`
- `const { currentUser } = useModel('@@initialState')` 错误，该 model 返回
  `{ initialState, setInitialState, loading, refresh }`，应为 `initialState?.currentUser`

**B3. 现有功能在新路由体系中丢失。**

- `/admin/account/center`（Profile：改密码、TOTP 绑定、安全设置）和
  `/admin/account/messages` 在新体系中无家可归，AvatarDropdown 依赖这些页面
- `/ops/notifications`（事件通知）在路由表和第 6 节配置中都丢失
- `/` 当前是 Welcome 落地页（`web/src/pages/Welcome.tsx`），被替换为自动跳转
  是产品决策还是疏漏，需明确
- 现有 `/account/*` redirect 链未提及如何处理

**B4. 文档内部自相矛盾。**

- 2.2 节称函数目录为 `/design/functions`，第 6 节实际配置为
  `/design/functions` → redirect → `/design/functions/catalog`
- 2.2 节 extensions 为单层路径，第 6 节为 store/installations/agent-sync 三层嵌套
- 4.4 节提议后端 `ConsoleMenuSpec` 扩展 `runtimeModules`（后端驱动运行态菜单），
  第 9 节实现却是前端静态路由合并（前端驱动）——两种方案并存，没有决策

**B5. ConsoleMenuSpec 路径前缀耦合（原文档未意识到的坑）。**
后端 `consoleCategoryPath()` 硬编码 `/console/` 前缀
（`internal/api/console/service.go:926`）。迁移到 `/runtime/console` 后，后端下发的
菜单 path 全部指向旧路由。必须三选一：后端改前缀（破坏旧前端）/ 前端消费时重写前缀 /
后端下发相对路径。方案 B 实施前必须先决策此项。

### 4.2 中等问题

**B6. "权限模型不变"的论断不准确。**

- `canConsoleRead = hasAny('console:read', 'pages:read', 'function:invoke')`
  （`web/src/access.ts:35`）——设计态的 `pages:read` 用户天然能看到运行态控制台，
  "物理隔离"在权限层并不干净；ModeSwitcher 对谁显示需要明确规则
- DesignAccess/RuntimeAccess 守卫的检查逻辑是占位注释，实际需要新增派生权限
  （如 `canDesignAccess = canFunctionsAndPagesRead || canSystemConfigRead || canPermissionManage`），
  与"不需要新增权限点"的说法矛盾

**B7. 旧路径重定向方案不完整且技术细节有误。**

- 带参数路径（`/system/functions/:id`、`/console/:categoryKey/:pageKey`）无法用
  Umi 静态 `redirect` 配置，需通配符路由 + 组件内 `history.replace`
- "301 重定向"说法不准确——SPA 是客户端跳转，不是 HTTP 301；若后端通知/邮件中
  有硬编码旧前端 URL，需排查
- 现有二级 legacy redirect（`/operations/extensions/*` 等）需跟着改

**B8. e2e 影响未提及。**
8+ 个 e2e spec 硬编码旧路径（`web/e2e/helpers/index.ts:48`、`page-studio.spec.ts`、
`openapi-source.spec.ts`、`resource-catalog.spec.ts` 等），实施步骤中没有更新计划。

### 4.3 小问题

- 根路由跳转应用 `history.replace` 而非 `push`（否则浏览器回退会死循环）
- 新增菜单 name 需同步补 locale 文件条目（`web/src/locales/*/menu.ts`）
- DesignLayout 无消息入口（MessagesBell）、RuntimeLayout 有，不对称设计未解释
- `pages/Design/` 与"保持现有目录结构"两处表述犹豫，应决断（建议保持现有）
- 缺回滚策略 / feature flag / 验收标准

## 5. 方案 C 设计

### 5.1 菜单分组规则

顶层菜单项按 path 精确匹配分入两组，未匹配的项原样保留在分组之后：

| 分组 | 包含的顶层菜单                                                                                 |
| ---- | ---------------------------------------------------------------------------------------------- |
| 设计 | `/system`（系统管理）、`/system/functions`（函数与页面）、`/admin`（权限与账号）               |
| 运行 | `/console`（运行控制台）、`/analytics`（分析中心）、`/ops`（运维中心）、`/support`（客服系统） |

分组节点为无 path 的 MenuDataItem（`locale: false` + 本地化名称），ProLayout 将其渲染为
不可导航的子菜单标题。access 过滤先于分组执行，无权限的分组自然为空——渲染前需剔除空分组。

### 5.2 运行态菜单动态化

方案 C 的菜单分组解决了"菜单冗长"问题，但运行态菜单仍然是静态的（`routes.ts` 硬编码）。
结合设计态的 PageStudio 能力，运行态菜单应完全由后端驱动：

**当前状态：**

```
routes.ts 硬编码 → ProLayout 渲染
  /console（动态子菜单来自 ConsoleMenuSpec）
  /analytics（静态）
  /ops（静态）
  /support（静态）
```

**目标状态：**

```
后端 API 返回完整运行态菜单 → ProLayout 渲染
  /console（业务页面，PageStudio 创建后自动入菜单）
  /analytics（系统模块，后端配置注册）
  /ops（系统模块，后端配置注册）
  /support（系统模块，后端配置注册）
```

**两类菜单来源：**

| 来源     | 说明                                 | 存储位置                                   |
| -------- | ------------------------------------ | ------------------------------------------ |
| 系统模块 | analytics、ops、support 等内置模块   | `configs/server.yaml` 的 `console.modules` |
| 业务页面 | 通过 PageStudio 创建，发布后自动生成 | `page_specs` / `published_page_specs` 表   |

**后端扩展 `ConsoleMenuSpec`：**

后端 `GET /api/v1/console/menu` 已经返回控制台的动态菜单。扩展为返回完整的运行态菜单：

```go
// 后端 Menu() 方法组装逻辑
func (s *Service) Menu(ctx, req) {
    var items []MenuItem

    // 1. 系统模块（从 configs/server.yaml 读取，按用户权限过滤）
    items = append(items, s.buildSystemModules(ctx, lang)...)

    // 2. 业务页面（从 PublishedPageSpec 读取，已有逻辑）
    items = append(items, s.buildBusinessPages(ctx, gameID, env, lang)...)
}
```

系统模块配置示例：

```yaml
# configs/server.yaml
console:
  modules:
    - key: analytics
      title: { zh-CN: 数据分析, en-US: Analytics }
      icon: areaChart
      permission: analytics:read
      children:
        - key: realtime
          path: /analytics/realtime
          title: { zh-CN: 实时, en-US: Realtime }
        - key: overview
          path: /analytics/overview
          title: { zh-CN: 概览, en-US: Overview }
    - key: ops
      title: { zh-CN: 运维, en-US: Ops }
      icon: tool
      permission: ops:read
      children:
        - key: nodes
          path: /ops/nodes
          title: { zh-CN: 节点, en-US: Nodes }
```

**前端改造：**

`menu.request` 中，运行态菜单完全从后端 API 获取，不再依赖 `routes.ts` 中的静态路由：

```typescript
// app.tsx — menu.request
request: async (params, defaultMenuData) => {
  if (!params.authed) return defaultMenuData;

  const consoleMenu = await getConsoleMenu(locale);

  // 设计态菜单：保持静态（从 defaultMenuData 过滤）
  // 运行态菜单：完全从 consoleMenu 渲染
  const designMenu = defaultMenuData.filter((item) => isDesignRoute(item.path));
  const runtimeMenu = buildRuntimeMenuFromSpec(consoleMenu, locale);

  // 分组
  return groupMenuByMode([...designMenu, ...runtimeMenu], locale);
};
```

**业务页面自动入菜单流程：**

```
设计态 PageStudio 创建页面
  → 配置分类、标题、权限
  → 发布 → 写入 published_page_specs 表
  → 运行态 ConsoleMenu API 读取
  → 菜单自动出现新项
  → 无需修改 routes.ts 或任何前端代码
```

### 5.3 实现点（如决策采用）

1. 新增 `web/src/utils/menuGrouping.ts`：`groupMenuByMode(menu: MenuDataItem[], locale: string): MenuDataItem[]`
2. `web/src/app.tsx` 的 `menu.request` 中，在 `buildMenuFromConsoleSpec` 之后套 `groupMenuByMode`
3. 后端 `ConsoleMenuSpec` 扩展支持系统模块配置（`console.modules` in server.yaml）
4. 后端 `Menu()` 方法合并系统模块 + 业务页面
5. 前端运行态菜单改为完全从后端 API 获取
6. 分组名称走 locale 文件（`menu.ModeDesign` / `menu.ModeRuntime`）
7. 单测参照 `web/tests/consoleMenu.test.ts` 风格新增 `web/tests/menuGrouping.test.ts`

### 5.4 明确不做的事

- 不改任何 URL、不动 routes.ts 的设计态部分
- Welcome 落地页保持不变；如需"按角色自动跳转首页"，是独立的产品决策，另行评审
- 不加 Header 视图过滤开关（等分组上线后按实际反馈再评估）
- 系统模块的页面组件保持不变（analytics、ops 的 React 组件不动）

## 6. 推荐结论与决策点

**评审推荐：先实施方案 C，方案 B 暂缓。**

理由：

1. 核心痛点（全权限用户菜单冗长）用方案 C 即可解决，成本为天级且零外部影响
2. 方案 B 的 5 个阻塞问题使其真实成本远超原文档预估
3. 方案 C 不阻塞方案 B——菜单分组的映射表与未来双 Layout 的分组语义一致，是平滑过渡

**需要决策者确认的问题：**

1. 是否接受"URL 扁平"作为长期形态？（方案 C 的隔离感弱于 B）
2. 运行态未来是否有独立产品形态规划（大屏、独立域名、移动端、独立发布节奏）？
   ——若有，方案 B 的价值显著提升，值得投入
3. Welcome 页是否保留为落地页，还是改为按角色自动跳转？

**方案 B 的演进触发条件**（满足其一即重启评估）：

- 运行态需要独立的头部布局/品牌/发布节奏
- 出现第三类功能簇，菜单分组无法容纳
- 需要按人群做完全不同的大数据屏/移动形态

---

## 附录 A：方案 B 原始详细设计（保留供未来参考，已附勘误）

> 以下为原提案内容。标注 ⚠️ 的条目为评审发现的勘误，实施时必须修正。

### A.1 路由结构 ⚠️

```
/design/*   → 设计态（DesignLayout）
/runtime/*  → 运行态（RuntimeLayout）
/login      → 共享登录页（无 Layout）
/403        → 共享无权限页
/404        → 共享 404
/           → 根据用户角色自动跳转到 /design 或 /runtime
```

⚠️ 勘误：需补充 `/admin/account/*`（Profile/消息）的归属；`/ops/notifications` 不可遗漏；
`/` 替换 Welcome 落地页需产品确认。

设计态路由：

```yaml
/design:
  /design/functions            # 函数目录 ⚠️ 实际应为 /design/functions/catalog
  /design/functions/:id        # 函数详情
  /design/functions/resources  # 资源/操作
  /design/functions/pages      # 页面设计（PageStudio）
  /design/functions/proposals  # 页面提案
  /design/functions/assignments # 分配管理
  /design/functions/instances  # 函数实例
  /design/functions/warnings   # 函数告警
  /design/functions/openapi-sources  # OpenAPI 来源
  /design/functions/resource-catalog # 资源目录
  /design/system/environments  # 游戏环境管理
  /design/system/extensions    # 扩展管理 ⚠️ 实际为 store/installations/agent-sync 三层
  /design/admin/users          # 用户管理
  /design/admin/roles          # 角色管理
  /design/admin/config         # 权限配置
  /design/admin/login-logs     # 登录日志
```

运行态路由：

```yaml
/runtime:
  /runtime/console                     # 控制台首页
  /runtime/console/:categoryKey        # 控制台分类
  /runtime/console/:categoryKey/:pageKey # 控制台页面 ⚠️ 需解决后端 /console/ 前缀耦合（B5）
  /runtime/analytics/realtime          # 实时分析
  /runtime/analytics/overview          # 概览
  /runtime/analytics/retention         # 留存
  /runtime/analytics/behavior          # 行为
  /runtime/analytics/payments          # 支付
  /runtime/analytics/levels            # 关卡
  /runtime/ops/nodes                   # 节点管理
  /runtime/ops/jobs                    # 任务管理
  /runtime/ops/alerts                  # 告警
  /runtime/ops/rate-limits             # 限流
  /runtime/ops/backups                 # 备份
  /runtime/ops/certificates            # 证书
  /runtime/ops/notifications           # 事件通知 ⚠️ 原文档遗漏
  /runtime/ops/analytics-filters       # 分析过滤
  /runtime/ops/terms                   # 术语
  /runtime/support/tickets             # 工单
  /runtime/support/faq                 # FAQ
  /runtime/support/bugs                # Bug
  /runtime/support/feedback            # 反馈
```

### A.2 Layout 结构

DesignLayout：

```
┌──────────────────────────────────────────────────────┐
│  Header: Logo │ [运行态 ▼] 切换 │ GameSelector │ 用户 │
├──────────┬───────────────────────────────────────────┤
│  侧边栏   │              内容区                       │
│  函数管理  │   PageContainer                           │
│  系统配置  │     └── 页面组件                           │
│  权限管理  │                                           │
├──────────┴───────────────────────────────────────────┤
│  Footer                                              │
└──────────────────────────────────────────────────────┘
```

RuntimeLayout：

```
┌──────────────────────────────────────────────────────┐
│  Header: Logo │ [设计态 ▼] 切换 │ GameSelector │ 消息 │ 用户 │
├──────────┬───────────────────────────────────────────┤
│  侧边栏   │              内容区                       │
│  控制台   │   PageContainer                           │
│  数据分析  │     └── 页面组件                           │
│  运维     │                                           │
│  客服     │                                           │
├──────────┴───────────────────────────────────────────┤
│  Footer                                              │
└──────────────────────────────────────────────────────┘
```

⚠️ 勘误：两个 Layout 的 chrome（header actions、avatar、footer、menu.request、childrenRender）
如何复用 `web/src/app.tsx` 现有 `RunTimeLayoutConfig`，必须给出明确方案（建议抽取共享 hook/配置工厂）。

### A.3 核心组件 ⚠️

Layout 切换（⚠️ `useHistory` 为无效 API，应使用 `history` 或 `useNavigate`）：

```tsx
// components/ModeSwitcher.tsx
import { history, useLocation } from "@umijs/max";

const ModeSwitcher: React.FC = () => {
  const location = useLocation();
  const isDesign = location.pathname.startsWith("/design");

  return (
    <Segmented
      options={[
        { label: "设计态", value: "design", icon: <ToolOutlined /> },
        { label: "运行态", value: "runtime", icon: <AppstoreOutlined /> },
      ]}
      value={isDesign ? "design" : "runtime"}
      onChange={(val) =>
        history.push(val === "design" ? "/design" : "/runtime")
      }
    />
  );
};
```

根路由自动跳转（⚠️ 修正 useModel 用法；⚠️ 应使用 replace 避免回退死循环）：

```tsx
// pages/Home/index.tsx
const HomePage: React.FC = () => {
  const { initialState, loading } = useModel("@@initialState");

  useEffect(() => {
    if (loading) return;
    const roles = initialState?.currentUser?.roles || [];
    const isAdmin = roles.some((r) =>
      ["admin", "super_admin"].includes(String(r).toLowerCase()),
    );
    history.replace(isAdmin ? "/design" : "/runtime");
  }, [loading]);

  return <Spin />;
};
```

权限守卫（⚠️ 需先定义具体派生权限，见 B6）：

```tsx
// wrappers/DesignAccess.tsx
const DesignAccess: React.FC<React.PropsWithChildren> = ({ children }) => {
  const { initialState } = useModel("@@initialState");
  const access = useAccess(); // 需在 access.ts 新增 canDesignAccess 派生权限
  if (!access.canDesignAccess) return <Navigate to="/403" />;
  return <>{children}</>;
};
```

### A.4 运行态动态菜单

⚠️ 勘误：4.4（后端 `ConsoleMenuSpec` 扩展 `runtimeModules`）与第 9 节（前端静态路由合并）
为两套竞争方案，实施前必须决策其一。推荐：短期复用现有 `buildMenuFromConsoleSpec`
（`web/src/utils/consoleMenu.ts:56`）做前端合并；后端驱动作为独立演进另行评审。

同时必须先解决后端 `/console/` 路径前缀耦合（B5）：
三选一——后端改前缀（需前后端同步发布）/ 前端消费时重写前缀（过渡期推荐）/
后端下发相对路径（长期最优，需改 spec 契约）。

### A.5 文件结构

```
web/src/
├── layouts/
│   ├── DesignLayout/
│   │   ├── index.tsx          # 设计态 Layout
│   │   └── index.less
│   └── RuntimeLayout/
│       ├── index.tsx          # 运行态 Layout
│       └── index.less
├── components/
│   └── ModeSwitcher/
│       └── index.tsx          # 设计态/运行态切换组件
├── wrappers/
│   ├── DesignAccess.tsx       # 设计态权限守卫
│   └── RuntimeAccess.tsx      # 运行态权限守卫
├── pages/
│   ├── Home/
│   │   └── index.tsx          # 根路由自动跳转
│   └── ...                    # 保持现有目录结构（组件引用不变）
└── config/
    ├── routes.ts              # 路由配置（重构）
    └── defaultSettings.ts
```

### A.6 路由配置要点 ⚠️

⚠️ 勘误：原文档 `layout: 'DesignLayout'` 写法无效。正确结构示例：

```typescript
// config/routes.ts
export default [
  {
    path: "/user",
    layout: false,
    routes: [{ path: "/user/login", component: "./User/Login" }],
  },
  { path: "/", component: "./Home" },

  // 设计态：layout:false 禁用全局 plugin-layout，由自定义 Layout 组件接管
  {
    path: "/design",
    layout: false,
    component: "@/layouts/DesignLayout",
    wrappers: ["@/wrappers/DesignAccess"],
    routes: [
      { path: "/design", redirect: "/design/functions/catalog" },
      {
        path: "/design/functions/catalog",
        name: "FunctionCatalog",
        access: "canFunctionsRead",
        component: "./Functions/Directory",
      },
      // ... 其余设计态路由（access key 与现状一致）
    ],
  },

  // 运行态同理
  {
    path: "/runtime",
    layout: false,
    component: "@/layouts/RuntimeLayout",
    wrappers: ["@/wrappers/RuntimeAccess"],
    routes: [
      { path: "/runtime", redirect: "/runtime/console" },
      // ... 其余运行态路由
    ],
  },

  { path: "/403", layout: false, component: "./403" },
  { path: "*", layout: false, component: "./404" },
];
```

### A.7 权限模型 ⚠️

⚠️ 勘误：需新增两个 Layout 级派生权限（在 `web/src/access.ts`）：

```typescript
canDesignAccess: canFunctionsAndPagesRead || canSystemConfigRead || canPermissionManage,
canRuntimeAccess: canConsoleRead || canAnalyticsRead || canOpsRead || canSupportRead,
```

并明确 ModeSwitcher 的显示规则：仅在 `canDesignAccess && canRuntimeAccess` 时显示。
同时注意 `canConsoleRead` 包含 `pages:read`（B6），设计师会看到运行态入口，
是否收紧该权限是独立决策。

访问控制规则：

- 管理员：两个 Layout 都可访问，默认进设计态
- 运营人员：仅运行态可见，进 `/runtime`
- 无权限用户：跳转 `/403`

### A.8 向后兼容 ⚠️

⚠️ 勘误：静态 redirect 表无法覆盖带参数路径。实施方案：

```typescript
// 1. 静态顶层 redirect（Umi redirect 配置即可）
const legacyRedirects = [
  { from: "/system/functions", to: "/design/functions" },
  { from: "/console", to: "/runtime/console" },
  { from: "/analytics", to: "/runtime/analytics" },
  { from: "/ops", to: "/runtime/ops" },
  { from: "/support", to: "/runtime/support" },
  { from: "/admin", to: "/design/admin" },
];

// 2. 带参数路径：通配符路由 + 组件内 history.replace（保留参数与 query）
// { path: '/console/*', component: './LegacyRedirect', layout: false }
```

并需排查：后端通知/邮件中的硬编码前端 URL、e2e spec 路径（B8）、
现有二级 legacy redirect（`/operations/extensions/*`）。

### A.9 实施步骤（修正版）

1. 抽取 `app.tsx` 的 ProLayout 配置为共享工厂（两个 Layout 复用）
2. 创建 `DesignLayout`、`RuntimeLayout`、`ModeSwitcher`、权限守卫
3. `access.ts` 新增 `canDesignAccess` / `canRuntimeAccess`
4. 决策并解决 ConsoleMenuSpec 路径前缀（B5）
5. 重构 `routes.ts`，旧路由迁移 + legacy redirect（含参数路径方案）
6. 补齐 Profile/消息、ops/notifications 等遗漏路由（B3）
7. locale 文件补新菜单 key
8. e2e spec 路径全量更新（B8）
9. 验收：全量 e2e + 手工回归两类人群的核心路径
10. 向后兼容期结束后移除旧路由
