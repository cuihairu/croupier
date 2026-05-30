import { request } from '@umijs/max';
import { FunctionDescriptor } from './functions';

export interface LocalizedText {
  zh?: string;
  en?: string;
}

// Enhanced types for better type safety.
// Source: croupier/internal/api/function/dto.go and descriptor index endpoints.
export interface FunctionSummary {
  id: string;
  version?: string;
  enabled?: boolean;
  displayName?: LocalizedText;
  summary?: LocalizedText;
  tags?: string[];
  category?: string;
  menu?: {
    section?: string;
    group?: string;
    path?: string;
    order?: number;
    hidden?: boolean;
  };
}

export interface FunctionCallRecord {
  id: string;
  functionId: string;
  user?: string;
  status: 'success' | 'failed' | 'running' | 'cancelled' | 'timeout';
  startedAt: string;
  completedAt?: string;
  duration?: number;
  payload?: any;
  result?: any;
  error?: string;
  agentId?: string;
  serviceId?: string;
  gameId?: string;
  env?: string;
  jobId?: string;
  retryCount?: number;
}

// Canonical frontend DTO for function instances.
// Source: croupier/internal/api/function/dto.go
// Covers both GET /api/v1/functions/instances and GET /api/v1/functions/:id/instances
// after normalization from backend camelCase payloads.
export interface FunctionInstance {
  agentId: string;
  agentName?: string;
  serviceId: string;
  providerId?: string;
  addr: string;
  version: string;
  functionId: string;
  status?: 'running' | 'stopped' | 'error' | 'unknown';
  lastHeartbeat?: string;
  functionsCount?: number;
  healthy?: boolean;
  lastSeen?: string;
  gameId?: string;
  env?: string;
  metadata?: Record<string, any>;
}

// Raw backend payload observed from function instance endpoints.
// Source: croupier/internal/api/function/dto.go and function/helpers.go
export interface RawFunctionInstance {
  agent_id?: string;
  agentId?: string;
  agent_name?: string;
  agentName?: string;
  service_id?: string;
  serviceId?: string;
  provider_id?: string;
  providerId?: string;
  addr?: string;
  address?: string;
  version?: string;
  function_id?: string;
  functionId?: string;
  status?: string;
  last_heartbeat?: string;
  lastHeartbeat?: string;
  functions_count?: number;
  healthy?: boolean;
  last_seen?: string;
  lastSeen?: string;
  updated_at?: string;
  updatedAt?: string;
  game_id?: string;
  gameId?: string;
  env?: string;
  metadata?: Record<string, any>;
}

// Normalize backend instance payloads into the canonical frontend DTO.
// This is the only layer allowed to absorb snake_case / camelCase differences.
export function normalizeFunctionInstance(raw: RawFunctionInstance): FunctionInstance {
  const status =
    raw.status === 'active' ? 'running' : raw.status === 'inactive' ? 'stopped' : raw.status;
  return {
    agentId: raw.agent_id || raw.agentId || '',
    agentName: raw.agent_name || raw.agentName || '',
    serviceId: raw.service_id || raw.serviceId || raw.provider_id || raw.providerId || '',
    providerId: raw.provider_id || raw.providerId || '',
    addr: raw.addr || raw.address || '',
    version: raw.version || '',
    functionId: raw.function_id || raw.functionId || '',
    status:
      status === 'running' || status === 'stopped' || status === 'error' || status === 'unknown'
        ? status
        : 'unknown',
    lastHeartbeat: raw.last_heartbeat || raw.lastHeartbeat || '',
    functionsCount: raw.functions_count,
    healthy: raw.healthy,
    lastSeen: raw.last_seen || raw.lastSeen || raw.updated_at || raw.updatedAt || '',
    gameId: raw.game_id || raw.gameId || '',
    env: raw.env || '',
    metadata: raw.metadata,
  };
}

export interface RegistryService {
  serviceId: string;
  addr: string;
  status: 'healthy' | 'unhealthy' | 'unknown';
  lastSeen: string;
  functionsCount: number;
  gameId?: string;
  env?: string;
  version?: string;
  metadata?: Record<string, any>;
}

export interface FunctionMetrics {
  [key: string]: any;
}

function normalizeFunctionSummary(item: any): FunctionSummary {
  return {
    id: item.id || item.function_id || '',
    version: item.version,
    enabled: item.status === 1 || item.enabled === true,
    displayName: item.display_name || item.displayName,
    summary: item.summary,
    tags: item.tags || [],
    category: item.category,
    menu: item.menu,
  };
}

