import { request } from '@umijs/max';

// Source: internal/api/ops/dto.go (health/maintenance/mq/services)
export type HealthCheck = {
  id: string;
  name: string;
  enabled: boolean;
  type: string;
  kind?: string;
  target?: string;
  expect?: string;
  region?: string;
  interval?: number;
  intervalSec?: number;
  timeoutMs?: number;
};

export type HealthRunResult = {
  id: string;
  ok: boolean;
  latencyMs: number;
  checkedAt?: string;
  error?: string;
};

export async function getOpsHealth(): Promise<{ checks: HealthCheck[]; updatedAt?: string }> {
  const res = await request<{ checks?: HealthCheck[]; updatedAt?: string }>('/api/v1/ops/health');
  return { checks: res?.checks || [], updatedAt: res?.updatedAt };
}

export async function runOpsHealthCheck(id: string): Promise<HealthRunResult> {
  return request<HealthRunResult>('/api/v1/ops/health/run', {
    method: 'POST',
    data: { id },
  });
}

export async function updateOpsHealth(payload: {
  enabled: boolean;
  checks: HealthCheck[];
}): Promise<void> {
  await request('/api/v1/ops/health', { method: 'PUT', data: payload });
}

export async function getOpsMaintenance(): Promise<{
  enabled?: boolean;
  message?: string;
  allowAdmins?: boolean;
}> {
  return request('/api/v1/ops/maintenance');
}

export async function updateOpsMaintenance(payload: {
  enabled: boolean;
  message?: string;
  allowAdmins?: boolean;
}): Promise<void> {
  await request('/api/v1/ops/maintenance', { method: 'PUT', data: payload });
}

export type OpsServiceItem = {
  name: string;
  status: string;
  addr?: string;
  version?: string;
  count?: number;
};

export async function getOpsServices(): Promise<OpsServiceItem[]> {
  const res = await request<{ services?: OpsServiceItem[] }>('/api/v1/ops/services');
  return res?.services || [];
}

export type OpsMQInfo = {
  type?: string;
  updatedAt?: string;
  lengths?: Record<string, number>;
  groups?: Array<{
    stream?: string;
    name?: string;
    consumers?: number;
    pending?: number;
    lag?: number;
  }>;
};

export async function getOpsMQ(): Promise<OpsMQInfo> {
  const res = await request<Record<string, unknown>>('/api/v1/ops/mq');
  return {
    type: res?.type as string | undefined,
    updatedAt: res?.updatedAt as string | undefined,
    lengths: res?.lengths as Record<string, number> | undefined,
    groups: res?.groups as OpsMQInfo['groups'],
  };
}
