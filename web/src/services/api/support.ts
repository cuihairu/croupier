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
  playerId: string;
  gameId: string;
  title: string;
  content?: string;
  status: string;
  category?: string;
  priority?: string;
  assignee?: string;
  tags: string[];
  // CSAT 满意度（1-5，0=未评；仅 resolved/closed 后可提交，重开清零）
  rating?: number;
  ratedBy?: string;
  createdAt: string;
  updatedAt: string;
  comments?: TicketComment[];
}

export interface TicketComment {
  id: number;
  content: string;
  author?: string;
  createdAt: string;
}

export interface FAQ {
  id: number;
  question: string;
  answer: string;
  category?: string;
  tags: string[];
  visible?: boolean;
  sort?: number;
  createdAt: string;
  updatedAt: string;
}

export interface Feedback {
  id: number;
  playerId: string;
  contact: string;
  content: string;
  category: string;
  priority: string;
  status: string;
  rating: number;
  attach: string;
  gameId: string;
  env: string;
  reply: string;
  createdAt: string;
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
  excludeStatus?: string;
  category?: string;
  gameId?: string;
  q?: string;
}

export interface TicketPayload {
  playerId?: string;
  gameId?: string;
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
  question?: string;
  answer?: string;
  category?: string;
  tags?: string | string[];
  visible?: boolean | string;
  sort?: number | string;
  [key: string]: JSONValue | undefined;
}

export interface FeedbackPayload {
  playerId?: string;
  contact?: string;
  content?: string;
  category?: string;
  rating?: number;
  attach?: string;
  gameId?: string;
  status?: string;
  priority?: string;
  reply?: string;
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
    playerId: String(item.playerId ?? ''),
    gameId: String(item.gameId ?? ''),
    title: String(item.title ?? ''),
    content: item.content ? String(item.content) : undefined,
    status: String(item.status ?? ''),
    category: item.category ? String(item.category) : undefined,
    priority: item.priority ? String(item.priority) : undefined,
    assignee: item.assignee ? String(item.assignee) : undefined,
    tags: splitTags(item.tags),
    createdAt: String(item.createdAt ?? ''),
    updatedAt: String(item.updatedAt ?? ''),
  };
}

function normalizeComment(item: Record<string, JSONValue>): TicketComment {
  return {
    id: Number(item.id ?? 0),
    content: String(item.content ?? ''),
    author: item.author ? String(item.author) : undefined,
    createdAt: String(item.createdAt ?? ''),
  };
}

function normalizeFAQ(item: Record<string, JSONValue>): FAQ {
  return {
    id: Number(item.id ?? 0),
    question: String(item.question ?? ''),
    answer: String(item.answer ?? ''),
    category: item.category ? String(item.category) : undefined,
    tags: splitTags(item.tags),
    visible: typeof item.visible === 'boolean' ? item.visible : undefined,
    sort: typeof item.sort === 'number' ? item.sort : undefined,
    createdAt: String(item.createdAt ?? ''),
    updatedAt: String(item.updatedAt ?? ''),
  };
}

function normalizeFeedback(item: Record<string, JSONValue>): Feedback {
  return {
    id: Number(item.id ?? 0),
    playerId: String(item.playerId ?? ''),
    contact: String(item.contact ?? ''),
    content: String(item.content ?? ''),
    category: String(item.category ?? ''),
    priority: String(item.priority ?? ''),
    status: String(item.status ?? ''),
    rating: Number(item.rating ?? 0),
    attach: String(item.attach ?? ''),
    gameId: String(item.gameId ?? ''),
    env: String(item.env ?? ''),
    reply: String(item.reply ?? ''),
    createdAt: String(item.createdAt ?? ''),
    updatedAt: String(item.updatedAt ?? ''),
  };
}

function buildTicketPayload(data: TicketPayload): Record<string, JSONValue> {
  return {
    ...data,
    playerId: data.playerId ?? '',
    gameId: data.gameId ?? '',
    tags: splitTags(data.tags),
  };
}

