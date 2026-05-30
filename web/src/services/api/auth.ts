import { request } from '@umijs/max';
import {
  changeMyPassword,
  getMyGames,
  getMyPermissions,
  getMyProfile,
  updateMyProfile,
  type MeProfile,
  type ProfileGame,
  type ProfilePermission,
} from './me';

// Source: croupier/internal/api/auth/dto.go LoginRequest / LoginResponse
export type SessionUser = {
  username: string;
  nickname?: string;
  roles: string[];
};

// Source: croupier/internal/api/auth/dto.go LoginResponse
export type SessionResponse = {
  token: string;
  user: SessionUser;
};

// Compatibility projection used by runtime bootstrap.
// Canonical source is MeProfile from ./me.
export type CurrentUser = {
  username: string;
  nickname?: string;
  email?: string;
  roles: string[];
};

// Source: croupier/internal/api/profile/dto.go ProfilePermissionsResponse
export type CurrentUserPermissionsResponse = {
  permissions: ProfilePermission[];
  admin: boolean;
  roles: string[];
  permissionIDs?: string[];
};

// Source: croupier/internal/api/profile/dto.go ProfileGamesResponse
export type CurrentUserGamesResponse = {
  games: ProfileGame[];
};

function toCurrentUser(profile: MeProfile): CurrentUser {
  return {
    username: profile.username,
    nickname: profile.nickname || profile.displayName,
    email: profile.email,
    roles: profile.roles || [],
  };
}

export async function createSession(params: {
  username: string;
  password: string;
}): Promise<SessionResponse> {
  return request<SessionResponse>('/api/v1/auth/login', {
    method: 'POST',
    data: params,
  });
}

// Compatibility wrapper over canonical profile API.
export async function fetchCurrentUser(): Promise<CurrentUser> {
  return toCurrentUser(await getMyProfile());
}

// Compatibility wrapper over canonical profile API.
export async function fetchCurrentUserProfile(): Promise<MeProfile> {
  return getMyProfile();
}

// Compatibility wrapper over canonical profile API.
export async function updateCurrentUserProfile(params: {
  nickname?: string;
  email?: string;
  phone?: string;
  avatar?: string;
}) {
  return updateMyProfile(params);
}

// Compatibility wrapper over canonical profile API.
export async function changeCurrentUserPassword(params: {
  oldPassword: string;
  newPassword: string;
}) {
  return changeMyPassword({ current: params.oldPassword, password: params.newPassword });
}

// Compatibility wrapper over canonical profile API.
export async function fetchCurrentUserPermissions(params?: {
  gameId?: string;
  env?: string;
}): Promise<CurrentUserPermissionsResponse> {
  const resp = await getMyPermissions({ gameId: params?.gameId, env: params?.env });
  return {
    permissions: resp.permissions || [],
    admin: Boolean(resp.admin),
    roles: resp.roles || [],
    permissionIDs: resp.permissionIDs,
  };
}

// Compatibility wrapper over canonical profile API.
export async function fetchCurrentUserGames(): Promise<CurrentUserGamesResponse> {
  return getMyGames();
}

// 向后兼容的别名函数
export async function loginAuth(params: { username: string; password: string }) {
  return createSession(params);
}

export async function fetchMe() {
  return fetchCurrentUser();
}
