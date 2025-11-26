import { request } from '@umijs/max';
import { FunctionDescriptor } from './functions';

// Enhanced types for better type safety
export interface FunctionSummary {
  id: string;
  version?: string;
  enabled?: boolean;
  display_name?: { zh?: string; en?: string };
  summary?: { zh?: string; en?: string };
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
  function_id: string;
  user?: string;
  status: 'success' | 'failed' | 'running' | 'cancelled' | 'timeout';
  started_at: string;
  completed_at?: string;
  duration?: number;
  payload?: any;
  result?: any;
  error?: string;
  agent_id?: string;
  service_id?: string;
  game_id?: string;
  env?: string;
  job_id?: string;
  retry_count?: number;
}

export interface FunctionInstance {
  agent_id: string;
  service_id: string;
  addr: string;
  version: string;
  function_id: string;
  status?: 'running' | 'stopped' | 'error' | 'unknown';
  last_heartbeat?: string;
  functions_count?: number;
  game_id?: string;
  env?: string;
  metadata?: Record<string, any>;
}

export interface RegistryService {
  service_id: string;
  addr: string;
  status: 'healthy' | 'unhealthy' | 'unknown';
  last_seen: string;
  functions_count: number;
  game_id?: string;
  env?: string;
  version?: string;
  metadata?: Record<string, any>;
}

export interface CoverageAnalysis {
  total_functions: number;
  covered_functions: number;
  coverage_percentage: number;
  total_instances: number;
  active_instances: number;
  inactive_instances: number;
  functions_by_category: Record<string, number>;
  instances_by_game: Record<string, number>;
}

export interface FunctionMetrics {
  function_id: string;
  total_calls: number;
  success_calls: number;
  failed_calls: number;
  average_duration: number;
  last_called_at?: string;
  most_active_user?: string;
  error_rate: number;
}

/**
 * 获取函数摘要列表
 */
export async function getFunctionSummary(params?: {
  game_id?: string;
  env?: string;
  category?: string;
  tags?: string[];
  enabled?: boolean;
}): Promise<FunctionSummary[]> {
  try {
    const res = await request('/api/v1/functions', { params });
    if (Array.isArray(res)) return res;
    if (res?.functions && Array.isArray(res.functions)) return res.functions;
    return [];
  } catch (error) {
    console.warn('Failed to fetch function summary, falling back to descriptors');
    const descriptors = await request('/api/v1/functions/descriptors');
    return descriptors.map((desc: FunctionDescriptor) => ({
      id: desc.id,
      version: desc.version,
      enabled: true,
      display_name: desc.display_name,
      summary: desc.summary,
      tags: desc.tags || [],
      category: desc.category
    }));
  }
}

/**
 * 获取函数详细信息
 */
export async function getFunctionDetail(functionId: string, params?: {
  game_id?: string;
  env?: string;
}): Promise<FunctionDescriptor & { instances?: FunctionInstance[]; metrics?: FunctionMetrics }> {
  const res = await request(`/api/v1/functions/${functionId}`, { method: 'GET' });
  return res;
}

/**
 * 获取函数调用历史
 */
export async function getFunctionCalls(params?: {
  function_id?: string;
  user_id?: string;
  game_id?: string;
  env?: string;
  status?: string;
  start_time?: string;
  end_time?: string;
  limit?: number;
  offset?: number;
}): Promise<{ calls: FunctionCallRecord[]; total: number; has_more: boolean }> {
  const res = await request('/api/v1/function-calls', { params });
  return {
    calls: res.calls || [],
    total: res.total || 0,
    has_more: res.has_more || false
  };
}

/**
 * 获取单个调用详情
 */
export async function getFunctionCall(callId: string): Promise<FunctionCallRecord> {
  return request(`/api/v1/function-calls/${callId}`, { method: 'GET' });
}

/**
 * 重新运行失败的调用
 */
export async function rerunFunctionCall(callId: string): Promise<{ job_id: string }> {
  return request(`/api/v1/function-calls/${callId}/rerun`, { method: 'POST' });
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
  function_id?: string;
  game_id?: string;
  env?: string;
  status?: string;
}): Promise<{ instances: FunctionInstance[]; total: number }> {
  const res = await request('/api/v1/function-instances', { params });
  return {
    instances: res.instances || [],
    total: res.total || 0
  };
}

