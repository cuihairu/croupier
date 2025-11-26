import { request } from '@umijs/max';

export type AuditEvent = {
  time: string;
  kind: string;
  actor: string;
  target: string;
  meta: Record<string, string>;
  hash: string;
  prev: string;
};

export async function listAudit(params?: { game_id?: string; env?: string; actor?: string; kind?: string; kinds?: string; ip?: string; limit?: number; offset?: number; page?: number; size?: number; start?: string; end?: string }) {
  return request<{ events: AuditEvent[]; total?: number }>('/api/v1/audit', { params });
}