import { request } from '@umijs/max';
import {
  changeMyPassword,
  getMyGames,
  getMyPermissions,
  getMyProfile,
  updateMyProfile,
} from './me';
import {
  changeCurrentUserPassword,
  confirmMfa,
  createSession,
  disableMfa,
  fetchCurrentUser,
  fetchCurrentUserGames,
  fetchCurrentUserPermissions,
  fetchCurrentUserProfile,
  fetchMfaStatus,
  setupMfa,
  updateCurrentUserProfile,
} from './auth';

jest.mock('@umijs/max', () => ({ request: jest.fn() }));
jest.mock('./me');

const mockedRequest = request as jest.MockedFunction<typeof request>;
const mockedGetMyProfile = jest.mocked(getMyProfile);
const mockedUpdateMyProfile = jest.mocked(updateMyProfile);
const mockedChangeMyPassword = jest.mocked(changeMyPassword);
const mockedGetMyPermissions = jest.mocked(getMyPermissions);
const mockedGetMyGames = jest.mocked(getMyGames);

// getMyPermissions 的真实实现恒返回 permissions: ProfilePermission[]；
// auth.ts 的 `resp.permissions || []` 是对后端契约外的防御，需要越界形态触发。
type PermissionsResp = Awaited<ReturnType<typeof getMyPermissions>>;

