import { request } from '@umijs/max';
import { getScope } from '@/stores/scope';
import { createEventSource } from '../core/http';
import type { JSONValue } from '@/types/dashboard';

type AnalyticsParams = Record<string, JSONValue>;

function normalizeAnalyticsParams(params?: AnalyticsParams): AnalyticsParams {
  const source = { ...(params || {}) };
  if (source.startDate == null && source.start != null) source.startDate = source.start;
  if (source.endDate == null && source.end != null) source.endDate = source.end;
  delete source.start;
  delete source.end;
  return source;
}

function analyticsRequestParams(params?: AnalyticsParams) {
  return withAnalyticsScope(normalizeAnalyticsParams(params));
}

function withAnalyticsScope(params?: AnalyticsParams): AnalyticsParams {
  const scope = getScope();
  return {
    ...(params || {}),
    ...(params?.gameId ? {} : scope.gameId ? { gameId: scope.gameId } : {}),
    ...(params?.env ? {} : scope.env ? { env: scope.env } : {}),
  };
}

// Overview KPI
export async function fetchAnalyticsOverview(params?: AnalyticsParams) {
  const response = await request<{
    metrics?: {
      dau?: number;
      mau?: number;
      newUsers?: number;
      revenue?: number;
      arpu?: number;
      arppu?: number;
      payingRate?: number;
    };
    trends?: {
      newUsers?: Array<{ date?: string; value?: number }>;
      activeUsers?: Array<{ date?: string; value?: number }>;
      revenue?: Array<{ date?: string; revenue?: number; value?: number }>;
    };
  }>('/api/v1/analytics/overview', { params: analyticsRequestParams(params) });
  const metrics = response.metrics || {};
  const trends = response.trends || {};
  const tuple = (items?: Array<{ date?: string; value?: number }>) =>
    (items || []).map((item) => [item.date || '', Number(item.value || 0)] as [string, number]);
  return {
    dau: metrics.dau ?? 0,
    mau: metrics.mau ?? 0,
    newUsers: metrics.newUsers ?? 0,
    revenue: metrics.revenue ?? 0,
    arpu: metrics.arpu ?? 0,
    arppu: metrics.arppu ?? 0,
    payRate: metrics.payingRate == null ? null : metrics.payingRate * 100,
    wau: null,
    registeredTotal: null,
    d1: null,
    d7: null,
    d30: null,
    series: {
      newUsers: tuple(trends.newUsers),
      peakOnline: tuple(trends.activeUsers),
      revenue: (trends.revenue || []).map(
        (item) => [item.date || '', Number(item.revenue ?? item.value ?? 0)] as [string, number],
      ),
    },
  };
}

// Retention (cohort)
export async function fetchAnalyticsRetention(params?: AnalyticsParams) {
  return request('/api/v1/analytics/retention', { params: analyticsRequestParams(params) });
}

// Realtime screen
export async function fetchAnalyticsRealtime(params?: AnalyticsParams) {
  return request('/api/v1/analytics/realtime', { params: analyticsRequestParams(params) });
}

export function openAnalyticsRealtimeEventSource(params?: AnalyticsParams) {
  const scoped = withAnalyticsScope(params);
  return createEventSource('/api/v1/analytics/realtime', {
    params: Object.fromEntries(
      Object.entries(scoped)
        .filter(([, value]) => value !== undefined && value !== null)
        .map(([key, value]) => [
          key,
          typeof value === 'string' || typeof value === 'number' ? value : JSON.stringify(value),
        ]),
    ),
  });
}

export async function fetchRealtimeSeries(params: AnalyticsParams) {
  const source = analyticsRequestParams(params);
  if (
    source.duration == null &&
    typeof source.startDate === 'string' &&
    typeof source.endDate === 'string'
  ) {
    const start = Date.parse(source.startDate);
    const end = Date.parse(source.endDate);
    if (Number.isFinite(start) && Number.isFinite(end) && end > start) {
      source.duration = Math.max(1, Math.ceil((end - start) / 60000));
    }
  }
  delete source.startDate;
  delete source.endDate;
  const response = await request<{
    series?: {
      users?: Array<{ timestamp?: string; value?: number }>;
      events?: Array<{ timestamp?: string; value?: number }>;
    };
  }>('/api/v1/analytics/realtime/series', {
    params: source,
  });
  const toTuples = (items?: Array<{ timestamp?: string; value?: number }>) =>
    (items || []).map(
      (item) => [item.timestamp || '', Number(item.value || 0)] as [string, number],
    );
  return {
    online: toTuples(response.series?.users),
    active5MSum: toTuples(response.series?.events),
    active15MSum: [],
    revenueCents: [],
  };
}

