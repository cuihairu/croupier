import { request } from '@umijs/max';
import {
  batchGetFunctionOpenAPI,
  batchUpdateFunctions,
  cancelFunctionCall,
  getFunctionCall,
  getFunctionCalls,
  getFunctionDetail,
  getFunctionInstances,
  getFunctionOpenAPIDetail,
  getFunctionResources,
  getFunctionSummary,
  getFunctionTags,
  getRegistryServices,
  normalizeFunctionInstance,
  normalizeFunctionSummary,
  normalizeLocalizedText,
  searchFunctions,
} from './functions-enhanced';

jest.mock('@umijs/max', () => ({
  request: jest.fn(),
  getIntl: jest.fn(() => ({
    formatMessage: (descriptor: { defaultMessage?: string }) => descriptor.defaultMessage ?? '',
  })),
}));

const mockedRequest = request as jest.MockedFunction<typeof request>;

describe('normalizeFunctionSummary', () => {
  it('normalizes backend locale keys used by FunctionSpec summaries', () => {
    expect(
      normalizeFunctionSummary({
        id: 'player.list',
        status: 1,
        displayName: { 'zh-CN': '玩家列表', 'en-US': 'Player List' },
        summary: { 'zh-CN': '查询玩家', 'en-US': 'List players' },
      }),
    ).toMatchObject({
      id: 'player.list',
      enabled: true,
      displayName: { 'zh-CN': '玩家列表', 'en-US': 'Player List' },
      summary: { 'zh-CN': '查询玩家', 'en-US': 'List players' },
    });
  });

  it('does not claim an enabled function when status is disabled', () => {
    expect(normalizeFunctionSummary({ id: 'player.delete', status: 0 }).enabled).toBe(false);
  });
});

describe('getFunctionInstances', () => {
  beforeEach(() => mockedRequest.mockReset());

  it('encodes function ids and normalizes the backend envelope', async () => {
    mockedRequest.mockResolvedValue({
      items: [
        {
          functionId: 'player/read',
          agentId: 'agent-1',
          serviceId: 'service-1',
          status: 'active',
        },
      ],
      total: 1,
    });

    await expect(getFunctionInstances({ functionId: 'player/read' })).resolves.toEqual({
      instances: [
        expect.objectContaining({
          functionId: 'player/read',
          agentId: 'agent-1',
          serviceId: 'service-1',
          status: 'running',
          ownerInstance: '',
        }),
      ],
      total: 1,
    });
    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/functions/player%2Fread/instances', {
      params: { gameId: undefined },
    });
  });

  it('passes through the cross-instance owner annotation for remote entries', async () => {
    mockedRequest.mockResolvedValue({
      instances: [
        {
          functionId: 'player/read',
          agentId: 'agent-remote',
          serviceId: 'service-remote',
          status: 'active',
          ownerInstance: 'server2',
        },
      ],
      total: 1,
    });

    const { instances } = await getFunctionInstances();
    // 远端条目的归属标注不得在 normalize 层被剥掉（跨实例聚合的 UI 呈现依据）。
    expect(instances[0].ownerInstance).toBe('server2');
  });

  it('propagates scope or permission errors instead of returning fake empty data', async () => {
    mockedRequest.mockRejectedValue(new Error('scope_required'));

    await expect(getFunctionInstances()).rejects.toThrow('scope_required');
  });
});

