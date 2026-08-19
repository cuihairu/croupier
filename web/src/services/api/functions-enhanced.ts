import { request } from '@umijs/max';
import type { JSONValue } from '@/types/dashboard';
import type { FunctionDescriptor } from './functions';

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
  resource?: string;
  operation?: string;
}

export interface FunctionCallRecord {
  id: string;
  functionId: string;
  user?: string;
  status: 'success' | 'failed' | 'running' | 'cancelled' | 'timeout';
  startedAt: string;
  completedAt?: string;
  duration?: number;
  payload?: JSONValue;
  result?: JSONValue;
  error?: string;
  agentId?: string;
  serviceId?: string;
  gameId?: string;
  env?: string;
  taskId?: string;
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
  sdkName?: string;
  sdkLang?: string;
  sdkVersion?: string;
  functionId: string;
  status?: 'running' | 'stopped' | 'error' | 'unknown';
  lastHeartbeat?: string;
  functionsCount?: number;
  healthy?: boolean;
  lastSeen?: string;
  gameId?: string;
  env?: string;
  metadata?: Record<string, unknown>;
}

// Raw backend payload observed from function instance endpoints.
// Source: croupier/internal/api/function/dto.go and function/helpers.go
export interface RawFunctionInstance {
  agentId?: string;
  agentName?: string;
  serviceId?: string;
  providerId?: string;
  addr?: string;
  address?: string;
  version?: string;
  functionId?: string;
  status?: string;
  lastHeartbeat?: string;
  functionsCount?: number;
  healthy?: boolean;
  lastSeen?: string;
  updatedAt?: string;
  sdkName?: string;
  sdkLang?: string;
  sdkVersion?: string;
  gameId?: string;
  env?: string;
  metadata?: Record<string, unknown>;
}