describe('auth session & profile API adapters', () => {
  beforeEach(() => {
    mockedRequest.mockReset();
    mockedGetMyProfile.mockReset();
    mockedUpdateMyProfile.mockReset();
    mockedChangeMyPassword.mockReset();
    mockedGetMyPermissions.mockReset();
    mockedGetMyGames.mockReset();
  });

  describe('createSession', () => {
    it('POSTs credentials with the login-local error handler flag', async () => {
      mockedRequest.mockResolvedValue({
        token: 'tok',
        user: { username: 'admin', roles: ['admin'] },
      });

      const resp = await createSession({ username: 'admin', password: 'pw' });

      expect(resp.token).toBe('tok');
      expect(mockedRequest).toHaveBeenCalledWith('/api/v1/auth/login', {
        method: 'POST',
        data: { username: 'admin', password: 'pw' },
        skipErrorHandler: true,
      });
    });

    it('passes the TOTP second factor through when retrying after mfa_required', async () => {
      mockedRequest.mockResolvedValue({
        token: 'tok',
        user: { username: 'admin', roles: ['admin'] },
        lastGameId: 'demo',
        lastEnv: 'prod',
      });

      const resp = await createSession({ username: 'admin', password: 'pw', totpCode: '123456' });

      expect(resp.lastGameId).toBe('demo');
      expect(mockedRequest).toHaveBeenCalledWith('/api/v1/auth/login', {
        method: 'POST',
        data: { username: 'admin', password: 'pw', totpCode: '123456' },
        skipErrorHandler: true,
      });
    });
  });

  describe('fetchCurrentUser (bootstrap projection)', () => {
    it('projects the canonical profile onto CurrentUser', async () => {
      mockedGetMyProfile.mockResolvedValue({
        username: 'admin',
        nickname: '管理员',
        email: 'admin@example.com',
        roles: ['ops'],
      });

      await expect(fetchCurrentUser()).resolves.toEqual({
        username: 'admin',
        nickname: '管理员',
        email: 'admin@example.com',
        roles: ['ops'],
      });
    });

    it('falls back to displayName when nickname is absent', async () => {
      mockedGetMyProfile.mockResolvedValue({
        username: 'admin',
        displayName: '展示名',
      });

      await expect(fetchCurrentUser()).resolves.toEqual({
        username: 'admin',
        nickname: '展示名',
        email: undefined,
        roles: [],
      });
    });

    it('defaults roles to an empty list and leaves nickname undefined', async () => {
      mockedGetMyProfile.mockResolvedValue({ username: 'admin' });

      await expect(fetchCurrentUser()).resolves.toEqual({
        username: 'admin',
        nickname: undefined,
        email: undefined,
        roles: [],
      });
    });
  });

  describe('profile passthrough', () => {
    it('fetchCurrentUserProfile returns the canonical profile untouched', async () => {
      const profile = { username: 'admin', nickname: '管理员', roles: ['ops'] };
      mockedGetMyProfile.mockResolvedValue(profile);

      await expect(fetchCurrentUserProfile()).resolves.toEqual(profile);
    });

    it('updateCurrentUserProfile forwards the editable fields', async () => {
      mockedUpdateMyProfile.mockResolvedValue(undefined);

      const body = { nickname: '新昵称', email: 'new@example.com', phone: '13800000000' };
      await updateCurrentUserProfile(body);

      expect(mockedUpdateMyProfile).toHaveBeenCalledWith(body);
    });

    it('changeCurrentUserPassword maps old/new onto the canonical payload', async () => {
      mockedChangeMyPassword.mockResolvedValue(undefined);

      await changeCurrentUserPassword({ oldPassword: 'old', newPassword: 'new' });

      expect(mockedChangeMyPassword).toHaveBeenCalledWith({ current: 'old', password: 'new' });
    });

    it('fetchCurrentUserGames returns the games response untouched', async () => {
      const games = { games: [{ gameId: 'demo', envs: ['prod'], permissions: [] }] };
      mockedGetMyGames.mockResolvedValue(games);

      await expect(fetchCurrentUserGames()).resolves.toEqual(games);
    });
  });

  describe('fetchCurrentUserPermissions', () => {
    it('normalizes missing fields to empty collections', async () => {
      mockedGetMyPermissions.mockResolvedValue({
        permissions: undefined,
        admin: undefined,
        roles: undefined,
      } as unknown as PermissionsResp);

      await expect(fetchCurrentUserPermissions()).resolves.toEqual({
        permissions: [],
        admin: false,
        roles: [],
        permissionIDs: undefined,
      });
      // params 缺省时仍以显式 undefined 字段调用 canonical API
      expect(mockedGetMyPermissions).toHaveBeenCalledWith({
        gameId: undefined,
        env: undefined,
      });
    });

    it('passes scope filters through and keeps populated fields', async () => {
      mockedGetMyPermissions.mockResolvedValue({
        permissions: [{ resource: 'player', actions: ['read'] }],
        admin: true,
        roles: ['ops'],
        permissionIDs: ['player:read'],
      });

      await expect(fetchCurrentUserPermissions({ gameId: 'demo', env: 'prod' })).resolves.toEqual({
        permissions: [{ resource: 'player', actions: ['read'] }],
        admin: true,
        roles: ['ops'],
        permissionIDs: ['player:read'],
      });
      expect(mockedGetMyPermissions).toHaveBeenCalledWith({ gameId: 'demo', env: 'prod' });
    });
  });

  describe('MFA endpoints', () => {
    it('fetchMfaStatus GETs the status endpoint', async () => {
      mockedRequest.mockResolvedValue({ enabled: true, local: true });

      await expect(fetchMfaStatus()).resolves.toEqual({ enabled: true, local: true });
      expect(mockedRequest).toHaveBeenCalledWith('/api/v1/auth/mfa/status', { method: 'GET' });
    });

    it('setupMfa POSTs the setup endpoint', async () => {
      mockedRequest.mockResolvedValue({
        secret: 'SECRET',
        otpauthUrl: 'otpauth://totp/x',
        alreadyEnabled: false,
      });

      await expect(setupMfa()).resolves.toEqual({
        secret: 'SECRET',
        otpauthUrl: 'otpauth://totp/x',
        alreadyEnabled: false,
      });
      expect(mockedRequest).toHaveBeenCalledWith('/api/v1/auth/mfa/setup', { method: 'POST' });
    });

    it('confirmMfa POSTs the verification code', async () => {
      mockedRequest.mockResolvedValue(undefined);

      await confirmMfa('123456');

      expect(mockedRequest).toHaveBeenCalledWith('/api/v1/auth/mfa/confirm', {
        method: 'POST',
        data: { code: '123456' },
      });
    });

    it('disableMfa POSTs code plus password', async () => {
      mockedRequest.mockResolvedValue(undefined);

      await disableMfa('123456', 'pw');

      expect(mockedRequest).toHaveBeenCalledWith('/api/v1/auth/mfa/disable', {
        method: 'POST',
        data: { code: '123456', password: 'pw' },
      });
    });
  });
});
