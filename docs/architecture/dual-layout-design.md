# 设计态 / 运行态 双 Layout 架构设计

> **状态**：Proposed — 待评审

## 1. 背景与目标

Croupier 的后台管理系统包含两类功能：

- **设计态（Design）**：函数注册、页面设计、资源编排、权限配置等——面向管理员/技术运营
- **运行态（Runtime）**：控制台操作、数据分析、运维监控、客服支持——面向日常运营人员

当前两套功能混在同一个菜单体系中，用户需要在大量菜单项中找到自己需要的操作。目标是将两者物理隔离为两套 Layout，各自有独立的菜单和导航结构。

## 2. 路由结构

### 2.1 路径规范

```
/design/*   → 设计态（DesignLayout）
/runtime/*  → 运行态（RuntimeLayout）
/login      → 共享登录页（无 Layout）
/403        → 共享无权限页
/404        → 共享 404
/           → 根据用户角色自动跳转到 /design 或 /runtime
```

### 2.2 设计态路由

```yaml
/design:
  /design/functions            # 函数目录
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
  /design/system/extensions    # 扩展管理
  /design/admin/users          # 用户管理
  /design/admin/roles          # 角色管理
  /design/admin/config         # 权限配置
  /design/admin/login-logs     # 登录日志
```

### 2.3 运行态路由

```yaml
/runtime:
  /runtime/console                     # 控制台首页
  /runtime/console/:categoryKey        # 控制台分类
  /runtime/console/:categoryKey/:pageKey # 控制台页面
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
  /runtime/ops/analytics-filters       # 分析过滤
  /runtime/ops/terms                   # 术语
  /runtime/support/tickets             # 工单
  /runtime/support/faq                 # FAQ
  /runtime/support/bugs                # Bug
  /runtime/support/feedback            # 反馈
```

## 3. Layout 结构

### 3.1 DesignLayout

```
┌──────────────────────────────────────────────────────┐
│  Header: Logo │ [运行态 ▼] 切换 │ GameSelector │ 用户 │
├──────────┬───────────────────────────────────────────┤
│          │                                           │
│  侧边栏   │              内容区                       │
│          │                                           │
│  函数管理  │   PageContainer                           │
│   ├ 目录  │     └── 页面组件                           │
│   ├ 资源  │                                           │
│   ├ 页面  │                                           │
│   ├ 分配  │                                           │
│   └ ...  │                                           │
│          │                                           │
│  系统配置  │                                           │
│   ├ 环境  │                                           │
│   └ 扩展  │                                           │
│          │                                           │
│  权限管理  │                                           │
│   ├ 用户  │                                           │
│   ├ 角色  │                                           │
│   └ 配置  │                                           │
│          │                                           │
├──────────┴───────────────────────────────────────────┤
│  Footer                                              │
└──────────────────────────────────────────────────────┘
```

### 3.2 RuntimeLayout

```
┌──────────────────────────────────────────────────────┐
│  Header: Logo │ [设计态 ▼] 切换 │ GameSelector │ 消息 │ 用户 │
├──────────┬───────────────────────────────────────────┤
│          │                                           │
│  侧边栏   │              内容区                       │
│          │                                           │
│  控制台   │   PageContainer                           │
│   ├ 首页  │     └── 页面组件                           │
│   ├ 动态  │                                           │
│   │ 分类  │                                           │
│   └ ...  │                                           │
│          │                                           │
│  数据分析  │                                           │
│   ├ 实时  │                                           │
│   ├ 概览  │                                           │
│   └ ...  │                                           │
│          │                                           │
│  运维     │                                           │
│   ├ 节点  │                                           │
│   ├ 任务  │                                           │
│   └ ...  │                                           │
│          │                                           │
│  客服     │                                           │
│   ├ 工单  │                                           │
│   └ ...  │                                           │
│          │                                           │
├──────────┴───────────────────────────────────────────┤
│  Footer                                              │
└──────────────────────────────────────────────────────┘
```

## 4. 核心组件

### 4.1 Layout 切换

Header 中放置一个 Segmented 或 Dropdown 组件，切换设计态/运行态：

```tsx
// components/ModeSwitcher.tsx
const ModeSwitcher: React.FC = () => {
  const { location, push } = useHistory();
  const isDesign = location.pathname.startsWith('/design');

  return (
    <Segmented
      options={[
        { label: '设计态', value: 'design', icon: <ToolOutlined /> },
        { label: '运行态', value: 'runtime', icon: <AppstoreOutlined /> },
      ]}
      value={isDesign ? 'design' : 'runtime'}
      onChange={(val) => push(val === 'design' ? '/design' : '/runtime')}
    />
  );
};
```

