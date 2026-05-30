import { request } from '@umijs/max';
import { getScope } from '@/stores/scope';
import { createEventSource } from '../core/http';

function withAnalyticsScope(params?: Record<string, any>) {
  const scope = getScope();
  return {
    ...(params || {}),
    ...(params?.gameId ? {} : scope.gameId ? { gameId: scope.gameId } : {}),
    ...(params?.env ? {} : scope.env ? { env: scope.env } : {}),
  };
}

// Overview KPI
export async function fetchAnalyticsOverview(params?: any) {
  try {
    return await request('/api/v1/analytics/overview', { params: withAnalyticsScope(params) });
  } catch {
    return {};
  }
}

// Retention (cohort)
export async function fetchAnalyticsRetention(params?: any) {
  try {
    return await request('/api/v1/analytics/retention', { params: withAnalyticsScope(params) });
  } catch {
    return { cohorts: [] };
  }
}

// Realtime screen
export async function fetchAnalyticsRealtime(params?: any) {
  try {
    return await request('/api/v1/analytics/realtime', { params: withAnalyticsScope(params) });
  } catch {
    return {};
  }
}

export function openAnalyticsRealtimeEventSource(params?: Record<string, any>) {
  const scoped = withAnalyticsScope(params);
  return createEventSource('/api/v1/analytics/realtime', {
    params: Object.fromEntries(
      Object.entries(scoped).filter(([, value]) => value !== undefined && value !== null),
    ),
  });
}

export async function fetchRealtimeSeries(params: any) {
  try {
    return await request('/api/v1/analytics/realtime/series', {
      params: withAnalyticsScope(params),
    });
  } catch {
    return { online: [], revenue_cents: [] };
  }
}

// Behavior events and funnel
export async function fetchAnalyticsEvents(params?: any) {
  try {
    return await request('/api/v1/analytics/behavior/events', {
      params: withAnalyticsScope(params),
    });
  } catch {
    return { events: [], total: 0 };
  }
}
export async function fetchAnalyticsFunnel(params?: any) {
  try {
    return await request('/api/v1/analytics/behavior/funnel', {
      params: withAnalyticsScope(params),
    });
  } catch {
    return { steps: [] };
  }
}

// Behavior paths (Top N)
export async function fetchAnalyticsPaths(params?: any) {
  try {
    return await request('/api/v1/analytics/behavior/paths', {
      params: withAnalyticsScope(params),
    });
  } catch {
    return { paths: [] };
  }
}

// Feature adoption
export async function fetchAnalyticsAdoption(params?: any) {
  try {
    return await request('/api/v1/analytics/behavior/adoption', {
      params: withAnalyticsScope(params),
    });
  } catch {
    return { features: [], baseline: 0 };
  }
}

export async function fetchAnalyticsAdoptionBreakdown(params?: any) {
  try {
    return await request('/api/v1/analytics/behavior/adoption/breakdown', {
      params: withAnalyticsScope(params),
    });
  } catch {
    return { by: 'channel', rows: [] };
  }
}

// Payments
export async function fetchAnalyticsPaymentsSummary(params?: any) {
  try {
    return await request('/api/v1/analytics/payments/summary', {
      params: withAnalyticsScope(params),
    });
  } catch {
    return { totals: {}, by_channel: [], by_product: [] };
  }
}
export async function fetchAnalyticsTransactions(params?: any) {
  try {
    return await request('/api/v1/analytics/payments/transactions', {
      params: withAnalyticsScope(params),
    });
  } catch {
    return { transactions: [], total: 0 };
  }
}

// Levels (funnel + winrate + time + retries)
export async function fetchAnalyticsLevels(params?: any) {
  try {
    return await request('/api/v1/analytics/levels', { params: withAnalyticsScope(params) });
  } catch {
    return { funnel: [], per_level: [] };
  }
}
export async function fetchAnalyticsLevelsEpisodes(params?: any) {
  try {
    return await request('/api/v1/analytics/levels/episodes', {
      params: withAnalyticsScope(params),
    });
  } catch {
    return { episodes: [] };
  }
}
export async function fetchAnalyticsLevelsMaps(params?: any) {
  try {
    return await request('/api/v1/analytics/levels/maps', {
      params: withAnalyticsScope(params),
    });
  } catch {
    return { maps: [] };
  }
}

// Payments product trend
export async function fetchProductTrend(params: any) {
  try {
    return await request('/api/v1/analytics/payments/product-trend', {
      params: withAnalyticsScope(params),
    });
  } catch {
    return { products: [] };
  }
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
  game_id?: string;
  env?: string;
  events?: string[];
  paymentsEnabled?: boolean;
  payments_enabled?: boolean;
  sampleGlobal?: number;
  sample_global?: number;
};

function normalizeAnalyticsFilters(raw?: RawAnalyticsFilters): AnalyticsFilters {
  return {
    gameId: raw?.gameId ?? raw?.game_id ?? '',
    env: raw?.env ?? '',
    events: Array.isArray(raw?.events) ? raw.events.map((item) => String(item)) : [],
    paymentsEnabled: raw?.paymentsEnabled ?? raw?.payments_enabled ?? true,
    sampleGlobal: Number(raw?.sampleGlobal ?? raw?.sample_global ?? 100),
  };
}

export async function fetchAnalyticsFilters(params: { gameId: string; env: string }) {
  const response = await request<RawAnalyticsFilters>('/api/v1/analytics/filters', {
    params: {
      game_id: params.gameId,
      env: params.env,
    },
  });
  return normalizeAnalyticsFilters(response);
}

export async function saveAnalyticsFilters(data: AnalyticsFilters) {
  return request<void>('/api/v1/analytics/filters', {
    method: 'PUT',
    data: {
      game_id: data.gameId,
      env: data.env,
      events: data.events,
      payments_enabled: data.paymentsEnabled,
      sample_global: data.sampleGlobal,
    },
  });
}
