# Croupier 项目 Web 前端结构分析报告

## 1. 技术栈概述

### 框架和库
- **框架**: Umi Max (基于 UmiJS 4.x) + React 18.2
- **UI 组件库**: Ant Design 5.20 + Ant Design Pro 2.7 (ProLayout, ProComponents)
- **样式**: antd-style 3.6 + Less
- **工具**: TypeScript 5.3, Prettier, ESLint, Jest
- **包管理**: pnpm (使用 pnpm-lock.yaml)

### 开发命令
```bash
pnpm dev / npm run start           # 本地开发(默认 port 8000)
pnpm start:dev                     # 开发模式(禁用Mock)
pnpm build                         # 生产构建
pnpm lint                          # 代码检查
pnpm test                          # 单元测试
```

### 后端服务期望
- 默认后端: http://localhost:8080
- 开发代理配置: `/api/*` 转发到后端
- 凭证: admin / admin123 (开发环境)

---

## 2. 项目结构

```
web/
├── config/                        # Umi Max 配置
│   ├── routes.ts                  # 路由定义(菜单、权限、组件映射)
│   ├── defaultSettings.ts         # 布局默认设置
│   ├── proxy.ts                   # 开发代理配置
│   └── config.ts                  # Umi Max 主配置
├── src/
│   ├── pages/                     # 页面组件(自动路由)
│   │   ├── GmFunctions/           # GM 函数调用界面(动态表单渲染)
│   │   ├── Assignments/           # 函数分配管理(权限分配)
│   │   ├── Audit/                 # 审计日志查看(带时间范围/过滤)
│   │   ├── Approvals/             # 审批流程管理(待审、已批准)
│   │   ├── Registry/              # 函数注册表(代理、函数覆盖率)
│   │   ├── Packs/                 # 函数包管理(导入/导出)
│   │   ├── GameManage/            # 游戏管理
│   │   ├── Welcome.tsx            # 欢迎页
│   │   └── User/Login/            # 登录页
│   ├── components/                # 共享组件
│   │   ├── GameSelector/          # 游戏/环境选择器(localStorage 存储)
│   │   ├── AvatarDropdown/        # 用户菜单
│   │   ├── Footer/                # 页脚
│   │   └── RightContent/          # 右侧工具栏
│   ├── services/                  # API 服务层
│   │   └── croupier/index.ts      # 所有后端 API 调用
│   ├── locales/                   # 国际化文本(8种语言)
│   │   ├── en-US/menu.ts          # 英文菜单翻译
│   │   ├── zh-CN/menu.ts          # 中文菜单翻译
│   │   └── ...
│   ├── access.ts                  # 权限检查逻辑(RBAC)
│   ├── app.tsx                    # App 入口(初始化状态、布局)
│   ├── global.tsx                 # 全局样式和设置
│   └── requestErrorConfig.ts      # 请求错误处理
├── public/                        # 静态资源
├── package.json
├── tsconfig.json
└── README.md
```

---

## 3. 路由和菜单系统

### 3.1 路由配置 (`config/routes.ts`)

```typescript
// 路由基本结构:
[
  {
    path: '/gm',                   // URL 路径
    name: 'GM',                    // 菜单标签(从 menu.GM 读取)
    icon: 'tool',                  // 菜单图标(Ant Design icon 名)
    routes: [                      # 子菜单
      {
        path: '/gm/functions',
        name: 'Functions',         # i18n key: menu.Functions
        component: './GmFunctions',# 相对于 src/pages/ 的路径
        access?: 'canFunctionRead' # 权限检查
      },
      ...
    ]
  },
  ...
]
```

### 3.2 菜单配置位置

**英文菜单** (`src/locales/en-US/menu.ts`):
```typescript
export default {
  'menu.GM': 'GM',
  'menu.GM.Functions': 'Functions',
  'menu.GM.Assignments': 'Assignments',
  'menu.GM.Audit': 'Audit',
  ...
};
```

**中文菜单** (`src/locales/zh-CN/menu.ts`):
```typescript
export default {
  'menu.GM': '运营',
  'menu.GM.Functions': '函数列表',
  'menu.GM.Assignments': '任务分配',
  'menu.GM.Audit': '审计',
  ...
};
```

> **注意**: i18n key 格式 = `menu.` + 路由 path 转驼峰或直接追加

### 3.3 图标映射

使用 Ant Design Icons (移除后缀+大小写):
- `tool` → `<ToolOutlined />`
- `crown` → `<CrownOutlined />`
- `smile` → `<SmileOutlined />`
- `table` → `<TableOutlined />`

