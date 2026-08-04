/**
 * Versioning API 服务
 *
 * 提供页面版本管理和冲突解决功能
 */

import { request } from '@umijs/max';
import type { JSONValue } from '@/types/dashboard';

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

/** 变更类型 */
export type ChangeType =
  | 'function_update'
  | 'semantic_update'
  | 'proposal_update'
  | 'draft_update'
  | 'publish';

/** 变更项 */
export interface ChangeItem {
  type: ChangeType;
  timestamp: string;
  version?: number;
  summary: string;
  details?: JSONValue;
  actor?: string;
}

/** 当前状态 */
export interface CurrentState {
  functionVersion?: string;
  semanticVersion?: number;
  proposalVersion?: number;
  draftRevision?: number;
  publishedVersion?: number;
}

/** 变更链 */
export interface ChangeChain {
  resourceKey: string;
  items: ChangeItem[];
  current: CurrentState;
}

/** 变更详情 */
export interface DiffChange {
  path: string;
  oldValue: JSONValue;
  newValue: JSONValue;
  changeType: 'added' | 'removed' | 'modified';
  isSemantic: boolean;
}

/** Diff 响应 */
export interface DiffResponse {
  changes: DiffChange[];
  summary: string;
}

/** 合并策略 */
export type MergeStrategy = 'auto' | 'accept' | 'reject' | 'manual';

/** 冲突解决方案 */
export interface ConflictResolution {
  path: string;
  acceptNew: boolean;
  value?: JSONValue;
}

/** 合并请求 */
export interface MergeRequest {
  strategy: MergeStrategy;
  conflicts?: ConflictResolution[];
  reason?: string;
}

/** 合并响应 */
export interface MergeResponse {
  merged: number;
  conflicts: number;
  message: string;
}

/** 回滚请求 */
export interface RollbackRequest {
  version?: number;
  reason?: string;
}

/** 回滚响应 */
export interface RollbackResponse {
  message: string;
}

// ---------------------------------------------------------------------------
// API Functions
// ---------------------------------------------------------------------------

/**
 * 获取变更链
 */
export async function getChangeChain(resourceKey: string): Promise<ChangeChain> {
  return request<ChangeChain>(`/api/v1/versioning/${resourceKey}/chain`, {
    method: 'GET',
  });
}

/**
 * 获取 Diff
 */
export async function getDiff(
  resourceKey: string,
  params?: { baseVersion?: number; targetVersion?: number }
): Promise<DiffResponse> {
  return request<DiffResponse>(`/api/v1/versioning/${resourceKey}/diff`, {
    method: 'GET',
    params,
  });
}

/**
 * 合并变更
 */
export async function mergeChanges(
  resourceKey: string,
  data: MergeRequest
): Promise<MergeResponse> {
  return request<MergeResponse>(`/api/v1/versioning/${resourceKey}/merge`, {
    method: 'POST',
    data,
  });
}

/**
 * 回滚草稿
 */
export async function rollbackDraft(
  resourceKey: string,
  data?: RollbackRequest
): Promise<RollbackResponse> {
  return request<RollbackResponse>(`/api/v1/versioning/${resourceKey}/rollback-draft`, {
    method: 'POST',
    data,
  });
}

/**
 * 回滚发布
 */
export async function rollbackPublish(
  resourceKey: string,
  data?: RollbackRequest
): Promise<RollbackResponse> {
  return request<RollbackResponse>(`/api/v1/versioning/${resourceKey}/rollback-publish`, {
    method: 'POST',
    data,
  });
}

/**
 * 重新生成 Proposal
 */
export async function regenerateProposal(resourceKey: string): Promise<{ message: string }> {
  return request<{ message: string }>(`/api/v1/versioning/${resourceKey}/regenerate`, {
    method: 'POST',
  });
}

/**
 * 重新发布
 */
export async function republish(resourceKey: string): Promise<{ message: string }> {
  return request<{ message: string }>(`/api/v1/versioning/${resourceKey}/republish`, {
    method: 'POST',
  });
}