// Behavior events and funnel
export async function fetchAnalyticsEvents(params?: AnalyticsParams) {
  const source = analyticsRequestParams(params);
  if (source.eventType == null && source.event != null) source.eventType = source.event;
  delete source.event;
  const response = await request<{
    items?: Array<{ eventType?: string; userId?: string; data?: JSONValue; timestamp?: string }>;
    total?: number;
  }>('/api/v1/analytics/behavior/events', { params: source });
  return {
    events: (response.items || []).map((item) => ({
      event: item.eventType || '',
      userId: item.userId || '',
      time: item.timestamp || '',
      data: item.data,
    })),
    total: response.total || 0,
  };
}
export async function fetchAnalyticsFunnel(params?: AnalyticsParams) {
  const source = analyticsRequestParams(params);
  if (typeof source.steps === 'string') {
    source.steps = source.steps
      .split(',')
      .map((item) => item.trim())
      .filter(Boolean);
  }
  const response = await request<{
    steps?: Array<{ step?: string; users?: number; conversionRate?: number; dropOffRate?: number }>;
  }>('/api/v1/analytics/behavior/funnel', { params: source });
  return {
    steps: (response.steps || []).map((item) => ({
      step: item.step || '',
      users: item.users || 0,
      rate: item.conversionRate || 0,
      conversionRate: item.conversionRate || 0,
      dropOffRate: item.dropOffRate || 0,
    })),
  };
}

// Behavior paths (Top N)
export async function fetchAnalyticsPaths(params?: AnalyticsParams) {
  const source = analyticsRequestParams(params);
  if (source.depth == null && source.steps != null) source.depth = source.steps;
  delete source.steps;
  const response = await request<{
    paths?:
      | { items?: Array<{ path?: string | string[]; groups?: number; count?: number }> }
      | Array<{ path?: string | string[]; groups?: number; count?: number }>;
  }>('/api/v1/analytics/behavior/paths', { params: source });
  const paths = (Array.isArray(response.paths) ? response.paths : response.paths?.items || []).map(
    (item) => ({
      path: Array.isArray(item.path) ? item.path.join('>') : item.path || '',
      groups: item.groups ?? item.count ?? 0,
    }),
  );
  return { paths };
}

// Feature adoption
export async function fetchAnalyticsAdoption(params?: AnalyticsParams) {
  const source = analyticsRequestParams(params);
  if (source.feature == null && typeof source.features === 'string') {
    source.feature = source.features.split(',')[0] || '';
  }
  delete source.features;
  const response = await request<{
    features?: Array<{
      feature?: string;
      users?: number;
      adoptionRate?: number;
      frequency?: number;
    }>;
  }>('/api/v1/analytics/behavior/adoption', { params: source });
  return {
    features: (response.features || []).map((item) => ({
      feature: item.feature || '',
      groups: item.users || 0,
      rate: item.adoptionRate == null ? 0 : item.adoptionRate * 100,
      frequency: item.frequency || 0,
    })),
    baseline: 0,
  };
}

export async function fetchAnalyticsAdoptionBreakdown(params?: AnalyticsParams) {
  const source = analyticsRequestParams(params);
  if (source.feature == null && typeof source.features === 'string') {
    source.feature = source.features.split(',')[0] || '';
  }
  delete source.features;
  const response = await request<{
    bySegment?: {
      regions?: Record<string, number>;
      platforms?: Record<string, number>;
      roles?: Record<string, number>;
    };
  }>('/api/v1/analytics/behavior/adoption/breakdown', { params: source });
  const by = String(source.by || 'channel');
  const segmentKey = by === 'platform' ? 'platforms' : by === 'country' ? 'roles' : 'regions';
  const values = response.bySegment?.[segmentKey] || {};
  return {
    by,
    rows: Object.entries(values).map(([dim, groups]) => ({ dim, groups, baseline: 0, rate: 0 })),
  };
}

