import { request } from '@umijs/max';

const TICKETS_BASE = '/api/v1/tickets';
const FAQ_BASE = '/api/v1/faqs';
const FEEDBACK_BASE = '/api/v1/feedback';

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

function normalizeTicket(item: any) {
  if (!item || typeof item !== 'object') return item;
  return {
    ...item,
    player_id: item.player_id ?? item.playerId ?? '',
    playerId: item.playerId ?? item.player_id ?? '',
    game_id: item.game_id ?? item.gameId ?? '',
    gameId: item.gameId ?? item.game_id ?? '',
    created_at: item.created_at ?? item.createdAt ?? '',
    createdAt: item.createdAt ?? item.created_at ?? '',
    updated_at: item.updated_at ?? item.updatedAt ?? '',
    updatedAt: item.updatedAt ?? item.updated_at ?? '',
    tags: splitTags(item.tags),
  };
}

function normalizeComment(item: any) {
  if (!item || typeof item !== 'object') return item;
  return {
    ...item,
    created_at: item.created_at ?? item.createdAt ?? '',
    createdAt: item.createdAt ?? item.created_at ?? '',
  };
}

function normalizeFAQ(item: any) {
  if (!item || typeof item !== 'object') return item;
  return {
    ...item,
    created_at: item.created_at ?? item.createdAt ?? '',
    createdAt: item.createdAt ?? item.created_at ?? '',
    updated_at: item.updated_at ?? item.updatedAt ?? '',
    updatedAt: item.updatedAt ?? item.updated_at ?? '',
    tags: splitTags(item.tags),
  };
}

function normalizeFeedback(item: any) {
  if (!item || typeof item !== 'object') return item;
  return {
    ...item,
    player_id: item.player_id ?? item.playerId ?? '',
    playerId: item.playerId ?? item.player_id ?? '',
    game_id: item.game_id ?? item.gameId ?? '',
    gameId: item.gameId ?? item.game_id ?? '',
    created_at: item.created_at ?? item.createdAt ?? '',
    createdAt: item.createdAt ?? item.created_at ?? '',
    updated_at: item.updated_at ?? item.updatedAt ?? '',
    updatedAt: item.updatedAt ?? item.updated_at ?? '',
  };
}

function buildTicketPayload(data: any) {
  return {
    ...data,
    playerId: data?.playerId ?? data?.player_id ?? '',
    gameId: data?.gameId ?? data?.game_id ?? '',
    tags: splitTags(data?.tags),
  };
}

function buildFAQPayload(data: any) {
  return {
    ...data,
    tags: splitTags(data?.tags),
    visible: typeof data?.visible === 'boolean' ? data.visible : Boolean(data?.visible),
    sort: typeof data?.sort === 'string' ? Number(data.sort) || 0 : data?.sort ?? 0,
  };
}

function buildFeedbackPayload(data: any) {
  return {
    ...data,
    playerId: data?.playerId ?? data?.player_id ?? '',
    gameId: data?.gameId ?? data?.game_id ?? '',
  };
}

// Tickets
export async function listTickets(params?: any) {
  const resp = await request<{ items?: any[]; total?: number; page?: number; pageSize?: number }>(
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

export async function createTicket(data: any) {
  const resp = await request<any>(TICKETS_BASE, { method: 'POST', data: buildTicketPayload(data) });
  return normalizeTicket(resp);
}

export async function updateTicket(id: number, data: any) {
  const resp = await request<any>(`${TICKETS_BASE}/${id}`, {
    method: 'PUT',
    data: buildTicketPayload(data),
  });
  return normalizeTicket(resp);
}

export async function deleteTicket(id: number) {
  return request<void>(`${TICKETS_BASE}/${id}`, { method: 'DELETE' });
}

export async function getTicket(id: string | number) {
  const resp = await request<any>(`${TICKETS_BASE}/${id}`);
  const normalized = normalizeTicket(resp);
  return {
    ...normalized,
    comments: toArray(resp?.comments).map(normalizeComment),
  };
}

export async function listTicketComments(id: string | number) {
  const resp = await request<{ items?: any[]; comments?: any[] }>(`${TICKETS_BASE}/${id}/comments`);
  const comments = toArray(resp?.items ?? resp?.comments).map(normalizeComment);
  return { comments, items: comments };
}

export async function addTicketComment(
  id: string | number,
  data: { content: string; attach?: any; note?: string },
) {
  const resp = await request<{ items?: any[]; comments?: any[] }>(
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
  data: { status?: string; comment?: string; attach?: any; note?: string },
) {
  return request<any>(`${TICKETS_BASE}/${id}/transition`, {
    method: 'POST',
    data: {
      status: data.status,
      note: data.note ?? data.comment ?? '',
    },
  });
}

// FAQ
export async function listFAQ(params?: any) {
  const resp = await request<{ items?: any[]; total?: number; page?: number; pageSize?: number }>(
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

export async function createFAQ(data: any) {
  const resp = await request<any>(FAQ_BASE, { method: 'POST', data: buildFAQPayload(data) });
  return normalizeFAQ(resp);
}

export async function updateFAQ(id: number, data: any) {
  const resp = await request<any>(`${FAQ_BASE}/${id}`, {
    method: 'PUT',
    data: buildFAQPayload(data),
  });
  return normalizeFAQ(resp);
}

export async function deleteFAQ(id: number) {
  return request<void>(`${FAQ_BASE}/${id}`, { method: 'DELETE' });
}

// Feedback
export async function listFeedback(params?: any) {
  const resp = await request<{ items?: any[]; total?: number; page?: number; pageSize?: number }>(
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

export async function createFeedback(data: any) {
  const resp = await request<any>(FEEDBACK_BASE, {
    method: 'POST',
    data: buildFeedbackPayload(data),
  });
  return normalizeFeedback(resp);
}

export async function updateFeedback(id: number, data: any) {
  const resp = await request<any>(`${FEEDBACK_BASE}/${id}`, {
    method: 'PUT',
    data: buildFeedbackPayload(data),
  });
  return normalizeFeedback(resp);
}

export async function deleteFeedback(id: number) {
  return request<void>(`${FEEDBACK_BASE}/${id}`, { method: 'DELETE' });
}
