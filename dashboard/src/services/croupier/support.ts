import { request } from '@umijs/max';

const TICKETS_BASE = '/api/v1/tickets';
const FAQ_BASE = '/api/v1/faqs';
const FEEDBACK_BASE = '/api/v1/feedback';

// Tickets
export async function listTickets(params?: any) {
  return request<{ tickets: any[]; total: number; page: number; size: number }>(TICKETS_BASE, { params });
}
export async function createTicket(data: any) {
  return request<{ id: number }>(TICKETS_BASE, { method: 'POST', data });
}
export async function updateTicket(id: number, data: any) {
  return request<void>(`${TICKETS_BASE}/${id}`, { method: 'PUT', data });
}
export async function deleteTicket(id: number) {
  return request<void>(`${TICKETS_BASE}/${id}`, { method: 'DELETE' });
}

export async function getTicket(id: string | number) {
  return request<any>(`${TICKETS_BASE}/${id}`);
}

export async function listTicketComments(id: string | number) {
  return request<{ comments: any[] }>(`${TICKETS_BASE}/${id}/comments`);
}

export async function addTicketComment(id: string | number, data: { content: string; attach?: any }) {
  return request<void>(`${TICKETS_BASE}/${id}/comments`, { method: 'POST', data });
}

export async function transitionTicket(
  id: string | number,
  data: { status?: string; comment?: string; attach?: any },
) {
  return request<void>(`${TICKETS_BASE}/${id}/transition`, { method: 'POST', data });
}

// FAQ
export async function listFAQ(params?: any) {
  return request<{ faq: any[] }>(FAQ_BASE, { params });
}
export async function createFAQ(data: any) {
  return request<{ id: number }>(FAQ_BASE, { method: 'POST', data });
}
export async function updateFAQ(id: number, data: any) {
  return request<void>(`${FAQ_BASE}/${id}`, { method: 'PUT', data });
}
export async function deleteFAQ(id: number) {
  return request<void>(`${FAQ_BASE}/${id}`, { method: 'DELETE' });
}

// Feedback
export async function listFeedback(params?: any) {
  return request<{ feedback: any[]; total: number; page: number; size: number }>(FEEDBACK_BASE, { params });
}
export async function createFeedback(data: any) {
  return request<{ id: number }>(FEEDBACK_BASE, { method: 'POST', data });
}
export async function updateFeedback(id: number, data: any) {
  return request<void>(`${FEEDBACK_BASE}/${id}`, { method: 'PUT', data });
}
export async function deleteFeedback(id: number) {
  return request<void>(`${FEEDBACK_BASE}/${id}`, { method: 'DELETE' });
}
