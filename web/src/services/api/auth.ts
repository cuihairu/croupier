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
  lastGameId?: string;
  lastEnv?: string;
};

// Compatibility projection used by runtime bootstrap.
// Canonical source is MeProfile from ./me.
export type CurrentUser = {
  username: string;
  nickname?: string;
  email?: string;
  roles: string[];
  /**
   * 头像地址。此前 CurrentUser 不带该字段，而 layout 的 avatarProps 读的是
   * initialState.currentUser.avatar —— 于是右上角头像恒为占位图标，永远不显示
   * 用户真实头像（docs/BUGS.md BUG-012）。
   */
  avatar?: string;
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
    // 透传头像，layout 顶栏才能显示真实头像而非恒定占位（BUG-012）。
    avatar: profile.avatar,
  };
}

export async function createSession(params: {
  username: string;
  password: string;
  /** MFA 已启用账号的二次验证码（TOTP 6 位）；登录 401+mfa_required 后重试携带 */
  totpCode?: string;
}): Promise<SessionResponse> {
  return request<SessionResponse>('/api/v1/auth/login', {
    method: 'POST',
    data: params,
    // 登录页自管错误展示（含 401+mfa_required 分支），跳过全局 401 跳转
    skipErrorHandler: true,
  });
}

// Runtime bootstrap projection over canonical profile API.
export async function fetchCurrentUser(): Promise<CurrentUser> {
  return toCurrentUser(await getMyProfile());
}

export async function fetchCurrentUserProfile(): Promise<MeProfile> {
  return getMyProfile();
}

export async function updateCurrentUserProfile(params: {
  nickname?: string;
  email?: string;
  phone?: string;
  avatar?: string;
}) {
  return updateMyProfile(params);
}

export async function changeCurrentUserPassword(params: {
  oldPassword: string;
  newPassword: string;
}) {
  return changeMyPassword({ current: params.oldPassword, password: params.newPassword });
}

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

export async function fetchCurrentUserGames(): Promise<CurrentUserGamesResponse> {
  return getMyGames();
}

// ---- 两步验证（TOTP MFA，仅本地账号） ----

export interface MfaStatus {
  enabled: boolean;
  local: boolean;
  /** 剩余未使用的备用恢复码数量（0 表示未启用或已全部用尽） */
  recoveryCodesRemaining?: number;
  /** 每次绑定签发的恢复码总数 */
  recoveryCodeTotal?: number;
}

export interface MfaSetupResult {
  secret: string;
  otpauthUrl: string;
  alreadyEnabled: boolean;
  /** 写入 otpauth:// 的签发方，前端用于二维码下方说明 */
  issuer?: string;
}

/** 确认启用后的一次性返回：明文恢复码只在此刻出现一次（库里仅存哈希） */
export interface MfaConfirmResult {
  recoveryCodes?: string[];
}

export async function fetchMfaStatus(): Promise<MfaStatus> {
  return request<MfaStatus>('/api/v1/auth/mfa/status', { method: 'GET' });
}

export async function setupMfa(): Promise<MfaSetupResult> {
  return request<MfaSetupResult>('/api/v1/auth/mfa/setup', { method: 'POST' });
}

export async function confirmMfa(code: string): Promise<MfaConfirmResult> {
  return request<MfaConfirmResult>('/api/v1/auth/mfa/confirm', {
    method: 'POST',
    data: { code },
  });
}

export async function disableMfa(code: string, password: string): Promise<void> {
  await request('/api/v1/auth/mfa/disable', {
    method: 'POST',
    data: { code, password },
  });
}
