import { request } from '@umijs/max';
import type { JSONValue } from '@/types/dashboard';

// ============================================================================
// 类型定义
// ============================================================================

export interface AlertDetails {
  [key: string]: JSONValue;
}

export interface Alert {
  id: string;
  type: string;
  level: string;
  message: string;
  source: string;
  status: string;
  details?: AlertDetails;
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

export interface SilenceMatchers {
  [key: string]: JSONValue;
}

export interface Silence {
  id: string;
  alertType: string;
  matchers?: SilenceMatchers;
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
  return request<AlertsListResponse>('/api/v1/alerts', {
    method: 'GET',
    params,
  });
}

/**
 * 静默告警
 */
export async function silenceAlert(id: string, duration: number, reason?: string) {
  return request<void>(`/api/v1/alerts/${id}/silence`, {
    method: 'POST',
    data: { duration, reason },
  });
}

/**
 * 获取静默规则列表
 */
export async function listAlertSilences() {
  return request<SilencesListResponse>('/api/v1/alerts/silences', {
    method: 'GET',
  });
}

/**
 * 删除静默规则
 */
export async function deleteAlertSilence(id: string) {
  return request<void>(`/api/v1/alerts/silences/${id}`, {
    method: 'DELETE',
  });
}