describe('normalizeLocalizedText', () => {
  it('returns undefined for absent or blank values', () => {
    expect(normalizeLocalizedText(undefined)).toBeUndefined();
    expect(normalizeLocalizedText('')).toBeUndefined();
    expect(normalizeLocalizedText('   ')).toBeUndefined();
  });

  it('trims bare strings into symmetric zh-CN/en-US entries', () => {
    expect(normalizeLocalizedText('  封禁  ')).toEqual({ 'zh-CN': '封禁', 'en-US': '封禁' });
  });

  it('keeps canonical BCP47 keys as-is', () => {
    expect(normalizeLocalizedText({ 'zh-CN': '中', 'en-US': 'EN' })).toEqual({
      'zh-CN': '中',
      'en-US': 'EN',
    });
  });

  it('maps legacy short keys onto BCP47 keys', () => {
    expect(normalizeLocalizedText({ zh: '中', en: 'EN' })).toEqual({
      'zh-CN': '中',
      'en-US': 'EN',
    });
    expect(normalizeLocalizedText({ zh_cn: '中', en_us: 'EN' })).toEqual({
      'zh-CN': '中',
      'en-US': 'EN',
    });
  });

  it('prefers canonical keys over legacy variants', () => {
    expect(normalizeLocalizedText({ 'zh-CN': 'canonical', zh: 'legacy' })).toEqual({
      'zh-CN': 'canonical',
    });
    expect(normalizeLocalizedText({ zh: 'legacy', zh_cn: 'older' })).toEqual({
      'zh-CN': 'legacy',
    });
    expect(normalizeLocalizedText({ 'en-US': 'canonical-en', en: 'legacy-en' })).toEqual({
      'en-US': 'canonical-en',
    });
  });

  it('emits single-locale objects when only one side exists', () => {
    expect(normalizeLocalizedText({ 'en-US': 'EN' })).toEqual({ 'en-US': 'EN' });
    expect(normalizeLocalizedText({ zh: '中' })).toEqual({ 'zh-CN': '中' });
  });

  it('returns undefined when neither locale has text', () => {
    expect(normalizeLocalizedText({})).toBeUndefined();
    expect(normalizeLocalizedText({ fr: 'Français' })).toBeUndefined();
  });
});

describe('normalizeFunctionInstance', () => {
  it('maps every canonical field through unchanged', () => {
    expect(
      normalizeFunctionInstance({
        agentId: 'a-1',
        agentName: 'Agent One',
        serviceId: 's-1',
        providerId: 'p-1',
        addr: '127.0.0.1:19091',
        version: 'v2',
        sdkName: 'croupier-go',
        sdkLang: 'go',
        sdkVersion: '1.1',
        functionId: 'player.ban',
        status: 'running',
        lastHeartbeat: '2026-01-01T00:00:01Z',
        functionsCount: 3,
        healthy: true,
        lastSeen: '2026-01-01T00:00:02Z',
        gameId: 'demo',
        env: 'prod',
        ownerInstance: 'server2',
        metadata: { region: 'cn' },
      }),
    ).toEqual({
      agentId: 'a-1',
      agentName: 'Agent One',
      serviceId: 's-1',
      providerId: 'p-1',
      addr: '127.0.0.1:19091',
      version: 'v2',
      sdkName: 'croupier-go',
      sdkLang: 'go',
      sdkVersion: '1.1',
      functionId: 'player.ban',
      status: 'running',
      lastHeartbeat: '2026-01-01T00:00:01Z',
      functionsCount: 3,
      healthy: true,
      lastSeen: '2026-01-01T00:00:02Z',
      gameId: 'demo',
      env: 'prod',
      ownerInstance: 'server2',
      metadata: { region: 'cn' },
    });
  });

  it('falls back across legacy field spellings', () => {
    expect(
      normalizeFunctionInstance({
        providerId: 'p-1',
        address: '1.2.3.4:5',
        updatedAt: '2026-01-01T00:00:00Z',
        status: 'active',
      }),
    ).toEqual(
      expect.objectContaining({
        serviceId: 'p-1',
        providerId: 'p-1',
        addr: '1.2.3.4:5',
        lastSeen: '2026-01-01T00:00:00Z',
        status: 'running',
        agentId: '',
        version: '',
        functionId: '',
      }),
    );
  });

  it('maps inactive to stopped and unrecognized statuses to unknown', () => {
    expect(normalizeFunctionInstance({ status: 'inactive' }).status).toBe('stopped');
    expect(normalizeFunctionInstance({ status: 'weird' }).status).toBe('unknown');
    expect(normalizeFunctionInstance({}).status).toBe('unknown');
  });

  it('passes through recognized statuses untouched', () => {
    for (const status of ['running', 'stopped', 'error', 'unknown'] as const) {
      expect(normalizeFunctionInstance({ status }).status).toBe(status);
    }
  });

  it('keeps optional numeric and boolean fields undefined when absent', () => {
    expect(normalizeFunctionInstance({})).toEqual({
      agentId: '',
      agentName: '',
      serviceId: '',
      providerId: '',
      addr: '',
      version: '',
      sdkName: '',
      sdkLang: '',
      sdkVersion: '',
      functionId: '',
      status: 'unknown',
      lastHeartbeat: '',
      functionsCount: undefined,
      healthy: undefined,
      lastSeen: '',
      gameId: '',
      env: '',
      ownerInstance: '',
      metadata: undefined,
    });
  });
});

