import { request } from '@umijs/max';
import type { NotificationChannelState } from '@/pages/Profile/shared';

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
  /**
   * 访问级别：full（持通配 = 全部权限）/ scoped（有显式权限）/ none（无）。
   * 修复前 permissions 对所有用户所有游戏恒为 []，admin 看不到任何权限
   * （docs/BUGS.md BUG-018），前端无从区分「全部」与「空」。
   */
  accessLevel?: 'full' | 'scoped' | 'none';
  /** 授权维度，固定 'role'：RBAC 不按游戏切分。 */
  permissionScope?: string;
};

// Canonical frontend permission DTO normalized from croupier/internal/api/profile/dto.go ProfilePermission.
export type ProfilePermission = {
  resource: string;
  actions: string[];
  gameId?: string;
  env?: string;
};

// Raw profile game from backend
type RawProfileGame = {
  gameId?: string;
  name?: string;
  gameName?: string;
  envs?: string[];
  envMeta?: Array<{ env?: string }>;
  permissions?: string[];
  accessLevel?: string;
  permissionScope?: string;
};

// Raw profile permission from backend
type RawProfilePermission = {
  resource?: string;
  actions?: string[];
  gameId?: string;
  env?: string;
};

// Raw profile from backend
type RawProfile = {
  id?: number;
  username?: string;
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
  profileInfo?: RawProfile;
};

// Normalize profile game payloads from backend DTO variants into one frontend shape.
function normalizeProfileGame(game: RawProfileGame): ProfileGame {
  return {
    gameId: game.gameId ?? game.name,
    gameName: game.gameName,
    envs: Array.isArray(game.envs)
      ? game.envs
      : Array.isArray(game.envMeta)
        ? (game.envMeta.map((env) => env?.env).filter(Boolean) as string[])
        : [],
    permissions: Array.isArray(game.permissions) ? game.permissions : [],
    accessLevel:
      game.accessLevel === 'full' || game.accessLevel === 'scoped' || game.accessLevel === 'none'
        ? game.accessLevel
        : undefined,
    permissionScope: game.permissionScope,
  };
}

// Normalize profile permission payloads from backend DTO variants into one frontend shape.
function normalizeProfilePermission(permission: RawProfilePermission): ProfilePermission {
  return {
    resource: permission.resource ?? '',
    actions: Array.isArray(permission.actions) ? permission.actions : [],
    gameId: permission.gameId,
    env: permission.env,
  };
}

// Normalize profile payloads from backend DTO variants into one frontend shape.
function normalizeMyProfile(profile: RawProfile): MeProfile {
  const source = profile?.profileInfo ?? profile ?? {};
  return {
    id: source?.id,
    username: source?.username ?? '',
    nickname: source?.nickname,
    displayName: source?.displayName ?? source?.nickname,
    email: source?.email,
    phone: source?.phone,
    avatar: source?.avatar,
    active: typeof source?.active === 'boolean' ? source.active : undefined,
    roles: Array.isArray(source?.roles) ? source.roles : [],
    createdAt: source?.createdAt,
    updatedAt: source?.updatedAt,
    lastLoginAt: source?.lastLoginAt,
  };
}

export async function getMyProfile() {
  const resp = await request<RawProfile>('/api/v1/profile');
  return normalizeMyProfile(resp);
}

export async function getMyGames() {
  const resp = await request<{ games?: RawProfileGame[] }>('/api/v1/profile/games');
  return {
    games: Array.isArray(resp?.games) ? resp.games.map(normalizeProfileGame) : [],
  };
}

/**
 * GET /api/v1/profile/permissions 的归一化结果。
 *
 * 显式声明返回类型：归一化后 role/permissionIds 一定存在（缺失会被补成
 * '' 与 []），但上游 request 的泛型把它们标成可选，不写出来的话调用方
 * （权限树）还得再判一次空。
 */
export type MyPermissions = {
  permissions: ProfilePermission[];
  admin?: boolean;
  roles?: string[];
  permissionIDs?: string[];
  rolePermissions?: Array<{ role: string; permissionIds: string[] }>;
  accessLevel?: string;
  permissionScope?: string;
  fullAccess?: boolean;
};

export async function getMyPermissions(params?: {
  gameId?: string;
  env?: string;
}): Promise<MyPermissions> {
  const query = params
    ? {
        gameId: params.gameId,
        env: params.env,
      }
    : undefined;
  const resp = await request<{
    permissions?: RawProfilePermission[];
    admin?: boolean;
    roles?: string[];
    permissionIDs?: string[];
    accessLevel?: string;
    permissionScope?: string;
    fullAccess?: boolean;
    rolePermissions?: Array<{ role?: string; permissionIds?: string[] }>;
  }>('/api/v1/profile/permissions', { params: query });

  // 先摘出要归一化的三个键，剩下的原样透传（后端新增字段自动到达前端）。
  const { permissions: rawPermissions, permissionIDs, rolePermissions, ...rest } = resp || {};

  return {
    ...rest,
    permissions: Array.isArray(rawPermissions)
      ? rawPermissions.map(normalizeProfilePermission)
      : [],
    // 只在响应真的带这两个字段时才写入，避免给「字段缺失」凭空注入空数组，
    // 那样会掩盖后端的契约变化。
    ...(Array.isArray(permissionIDs) ? { permissionIDs: sanitizePermissionIds(permissionIDs) } : {}),
    ...(Array.isArray(rolePermissions)
      ? {
          rolePermissions: rolePermissions.map((g) => ({
            role: String(g?.role ?? ''),
            permissionIds: sanitizePermissionIds(g?.permissionIds),
          })),
        }
      : {}),
  };
}

/**
 * 丢掉非法项，并剔除被误塞进列表的**角色名**。
 *
 * 后端已把两者分开（BUG-019），这里再兜一层：权限 id 必须是
 * `resource:action` 形态或纯通配 `*`；`viewer` / `admin` / `super_admin`
 * 这类角色名不含冒号，会被权限目录查不到，最终在树上变成一条永远查不到的
 * 假权限。
 */
function sanitizePermissionIds(ids: readonly unknown[] | undefined): string[] {
  const seen = new Set<string>();
  const out: string[] = [];
  for (const raw of ids || []) {
    if (typeof raw !== 'string') continue;
    const id = raw.trim();
    // 纯通配 `*` 没有冒号，但它是真权限（configs/permissions.json 里就有）
    if (id === '' || !(id.includes(':') || id === '*')) continue;
    if (seen.has(id)) continue;
    seen.add(id);
    out.push(id);
  }
  return out;
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

// Persist the user's game/env scope selection to the server.
// Best-effort: called on scope change, errors are silently ignored.
export async function persistMyScope(gameId: string, env: string): Promise<void> {
  await request<void>('/api/v1/profile/scope', {
    method: 'PATCH',
    data: { gameId, env },
  });
}

/**
 * 拉取通知通道的真实状态。
 *
 * Source: croupier/internal/api/profile/notification_channels.go
 *        NotificationChannelsResponse
 */
export type MyNotificationChannelsResponse = {
  channels: NotificationChannelState[];
};

export async function fetchMyNotificationChannels(): Promise<MyNotificationChannelsResponse> {
  return request<MyNotificationChannelsResponse>('/api/v1/profile/notification-channels', {
    method: 'GET',
  });
}
