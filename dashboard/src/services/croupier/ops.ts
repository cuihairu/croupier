import { request } from '@umijs/max';

export type OpsAgent = {
  agent_id: string;
  game_id: string;
  env: string;
  rpc_addr: string;
  ip?: string;
  type?: string;
  version?: string;
  functions: number;
  healthy: boolean;
  expires_in_sec: number;
  active_conns?: number;
  total_requests?: number;
  failed_requests?: number;
  error_rate?: number;
  avg_latency_ms?: number;
  last_seen?: string;
  qps_limit?: number;
  qps_1m?: number;
};

export async function fetchOpsServices() {
  return request<{ agents: OpsAgent[] }>("/api/ops/services");
}

export type RateLimitRule = { scope: 'function'|'service'; key: string; limit_qps: number; match?: Record<string,string>; percent?: number };
export async function listRateLimits() {
  return request<{ rules: RateLimitRule[] }>("/api/ops/rate-limits");
}
export async function putRateLimits(rules: RateLimitRule[]) {
  return request<void>("/api/ops/rate-limits", { method: 'PUT', data: { rules } });
}
export async function deleteRateLimit(scope: string, key: string) {
  return request<void>(`/api/ops/rate-limits?scope=${encodeURIComponent(scope)}&key=${encodeURIComponent(key)}`, { method: 'DELETE' });
}
export async function previewRateLimit(params: { scope: 'service'; key?: string; limit_qps: number; percent?: number; match_game_id?: string; match_env?: string; match_region?: string; match_zone?: string }) {
  return request<{ matched: number; agents: { agent_id: string; game_id?: string; env?: string; region?: string; zone?: string; rpc_addr?: string; qps: number }[] }>(
    "/api/ops/rate-limits/preview",
    { params },
  );
}

export async function listOpsFunctions() {
  return request<{ functions: { id: string; category?: string }[] }>("/api/ops/functions");
}

export type OpsJob = {
  id: string;
  function_id: string;
  actor?: string;
  game_id?: string;
  env?: string;
  state: 'running'|'succeeded'|'failed'|'canceled'|string;
  started_at?: string;
  ended_at?: string;
  duration_ms?: number;
  error?: string;
  rpc_addr?: string;
  trace_id?: string;
};
export async function listOpsJobs(params?: { status?: string; function_id?: string; actor?: string; game_id?: string; env?: string; page?: number; size?: number }) {
  return request<{ jobs: OpsJob[]; total: number }>("/api/ops/jobs", { params });
}

export async function fetchOpsMetrics(params: { instance: string; range?: string; step?: string }) {
  return request<{ qps: [number, string][]; err_rate: [number, string][]; p95_ms: [number, string][] }>("/api/ops/metrics", { params });
}

export async function listSilences() {
  return request<{ silences: any[] }>("/api/ops/alerts/silences");
}
export async function deleteSilence(id: string) {
  return request<void>(`/api/ops/alerts/silences/${encodeURIComponent(id)}`, { method: 'DELETE' });
}
export async function fetchOpsConfig() {
  return request<{ alertmanager_url?: string; grafana_explore_url?: string }>("/api/ops/config");
}

export async function updateAgentMeta(agent_id: string, data: { region?: string; zone?: string }) {
  return request<void>(`/api/ops/agents/${encodeURIComponent(agent_id)}/meta`, { method: 'PUT', data });
}

// Registry API 已在 services/croupier/registry.ts 提供，避免重复导出导致冲突

// --- Certificates (HTTPS) ---
export type Certificate = {
  id: number; domain: string; port: number; issuer?: string; subject?: string; algorithm?: string; key_usage?: string;
  valid_from?: string; valid_to?: string; days_left?: number; status?: 'valid'|'expiring'|'expired'|'error'|'pending'; last_checked?: string; error_msg?: string; alert_days?: number;
};
export async function listCertificates(params?: { page?: number; size?: number; status?: string }) {
  const r = await request<any>("/api/certificates", { params });
  const raw = (r?.certificates || []) as any[];
  const norm = raw.map((c: any) => ({
    id: c.id ?? c.ID,
    domain: c.domain ?? c.Domain,
    port: c.port ?? c.Port,
    issuer: c.issuer ?? c.Issuer,
    subject: c.subject ?? c.Subject,
    algorithm: c.algorithm ?? c.Algorithm,
    key_usage: c.key_usage ?? c.KeyUsage,
    valid_from: c.valid_from ?? c.ValidFrom,
    valid_to: c.valid_to ?? c.ValidTo,
    days_left: c.days_left ?? c.DaysLeft,
    status: c.status ?? c.Status,
    last_checked: c.last_checked ?? c.LastChecked,
    error_msg: c.error_msg ?? c.ErrorMsg,
    alert_days: c.alert_days ?? c.AlertDays,
  })) as Certificate[];
  return { certificates: norm, total: r?.total || 0, page: r?.page || 1, size: r?.size || (params?.size || 10) };
}
export async function addCertificate(data: { domain: string; port?: number; alert_days?: number }) {
  return request("/api/certificates", { method: 'POST', data });
}
export async function checkCertificate(id: number) {
  return request(`/api/certificates/${id}/check`, { method: 'POST' });
}
export async function checkAllCertificates() {
  return request(`/api/certificates/check-all`, { method: 'POST' });
}
export async function deleteCertificate(id: number) {
  return request(`/api/certificates/${id}`, { method: 'DELETE' });
}
