import { request } from '@umijs/max';

export async function fetchAssignments(params?: { game_id?: string; env?: string }) {
  return request<{ assignments: Record<string, string[]> }>('/api/v1/assignments', { params });
}

export async function setAssignments(params: { game_id: string; env?: string; functions: string[] }) {
  return request<{ ok: boolean; unknown?: string[] }>('/api/v1/assignments', { method: 'PUT', data: params });
}