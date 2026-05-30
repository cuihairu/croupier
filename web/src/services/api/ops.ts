import { request } from '@umijs/max';
import { buildDownloadUrl } from '../core/http';

// Source: croupier/internal/api/ops/dto.go OpsAgentInfo and legacy ops agent listings.
export type OpsAgent = {
  agentId: string;
  gameId: string;
  env: string;
  rpcAddr: string;
  ip?: string;
  type?: string;
  version?: string;
  functions: number;
  healthy: boolean;
  expiresInSec: number;
  activeConns?: number;
  totalRequests?: number;
  failedRequests?: number;
  errorRate?: number;
  avgLatencyMs?: number;
  lastSeen?: string;
  qpsLimit?: number;
  qps1m?: number;
};

// Source: croupier/internal/api/ops/dto.go OpsServiceProcess.
export type AgentProcess = {
  serviceId: string;
  addr?: string;
  version?: string;
  lastSeenUnix?: number;
  functionIds?: string[];
  functions?: number;
};

export type RateLimitRule = {
  scope: 'function' | 'service';
  key: string;
  limitQps: number;
  match?: Record<string, string>;
  percent?: number;
};
export type RateLimitPreviewAgent = {
  agentId: string;
  gameId?: string;
  env?: string;
  region?: string;
  zone?: string;
  rpcAddr?: string;
  qps: number;
  qps1m?: number;
};
type RawRateLimitRule = {
  scope: 'function' | 'service';
  key: string;
  limit_qps: number;
  match?: Record<string, string>;
  percent?: number;
};
type RawRateLimitPreviewAgent = {
  agent_id: string;
  game_id?: string;
  env?: string;
  region?: string;
  zone?: string;
  rpc_addr?: string;
  qps: number;
  qps_1m?: number;
};
function normalizeRateLimitRule(raw: RawRateLimitRule): RateLimitRule {
  return {
    scope: raw.scope,
    key: raw.key,
    limitQps: raw.limit_qps,
    match: raw.match,
    percent: raw.percent,
  };
}
function normalizeRateLimitPreviewAgent(raw: RawRateLimitPreviewAgent): RateLimitPreviewAgent {
  return {
    agentId: raw.agent_id,
    gameId: raw.game_id,
    env: raw.env,
    region: raw.region,
    zone: raw.zone,
    rpcAddr: raw.rpc_addr,
    qps: raw.qps,
    qps1m: raw.qps_1m,
  };
}
const RATE_LIMIT_BASE = '/api/v1/rate-limits';
export async function listRateLimits() {
  const response = await request<{ rules: RawRateLimitRule[] }>(RATE_LIMIT_BASE);
  return { rules: (response.rules || []).map(normalizeRateLimitRule) };
}
export async function putRateLimits(rules: RateLimitRule[]) {
  return request<void>(RATE_LIMIT_BASE, {
    method: 'PUT',
    data: {
      rules: rules.map((rule) => ({
        scope: rule.scope,
        key: rule.key,
        limit_qps: rule.limitQps,
        match: rule.match,
        percent: rule.percent,
      })),
    },
  });
}
export async function deleteRateLimit(scope: string, key: string) {
  return request<void>(
    `${RATE_LIMIT_BASE}?scope=${encodeURIComponent(scope)}&key=${encodeURIComponent(key)}`,
    { method: 'DELETE' },
  );
}
export async function previewRateLimit(params: {
  scope: 'service';
  key?: string;
  limitQps: number;
  percent?: number;
  matchGameId?: string;
  matchEnv?: string;
  matchRegion?: string;
  matchZone?: string;
}) {
  const response = await request<{
    matched: number;
    agents: RawRateLimitPreviewAgent[];
  }>(`${RATE_LIMIT_BASE}/preview`, {
    params: {
      scope: params.scope,
      key: params.key,
      limit_qps: params.limitQps,
      percent: params.percent,
      match_game_id: params.matchGameId,
      match_env: params.matchEnv,
      match_region: params.matchRegion,
      match_zone: params.matchZone,
    },
  });
  return {
    matched: response.matched,
    agents: (response.agents || []).map(normalizeRateLimitPreviewAgent),
  };
}

export async function listOpsFunctions() {
  return request<{ functions: { id: string; category?: string }[] }>('/api/v1/ops/functions');
}

// Source: croupier/internal/api/ops/dto.go OpsConfigResponse
export type OpsConfig = {
  alertmanagerUrl?: string;
  grafanaExploreUrl?: string;
  jaegerUrl?: string;
};

