import { request } from '@umijs/max';

export async function listPacks() {
  return request<{ manifest: any; counts: { descriptors: number; ui_schema: number }; etag?: string; export_auth_required?: boolean }>('/api/packs/list');
}

export async function reloadPacks() {
  return request<{ ok: boolean }>('/api/packs/reload', { method: 'POST' });
}

