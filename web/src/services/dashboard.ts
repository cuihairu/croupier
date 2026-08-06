/**
 * Dashboard API 服务。
 *
 * 提供 Resource Catalog、Versioning、Contract、Proposal 等 API 调用。
 */

import { request } from '@umijs/max';
import type {
  ResourceCatalogItem,
  PageProposal,
  ProposalInbox,
  ChangeChain,
  MergeRequest,
  MergeResponse,
  JSONValue,
  ResourceSemanticConflicts,
  ResourceSemanticVersions,
  ResolveSemanticConflictRequest,
  UpdateResourceSemanticsRequest,
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
  data: UpdateResourceSemanticsRequest,
): Promise<{ version: number; source: string; message: string }> {
  return request(`${BASE_URL}/resource-catalog/${resourceKey}/semantics`, {
    method: 'PUT',
    data,
  });
}

/** 获取资源语义冲突和来源 */
export async function getResourceSemanticConflicts(
  resourceKey: string,
): Promise<ResourceSemanticConflicts> {
  return request(`${BASE_URL}/resource-catalog/${resourceKey}/conflicts`, {
    method: 'GET',
  });
}

/** 获取资源语义版本历史 */
export async function getResourceSemanticVersions(
  resourceKey: string,
): Promise<ResourceSemanticVersions> {
  return request(`${BASE_URL}/resource-catalog/${resourceKey}/semantics/versions`, {
    method: 'GET',
  });
}

/** 解决资源语义冲突 */
export async function resolveResourceSemanticConflict(
  resourceKey: string,
  field: string,
  data: ResolveSemanticConflictRequest,
): Promise<{ message: string }> {
  return request(`${BASE_URL}/resource-catalog/${resourceKey}/conflicts/${field}/resolve`, {
    method: 'POST',
    data,
  });
}

/** 获取页面变更链 */
export async function getChangeChain(pageKey: string): Promise<ChangeChain> {
  return request(`${BASE_URL}/versioning/pages/${pageKey}/chain`, {
    method: 'GET',
  });
}

/** 页面版本对比 */
export async function getVersionDiff(
  pageKey: string,
  params: { fromVersion: number; toVersion: number },
): Promise<{ changes: JSONValue[]; summary: string }> {
  return request(`${BASE_URL}/versioning/pages/${pageKey}/diff`, {
    method: 'GET',
    params,
  });
}

/** 合并页面变更 */
export async function mergeChanges(pageKey: string, data: MergeRequest): Promise<MergeResponse> {
  return request(`${BASE_URL}/versioning/pages/${pageKey}/merge`, {
    method: 'POST',
    data,
  });
}

/** 回滚草稿 */
export async function rollbackDraft(
  pageKey: string,
  data: { version: number; reason?: string },
): Promise<{ message: string }> {
  return request(`${BASE_URL}/versioning/pages/${pageKey}/rollback-draft`, {
    method: 'POST',
    data,
  });
}

/** 回滚发布 */
export async function rollbackPublish(
  pageKey: string,
  data: { version: number; reason?: string },
): Promise<{ message: string }> {
  return request(`${BASE_URL}/versioning/pages/${pageKey}/rollback-publish`, {
    method: 'POST',
    data,
  });
}

/** 重新生成提案 */
export async function regenerateProposal(
  pageKey: string,
  data?: { force?: boolean },
): Promise<{ message: string }> {
  return request(`${BASE_URL}/versioning/pages/${pageKey}/regenerate`, {
    method: 'POST',
    data,
  });
}

/** 重新发布 */
export async function republish(
  pageKey: string,
  data?: { reason?: string },
): Promise<{ version: number; message: string }> {
  return request(`${BASE_URL}/versioning/pages/${pageKey}/republish`, {
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
export async function listProposals(params?: {
  status?: string;
  resourceKey?: string;
}): Promise<PageProposal[]> {
  return request(`${BASE_URL}/proposals`, {
    method: 'GET',
    params,
  });
}

/** 获取 Page Studio 三队列入口 */
export async function listProposalInbox(params?: {
  resourceKey?: string;
}): Promise<ProposalInbox> {
  return request(`${BASE_URL}/proposals/inbox`, {
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

/** 接受并直接发布提案 */
export async function acceptAndPublishProposal(
  proposalKey: string,
): Promise<{ pageKey: string; draftRevision: number; publishedVersion: number }> {
  return request(`${BASE_URL}/proposals/${proposalKey}/accept-and-publish`, {
    method: 'POST',
  });
}

/** 拒绝提案 */
export async function rejectProposal(proposalKey: string): Promise<{ message: string }> {
  return request(`${BASE_URL}/proposals/${proposalKey}/reject`, {
    method: 'POST',
  });
}
