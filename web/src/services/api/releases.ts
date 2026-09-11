import { getIntl, request } from '@umijs/max';

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

// 展示 label 经 getIntl 解析（SelectLang 切换语言会整页刷新重新求值）；
// Map key / options value 为后端枚举契约，保持不动
const intl = getIntl();

export const releaseStatusLabels: Record<string, string> = {
  draft: intl.formatMessage({
    id: 'services.releases.statusLabel.draft',
    defaultMessage: '草稿',
  }),
  uploading: intl.formatMessage(
    // artifact uploaded, awaiting testing promotion
    { id: 'services.releases.statusLabel.uploading', defaultMessage: '待验证' },
  ),
  testing: intl.formatMessage({
    id: 'services.releases.statusLabel.testing',
    defaultMessage: '内测',
  }),
  gray: intl.formatMessage({ id: 'services.releases.statusLabel.gray', defaultMessage: '灰度' }),
  full: intl.formatMessage({ id: 'services.releases.statusLabel.full', defaultMessage: '全量' }),
  archived: intl.formatMessage({
    id: 'services.releases.statusLabel.archived',
    defaultMessage: '已归档',
  }),
  rolled_back: intl.formatMessage({
    id: 'services.releases.statusLabel.rolledBack',
    defaultMessage: '已回滚',
  }),
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
  hotfix: intl.formatMessage({
    id: 'services.releases.typeLabel.hotfix',
    defaultMessage: '热更',
  }),
  full: intl.formatMessage({ id: 'services.releases.typeLabel.full', defaultMessage: '整包' }),
  forced: intl.formatMessage({
    id: 'services.releases.typeLabel.forced',
    defaultMessage: '强更',
  }),
};

export const releasePlatformLabels: Record<string, string> = {
  ios: 'iOS',
  android: 'Android',
  pc: 'PC',
  webgl: 'WebGL',
};
