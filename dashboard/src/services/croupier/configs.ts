import { request } from '@umijs/max';

export async function listConfigs(params: any) {
  return request<{ items: any[] }>(`/api/configs`, { params });
}
export async function getConfig(id: string, params: any) {
  return request<any>(`/api/configs/${encodeURIComponent(id)}`, { params });
}
export async function validateConfig(id: string, data: { format: string; content: string }) {
  return request<any>(`/api/configs/${encodeURIComponent(id)}/validate`, { method: 'POST', data });
}
export async function saveConfig(id: string, data: { game_id: string; env: string; format: string; content: string; message?: string; base_version?: number }) {
  return request<any>(`/api/configs/${encodeURIComponent(id)}`, { method: 'POST', data: { GameID: data.game_id, Env: data.env, Format: data.format, Content: data.content, Message: data.message||'', BaseVersion: data.base_version||0 } });
}
export async function listVersions(id: string, params: any) {
  return request<any>(`/api/configs/${encodeURIComponent(id)}/versions`, { params });
}
export async function getVersion(id: string, ver: number, params: any) {
  return request<any>(`/api/configs/${encodeURIComponent(id)}/versions/${ver}`, { params });
}

