import { request } from '@umijs/max';

export async function loginAuth(params: { username: string; password: string }) {
  return request<{ token: string; user: { username: string; roles: string[] } }>('/api/auth/login', { method: 'POST', data: params });
}

export async function fetchMe() {
  const token = typeof window !== 'undefined' ? localStorage.getItem('token') : '';
  // Pass Authorization explicitly to avoid any interceptor timing issues
  return request<{ username: string; roles: string[] }>('/api/auth/me', {
    headers: token ? { Authorization: `Bearer ${token}` } : undefined,
  });
}
