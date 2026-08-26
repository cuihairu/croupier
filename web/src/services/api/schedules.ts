import { request } from '@umijs/max';

// Source: internal/api/schedule/service.go ScheduleItem / RunLogItem
export type ScheduleItem = {
  id: number;
  name: string;
  cronExpr: string;
  gameId: string;
  env: string;
  functionId: string;
  status: 'active' | 'paused' | 'dead_letter';
  maxFailedRuns: number;
  consecutiveFailures: number;
  lastRunId?: string;
  actor?: string;
  nextTriggerAt?: string;
  lastTriggerAt?: string;
  createdAt: string;
  updatedAt: string;
};

export type RunLogItem = {
  id: number;
  taskRunId?: string;
  status: 'dispatched' | 'skipped' | 'failed';
  message?: string;
  slot: string;
  createdAt: string;
};

export async function listSchedules(params?: {
  page?: number;
  pageSize?: number;
  gameId?: string;
  env?: string;
  status?: string;
}): Promise<{ items: ScheduleItem[]; total: number }> {
  return request('/api/v1/schedules', { params });
}

export async function createSchedule(payload: {
  name: string;
  cronExpr: string;
  functionId: string;
  gameId?: string;
  env?: string;
  payload?: unknown;
  maxFailedRuns?: number;
}): Promise<{ item: ScheduleItem }> {
  return request('/api/v1/schedules', { method: 'POST', data: payload });
}

export async function setScheduleStatus(
  id: number,
  status: 'active' | 'paused',
): Promise<{ item: ScheduleItem }> {
  return request(`/api/v1/schedules/${id}/status`, { method: 'PUT', data: { status } });
}

export async function triggerScheduleNow(id: number): Promise<{ taskRunId: string }> {
  return request(`/api/v1/schedules/${id}/trigger`, { method: 'POST' });
}

export async function deleteSchedule(id: number): Promise<{ ok: boolean }> {
  return request(`/api/v1/schedules/${id}`, { method: 'DELETE' });
}

export async function listScheduleRuns(
  id: number,
  params?: { page?: number; pageSize?: number },
): Promise<{ items: RunLogItem[]; total: number }> {
  return request(`/api/v1/schedules/${id}/runs`, { params });
}