function buildFAQPayload(data: FAQPayload): Record<string, JSONValue> {
  return {
    ...data,
    tags: splitTags(data.tags),
    visible: typeof data.visible === 'boolean' ? data.visible : Boolean(data.visible),
    sort: typeof data.sort === 'string' ? Number(data.sort) || 0 : (data.sort ?? 0),
  };
}

function buildFeedbackPayload(data: FeedbackPayload): Record<string, JSONValue> {
  return {
    ...data,
    playerId: data.playerId ?? '',
    gameId: data.gameId ?? '',
  };
}

// ---------------------------------------------------------------------------
// Tickets
// ---------------------------------------------------------------------------

export async function listTickets(params?: TicketListParams) {
  const resp = await request<{
    items?: Record<string, JSONValue>[];
    total?: number;
    page?: number;
    pageSize?: number;
  }>(TICKETS_BASE, {
    params: {
      page: params?.page,
      pageSize: params?.pageSize || params?.size,
      q: params?.q,
      status: params?.status,
      category: params?.category,
      priority: params?.priority,
      assignee: params?.assignee,
      gameId: params?.gameId,
      env: params?.env,
    },
  });
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
  const resp = await request<Record<string, JSONValue>>(TICKETS_BASE, {
    method: 'POST',
    data: buildTicketPayload(data),
  });
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
  const resp = await request<{
    items?: Record<string, JSONValue>[];
    comments?: Record<string, JSONValue>[];
  }>(`${TICKETS_BASE}/${id}/comments`);
  const comments = toArray(resp?.items ?? resp?.comments).map(normalizeComment);
  return { comments, items: comments };
}

export async function addTicketComment(
  id: string | number,
  data: { content: string; attach?: JSONValue; note?: string },
) {
  const resp = await request<{
    items?: Record<string, JSONValue>[];
    comments?: Record<string, JSONValue>[];
  }>(`${TICKETS_BASE}/${id}/comments`, {
    method: 'POST',
    data: { content: data.content },
  });
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
  const resp = await request<{
    items?: Record<string, JSONValue>[];
    total?: number;
    page?: number;
    pageSize?: number;
  }>(FAQ_BASE, {
    params: {
      page: params?.page,
      pageSize: params?.pageSize || params?.size,
      category: params?.category,
      keyword: params?.keyword ?? params?.q,
      visible: parseVisible(params?.visible),
    },
  });
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
  const resp = await request<Record<string, JSONValue>>(FAQ_BASE, {
    method: 'POST',
    data: buildFAQPayload(data),
  });
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
  const resp = await request<{
    items?: Record<string, JSONValue>[];
    total?: number;
    page?: number;
    pageSize?: number;
  }>(FEEDBACK_BASE, {
    params: {
      page: params?.page,
      pageSize: params?.pageSize || params?.size,
      status: params?.status,
      excludeStatus: params?.excludeStatus,
      category: params?.category,
      q: params?.q,
      gameId: params?.gameId,
    },
  });
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

// 反馈一键转化工单（服务端携带玩家上下文；幂等：重复调用返回原工单）
export async function convertFeedbackToTicket(
  id: number,
  payload?: { title?: string; category?: string; priority?: string; note?: string },
): Promise<{ ticketId: string; alreadyConverted?: boolean }> {
  return request(`/api/v1/feedback/${encodeURIComponent(id)}/convert`, {
    method: 'POST',
    data: payload ?? {},
  });
}

// 工单升级为缺陷（bug-tracking P2：携带玩家上下文，source=ticket）
export async function convertTicketToBug(
  id: number,
  payload?: { severity?: string; platform?: string; steps?: string; fixVersion?: string },
): Promise<{ bugId: string }> {
  return request(`/api/v1/tickets/${encodeURIComponent(id)}/convert-bug`, {
    method: 'POST',
    data: payload ?? {},
  });
}

// 工单满意度评价（CSAT；服务端仅接受已解决/已关闭工单）
export async function rateTicket(id: number, rating: number): Promise<void> {
  await request(`/api/v1/tickets/${encodeURIComponent(id)}/rate`, {
    method: 'POST',
    data: { rating },
  });
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
