import { request } from '@umijs/max';

// Source: internal/api/release/dto.go
export type Release = {
  id: number;
  gameId: string;
  env?: string;
  channel: string;
  platform: string;
  version: string;
  type: string;
  status: string;
  objectKey?: string;
  size?: number;
  checksum?: string;
  notes?: Record<string, unknown>;
  grayPercent: number;
  whitelist?: string[];
  createdBy?: string;
  createdAt: string;
  updatedAt: string;
};

export type ReleaseListParams = {
  gameId?: string;
  env?: string;
  channel?: string;
  platform?: string;
  status?: string;
  page?: number;
  pageSize?: number;
};

export async function listReleases(
  params?: ReleaseListParams,
): Promise<{ items: Release[]; total: number; page: number; pageSize: number }> {
  return request('/api/v1/releases', { params });
}

export async function createRelease(payload: {
  gameId: string;
  channel: string;
  platform: string;
  version: string;
  type?: string;
  notes?: Record<string, unknown>;
}): Promise<Release> {
  return request('/api/v1/releases', { method: 'POST', data: payload });
}

export async function uploadReleaseArtifact(
  id: number | string,
  file: File,
  manifest?: unknown,
): Promise<Release> {
  const form = new FormData();
  form.append('file', file);
  if (manifest !== undefined && manifest !== null) {
    form.append('manifest', JSON.stringify(manifest));
  }
  return request(`/api/v1/releases/${encodeURIComponent(String(id))}/artifact`, {
    method: 'POST',
    data: form,
  });
}

export async function transitionRelease(
  id: number | string,
  action: 'testing' | 'gray' | 'full' | 'archive' | 'rollback',
  grayPercent?: number,
): Promise<Release> {
  return request(`/api/v1/releases/${encodeURIComponent(String(id))}/transition`, {
    method: 'POST',
    data: { action, grayPercent },
  });
}

export const releaseStatusLabels: Record<string, string> = {
  draft: '草稿',
  uploading: '待验证', // artifact uploaded, awaiting testing promotion
  testing: '内测',
  gray: '灰度',
  full: '全量',
  archived: '已归档',
  rolled_back: '已回滚',
};

export const releaseStatusColors: Record<string, string> = {
  draft: 'default',
  uploading: 'default',
  testing: 'purple',
  gray: 'orange',
  full: 'green',
  archived: 'default',
  rolled_back: 'red',
};

export const releaseTypeLabels: Record<string, string> = {
  hotfix: '热更',
  full: '整包',
  forced: '强更',
};

export const releasePlatformLabels: Record<string, string> = {
  ios: 'iOS',
  android: 'Android',
  pc: 'PC',
  webgl: 'WebGL',
};