// Source: croupier/internal/api/ops/dto.go Backup / OpsBackupsListResponse
export type OpsBackup = {
  id: string;
  name?: string;
  type?: string;
  status?: string;
  size?: number;
  createdAt?: string;
};

// Source: croupier/internal/api/ops/dto.go OpsNotificationChannel / OpsNotificationRule / OpsNotificationsGetResponse
export type OpsNotificationChannel = {
  id: string;
  type: string;
  url?: string;
  secret?: string;
};

export type OpsNotificationRule = {
  event: string;
  channels: string[];
  thresholdDays?: number;
};

export type OpsNotifications = {
  enabled: boolean;
  channels: OpsNotificationChannel[];
  rules: OpsNotificationRule[];
};

// Source: croupier/internal/api/ops/dto.go Silence / OpsSilencesResponse
export type OpsSilence = {
  id: string;
  alertType?: string;
  matchers?: unknown;
  startAt?: string;
  endAt?: string;
  createdBy?: string;
};

// Source: croupier/internal/api/ops/dto.go Node / OpsNodesResponse
export type OpsNode = {
  id: string;
  hostname?: string;
  addr?: string;
  gameId?: string;
  env?: string;
  status?: string;
  labels?: Record<string, string>;
  lastSeen?: string;
};
export type OpsAlert = {
  severity?: string;
  instance?: string;
  service?: string;
  summary?: string;
  startsAt?: string;
  endsAt?: string;
  duration?: string;
  silenced?: boolean;
  labels?: Record<string, unknown>;
  annotations?: Record<string, unknown>;
};
export type OpsJob = {
  id: string;
  functionId: string;
  actor?: string;
  gameId?: string;
  env?: string;
  state: 'running' | 'succeeded' | 'failed' | 'canceled' | string;
  startedAt?: string;
  endedAt?: string;
  durationMs?: number;
  error?: string;
  rpcAddr?: string;
  traceId?: string;
};

// Source: /api/v1/ops/metrics timeseries payload.
export type OpsMetrics = {
  qps: [number, string][];
  errRate: [number, string][];
  p95Ms: [number, string][];
};

type RawOpsConfig = {
  alertmanager_url?: string;
  grafana_explore_url?: string;
  jaeger_url?: string;
};

type RawOpsNotificationRule = {
  event: string;
  channels: string[];
  thresholdDays?: number;
  threshold_days?: number;
};

type RawOpsSilence = {
  id: string;
  alertType?: string;
  alert_type?: string;
  matchers?: unknown;
  startAt?: string;
  start_at?: string;
  endAt?: string;
  end_at?: string;
  createdBy?: string;
  created_by?: string;
};

type RawOpsNode = {
  id: string;
  hostname?: string;
  addr?: string;
  gameId?: string;
  game_id?: string;
  env?: string;
  status?: string;
  labels?: Record<string, string>;
  lastSeen?: string;
  last_seen?: string;
};
type RawOpsAlert = {
  severity?: string;
  instance?: string;
  service?: string;
  summary?: string;
  starts_at?: string;
  ends_at?: string;
  duration?: string;
  silenced?: boolean;
  labels?: Record<string, unknown>;
  annotations?: Record<string, unknown>;
};
type RawOpsJob = {
  id: string;
  function_id: string;
  actor?: string;
  game_id?: string;
  env?: string;
  state: 'running' | 'succeeded' | 'failed' | 'canceled' | string;
  started_at?: string;
  ended_at?: string;
  duration_ms?: number;
  error?: string;
  rpc_addr?: string;
  trace_id?: string;
};

function normalizeOpsConfig(raw?: RawOpsConfig): OpsConfig {
  return {
    alertmanagerUrl: raw?.alertmanager_url,
    grafanaExploreUrl: raw?.grafana_explore_url,
    jaegerUrl: raw?.jaeger_url,
  };
}

function normalizeOpsBackup(raw: any): OpsBackup {
  return {
    id: raw?.id ?? '',
    name: raw?.name,
    type: raw?.type,
    status: raw?.status,
    size: raw?.size,
    createdAt: raw?.createdAt ?? raw?.created_at,
  };
}

function normalizeOpsNotificationRule(raw: RawOpsNotificationRule): OpsNotificationRule {
  return {
    event: raw?.event ?? '',
    channels: Array.isArray(raw?.channels) ? raw.channels : [],
    thresholdDays: raw?.thresholdDays ?? raw?.threshold_days,
  };
}

function normalizeOpsNotifications(raw: any): OpsNotifications {
  return {
    enabled: !!raw?.enabled,
    channels: Array.isArray(raw?.channels) ? raw.channels : [],
    rules: Array.isArray(raw?.rules) ? raw.rules.map(normalizeOpsNotificationRule) : [],
  };
}

