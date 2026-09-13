import { request } from '@umijs/max';
import {
  changeMyPassword,
  getMyGames,
  getMyPermissions,
  getMyProfile,
  persistMyScope,
  updateMyProfile,
} from './me';

jest.mock('@umijs/max', () => ({ request: jest.fn() }));

const mockedRequest = request as jest.MockedFunction<typeof request>;

describe('services/api/me getMyProfile', () => {
  beforeEach(() => mockedRequest.mockReset());

  it('normalizes the profileInfo envelope into one profile shape', async () => {
    mockedRequest.mockResolvedValue({
      profileInfo: {
        id: 7,
        username: 'admin',
        nickname: '管理员',
        displayName: '展示名',
        email: 'admin@example.com',
        phone: '13800000000',
        avatar: '/avatar.png',
        active: true,
        roles: ['ops', 'viewer'],
        createdAt: '2026-01-01T00:00:00Z',
        updatedAt: '2026-02-01T00:00:00Z',
        lastLoginAt: '2026-03-01T00:00:00Z',
      },
    });

    await expect(getMyProfile()).resolves.toEqual({
      id: 7,
      username: 'admin',
      nickname: '管理员',
      displayName: '展示名',
      email: 'admin@example.com',
      phone: '13800000000',
      avatar: '/avatar.png',
      active: true,
      roles: ['ops', 'viewer'],
      createdAt: '2026-01-01T00:00:00Z',
      updatedAt: '2026-02-01T00:00:00Z',
      lastLoginAt: '2026-03-01T00:00:00Z',
    });
    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/profile');
  });

  it('falls back to the top-level profile and repairs malformed fields', async () => {
    mockedRequest.mockResolvedValue({
      username: 'legacy',
      nickname: '旧昵称',
      // displayName 缺失 → 回退 nickname；active 非 boolean → undefined；roles 非数组 → []
      active: 'yes',
      roles: 'admin',
    });

    await expect(getMyProfile()).resolves.toEqual({
      id: undefined,
      username: 'legacy',
      nickname: '旧昵称',
      displayName: '旧昵称',
      email: undefined,
      phone: undefined,
      avatar: undefined,
      active: undefined,
      roles: [],
      createdAt: undefined,
      updatedAt: undefined,
      lastLoginAt: undefined,
    });
  });

  it('derives empty defaults from an empty object response', async () => {
    mockedRequest.mockResolvedValue({});

    await expect(getMyProfile()).resolves.toEqual({
      id: undefined,
      username: '',
      nickname: undefined,
      displayName: undefined,
      email: undefined,
      phone: undefined,
      avatar: undefined,
      active: undefined,
      roles: [],
      createdAt: undefined,
      updatedAt: undefined,
      lastLoginAt: undefined,
    });
  });

  it('derives empty defaults from an undefined (no-content) response', async () => {
    mockedRequest.mockResolvedValue(undefined);

    await expect(getMyProfile()).resolves.toEqual({
      id: undefined,
      username: '',
      nickname: undefined,
      displayName: undefined,
      email: undefined,
      phone: undefined,
      avatar: undefined,
      active: undefined,
      roles: [],
      createdAt: undefined,
      updatedAt: undefined,
      lastLoginAt: undefined,
    });
  });

  it('propagates request failures untouched', async () => {
    mockedRequest.mockRejectedValueOnce(new Error('401 unauthorized'));

    await expect(getMyProfile()).rejects.toThrow('401 unauthorized');
  });
});

describe('services/api/me getMyGames', () => {
  beforeEach(() => mockedRequest.mockReset());

  it('normalizes each game and repairs variant field shapes', async () => {
    mockedRequest.mockResolvedValue({
      games: [
        {
          gameId: 'demo',
          gameName: '演示',
          envs: ['prod', 'dev'],
          permissions: ['players:read'],
        },
        {
          // gameId 缺失 → 回退 name；envs 缺失 → 从 envMeta 提取（滤掉空值与 undefined 元素）
          name: 'legacy',
          envMeta: [{ env: 'prod' }, { env: undefined }, undefined],
          // permissions 非数组 → []
          permissions: 'not-an-array',
        },
      ],
    });

    await expect(getMyGames()).resolves.toEqual({
      games: [
        { gameId: 'demo', gameName: '演示', envs: ['prod', 'dev'], permissions: ['players:read'] },
        { gameId: 'legacy', gameName: undefined, envs: ['prod'], permissions: [] },
      ],
    });
    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/profile/games');
  });

  it('defaults envs and permissions to empty arrays when both env shapes are missing', async () => {
    mockedRequest.mockResolvedValue({ games: [{ gameId: 'demo' }] });

    await expect(getMyGames()).resolves.toEqual({
      games: [{ gameId: 'demo', gameName: undefined, envs: [], permissions: [] }],
    });
  });

  it('returns an empty games list when games is missing', async () => {
    mockedRequest.mockResolvedValue({});

    await expect(getMyGames()).resolves.toEqual({ games: [] });
  });

  it('returns an empty games list when the response is undefined', async () => {
    mockedRequest.mockResolvedValue(undefined);

    await expect(getMyGames()).resolves.toEqual({ games: [] });
  });
});

