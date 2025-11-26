import { request } from '@umijs/max';

type ApiResponse<T> = T | { data?: T } | { Data?: T } | null;

function unwrap<T>(resp: ApiResponse<T>): T {
  if (resp && typeof resp === 'object') {
    const anyResp = resp as any;
    if (anyResp.data) return anyResp.data as T;
    if (anyResp.Data) return anyResp.Data as T;
  }
  return resp as T;
}

// 权限相关类型定义
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

export type UserPermission = {
  resource: string;
  actions: string[];
  gameId?: string;
  env?: string;
};

export type UserPermissionsResponse = {
  items: UserPermission[];
  admin: boolean;
  roles: string[];
};

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

export async function listPermissions(params?: { page?: number; pageSize?: number; category?: string; resource?: string }) {
  const token = typeof window !== 'undefined' ? localStorage.getItem('token') : '';
  const resp = await request<ApiResponse<{ items: PermissionRecord[]; total: number; page: number; pageSize: number }>>('/api/v1/permissions', {
    params,
    headers: token ? { Authorization: `Bearer ${token}` } : undefined,
  });
  return unwrap(resp);
}

export async function getPermission(id: string) {
  const token = typeof window !== 'undefined' ? localStorage.getItem('token') : '';
  const resp = await request<ApiResponse<PermissionRecord>>(`/api/v1/permissions/${id}`, {
    method: 'GET',
    headers: token ? { Authorization: `Bearer ${token}` } : undefined,
  });
  return unwrap(resp);
}

// === 管理员管理 API ===

export type AdminRecord = {
  id: number;
  username: string;
  nickname: string;
  email?: string;
  phone?: string;
  roles: string[];
  status: number;
  createdAt: string;
  updatedAt: string;
};

export type AdminGame = {
  gameId: string;
  gameName: string;
  envs: string[];
};

export async function listAdmins(params?: { page?: number; pageSize?: number; search?: string; role?: string; status?: number }) {
  const token = typeof window !== 'undefined' ? localStorage.getItem('token') : '';
  const resp = await request<ApiResponse<{ items: AdminRecord[]; total: number; page: number; pageSize: number }>>('/api/v1/admin', {
    params,
    headers: token ? { Authorization: `Bearer ${token}` } : undefined,
  });
  return unwrap(resp);
}

export async function createAdmin(body: {
  username: string;
  password: string;
  nickname?: string;
  email?: string;
  phone?: string;
  roles: string[]
}) {
  const token = typeof window !== 'undefined' ? localStorage.getItem('token') : '';
  const resp = await request<ApiResponse<AdminRecord>>('/api/v1/admin', {
    method: 'POST',
    data: body,
    headers: token ? { Authorization: `Bearer ${token}` } : undefined,
  });
  return unwrap(resp);
}

export async function getAdmin(id: number) {
  const token = typeof window !== 'undefined' ? localStorage.getItem('token') : '';
  const resp = await request<ApiResponse<AdminRecord>>(`/api/v1/admin/${id}`, {
    method: 'GET',
    headers: token ? { Authorization: `Bearer ${token}` } : undefined,
  });
  return unwrap(resp);
}

export async function updateAdmin(id: number, body: {
  nickname?: string;
  email?: string;
  phone?: string;
  roles?: string[];
  status?: number
}) {
  const token = typeof window !== 'undefined' ? localStorage.getItem('token') : '';
  const resp = await request<ApiResponse<AdminRecord>>(`/api/v1/admin/${id}`, {
    method: 'PUT',
    data: body,
    headers: token ? { Authorization: `Bearer ${token}` } : undefined,
  });
  return unwrap(resp);
}

export async function deleteAdmin(id: number) {
  const token = typeof window !== 'undefined' ? localStorage.getItem('token') : '';
  return request<void>(`/api/v1/admin/${id}`, {
    method: 'DELETE',
    headers: token ? { Authorization: `Bearer ${token}` } : undefined,
  });
}

export async function resetAdminPassword(id: number, newPassword: string) {
  const token = typeof window !== 'undefined' ? localStorage.getItem('token') : '';
  return request<void>(`/api/v1/admin/${id}/password-reset`, {
    method: 'POST',
    data: { newPassword },
    headers: token ? { Authorization: `Bearer ${token}` } : undefined,
  });
}

