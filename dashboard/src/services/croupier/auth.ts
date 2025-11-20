import { request } from '@umijs/max';

// RESTful: 创建会话而不是登录
export async function createSession(params: { username: string; password: string }) {
  return request<{ token: string; user: { username: string; roles: string[] } }>('/api/auth/sessions', { method: 'POST', data: params });
}

// RESTful: 获取当前用户信息
export async function fetchCurrentUser() {
  const token = typeof window !== 'undefined' ? localStorage.getItem('token') : '';
  // Pass Authorization explicitly to avoid any interceptor timing issues
  return request<{ username: string; roles: string[] }>('/api/users/current', {
    headers: token ? { Authorization: `Bearer ${token}` } : undefined,
  });
}

// RESTful: 获取当前用户资料
export async function fetchCurrentUserProfile() {
  const token = typeof window !== 'undefined' ? localStorage.getItem('token') : '';
  return request<{ username: string; display_name: string; email: string; roles: string[] }>('/api/users/current/profile', {
    headers: token ? { Authorization: `Bearer ${token}` } : undefined,
  });
}

// RESTful: 更新当前用户资料
export async function updateCurrentUserProfile(params: { display_name?: string; email?: string; phone?: string }) {
  const token = typeof window !== 'undefined' ? localStorage.getItem('token') : '';
  return request('/api/users/current/profile', {
    method: 'PUT',
    data: params,
    headers: token ? { Authorization: `Bearer ${token}` } : undefined,
  });
}

// RESTful: 修改当前用户密码
export async function changeCurrentUserPassword(params: { old_password: string; new_password: string }) {
  const token = typeof window !== 'undefined' ? localStorage.getItem('token') : '';
  return request('/api/users/current/password', {
    method: 'PUT',
    data: params,
    headers: token ? { Authorization: `Bearer ${token}` } : undefined,
  });
}

// RESTful: 获取当前用户游戏权限
export async function fetchCurrentUserGames() {
  const token = typeof window !== 'undefined' ? localStorage.getItem('token') : '';
  return request('/api/users/current/games', {
    headers: token ? { Authorization: `Bearer ${token}` } : undefined,
  });
}

// 向后兼容的别名函数 (保持原有API调用方式)
export async function loginAuth(params: { username: string; password: string }) {
  return createSession(params);
}

export async function fetchMe() {
  return fetchCurrentUser();
}