describe('normalizeFunctionSummary fallbacks', () => {
  it('falls back id to functionId and defaults tags to an empty array', () => {
    expect(
      normalizeFunctionSummary({
        functionId: 'f-1',
        version: '1.0',
        resource: 'player',
        operation: 'ban',
        tags: ['gm'],
      }),
    ).toEqual({
      id: 'f-1',
      version: '1.0',
      enabled: false,
      displayName: undefined,
      summary: undefined,
      tags: ['gm'],
      resource: 'player',
      operation: 'ban',
    });
  });

  it('normalizes string display names into LocalizedText', () => {
    expect(normalizeFunctionSummary({ id: 'f-2', displayName: '玩家', enabled: true })).toEqual({
      id: 'f-2',
      version: undefined,
      enabled: true,
      displayName: { 'zh-CN': '玩家', 'en-US': '玩家' },
      summary: undefined,
      tags: [],
      resource: undefined,
      operation: undefined,
    });
  });

  it('prefers the legacy numeric status over an explicit enabled flag', () => {
    expect(normalizeFunctionSummary({ status: 1, enabled: false }).enabled).toBe(true);
    expect(normalizeFunctionSummary({ status: 0, enabled: true }).enabled).toBe(true);
    expect(normalizeFunctionSummary({ status: 0 }).enabled).toBe(false);
  });
});

describe('getFunctionSummary', () => {
  beforeEach(() => mockedRequest.mockReset());

  it('passes scope filters and normalizes a bare array response', async () => {
    mockedRequest.mockResolvedValue([{ id: 'f-1', displayName: '玩家' }]);

    const res = await getFunctionSummary({
      gameId: 'demo',
      env: 'prod',
      resource: 'player',
      tags: ['gm'],
      enabled: true,
    });

    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/functions', {
      params: { gameId: 'demo', env: 'prod', resource: 'player', tags: ['gm'], enabled: true },
    });
    expect(res).toEqual([
      expect.objectContaining({
        id: 'f-1',
        displayName: { 'zh-CN': '玩家', 'en-US': '玩家' },
      }),
    ]);
  });

  it('normalizes the { functions } envelope', async () => {
    mockedRequest.mockResolvedValue({ functions: [{ functionId: 'f-2', status: 1 }] });

    const res = await getFunctionSummary();

    expect(res[0]).toMatchObject({ id: 'f-2', enabled: true });
    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/functions', {
      params: {
        gameId: undefined,
        env: undefined,
        resource: undefined,
        tags: undefined,
        enabled: undefined,
      },
    });
  });

  it('normalizes the { items } envelope', async () => {
    mockedRequest.mockResolvedValue({ items: [{ id: 'f-3' }] });

    await expect(getFunctionSummary()).resolves.toEqual([expect.objectContaining({ id: 'f-3' })]);
  });

  it('throws a localized error for unrecognized response shapes', async () => {
    mockedRequest.mockResolvedValue({ total: 3 });

    await expect(getFunctionSummary()).rejects.toThrow('函数摘要接口返回了无法识别的数据格式');
  });
});

describe('getFunctionDetail (enhanced)', () => {
  beforeEach(() => mockedRequest.mockReset());

  it('GETs the raw detail payload without extra normalization', async () => {
    const detail = { id: 'f-1', instances: [], metrics: { calls: 1 } };
    mockedRequest.mockResolvedValue(detail);

    await expect(getFunctionDetail('f-1', { gameId: 'demo', env: 'prod' })).resolves.toBe(detail);
    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/functions/f-1', { method: 'GET' });
  });
});