// Payments
export async function fetchAnalyticsPaymentsSummary(params?: AnalyticsParams) {
  const source = analyticsRequestParams(params);
  if (source.groupBy == null) source.groupBy = 'day';
  const response = await request<{
    items?: Array<{ date?: string; revenue?: number; transactions?: number; users?: number }>;
  }>('/api/v1/analytics/payments/summary', { params: source });
  const items = (response.items || []).map((item) => ({
    date: item.date || '',
    revenue: Number(item.revenue || 0),
    transactions: Number(item.transactions || 0),
    users: Number(item.users || 0),
  }));
  return {
    totals: {
      revenue: items.reduce((sum, item) => sum + Number(item.revenue || 0), 0),
      transactions: items.reduce((sum, item) => sum + Number(item.transactions || 0), 0),
      users: items.reduce((sum, item) => sum + Number(item.users || 0), 0),
    },
    byChannel: [],
    byPlatform: [],
    byCountry: [],
    byRegion: [],
    byCity: [],
    byProduct: [],
    items,
  };
}
export async function fetchAnalyticsTransactions(params?: AnalyticsParams) {
  const source = analyticsRequestParams(params);
  if (source.pageSize == null && source.size != null) source.pageSize = source.size;
  delete source.size;
  const response = await request<{
    items?: Array<{
      id?: string;
      userId?: string;
      productId?: string;
      amount?: number;
      currency?: string;
      status?: string;
      paymentMethod?: string;
      createdAt?: string;
    }>;
    total?: number;
    page?: number;
    size?: number;
  }>('/api/v1/analytics/payments/transactions', { params: source });
  return {
    transactions: (response.items || []).map((item) => ({
      orderId: item.id || '',
      userId: item.userId || '',
      productId: item.productId || '',
      amount: item.amount || 0,
      status: item.status || '',
      time: item.createdAt || '',
      channel: item.paymentMethod || '',
      currency: item.currency || '',
    })),
    total: response.total || 0,
  };
}

export type AnalyticsLevelMetric = {
  levelId: string;
  attempts: number;
  completions: number;
  completionRate: number;
  avgDuration: number;
  avgRetries: number;
};

export type AnalyticsEpisodeMetric = {
  episodeId: string;
  players: number;
  completionRate: number;
  avgProgress: number;
};

export type AnalyticsMapMetric = {
  mapId: string;
  heatMap: Array<Record<string, number>>;
  deathSpots: Array<Record<string, number>>;
};

// Levels (funnel + winrate + time + retries)
export async function fetchAnalyticsLevels(params?: AnalyticsParams) {
  return request<{ levels?: AnalyticsLevelMetric[] }>('/api/v1/analytics/levels', {
    params: analyticsRequestParams(params),
  });
}
export async function fetchAnalyticsLevelsEpisodes(params?: AnalyticsParams) {
  return request<{ episodes?: AnalyticsEpisodeMetric[] }>('/api/v1/analytics/levels/episodes', {
    params: analyticsRequestParams(params),
  });
}
export async function fetchAnalyticsLevelsMaps(params?: AnalyticsParams) {
  return request<{ maps?: AnalyticsMapMetric[] }>('/api/v1/analytics/levels/maps', {
    params: analyticsRequestParams(params),
  });
}

// Payments product trend
export async function fetchProductTrend(params: AnalyticsParams) {
  const response = await request<{
    items?: Array<{
      productId?: string;
      productName?: string;
      revenue?: number;
      sales?: number;
      growth?: number;
    }>;
  }>('/api/v1/analytics/payments/product-trend', { params: analyticsRequestParams(params) });
  return {
    products: (response.items || []).map((item) => ({
      productId: item.productId || '',
      points: [
        {
          time: '',
          amount: Number(item.revenue || 0),
          count: Number(item.sales || 0),
        },
      ],
    })),
  };
}

// Source: croupier/internal/api/analytics/dto.go AnalyticsFilters
export type AnalyticsFilters = {
  gameId: string;
  env: string;
  events: string[];
  paymentsEnabled: boolean;
  sampleGlobal: number;
};

