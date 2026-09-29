import { request } from '@umijs/max';
import { getScope, scopeReadyPromise } from '@/stores/scope';

// 外部 CI/CD 接入（OPEN-ISSUES #58，可插拔 provider）。
// Source: internal/api/cicd/service.go

export type CicdKind = 'jenkins' | 'gitlab-ci' | 'github-actions' | 'generic';

export type CicdIntegration = {
  id: number;
  gameId: string;
  env: string;
  kind: CicdKind;
  name: string;
  endpoint: string;
  /** 是否已配置凭据（明文不回显） */
  tokenSet: boolean;
  /** 掩码回显：****+尾4 */
  tokenMasked: string;
  /** provider 专属键值（jenkins: job/crumbDisabled；gitlab-ci: project/ref；
   *  github-actions: repo/workflow/ref；generic: pipeline/triggerUrl/statusUrl/headerName/headerValue） */
  extra: Record<string, string>;
  enabled: boolean;
  createdBy: string;
  updatedAt: string;
};

export type CicdIntegrationListResp = {
  items: CicdIntegration[];
  /** 注册表可用类型（后端返回，前端动态表单据此渲染） */
  kinds: string[];
};

export type CicdIntegrationInput = {
  gameId?: string;
  env?: string;
  kind: CicdKind;
  name: string;
  endpoint: string;
  /** 空串 = 保留原凭据（更新时） */
  token?: string;
  extra?: Record<string, string>;
  enabled?: boolean;
};

export type CicdBuild = {
  id: number;
  gameId: string;
  env: string;
  integrationId: number;
  kind: CicdKind;
  pipeline: string;
  externalId: string;
  /** queued|running|success|failed|cancelled|unknown */
  status: string;
  triggeredBy: 'api' | 'webhook';
  version: string;
  webUrl: string;
  artifactUrl: string;
  artifactChecksum: string;
  startedAt?: string;
  finishedAt?: string;
  createdAt: string;
};

export type CicdBuildListResp = {
  items: CicdBuild[];
  total: number;
};

const withScope = (params?: Record<string, unknown>): Record<string, unknown> => {
  const scope = getScope();
  return { gameId: scope.gameId ?? '', env: scope.env ?? '', ...params };
};

/** 接入列表 + 注册类型闭集（等待顶栏 scope 校验完成，避免拿旧 scope 查询） */
export async function fetchCicdIntegrations(): Promise<CicdIntegrationListResp> {
  await scopeReadyPromise;
  return request<CicdIntegrationListResp>('/api/v1/cicd/integrations', {
    params: withScope(),
    skipErrorHandler: true,
  });
}

export async function createCicdIntegration(
  input: CicdIntegrationInput,
): Promise<{ integration: CicdIntegration }> {
  await scopeReadyPromise;
  return request('/api/v1/cicd/integrations', {
    method: 'POST',
    data: { ...input, gameId: getScope().gameId ?? '', env: getScope().env ?? '' },
    skipErrorHandler: true,
  });
}

export async function updateCicdIntegration(
  id: number,
  input: Partial<CicdIntegrationInput>,
): Promise<{ integration: CicdIntegration }> {
  return request(`/api/v1/cicd/integrations/${id}`, {
    method: 'PUT',
    data: input,
    skipErrorHandler: true,
  });
}

export async function deleteCicdIntegration(id: number): Promise<void> {
  await request(`/api/v1/cicd/integrations/${id}`, { method: 'DELETE' });
}

export async function testCicdConnection(id: number): Promise<{ ok: boolean; message: string }> {
  return request(`/api/v1/cicd/integrations/${id}/test`, {
    method: 'POST',
    skipErrorHandler: true,
  });
}

export async function triggerCicdBuild(
  id: number,
  input: { pipeline?: string; params?: Record<string, string>; version?: string },
): Promise<{ integration: CicdIntegration; build: CicdBuild }> {
  await scopeReadyPromise;
  return request(`/api/v1/cicd/integrations/${id}/trigger`, {
    method: 'POST',
    data: input,
    skipErrorHandler: true,
  });
}

export async function fetchCicdBuilds(
  params?: Record<string, unknown>,
): Promise<CicdBuildListResp> {
  await scopeReadyPromise;
  return request<CicdBuildListResp>('/api/v1/cicd/builds', {
    params: withScope(params),
    skipErrorHandler: true,
  });
}

export async function refreshCicdBuild(id: number): Promise<{ build: CicdBuild }> {
  return request(`/api/v1/cicd/builds/${id}/refresh`, {
    method: 'POST',
    skipErrorHandler: true,
  });
}
