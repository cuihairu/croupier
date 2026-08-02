import { request } from '@umijs/max';
import type { JSONValue } from '@/types/dashboard';

const TICKETS_BASE = '/api/v1/tickets';
const FAQ_BASE = '/api/v1/faqs';
const FEEDBACK_BASE = '/api/v1/feedback';

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

export interface Ticket {
  id: number;
  player_id: string;
  playerId: string;
  game_id: string;
  gameId: string;
  title: string;
  content?: string;
  status: string;
  category?: string;
  priority?: string;
  assignee?: string;
  tags: string[];
  created_at: string;
  createdAt: string;
  updated_at: string;
  updatedAt: string;
  comments?: TicketComment[];
}

export interface TicketComment {
  id: number;
  content: string;
  author?: string;
  created_at: string;
  createdAt: string;
}

export interface FAQ {
  id: number;
  title: string;
  content: string;
  category?: string;
  tags: string[];
  visible?: boolean;
  sort?: number;
  created_at: string;
  createdAt: string;
  updated_at: string;
  updatedAt: string;
}

export interface Feedback {
  id: number;
  player_id: string;
  playerId: string;
  game_id: string;
  gameId: string;
  title: string;
  content?: string;
  status: string;
  category?: string;
  created_at: string;
  createdAt: string;
  updated_at: string;
  updatedAt: string;
}

export interface TicketListParams {
  page?: number;
  pageSize?: number;
  size?: number;
  status?: string;
  category?: string;
  priority?: string;
  assignee?: string;
  q?: string;
  gameId?: string;
  game_id?: string;
  env?: string;
}

export interface FAQListParams {
  page?: number;
  pageSize?: number;
  size?: number;
  category?: string;
  keyword?: string;
  q?: string;
  visible?: string | boolean;
}

export interface FeedbackListParams {
  page?: number;
  pageSize?: number;
  size?: number;
  status?: string;
  category?: string;
  gameId?: string;
  game_id?: string;
  q?: string;
  env?: string;
}

export interface TicketPayload {
  playerId?: string;
  player_id?: string;
  gameId?: string;
  game_id?: string;
  title?: string;
  content?: string;
  status?: string;
  category?: string;
  priority?: string;
  assignee?: string;
  tags?: string | string[];
  [key: string]: JSONValue | undefined;
}

export interface FAQPayload {
  title?: string;
  content?: string;
  category?: string;
  tags?: string | string[];
  visible?: boolean | string;
  sort?: number | string;
  [key: string]: JSONValue | undefined;
}