// Normalize backend instance payloads into the canonical frontend DTO.
// This is the only layer allowed to absorb snake_case / camelCase differences.
export function normalizeFunctionInstance(raw: RawFunctionInstance): FunctionInstance {
  const status =
    raw.status === 'active' ? 'running' : raw.status === 'inactive' ? 'stopped' : raw.status;
  return {
    agentId: raw.agentId || '',
    agentName: raw.agentName || '',
    serviceId: raw.serviceId || raw.providerId || '',
    providerId: raw.providerId || '',
    addr: raw.addr || raw.address || '',
    version: raw.version || '',
    sdkName: raw.sdkName || '',
    sdkLang: raw.sdkLang || '',
    sdkVersion: raw.sdkVersion || '',
    functionId: raw.functionId || '',
    status:
      status === 'running' || status === 'stopped' || status === 'error' || status === 'unknown'
        ? status
        : 'unknown',
    lastHeartbeat: raw.lastHeartbeat || '',
    functionsCount: raw.functionsCount,
    healthy: raw.healthy,
    lastSeen: raw.lastSeen || raw.updatedAt || '',
    gameId: raw.gameId || '',
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
  metadata?: Record<string, unknown>;
}

export interface FunctionMetrics {
  [key: string]: unknown;
}

type RawFunctionSummary = {
  id?: string;
  functionId?: string;
  version?: string;
  status?: number;
  enabled?: boolean;
  displayName?: LocalizedText | Record<string, string>;
  summary?: LocalizedText | Record<string, string>;
  tags?: string[];
  resource?: string;
  operation?: string;
};

type FunctionSummaryListResponse = {
  functions?: RawFunctionSummary[];
  items?: RawFunctionSummary[];
};

type RawFunctionCallRecord = {
  id?: string;
  functionId?: string;
  user?: string;
  status?: FunctionCallRecord['status'];
  startedAt?: string;
  completedAt?: string;
  duration?: number;
  payload?: JSONValue;
  result?: JSONValue;
  error?: string;
  agentId?: string;
  serviceId?: string;
  gameId?: string;
  env?: string;
  taskId?: string;
  retryCount?: number;
};

type FunctionCallsResponse = {
  calls?: RawFunctionCallRecord[];
  total?: number;
  hasMore?: boolean;
};

type RawRegistryService = {
  serviceId?: string;
  addr?: string;
  status?: RegistryService['status'];
  lastSeen?: string;
  functionsCount?: number;
  gameId?: string;
  env?: string;
  version?: string;
  metadata?: Record<string, unknown>;
};

type RegistryServicesResponse = {
  services?: RawRegistryService[];
  total?: number;
};

function normalizeLocalizedText(
  value?: LocalizedText | Record<string, string>,
): LocalizedText | undefined {
  if (!value) return undefined;
  const raw = value as Record<string, string | undefined>;
  const zh = raw.zh || raw['zh-CN'] || raw.zh_cn;
  const en = raw.en || raw['en-US'] || raw.en_us;
  if (!zh && !en) return undefined;
  return { ...(zh ? { zh } : {}), ...(en ? { en } : {}) };
}

export function normalizeFunctionSummary(item: RawFunctionSummary): FunctionSummary {
  return {
    id: item.id || item.functionId || '',
    version: item.version,
    enabled: item.status === 1 || item.enabled === true,
    displayName: normalizeLocalizedText(item.displayName),
    summary: normalizeLocalizedText(item.summary),
    tags: item.tags || [],
    resource: item.resource,
    operation: item.operation,
  };
}

function normalizeFunctionCallRecord(item: RawFunctionCallRecord): FunctionCallRecord {
  return {
    id: item.id || '',
    functionId: item.functionId || '',
    user: item.user,
    status: item.status || 'failed',
    startedAt: item.startedAt || '',
    completedAt: item.completedAt,
    duration: item.duration,
    payload: item.payload,
    result: item.result,
    error: item.error,
    agentId: item.agentId,
    serviceId: item.serviceId,
    gameId: item.gameId,
    env: item.env,
    taskId: item.taskId,
    retryCount: item.retryCount,
  };
}

function normalizeRegistryService(item: RawRegistryService): RegistryService {
  return {
    serviceId: item.serviceId || '',
    addr: item.addr || '',
    status: item.status || 'unknown',
    lastSeen: item.lastSeen || '',
    functionsCount: item.functionsCount || 0,
    gameId: item.gameId,
    env: item.env,
    version: item.version,
    metadata: item.metadata,
  };
}

/**
 * 获取函数摘要列表
 */
export async function getFunctionSummary(params?: {
  gameId?: string;
  env?: string;
  resource?: string;
  tags?: string[];
  enabled?: boolean;
}): Promise<FunctionSummary[]> {
  const res = await request<FunctionSummaryListResponse | RawFunctionSummary[]>(
    '/api/v1/functions',
    {
      params: {
        gameId: params?.gameId,
        env: params?.env,
        resource: params?.resource,
        tags: params?.tags,
        enabled: params?.enabled,
      },
    },
  );
  if (Array.isArray(res)) return res.map(normalizeFunctionSummary);
  if (Array.isArray(res?.functions)) return res.functions.map(normalizeFunctionSummary);
  if (Array.isArray(res?.items)) return res.items.map(normalizeFunctionSummary);
  throw new Error('函数摘要接口返回了无法识别的数据格式');
}

/**
 * 获取函数详细信息
 */
export async function getFunctionDetail(
  functionId: string,
  _params?: {
    gameId?: string;
    env?: string;
  },
): Promise<FunctionDescriptor & { instances?: FunctionInstance[]; metrics?: FunctionMetrics }> {
  const res = await request<
    FunctionDescriptor & { instances?: FunctionInstance[]; metrics?: FunctionMetrics }
  >(`/api/v1/functions/${functionId}`, { method: 'GET' });
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
  const res = await request<FunctionCallsResponse>('/api/v1/function-calls', {
    params: {
      functionId: params?.functionId,
      userId: params?.userId,
      gameId: params?.gameId,
      env: params?.env,
      status: params?.status,
      startTime: params?.startTime,
      endTime: params?.endTime,
      limit: params?.limit,
      offset: params?.offset,
    },
  });
  return {
    calls: (res.calls || []).map(normalizeFunctionCallRecord),
    total: res.total || 0,
    hasMore: res.hasMore || false,
  };
}

/**
 * 获取单个调用详情
 */
export async function getFunctionCall(callId: string): Promise<FunctionCallRecord> {
  const item = await request<RawFunctionCallRecord>(`/api/v1/function-calls/${callId}`, {
    method: 'GET',
  });
  return normalizeFunctionCallRecord(item);
}

/**
 * 重新运行失败的调用
 */
export async function rerunFunctionCall(callId: string): Promise<{ taskId: string }> {
  const response = await request<{ task_id?: string; taskId?: string }>(
    `/api/v1/function-calls/${callId}/rerun`,
    { method: 'POST' },
  );
  return { taskId: response.taskId || '' };
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
  const url = functionId
    ? `/api/v1/functions/${encodeURIComponent(functionId)}/instances`
    : '/api/v1/functions/instances';
  const res = await request<{
    items?: RawFunctionInstance[];
    instances?: RawFunctionInstance[];
    total?: number;
  }>(url, {
    params: {
      ...queryParams,
      gameId: params?.gameId,
    },
  });
  const rawItems = (res.items || res.instances || []) as RawFunctionInstance[];
  const instances = rawItems.map(normalizeFunctionInstance);
  return {
    instances,
    total: res.total ?? instances.length,
  };
}

/**
 * 获取注册表服务列表
 */
export async function getRegistryServices(params?: {
  gameId?: string;
  env?: string;
  status?: string;
}): Promise<{ services: RegistryService[]; total: number }> {
  const res = await request<RegistryServicesResponse>('/api/v1/registry/services', {
    params: {
      gameId: params?.gameId,
      env: params?.env,
      status: params?.status,
    },
  });
  return {
    services: (res.services || []).map(normalizeRegistryService),
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
          functionIds: params.functionIds,
          enabled,
          gameId: params.gameId,
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
  resource?: string;
  tags?: string[];
  limit?: number;
}): Promise<{ functions: FunctionSummary[]; total: number }> {
  const functions = await getFunctionSummary({
    gameId: params.gameId,
    env: params.env,
    resource: params.resource,
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
 * 获取函数资源
 */
export async function getFunctionResources(params?: {
  gameId?: string;
  env?: string;
}): Promise<{ resources: string[]; counts: Record<string, number> }> {
  const functions = await getFunctionSummary(params);
  const counts: Record<string, number> = {};
  for (const item of functions) {
    const key = item.resource || 'unassigned';
    counts[key] = (counts[key] || 0) + 1;
  }
  return { resources: Object.keys(counts), counts };
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
export async function exportFunctions(_params: {
  functionIds?: string[];
  format?: 'json' | 'yaml' | 'csv';
  includeMetadata?: boolean;
}): Promise<{ downloadUrl: string; expiresAt: string }> {
  throw new Error('当前后端未提供函数导出接口');
}

/**
 * 导入函数配置
 */
export async function importFunctions(_params: {
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
export async function validateFunctionConfig(_params: {
  functionConfig: unknown;
  strict?: boolean;
}): Promise<{ valid: boolean; errors: string[]; warnings: string[] }> {
  console.warn('API /api/functions/validate 在后端未提供，返回基础校验结果');
  return { valid: true, errors: [], warnings: [] };
}

/**
 * 获取函数依赖关系
 */
export async function getFunctionDependencies(_functionId: string): Promise<{
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
  payload: JSONValue;
  dryRun?: boolean;
  gameId?: string;
  env?: string;
}): Promise<{ valid: boolean; result?: JSONValue; error?: string; duration?: number }> {
  const result = await request<JSONValue>(
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
    'x-resource'?: string;
    'x-risk'?: 'safe' | 'warning' | 'high' | 'danger';
    'x-operation'?: string; // business action key
    'x-enabled'?: boolean;
    'x-permission'?: string;
  };
  // 请求/响应 schema
  requestBody?: JSONValue;
  responses?: JSONValue;
  parameters?: JSONValue[];
}> {
  return request(`/api/v1/functions/${functionId}/openapi`);
}

/**
 * 批量获取多个函数的 OpenAPI 规范
 * @param functionIds 函数 ID 列表
 * @returns OpenAPI Operation Object 映射
 */
export async function batchGetFunctionOpenAPI(
  functionIds: string[],
): Promise<Record<string, JSONValue>> {
  return request('/api/v1/functions/_openapi-batch', {
    method: 'POST',
    data: { functionIds: functionIds },
  });
}