// === 角色管理 API ===

export async function listRoles(params?: { page?: number; pageSize?: number; category?: string; search?: string }) {
  const token = typeof window !== 'undefined' ? localStorage.getItem('token') : '';
  const resp = await request<ApiResponse<{ items: RoleRecord[]; total: number; page: number; pageSize: number }>>('/api/v1/roles', {
    params,
    headers: token ? { Authorization: `Bearer ${token}` } : undefined,
  });
  return unwrap(resp);
}

export async function createRole(body: {
  name: string;
  description?: string;
  category?: string;
  permissions?: string[];
}) {
  const token = typeof window !== 'undefined' ? localStorage.getItem('token') : '';
  const resp = await request<ApiResponse<RoleRecord>>('/api/v1/roles', {
    method: 'POST',
    data: body,
    headers: token ? { Authorization: `Bearer ${token}` } : undefined,
  });
  return unwrap(resp);
}

export async function getRole(id: number) {
  const token = typeof window !== 'undefined' ? localStorage.getItem('token') : '';
  const resp = await request<ApiResponse<RoleRecord>>(`/api/v1/roles/${id}`, {
    method: 'GET',
    headers: token ? { Authorization: `Bearer ${token}` } : undefined,
  });
  return unwrap(resp);
}

export async function updateRole(id: number, body: {
  name?: string;
  description?: string;
  category?: string;
  permissions?: string[];
}) {
  const token = typeof window !== 'undefined' ? localStorage.getItem('token') : '';
  const resp = await request<ApiResponse<RoleRecord>>(`/api/v1/roles/${id}`, {
    method: 'PUT',
    data: body,
    headers: token ? { Authorization: `Bearer ${token}` } : undefined,
  });
  return unwrap(resp);
}

export async function deleteRole(id: number) {
  const token = typeof window !== 'undefined' ? localStorage.getItem('token') : '';
  return request<void>(`/api/v1/roles/${id}`, {
    method: 'DELETE',
    headers: token ? { Authorization: `Bearer ${token}` } : undefined,
  });
}

export async function updateRolePermissions(id: number, permissions: string[]) {
  const token = typeof window !== 'undefined' ? localStorage.getItem('token') : '';
  return request<void>(`/api/v1/roles/${id}/permissions`, {
    method: 'PUT',
    data: { permissions },
    headers: token ? { Authorization: `Bearer ${token}` } : undefined,
  });
}

// 向后兼容的别名
export const setRolePerms = updateRolePermissions;

// === 权限检查 API ===

export async function getUserPermissions(params?: { gameId?: string; env?: string }) {
  const resp = await request<ApiResponse<UserPermissionsResponse>>('/api/v1/profile/permissions', { params });
  return unwrap(resp);
}

export async function checkPermission(params: {
  resource: string;
  action: string;
  gameId?: string;
  env?: string
}) {
  const resp = await request<ApiResponse<{ allowed: boolean; reason?: string }>>('/api/v1/auth/check', {
    method: 'POST',
    data: params
  });
  return unwrap(resp);
}

export async function batchCheckPermissions(checks: Array<{
  resource: string;
  action: string;
  gameId?: string;
  env?: string
}>) {
  const resp = await request<ApiResponse<{ results: Array<{ allowed: boolean; reason?: string }> }>>('/api/v1/auth/check/batch', {
    method: 'POST',
    data: { checks }
  });
  return unwrap(resp);
}

// === 管理员游戏权限 API ===

export async function getAdminGames(adminId: number) {
  const token = typeof window !== 'undefined' ? localStorage.getItem('token') : '';
  const resp = await request<ApiResponse<{ games: AdminGame[] }>>(`/api/v1/admin/${adminId}/games`, {
    method: 'GET',
    headers: token ? { Authorization: `Bearer ${token}` } : undefined,
  });
  return unwrap(resp);
}

export async function updateAdminGames(adminId: number, games: Array<{ gameId: string; envs: string[] }>) {
  const token = typeof window !== 'undefined' ? localStorage.getItem('token') : '';
  return request<void>(`/api/v1/admin/${adminId}/games`, {
    method: 'PUT',
    data: { games },
    headers: token ? { Authorization: `Bearer ${token}` } : undefined,
  });
}