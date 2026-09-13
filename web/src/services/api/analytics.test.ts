import { request } from '@umijs/max';
import { createEventSource } from '../core/http';
import {
  fetchAnalyticsAdoption,
  fetchAnalyticsAdoptionBreakdown,
  fetchAnalyticsEvents,
  fetchAnalyticsFilters,
  fetchAnalyticsFunnel,
  fetchAnalyticsLevels,
  fetchAnalyticsLevelsEpisodes,
  fetchAnalyticsLevelsMaps,
  fetchAnalyticsOverview,
  fetchAnalyticsPaymentsSummary,
  fetchAnalyticsPaths,
  fetchAnalyticsRealtime,
  fetchAnalyticsRetention,
  fetchAnalyticsTransactions,
  fetchInvocationsList,
  fetchInvocationsSummary,
  fetchInvocationsTrend,
  fetchProductTrend,
  fetchRealtimeSeries,
  fetchWarehouseDAU,
  fetchWarehouseOnline,
  fetchWarehouseRevenue,
  openAnalyticsRealtimeEventSource,
  saveAnalyticsFilters,
} from './analytics';

jest.mock('@umijs/max', () => ({ request: jest.fn() }));
// 默认 scope 与既有用例一致；新 describe 内可变更为空 scope 覆盖注入分支
const mockScope: { gameId?: string; env?: string } = { gameId: 'game-a', env: 'prod' };
jest.mock('@/stores/scope', () => ({ getScope: () => ({ ...mockScope }) }));
// jsdom 无 EventSource：mock 掉 SSE 构造以便断言参数映射
jest.mock('../core/http', () => ({ createEventSource: jest.fn() }));

const mockedRequest = request as jest.MockedFunction<typeof request>;
const mockedCreateEventSource = createEventSource as jest.MockedFunction<typeof createEventSource>;

describe('analytics API adapters', () => {
  beforeEach(() => mockedRequest.mockReset());

  it('adapts the overview DTO and normalizes date query names', async () => {
    mockedRequest.mockResolvedValue({
      metrics: { dau: 12, mau: 50, revenue: 19.5, payingRate: 0.25 },
      trends: { newUsers: [{ date: '2026-08-19', value: 3 }] },
    });

    await expect(fetchAnalyticsOverview({ start: 's', end: 'e' })).resolves.toMatchObject({
      dau: 12,
      payRate: 25,
      series: { newUsers: [['2026-08-19', 3]] },
    });
    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/analytics/overview', {
      params: expect.objectContaining({
        startDate: 's',
        endDate: 'e',
        gameId: 'game-a',
        env: 'prod',
      }),
    });
  });

  it('adapts behavior event and path DTOs', async () => {
    mockedRequest
      .mockResolvedValueOnce({
        items: [{ eventType: 'login', userId: 'u-1', timestamp: '2026-08-19T00:00:00Z' }],
        total: 1,
      })
      .mockResolvedValueOnce({ paths: { items: [{ path: 'login>start', groups: 2 }] } });

    await expect(fetchAnalyticsEvents({ event: 'login' })).resolves.toEqual({
      events: [{ event: 'login', userId: 'u-1', time: '2026-08-19T00:00:00Z', data: undefined }],
      total: 1,
    });
    await expect(fetchAnalyticsPaths({ steps: 3 })).resolves.toEqual({
      paths: [{ path: 'login>start', groups: 2 }],
    });
  });

  it('adapts payments summary and transaction DTOs', async () => {
    mockedRequest
      .mockResolvedValueOnce({
        items: [{ date: '2026-08-19', revenue: 20, transactions: 2, users: 1 }],
      })
      .mockResolvedValueOnce({
        items: [
          {
            id: 'o-1',
            userId: 'u-1',
            productId: 'sku-a',
            amount: 20,
            status: 'paid',
            paymentMethod: 'card',
            createdAt: '2026-08-19T00:00:00Z',
          },
        ],
        total: 1,
      });

    await expect(fetchAnalyticsPaymentsSummary()).resolves.toMatchObject({
      totals: { revenue: 20, transactions: 2, users: 1 },
      items: [{ date: '2026-08-19', revenue: 20, transactions: 2, users: 1 }],
    });
    await expect(fetchAnalyticsTransactions({ size: 50 })).resolves.toEqual({
      transactions: [
        {
          orderId: 'o-1',
          userId: 'u-1',
          productId: 'sku-a',
          amount: 20,
          status: 'paid',
          time: '2026-08-19T00:00:00Z',
          channel: 'card',
          currency: '',
        },
      ],
      total: 1,
    });
  });
});

