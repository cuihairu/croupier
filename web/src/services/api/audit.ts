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
  hash?: string;
  prevHash?: string;
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
  /**
   * 服务端分配的唯一事件 ID（`audit_<ts>_<rand>`）。
   *
   * 列表 rowKey 的首选：审计链上的 `hash` 在部分部署里并不回填（实测
   * `/api/v1/audit` 返回的 19 条记录 `hash` 全为空），若拿它当 rowKey，
   * 整页所有行 key 相同，React 会报 "two children with the same key"，
   * 且可能导致行被重复/漏渲染（docs/BUGS.md BUG-011）。
   */
  id: string;
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
    id: item?.id ?? '',
    time: item?.createdAt ?? '',
    kind: item?.action ?? '',
    actor: item?.userId ?? '',
    target: item?.target ?? '',
    hash: item?.hash ?? '',
    prev: item?.prevHash ?? '',
    meta: {
      ...metadata,
      traceId: (metadata.traceId as string) ?? item?.traceId,
      gameId: (metadata.gameId as string) ?? item?.gameId,
      env: (metadata.env as string) ?? item?.env,
      ip: metadata.ip as string,
      ua: (metadata.userAgent as string) ?? (metadata.ua as string),
      userAgent: (metadata.userAgent as string) ?? (metadata.ua as string),
      ipRegion: (metadata.ipRegion as string) || '',
    },
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

/**
 * 审计列表的 antd `rowKey`。
 *
 * 优先用服务端唯一 `id`；缺失时退回审计链 `hash`。两者都不回填的部署里，
 * 再用 `time+actor+kind+target` 组合，最后退到行序号，保证同一页内 key 唯一
 * ——重复 key 会让 React 报 "two children with the same key"，并可能重复/漏渲染行
 * （docs/BUGS.md BUG-011）。
 */
export function auditRowKey(event: AuditEvent, index?: number): string {
  if (event.id) return event.id;
  if (event.hash) return event.hash;
  const parts = [event.time, event.actor, event.kind, event.target].filter(Boolean);
  if (parts.length > 0) return parts.join('|');
  return `audit-row-${index ?? 0}`;
}
