import { request } from '@umijs/max';

export async function listPacks() {
  return request<{ manifest: any; counts: { descriptors: number; ui_schema: number }; etag?: string; export_auth_required?: boolean }>('/api/v1/packs');
}

export async function reloadPacks() {
  return request<{ ok: boolean }>('/api/v1/packs/reload', { method: 'POST' });
}

