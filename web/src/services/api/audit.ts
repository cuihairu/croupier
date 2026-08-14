import { request } from '@umijs/max';
import type { JSONValue } from '@/types/dashboard';

// Source: croupier/internal/api/audit/dto.go AuditItem
export type AuditItem = {
  id: string;
  action: string;
  userId: string;
  gameId?: string;
  env?: string;
  target?: string;
  result?: string;
  traceId?: string;
  metadata?: Record<string, JSONValue>;
  createdAt: string;
};

// Source: croupier/internal/api/audit/dto.go AuditListResponse
export type AuditListResponse = {
  items: AuditItem[];
  total: number;
  page: number;
  pageSize: number;
};

// View-model DTO used by current pages. Transport normalization must stay in services/api.
export type AuditEvent = {
  time: string;
  kind: string;
  actor: string;
  target: string;
  meta: Record<string, JSONValue>;
  hash: string;
  prev: string;
};

function normalizeAuditEvent(item: AuditItem): AuditEvent {
  const metadata = item?.metadata ?? {};
  return {
    time: item?.createdAt ?? '',
    kind: item?.action ?? '',
    actor: item?.userId ?? '',
    target: item?.target ?? '',
    meta: {
      ...metadata,
      traceId: (metadata.traceId as string) ?? item?.traceId,
      gameId: (metadata.gameId as string) ?? item?.gameId,
      env: (metadata.env as string) ?? item?.env,
      ip: metadata.ip as string,
      ua: (metadata.ua as string) ?? (metadata.userAgent as string),
      userAgent: (metadata.userAgent as string) ?? (metadata.ua as string),
    },
    hash: item?.id ?? '',
    prev: '',
  };
}

export async function listAudit(params?: {
  gameId?: string;
  env?: string;
  actor?: string;
  kind?: string;
  kinds?: string;
  ip?: string;
  limit?: number;
  offset?: number;
  page?: number;
  size?: number;
  pageSize?: number;
  start?: string;
  end?: string;
}) {
  const response = await request<AuditListResponse>('/api/v1/audit', {
    params: {
      actor: params?.actor,
      kind: params?.kind,
      kinds: params?.kinds,
      env: params?.env,
      ip: params?.ip,
      start: params?.start,
      end: params?.end,
      page: params?.page,
      pageSize: params?.pageSize ?? params?.size ?? params?.limit,
      gameId: params?.gameId,
    },
  });

  const items = Array.isArray(response?.items) ? response.items : [];
  return {
    events: items.map(normalizeAuditEvent),
    total: response?.total ?? items.length,
    page: response?.page ?? params?.page ?? 1,
    pageSize: response?.pageSize ?? params?.pageSize ?? params?.size ?? params?.limit ?? 20,
  };
}
