import { request } from '@umijs/max';

// Overview KPI
export async function fetchAnalyticsOverview(params?: any) {
  try { return await request('/api/v1/analytics/overview', { params }); } catch { return {}; }
}

// Retention (cohort)
export async function fetchAnalyticsRetention(params?: any) {
  try { return await request('/api/v1/analytics/retention', { params }); } catch { return { cohorts: [] }; }
}

// Realtime screen
export async function fetchAnalyticsRealtime() {
  try { return await request('/api/v1/analytics/realtime'); } catch { return {}; }
}

export async function fetchRealtimeSeries(params: any) {
  try { return await request('/api/v1/analytics/realtime/series', { params }); } catch { return { online: [], revenue_cents: [] }; }
}

// Behavior events and funnel
export async function fetchAnalyticsEvents(params?: any) {
  try { return await request('/api/v1/analytics/behavior/events', { params }); } catch { return { events: [], total: 0 }; }
}
export async function fetchAnalyticsFunnel(params?: any) {
  try { return await request('/api/v1/analytics/behavior/funnel', { params }); } catch { return { steps: [] }; }
}

// Behavior paths (Top N)
export async function fetchAnalyticsPaths(params?: any) {
  try { return await request('/api/v1/analytics/behavior/paths', { params }); } catch { return { paths: [] }; }
}

// Feature adoption
export async function fetchAnalyticsAdoption(params?: any) {
  try { return await request('/api/v1/analytics/behavior/adoption', { params }); } catch { return { features: [], baseline: 0 }; }
}

export async function fetchAnalyticsAdoptionBreakdown(params?: any) {
  try { return await request('/api/v1/analytics/behavior/adoption/breakdown', { params }); } catch { return { by: 'channel', rows: [] }; }
}

// Payments
export async function fetchAnalyticsPaymentsSummary(params?: any) {
  try { return await request('/api/v1/analytics/payments/summary', { params }); } catch { return { totals: {}, by_channel: [], by_product: [] }; }
}
export async function fetchAnalyticsTransactions(params?: any) {
  try { return await request('/api/v1/analytics/payments/transactions', { params }); } catch { return { transactions: [], total: 0 }; }
}

// Levels (funnel + winrate + time + retries)
export async function fetchAnalyticsLevels(params?: any) {
  try { return await request('/api/v1/analytics/levels', { params }); } catch { return { funnel: [], per_level: [] }; }
}
export async function fetchAnalyticsLevelsEpisodes(params?: any) {
  try { return await request('/api/v1/analytics/levels/episodes', { params }); } catch { return { episodes: [] }; }
}
export async function fetchAnalyticsLevelsMaps(params?: any) {
  try { return await request('/api/v1/analytics/levels/maps', { params }); } catch { return { maps: [] }; }
}

// Payments product trend
export async function fetchProductTrend(params: any) {
  try { return await request('/api/v1/analytics/payments/product-trend', { params }); } catch { return { products: [] }; }
}

// Attribution & Segments - 注意：这些端点在后端不存在，暂时保留但会返回空数据
export async function fetchAnalyticsAttribution(params?: any) {
  console.warn('API /api/v1/analytics/attribution 在后端不存在');
  return { summary: {}, by_channel: [], by_campaign: [] };
}

export async function fetchAnalyticsSegments(params?: any) {
  console.warn('API /api/v1/analytics/segments 在后端不存在');
  return { segments: [] };
}
