import { request } from '@umijs/max';

// Source: croupier/internal/api/permission/dto.go Permission
export type PermissionRecord = {
  id: string;
  name: string;
  description: string;
  resource: string;
  action: string;
  category: string;
  createdAt: string;
  updatedAt: string;
};

// Source: croupier/internal/api/profile/dto.go ProfilePermission
export type UserPermission = {
  resource: string;
  actions: string[];
  gameId?: string;
  env?: string;
};

// Source: croupier/internal/api/profile/dto.go ProfilePermissionsResponse
export type UserPermissionsResponse = {
  items: UserPermission[];
  admin: boolean;
  roles: string[];
};

// Source: croupier/internal/api/role/dto.go Role
export type RoleRecord = {
  id: number;
  name: string;
  description: string;
  category: string;
  permissions: string[];
  createdAt: string;
  updatedAt: string;
};

// === 权限管理 API ===

export async function listPermissions(params?: {
  page?: number;
  pageSize?: number;
  resource?: string;
}) {
  return request<{ items: PermissionRecord[]; total: number; page: number; pageSize: number }>(
    '/api/v1/permissions',
    {
      params,
    },
  );
}

export async function getPermission(id: string) {
  return request<PermissionRecord>(`/api/v1/permissions/${id}`, {
    method: 'GET',
  });
}

// === 管理员管理 API ===

// Source: croupier/internal/api/admin/dto.go Admin
/** 管理员账号状态（对齐后端 model.StatusEnabled / model.StatusDisabled） */
export const ADMIN_STATUS_DISABLED = 0;
export const ADMIN_STATUS_ACTIVE = 1;
/** 后端 Update 契约哨兵：-1 = 不修改状态（Go int 零值与 DISABLED 撞值，只能显式表达） */
export const ADMIN_STATUS_UNCHANGED = -1;

export type AdminRecord = {
  id: number;
  username: string;
  nickname: string;
  email?: string;
  phone?: string;
  roles: string[];
  status: number;
  /** 引导账号（admins.json 等自举配置声明）：不可删除，可禁用（BUG-028） */
  bootstrap?: boolean;
  createdAt: string;
  updatedAt: string;
};

// Source: croupier/internal/api/admin/dto.go AdminGame
export type AdminGame = {
  gameId: string;
  gameName: string;
  envs: string[];
};

export async function listAdmins(params?: {
  page?: number;
  pageSize?: number;
  search?: string;
  role?: string;
  status?: number;
}) {
  return request<{ items: AdminRecord[]; total: number; page: number; pageSize: number }>(
    '/api/v1/admin',
    {
      params,
    },
  );
}

export async function createAdmin(body: {
  username: string;
  password: string;
  nickname?: string;
  email?: string;
  phone?: string;
  roles: string[];
}) {
  return request<AdminRecord>('/api/v1/admin', {
    method: 'POST',
    data: body,
  });
}

export async function getAdmin(id: number) {
  return request<AdminRecord>(`/api/v1/admin/${id}`, {
    method: 'GET',
  });
}

export async function updateAdmin(
  id: number,
  body: {
    nickname?: string;
    email?: string;
    phone?: string;
    roles?: string[];
    status?: number;
  },
) {
  return request<AdminRecord>(`/api/v1/admin/${id}`, {
    method: 'PUT',
    data: body,
  });
}

export async function deleteAdmin(id: number) {
  return request<void>(`/api/v1/admin/${id}`, {
    method: 'DELETE',
  });
}

export async function resetAdminPassword(id: number, newPassword: string) {
  return request<void>(`/api/v1/admin/${id}/password-reset`, {
    method: 'POST',
    data: { newPassword },
  });
}

// === 角色管理 API ===

export async function listRoles(params?: {
  page?: number;
  pageSize?: number;
  category?: string;
  search?: string;
}) {
  return request<{ items: RoleRecord[]; total: number; page: number; pageSize: number }>(
    '/api/v1/roles',
    {
      params,
    },
  );
}

export async function createRole(body: {
  name: string;
  description?: string;
  category?: string;
  permissions?: string[];
}) {
  return request<RoleRecord>('/api/v1/roles', {
    method: 'POST',
    data: body,
  });
}

export async function getRole(id: number) {
  return request<RoleRecord>(`/api/v1/roles/${id}`, {
    method: 'GET',
  });
}

export async function updateRole(
  id: number,
  body: {
    name?: string;
    description?: string;
    category?: string;
    permissions?: string[];
  },
) {
  return request<RoleRecord>(`/api/v1/roles/${id}`, {
    method: 'PUT',
    data: body,
  });
}

export async function deleteRole(id: number) {
  return request<void>(`/api/v1/roles/${id}`, {
    method: 'DELETE',
  });
}

export async function updateRolePermissions(id: number, permissions: string[]) {
  return request<void>(`/api/v1/roles/${id}/permissions`, {
    method: 'PUT',
    data: { permissions },
  });
}

// === 权限检查 API ===

export async function getUserPermissions(params?: { gameId?: string; env?: string }) {
  return request<UserPermissionsResponse>('/api/v1/profile/permissions', {
    params,
  });
}

export async function checkPermission(params: {
  resource: string;
  action: string;
  gameId?: string;
  env?: string;
}) {
  return request<{ allowed: boolean; reason?: string }>('/api/v1/auth/check', {
    method: 'POST',
    data: params,
  });
}

export async function batchCheckPermissions(
  checks: Array<{
    resource: string;
    action: string;
    gameId?: string;
    env?: string;
  }>,
) {
  return request<{ results: Array<{ allowed: boolean; reason?: string }> }>(
    '/api/v1/auth/check/batch',
    {
      method: 'POST',
      data: { checks },
    },
  );
}

// === 管理员游戏权限 API ===

export async function getAdminGames(adminId: number) {
  return request<{ games: AdminGame[] }>(`/api/v1/admin/${adminId}/games`, {
    method: 'GET',
  });
}

export async function updateAdminGames(
  adminId: number,
  games: Array<{ gameId: string; envs: string[] }>,
) {
  return request<void>(`/api/v1/admin/${adminId}/games`, {
    method: 'PUT',
    data: { games },
  });
}
