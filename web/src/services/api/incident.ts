import { request } from '@umijs/max';

// Source: internal/api/incident/dto.go + report.go

// ---- 类别（incident_categories） ----

export type CategoryAudience = {
  roles: string[];
  users: string[];
};

export type IncidentCategory = {
  id: number;
  name: string;
  slug: string;
  sort: number;
  leader: string;
  subcategories: string[];
  audience?: CategoryAudience;
  enabled: boolean;
  builtin: boolean;
  createdAt: string;
  updatedAt: string;
};

export type CategoryUpsertPayload = {
  name?: string;
  slug?: string;
  sort?: number;
  leader?: string;
  subcategories?: string[];
  audience?: CategoryAudience;
  enabled?: boolean;
};

export type CategoryUsage = {
  incidents: number;
  bugs: number;
};

export type CategoryListResponse = {
  items: IncidentCategory[];
  total: number;
};

// ---- 事故（incidents） ----

export type IncidentItem = {
  id: number;
  incidentKey?: string;
  title: string;
  categoryId: number;
  categorySlug?: string;
  categoryName?: string;
  subcategory?: string;
  severity: string;
  status: string;
  source: string;
  responsibleType?: string;
  responsibleId?: string;
  detectedAt: string;
  resolvedAt?: string;
  gameId?: string;
  env?: string;
  execLogIds?: number[];
  refType?: string;
  refId?: string;
  details?: Record<string, unknown>;
  createdBy?: string;
  createdAt: string;
  updatedAt: string;
};

export type IncidentCreatePayload = {
  title: string;
  categoryId: number;
  subcategory?: string;
  severity?: string;
  detectedAt?: string;
  responsibleType?: string;
  responsibleId?: string;
  gameId?: string;
  env?: string;
  execLogIds?: number[];
  refType?: string;
  refId?: string;
  details?: Record<string, unknown>;
  incidentKey?: string;
};

export type IncidentUpdatePayload = {
  title?: string;
  categoryId?: number;
  subcategory?: string;
  severity?: string;
  responsibleType?: string;
  responsibleId?: string;
  gameId?: string;
  env?: string;
  execLogIds?: number[];
  refType?: string;
  refId?: string;
  details?: Record<string, unknown>;
};

export type IncidentListParams = {
  page?: number;
  pageSize?: number;
  categoryId?: number;
  subcategory?: string;
  status?: string;
  severity?: string;
  source?: string;
  responsibleType?: string;
  responsibleId?: string;
  gameId?: string;
  env?: string;
  from?: string;
  to?: string;
};

export type IncidentListResponse = {
  items: IncidentItem[];
  total: number;
};

// ---- 报表（incident-reports） ----

export type CompareDelta = {
  value?: number | null;
  delta?: number | null;
  pct?: number | null;
  missing?: boolean;
};

export type CompareValue = {
  value?: number | null;
  prev?: CompareDelta;
  yoy?: CompareDelta;
};

export type CategoryShare = {
  categoryId: number;
  slug?: string;
  name?: string;
  count: number;
  sharePct?: number | null;
};

export type ReportSummary = {
  period: string;
  periodKey: string;
  start: string;
  end: string;
  generatedAt: string;
  metrics: {
    total: CompareValue;
    duration: CompareValue;
    mttr: CompareValue;
    recurrence: CompareValue;
  };
  incidents: number;
  bugs: number;
  categoryBreakdown: CategoryShare[];
  resolvedSample: number;
  recurrenceChains: number;
};

export type TrendCategory = {
  id: number;
  slug?: string;
  name?: string;
};

export type TrendPoint = {
  t: string;
  total: number;
  byCategory: Record<string, number>;
};

export type ReportTrend = {
  bucket: string;
  start: string;
  end: string;
  categories: TrendCategory[];
  points: TrendPoint[];
  yoyPoints: TrendPoint[];
  yoyMissing: boolean;
};

export type LeaderboardRow = {
  key: string;
  label: string;
  categoryId?: number;
  categoryName?: string;
  categorySlug?: string;
  value: number;
  incidents: number;
  bugs: number;
  durationMs: number;
  resolved: number;
  mttrMs?: number | null;
  prev?: CompareDelta;
  yoy?: CompareDelta;
};

