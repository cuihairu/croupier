import { request } from '@umijs/max';
import { listAudit } from './audit';

jest.mock('@umijs/max', () => ({ request: jest.fn() }));

const mockedRequest = request as jest.MockedFunction<typeof request>;

describe('audit API adapter', () => {
  beforeEach(() => mockedRequest.mockReset());

  it('maps list filters to the backend query params (size/limit fall back into pageSize)', async () => {
    mockedRequest.mockResolvedValue({ items: [], total: 0, page: 3, pageSize: 10 });

    await listAudit({ actor: 'admin', size: 10, page: 3, kinds: 'a,b' });

    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/audit', {
      params: expect.objectContaining({ actor: 'admin', pageSize: 10, page: 3, kinds: 'a,b' }),
    });
  });

  it('normalizes raw audit items into view-model events with metadata fallbacks', async () => {
    mockedRequest.mockResolvedValue({
      items: [
        {
          id: '1',
          action: 'player.update',
          userId: 'admin',
          target: 'player_1001',
          createdAt: '2026-09-13T00:00:00Z',
          hash: 'h1',
          prevHash: 'h0',
          traceId: 'trace-1',
          gameId: 'demo_game',
          env: 'prod',
          metadata: { ip: '10.0.0.1', userAgent: 'ua/1' },
        },
      ],
      total: 1,
      page: 1,
      pageSize: 20,
    });

    const res = await listAudit();

    expect(res.events).toHaveLength(1);
    expect(res.events[0]).toMatchObject({
      time: '2026-09-13T00:00:00Z',
      kind: 'player.update',
      actor: 'admin',
      target: 'player_1001',
      hash: 'h1',
      prev: 'h0',
    });
    // 顶层 traceId/gameId/env 在 metadata 缺失时回填进 meta
    expect(res.events[0].meta).toMatchObject({
      traceId: 'trace-1',
      gameId: 'demo_game',
      env: 'prod',
      ip: '10.0.0.1',
      ua: 'ua/1',
      userAgent: 'ua/1',
      ipRegion: '',
    });
  });

  it('prefers metadata values over top-level fields and tolerates empty items', async () => {
    mockedRequest.mockResolvedValue({ items: null });

    const res = await listAudit({ limit: 5 });

    expect(res).toEqual({ events: [], total: 0, page: 1, pageSize: 5 });
  });

  it('item 全字段缺失时空值兜底（normalizeAuditEvent 全部 ?? 右侧）', async () => {
    mockedRequest.mockResolvedValue({ items: [{}] });

    const res = await listAudit();

    expect(res.events[0]).toEqual({
      id: '',
      time: '',
      kind: '',
      actor: '',
      target: '',
      hash: '',
      prev: '',
      // metadata 与顶层字段均缺失：traceId/gameId/env/ua 等回退 undefined（键视同不存在）
      meta: { ipRegion: '' },
    });
  });

  it('response 缺分页字段时 pageSize 按 params.size 回退', async () => {
    mockedRequest.mockResolvedValue({ items: [] });

    const res = await listAudit({ size: 7 });

    expect(res.pageSize).toBe(7);
  });
});