function normalizeOpsSilence(raw: RawOpsSilence): OpsSilence {
  return {
    id: raw?.id ?? '',
    alertType: raw?.alertType ?? raw?.alert_type,
    matchers: raw?.matchers,
    startAt: raw?.startAt ?? raw?.start_at,
    endAt: raw?.endAt ?? raw?.end_at,
    createdBy: raw?.createdBy ?? raw?.created_by,
  };
}

function normalizeOpsNode(raw: RawOpsNode): OpsNode {
  return {
    id: raw?.id ?? '',
    hostname: raw?.hostname,
    addr: raw?.addr,
    gameId: raw?.gameId ?? raw?.game_id,
    env: raw?.env,
    status: raw?.status,
    labels: raw?.labels,
    lastSeen: raw?.lastSeen ?? raw?.last_seen,
  };
}
function normalizeOpsAlert(raw: RawOpsAlert): OpsAlert {
  return {
    severity: raw.severity,
    instance: raw.instance,
    service: raw.service,
    summary: raw.summary,
    startsAt: raw.starts_at,
    endsAt: raw.ends_at,
    duration: raw.duration,
    silenced: raw.silenced,
    labels: raw.labels,
    annotations: raw.annotations,
  };
}
function normalizeOpsJob(raw: RawOpsJob): OpsJob {
  return {
    id: raw.id,
    functionId: raw.function_id,
    actor: raw.actor,
    gameId: raw.game_id,
    env: raw.env,
    state: raw.state,
    startedAt: raw.started_at,
    endedAt: raw.ended_at,
    durationMs: raw.duration_ms,
    error: raw.error,
    rpcAddr: raw.rpc_addr,
    traceId: raw.trace_id,
  };
}
export async function listOpsJobs(params?: {
  status?: string;
  functionId?: string;
  actor?: string;
  gameId?: string;
  env?: string;
  page?: number;
  size?: number;
}) {
  const response = await request<{ jobs: RawOpsJob[]; total: number }>('/api/v1/jobs', {
    params: {
      status: params?.status,
      function_id: params?.functionId,
      actor: params?.actor,
      game_id: params?.gameId,
      env: params?.env,
      page: params?.page,
      size: params?.size,
    },
  });
  return { jobs: (response.jobs || []).map(normalizeOpsJob), total: response.total || 0 };
}

export async function fetchOpsMetrics(params: { instance: string; range?: string; step?: string }) {
  const response = await request<{
    qps: [number, string][];
    err_rate: [number, string][];
    p95_ms: [number, string][];
  }>('/api/v1/ops/metrics', { params });
  return {
    qps: response.qps || [],
    errRate: response.err_rate || [],
    p95Ms: response.p95_ms || [],
  } satisfies OpsMetrics;
}

export async function listSilences() {
  const response = await request<{ silences?: RawOpsSilence[] }>('/api/v1/ops/silences');
  return { silences: (response?.silences || []).map(normalizeOpsSilence) };
}
export async function deleteSilence(id: string) {
  return request<void>(`/api/v1/ops/silences/${encodeURIComponent(id)}`, { method: 'DELETE' });
}
export async function fetchOpsConfig() {
  const response = await request<RawOpsConfig>('/api/v1/ops/config');
  return normalizeOpsConfig(response);
}

export async function updateAgentMeta(agentId: string, data: { region?: string; zone?: string }) {
  return request<void>('/api/v1/ops/agent-meta', {
    method: 'PUT',
    data: {
      agentId,
      ...data,
    },
  });
}

// Registry API 已在 services/api/registry.ts 提供，避免重复导出导致冲突

// Source: /api/v1/certificates certificate inventory payload.
export type Certificate = {
  id: number;
  domain: string;
  port: number;
  issuer?: string;
  subject?: string;
  algorithm?: string;
  keyUsage?: string;
  validFrom?: string;
  validTo?: string;
  daysLeft?: number;
  status?: 'valid' | 'expiring' | 'expired' | 'error' | 'pending';
  lastChecked?: string;
  errorMessage?: string;
  alertDays?: number;
};
type RawCertificate = {
  id?: number;
  ID?: number;
  domain?: string;
  Domain?: string;
  port?: number;
  Port?: number;
  issuer?: string;
  Issuer?: string;
  subject?: string;
  Subject?: string;
  algorithm?: string;
  Algorithm?: string;
  key_usage?: string;
  KeyUsage?: string;
  valid_from?: string;
  ValidFrom?: string;
  valid_to?: string;
  ValidTo?: string;
  days_left?: number;
  DaysLeft?: number;
  status?: Certificate['status'];
  Status?: Certificate['status'];
  last_checked?: string;
  LastChecked?: string;
  error_msg?: string;
  ErrorMsg?: string;
  alert_days?: number;
  AlertDays?: number;
};