---

## 4. 权限系统实现

### 4.1 权限检查逻辑 (`src/access.ts`)

```typescript
export default function access(initialState: { currentUser?: API.CurrentUser } | undefined) {
  // 从用户对象解析权限字符串(逗号分隔)
  const acc = (initialState?.currentUser as any)?.access as string | undefined;
  const perms = new Set((acc || '').split(',').filter(Boolean));
  
  // 权限检查函数
  const has = (p: string) => perms.has('*') || perms.has(p);
  
  return {
    canAdmin: has('admin'),
    canRegistryRead: has('registry:read'),
    canAssignmentsRead: has('assignments:read'),
    canAssignmentsWrite: has('assignments:write'),
    canPacksReload: has('packs:reload'),
    canPacksExport: has('packs:export'),
    canAuditRead: has('audit:read'),
  };
}
```

### 4.2 路由级权限控制

在 `config/routes.ts` 中使用 `access` 属性:

```typescript
{
  path: '/gm/audit',
  name: 'Audit',
  access: 'canAuditRead',        // 只有权限检查通过才显示菜单/访问页面
  component: './Audit',
}
```

### 4.3 用户信息初始化 (`src/app.tsx`)

```typescript
export async function getInitialState() {
  const fetchUserInfo = async () => {
    const me = await fetchMe();  // GET /api/users/current
    return {
      name: me.username,
      userid: me.username,
      access: (me.roles || []).join(',')  // 角色作为权限字符串
    } as any;
  };
  
  const { location } = history;
  if (location.pathname !== loginPath) {
    const currentUser = await fetchUserInfo();
    return { fetchUserInfo, currentUser, ... };
  }
}
```

### 4.4 页面级权限检查示例

**Audit 页面** (`src/pages/Audit/index.tsx`):
```typescript
export default function AuditPage() {
  const { initialState } = useModel('@@initialState');
  const roles = useMemo(() => {
    const acc = (initialState as any)?.currentUser?.access as string | undefined;
    return (acc ? acc.split(',') : []).filter(Boolean);
  }, [initialState]);
  
  const canRead = roles.includes('*') || roles.includes('audit:read');
  
  if (!canRead) {
    return (
      <Card title="Audit">
        <Typography.Text type="secondary">No permission: audit:read</Typography.Text>
      </Card>
    );
  }
  
  // 页面内容...
}
```

---

## 5. 现有页面分析

### 5.1 权限相关页面

| 页面 | 路径 | 权限控制 | 功能 |
|------|------|---------|------|
| **Assignments** | `/gm/assignments` | `assignments:read` (菜单) + `assignments:write` (操作) | 管理函数分配 |
| **Audit** | `/gm/audit` | `audit:read` | 审计日志查看、导出、自动刷新 |
| **Approvals** | `/gm/approvals` | 无(默认可见) | 审批流程、OTP 验证、风险标记 |
| **Registry** | `/gm/registry` | `registry:read` | 代理/函数注册表、覆盖率分析 |
| **GmFunctions** | `/gm/functions` | 无(默认可见) | 函数动态表单调用、作业执行 |
| **Packs** | `/gm/packs` | 部分(reload/export) | 函数包导入/导出 |

### 5.2 API 服务层 (`src/services/croupier/index.ts`)

所有后端调用:
- `loginAuth(params)` → POST /api/auth/login
- `fetchMe()` → GET /api/users/current
- `listDescriptors()` → GET /api/descriptors
- `invokeFunction(id, payload)` → POST /api/invoke
- `startJob()` → POST /api/start_job
- `cancelJob()` → POST /api/cancel_job
- `listAudit(params)` → GET /api/audit
- `fetchAssignments(params)` → GET /api/assignments
- `setAssignments(params)` → POST /api/assignments
- `fetchRegistry()` → GET /api/registry
- `listPacks()` → GET /api/packs/list
- `reloadPacks()` → POST /api/packs/reload

---

## 6. 如何添加新的权限管理页面

### 步骤 1: 更新权限定义 (`src/access.ts`)

```typescript
export default function access(initialState) {
  const acc = (initialState?.currentUser as any)?.access as string | undefined;
  const perms = new Set((acc || '').split(',').filter(Boolean));
  const has = (p: string) => perms.has('*') || perms.has(p);
  
  return {
    // ... 现有权限
    canUsersRead: has('users:read'),      // 新增: 用户管理读权限
    canUsersWrite: has('users:write'),    // 新增: 用户管理写权限
    canRolesRead: has('roles:read'),      // 新增: 角色管理读权限
    canRolesWrite: has('roles:write'),    // 新增: 角色管理写权限
  };
}
```

