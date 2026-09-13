import { request } from '@umijs/max';
import * as api from './permissions';

jest.mock('@umijs/max', () => ({ request: jest.fn() }));

const mockedRequest = request as jest.MockedFunction<typeof request>;

// 表格驱动的请求形状断言：[名称, 调用, 期望 URL, 期望 options（不含 Authorization）]
type Case = {
  name: string;
  call: () => Promise<unknown>;
  url: string;
  options: Record<string, unknown>;
};

const cases: Case[] = [
  {
    name: 'listPermissions',
    call: () => api.listPermissions({ page: 1, resource: 'player' }),
    url: '/api/v1/permissions',
    options: { params: { page: 1, resource: 'player' } },
  },
  {
    name: 'getPermission',
    call: () => api.getPermission('player:read'),
    url: '/api/v1/permissions/player:read',
    options: { method: 'GET' },
  },
  {
    name: 'listAdmins',
    call: () => api.listAdmins({ search: 'op', status: 1 }),
    url: '/api/v1/admin',
    options: { params: { search: 'op', status: 1 } },
  },
  {
    name: 'createAdmin',
    call: () => api.createAdmin({ username: 'u', password: 'p', roles: ['ops'] }),
    url: '/api/v1/admin',
    options: { method: 'POST', data: { username: 'u', password: 'p', roles: ['ops'] } },
  },
  {
    name: 'getAdmin',
    call: () => api.getAdmin(7),
    url: '/api/v1/admin/7',
    options: { method: 'GET' },
  },
  {
    name: 'updateAdmin',
    call: () => api.updateAdmin(7, { status: 0 }),
    url: '/api/v1/admin/7',
    options: { method: 'PUT', data: { status: 0 } },
  },
  {
    name: 'deleteAdmin',
    call: () => api.deleteAdmin(7),
    url: '/api/v1/admin/7',
    options: { method: 'DELETE' },
  },
  {
    name: 'resetAdminPassword',
    call: () => api.resetAdminPassword(7, 'new-pw'),
    url: '/api/v1/admin/7/password-reset',
    options: { method: 'POST', data: { newPassword: 'new-pw' } },
  },
  {
    name: 'listRoles',
    call: () => api.listRoles({ category: 'ops' }),
    url: '/api/v1/roles',
    options: { params: { category: 'ops' } },
  },
  {
    name: 'createRole',
    call: () => api.createRole({ name: 'ops', permissions: ['player:read'] }),
    url: '/api/v1/roles',
    options: { method: 'POST', data: { name: 'ops', permissions: ['player:read'] } },
  },
  {
    name: 'getRole',
    call: () => api.getRole(3),
    url: '/api/v1/roles/3',
    options: { method: 'GET' },
  },
  {
    name: 'updateRole',
    call: () => api.updateRole(3, { name: 'ops2' }),
    url: '/api/v1/roles/3',
    options: { method: 'PUT', data: { name: 'ops2' } },
  },
  {
    name: 'deleteRole',
    call: () => api.deleteRole(3),
    url: '/api/v1/roles/3',
    options: { method: 'DELETE' },
  },
  {
    name: 'updateRolePermissions',
    call: () => api.updateRolePermissions(3, ['player:read']),
    url: '/api/v1/roles/3/permissions',
    options: { method: 'PUT', data: { permissions: ['player:read'] } },
  },
  {
    name: 'getAdminGames',
    call: () => api.getAdminGames(7),
    url: '/api/v1/admin/7/games',
    options: { method: 'GET' },
  },
  {
    name: 'updateAdminGames',
    call: () => api.updateAdminGames(7, [{ gameId: 'demo', envs: ['dev'] }]),
    url: '/api/v1/admin/7/games',
    options: { method: 'PUT', data: { games: [{ gameId: 'demo', envs: ['dev'] }] } },
  },
];

describe('permissions & role/admin API adapters', () => {
  beforeEach(() => {
    mockedRequest.mockReset().mockResolvedValue(undefined);
    (localStorage.getItem as unknown as jest.Mock).mockReturnValue('tok-abc');
  });

  it.each(cases)('$name hits the right URL, method and payload', async ({ call, url, options }) => {
    await call();
    expect(mockedRequest).toHaveBeenCalledTimes(1);
    const [calledUrl, calledOptions] = mockedRequest.mock.calls[0];
    expect(calledUrl).toBe(url);
    expect(calledOptions).toEqual({ ...options, headers: { Authorization: 'Bearer tok-abc' } });
  });

  it('omits the Authorization header when no token is stored', async () => {
    (localStorage.getItem as unknown as jest.Mock).mockReturnValue(null);

    await api.listPermissions();

    const [, options] = mockedRequest.mock.calls[0];
    expect(options.headers).toBeUndefined();
  });

  it('passes scope filters to the profile permissions and check endpoints', async () => {
    await api.getUserPermissions({ gameId: 'demo', env: 'prod' });
    expect(mockedRequest).toHaveBeenLastCalledWith('/api/v1/profile/permissions', {
      params: { gameId: 'demo', env: 'prod' },
    });

    await api.checkPermission({ resource: 'player', action: 'read' });
    expect(mockedRequest).toHaveBeenLastCalledWith('/api/v1/auth/check', {
      method: 'POST',
      data: { resource: 'player', action: 'read' },
    });

    await api.batchCheckPermissions([{ resource: 'player', action: 'write' }]);
    expect(mockedRequest).toHaveBeenLastCalledWith('/api/v1/auth/check/batch', {
      method: 'POST',
      data: { checks: [{ resource: 'player', action: 'write' }] },
    });
  });
});
