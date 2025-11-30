import { request } from '@umijs/max';

const BASE = '/api/v1/configs';

export async function listConfigs(params: any) {
  return request<{ items: any[] }>(BASE, { params });
}
export async function getConfig(id: string, params: any) {
  return request<any>(`${BASE}/${encodeURIComponent(id)}`, { params });
}
export async function validateConfig(id: string, data: { format: string; content: string }) {
  return request<any>(`${BASE}/${encodeURIComponent(id)}/validate`, { method: 'POST', data });
}
export async function saveConfig(id: string, data: { game_id: string; env: string; format: string; content: string; message?: string; base_version?: number }) {
  return request<any>(`${BASE}/${encodeURIComponent(id)}`, {
    method: 'POST',
    data: { game_id: data.game_id, env: data.env, format: data.format, content: data.content, message: data.message || '', base_version: data.base_version || 0 },
  });
}
export async function listVersions(id: string, params: any) {
  return request<any>(`${BASE}/${encodeURIComponent(id)}/versions`, { params });
}
export async function getVersion(id: string, ver: number, params: any) {
  return request<any>(`${BASE}/${encodeURIComponent(id)}/versions/${ver}`, { params });
}