export type ReportLeaderboard = {
  period: string;
  periodKey: string;
  view: string;
  board: string;
  rows: LeaderboardRow[];
};

export type ResponsibilityOverview = {
  total: CompareValue;
  duration: CompareValue;
  mttr: CompareValue;
  recurrence: CompareValue;
};

export type ResponsibilityCategoryRow = {
  categoryId: number;
  slug?: string;
  name?: string;
  leader?: string;
  count: number;
  prev?: CompareDelta;
  yoy?: CompareDelta;
  byResponsible: Record<string, number>;
};

export type ResponsibilityRow = {
  key: string;
  label: string;
  total: number;
  prev?: CompareDelta;
  yoy?: CompareDelta;
  byCategory: Record<string, number>;
  durationMs: number;
  mttrMs?: number | null;
  resolved: number;
  recurrences: number;
};

export type UnattributedIncident = {
  id: number;
  title: string;
  categoryId: number;
  detectedAt: string;
  severity: string;
};

export type ResponsibilityReport = {
  periodType: string;
  periodKey: string;
  start: string;
  end: string;
  generatedAt: string;
  overview: ResponsibilityOverview;
  categories: ResponsibilityCategoryRow[];
  responsibles: ResponsibilityRow[];
  unattributed: UnattributedIncident[];
};

// ---- 类别 API ----

export async function fetchIncidentCategories(enabledOnly?: boolean) {
  return request<CategoryListResponse>('/api/v1/incident-categories', {
    params: enabledOnly ? { enabledOnly: true } : undefined,
  });
}

export async function createIncidentCategory(payload: CategoryUpsertPayload) {
  return request<IncidentCategory>('/api/v1/incident-categories', {
    method: 'POST',
    data: payload,
  });
}

export async function updateIncidentCategory(id: number, payload: CategoryUpsertPayload) {
  return request<IncidentCategory>(`/api/v1/incident-categories/${id}`, {
    method: 'PUT',
    data: payload,
  });
}

export async function fetchIncidentCategoryUsage(id: number) {
  return request<CategoryUsage>(`/api/v1/incident-categories/${id}/usage`);
}

export async function deleteIncidentCategory(id: number) {
  return request<void>(`/api/v1/incident-categories/${id}`, { method: 'DELETE' });
}

// ---- 事故 API ----

export async function fetchIncidents(params?: IncidentListParams) {
  return request<IncidentListResponse>('/api/v1/incidents', { params });
}

export async function fetchIncident(id: number) {
  return request<IncidentItem>(`/api/v1/incidents/${id}`);
}

export async function createIncident(payload: IncidentCreatePayload) {
  return request<IncidentItem>('/api/v1/incidents', { method: 'POST', data: payload });
}

export async function updateIncident(id: number, payload: IncidentUpdatePayload) {
  return request<IncidentItem>(`/api/v1/incidents/${id}`, { method: 'PUT', data: payload });
}

export async function transitionIncident(id: number, status: string, resolvedAt?: string) {
  return request<IncidentItem>(`/api/v1/incidents/${id}/status`, {
    method: 'POST',
    data: resolvedAt ? { status, resolvedAt } : { status },
  });
}

// ---- 报表 API ----

export async function fetchIncidentReportSummary(period: string, periodKey?: string) {
  return request<ReportSummary>('/api/v1/incident-reports/summary', {
    params: periodKey ? { period, key: periodKey } : { period },
  });
}

export async function fetchIncidentReportTrend(params: {
  bucket: string;
  from?: string;
  to?: string;
}) {
  return request<ReportTrend>('/api/v1/incident-reports/trend', { params });
}

export async function fetchIncidentReportLeaderboard(params: {
  period: string;
  key?: string;
  view: string;
  board: string;
}) {
  return request<ReportLeaderboard>('/api/v1/incident-reports/leaderboard', { params });
}

export async function fetchIncidentResponsibility(period: string, periodKey?: string) {
  return request<ResponsibilityReport>('/api/v1/incident-reports/responsibility', {
    params: periodKey ? { period, key: periodKey } : { period },
  });
}
