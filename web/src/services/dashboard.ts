/**
 * Dashboard API 服务。
 *
 * 提供 Resource Catalog、Versioning、Contract、Proposal 等 API 调用。
 */

import { request } from '@umijs/max';
import type {
  ResourceCatalogItem,
  PageProposal,
  ChangeChain,
  MergeRequest,
  MergeResponse,
  SemanticsInfo,
  JSONValue,
} from '@/types/dashboard';

const BASE_URL = '/api/v1';

/** 列出资源目录 */
export async function listResourceCatalog(params?: {
  category?: string;
  query?: string;
}): Promise<{ items: ResourceCatalogItem[]; total: number }> {
  return request(`${BASE_URL}/resource-catalog`, {
    method: 'GET',
    params,
  });
}

/** 获取资源详情 */
export async function getResourceDetail(resourceKey: string): Promise<ResourceCatalogItem> {
  return request(`${BASE_URL}/resource-catalog/${resourceKey}`, {
    method: 'GET',
  });
}

/** 更新资源语义 */
export async function updateResourceSemantics(
  resourceKey: string,
  data: Partial<SemanticsInfo> & { changeReason?: string },
): Promise<{ version: number; source: string; message: string }> {
  return request(`${BASE_URL}/resource-catalog/${resourceKey}/semantics`, {
    method: 'PUT',
    data,
  });
}

/** 获取变更链 */
export async function getChangeChain(resourceKey: string): Promise<ChangeChain> {
  return request(`${BASE_URL}/versioning/${resourceKey}/chain`, {
    method: 'GET',
  });
}

/** 版本对比 */
export async function getVersionDiff(
  resourceKey: string,
  params: { fromVersion: number; toVersion: number },
): Promise<{ changes: JSONValue[]; summary: string }> {
  return request(`${BASE_URL}/versioning/${resourceKey}/diff`, {
    method: 'GET',
    params,
  });
}

/** 合并变更 */
export async function mergeChanges(resourceKey: string, data: MergeRequest): Promise<MergeResponse> {
  return request(`${BASE_URL}/versioning/${resourceKey}/merge`, {
    method: 'POST',
    data,
  });
}

/** 回滚草稿 */
export async function rollbackDraft(
  resourceKey: string,
  data: { version: number; reason?: string },
): Promise<{ message: string }> {
  return request(`${BASE_URL}/versioning/${resourceKey}/rollback-draft`, {
    method: 'POST',
    data,
  });
}

/** 回滚发布 */
export async function rollbackPublish(
  resourceKey: string,
  data: { version: number; reason?: string },
): Promise<{ message: string }> {
  return request(`${BASE_URL}/versioning/${resourceKey}/rollback-publish`, {
    method: 'POST',
    data,
  });
}

/** 重新生成提案 */
export async function regenerateProposal(
  resourceKey: string,
  data?: { force?: boolean },
): Promise<{ message: string }> {
  return request(`${BASE_URL}/versioning/${resourceKey}/regenerate`, {
    method: 'POST',
    data,
  });
}

/** 重新发布 */
export async function republish(
  resourceKey: string,
  data?: { reason?: string },
): Promise<{ version: number; message: string }> {
  return request(`${BASE_URL}/versioning/${resourceKey}/republish`, {
    method: 'POST',
    data,
  });
}

/** 列出合约 */
export async function listContracts(): Promise<JSONValue[]> {
  return request(`${BASE_URL}/contracts`, {
    method: 'GET',
  });
}

/** 获取合约 */
export async function getContract(functionId: string): Promise<JSONValue> {
  return request(`${BASE_URL}/contracts/${functionId}`, {
    method: 'GET',
  });
}

/** 列出提案 */
export async function listProposals(params?: { status?: string }): Promise<PageProposal[]> {
  return request(`${BASE_URL}/proposals`, {
    method: 'GET',
    params,
  });
}

/** 获取提案 */
export async function getProposal(proposalKey: string): Promise<PageProposal> {
  return request(`${BASE_URL}/proposals/${proposalKey}`, {
    method: 'GET',
  });
}

/** 接受提案 */
export async function acceptProposal(proposalKey: string): Promise<{ message: string }> {
  return request(`${BASE_URL}/proposals/${proposalKey}/accept`, {
    method: 'POST',
  });
}

/** 拒绝提案 */
export async function rejectProposal(proposalKey: string): Promise<{ message: string }> {
  return request(`${BASE_URL}/proposals/${proposalKey}/reject`, {
    method: 'POST',
  });
}