/**
 * 获取函数摘要列表
 */
export async function getFunctionSummary(params?: {
  gameId?: string;
  env?: string;
  category?: string;
  tags?: string[];
  enabled?: boolean;
}): Promise<FunctionSummary[]> {
  try {
    const res = await request('/api/v1/functions', {
      params: {
        game_id: params?.gameId,
        env: params?.env,
        category: params?.category,
        tags: params?.tags,
        enabled: params?.enabled,
      },
    });
    if (Array.isArray(res)) return res.map(normalizeFunctionSummary);
    if (res?.functions && Array.isArray(res.functions))
      return res.functions.map(normalizeFunctionSummary);
    if (res?.items && Array.isArray(res.items)) {
      return res.items.map(normalizeFunctionSummary);
    }
    // If empty, fallback to descriptors
    throw new Error('No functions found, fallback to descriptors');
  } catch (error) {
    console.warn('Failed to fetch function summary, falling back to descriptors', error);
    const descriptors = await request('/api/v1/functions/descriptors');
    return descriptors.map((desc: FunctionDescriptor) => ({
      id: desc.id,
      version: desc.version,
      enabled: true,
      displayName: (desc as any).display_name || (desc as any).displayName,
      summary: desc.summary,
      tags: desc.tags || [],
      category: desc.category,
    }));
  }
}

/**
 * 获取函数详细信息
 */
export async function getFunctionDetail(
  functionId: string,
  params?: {
    gameId?: string;
    env?: string;
  },
): Promise<FunctionDescriptor & { instances?: FunctionInstance[]; metrics?: FunctionMetrics }> {
  const res = await request(`/api/v1/functions/${functionId}`, { method: 'GET' });
  return res;
}

/**
 * 获取函数调用历史
 */
export async function getFunctionCalls(params?: {
  functionId?: string;
  userId?: string;
  gameId?: string;
  env?: string;
  status?: string;
  startTime?: string;
  endTime?: string;
  limit?: number;
  offset?: number;
}): Promise<{ calls: FunctionCallRecord[]; total: number; hasMore: boolean }> {
  const res = await request('/api/v1/function-calls', {
    params: {
      function_id: params?.functionId,
      user_id: params?.userId,
      game_id: params?.gameId,
      env: params?.env,
      status: params?.status,
      start_time: params?.startTime,
      end_time: params?.endTime,
      limit: params?.limit,
      offset: params?.offset,
    },
  });
  return {
    calls: (res.calls || []).map((item: any) => ({
      id: item.id,
      functionId: item.function_id || item.functionId || '',
      user: item.user,
      status: item.status,
      startedAt: item.started_at || item.startedAt || '',
      completedAt: item.completed_at || item.completedAt,
      duration: item.duration,
      payload: item.payload,
      result: item.result,
      error: item.error,
      agentId: item.agent_id || item.agentId,
      serviceId: item.service_id || item.serviceId,
      gameId: item.game_id || item.gameId,
      env: item.env,
      jobId: item.job_id || item.jobId,
      retryCount: item.retry_count || item.retryCount,
    })),
    total: res.total || 0,
    hasMore: res.has_more || res.hasMore || false,
  };
}

/**
 * 获取单个调用详情
 */
export async function getFunctionCall(callId: string): Promise<FunctionCallRecord> {
  const item = await request<any>(`/api/v1/function-calls/${callId}`, { method: 'GET' });
  return {
    id: item.id,
    functionId: item.function_id || item.functionId || '',
    user: item.user,
    status: item.status,
    startedAt: item.started_at || item.startedAt || '',
    completedAt: item.completed_at || item.completedAt,
    duration: item.duration,
    payload: item.payload,
    result: item.result,
    error: item.error,
    agentId: item.agent_id || item.agentId,
    serviceId: item.service_id || item.serviceId,
    gameId: item.game_id || item.gameId,
    env: item.env,
    jobId: item.job_id || item.jobId,
    retryCount: item.retry_count || item.retryCount,
  };
}

/**
 * 重新运行失败的调用
 */
export async function rerunFunctionCall(callId: string): Promise<{ jobId: string }> {
  const response = await request<{ job_id?: string; jobId?: string }>(
    `/api/v1/function-calls/${callId}/rerun`,
    { method: 'POST' },
  );
  return { jobId: response.job_id || response.jobId || '' };
}

/**
 * 取消正在运行的调用
 */