describe('getFunctionCalls (enhanced)', () => {
  beforeEach(() => mockedRequest.mockReset());

  it('passes every filter and defaults pagination metadata', async () => {
    mockedRequest.mockResolvedValue({});

    const res = await getFunctionCalls({
      functionId: 'f-1',
      userId: 'u-1',
      gameId: 'demo',
      env: 'prod',
      status: 'success',
      startTime: '2026-01-01T00:00:00Z',
      endTime: '2026-01-02T00:00:00Z',
      limit: 5,
      offset: 10,
    });

    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/function-calls', {
      params: {
        functionId: 'f-1',
        userId: 'u-1',
        gameId: 'demo',
        env: 'prod',
        status: 'success',
        startTime: '2026-01-01T00:00:00Z',
        endTime: '2026-01-02T00:00:00Z',
        limit: 5,
        offset: 10,
      },
    });
    expect(res).toEqual({ calls: [], total: 0, hasMore: false });
  });

  it('normalizes call records with a status fallback', async () => {
    mockedRequest.mockResolvedValue({
      calls: [
        { id: 'c-1', functionId: 'f-1', status: 'success', startedAt: 't-1' },
        { id: 'c-2' },
        { functionId: 'f-9' },
      ],
      total: 3,
      hasMore: true,
    });

    const res = await getFunctionCalls();

    expect(res.total).toBe(3);
    expect(res.hasMore).toBe(true);
    expect(res.calls[0]).toMatchObject({
      id: 'c-1',
      functionId: 'f-1',
      status: 'success',
      startedAt: 't-1',
    });
    expect(res.calls[1]).toMatchObject({
      id: 'c-2',
      functionId: '',
      status: 'failed',
      startedAt: '',
    });
    expect(res.calls[2]).toEqual({
      id: '',
      functionId: 'f-9',
      user: undefined,
      status: 'failed',
      startedAt: '',
      completedAt: undefined,
      duration: undefined,
      payload: undefined,
      result: undefined,
      error: undefined,
      agentId: undefined,
      serviceId: undefined,
      gameId: undefined,
      env: undefined,
      taskId: undefined,
      retryCount: undefined,
    });
  });
});

describe('getFunctionCall and cancelFunctionCall', () => {
  beforeEach(() => mockedRequest.mockReset());

  it('normalizes a single call detail', async () => {
    mockedRequest.mockResolvedValue({ id: 'c-3', status: 'timeout', error: 'boom' });

    const call = await getFunctionCall('c-3');

    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/function-calls/c-3', { method: 'GET' });
    expect(call).toMatchObject({ id: 'c-3', status: 'timeout', error: 'boom' });
  });

  it('POSTs the cancel route', async () => {
    mockedRequest.mockResolvedValue(undefined);

    await cancelFunctionCall('c-3');

    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/function-calls/c-3/cancel', {
      method: 'POST',
    });
  });
});

describe('getFunctionInstances fallbacks', () => {
  beforeEach(() => mockedRequest.mockReset());

  it('falls back total to the normalized instance count', async () => {
    mockedRequest.mockResolvedValue({ items: [{ functionId: 'f-1' }, { functionId: 'f-2' }] });

    await expect(getFunctionInstances()).resolves.toEqual({
      instances: [
        expect.objectContaining({ functionId: 'f-1' }),
        expect.objectContaining({ functionId: 'f-2' }),
      ],
      total: 2,
    });
    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/functions/instances', {
      params: { gameId: undefined },
    });
  });

  it('returns an empty list when the envelope carries neither items nor instances', async () => {
    mockedRequest.mockResolvedValue({});

    await expect(
      getFunctionInstances({ gameId: 'demo', env: 'prod', status: 'running' }),
    ).resolves.toEqual({ instances: [], total: 0 });
    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/functions/instances', {
      params: { env: 'prod', status: 'running', gameId: 'demo' },
    });
  });
});

