import { request } from '@umijs/max';

// Source: internal/api/hotpatch/dto.go
export type HotpatchResult = {
  agentId: string;
  node?: string;
  status: string; // ok | failed | rolled_back
  log?: string;
  at: string;
};

export type HotpatchItem = {
  id: number;
  gameId?: string;
  env?: string;
  framework: string;
  status: string;
  targets?: string[];
  entrySpec?: Record<string, unknown>;
  packageKey?: string;
  size?: number;
  checksum?: string;
  rolloutPercent: number;
  bugId: number;
  results?: HotpatchResult[];
  createdBy?: string;
  createdAt: string;
  updatedAt: string;
};

export async function listHotpatches(params?: {
  gameId?: string;
  env?: string;
  framework?: string;
  status?: string;
  page?: number;
  pageSize?: number;
}): Promise<{ items: HotpatchItem[]; total: number; page: number; pageSize: number }> {
  return request('/api/v1/hotpatches', { params });
}

export async function createHotpatch(payload: {
  gameId: string;
  env?: string;
  framework: string;
  targets?: string[];
  entrySpec?: Record<string, unknown>;
  bugId: number;
  title: string;
}): Promise<HotpatchItem> {
  return request('/api/v1/hotpatches', { method: 'POST', data: payload });
}

export async function uploadHotpatchPackage(
  id: number | string,
  file: File,
): Promise<HotpatchItem> {
  const form = new FormData();
  form.append('file', file);
  return request(`/api/v1/hotpatches/${encodeURIComponent(String(id))}/package`, {
    method: 'POST',
    data: form,
  });
}

export async function transitionHotpatch(
  id: number | string,
  action: 'approve' | 'roll' | 'applied' | 'fail' | 'rollback',
  rolloutPercent?: number,
): Promise<HotpatchItem> {
  return request(`/api/v1/hotpatches/${encodeURIComponent(String(id))}/transition`, {
    method: 'POST',
    data: { action, rolloutPercent },
  });
}

export const hotpatchStatusLabels: Record<string, string> = {
  draft: '草稿',
  approved: '已审批',
  rolling: '灰度中',
  applied: '已生效',
  failed: '失败',
  rolled_back: '已回滚',
};

export const hotpatchStatusColors: Record<string, string> = {
  draft: 'default',
  approved: 'purple',
  rolling: 'orange',
  applied: 'green',
  failed: 'red',
  rolled_back: 'red',
};

export const hotpatchFrameworkLabels: Record<string, string> = {
  skynet: 'skynet (Lua)',
  kbengine: 'KBEngine (Python)',
  jvm: 'JVM (Java)',
  nodejs: 'Node.js (JS/TS)',
  custom: '自定义',
};
