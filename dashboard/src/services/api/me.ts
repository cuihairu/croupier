import { request } from '@umijs/max';

// Canonical frontend profile DTO normalized from croupier/internal/api/profile/dto.go ProfileGetResponse.
export type MeProfile = {
  id?: number;
  username: string;
  nickname?: string;
  displayName?: string;
  email?: string;
  phone?: string;
  avatar?: string;
  active?: boolean;
  roles?: string[];
  createdAt?: string;
  updatedAt?: string;
  lastLoginAt?: string;
};

// Canonical frontend game-scope DTO normalized from croupier/internal/api/profile/dto.go ProfileGame.
export type ProfileGame = {
  gameId?: string;
  gameName?: string;
  envs?: string[];
  permissions?: string[];
};

// Canonical frontend permission DTO normalized from croupier/internal/api/profile/dto.go ProfilePermission.
export type ProfilePermission = {
  resource: string;
  actions: string[];
  gameId?: string;
  env?: string;
};

// Normalize profile game payloads from backend DTO variants into one frontend shape.
function normalizeProfileGame(game: any): ProfileGame {
  return {
    gameId: game?.gameId ?? game?.game_id ?? game?.name,
    gameName: game?.gameName ?? game?.game_name ?? game?.display_name ?? game?.alias_name,
    envs: Array.isArray(game?.envs)
      ? game.envs
      : Array.isArray(game?.envMeta)
      ? game.envMeta.map((env: any) => env?.env).filter(Boolean)
      : [],
    permissions: Array.isArray(game?.permissions) ? game.permissions : [],
  };
}

// Normalize profile permission payloads from backend DTO variants into one frontend shape.
function normalizeProfilePermission(permission: any): ProfilePermission {
  return {
    resource: permission?.resource ?? '',
    actions: Array.isArray(permission?.actions) ? permission.actions : [],
    gameId: permission?.gameId ?? permission?.game_id,
    env: permission?.env,
  };
}

// Normalize profile payloads from backend DTO variants into one frontend shape.
function normalizeMyProfile(profile: any): MeProfile {
  const source = profile?.profileInfo ?? profile ?? {};
  return {
    id: source?.id,
    username: source?.username ?? '',
    nickname: source?.nickname,
    displayName: source?.displayName ?? source?.display_name ?? source?.nickname,
    email: source?.email,
    phone: source?.phone,
    avatar: source?.avatar,
    active: typeof source?.active === 'boolean' ? source.active : undefined,
    roles: Array.isArray(source?.roles) ? source.roles : [],
    createdAt: source?.createdAt ?? source?.created_at,
    updatedAt: source?.updatedAt ?? source?.updated_at,
    lastLoginAt: source?.lastLoginAt ?? source?.last_login_at,
  };
}

export async function getMyProfile() {
  const resp = await request<any>('/api/v1/profile');
  return normalizeMyProfile(resp);
}

export async function getMyGames() {
  const resp = await request<{ games?: ProfileGame[] }>('/api/v1/profile/games');
  return {
    games: Array.isArray(resp?.games) ? resp.games.map(normalizeProfileGame) : [],
  };
}

export async function getMyPermissions(params?: { gameId?: string; env?: string }) {
  const query = params
    ? {
        gameId: params.gameId,
        env: params.env,
      }
    : undefined;
  const resp = await request<{
    permissions?: ProfilePermission[];
    admin?: boolean;
    roles?: string[];
    permissionIDs?: string[];
  }>('/api/v1/profile/permissions', { params: query });
  return {
    ...resp,
    permissions: Array.isArray(resp?.permissions)
      ? resp.permissions.map(normalizeProfilePermission)
      : [],
  };
}

export async function updateMyProfile(body: {
  displayName?: string;
  nickname?: string;
  email?: string;
  phone?: string;
  avatar?: string;
}) {
  return request<void>('/api/v1/profile', {
    method: 'PUT',
    data: {
      nickname: body.nickname || body.displayName,
      email: body.email,
      phone: body.phone,
      avatar: body.avatar,
    },
  });
}

export async function changeMyPassword(body: { current: string; password: string }) {
  return request<void>('/api/v1/profile/password', {
    method: 'PUT',
    data: {
      oldPassword: body.current,
      newPassword: body.password,
    },
  });
}