describe('getRegistryServices', () => {
  beforeEach(() => mockedRequest.mockReset());

  it('passes filters and normalizes services', async () => {
    mockedRequest.mockResolvedValue({
      services: [
        {
          serviceId: 's-1',
          addr: '127.0.0.1:19091',
          status: 'healthy',
          lastSeen: 't-1',
          functionsCount: 3,
          gameId: 'demo',
          env: 'prod',
          version: 'v1',
          metadata: { region: 'cn' },
        },
      ],
      total: 1,
    });

    const res = await getRegistryServices({ gameId: 'demo', env: 'prod', status: 'healthy' });

    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/registry/services', {
      params: { gameId: 'demo', env: 'prod', status: 'healthy' },
    });
    expect(res.total).toBe(1);
    expect(res.services[0]).toEqual({
      serviceId: 's-1',
      addr: '127.0.0.1:19091',
      status: 'healthy',
      lastSeen: 't-1',
      functionsCount: 3,
      gameId: 'demo',
      env: 'prod',
      version: 'v1',
      metadata: { region: 'cn' },
    });
  });

  it('defaults every registry field for an empty raw service row', async () => {
    mockedRequest.mockResolvedValue({ services: [{}] });

    const res = await getRegistryServices();

    expect(res.services[0]).toEqual({
      serviceId: '',
      addr: '',
      status: 'unknown',
      lastSeen: '',
      functionsCount: 0,
      gameId: undefined,
      env: undefined,
      version: undefined,
      metadata: undefined,
    });
  });

  it('defaults empty services, counters and filters', async () => {
    mockedRequest.mockResolvedValue({});

    const res = await getRegistryServices();

    expect(res).toEqual({ services: [], total: 0 });
    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/registry/services', {
      params: { gameId: undefined, env: undefined, status: undefined },
    });
  });
});

describe('batchUpdateFunctions (enhanced)', () => {
  beforeEach(() => mockedRequest.mockReset());

  it('deletes each id and counts settled outcomes', async () => {
    mockedRequest.mockResolvedValueOnce(undefined).mockRejectedValueOnce(new Error('deny'));

    const res = await batchUpdateFunctions({ functionIds: ['a', 'b'], operation: 'delete' });

    expect(res).toEqual({ success: 1, failed: 1, errors: [] });
    expect(mockedRequest).toHaveBeenNthCalledWith(1, '/api/v1/functions/a', {
      method: 'DELETE',
    });
    expect(mockedRequest).toHaveBeenNthCalledWith(2, '/api/v1/functions/b', {
      method: 'DELETE',
    });
  });

  it('enables functions through the batch endpoint and reports failed ids', async () => {
    mockedRequest.mockResolvedValue({ updated: 3, failed: ['b'] });

    const res = await batchUpdateFunctions({
      functionIds: ['a', 'b', 'c'],
      operation: 'enable',
      gameId: 'demo',
      env: 'prod',
    });

    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/functions/batch-update', {
      method: 'POST',
      data: { functionIds: ['a', 'b', 'c'], enabled: true, gameId: 'demo', env: 'prod' },
    });
    expect(res).toEqual({ success: 2, failed: 1, errors: ['b'] });
  });

  it('disables functions through the batch endpoint', async () => {
    mockedRequest.mockResolvedValue({ updated: 2 });

    const res = await batchUpdateFunctions({ functionIds: ['a', 'b'], operation: 'disable' });

    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/functions/batch-update', {
      method: 'POST',
      data: { functionIds: ['a', 'b'], enabled: false, gameId: undefined, env: undefined },
    });
    expect(res).toEqual({ success: 2, failed: 0, errors: [] });
  });

  it('tolerates an empty batch response body', async () => {
    mockedRequest.mockResolvedValue(undefined);

    const res = await batchUpdateFunctions({ functionIds: ['a'], operation: 'enable' });

    expect(res).toEqual({ success: 0, failed: 0, errors: [] });
  });

  it('reports unsupported operations without calling the backend', async () => {
    const res = await batchUpdateFunctions({
      functionIds: ['a', 'b'],
      operation: 'bogus' as 'enable',
    });

    expect(res).toEqual({ success: 0, failed: 2, errors: ['unsupported operation'] });
    expect(mockedRequest).not.toHaveBeenCalled();
  });
});

