import { request } from '@umijs/max';

export type MeProfile = {
  id?: number;
  username: string;
  nickname?: string;
  display_name?: string;
  email?: string;
  phone?: string;
  avatar?: string;
  active?: boolean;
  roles?: string[];
  created_at?: string;
  updated_at?: string;
  last_login_at?: string;
};

export type ProfileGame = {
  game_id?: string;
  gameId?: string;
  game_name?: string;
  gameName?: string;
  envs?: string[];
  permissions?: string[];
};

export type ProfilePermission = {
  resource: string;
  actions: string[];
  game_id?: string;
  gameId?: string;
  env?: string;
};

export async function getMyProfile() {
  return request<MeProfile>('/api/v1/profile');
}

export async function getMyGames() {
  return request<{ games: ProfileGame[] }>('/api/v1/profile/games');
}

export async function getMyPermissions(params?: { game_id?: string; env?: string }) {
  return request<{ permissions: ProfilePermission[]; admin?: boolean; roles?: string[] }>(
    '/api/v1/profile/permissions',
    { params },
  );
}

export async function updateMyProfile(body: { display_name?: string; email?: string; phone?: string }) {
  return request<void>('/api/v1/profile', { method: 'PUT', data: body });
}

export async function changeMyPassword(body: { current: string; password: string }) {
  return request<void>('/api/v1/profile/password', { method: 'PUT', data: body });
}