### 步骤 2: 添加路由 (`config/routes.ts`)

```typescript
{
  path: '/gm',
  name: 'GM',
  icon: 'tool',
  routes: [
    // ... 现有菜单
    {
      path: '/gm/users',
      name: 'Users',
      access: 'canUsersRead',
      component: './Users',      // 新建页面: src/pages/Users/index.tsx
    },
    {
      path: '/gm/roles',
      name: 'Roles',
      access: 'canRolesRead',
      component: './Roles',      // 新建页面: src/pages/Roles/index.tsx
    },
  ]
}
```

### 步骤 3: 添加菜单翻译

**英文** (`src/locales/en-US/menu.ts`):
```typescript
export default {
  'menu.GM': 'GM',
  'menu.GM.Users': 'Users',      // 新增
  'menu.GM.Roles': 'Roles',      // 新增
  // ... 现有
};
```

**中文** (`src/locales/zh-CN/menu.ts`):
```typescript
export default {
  'menu.GM': '运营',
  'menu.GM.Users': '用户管理',    // 新增
  'menu.GM.Roles': '角色管理',    // 新增
  // ... 现有
};
```

### 步骤 4: 创建页面组件

**`src/pages/Users/index.tsx`**:
```typescript
import React, { useEffect, useMemo, useState } from 'react';
import { Card, Table, Space, Button, Modal, Form, Input, message } from 'antd';
import { useModel } from '@umijs/max';
import { request } from '@umijs/max';

export default function UsersPage() {
  const [data, setData] = useState([]);
  const [loading, setLoading] = useState(false);
  const { initialState } = useModel('@@initialState');
  
  // 权限检查
  const roles = useMemo(() => {
    const acc = (initialState as any)?.currentUser?.access as string | undefined;
    return (acc ? acc.split(',') : []).filter(Boolean);
  }, [initialState]);
  
  const canRead = roles.includes('*') || roles.includes('users:read');
  const canWrite = roles.includes('*') || roles.includes('users:write');
  
  // 页面实现...
  
  if (!canRead) {
    return <Card title="Users"><span>No permission: users:read</span></Card>;
  }
  
  return (
    <Card title="Users" extra={<Button onClick={() => {}}>Add User</Button>}>
      {/* 表格、表单等内容 */}
    </Card>
  );
}
```

### 步骤 5: 添加后端 API 调用

**`src/services/croupier/index.ts` 中追加**:
```typescript
// Users
export async function listUsers(params?: any) {
  return request<{ users: any[]; total?: number }>('/api/users', { params });
}

export async function createUser(data: any) {
  return request<void>('/api/users', { method: 'POST', data });
}

export async function updateUser(id: string, data: any) {
  return request<void>(`/api/users/${id}`, { method: 'PUT', data });
}

export async function deleteUser(id: string) {
  return request<void>(`/api/users/${id}`, { method: 'DELETE' });
}

// Roles
export async function listRoles(params?: any) {
  return request<{ roles: any[]; total?: number }>('/api/roles', { params });
}

export async function createRole(data: any) {
  return request<void>('/api/roles', { method: 'POST', data });
}
```

---

## 7. 权限最佳实践

### 7.1 权限命名约定

```
<resource>:<action>
- audit:read         # 查看审计日志
- audit:write        # 编辑审计(通常不需要)
- assignments:read   # 查看分配
- assignments:write  # 修改分配
- users:read         # 查看用户列表
- users:write        # 创建/编辑/删除用户
- roles:read         # 查看角色
- roles:write        # 创建/编辑/删除角色
- admin              # 管理员全权限
```

### 7.2 权限级别

- `*` 表示超级权限(包含所有权限)
- 特定权限如 `audit:read` 只开放该资源读权限
- 通常不需要 `:write` 权限时仅声明 `:read`

### 7.3 页面内部权限检查

```typescript
// 方式 1: 在 access.ts 中检查路由权限(隐藏菜单)
access: 'canAuditRead'

// 方式 2: 在页面中检查按钮权限(显示禁用按钮)
const canDelete = canWrite && !isSystemRole;
<Button disabled={!canDelete} onClick={deleteRole}>Delete</Button>

// 方式 3: 在页面中检查整个功能权限(显示"无权限"提示)
if (!canWrite) {
  return <Typography.Text type="secondary">No permission</Typography.Text>;
}
```

---

## 8. 国际化 (i18n) 系统

### 8.1 添加新文本

