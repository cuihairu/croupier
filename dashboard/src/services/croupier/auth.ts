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

// RESTful: 登录，返回 token + 用户信息
export async function createSession(params: { username: string; password: string }) {
  const resp = await request<ApiResponse<{ token: string; user: { username: string; roles: string[] } }>>('/api/v1/auth/login', { method: 'POST', data: params });
  return unwrap(resp);
}

// RESTful: 获取当前用户信息
export async function fetchCurrentUser() {
  const token = typeof window !== 'undefined' ? localStorage.getItem('token') : '';
  // Pass Authorization explicitly to avoid any interceptor timing issues
  const resp = await request<ApiResponse<{ username: string; roles: string[] }>>('/api/v1/profile', {
    headers: token ? { Authorization: `Bearer ${token}` } : undefined,
  });
  return unwrap(resp);
}

// RESTful: 获取当前用户资料
export async function fetchCurrentUserProfile() {
  const token = typeof window !== 'undefined' ? localStorage.getItem('token') : '';
  const resp = await request<ApiResponse<{ username: string; nickname: string; email: string; roles: string[] }>>('/api/v1/profile', {
    headers: token ? { Authorization: `Bearer ${token}` } : undefined,
  });
  return unwrap(resp);
}

// RESTful: 更新当前用户资料
export async function updateCurrentUserProfile(params: { nickname?: string; email?: string; phone?: string }) {
  const token = typeof window !== 'undefined' ? localStorage.getItem('token') : '';
  return request('/api/v1/profile', {
    method: 'PUT',
    data: params,
    headers: token ? { Authorization: `Bearer ${token}` } : undefined,
  });
}

// RESTful: 修改当前用户密码
export async function changeCurrentUserPassword(params: { oldPassword: string; newPassword: string }) {
  const token = typeof window !== 'undefined' ? localStorage.getItem('token') : '';
  return request('/api/v1/profile/password', {
    method: 'PUT',
    data: params,
    headers: token ? { Authorization: `Bearer ${token}` } : undefined,
  });
}

// RESTful: 获取当前用户权限
export async function fetchCurrentUserPermissions(params?: { gameId?: string; env?: string }) {
  const token = typeof window !== 'undefined' ? localStorage.getItem('token') : '';
  const resp = await request<ApiResponse<{ permissions: any[]; admin: boolean; roles: string[] }>>('/api/v1/profile/permissions', {
    method: 'GET',
    params,
    headers: token ? { Authorization: `Bearer ${token}` } : undefined,
  });
  return unwrap(resp);
}

// RESTful: 获取当前用户游戏权限
export async function fetchCurrentUserGames() {
  const token = typeof window !== 'undefined' ? localStorage.getItem('token') : '';
  const resp = await request<ApiResponse<{ games: any[] }>>('/api/v1/profile/games', {
    method: 'GET',
    headers: token ? { Authorization: `Bearer ${token}` } : undefined,
  });
  return unwrap(resp);
}

// 向后兼容的别名函数 (保持原有API调用方式)
export async function loginAuth(params: { username: string; password: string }) {
  return createSession(params);
}

export async function fetchMe() {
  return fetchCurrentUser();
}
