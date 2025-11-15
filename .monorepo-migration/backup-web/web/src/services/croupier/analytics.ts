import { request } from '@umijs/max';

// Overview KPI
export async function fetchAnalyticsOverview(params?: any) {
  try { return await request('/api/analytics/overview', { params }); } catch { return {}; }
}

// Retention (cohort)
export async function fetchAnalyticsRetention(params?: any) {
  try { return await request('/api/analytics/retention', { params }); } catch { return { cohorts: [] }; }
}

// Realtime screen
export async function fetchAnalyticsRealtime() {
  try { return await request('/api/analytics/realtime'); } catch { return {}; }
}

export async function fetchRealtimeSeries(params: any) {
  try { return await request('/api/analytics/realtime/series', { params }); } catch { return { online: [], revenue_cents: [] }; }
}

// Behavior events and funnel
export async function fetchAnalyticsEvents(params?: any) {
  try { return await request('/api/analytics/behavior/events', { params }); } catch { return { events: [], total: 0 }; }
}
export async function fetchAnalyticsFunnel(params?: any) {
  try { return await request('/api/analytics/behavior/funnel', { params }); } catch { return { steps: [] }; }
}

// Behavior paths (Top N)
export async function fetchAnalyticsPaths(params?: any) {
  try { return await request('/api/analytics/behavior/paths', { params }); } catch { return { paths: [] }; }
}

// Feature adoption
export async function fetchAnalyticsAdoption(params?: any) {
  try { return await request('/api/analytics/behavior/adoption', { params }); } catch { return { features: [], baseline: 0 }; }
}

export async function fetchAnalyticsAdoptionBreakdown(params?: any) {
  try { return await request('/api/analytics/behavior/adoption_breakdown', { params }); } catch { return { by: 'channel', rows: [] }; }
}

// Payments
export async function fetchAnalyticsPaymentsSummary(params?: any) {
  try { return await request('/api/analytics/payments/summary', { params }); } catch { return { totals: {}, by_channel: [], by_product: [] }; }
}
export async function fetchAnalyticsTransactions(params?: any) {
  try { return await request('/api/analytics/payments/transactions', { params }); } catch { return { transactions: [], total: 0 }; }
}

// Levels (funnel + winrate + time + retries)
export async function fetchAnalyticsLevels(params?: any) {
  try { return await request('/api/analytics/levels', { params }); } catch { return { funnel: [], per_level: [] }; }
}
export async function fetchAnalyticsLevelsEpisodes(params?: any) {
  try { return await request('/api/analytics/levels/episodes', { params }); } catch { return { episodes: [] }; }
}
export async function fetchAnalyticsLevelsMaps(params?: any) {
  try { return await request('/api/analytics/levels/maps', { params }); } catch { return { maps: [] }; }
}

// Attribution & Segments
export async function fetchAnalyticsAttribution(params?: any) {
  try { return await request('/api/analytics/attribution', { params }); } catch { return { summary: {}, by_channel: [], by_campaign: [] }; }
}
export async function fetchAnalyticsSegments(params?: any) {
  try { return await request('/api/analytics/segments', { params }); } catch { return { segments: [] }; }
}

// Payments product trend
export async function fetchProductTrend(params: any) {
  try { return await request('/api/analytics/payments/product_trend', { params }); } catch { return { products: [] }; }
}
