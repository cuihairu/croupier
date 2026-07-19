# 运行控制台动态菜单方案

## 问题描述

当前运行控制台的问题：
1. 没有左侧菜单，只有单页面展示
2. 不支持按类型分类
3. 没有根据用户权限过滤显示

## 目标架构

```
左侧菜单（系统菜单）：
├── 函数管理
│   ├── 函数目录
│   └── 对象工作台
├── 运行控制台 ← 动态生成，按分类组织
│   ├── 玩家管理
│   │   ├── 玩家查询
│   │   └── 玩家列表
│   ├── 物品管理
│   │   └── 物品发放
│   └── 其他工具
│       └── 订单查询
└── 运维管理
```

## 技术方案

### 1. 数据结构扩展

```typescript
// web/src/types/workspace.ts
export interface WorkspaceConfig {
  // ... 现有字段
  category?: WorkspaceCategory;  // 分类
  permissions?: {
    roles?: string[];       // 允许的角色
    permissions?: string[]; // 允许的权限ID
  };
}

export type WorkspaceCategory = 
  | 'player'      // 玩家管理
  | 'inventory'   // 物品管理
  | 'order'       // 订单管理
  | 'economy'     // 经济系统
  | 'social'      // 社交系统
  | 'other';      // 其他
```

### 2. 分类配置

```typescript
// web/src/config/workspaceCategories.ts
export const WORKSPACE_CATEGORIES = {
  player: {
    name: '玩家管理',
    icon: 'user',
    order: 1,
  },
  inventory: {
    name: '物品管理',
    icon: 'gift',
    order: 2,
  },
  order: {
    name: '订单管理',
    icon: 'shopping',
    order: 3,
  },
  economy: {
    name: '经济系统',
    icon: 'dollar',
    order: 4,
  },
  social: {
    name: '社交系统',
    icon: 'team',
    order: 5,
  },
  other: {
    name: '其他工具',
    icon: 'tool',
    order: 99,
  },
};
```

### 3. 动态路由注入

```typescript
// web/src/app.tsx
import { patchClientRoutes } from '@umijs/max';
import { listPublishedWorkspaceConfigs } from '@/services/workspaceConfig';
import { WORKSPACE_CATEGORIES } from '@/config/workspaceCategories';

export async function patchClientRoutes({ routes }) {
  // 获取已发布的工作台
  const configs = await listPublishedWorkspaceConfigs();
  
  // 根据权限过滤
  const filteredConfigs = filterByPermission(configs);
  
  // 按分类分组
  const grouped = groupByCategory(filteredConfigs);
  
  // 找到 console 路由，添加动态子路由
  const consoleRoute = routes.find(r => r.path === '/console');
  if (consoleRoute) {
    consoleRoute.routes = buildDynamicRoutes(grouped);
  }
}

function filterByPermission(configs, userRoles) {
  return configs.filter(config => {
    // 1. 检查显式配置的权限
    if (config.permissions?.roles?.length) {
      return hasAnyRole(userRoles, config.permissions.roles);
    }
    // 2. 无权限配置则显示
    return true;
  });
}

function groupByCategory(configs) {
  const grouped = {};
  configs.forEach(config => {
    const category = config.category || 'other';
    if (!grouped[category]) {
      grouped[category] = [];
    }
    grouped[category].push(config);
  });
  return grouped;
}

function buildDynamicRoutes(grouped) {
  return Object.entries(grouped).map(([category, configs]) => ({
    path: `/console/${category}`,
    name: WORKSPACE_CATEGORIES[category]?.name || category,
    icon: WORKSPACE_CATEGORIES[category]?.icon || 'appstore',
    routes: configs.map(config => ({
      path: `/console/${config.objectKey}`,
      name: config.title,
      component: './Console/Workspace',  // 复用同一个组件
    })),
  }));
}
```

### 4. Workspace 组件

```typescript
// web/src/pages/Console/Workspace.tsx
export default function ConsoleWorkspace() {
  const { objectKey } = useParams();
  const { config, loading, error } = useWorkspaceConfig(objectKey);
  
  if (loading) return <Spin />;
  if (error) return <ErrorPage error={error} />;
  
  return <WorkspaceRenderer config={config} runtimeMode="console" />;
}
```

### 5. 后端 API 改动

```go
// internal/api/workspace/dto.go
type WorkspaceConfig struct {
    // ... 现有字段
    Category    string              `json:"category,omitempty"`
    Permissions *WorkspacePermissions `json:"permissions,omitempty"`
}

type WorkspacePermissions struct {
    Roles       []string `json:"roles,omitempty"`
    Permissions []string `json:"permissions,omitempty"`
}
```

## 实施步骤

### Phase 1：基础框架
1. 添加 `category` 字段到 WorkspaceConfig
2. 创建分类配置文件
3. 实现 `patchClientRoutes` 动态路由

### Phase 2：权限过滤
1. 添加 `permissions` 字段到 WorkspaceConfig
2. 实现权限过滤逻辑
3. 前端根据权限显示/隐藏菜单

### Phase 3：优化
1. 支持自定义分类
2. 支持菜单排序
3. 缓存优化

## 测试用例

1. 无权限用户看不到受限工作台
2. admin 用户看到所有工作台
3. 按分类正确分组显示
4. 动态路由正确跳转
