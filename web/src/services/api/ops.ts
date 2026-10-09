import { request } from '@umijs/max';
import { buildDownloadUrl } from '../core/http';
import type { JSONValue } from '@/types/dashboard';

// Source: croupier/internal/api/ops/dto.go OpsAgentInfo and legacy ops agent listings.
export type OpsAgent = {
  agentId: string;
  gameId: string;
  env: string;
  addr: string;
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
  // 集群模式下该 agent 连接的持有实例（连接不在处理请求的实例上时标注）
  ownerInstance?: string;
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
  addr?: string;
  qps: number;
  qps1m?: number;
};
type RawRateLimitRule = {
  scope: 'function' | 'service';
  key: string;
  limitQps: number;
  match?: Record<string, string>;
  percent?: number;
};
type RawRateLimitPreviewAgent = {
  agentId: string;
  gameId?: string;
  env?: string;
  region?: string;
  zone?: string;
  qps: number;
  qps1M?: number;
};
function normalizeRateLimitRule(raw: RawRateLimitRule): RateLimitRule {
  return {
    scope: raw.scope,
    key: raw.key,
    limitQps: raw.limitQps,
    match: raw.match,
    percent: raw.percent,
  };
}
function normalizeRateLimitPreviewAgent(raw: RawRateLimitPreviewAgent): RateLimitPreviewAgent {
  return {
    agentId: raw.agentId,
    gameId: raw.gameId,
    env: raw.env,
    region: raw.region,
    zone: raw.zone,
    qps: raw.qps,
    qps1m: raw.qps1M,
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
        limitQps: rule.limitQps,
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
      limitQps: params.limitQps,
      percent: params.percent,
      matchGameId: params.matchGameId,
      matchEnv: params.matchEnv,
      matchRegion: params.matchRegion,
      matchZone: params.matchZone,
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
  matchers?: JSONValue;
  startAt?: string;
  endAt?: string;
  createdBy?: string;
};

// Source: croupier/internal/api/ops/dto.go Node / OpsNodesResponse
export type OpsNode = {
  id: string;
  type?: string;
  hostname?: string;
  addr?: string;
  gameId?: string;
  env?: string;
  status?: string;
  labels?: Record<string, string>;
  lastSeen?: string;
  sdkLanguage?: string;
  sdkVersion?: string;
  sdkName?: string;
  version?: string;
  functions?: number;
  expiresInSec?: number;
  // System metrics
  cpu?: {
    usagePercent: number;
    cores: number;
    perCore?: number[];
    load1m: number;
    load5m: number;
    load15m: number;
  };
  memory?: {
    totalBytes: number;
    usedBytes: number;
    availableBytes: number;
    usagePercent: number;
    swapTotal: number;
    swapUsed: number;
  };
  disks?: Array<{
    mountPoint: string;
    device: string;
    fsType: string;
    totalBytes: number;
    usedBytes: number;
    availableBytes: number;
    usagePercent: number;
    inodeTotal?: number;
    inodeUsed?: number;
  }>;
  // Supervisor 监管聚合灯（数据源同 cpu/memory：metrics 上报捎带的托管进程快照）
  supervisor?: OpsSupervisorSummary;
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
  labels?: Record<string, JSONValue>;
  annotations?: Record<string, JSONValue>;
};
export type OpsTask = {
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
  addr?: string;
  traceId?: string;
};

// Source: /api/v1/ops/metrics timeseries payload.
export type OpsMetrics = {
  qps: [number, string][];
  errRate: [number, string][];
  p95Ms: [number, string][];
};

type RawOpsConfig = {
  alertmanagerUrl?: string;
  grafanaExploreUrl?: string;
  jaegerUrl?: string;
};

type RawOpsNotificationRule = {
  event: string;
  channels: string[];
  thresholdDays?: number;
};

type RawOpsSilence = {
  id: string;
  alertType?: string;
  matchers?: JSONValue;
  startAt?: string;
  endAt?: string;
  createdBy?: string;
};

type RawOpsNode = {
  id: string;
  hostname?: string;
  addr?: string;
  gameId?: string;
  env?: string;
  status?: string;
  labels?: Record<string, string>;
  lastSeen?: string;
  sdkLanguage?: string;
  sdkVersion?: string;
  sdkName?: string;
  version?: string;
  functions?: number;
  expiresInSec?: number;
  // System metrics
  cpu?: OpsNode['cpu'];
  memory?: OpsNode['memory'];
  disks?: OpsNode['disks'];
};
type RawOpsAlert = {
  severity?: string;
  instance?: string;
  service?: string;
  summary?: string;
  startsAt?: string;
  endsAt?: string;
  duration?: string;
  silenced?: boolean;
  labels?: Record<string, JSONValue>;
  annotations?: Record<string, JSONValue>;
};
type RawOpsTask = {
  id: string;
  functionId?: string;
  actor?: string;
  gameId?: string;
  env?: string;
  // The canonical task endpoint exposes `status` (see internal/api/task/dto.go
  // Item). Legacy/ops payloads used `state`; accept both.
  status?: string;
  state?: 'running' | 'succeeded' | 'failed' | 'canceled' | string;
  startedAt?: string;
  finishedAt?: string;
  endedAt?: string;
  durationMs?: number;
  error?: string;
  addr?: string;
  traceId?: string;
};

function normalizeOpsConfig(raw?: RawOpsConfig): OpsConfig {
  return {
    alertmanagerUrl: raw?.alertmanagerUrl,
    grafanaExploreUrl: raw?.grafanaExploreUrl,
    jaegerUrl: raw?.jaegerUrl,
  };
}

function normalizeOpsBackup(raw: Record<string, JSONValue>): OpsBackup {
  return {
    id: String(raw.id ?? ''),
    name: raw.name ? String(raw.name) : undefined,
    type: raw.type ? String(raw.type) : undefined,
    status: raw.status ? String(raw.status) : undefined,
    size: typeof raw.size === 'number' ? raw.size : undefined,
    createdAt: String(raw.createdAt ?? raw.createdAt ?? ''),
  };
}

function normalizeOpsNotificationRule(raw: RawOpsNotificationRule): OpsNotificationRule {
  return {
    event: raw?.event ?? '',
    channels: Array.isArray(raw?.channels) ? raw.channels : [],
    thresholdDays: raw?.thresholdDays ?? raw?.thresholdDays,
  };
}

function normalizeOpsNotifications(raw: Record<string, JSONValue>): OpsNotifications {
  return {
    enabled: !!raw?.enabled,
    channels: Array.isArray(raw?.channels) ? (raw.channels as OpsNotificationChannel[]) : [],
    rules: Array.isArray(raw?.rules)
      ? (raw.rules as RawOpsNotificationRule[]).map(normalizeOpsNotificationRule)
      : [],
  };
}

function normalizeOpsSilence(raw: RawOpsSilence): OpsSilence {
  return {
    id: raw?.id ?? '',
    alertType: raw?.alertType,
    matchers: raw?.matchers,
    startAt: raw?.startAt,
    endAt: raw?.endAt,
    createdBy: raw?.createdBy,
  };
}

function normalizeOpsNode(raw: RawOpsNode): OpsNode {
  return {
    id: raw?.id ?? '',
    hostname: raw?.hostname,
    addr: raw?.addr,
    gameId: raw?.gameId,
    env: raw?.env,
    status: raw?.status,
    labels: raw?.labels,
    lastSeen: raw?.lastSeen,
    sdkLanguage: raw?.sdkLanguage,
    sdkVersion: raw?.sdkVersion,
    sdkName: raw?.sdkName,
    version: raw?.version,
    functions: raw?.functions ?? 0,
    expiresInSec: raw?.expiresInSec ?? 0,
    // System metrics
    cpu: raw?.cpu,
    memory: raw?.memory,
    disks: raw?.disks,
  };
}
function normalizeOpsAlert(raw: RawOpsAlert): OpsAlert {
  return {
    severity: raw.severity,
    instance: raw.instance,
    service: raw.service,
    summary: raw.summary,
    startsAt: raw.startsAt,
    endsAt: raw.endsAt,
    duration: raw.duration,
    silenced: raw.silenced,
    labels: raw.labels,
    annotations: raw.annotations,
  };
}
function normalizeOpsTask(raw: RawOpsTask): OpsTask {
  return {
    id: raw.id,
    functionId: raw.functionId || '',
    actor: raw.actor,
    gameId: raw.gameId,
    env: raw.env,
    state: raw.state ?? raw.status ?? '',
    startedAt: raw.startedAt,
    endedAt: raw.endedAt || raw.finishedAt,
    durationMs: raw?.durationMs,
    error: raw.error,
    addr: raw.addr,
    traceId: raw.traceId,
  };
}
export async function listOpsTasks(params?: {
  status?: string;
  functionId?: string;
  actor?: string;
  gameId?: string;
  env?: string;
  page?: number;
  size?: number;
}) {
  // Backed by the canonical task list route GET /api/v1/tasks. The server
  // returns {items, total} (internal/api/task/dto.go ListResponse); accept the
  // legacy {jobs, total} shape as well for safety.
  const response = await request<{ items?: RawOpsTask[]; jobs?: RawOpsTask[]; total?: number }>(
    '/api/v1/tasks',
    {
      params: {
        status: params?.status,
        functionId: params?.functionId,
        actor: params?.actor,
        gameId: params?.gameId,
        env: params?.env,
        page: params?.page,
        size: params?.size,
      },
    },
  );
  const rows = response.items || response.jobs || [];
  return { tasks: rows.map(normalizeOpsTask), total: response.total || 0 };
}

export async function fetchOpsMetrics(params: { instance: string; range?: string; step?: string }) {
  const response = await request<{
    qps: [number, string][];
    errRate: [number, string][];
    p95Ms: [number, string][];
  }>('/api/v1/ops/metrics', { params });
  return {
    qps: response.qps || [],
    errRate: response.errRate || [],
    p95Ms: response.p95Ms || [],
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
  keyUsage?: string;
  KeyUsage?: string;
  validFrom?: string;
  ValidFrom?: string;
  validTo?: string;
  ValidTo?: string;
  daysLeft?: number;
  DaysLeft?: number;
  status?: Certificate['status'];
  Status?: Certificate['status'];
  lastChecked?: string;
  lastCheckedAt?: string;
  LastChecked?: string;
  errorMsg?: string;
  ErrorMsg?: string;
  alertDays?: number;
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
    keyUsage: raw.keyUsage ?? raw.KeyUsage,
    validFrom: raw.validFrom ?? raw.ValidFrom,
    validTo: raw.validTo ?? raw.ValidTo,
    daysLeft: raw.daysLeft ?? raw.DaysLeft,
    status: raw.status ?? raw.Status,
    lastChecked: raw.lastChecked ?? raw.lastCheckedAt ?? raw.LastChecked,
    errorMessage: raw.errorMsg ?? raw.ErrorMsg,
    alertDays: raw.alertDays ?? raw.AlertDays,
  };
}

export async function listCertificates(params?: { page?: number; size?: number; status?: string }) {
  // 契约：GET /api/v1/certificates -> { items, total, page, size }
  const r = await request<{
    items?: RawCertificate[];
    total?: number;
    page?: number;
    size?: number;
  }>('/api/v1/certificates', { params });
  const raw = (r?.items || []) as RawCertificate[];
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
      alertDays: data.alertDays,
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
  const response = await request<{ backups?: Record<string, JSONValue>[] }>('/api/v1/ops/backups');
  return { backups: (response?.backups || []).map(normalizeOpsBackup) };
}
export async function createOpsBackup(data: Record<string, JSONValue>) {
  return request<void>('/api/v1/ops/backups', { method: 'POST', data });
}
export async function deleteOpsBackup(id: string) {
  return request<void>(`/api/v1/ops/backups/${encodeURIComponent(id)}`, { method: 'DELETE' });
}
export function getOpsBackupDownloadUrl(id: string) {
  return buildDownloadUrl(`/api/v1/ops/backups/${encodeURIComponent(id)}/download`);
}

export async function fetchOpsNotifications() {
  const response = await request<Record<string, JSONValue>>('/api/v1/ops/notifications');
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
  return request<void>(`/api/v1/ops/nodes/${encodeURIComponent(id)}/drain`, {
    method: 'POST',
    data: { nodeId: id },
  });
}

export async function undrainOpsNode(id: string) {
  return request<void>(`/api/v1/ops/nodes/${encodeURIComponent(id)}/undrain`, {
    method: 'POST',
    data: { nodeId: id },
  });
}

export async function restartOpsNode(id: string) {
  return request<void>(`/api/v1/ops/nodes/${encodeURIComponent(id)}/restart`, {
    method: 'POST',
    data: { nodeId: id },
  });
}

// Historical metrics
export type MetricsHistoryEntry = {
  timestamp: string;
  cpu?: {
    usagePercent: number;
    cores: number;
    load1m: number;
    load5m: number;
    load15m: number;
  };
  memory?: {
    totalBytes: number;
    usedBytes: number;
    availableBytes: number;
    usagePercent: number;
    swapTotal: number;
    swapUsed: number;
  };
  disks?: Array<{
    mountPoint: string;
    device: string;
    fsType: string;
    totalBytes: number;
    usedBytes: number;
    availableBytes: number;
    usagePercent: number;
  }>;
};

export async function getAgentMetricsHistory(
  agentId: string,
  options?: { since?: string; limit?: number },
): Promise<MetricsHistoryEntry[]> {
  const params = new URLSearchParams();
  if (agentId) params.set('agentId', agentId);
  if (options?.since) params.set('since', options.since);
  if (options?.limit) params.set('limit', String(options.limit));

  const response = await request<{ entries?: MetricsHistoryEntry[] }>(
    `/api/v1/ops/agent/metrics/history?${params.toString()}`,
  );
  return response?.entries || [];
}

// ---- 告警规则（阈值评估，/alerts/rules） ----

export type AlertRuleItem = {
  id: number;
  name: string;
  description?: string;
  metric: string;
  operator: 'gt' | 'gte' | 'lt' | 'lte';
  threshold: number;
  forCount: number;
  cooldownSeconds: number;
  level: 'info' | 'warning' | 'critical';
  enabled: boolean;
  agentFilter?: string;
  hitCount: number;
  lastFiredAt?: string;
  createdBy?: string;
};

export type AlertRuleCreatePayload = {
  name: string;
  metric: string;
  operator: string;
  threshold: number;
  description?: string;
  forCount?: number;
  cooldownSeconds?: number;
  level?: string;
  agentFilter?: string;
  enabled?: boolean;
};

export async function listAlertRules(params?: {
  metric?: string;
  enabled?: string;
}): Promise<{ items: AlertRuleItem[] }> {
  return request('/api/v1/alerts/rules', { params });
}

export async function createAlertRule(
  data: AlertRuleCreatePayload,
): Promise<{ item: AlertRuleItem }> {
  return request('/api/v1/alerts/rules', { method: 'POST', data });
}

export async function updateAlertRule(
  id: number,
  data: Partial<AlertRuleCreatePayload>,
): Promise<{ item: AlertRuleItem }> {
  return request(`/api/v1/alerts/rules/${id}`, { method: 'PUT', data });
}

export async function deleteAlertRule(id: number): Promise<{ ok: boolean }> {
  return request(`/api/v1/alerts/rules/${id}`, { method: 'DELETE' });
}

// ---- 集群拓扑（/ops/cluster） ----

export type ClusterInstanceItem = {
  instanceId: string;
  advertiseAddr: string;
  epoch: number;
  startedAt?: string;
  self: boolean;
  alive: boolean;
  agentCount: number;
};

export type ClusterLbStatsInfo = {
  enabled: boolean;
  queryUrl: string;
};

export type ClusterInfo = {
  enabled: boolean;
  self?: string;
  items: ClusterInstanceItem[];
  total: number;
  aliveCount: number;
  lbStats?: ClusterLbStatsInfo;
};

export async function fetchClusterInfo(): Promise<ClusterInfo> {
  return request('/api/v1/ops/cluster');
}

// 节点主机定时任务（agent 读取 crontab + /etc/cron.d）。
export interface NodeCronJob {
  schedule: string;
  command: string;
  user: string;
  sourceFile: string;
  enabled: boolean;
}

export async function fetchNodeCronJobs(nodeId: string): Promise<NodeCronJob[]> {
  const res = await request<{ items: NodeCronJob[]; total: number }>(
    `/api/v1/nodes/${encodeURIComponent(nodeId)}/cron-jobs`,
    { method: 'GET' },
  );
  return res.items ?? [];
}

// Supervisor 监管视图 + 事件流 + 手工动作（Source: croupier/internal/api/ops/dto.go
// OpsSupervisorSummary / OpsSupervisedProcess / OpsAgentSupervisorResponse /
// supervisor events；事件为 server 端 ring buffer 快照，非流式）

/** 聚合状态灯：ok=全部 RUNNING 且无超限标记；warn=有非 RUNNING 或超限；error=有 FAILED/BROKEN */
export type OpsSupervisorSummary = {
  status: 'ok' | 'warn' | 'error';
  total: number;
  running: number;
};

/** 被监管进程快照（agent 每 metrics 周期采样上报） */
export type SupervisedProcess = {
  name: string;
  pid: number;
  state: string;
  uptimeSeconds: number;
  restartCount: number;
  rssBytes: number;
  cpuPercent: number;
  flags: string[];
};

type RawOpsAgentSupervisor = {
  agentId?: string;
  timestamp?: string;
  processes?: Array<{
    name?: string;
    pid?: number;
    state?: string;
    uptimeSeconds?: number;
    restartCount?: number;
    rssBytes?: number;
    cpuPercent?: number;
    flags?: string[];
  }>;
  summary?: { status?: string; total?: number; running?: number };
};

function normalizeSupervisedProcess(
  raw: NonNullable<RawOpsAgentSupervisor['processes']>[number],
): SupervisedProcess {
  return {
    name: raw.name || '',
    pid: raw.pid || 0,
    state: raw.state || 'unknown',
    uptimeSeconds: raw.uptimeSeconds || 0,
    restartCount: raw.restartCount || 0,
    rssBytes: raw.rssBytes || 0,
    cpuPercent: raw.cpuPercent || 0,
    flags: Array.isArray(raw.flags) ? raw.flags : [],
  };
}

function normalizeSupervisorSummary(raw: RawOpsAgentSupervisor['summary']): OpsSupervisorSummary {
  const status = raw?.status;
  return {
    status: status === 'error' || status === 'warn' ? status : 'ok',
    total: raw?.total || 0,
    running: raw?.running || 0,
  };
}

/** 读取 agent 的 supervisor 快照（server 端 MetricsStore 最新一报，零隧道请求）。 */
export async function fetchAgentSupervisor(agentId: string): Promise<OpsAgentSupervisorResponse> {
  const raw = await request<RawOpsAgentSupervisor>(
    `/api/v1/ops/agents/${encodeURIComponent(agentId)}/supervisor`,
    { method: 'GET' },
  );
  return {
    agentId: raw.agentId || agentId,
    timestamp: raw.timestamp || '',
    processes: (raw.processes || []).map(normalizeSupervisedProcess),
    summary: normalizeSupervisorSummary(raw.summary),
  };
}

export type OpsAgentSupervisorResponse = {
  agentId: string;
  /** server 收到最新 metrics 上报的时间；面板据此判断数据新鲜度 */
  timestamp: string;
  processes: SupervisedProcess[];
  summary: OpsSupervisorSummary;
};

/** supervisor 事件闭集：detect_down | auto_restart | restart_failed |
 *  breaker_tripped | resource_over_limit | manual_start | manual_stop */
export const SUPERVISOR_EVENT_TYPES = [
  'detect_down',
  'auto_restart',
  'restart_failed',
  'breaker_tripped',
  'resource_over_limit',
  'manual_start',
  'manual_stop',
] as const;

export type SupervisorEventType = (typeof SUPERVISOR_EVENT_TYPES)[number];

/** supervisor 事件（server 端 ring buffer；零值字段可能被 omitempty 省略） */
export type SupervisorEvent = {
  seq: number;
  tsUnix: number;
  process: string;
  event: string;
  oldPid: number;
  newPid: number;
  exitCode: number;
  signal: string;
  restartCount: number;
  message: string;
  lastHeartbeatUnix: number;
  lastError: string;
  oomSuspect: boolean;
  lastRssBytes: number;
};

export type OpsAgentSupervisorEventsResponse = {
  agentId: string;
  events: SupervisorEvent[];
};

type RawSupervisorEvent = Record<string, JSONValue>;

function asNumber(v: JSONValue | undefined): number {
  return typeof v === 'number' && Number.isFinite(v) ? v : 0;
}

function asString(v: JSONValue | undefined): string {
  return typeof v === 'string' ? v : '';
}

/** 事件原始形态 → 显式零值（server omitempty 缺字段时兜底 0/''/false） */
export function normalizeSupervisorEvent(raw: RawSupervisorEvent): SupervisorEvent {
  return {
    seq: asNumber(raw.seq),
    tsUnix: asNumber(raw.tsUnix),
    process: asString(raw.process),
    event: asString(raw.event),
    oldPid: asNumber(raw.oldPid),
    newPid: asNumber(raw.newPid),
    exitCode: asNumber(raw.exitCode),
    signal: asString(raw.signal),
    restartCount: asNumber(raw.restartCount),
    message: asString(raw.message),
    lastHeartbeatUnix: asNumber(raw.lastHeartbeatUnix),
    lastError: asString(raw.lastError),
    oomSuspect: raw.oomSuspect === true,
    lastRssBytes: asNumber(raw.lastRssBytes),
  };
}

/** 读取 agent 的 supervisor 事件 ring（seq 降序返回，limit 供服务端截断）。 */
export async function fetchAgentSupervisorEvents(
  agentId: string,
  sinceSeq = 0,
  limit = 200,
): Promise<OpsAgentSupervisorEventsResponse> {
  const raw = await request<{ agentId?: string; events?: RawSupervisorEvent[] }>(
    `/api/v1/ops/agents/${encodeURIComponent(agentId)}/supervisor/events?sinceSeq=${sinceSeq}&limit=${limit}`,
    { method: 'GET' },
  );
  return {
    agentId: raw.agentId || agentId,
    events: (raw.events || []).map(normalizeSupervisorEvent),
  };
}

/** 手工拉起进程（server → agent 隧道下发 start），返回新 pid（失败/未知为 0）。 */
export async function startAgentProcess(agentId: string, name: string): Promise<number> {
  const raw = await request<{ pid?: number }>(
    `/api/v1/ops/agents/${encodeURIComponent(agentId)}/processes/${encodeURIComponent(name)}/start`,
    { method: 'POST' },
  );
  return typeof raw?.pid === 'number' && Number.isFinite(raw.pid) ? raw.pid : 0;
}

/** 手工停止进程（server → agent 隧道下发 stop）。 */
export async function stopAgentProcess(agentId: string, name: string): Promise<void> {
  await request<void>(
    `/api/v1/ops/agents/${encodeURIComponent(agentId)}/processes/${encodeURIComponent(name)}/stop`,
    { method: 'POST' },
  );
}

/** 手工重启进程（server → agent 隧道下发 restart）。 */
export async function restartAgentProcess(agentId: string, name: string): Promise<void> {
  await request<void>(
    `/api/v1/ops/agents/${encodeURIComponent(agentId)}/processes/${encodeURIComponent(name)}/restart`,
    { method: 'POST' },
  );
}

/** supervisor ndjson 日志下载直链（浏览器直接 attachment 下载，不走 blob）。 */
export function getAgentSupervisorLogUrl(agentId: string) {
  return buildDownloadUrl(`/api/v1/ops/agents/${encodeURIComponent(agentId)}/supervisor/logs`);
}

/** 服务端聚合选项行（#23/#33/#34 族：操作者/函数/Agent 过滤下拉） */
export type ServerSelectOptionRow = {
  value: string;
  label?: string;
  count?: number;
};

/** 任务操作者聚合选项（#23：GET /api/v1/tasks/operator-options）。
 *  供 /ops/jobs 操作者下拉消费，选项为当前 scope 下 distinct actor 全集，
 *  不随列表过滤塌缩。 */
export async function listOpsTaskOperatorOptions(): Promise<ServerSelectOptionRow[]> {
  const res = await request<{ items?: ServerSelectOptionRow[] }>('/api/v1/tasks/operator-options');
  return res.items ?? [];
}
