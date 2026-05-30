import { request } from '@umijs/max';

// ============================================================================
// 类型定义
// ============================================================================

export interface Alert {
  id: string;
  type: string;
  level: string;
  message: string;
  source: string;
  status: string;
  details?: any;
  createdAt: string;
}

export interface AlertsListParams {
  page?: number;
  pageSize?: number;
  level?: string;
  status?: string;
}

export interface AlertsListResponse {
  items: Alert[];
  total: number;
  page: number;
  pageSize: number;
}

export interface Silence {
  id: string;
  alertType: string;
  matchers?: any;
  startAt: string;
  endAt: string;
  createdBy: string;
}

export interface SilencesListResponse {
  items: Silence[];
}

// ============================================================================
// API 函数
// ============================================================================

/**
 * 获取告警列表
 */
export async function listAlerts(params?: AlertsListParams) {
  const token = typeof window !== 'undefined' ? localStorage.getItem('token') : '';
  return request<AlertsListResponse>('/api/v1/alerts', {
    method: 'GET',
    params,
    headers: token ? { Authorization: `Bearer ${token}` } : undefined,
  });
}

/**
 * 静默告警
 */
export async function silenceAlert(id: string, duration: number, reason?: string) {
  const token = typeof window !== 'undefined' ? localStorage.getItem('token') : '';
  return request<void>(`/api/v1/alerts/${id}/silence`, {
    method: 'POST',
    data: { duration, reason },
    headers: token ? { Authorization: `Bearer ${token}` } : undefined,
  });
}

/**
 * 获取静默规则列表
 */
export async function listAlertSilences() {
  const token = typeof window !== 'undefined' ? localStorage.getItem('token') : '';
  return request<SilencesListResponse>('/api/v1/alerts/silences', {
    method: 'GET',
    headers: token ? { Authorization: `Bearer ${token}` } : undefined,
  });
}

/**
 * 删除静默规则
 */
export async function deleteAlertSilence(id: string) {
  const token = typeof window !== 'undefined' ? localStorage.getItem('token') : '';
  return request<void>(`/api/v1/alerts/silences/${id}`, {
    method: 'DELETE',
    headers: token ? { Authorization: `Bearer ${token}` } : undefined,
  });
}
