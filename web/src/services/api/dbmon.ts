import { request } from '@umijs/max';

// Source: internal/api/dbmon/dto.go
export type DBSource = {
  id: number;
  name: string;
  driver: string; // mysql | postgres
  kind: string; // self | aliyun | huawei
  dsnMask?: string;
  gameId?: string;
  env?: string;
  enabled: boolean;
  sort: number;
  lockWaitWarn?: number;
  connWarnRatio?: number;
  createdBy?: string;
  createdAt: string;
  updatedAt: string;
};

export async function listDBSources(): Promise<{ items: DBSource[] }> {
  return request('/api/v1/dbmon/sources');
}

export async function createDBSource(payload: {
  name: string;
  driver: string;
  kind?: string;
  dsn: string;
  gameId?: string;
  env?: string;
  lockWaitWarn?: number;
  connWarnRatio?: number;
}): Promise<DBSource> {
  return request('/api/v1/dbmon/sources', { method: 'POST', data: payload });
}

export async function updateDBSource(
  id: number,
  payload: {
    name?: string;
    driver?: string;
    kind?: string;
    dsn?: string;
    gameId?: string;
    env?: string;
    enabled?: boolean;
    lockWaitWarn?: number;
    connWarnRatio?: number;
  },
): Promise<DBSource> {
  return request(`/api/v1/dbmon/sources/${id}`, { method: 'PUT', data: payload });
}

export async function deleteDBSource(id: number): Promise<void> {
  return request(`/api/v1/dbmon/sources/${id}`, { method: 'DELETE' });
}

// Source: internal/api/dbmon/probe.go ProbeResult
export type LockWait = {
  waitId: string;
  blockedBy: string;
  table?: string;
  waitSecs: number;
  query?: string;
};

export type ProbeResult = {
  sourceId: number;
  name: string;
  driver: string;
  kind: string;
  ok: boolean;
  error?: string;
  latencyMs?: number;
  connections?: { current: number; max: number; active: number };
  lockWaits?: LockWait[];
  deadlockCount?: number;
  deadlockNote?: string;
  queryCount?: number;
  txnCount?: number;
  probedAt: string;
};

export function probeAll(): Promise<{ results: ProbeResult[] }> {
  return request('/api/v1/dbmon/probe', { method: 'POST' });
}

export const dbKindLabels: Record<string, string> = {
  self: '自建',
  aliyun: '阿里云',
  huawei: '华为云',
};