function normalizeCertificate(raw: RawCertificate): Certificate {
  return {
    id: raw.id ?? raw.ID ?? 0,
    domain: raw.domain ?? raw.Domain ?? '',
    port: raw.port ?? raw.Port ?? 443,
    issuer: raw.issuer ?? raw.Issuer,
    subject: raw.subject ?? raw.Subject,
    algorithm: raw.algorithm ?? raw.Algorithm,
    keyUsage: raw.key_usage ?? raw.KeyUsage,
    validFrom: raw.valid_from ?? raw.ValidFrom,
    validTo: raw.valid_to ?? raw.ValidTo,
    daysLeft: raw.days_left ?? raw.DaysLeft,
    status: raw.status ?? raw.Status,
    lastChecked: raw.last_checked ?? raw.LastChecked,
    errorMessage: raw.error_msg ?? raw.ErrorMsg,
    alertDays: raw.alert_days ?? raw.AlertDays,
  };
}

export async function listCertificates(params?: { page?: number; size?: number; status?: string }) {
  const r = await request<any>('/api/v1/certificates', { params });
  const raw = (r?.certificates || []) as RawCertificate[];
  return {
    certificates: raw.map(normalizeCertificate),
    total: r?.total || 0,
    page: r?.page || 1,
    size: r?.size || params?.size || 10,
  };
}
export async function addCertificate(data: { domain: string; port?: number; alertDays?: number }) {
  return request('/api/v1/certificates', {
    method: 'POST',
    data: {
      domain: data.domain,
      port: data.port,
      alert_days: data.alertDays,
    },
  });
}
export async function checkCertificate(id: number) {
  return request(`/api/v1/certificates/${id}/check`, { method: 'POST' });
}
export async function checkAllCertificates() {
  return request(`/api/v1/certificates/check-all`, { method: 'POST' });
}
export async function deleteCertificate(id: number) {
  return request(`/api/v1/certificates/${id}`, { method: 'DELETE' });
}

export async function listOpsBackups() {
  const response = await request<{ backups?: any[] }>('/api/v1/ops/backups');
  return { backups: (response?.backups || []).map(normalizeOpsBackup) };
}
export async function createOpsBackup(data: any) {
  return request<void>('/api/v1/ops/backups', { method: 'POST', data });
}
export async function deleteOpsBackup(id: string) {
  return request<void>(`/api/v1/ops/backups/${encodeURIComponent(id)}`, { method: 'DELETE' });
}
export function getOpsBackupDownloadUrl(id: string) {
  return buildDownloadUrl(`/api/v1/ops/backups/${encodeURIComponent(id)}/download`);
}

export async function fetchOpsNotifications() {
  const response = await request<any>('/api/v1/ops/notifications');
  return normalizeOpsNotifications(response);
}
export async function saveOpsNotifications(data: {
  channels: OpsNotificationChannel[];
  rules: OpsNotificationRule[];
}) {
  return request<void>('/api/v1/ops/notifications', { method: 'PUT', data });
}

export async function fetchOpsAlerts() {
  const response = await request<{ alerts?: RawOpsAlert[] }>('/api/v1/ops/alerts');
  return { alerts: (response?.alerts || []).map(normalizeOpsAlert) };
}

export async function silenceOpsAlert(data: {
  matchers: Record<string, string>;
  duration: string;
  comment?: string;
}) {
  return request<void>('/api/v1/ops/alerts/silence', {
    method: 'POST',
    data: {
      Matchers: data.matchers,
      Duration: data.duration,
      Comment: data.comment || '',
      Creator: 'ui',
    },
  });
}

export async function listOpsNodes() {
  const response = await request<{ nodes?: RawOpsNode[] }>('/api/v1/ops/nodes');
  return { nodes: (response?.nodes || []).map(normalizeOpsNode) };
}

export async function drainOpsNode(id: string) {
  return request<void>(`/api/v1/ops/nodes/${encodeURIComponent(id)}/drain`, { method: 'POST' });
}

export async function undrainOpsNode(id: string) {
  return request<void>(`/api/v1/ops/nodes/${encodeURIComponent(id)}/undrain`, { method: 'POST' });
}

export async function restartOpsNode(id: string) {
  return request<void>(`/api/v1/ops/nodes/${encodeURIComponent(id)}/restart`, { method: 'POST' });
}
