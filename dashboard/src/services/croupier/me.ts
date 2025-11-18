import { request } from '@umijs/max';

export type MeProfile = {
  username: string;
  display_name?: string;
  email?: string;
  phone?: string;
  active?: boolean;
  roles?: string[];
};

export async function getMyProfile() {
  return request<MeProfile>('/api/me/profile');
}

export async function updateMyProfile(body: { display_name?: string; email?: string; phone?: string }) {
  return request<void>('/api/me/profile', { method: 'PUT', data: body });
}

export async function changeMyPassword(body: { current: string; password: string }) {
  return request<void>('/api/me/password', { method: 'POST', data: body });
}