/**
 * 获取覆盖率分析
 */
export async function getCoverageAnalysis(params?: {
  game_id?: string;
  env?: string;
}): Promise<CoverageAnalysis> {
  return request('/api/v1/analytics/coverage', { params });
}

/**
 * 获取注册表服务列表
 */
export async function getRegistryServices(params?: {
  game_id?: string;
  env?: string;
  status?: string;
}): Promise<{ services: RegistryService[]; total: number }> {
  const res = await request('/api/v1/registry/services', { params });
  return {
    services: res.services || [],
    total: res.total || 0
  };
}

/**
 * 获取函数指标
 */
export async function getFunctionMetrics(params?: {
  function_id?: string;
  game_id?: string;
  env?: string;
  time_range?: '1h' | '24h' | '7d' | '30d';
}): Promise<{ metrics: FunctionMetrics[]; summary: any }> {
  const res = await request('/api/functions/metrics', { params });
  return {
    metrics: res.metrics || [],
    summary: res.summary
  };
}

/**
 * 批量操作函数
 */
export async function batchUpdateFunctions(params: {
  function_ids: string[];
  operation: 'enable' | 'disable' | 'delete';
  game_id?: string;
  env?: string;
}): Promise<{ success: number; failed: number; errors: string[] }> {
  return request('/api/functions/batch', {
    method: 'POST',
    data: params
  });
}

/**
 * 搜索函数
 */
export async function searchFunctions(params: {
  query: string;
  game_id?: string;
  env?: string;
  category?: string;
  tags?: string[];
  limit?: number;
}): Promise<{ functions: FunctionSummary[]; total: number }> {
  const res = await request('/api/functions/search', { params });
  return {
    functions: res.functions || [],
    total: res.total || 0
  };
}

/**
 * 获取函数分类
 */
export async function getFunctionCategories(params?: {
  game_id?: string;
  env?: string;
}): Promise<{ categories: string[]; counts: Record<string, number> }> {
  const res = await request('/api/functions/categories', { params });
  return {
    categories: res.categories || [],
    counts: res.counts || {}
  };
}

/**
 * 获取函数标签
 */
export async function getFunctionTags(params?: {
  game_id?: string;
  env?: string;
  limit?: number;
}): Promise<{ tags: string[]; counts: Record<string, number> }> {
  const res = await request('/api/functions/tags', { params });
  return {
    tags: res.tags || [],
    counts: res.counts || {}
  };
}

/**
 * 导出函数配置
 */
export async function exportFunctions(params: {
  function_ids?: string[];
  format?: 'json' | 'yaml' | 'csv';
  include_metadata?: boolean;
}): Promise<{ download_url: string; expires_at: string }> {
  return request('/api/functions/export', {
    method: 'POST',
    data: params
  });
}

/**
 * 导入函数配置
 */
export async function importFunctions(params: {
  file_url: string;
  format?: 'json' | 'yaml' | 'csv';
  overwrite?: boolean;
  game_id?: string;
  env?: string;
}): Promise<{ imported: number; skipped: number; errors: string[] }> {
  return request('/api/functions/import', {
    method: 'POST',
    data: params
  });
}

/**
 * 验证函数配置
 */
export async function validateFunctionConfig(params: {
  function_config: any;
  strict?: boolean;
}): Promise<{ valid: boolean; errors: string[]; warnings: string[] }> {
  return request('/api/functions/validate', {
    method: 'POST',
    data: params
  });
}

/**
 * 获取函数依赖关系
 */
export async function getFunctionDependencies(functionId: string): Promise<{
  dependencies: string[];
  dependents: string[];
  circular_dependencies: string[];
}> {
  return request(`/api/functions/${functionId}/dependencies`);
}

/**
 * 测试函数
 */
export async function testFunction(params: {
  function_id: string;
  payload: any;
  dry_run?: boolean;
  game_id?: string;
  env?: string;
}): Promise<{ valid: boolean; result?: any; error?: string; duration?: number }> {
  return request('/api/functions/test', {
    method: 'POST',
    data: params
  });
}