export interface FeedbackPayload {
  playerId?: string;
  player_id?: string;
  gameId?: string;
  game_id?: string;
  title?: string;
  content?: string;
  status?: string;
  category?: string;
  [key: string]: JSONValue | undefined;
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function toArray<T>(value: T[] | undefined | null): T[] {
  return Array.isArray(value) ? value : [];
}

function splitTags(input: unknown): string[] {
  if (Array.isArray(input)) {
    return input.map((item) => String(item).trim()).filter(Boolean);
  }
  if (typeof input === 'string') {
    return input
      .split(',')
      .map((item) => item.trim())
      .filter(Boolean);
  }
  return [];
}

function parseVisible(value: unknown): boolean | undefined {
  if (typeof value === 'boolean') {
    return value;
  }
  if (typeof value === 'string') {
    const normalized = value.trim().toLowerCase();
    if (normalized === 'true') return true;
    if (normalized === 'false') return false;
  }
  return undefined;
}

function normalizeTicket(item: Record<string, JSONValue>): Ticket {
  return {
    id: Number(item.id ?? 0),
    player_id: String(item.player_id ?? item.playerId ?? ''),
    playerId: String(item.playerId ?? item.player_id ?? ''),
    game_id: String(item.game_id ?? item.gameId ?? ''),
    gameId: String(item.gameId ?? item.game_id ?? ''),
    title: String(item.title ?? ''),
    content: item.content ? String(item.content) : undefined,
    status: String(item.status ?? ''),
    category: item.category ? String(item.category) : undefined,
    priority: item.priority ? String(item.priority) : undefined,
    assignee: item.assignee ? String(item.assignee) : undefined,
    tags: splitTags(item.tags),
    created_at: String(item.created_at ?? item.createdAt ?? ''),
    createdAt: String(item.createdAt ?? item.created_at ?? ''),
    updated_at: String(item.updated_at ?? item.updatedAt ?? ''),
    updatedAt: String(item.updatedAt ?? item.updated_at ?? ''),
  };
}

function normalizeComment(item: Record<string, JSONValue>): TicketComment {
  return {
    id: Number(item.id ?? 0),
    content: String(item.content ?? ''),
    author: item.author ? String(item.author) : undefined,
    created_at: String(item.created_at ?? item.createdAt ?? ''),
    createdAt: String(item.createdAt ?? item.created_at ?? ''),
  };
}

function normalizeFAQ(item: Record<string, JSONValue>): FAQ {
  return {
    id: Number(item.id ?? 0),
    title: String(item.title ?? ''),
    content: String(item.content ?? ''),
    category: item.category ? String(item.category) : undefined,
    tags: splitTags(item.tags),
    visible: typeof item.visible === 'boolean' ? item.visible : undefined,
    sort: typeof item.sort === 'number' ? item.sort : undefined,
    created_at: String(item.created_at ?? item.createdAt ?? ''),
    createdAt: String(item.createdAt ?? item.created_at ?? ''),
    updated_at: String(item.updated_at ?? item.updatedAt ?? ''),
    updatedAt: String(item.updatedAt ?? item.updated_at ?? ''),
  };
}

function normalizeFeedback(item: Record<string, JSONValue>): Feedback {
  return {
    id: Number(item.id ?? 0),
    player_id: String(item.player_id ?? item.playerId ?? ''),
    playerId: String(item.playerId ?? item.player_id ?? ''),
    game_id: String(item.game_id ?? item.gameId ?? ''),
    gameId: String(item.gameId ?? item.game_id ?? ''),
    title: String(item.title ?? ''),
    content: item.content ? String(item.content) : undefined,
    status: String(item.status ?? ''),
    category: item.category ? String(item.category) : undefined,
    created_at: String(item.created_at ?? item.createdAt ?? ''),
    createdAt: String(item.createdAt ?? item.created_at ?? ''),
    updated_at: String(item.updated_at ?? item.updatedAt ?? ''),
    updatedAt: String(item.updatedAt ?? item.updated_at ?? ''),
  };
}

function buildTicketPayload(data: TicketPayload): Record<string, JSONValue> {
  return {
    ...data,
    playerId: data.playerId ?? data.player_id ?? '',
    gameId: data.gameId ?? data.game_id ?? '',
    tags: splitTags(data.tags),
  };
}

function buildFAQPayload(data: FAQPayload): Record<string, JSONValue> {
  return {
    ...data,
    tags: splitTags(data.tags),
    visible: typeof data.visible === 'boolean' ? data.visible : Boolean(data.visible),
    sort: typeof data.sort === 'string' ? Number(data.sort) || 0 : data.sort ?? 0,
  };
}

function buildFeedbackPayload(data: FeedbackPayload): Record<string, JSONValue> {
  return {
    ...data,
    playerId: data.playerId ?? data.player_id ?? '',
    gameId: data.gameId ?? data.game_id ?? '',
  };
}

// ---------------------------------------------------------------------------
// Tickets
// ---------------------------------------------------------------------------

export async function listTickets(params?: TicketListParams) {
  const resp = await request<{ items?: Record<string, JSONValue>[]; total?: number; page?: number; pageSize?: number }>(
    TICKETS_BASE,
    {
      params: {
        page: params?.page,
        pageSize: params?.pageSize || params?.size,
        status: params?.status,
        category: params?.category,
        priority: params?.priority,
        assignee: params?.assignee,
      },
    },
  );
  const tickets = toArray(resp?.items).map(normalizeTicket);
  return {
    tickets,
    items: tickets,
    total: resp?.total || 0,
    page: resp?.page || params?.page || 1,
    size: resp?.pageSize || params?.pageSize || params?.size || 20,
  };
}

export async function createTicket(data: TicketPayload) {
  const resp = await request<Record<string, JSONValue>>(TICKETS_BASE, { method: 'POST', data: buildTicketPayload(data) });
  return normalizeTicket(resp);
}

export async function updateTicket(id: number, data: TicketPayload) {
  const resp = await request<Record<string, JSONValue>>(`${TICKETS_BASE}/${id}`, {
    method: 'PUT',
    data: buildTicketPayload(data),
  });
  return normalizeTicket(resp);
}

export async function deleteTicket(id: number) {
  return request<void>(`${TICKETS_BASE}/${id}`, { method: 'DELETE' });
}

export async function getTicket(id: string | number) {
  const resp = await request<Record<string, JSONValue>>(`${TICKETS_BASE}/${id}`);
  const normalized = normalizeTicket(resp);
  return {
    ...normalized,
    comments: toArray(resp?.comments as Record<string, JSONValue>[]).map(normalizeComment),
  };
}

export async function listTicketComments(id: string | number) {
  const resp = await request<{ items?: Record<string, JSONValue>[]; comments?: Record<string, JSONValue>[] }>(`${TICKETS_BASE}/${id}/comments`);
  const comments = toArray(resp?.items ?? resp?.comments).map(normalizeComment);
  return { comments, items: comments };
}

export async function addTicketComment(
  id: string | number,
  data: { content: string; attach?: JSONValue; note?: string },
) {
  const resp = await request<{ items?: Record<string, JSONValue>[]; comments?: Record<string, JSONValue>[] }>(
    `${TICKETS_BASE}/${id}/comments`,
    {
      method: 'POST',
      data: { content: data.content },
    },
  );
  const comments = toArray(resp?.items ?? resp?.comments).map(normalizeComment);
  return { comments, items: comments };
}

export async function transitionTicket(
  id: string | number,
  data: { status?: string; comment?: string; attach?: JSONValue; note?: string },
) {
  return request<Record<string, JSONValue>>(`${TICKETS_BASE}/${id}/transition`, {
    method: 'POST',
    data: {
      status: data.status,
      note: data.note ?? data.comment ?? '',
    },
  });
}

// ---------------------------------------------------------------------------
// FAQ
// ---------------------------------------------------------------------------

export async function listFAQ(params?: FAQListParams) {
  const resp = await request<{ items?: Record<string, JSONValue>[]; total?: number; page?: number; pageSize?: number }>(
    FAQ_BASE,
    {
      params: {
        page: params?.page,
        pageSize: params?.pageSize || params?.size,
        category: params?.category,
        keyword: params?.keyword ?? params?.q,
        visible: parseVisible(params?.visible),
      },
    },
  );
  const faq = toArray(resp?.items).map(normalizeFAQ);
  return {
    faq,
    items: faq,
    total: resp?.total || 0,
    page: resp?.page || params?.page || 1,
    size: resp?.pageSize || params?.pageSize || params?.size || faq.length,
  };
}

export async function createFAQ(data: FAQPayload) {
  const resp = await request<Record<string, JSONValue>>(FAQ_BASE, { method: 'POST', data: buildFAQPayload(data) });
  return normalizeFAQ(resp);
}

export async function updateFAQ(id: number, data: FAQPayload) {
  const resp = await request<Record<string, JSONValue>>(`${FAQ_BASE}/${id}`, {
    method: 'PUT',
    data: buildFAQPayload(data),
  });
  return normalizeFAQ(resp);
}

export async function deleteFAQ(id: number) {
  return request<void>(`${FAQ_BASE}/${id}`, { method: 'DELETE' });
}

// ---------------------------------------------------------------------------
// Feedback
// ---------------------------------------------------------------------------

export async function listFeedback(params?: FeedbackListParams) {
  const resp = await request<{ items?: Record<string, JSONValue>[]; total?: number; page?: number; pageSize?: number }>(
    FEEDBACK_BASE,
    {
      params: {
        page: params?.page,
        pageSize: params?.pageSize || params?.size,
        status: params?.status,
        category: params?.category,
        gameId: params?.gameId ?? params?.game_id,
      },
    },
  );
  const feedback = toArray(resp?.items).map(normalizeFeedback);
  return {
    feedback,
    items: feedback,
    total: resp?.total || 0,
    page: resp?.page || params?.page || 1,
    size: resp?.pageSize || params?.pageSize || params?.size || 20,
  };
}

export async function createFeedback(data: FeedbackPayload) {
  const resp = await request<Record<string, JSONValue>>(FEEDBACK_BASE, {
    method: 'POST',
    data: buildFeedbackPayload(data),
  });
  return normalizeFeedback(resp);
}

export async function updateFeedback(id: number, data: FeedbackPayload) {
  const resp = await request<Record<string, JSONValue>>(`${FEEDBACK_BASE}/${id}`, {
    method: 'PUT',
    data: buildFeedbackPayload(data),
  });
  return normalizeFeedback(resp);
}

export async function deleteFeedback(id: number) {
  return request<void>(`${FEEDBACK_BASE}/${id}`, { method: 'DELETE' });
}