describe('searchFunctions', () => {
  beforeEach(() => mockedRequest.mockReset());

  const seed = [
    { id: 'player.ban', displayName: '封禁玩家', summary: 'ban a player' },
    {
      id: 'mail.send',
      displayName: { 'zh-CN': '发邮件', 'en-US': 'Send Mail' },
      summary: { 'zh-CN': '发送', 'en-US': 'send' },
    },
    { id: 'report.daily' },
  ];

  it('matches ids and localized display/summary fields case-insensitively', async () => {
    mockedRequest.mockResolvedValue(seed);
    const byId = await searchFunctions({ query: 'PLAYER.BAN' });
    expect(byId).toEqual({ functions: [expect.objectContaining({ id: 'player.ban' })], total: 1 });

    mockedRequest.mockReset();
    mockedRequest.mockResolvedValue(seed);
    const byZh = await searchFunctions({ query: '发邮' });
    expect(byZh.total).toBe(1);
    expect(byZh.functions[0]).toMatchObject({ id: 'mail.send' });

    mockedRequest.mockReset();
    mockedRequest.mockResolvedValue(seed);
    const bySummary = await searchFunctions({ query: 'SEND MAIL' });
    expect(bySummary.total).toBe(1);
    expect(bySummary.functions[0]).toMatchObject({ id: 'mail.send' });
  });

  it('returns the unfiltered list for blank queries and forwards scope filters', async () => {
    mockedRequest.mockResolvedValue(seed);

    const res = await searchFunctions({
      query: '   ',
      gameId: 'demo',
      env: 'prod',
      resource: 'player',
      tags: ['gm'],
    });

    expect(res.total).toBe(3);
    expect(res.functions).toHaveLength(3);
    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/functions', {
      params: { gameId: 'demo', env: 'prod', resource: 'player', tags: ['gm'], enabled: undefined },
    });
  });

  it('applies the limit after filtering while keeping the pre-limit total', async () => {
    mockedRequest.mockResolvedValue(seed);

    const res = await searchFunctions({ query: '', limit: 2 });

    expect(res.functions).toHaveLength(2);
    expect(res.total).toBe(3);
  });
});

describe('getFunctionResources', () => {
  beforeEach(() => mockedRequest.mockReset());

  it('counts resources with an unassigned bucket for missing resources', async () => {
    mockedRequest.mockResolvedValue([
      { id: 'a', resource: 'player' },
      { id: 'b', resource: 'player' },
      { id: 'c' },
    ]);

    const res = await getFunctionResources({ gameId: 'demo' });

    expect(res).toEqual({
      resources: ['player', 'unassigned'],
      counts: { player: 2, unassigned: 1 },
    });
  });
});

describe('getFunctionTags', () => {
  beforeEach(() => mockedRequest.mockReset());

  it('counts tags across functions and applies the limit', async () => {
    const seed = [{ id: 'a', tags: ['gm', 'player'] }, { id: 'b', tags: ['gm'] }, { id: 'c' }];
    mockedRequest.mockResolvedValue(seed);

    const limited = await getFunctionTags({ limit: 1 });
    expect(limited.tags).toHaveLength(1);
    expect(limited.counts).toEqual({ gm: 2, player: 1 });

    mockedRequest.mockReset();
    mockedRequest.mockResolvedValue(seed);
    const all = await getFunctionTags();
    expect([...all.tags].sort()).toEqual(['gm', 'player']);
    expect(all.counts).toEqual({ gm: 2, player: 1 });
  });
});

describe('openapi helpers (enhanced)', () => {
  beforeEach(() => mockedRequest.mockReset());

  it('getFunctionOpenAPIDetail GETs the raw operation payload', async () => {
    const detail = { operationId: 'player.ban', extensions: { 'x-risk': 'high' } };
    mockedRequest.mockResolvedValue(detail);

    await expect(getFunctionOpenAPIDetail('player.ban')).resolves.toBe(detail);
    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/functions/player.ban/openapi');
  });

  it('batchGetFunctionOpenAPI posts the id list', async () => {
    const map = { 'player.ban': { operationId: 'player.ban' } };
    mockedRequest.mockResolvedValue(map);

    await expect(batchGetFunctionOpenAPI(['player.ban'])).resolves.toBe(map);
    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/functions/_openapi-batch', {
      method: 'POST',
      data: { functionIds: ['player.ban'] },
    });
  });
});
