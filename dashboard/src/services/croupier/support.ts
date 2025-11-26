import { request } from '@umijs/max';

// Tickets
export async function listTickets(params?: any) {
  return request<{ tickets: any[]; total: number; page: number; size: number }>(
    '/api/v1/support/tickets',
    { params },
  );
}
export async function createTicket(data: any) {
  return request<{ id: number }>('/api/v1/support/tickets', { method: 'POST', data });
}
export async function updateTicket(id: number, data: any) {
  return request<void>(`/api/v1/support/tickets/${id}`, { method: 'PUT', data });
}
export async function deleteTicket(id: number) {
  return request<void>(`/api/v1/support/tickets/${id}`, { method: 'DELETE' });
}

export async function getTicket(id: string | number) {
  return request<any>(`/api/v1/support/tickets/${id}`);
}

export async function listTicketComments(id: string | number) {
  return request<{ comments: any[] }>(`/api/v1/support/tickets/${id}/comments`);
}

export async function addTicketComment(id: string | number, data: { content: string; attach?: any }) {
  return request<void>(`/api/v1/support/tickets/${id}/comments`, { method: 'POST', data });
}

export async function transitionTicket(
  id: string | number,
  data: { status?: string; comment?: string; attach?: any },
) {
  return request<void>(`/api/v1/support/tickets/${id}/transition`, { method: 'POST', data });
}

// FAQ
export async function listFAQ(params?: any) {
  return request<{ faq: any[] }>('/api/v1/support/faq', { params });
}
export async function createFAQ(data: any) {
  return request<{ id: number }>('/api/v1/support/faq', { method: 'POST', data });
}
export async function updateFAQ(id: number, data: any) {
  return request<void>(`/api/v1/support/faq/${id}`, { method: 'PUT', data });
}
export async function deleteFAQ(id: number) {
  return request<void>(`/api/v1/support/faq/${id}`, { method: 'DELETE' });
}

// Feedback
export async function listFeedback(params?: any) {
  return request<{ feedback: any[]; total: number; page: number; size: number }>(
    '/api/v1/support/feedback',
    { params },
  );
}
export async function createFeedback(data: any) {
  return request<{ id: number }>('/api/v1/support/feedback', { method: 'POST', data });
}
export async function updateFeedback(id: number, data: any) {
  return request<void>(`/api/v1/support/feedback/${id}`, { method: 'PUT', data });
}
export async function deleteFeedback(id: number) {
  return request<void>(`/api/v1/support/feedback/${id}`, { method: 'DELETE' });
}