type RawAnalyticsFilters = {
  gameId?: string;
  env?: string;
  events?: string[];
  paymentsEnabled?: boolean;
  sampleGlobal?: number;
};

function normalizeAnalyticsFilters(raw?: RawAnalyticsFilters): AnalyticsFilters {
  return {
    gameId: raw?.gameId ?? '',
    env: raw?.env ?? '',
    events: Array.isArray(raw?.events) ? raw.events.map((item) => String(item)) : [],
    paymentsEnabled: raw?.paymentsEnabled ?? true,
    sampleGlobal: Number(raw?.sampleGlobal ?? 100),
  };
}

export async function fetchAnalyticsFilters(params: { gameId: string; env: string }) {
  const response = await request<RawAnalyticsFilters>('/api/v1/analytics/filters', {
    params: {
      gameId: params.gameId,
      env: params.env,
    },
  });
  return normalizeAnalyticsFilters(response);
}

export async function saveAnalyticsFilters(data: AnalyticsFilters) {
  return request<void>('/api/v1/analytics/filters', {
    method: 'PUT',
    data: {
      gameId: data.gameId,
      env: data.env,
      events: data.events,
      paymentsEnabled: data.paymentsEnabled,
      sampleGlobal: data.sampleGlobal,
    },
  });
}

// ---------------------------------------------------------------------------
// Invocation analytics (audit-based: function.invoke + page.execute)
// Source: croupier/internal/api/analytics/invocations.go
// ---------------------------------------------------------------------------

export type InvocationsTrendPoint = {
  bucket: string;
  total: number;
  failed: number;
};

export type InvocationFunctionStats = {
  functionId: string;
  total: number;
  failed: number;
  avgDurationMs: number;
};

export type InvocationsSummary = {
  total: number;
  failed: number;
  successRate: number;
  avgDurationMs: number;
  p95DurationMs: number;
  topFunctions: InvocationFunctionStats[];
};

export type InvocationItem = {
  timestamp: string;
  functionId: string;
  actor: string;
  outcome: string;
  error?: string;
  durationMs?: number;
  traceId?: string;
  gameId?: string;
  env?: string;
};

export type InvocationsList = {
  items: InvocationItem[];
  total: number;
  page: number;
  pageSize: number;
};

export async function fetchInvocationsSummary(params?: AnalyticsParams) {
  return request<InvocationsSummary>('/api/v1/analytics/invocations/summary', {
    params: analyticsRequestParams(params),
  });
}

export async function fetchInvocationsTrend(params?: AnalyticsParams) {
  return request<{ points?: InvocationsTrendPoint[] }>('/api/v1/analytics/invocations/trend', {
    params: analyticsRequestParams(params),
  });
}

export async function fetchInvocationsList(params?: AnalyticsParams) {
  return request<InvocationsList>('/api/v1/analytics/invocations', {
    params: analyticsRequestParams(params),
  });
}

// ---------------------------------------------------------------------------
// Warehouse (ClickHouse aggregates). 503 when the warehouse is not enabled.
// Source: croupier/internal/api/analytics/warehouse.go
// ---------------------------------------------------------------------------

export type WarehouseDAUPoint = {
  date: string;
  gameId?: string;
  env?: string;
  dau: number;
  newUsers: number;
};

export type WarehouseOnlinePoint = {
  minute: string;
  online: number;
};

export type WarehouseRevenuePoint = {
  date: string;
  gameId?: string;
  env?: string;
  revenueCents: number;
  refundsCents: number;
  failed: number;
};

export async function fetchWarehouseDAU(params?: AnalyticsParams) {
  return request<{ points?: WarehouseDAUPoint[] }>('/api/v1/analytics/warehouse/dau', {
    params: analyticsRequestParams(params),
  });
}

export async function fetchWarehouseOnline(params?: AnalyticsParams) {
  return request<{ points?: WarehouseOnlinePoint[] }>('/api/v1/analytics/warehouse/online', {
    params: analyticsRequestParams(params),
  });
}

export async function fetchWarehouseRevenue(params?: AnalyticsParams) {
  return request<{ points?: WarehouseRevenuePoint[] }>('/api/v1/analytics/warehouse/revenue', {
    params: analyticsRequestParams(params),
  });
}
