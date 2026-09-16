import { request } from '@umijs/max';
import {
  getAdminFunctionPermissions,
  listPendingFunctions,
  publishPendingFunction,
  setAdminFunctionPermissions,
} from './admin';

jest.mock('@umijs/max', () => ({ request: jest.fn() }));

const mockedRequest = request as jest.MockedFunction<typeof request>;

describe('admin pending-function API adapters', () => {
  beforeEach(() => mockedRequest.mockReset());

  it('normalizes pending rows through normalizeLocalizedText (raw string forms included)', async () => {
    mockedRequest.mockResolvedValue({
      pending: [
        {
          functionId: 'player.ban',
          displayName: '封禁玩家',
          summary: { 'zh-CN': '封禁', 'en-US': 'Ban' },
          suggestedPermissions: { verbs: ['write'], scopes: ['player'] },
        },
        { functionId: '', displayName: undefined },
      ],
    });

    const rows = await listPendingFunctions();

    expect(rows).toHaveLength(2);
    expect(rows[0]).toEqual({
      functionId: 'player.ban',
      displayName: { 'zh-CN': '封禁玩家' },
      summary: { 'zh-CN': '封禁', 'en-US': 'Ban' },
      suggestedPermissions: { verbs: ['write'], scopes: ['player'] },
    });
    // 空行兜底：functionId 空串、缺省本地化字段归一为 undefined
    expect(rows[1]).toEqual({
      functionId: '',
      displayName: undefined,
      summary: undefined,
      suggestedPermissions: undefined,
    });
  });

  it('returns an empty list when the backend response has no pending field', async () => {
    mockedRequest.mockResolvedValue({});

    await expect(listPendingFunctions()).resolves.toEqual([]);
  });

  it('URL-encodes function ids on publish', async () => {
    mockedRequest.mockResolvedValue(undefined);

    await publishPendingFunction('mail/batch_send');

    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/functions/mail%2Fbatch_send/publish', {
      method: 'POST',
    });
  });

  it('reads permissions with a record-shaped fallback', async () => {
    mockedRequest.mockResolvedValue({});

    await expect(getAdminFunctionPermissions('player.ban')).resolves.toEqual({});
    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/functions/player.ban/permissions', {
      method: 'GET',
    });

    mockedRequest.mockResolvedValue({ permissions: { 'player:read': true } });
    await expect(getAdminFunctionPermissions('player.ban')).resolves.toEqual({
      'player:read': true,
    });
  });

  it('writes permissions via PUT with the body payload', async () => {
    mockedRequest.mockResolvedValue(undefined);

    await setAdminFunctionPermissions('player.ban', { 'player:write': true });

    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/functions/player.ban/permissions', {
      method: 'PUT',
      data: { 'player:write': true },
    });
  });
});