### 4.2 根路由自动跳转

```tsx
// pages/Home/index.tsx
const HomePage: React.FC = () => {
  const { currentUser } = useModel('@@initialState');
  const { push } = useHistory();

  useEffect(() => {
    // 管理员默认进设计态，普通用户默认进运行态
    const isAdmin = currentUser?.roles?.some(r =>
      ['admin', 'super_admin'].includes(r.toLowerCase())
    );
    push(isAdmin ? '/design' : '/runtime');
  }, []);

  return <Spin />;
};
```

### 4.3 权限守卫

```tsx
// wrappers/DesignAccess.tsx
const DesignAccess: React.FC = ({ children }) => {
  const { currentUser } = useModel('@@initialState');
  const hasDesignAccess = /* 检查用户是否有设计态权限 */;

  if (!hasDesignAccess) return <Redirect to="/403" />;
  return <>{children}</>;
};

// wrappers/RuntimeAccess.tsx（同理）
```

### 4.4 运行态动态菜单

运行态菜单来自后端 `ConsoleMenuSpec`，但需要扩展支持多模块：

```typescript
// 后端 ConsoleMenuSpec 扩展
type ConsoleMenuSpec = {
  items: ConsoleMenuCategory[];  // 控制台页面（已有）
  runtimeModules?: RuntimeModule[];  // 运行态模块菜单（新增）
};

type RuntimeModule = {
  key: string;          // 'analytics' | 'ops' | 'support'
  title: LocalizedText;
  icon?: string;
  children: RuntimeMenuItem[];
};
```

运行态菜单渲染逻辑：
1. 静态模块（analytics、ops、support）从 `routes.ts` 读取
2. 动态模块（console）从 `ConsoleMenuSpec` 读取
3. 合并后按权限过滤

## 5. 文件结构

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
│   ├── Design/                # 设计态页面（或保持现有目录结构）
│   └── Runtime/               # 运行态页面（或保持现有目录结构）
└── config/
    ├── routes.ts              # 路由配置（重构）
    └── defaultSettings.ts     # Layout 配置
