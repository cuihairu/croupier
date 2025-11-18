import { request } from '@umijs/max';

// Tickets
export async function listTickets(params?: any) {
  return request<{ tickets: any[]; total: number; page: number; size: number }>(
    '/api/support/tickets',
    { params },
  );
}
export async function createTicket(data: any) {
  return request<{ id: number }>('/api/support/tickets', { method: 'POST', data });
}
export async function updateTicket(id: number, data: any) {
  return request<void>(`/api/support/tickets/${id}`, { method: 'PUT', data });
}
export async function deleteTicket(id: number) {
  return request<void>(`/api/support/tickets/${id}`, { method: 'DELETE' });
}

// FAQ
export async function listFAQ(params?: any) {
  return request<{ faq: any[] }>('/api/support/faq', { params });
}
export async function createFAQ(data: any) {
  return request<{ id: number }>('/api/support/faq', { method: 'POST', data });
}
export async function updateFAQ(id: number, data: any) {
  return request<void>(`/api/support/faq/${id}`, { method: 'PUT', data });
}
export async function deleteFAQ(id: number) {
  return request<void>(`/api/support/faq/${id}`, { method: 'DELETE' });
}

// Feedback
export async function listFeedback(params?: any) {
  return request<{ feedback: any[]; total: number; page: number; size: number }>(
    '/api/support/feedback',
    { params },
  );
}
export async function createFeedback(data: any) {
  return request<{ id: number }>('/api/support/feedback', { method: 'POST', data });
}
export async function updateFeedback(id: number, data: any) {
  return request<void>(`/api/support/feedback/${id}`, { method: 'PUT', data });
}
export async function deleteFeedback(id: number) {
  return request<void>(`/api/support/feedback/${id}`, { method: 'DELETE' });
}