1. **英文** (`src/locales/en-US/menu.ts`):
   ```typescript
   'menu.GM.Users': 'Users',
   'pages.users.add': 'Add User',
   'pages.users.edit': 'Edit User',
   ```

2. **中文** (`src/locales/zh-CN/menu.ts`):
   ```typescript
   'menu.GM.Users': '用户管理',
   'pages.users.add': '添加用户',
   'pages.users.edit': '编辑用户',
   ```

3. **页面中使用**:
   ```typescript
   import { FormattedMessage } from '@umijs/max';
   
   <Button><FormattedMessage id="pages.users.add" /></Button>
   // 或使用 formatMessage hook
   const { formatMessage } = useIntl();
   <Button>{formatMessage({ id: 'pages.users.add' })}</Button>
   ```

### 8.2 支持的语言

- en-US (英文)
- zh-CN (简体中文)
- zh-TW (繁体中文)
- ja-JP (日文)
- pt-BR (葡萄牙文)
- fa-IR (波斯文)
- bn-BD (孟加拉文)
- id-ID (印尼文)

---

## 9. 现有权限管理功能

### 9.1 Assignments (函数分配)

- **路径**: `/gm/assignments`
- **权限**: `assignments:read` (查看) + `assignments:write` (编辑)
- **功能**:
  - 选择游戏/环境
  - 配置允许调用的函数列表
  - 空列表表示允许所有函数

### 9.2 Audit (审计日志)

- **路径**: `/gm/audit`
- **权限**: `audit:read`
- **功能**:
  - 查看所有操作审计日志
  - 按时间、用户、操作类型过滤
  - CSV 导出
  - 自动刷新

### 9.3 Approvals (审批流程)

- **路径**: `/gm/approvals`
- **权限**: 无(默认可见)
- **功能**:
  - 批准/拒绝待处理请求
  - OTP 验证
  - 高风险函数确认
  - 审批历史记录

---

## 10. 建议: 权限管理功能完整方案

### 功能模块 1: 用户管理 (`/gm/users`)

```
权限: users:read / users:write
功能:
- 用户列表(分页、搜索、排序)
- 创建/编辑/删除用户
- 用户状态切换(启用/禁用)
- 角色分配
```

### 功能模块 2: 角色管理 (`/gm/roles`)

```
权限: roles:read / roles:write
功能:
- 角色列表(内置+自定义)
- 权限编辑(勾选权限清单)
- 角色复制
- 角色使用统计
```

### 功能模块 3: 权限审计 (`/gm/permissions`)

```
权限: permissions:read
功能:
- 权限矩阵(用户 x 权限)
- 角色权限清单
- 权限变更历史
- 异常权限告警
```

### 功能模块 4: API 密钥管理 (`/gm/api-keys`)

```
权限: apikeys:read / apikeys:write
功能:
- 生成/撤销 API 密钥
- 权限绑定
- 访问日志
- 有效期设置
```

---

## 11. 开发工作流

### 快速启动

```bash
# 1. 安装依赖
cd web && pnpm install

# 2. 启动开发服务器
pnpm dev

# 3. 确保后端运行在 localhost:8080
# go run ./cmd/server -c configs/server.example.yaml

# 4. 访问 http://localhost:8000
# 登录: admin / admin123
```

### 添加页面清单

- [ ] 1. 更新 `config/routes.ts` 添加路由 + 权限
- [ ] 2. 在 `src/locales/en-US/menu.ts` 和 `zh-CN/menu.ts` 添加菜单标签
- [ ] 3. 创建 `src/pages/YourPage/index.tsx` 页面组件
- [ ] 4. 在 `src/services/croupier/index.ts` 添加 API 调用函数
- [ ] 5. 在 `src/access.ts` 定义权限检查函数
- [ ] 6. 测试权限(有权限用户能看到菜单/按钮)

---

## 12. 现存问题和优化建议

### 现存问题

1. **权限字符串格式**: 后端返回 roles 数组,前端转为逗号分隔字符串
   - 建议: 后端返回结构化权限对象而非字符串
   
2. **权限缓存**: 页面刷新时重新获取权限
   - 建议: 实现权限缓存和更新通知机制

3. **细粒度控制缺失**: 目前仅支持 resource:action 二级
   - 建议: 支持三级(resource:action:scope) 如 `audit:export:current_game`

### 优化建议

1. **权限管理 UI**: 添加可视化权限矩阵编辑器
2. **权限继承**: 角色权限继承和覆盖机制
3. **权限审计**: 完整的权限变更日志
4. **临时权限**: 支持时间限制的临时权限提升
5. **权限预加载**: 优化初始状态加载
