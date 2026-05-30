import { request } from '@umijs/max';

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
  metadata?: Record<string, unknown>;
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
  meta: Record<string, unknown>;
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
      trace_id: (metadata as any)?.trace_id ?? item?.traceId,
      game_id: (metadata as any)?.game_id ?? item?.gameId,
      env: (metadata as any)?.env ?? item?.env,
      ip: (metadata as any)?.ip,
      ua: (metadata as any)?.ua ?? (metadata as any)?.user_agent,
      user_agent: (metadata as any)?.user_agent ?? (metadata as any)?.ua,
    },
    hash: item?.id ?? '',
    prev: '',
  };
}

export async function listAudit(params?: {
  gameId?: string;
  game_id?: string;
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
      gameId: params?.gameId ?? params?.game_id,
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