describe('services/api/me getMyPermissions', () => {
  beforeEach(() => mockedRequest.mockReset());

  it('sends the scope query params and spreads extra response fields', async () => {
    mockedRequest.mockResolvedValue({
      permissions: [
        { resource: 'players', actions: ['read', 'write'], gameId: 'demo', env: 'prod' },
      ],
      admin: true,
      roles: ['ops'],
      permissionIDs: ['players:read'],
    });

    await expect(getMyPermissions({ gameId: 'demo', env: 'prod' })).resolves.toEqual({
      permissions: [
        { resource: 'players', actions: ['read', 'write'], gameId: 'demo', env: 'prod' },
      ],
      admin: true,
      roles: ['ops'],
      permissionIDs: ['players:read'],
    });
    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/profile/permissions', {
      params: { gameId: 'demo', env: 'prod' },
    });
  });

  it('normalizes malformed permission rows', async () => {
    mockedRequest.mockResolvedValue({
      permissions: [
        // resource 缺失 → ''；actions 非数组 → []
        { actions: 'not-an-array' },
      ],
    });

    await expect(getMyPermissions({ gameId: 'demo' })).resolves.toEqual({
      permissions: [{ resource: '', actions: [], gameId: undefined, env: undefined }],
    });
    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/profile/permissions', {
      params: { gameId: 'demo', env: undefined },
    });
  });

  it('omits params entirely when no scope query is given', async () => {
    mockedRequest.mockResolvedValue({});

    await expect(getMyPermissions()).resolves.toEqual({ permissions: [] });
    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/profile/permissions', {
      params: undefined,
    });
  });

  it('falls back to an empty permissions list for an undefined response', async () => {
    mockedRequest.mockResolvedValue(undefined);

    await expect(getMyPermissions()).resolves.toEqual({ permissions: [] });
  });

  it('propagates request failures untouched', async () => {
    mockedRequest.mockRejectedValueOnce(new Error('500 boom'));

    await expect(getMyPermissions()).rejects.toThrow('500 boom');
  });
});

describe('services/api/me updateMyProfile', () => {
  beforeEach(() => mockedRequest.mockReset());

  it('sends a PUT with the full body payload', async () => {
    mockedRequest.mockResolvedValue(undefined);

    await updateMyProfile({
      nickname: '新昵称',
      email: 'new@example.com',
      phone: '13900000000',
      avatar: '/new.png',
    });

    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/profile', {
      method: 'PUT',
      data: {
        nickname: '新昵称',
        email: 'new@example.com',
        phone: '13900000000',
        avatar: '/new.png',
      },
    });
  });

  it('falls back to displayName when nickname is blank', async () => {
    mockedRequest.mockResolvedValue(undefined);

    await updateMyProfile({ displayName: '展示名' });

    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/profile', {
      method: 'PUT',
      data: {
        nickname: '展示名',
        email: undefined,
        phone: undefined,
        avatar: undefined,
      },
    });
  });
});

describe('services/api/me changeMyPassword', () => {
  beforeEach(() => mockedRequest.mockReset());

  it('maps current/password onto oldPassword/newPassword via PUT', async () => {
    mockedRequest.mockResolvedValue(undefined);

    await changeMyPassword({ current: 'old-pass', password: 'new-pass' });

    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/profile/password', {
      method: 'PUT',
      data: { oldPassword: 'old-pass', newPassword: 'new-pass' },
    });
  });
});

describe('services/api/me persistMyScope', () => {
  beforeEach(() => mockedRequest.mockReset());

  it('persists the selected scope via PATCH', async () => {
    mockedRequest.mockResolvedValue(undefined);

    await expect(persistMyScope('demo', 'prod')).resolves.toBeUndefined();

    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/profile/scope', {
      method: 'PATCH',
      data: { gameId: 'demo', env: 'prod' },
    });
  });

  it('propagates failures to the caller', async () => {
    mockedRequest.mockRejectedValueOnce(new Error('409 conflict'));

    await expect(persistMyScope('demo', 'prod')).rejects.toThrow('409 conflict');
  });
});
