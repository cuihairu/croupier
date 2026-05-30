import { request } from '@umijs/max';
import { createEventSource } from '../core/http';

// Source: croupier/internal/api/message/dto.go MessageItem
// Backend builder: croupier/internal/logic/utils/message_helpers.go BuildMessageDTO
export type MessageItem = {
  id: string | number;
  to: string;
  type: string;
  title: string;
  content: string;
  data?: unknown;
  status: string;
  readAt?: string;
  createdAt: string;
  updatedAt: string;
};

// Source: croupier/internal/api/message/dto.go MessagesListResponse
export type MessagesListResponse = {
  items: MessageItem[];
  total: number;
  page: number;
  pageSize: number;
};

// Source: croupier/internal/api/message/dto.go MessagesUnreadCountResponse
export type MessagesUnreadCountResponse = {
  count: number;
};

// Source: croupier/internal/api/message/dto.go MessageSendRequest
export type MessageSendRequest = {
  to: string;
  type: string;
  title: string;
  content: string;
  data?: unknown;
};

const getToken = () => (typeof window !== 'undefined' ? localStorage.getItem('token') || '' : '');

export async function unreadCount(): Promise<MessagesUnreadCountResponse> {
  if (!getToken()) return { count: 0 };
  return request<MessagesUnreadCountResponse>('/api/v1/messages/unread-count');
}

export async function listMessages(params?: {
  status?: 'unread' | 'all';
  page?: number;
  pageSize?: number;
  type?: string;
}): Promise<MessagesListResponse> {
  const page = params?.page ?? 1;
  const pageSize = params?.pageSize ?? 10;
  if (!getToken()) {
    return { items: [], total: 0, page, pageSize };
  }
  return request<MessagesListResponse>('/api/v1/messages', {
    params: {
      status: params?.status,
      page,
      pageSize,
      type: params?.type,
    },
  });
}

export async function markMessagesRead(
  ids: Array<string | number>,
  options?: { broadcastIds?: Array<string | number> },
): Promise<void> {
  if (!ids || ids.length === 0) return;
  await Promise.all(
    ids.map((id) => request<MessageItem>(`/api/v1/messages/${id}/read`, { method: 'POST' })),
  );
  // Current backend contract has no dedicated broadcast batch-read endpoint.
  void options;
}

export async function sendMessage(body: MessageSendRequest): Promise<MessageItem> {
  return request<MessageItem>('/api/v1/messages', { method: 'POST', data: body });
}

export function openMessagesStream() {
  return createEventSource('/api/v1/messages/stream');
}
