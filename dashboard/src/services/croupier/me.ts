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
  return request<MeProfile>('/api/v1/profile');
}

export async function updateMyProfile(body: { display_name?: string; email?: string; phone?: string }) {
  return request<void>('/api/v1/profile', { method: 'PUT', data: body });
}

export async function changeMyPassword(body: { current: string; password: string }) {
  return request<void>('/api/v1/profile/password', { method: 'PUT', data: body });
}