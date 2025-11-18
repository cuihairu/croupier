/**
 * 添加新菜单项的配置示例
 * 
 * 这是一个完整的配置示例,展示如何添加用户管理和角色管理两个新菜单项
 */

// ============================================================
// 1. config/routes.ts - 路由配置
// ============================================================

export default [
  {
    path: '/user',
    layout: false,
    routes: [
      {
        name: 'login',
        path: '/user/login',
        component: './User/Login',
      },
    ],
  },
  {
    path: '/gm',
    name: 'GM',
    icon: 'tool',
    routes: [
      {
        path: '/gm/approvals',
        name: 'Approvals',
        component: './Approvals',
      },
      {
        path: '/gm/functions',
        name: 'Functions',
        component: './GmFunctions',
      },
      {
        path: '/gm/games',
        name: 'Games',
        component: './GameManage',
      },
      {
        path: '/gm/assignments',
        name: 'Assignments',
        access: 'canAssignmentsRead',
        component: './Assignments',
      },
      {
        path: '/gm/audit',
        name: 'Audit',
        access: 'canAuditRead',
        component: './Audit',
      },
      {
        path: '/gm/packs',
        name: 'Packs',
        component: './Packs',
      },
      {
        path: '/gm/registry',
        name: 'Registry',
        access: 'canRegistryRead',
        component: './Registry',
      },
      // ===== 新增菜单项 =====
      {
        path: '/gm/users',
        name: 'Users',
        access: 'canUsersRead',
        component: './Users',
      },
      {
        path: '/gm/roles',
        name: 'Roles',
        access: 'canRolesRead',
        component: './Roles',
      },
    ],
  },
  // ... 其他路由
];

// ============================================================
// 2. src/access.ts - 权限定义
// ============================================================

export default function access(initialState: { currentUser?: API.CurrentUser } | undefined) {
  const acc = (initialState?.currentUser as any)?.access as string | undefined;
  const perms = new Set((acc || '').split(',').filter(Boolean));
  const has = (p: string) => perms.has('*') || perms.has(p);
  
  return {
    canAdmin: has('admin'),
    canRegistryRead: has('registry:read'),
    canAssignmentsRead: has('assignments:read'),
    canAssignmentsWrite: has('assignments:write'),
    canPacksReload: has('packs:reload'),
    canPacksExport: has('packs:export'),
    canAuditRead: has('audit:read'),
    // ===== 新增权限 =====
    canUsersRead: has('users:read'),
    canUsersWrite: has('users:write'),
    canUsersDelete: has('users:delete'),
    canRolesRead: has('roles:read'),
    canRolesWrite: has('roles:write'),
  };
}

// ============================================================
// 3. src/locales/en-US/menu.ts - 英文菜单
// ============================================================

export default {
  'menu.welcome': 'Welcome',
  'menu.admin': 'Admin',
  'menu.list.table-list': 'Search Table',
  // ... 其他菜单项
  
  // ===== 新增菜单翻译 =====
  'menu.GM.Users': 'Users',
  'menu.GM.Roles': 'Roles',
};

// ============================================================
// 4. src/locales/zh-CN/menu.ts - 中文菜单
// ============================================================

export default {
  'menu.welcome': '欢迎',
  'menu.admin': '管理页',
  'menu.list.table-list': '查询表格',
  // ... 其他菜单项
  
  // ===== 新增菜单翻译 =====
  'menu.GM.Users': '用户管理',
  'menu.GM.Roles': '角色管理',
};

// ============================================================
// 5. src/services/croupier/index.ts - API 调用
// ============================================================

// 添加以下函数到现有文件中:

// Users
export async function listUsers(params?: any) {
  return request<{ users: any[]; total?: number }>('/api/users', { params });
}

export async function createUser(data: any) {
  return request<void>('/api/users', { method: 'POST', data });
}

export async function updateUser(id: string, data: any) {
  return request<void>(`/api/users/id}`, { method: 'PUT', data });
}

export async function deleteUser(id: string) {
  return request<void>(`/api/users/id}`, { method: 'DELETE' });
}

// Roles
export async function listRoles(params?: any) {
  return request<{ roles: any[]; total?: number }>('/api/roles', { params });
}

export async function createRole(data: any) {
  return request<void>('/api/roles', { method: 'POST', data });
}

export async function updateRole(id: string, data: any) {
  return request<void>(`/api/roles/id}`, { method: 'PUT', data });
}

export async function deleteRole(id: string) {
  return request<void>(`/api/roles/id}`, { method: 'DELETE' });
}