describe('analytics API adapters (param/scope normalization matrix)', () => {
  beforeEach(() => {
    mockedRequest.mockReset();
    mockedCreateEventSource.mockReset();
    mockScope.gameId = 'game-a';
    mockScope.env = 'prod';
  });

  it('keeps canonical startDate/endDate untouched and skips legacy aliases', async () => {
    mockedRequest.mockResolvedValue({});

    await fetchAnalyticsOverview({ startDate: 'x1', endDate: 'y1' });
    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/analytics/overview', {
      params: { startDate: 'x1', endDate: 'y1', gameId: 'game-a', env: 'prod' },
    });

    await fetchAnalyticsOverview({});
    expect(mockedRequest).toHaveBeenLastCalledWith('/api/v1/analytics/overview', {
      params: { gameId: 'game-a', env: 'prod' },
    });
  });

  it('does not override explicit scope params and skips empty scope injection', async () => {
    mockedRequest.mockResolvedValue({});

    await fetchAnalyticsRetention({ gameId: 'g1', env: 'e1' });
    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/analytics/retention', {
      params: { gameId: 'g1', env: 'e1' },
    });

    mockScope.gameId = undefined;
    mockScope.env = undefined;
    await fetchAnalyticsRetention({});
    expect(mockedRequest).toHaveBeenLastCalledWith('/api/v1/analytics/retention', {
      params: {},
    });

    await fetchAnalyticsRealtime({ gameId: 'g2' });
    expect(mockedRequest).toHaveBeenLastCalledWith('/api/v1/analytics/realtime', {
      params: { gameId: 'g2' },
    });
  });

  it('defaults overview KPIs and series tuples for missing fields', async () => {
    mockedRequest.mockResolvedValue({
      metrics: { arpu: 1, arppu: 2, newUsers: 3 },
      trends: {
        newUsers: [{}],
        activeUsers: [{ date: 'd' }],
        revenue: [{ date: 'd', value: 5 }, { date: 'd2', revenue: 7 }, {}],
      },
    });

    await expect(fetchAnalyticsOverview({ startDate: 's', endDate: 'e' })).resolves.toEqual({
      dau: 0,
      mau: 0,
      newUsers: 3,
      revenue: 0,
      arpu: 1,
      arppu: 2,
      payRate: null,
      wau: null,
      registeredTotal: null,
      d1: null,
      d7: null,
      d30: null,
      series: {
        newUsers: [['', 0]],
        peakOnline: [['d', 0]],
        revenue: [
          ['d', 5],
          ['d2', 7],
          ['', 0],
        ],
      },
    });
  });

  it('returns zeroed overview when the payload is empty', async () => {
    mockedRequest.mockResolvedValue({});

    await expect(fetchAnalyticsOverview()).resolves.toEqual({
      dau: 0,
      mau: 0,
      newUsers: 0,
      revenue: 0,
      arpu: 0,
      arppu: 0,
      payRate: null,
      wau: null,
      registeredTotal: null,
      d1: null,
      d7: null,
      d30: null,
      series: { newUsers: [], peakOnline: [], revenue: [] },
    });
  });

  it('maps, stringifies and filters realtime SSE params', () => {
    mockedCreateEventSource.mockReturnValue({} as EventSource);
    openAnalyticsRealtimeEventSource({
      q: 'x',
      page: 2,
      extra: { k: 1 },
      skip: undefined,
      drop: null,
    });
    expect(mockedCreateEventSource).toHaveBeenCalledWith('/api/v1/analytics/realtime', {
      params: { gameId: 'game-a', env: 'prod', q: 'x', page: 2, extra: '{"k":1}' },
    });

    openAnalyticsRealtimeEventSource();
    expect(mockedCreateEventSource).toHaveBeenLastCalledWith('/api/v1/analytics/realtime', {
      params: { gameId: 'game-a', env: 'prod' },
    });
  });

  it('derives realtime-series duration from the date window', async () => {
    mockedRequest
      .mockResolvedValueOnce({})
      .mockResolvedValueOnce({})
      .mockResolvedValueOnce({})
      .mockResolvedValueOnce({})
      .mockResolvedValueOnce({})
      .mockResolvedValueOnce({})
      .mockResolvedValueOnce({})
      .mockResolvedValueOnce({});

    await fetchRealtimeSeries({ duration: 10, startDate: 'a', endDate: 'b' });
    expect(mockedRequest).toHaveBeenNthCalledWith(1, '/api/v1/analytics/realtime/series', {
      params: { duration: 10, gameId: 'game-a', env: 'prod' },
    });

    // 150s window -> ceil(2.5) = 3 minutes
    await fetchRealtimeSeries({
      startDate: '2026-09-01T00:00:00Z',
      endDate: '2026-09-01T00:02:30Z',
    });
    expect(mockedRequest).toHaveBeenNthCalledWith(2, '/api/v1/analytics/realtime/series', {
      params: { duration: 3, gameId: 'game-a', env: 'prod' },
    });

    // sub-minute window clamps to 1
    await fetchRealtimeSeries({
      startDate: '2026-09-01T00:00:00Z',
      endDate: '2026-09-01T00:00:30Z',
    });
    expect(mockedRequest).toHaveBeenNthCalledWith(3, '/api/v1/analytics/realtime/series', {
      params: { duration: 1, gameId: 'game-a', env: 'prod' },
    });

    // end before start: no duration derived
    await fetchRealtimeSeries({
      startDate: '2026-09-01T01:00:00Z',
      endDate: '2026-09-01T00:00:00Z',
    });
    expect(mockedRequest).toHaveBeenNthCalledWith(4, '/api/v1/analytics/realtime/series', {
      params: { gameId: 'game-a', env: 'prod' },
    });

    // unparseable dates on both ends
    await fetchRealtimeSeries({ startDate: 'bad', endDate: 'bad' });
    expect(mockedRequest).toHaveBeenNthCalledWith(5, '/api/v1/analytics/realtime/series', {
      params: { gameId: 'game-a', env: 'prod' },
    });

    // end unparseable only
    await fetchRealtimeSeries({ startDate: '2026-09-01T00:00:00Z', endDate: 'bad' });
    expect(mockedRequest).toHaveBeenNthCalledWith(6, '/api/v1/analytics/realtime/series', {
      params: { gameId: 'game-a', env: 'prod' },
    });

    // non-string window values never reach the duration computation
    await fetchRealtimeSeries({ startDate: 5 });
    expect(mockedRequest).toHaveBeenNthCalledWith(7, '/api/v1/analytics/realtime/series', {
      params: { gameId: 'game-a', env: 'prod' },
    });
    await fetchRealtimeSeries({ startDate: 'x', endDate: 8 });
    expect(mockedRequest).toHaveBeenNthCalledWith(8, '/api/v1/analytics/realtime/series', {
      params: { gameId: 'game-a', env: 'prod' },
    });
  });

  it('maps realtime series rows to tuples and tolerates missing series', async () => {
    mockedRequest.mockResolvedValueOnce({
      series: {
        users: [{ timestamp: 't', value: 9 }, {}],
        events: [{ timestamp: 't2', value: 3 }],
      },
    });

    await expect(fetchRealtimeSeries({ duration: 1 })).resolves.toEqual({
      online: [
        ['t', 9],
        ['', 0],
      ],
      active5MSum: [['t2', 3]],
      active15MSum: [],
      revenueCents: [],
    });

    mockedRequest.mockResolvedValueOnce({});
    await expect(fetchRealtimeSeries({ duration: 1 })).resolves.toEqual({
      online: [],
      active5MSum: [],
      active15MSum: [],
      revenueCents: [],
    });
  });

  it('normalizes behavior events without legacy aliases and with empty rows', async () => {
    mockedRequest.mockResolvedValueOnce({ items: [{}], total: 0 });

    await expect(fetchAnalyticsEvents({})).resolves.toEqual({
      events: [{ event: '', userId: '', time: '', data: undefined }],
      total: 0,
    });
    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/analytics/behavior/events', {
      params: { gameId: 'game-a', env: 'prod' },
    });

    mockedRequest.mockResolvedValueOnce({ total: 4 });
    await expect(fetchAnalyticsEvents({ eventType: 'e1', event: 'login' })).resolves.toEqual({
      events: [],
      total: 4,
    });
    expect(mockedRequest).toHaveBeenLastCalledWith('/api/v1/analytics/behavior/events', {
      params: { eventType: 'e1', gameId: 'game-a', env: 'prod' },
    });
  });

  it('splits funnel steps strings and defaults step rows', async () => {
    mockedRequest.mockResolvedValueOnce({ steps: [{}] });

    await expect(fetchAnalyticsFunnel({ steps: 'a, b ,' })).resolves.toEqual({
      steps: [{ step: '', users: 0, rate: 0, conversionRate: 0, dropOffRate: 0 }],
    });
    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/analytics/behavior/funnel', {
      params: { steps: ['a', 'b'], gameId: 'game-a', env: 'prod' },
    });

    mockedRequest.mockResolvedValueOnce({});
    await expect(fetchAnalyticsFunnel({ steps: ['x'] })).resolves.toEqual({ steps: [] });
    expect(mockedRequest).toHaveBeenLastCalledWith('/api/v1/analytics/behavior/funnel', {
      params: { steps: ['x'], gameId: 'game-a', env: 'prod' },
    });
  });

  it('resolves paths depth, array shapes and group fallbacks', async () => {
    mockedRequest.mockResolvedValueOnce({
      paths: [{ path: ['a', 'b'], groups: 2 }, { path: 'x>y', count: 3 }, {}],
    });

    await expect(fetchAnalyticsPaths({ depth: 2, steps: 3 })).resolves.toEqual({
      paths: [
        { path: 'a>b', groups: 2 },
        { path: 'x>y', groups: 3 },
        { path: '', groups: 0 },
      ],
    });
    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/analytics/behavior/paths', {
      params: { depth: 2, gameId: 'game-a', env: 'prod' },
    });

    mockedRequest.mockResolvedValueOnce({ paths: {} });
    await expect(fetchAnalyticsPaths({})).resolves.toEqual({ paths: [] });
    expect(mockedRequest).toHaveBeenNthCalledWith(2, '/api/v1/analytics/behavior/paths', {
      params: { gameId: 'game-a', env: 'prod' },
    });

    mockedRequest.mockResolvedValueOnce({});
    await expect(fetchAnalyticsPaths({})).resolves.toEqual({ paths: [] });
  });

  it('derives the adoption feature from a features list and scales rates', async () => {
    mockedRequest.mockResolvedValueOnce({
      features: [{}, { feature: 'f', users: 5, adoptionRate: 0.5, frequency: 2 }],
    });

    await expect(fetchAnalyticsAdoption({ features: 'f1,f2' })).resolves.toEqual({
      features: [
        { feature: '', groups: 0, rate: 0, frequency: 0 },
        { feature: 'f', groups: 5, rate: 50, frequency: 2 },
      ],
      baseline: 0,
    });
    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/analytics/behavior/adoption', {
      params: { feature: 'f1', gameId: 'game-a', env: 'prod' },
    });

    mockedRequest.mockResolvedValueOnce({ features: [] });
    await fetchAnalyticsAdoption({ features: '' });
    expect(mockedRequest).toHaveBeenNthCalledWith(2, '/api/v1/analytics/behavior/adoption', {
      params: { feature: '', gameId: 'game-a', env: 'prod' },
    });

    mockedRequest.mockResolvedValueOnce({});
    await expect(fetchAnalyticsAdoption({ feature: 'fx', features: 'f1' })).resolves.toEqual({
      features: [],
      baseline: 0,
    });
    expect(mockedRequest).toHaveBeenNthCalledWith(3, '/api/v1/analytics/behavior/adoption', {
      params: { feature: 'fx', gameId: 'game-a', env: 'prod' },
    });

    mockedRequest.mockResolvedValueOnce({ features: [] });
    await expect(fetchAnalyticsAdoption()).resolves.toEqual({ features: [], baseline: 0 });
  });

  it('segments adoption breakdowns by platform/country/region keys', async () => {
    mockedRequest
      .mockResolvedValueOnce({ bySegment: { platforms: { ios: 3 } } })
      .mockResolvedValueOnce({ bySegment: { roles: { cn: 4 } } })
      .mockResolvedValueOnce({ bySegment: { regions: { eu: 5 } } })
      .mockResolvedValueOnce({});

    await expect(
      fetchAnalyticsAdoptionBreakdown({ by: 'platform', features: 'f1' }),
    ).resolves.toEqual({
      by: 'platform',
      rows: [{ dim: 'ios', groups: 3, baseline: 0, rate: 0 }],
    });
    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/analytics/behavior/adoption/breakdown', {
      params: { by: 'platform', feature: 'f1', gameId: 'game-a', env: 'prod' },
    });

    await expect(fetchAnalyticsAdoptionBreakdown({ by: 'country' })).resolves.toEqual({
      by: 'country',
      rows: [{ dim: 'cn', groups: 4, baseline: 0, rate: 0 }],
    });

    await expect(fetchAnalyticsAdoptionBreakdown({ by: 'region' })).resolves.toEqual({
      by: 'region',
      rows: [{ dim: 'eu', groups: 5, baseline: 0, rate: 0 }],
    });

    // 缺省 by -> channel（regions 段）；空 bySegment -> 空 rows
    await expect(fetchAnalyticsAdoptionBreakdown()).resolves.toEqual({
      by: 'channel',
      rows: [],
    });
    expect(mockedRequest).toHaveBeenLastCalledWith(
      '/api/v1/analytics/behavior/adoption/breakdown',
      {
        params: { by: undefined, gameId: 'game-a', env: 'prod' },
      },
    );

    // 空 features 列表 -> feature 兜底为空串
    mockedRequest.mockResolvedValueOnce({});
    await fetchAnalyticsAdoptionBreakdown({ features: '' });
    expect(mockedRequest).toHaveBeenLastCalledWith(
      '/api/v1/analytics/behavior/adoption/breakdown',
      {
        params: { feature: '', gameId: 'game-a', env: 'prod' },
      },
    );
  });

  it('defaults payment summary rows and keeps explicit groupBy', async () => {
    mockedRequest.mockResolvedValueOnce({ items: [{}] });

    await expect(fetchAnalyticsPaymentsSummary({ groupBy: 'week' })).resolves.toMatchObject({
      totals: { revenue: 0, transactions: 0, users: 0 },
      items: [{ date: '', revenue: 0, transactions: 0, users: 0 }],
    });
    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/analytics/payments/summary', {
      params: { groupBy: 'week', gameId: 'game-a', env: 'prod' },
    });

    mockedRequest.mockResolvedValueOnce({});
    await expect(fetchAnalyticsPaymentsSummary()).resolves.toMatchObject({
      totals: { revenue: 0, transactions: 0, users: 0 },
      items: [],
    });
    expect(mockedRequest).toHaveBeenLastCalledWith('/api/v1/analytics/payments/summary', {
      params: { groupBy: 'day', gameId: 'game-a', env: 'prod' },
    });
  });

  it('normalizes transactions paging and empty rows', async () => {
    mockedRequest.mockResolvedValueOnce({ items: [{}] });

    await expect(fetchAnalyticsTransactions({ pageSize: 20, size: 50 })).resolves.toEqual({
      transactions: [
        {
          orderId: '',
          userId: '',
          productId: '',
          amount: 0,
          status: '',
          time: '',
          channel: '',
          currency: '',
        },
      ],
      total: 0,
    });
    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/analytics/payments/transactions', {
      params: { pageSize: 20, gameId: 'game-a', env: 'prod' },
    });

    mockedRequest.mockResolvedValueOnce({ total: 2 });
    await expect(fetchAnalyticsTransactions({})).resolves.toEqual({
      transactions: [],
      total: 2,
    });
    expect(mockedRequest).toHaveBeenLastCalledWith('/api/v1/analytics/payments/transactions', {
      params: { gameId: 'game-a', env: 'prod' },
    });
  });

  it('passes levels/episodes/maps through with scoped params', async () => {
    mockedRequest
      .mockResolvedValueOnce({ levels: [] })
      .mockResolvedValueOnce({ episodes: [] })
      .mockResolvedValueOnce({ maps: [] });

    await expect(fetchAnalyticsLevels({ gameId: 'g1', env: 'e1' })).resolves.toEqual({
      levels: [],
    });
    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/analytics/levels', {
      params: { gameId: 'g1', env: 'e1' },
    });

    await expect(fetchAnalyticsLevelsEpisodes()).resolves.toEqual({ episodes: [] });
    expect(mockedRequest).toHaveBeenNthCalledWith(2, '/api/v1/analytics/levels/episodes', {
      params: { gameId: 'game-a', env: 'prod' },
    });

    await expect(fetchAnalyticsLevelsMaps()).resolves.toEqual({ maps: [] });
    expect(mockedRequest).toHaveBeenNthCalledWith(3, '/api/v1/analytics/levels/maps', {
      params: { gameId: 'game-a', env: 'prod' },
    });
  });

  it('adapts product trend rows with defaults', async () => {
    mockedRequest.mockResolvedValueOnce({
      items: [{ productId: 'sku', revenue: 10, sales: 2 }, {}],
    });

    await expect(fetchProductTrend({ gameId: 'g1', env: 'e1' })).resolves.toEqual({
      products: [
        { productId: 'sku', points: [{ time: '', amount: 10, count: 2 }] },
        { productId: '', points: [{ time: '', amount: 0, count: 0 }] },
      ],
    });

    mockedRequest.mockResolvedValueOnce({});
    await expect(fetchProductTrend({})).resolves.toEqual({ products: [] });
  });

  it('normalizes analytics filters with full, partial and missing payloads', async () => {
    mockedRequest.mockResolvedValueOnce({
      gameId: 'g',
      env: 'e',
      events: ['a', 1],
      paymentsEnabled: false,
      sampleGlobal: 50,
    });
    await expect(fetchAnalyticsFilters({ gameId: 'g', env: 'e' })).resolves.toEqual({
      gameId: 'g',
      env: 'e',
      events: ['a', '1'],
      paymentsEnabled: false,
      sampleGlobal: 50,
    });
    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/analytics/filters', {
      params: { gameId: 'g', env: 'e' },
    });

    mockedRequest.mockResolvedValueOnce({ events: 'nope' });
    await expect(fetchAnalyticsFilters({ gameId: 'g', env: 'e' })).resolves.toEqual({
      gameId: '',
      env: '',
      events: [],
      paymentsEnabled: true,
      sampleGlobal: 100,
    });

    mockedRequest.mockResolvedValueOnce(undefined);
    await expect(fetchAnalyticsFilters({ gameId: 'g', env: 'e' })).resolves.toEqual({
      gameId: '',
      env: '',
      events: [],
      paymentsEnabled: true,
      sampleGlobal: 100,
    });
  });

  it('saves analytics filters via PUT with the explicit body', async () => {
    mockedRequest.mockResolvedValue(undefined);

    await saveAnalyticsFilters({
      gameId: 'g',
      env: 'e',
      events: ['a'],
      paymentsEnabled: true,
      sampleGlobal: 100,
    });

    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/analytics/filters', {
      method: 'PUT',
      data: {
        gameId: 'g',
        env: 'e',
        events: ['a'],
        paymentsEnabled: true,
        sampleGlobal: 100,
      },
    });
  });

  it('passes invocation analytics through with scoped params', async () => {
    mockedRequest
      .mockResolvedValueOnce({ total: 1 })
      .mockResolvedValueOnce({ points: [] })
      .mockResolvedValueOnce({ items: [], total: 0, page: 1, pageSize: 20 });

    await expect(fetchInvocationsSummary({ gameId: 'g1', env: 'e1' })).resolves.toEqual({
      total: 1,
    });
    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/analytics/invocations/summary', {
      params: { gameId: 'g1', env: 'e1' },
    });

    await expect(fetchInvocationsTrend()).resolves.toEqual({ points: [] });
    expect(mockedRequest).toHaveBeenNthCalledWith(2, '/api/v1/analytics/invocations/trend', {
      params: { gameId: 'game-a', env: 'prod' },
    });

    await expect(fetchInvocationsList()).resolves.toEqual({
      items: [],
      total: 0,
      page: 1,
      pageSize: 20,
    });
    expect(mockedRequest).toHaveBeenNthCalledWith(3, '/api/v1/analytics/invocations', {
      params: { gameId: 'game-a', env: 'prod' },
    });
  });

  it('passes warehouse aggregates through with scoped params', async () => {
    mockedRequest
      .mockResolvedValueOnce({ points: [] })
      .mockResolvedValueOnce({ points: [] })
      .mockResolvedValueOnce({ points: [] });

    await expect(fetchWarehouseDAU({ gameId: 'g1', env: 'e1' })).resolves.toEqual({ points: [] });
    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/analytics/warehouse/dau', {
      params: { gameId: 'g1', env: 'e1' },
    });

    await expect(fetchWarehouseOnline()).resolves.toEqual({ points: [] });
    expect(mockedRequest).toHaveBeenNthCalledWith(2, '/api/v1/analytics/warehouse/online', {
      params: { gameId: 'game-a', env: 'prod' },
    });

    await expect(fetchWarehouseRevenue()).resolves.toEqual({ points: [] });
    expect(mockedRequest).toHaveBeenNthCalledWith(3, '/api/v1/analytics/warehouse/revenue', {
      params: { gameId: 'game-a', env: 'prod' },
    });
  });
});
