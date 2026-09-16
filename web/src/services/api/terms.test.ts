import { request } from '@umijs/max';
import type { TermItem } from './terms';
import { deleteTerm, listTerms, upsertTerm } from './terms';

jest.mock('@umijs/max', () => ({ request: jest.fn() }));

const mockedRequest = request as jest.MockedFunction<typeof request>;

describe('services/api/terms listTerms', () => {
  beforeEach(() => mockedRequest.mockReset());

  it('sends the domain filter and normalizes an array-shaped response', async () => {
    mockedRequest.mockResolvedValue([
      {
        id: 1,
        domain: 'resource',
        termKey: 'player',
        alias: '玩家',
        display: { 'zh-CN': '玩家', 'en-US': 'Player' },
        order: 3,
      },
      {
        id: 2,
        domain: 'operation',
        termKey: 'ban',
        alias: '封禁',
        // 裸串形态 → 双 locale 归一
        display: '封禁操作',
      },
      {
        id: 3,
        domain: 'resource',
        termKey: 'guild',
        alias: '公会',
        // 遗留短 key → BCP47 归一
        display: { zh: '公会', en_us: 'Guild' },
      },
      {
        id: 4,
        domain: 'operation',
        termKey: 'kick',
        alias: '踢出',
        // display 缺失 → undefined
      },
      {
        id: 5,
        domain: 'operation',
        termKey: 'mute',
        alias: '禁言',
        // zh/en 均为空 → undefined
        display: {},
      },
    ]);

    const items = await listTerms('operation');

    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/terms', {
      params: { domain: 'operation' },
    });
    expect(items).toEqual([
      {
        id: 1,
        domain: 'resource',
        termKey: 'player',
        alias: '玩家',
        display: { 'zh-CN': '玩家', 'en-US': 'Player' },
        order: 3,
      },
      {
        id: 2,
        domain: 'operation',
        termKey: 'ban',
        alias: '封禁',
        display: { 'zh-CN': '封禁操作' },
        order: undefined,
      },
      {
        id: 3,
        domain: 'resource',
        termKey: 'guild',
        alias: '公会',
        display: { 'zh-CN': '公会', 'en-US': 'Guild' },
        order: undefined,
      },
      {
        id: 4,
        domain: 'operation',
        termKey: 'kick',
        alias: '踢出',
        display: undefined,
        order: undefined,
      },
      {
        id: 5,
        domain: 'operation',
        termKey: 'mute',
        alias: '禁言',
        display: undefined,
        order: undefined,
      },
    ]);
  });

  it('sends empty params when no domain filter is given', async () => {
    mockedRequest.mockResolvedValue([]);

    await expect(listTerms()).resolves.toEqual([]);

    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/terms', { params: {} });
  });

  it('normalizes an items-envelope response', async () => {
    mockedRequest.mockResolvedValue({
      items: [{ domain: 'resource', termKey: 'player', alias: '玩家' }],
    });

    const items: TermItem[] = await listTerms('resource');

    expect(items).toEqual([
      {
        domain: 'resource',
        termKey: 'player',
        alias: '玩家',
        display: undefined,
        order: undefined,
      },
    ]);
  });

  it('falls back to an empty list when items is missing', async () => {
    mockedRequest.mockResolvedValue({});

    await expect(listTerms('resource')).resolves.toEqual([]);
  });

  it('falls back to an empty list when the response is undefined', async () => {
    mockedRequest.mockResolvedValue(undefined);

    await expect(listTerms('resource')).resolves.toEqual([]);
  });

  it('propagates request failures untouched', async () => {
    mockedRequest.mockRejectedValueOnce(new Error('500 terms boom'));

    await expect(listTerms('resource')).rejects.toThrow('500 terms boom');
  });
});

describe('services/api/terms upsertTerm', () => {
  beforeEach(() => mockedRequest.mockReset());

  it('sends the term payload via PUT unchanged', async () => {
    mockedRequest.mockResolvedValue({ ok: true });

    const payload: TermItem = {
      id: 9,
      domain: 'operation',
      termKey: 'grant',
      alias: '发放',
      display: { 'zh-CN': '发放', 'en-US': 'Grant' },
      order: 12,
    };

    await expect(upsertTerm(payload)).resolves.toEqual({ ok: true });

    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/terms', {
      method: 'PUT',
      data: payload,
    });
  });
});

describe('services/api/terms deleteTerm', () => {
  beforeEach(() => mockedRequest.mockReset());

  it('sends domain/alias as DELETE query params', async () => {
    mockedRequest.mockResolvedValue({ ok: true });

    await expect(deleteTerm('operation', 'grant')).resolves.toEqual({ ok: true });

    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/terms', {
      method: 'DELETE',
      params: { domain: 'operation', alias: 'grant' },
    });
  });

  it('propagates request failures untouched', async () => {
    mockedRequest.mockRejectedValueOnce(new Error('404 not found'));

    await expect(deleteTerm('operation', 'grant')).rejects.toThrow('404 not found');
  });
});