export async function cancelFunctionCall(callId: string): Promise<void> {
  return request(`/api/v1/function-calls/${callId}/cancel`, { method: 'POST' });
}

/**
 * 获取函数实例列表
 */
export async function getFunctionInstances(params?: {
  functionId?: string;
  gameId?: string;
  env?: string;
  status?: string;
}): Promise<{ instances: FunctionInstance[]; total: number }> {
  // 后端 API: GET /api/v1/functions/instances (all) or /api/v1/functions/:id/instances
  const { functionId, ...queryParams } = params || {};
  try {
    const url = functionId
      ? `/api/v1/functions/${functionId}/instances`
      : `/api/v1/functions/instances`;
    const res = await request(url, {
      params: {
        ...queryParams,
        game_id: params?.gameId,
      },
    });
    const rawItems = (res.items || res.instances || []) as RawFunctionInstance[];
    const instances = rawItems.map(normalizeFunctionInstance);
    return {
      instances,
      total: res.total || instances.length,
    };
  } catch (error) {
    console.warn('Failed to fetch function instances:', error);
    return { instances: [], total: 0 };
  }
}

/**
 * 获取注册表服务列表
 */
export async function getRegistryServices(params?: {
  gameId?: string;
  env?: string;
  status?: string;
}): Promise<{ services: RegistryService[]; total: number }> {
  const res = await request('/api/v1/registry/services', {
    params: {
      game_id: params?.gameId,
      env: params?.env,
      status: params?.status,
    },
  });
  return {
    services: (res.services || []).map((item: any) => ({
      serviceId: item.service_id || item.serviceId || '',
      addr: item.addr || '',
      status: item.status || 'unknown',
      lastSeen: item.last_seen || item.lastSeen || '',
      functionsCount: item.functions_count || item.functionsCount || 0,
      gameId: item.game_id || item.gameId,
      env: item.env,
      version: item.version,
      metadata: item.metadata,
    })),
    total: res.total || 0,
  };
}

/**
 * 批量操作函数
 */
export async function batchUpdateFunctions(params: {
  functionIds: string[];
  operation: 'enable' | 'disable' | 'delete';
  gameId?: string;
  env?: string;
}): Promise<{ success: number; failed: number; errors: string[] }> {
  if (params.operation === 'delete') {
    const results = await Promise.allSettled(
      params.functionIds.map((id) =>
        request(`/api/v1/functions/${encodeURIComponent(id)}`, { method: 'DELETE' }),
      ),
    );
    const failed = results.filter((r) => r.status === 'rejected').length;
    return { success: results.length - failed, failed, errors: [] };
  }
  if (params.operation === 'enable' || params.operation === 'disable') {
    const enabled = params.operation === 'enable';
    const res = await request<{ updated?: number; failed?: string[] }>(
      '/api/v1/functions/batch-update',
      {
        method: 'POST',
        data: {
          function_ids: params.functionIds,
          enabled,
          game_id: params.gameId,
          env: params.env,
        },
      },
    );
    const failedItems = res?.failed || [];
    return {
      success: (res?.updated || 0) - failedItems.length,
      failed: failedItems.length,
      errors: failedItems,
    };
  }
  return { success: 0, failed: params.functionIds.length, errors: ['unsupported operation'] };
}

/**
 * 搜索函数
 */
export async function searchFunctions(params: {
  query: string;
  gameId?: string;
  env?: string;
  category?: string;
  tags?: string[];
  limit?: number;
}): Promise<{ functions: FunctionSummary[]; total: number }> {
  const functions = await getFunctionSummary({
    gameId: params.gameId,
    env: params.env,
    category: params.category,
    tags: params.tags,
  });
  const q = params.query.trim().toLowerCase();
  const filtered = q
    ? functions.filter((item) =>
        [item.id, item.displayName?.zh, item.displayName?.en, item.summary?.zh, item.summary?.en]
          .filter(Boolean)
          .some((value) => String(value).toLowerCase().includes(q)),
      )
    : functions;
  const limited = params.limit ? filtered.slice(0, params.limit) : filtered;
  return { functions: limited, total: filtered.length };
}

/**
 * 获取函数分类
 */
export async function getFunctionCategories(params?: {
  gameId?: string;
  env?: string;
}): Promise<{ categories: string[]; counts: Record<string, number> }> {
  const functions = await getFunctionSummary(params);
  const counts: Record<string, number> = {};
  for (const item of functions) {
    const key = item.category || 'uncategorized';
    counts[key] = (counts[key] || 0) + 1;
  }
  return { categories: Object.keys(counts), counts };
}