```

## 6. 路由配置示例

```typescript
// config/routes.ts
export default [
  // 登录（共享）
  { path: '/user', layout: false, routes: [
    { path: '/user/login', component: './User/Login' },
  ]},

  // 根路由跳转
  { path: '/', component: './Home' },

  // ==================== 设计态 ====================
  {
    path: '/design',
    layout: 'DesignLayout',
    component: './DesignLayout',
    wrappers: ['@/wrappers/DesignAccess'],
    routes: [
      { path: '/design', redirect: '/design/functions' },
      {
        path: '/design/functions',
        name: 'FunctionsAndPages',
        access: 'canFunctionsAndPagesRead',
        routes: [
          { path: '/design/functions', redirect: '/design/functions/catalog' },
          { path: '/design/functions/catalog', name: 'FunctionCatalog', access: 'canFunctionsRead', component: './Functions/Directory' },
          { path: '/design/functions/resources', name: 'Resources', access: 'canResourcesRead', component: './Resources' },
          { path: '/design/functions/pages', name: 'PageStudio', access: 'canPageRead', component: './PageStudio' },
          { path: '/design/functions/proposals', name: 'Proposals', access: 'canPageRead', component: './Proposals' },
          { path: '/design/functions/assignments', name: 'FunctionAssignments', access: 'canAssignmentsRead', component: './Assignments' },
          { path: '/design/functions/instances', name: 'FunctionInstances', access: 'canFunctionsRead', component: './Functions/Instances' },
          { path: '/design/functions/warnings', name: 'FunctionWarnings', access: 'canFunctionsRead', component: './Functions/Warnings' },
          { path: '/design/functions/openapi-sources', name: 'OpenAPISources', access: 'canOpenAPISourcesRead', component: './OpenAPISources' },
          { path: '/design/functions/resource-catalog', name: 'ResourceCatalog', access: 'canResourcesRead', component: './ResourceCatalog' },
          { path: '/design/functions/:id', name: 'FunctionDetail', access: 'canFunctionsRead', component: './Functions/Detail', hideInMenu: true },
          { path: '/design/functions/invoke', name: 'FunctionInvoke', access: 'canFunctionsRead', component: './Functions/Invoke', hideInMenu: true },
        ],
      },
      {
        path: '/design/system',
        name: 'SystemConfig',
        access: 'canSystemConfigRead',
        routes: [
          { path: '/design/system/environments', name: 'GameEnvironments', access: 'canGamesRead', component: './GamesEnvs' },
          { path: '/design/system/extensions', name: 'Extensions', access: 'canExtensionsRead', component: './Extensions/Store' },
          { path: '/design/system/extensions/installations', name: 'ExtensionsInstallations', access: 'canExtensionsRead', component: './Extensions/Installations' },
          { path: '/design/system/extensions/agent-sync', name: 'ExtensionsAgentSync', access: 'canExtensionsRead', component: './Extensions/AgentSync' },
        ],
      },
      {
        path: '/design/admin',
        name: 'AccessControl',
        routes: [
          { path: '/design/admin/users', name: 'Users', access: 'canUserManage', component: './Permissions/UsersV2' },
          { path: '/design/admin/roles', name: 'Roles', access: 'canRoleManage', component: './Permissions/RolesV2' },
          { path: '/design/admin/config', name: 'Config', access: 'canPermissionConfig', component: './Permissions/Config' },
          { path: '/design/admin/login-logs', name: 'LoginLogs', access: 'canAuditRead', component: './Admin/LoginLogs' },
        ],
      },
    ],
  },

  // ==================== 运行态 ====================
  {
    path: '/runtime',
    layout: 'RuntimeLayout',
    component: './RuntimeLayout',
    wrappers: ['@/wrappers/RuntimeAccess'],
    routes: [
      { path: '/runtime', redirect: '/runtime/console' },
      {
        path: '/runtime/console',
        name: 'ControlConsole',
        access: 'canConsoleRead',
        routes: [
          { path: '/runtime/console', redirect: '/runtime/console/home' },
          { path: '/runtime/console/home', name: 'ConsoleHome', access: 'canConsoleRead', component: './Console', hideInMenu: true },
          { path: '/runtime/console/:categoryKey/:pageKey', name: 'ConsolePage', access: 'canConsoleRead', component: './Console/Page', hideInMenu: true },
          { path: '/runtime/console/:categoryKey', name: 'ConsoleCategory', access: 'canConsoleRead', component: './Console', hideInMenu: true },
        ],
      },
      {
        path: '/runtime/analytics',
        name: 'Analytics',
        access: 'canAnalyticsRead',
        routes: [
          { path: '/runtime/analytics', redirect: '/runtime/analytics/realtime' },
          { path: '/runtime/analytics/realtime', name: 'Realtime', access: 'canAnalyticsRead', component: './Analytics/Realtime' },
          { path: '/runtime/analytics/overview', name: 'Overview', access: 'canAnalyticsRead', component: './Analytics/Overview' },
          { path: '/runtime/analytics/retention', name: 'Retention', access: 'canAnalyticsRead', component: './Analytics/Retention' },
          { path: '/runtime/analytics/behavior', name: 'Behavior', access: 'canAnalyticsRead', component: './Analytics/Behavior' },
          { path: '/runtime/analytics/payments', name: 'Payments', access: 'canAnalyticsRead', component: './Analytics/Payments' },
          { path: '/runtime/analytics/levels', name: 'Levels', access: 'canAnalyticsRead', component: './Analytics/Levels' },
        ],
      },
      {
        path: '/runtime/ops',
        name: 'Ops',
        access: 'canOpsRead',
        routes: [
          { path: '/runtime/ops', redirect: '/runtime/ops/nodes' },
          { path: '/runtime/ops/nodes', name: 'Nodes', access: 'canOpsRead', component: './Ops/Nodes' },
          { path: '/runtime/ops/jobs', name: 'Jobs', access: 'canOpsRead', component: './Ops/Jobs' },
          { path: '/runtime/ops/alerts', name: 'Alerts', access: 'canOpsRead', component: './Ops/Alerts' },
          { path: '/runtime/ops/rate-limits', name: 'RateLimits', access: 'canOpsManage', component: './Ops/RateLimits' },
          { path: '/runtime/ops/backups', name: 'Backups', access: 'canOpsManage', component: './Extensions/DomainEntry' },
          { path: '/runtime/ops/certificates', name: 'Certificates', access: 'canOpsManage', component: './Ops/Certificates' },
          { path: '/runtime/ops/analytics-filters', name: 'AnalyticsFilters', access: 'canOpsManage', component: './Ops/AnalyticsFilters' },
          { path: '/runtime/ops/terms', name: 'Terms', access: 'canOpsManage', component: './Ops/Terms' },
        ],
      },
      {
        path: '/runtime/support',
        name: 'Support',
        access: 'canSupportRead',
        routes: [
          { path: '/runtime/support', redirect: '/runtime/support/tickets' },
          { path: '/runtime/support/tickets', name: 'Tickets', access: 'canSupportRead', component: './Support/Tickets' },
          { path: '/runtime/support/tickets/:id', name: 'TicketDetail', access: 'canSupportRead', component: './Support/Tickets/Detail', hideInMenu: true },
          { path: '/runtime/support/faq', name: 'FAQ', access: 'canSupportRead', component: './Support/FAQ' },
          { path: '/runtime/support/bugs', name: 'Bugs', access: 'canSupportRead', component: './Support/Bugs' },
          { path: '/runtime/support/feedback', name: 'Feedback', access: 'canSupportRead', component: './Support/Feedback' },
        ],
      },
    ],
  },

  // 共享页面
  { path: '/403', layout: false, component: './403' },
  { path: '*', layout: false, component: './404' },
];
```

## 7. 权限模型（不变）

现有权限体系完全复用，不需要新增权限点：

| 权限 | 设计态 | 运行态 |
|---|---|---|
| `functions:read` | ✅ 函数目录 | - |
| `functions:manage` | ✅ 函数管理 | - |
| `pages:read` | ✅ 页面设计 | - |
| `resources:read` | ✅ 资源管理 | - |
| `console:read` | - | ✅ 控制台 |
| `analytics:read` | - | ✅ 数据分析 |
| `ops:read` | - | ✅ 运维 |
| `support:read` | - | ✅ 客服 |
| `admin` / `super_admin` | ✅ 全部 | ✅ 全部 |

**访问控制规则：**
- 管理员：两个 Layout 都可访问，默认进设计态
- 运营人员：仅运行态可见，进 `/runtime`
- 无权限用户：跳转 `/403`

## 8. 向后兼容

### 8.1 旧路径重定向

```typescript
// 旧路径 → 新路径的 301 重定向
const legacyRedirects = [
  { from: '/system/functions', to: '/design/functions' },
  { from: '/console', to: '/runtime/console' },
  { from: '/analytics', to: '/runtime/analytics' },
  { from: '/ops', to: '/runtime/ops' },
  { from: '/support', to: '/runtime/support' },
  { from: '/admin', to: '/design/admin' },
];
```

### 8.2 渐进迁移

Phase 1（当前）：
- 创建两个 Layout 组件
- 新增 `/design/*` 和 `/runtime/*` 路由
- 保留旧路由，加 redirect

Phase 2：
- 前端页面组件不需要移动（路径变了，组件引用不变）
- 运行态菜单接入 ConsoleMenuSpec 动态渲染
- 移除旧路由

## 9. 运行态动态菜单扩展

运行态的控制台菜单已经是动态的（`ConsoleMenuSpec`）。扩展支持其他模块：

```typescript
// utils/runtimeMenu.ts
export function buildRuntimeMenu(
  staticRoutes: MenuDataItem[],
  consoleMenu: ConsoleMenuSpec,
  locale: string,
): MenuDataItem[] {
  const menu: MenuDataItem[] = [];

  // 1. 控制台（动态）
  menu.push({
    key: '/runtime/console',
    name: resolveLocalizedText({ 'zh-CN': '控制台', 'en-US': 'Console' }, locale, 'Console'),
    icon: 'appstore',
    children: (consoleMenu?.items || []).map(category => ({
      key: category.path,
      name: resolveLocalizedText(category.title, locale, category.key),
      icon: category.icon,
      children: (category.children || []).map(page => ({
        key: page.path,
        name: resolveLocalizedText(page.title, locale, page.key),
      })),
    })),
  });

  // 2. 静态模块（analytics、ops、support）
  for (const route of staticRoutes) {
    if (route.path?.startsWith('/runtime/') && route.path !== '/runtime/console') {
      menu.push(route);
    }
  }

  return menu;
}
```

## 10. 实施步骤

1. 创建 `DesignLayout` 和 `RuntimeLayout` 组件
2. 创建 `ModeSwitcher` 组件
3. 创建 `DesignAccess` 和 `RuntimeAccess` 权限守卫
4. 重构 `routes.ts`，新增 `/design/*` 和 `/runtime/*` 路由
5. 旧路由添加 redirect 到新路径
6. 根路由 `/` 实现自动跳转逻辑
7. 运行态菜单接入 `ConsoleMenuSpec` 动态渲染
8. 移除旧路由（向后兼容期结束后）