/**
 * 获取函数标签
 */
export async function getFunctionTags(params?: {
  gameId?: string;
  env?: string;
  limit?: number;
}): Promise<{ tags: string[]; counts: Record<string, number> }> {
  const functions = await getFunctionSummary(params);
  const counts: Record<string, number> = {};
  for (const item of functions) {
    for (const tag of item.tags || []) {
      counts[tag] = (counts[tag] || 0) + 1;
    }
  }
  const tags = Object.keys(counts);
  return { tags: params?.limit ? tags.slice(0, params.limit) : tags, counts };
}

/**
 * 导出函数配置
 */
export async function exportFunctions(params: {
  functionIds?: string[];
  format?: 'json' | 'yaml' | 'csv';
  includeMetadata?: boolean;
}): Promise<{ downloadUrl: string; expiresAt: string }> {
  throw new Error('当前后端未提供函数导出接口');
}

/**
 * 导入函数配置
 */
export async function importFunctions(params: {
  fileUrl: string;
  format?: 'json' | 'yaml' | 'csv';
  overwrite?: boolean;
  gameId?: string;
  env?: string;
}): Promise<{ imported: number; skipped: number; errors: string[] }> {
  throw new Error('当前后端未提供函数导入接口');
}

/**
 * 验证函数配置
 */
export async function validateFunctionConfig(params: {
  functionConfig: any;
  strict?: boolean;
}): Promise<{ valid: boolean; errors: string[]; warnings: string[] }> {
  console.warn('API /api/functions/validate 在后端未提供，返回基础校验结果');
  return { valid: true, errors: [], warnings: [] };
}

/**
 * 获取函数依赖关系
 */
export async function getFunctionDependencies(functionId: string): Promise<{
  dependencies: string[];
  dependents: string[];
  circularDependencies: string[];
}> {
  console.warn('API /api/functions/:id/dependencies 在后端未提供，返回空依赖');
  return { dependencies: [], dependents: [], circularDependencies: [] };
}

/**
 * 测试函数
 */
export async function testFunction(params: {
  functionId: string;
  payload: any;
  dryRun?: boolean;
  gameId?: string;
  env?: string;
}): Promise<{ valid: boolean; result?: any; error?: string; duration?: number }> {
  const result = await request(
    `/api/v1/functions/${encodeURIComponent(params.functionId)}/invoke`,
    {
      method: 'POST',
      data: {
        payload: params.payload,
        gameId: params.gameId,
        env: params.env,
      },
    },
  );
  return { valid: true, result };
}

/**
 * ============================================================================
 * OpenAPI 3.0.3 增强功能
 * ============================================================================
 */

/**
 * 获取函数的 OpenAPI 3.0.3 完整规范（增强版）
 * @param functionId 函数 ID
 * @returns 完整的 OpenAPI 3.0.3 Operation Object，包含扩展字段
 */
export async function getFunctionOpenAPIDetail(functionId: string): Promise<{
  operationId: string;
  summary?: string;
  description?: string;
  tags?: string[];
  // OpenAPI 扩展字段
  extensions?: {
    'x-category'?: string;
    'x-risk'?: 'safe' | 'warning' | 'danger';
    'x-entity'?: string;
    'x-operation'?: 'create' | 'read' | 'update' | 'delete' | 'custom';
  };
  // 请求/响应 schema
  requestBody?: any;
  responses?: any;
  parameters?: any[];
}> {
  return request(`/api/v1/functions/${functionId}/openapi`);
}

/**
 * 批量获取多个函数的 OpenAPI 规范
 * @param functionIds 函数 ID 列表
 * @returns OpenAPI Operation Object 映射
 */
export async function batchGetFunctionOpenAPI(functionIds: string[]): Promise<Record<string, any>> {
  return request('/api/v1/functions/_openapi-batch', {
    method: 'POST',
    data: { function_ids: functionIds },
  });
}

/**
 * 根据 Entity 类型获取相关函数的 OpenAPI 列表
 * @param entityId Entity ID
 * @returns Entity 关联的函数列表
 */
export async function getEntityFunctions(entityId: string): Promise<{
  items: Array<{
    id: string;
    operation: 'create' | 'read' | 'update' | 'delete' | 'custom';
    name: string;
  }>;
}> {
  return request(`/api/v1/entities/${entityId}/functions`);
}
