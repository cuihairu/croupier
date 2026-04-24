import { request } from '@umijs/max';
import { createEventSource } from '../core/http';
import {
  normalizeFunctionInstance,
  type FunctionInstance,
  type LocalizedText,
} from './functions-enhanced';
import {
  normalizeFunctionOpenAPIResponse,
  type OpenAPIImportResponse,
  type OpenAPIOperation,
} from './openapi';

// Source: backend function descriptor endpoints and registry-derived descriptors.
// Primary backend references: croupier/internal/api/function/dto.go and internal/logic/function/descriptors logic.
export type FunctionDescriptor = {
  id: string;
  type?: 'function' | 'entity'; // Type of descriptor: function or entity
  version?: string;
  category?: string;
  description?: string;
  displayName?: LocalizedText;
  summary?: LocalizedText;
  entity?: string;
  operation?: string;
  entityDisplay?: LocalizedText;
  operationDisplay?: LocalizedText;
  tags?: string[];
  menu?: {
    nodes?: string[];
    path?: string;
    order?: number;
    icon?: string;
    badge?: string;
    hidden?: boolean;
  };
  params?: any;
  auth?: Record<string, any>;
  // Optional outputs schema for UI rendering (views/layout); present in generated descriptors
  outputs?: any;
  // Entity-specific fields
  schema?: any;
  operations?: any;
  ui?: any;
  // OpenAPI 3.0.3 Schema fields (JSON Schema format, stringified)
  inputSchema?: string; // JSON Schema for request body (from proto)
  outputSchema?: string; // JSON Schema for response body (from proto)
};

type RawFunctionDescriptor = FunctionDescriptor & {
  display_name?: LocalizedText;
  entity_display?: LocalizedText;
  operation_display?: LocalizedText;
  input_schema?: string;
  output_schema?: string;
};

// Source: croupier/internal/api/function/dto.go FunctionPermission
export type FunctionPermission = {
  resource: string;
  actions: string[];
  roles: string[];
  gameId?: string;
  env?: string;
};

// Frontend warning DTO for /api/v1/functions/warnings.
// Source should be kept aligned with croupier/internal/api/function/dto.go FunctionWarningItem / response types.
export type FunctionRegistrationWarning = {
  key: string;
  agentId: string;
  functionId: string;
  version?: string;
  code: string;
  message: string;
  count: number;
  firstSeen: string;
  lastSeen: string;
};

// Frontend call options mapped onto backend FunctionInvokeRequest.
// Source: croupier/internal/api/function/dto.go FunctionInvokeRequest
export type InvokeFunctionOptions = {
  route?: 'lb' | 'broadcast' | 'targeted' | 'hash';
  targetServiceId?: string;
  hashKey?: string;
  gameId?: string;
  env?: string;
};

// JSON request body sent to POST /api/v1/functions/:id/invoke.
// Source: croupier/internal/api/function/dto.go FunctionInvokeRequest
type FunctionInvokeRequestDTO = {
  payload: any;
  gameId?: string;
  env?: string;
  route?: 'lb' | 'broadcast' | 'targeted' | 'hash';
  target_service_id?: string;
  hash_key?: string;
  mode?: 'async';
};

type RawFunctionRegistrationWarning = {
  key: string;
  agent_id?: string;
  function_id?: string;
  version?: string;
  code: string;
  message: string;
  count: number;
  first_seen?: string;
  last_seen?: string;
};

function buildFunctionUiPayload(uiConfig: {
  schema?: any;
  layout?: any;
  components?: any;
  clearCustom?: boolean;
}) {
  if (uiConfig.clearCustom) {
    const clearSchema = { __clear_custom_ui: true };
    return {
      ui: clearSchema,
      schema: clearSchema,
      layout: uiConfig.layout,
      components: uiConfig.components,
    };
  }

  return {
    ui: uiConfig.schema,
    schema: uiConfig.schema,
    layout: uiConfig.layout,
    components: uiConfig.components,
  };
}

function normalizeFunctionRegistrationWarning(
  raw: RawFunctionRegistrationWarning,
): FunctionRegistrationWarning {
  return {
    key: raw.key,
    agentId: raw.agent_id ?? '',
    functionId: raw.function_id ?? '',
    version: raw.version,
    code: raw.code,
    message: raw.message,
    count: raw.count,
    firstSeen: raw.first_seen ?? '',
    lastSeen: raw.last_seen ?? '',
  };
}

function normalizeFunctionDescriptor(raw: RawFunctionDescriptor): FunctionDescriptor {
  return {
    ...raw,
    displayName: raw.displayName || raw.display_name,
    summary: raw.summary,
    entityDisplay: raw.entityDisplay || raw.entity_display,
    operationDisplay: raw.operationDisplay || raw.operation_display,
    inputSchema: raw.inputSchema || raw.input_schema,
    outputSchema: raw.outputSchema || raw.output_schema,
  };
}

// Canonical frontend projection of invoke response.
// Backend references:
// - croupier/internal/api/function/dto.go FunctionInvokeResponse
// - croupier/internal/api/function/helpers.go functionInvoke(...)
export type FunctionInvokeResponse = {
  result?: any;
  error?: string;
  duration?: number;
  timestamp?: string;
  jobId?: string;
  jobID?: string;
};

export type FunctionUiSchemaDocument = {
  schema?: any;
  layout?: any;
  components?: any;
  custom?: boolean;
  hasDefault?: boolean;
  uiSource?: 'custom_metadata' | 'config_file_override' | 'openapi_x_ui' | 'none' | string;
  uiSourceDetail?: string;
  updatedAt?: string;
};

export async function listDescriptors() {
  const response = await request<RawFunctionDescriptor[]>('/api/v1/functions/descriptors');
  return Array.isArray(response) ? response.map(normalizeFunctionDescriptor) : [];
}

export async function listFunctionWarnings(params?: {
  functionId?: string;
  agentId?: string;
  code?: string;
  limit?: number;
}) {
  const response = await request<{ items?: RawFunctionRegistrationWarning[] }>(
    '/api/v1/functions/warnings',
    {
      method: 'GET',
      params: {
        function_id: params?.functionId,
        agent_id: params?.agentId,
        code: params?.code,
        limit: params?.limit,
      },
    },
  );
  return {
    items: Array.isArray(response?.items)
      ? response.items.map(normalizeFunctionRegistrationWarning)
      : [],
  };
}

export async function invokeFunction(
  functionId: string,
  payload: any,
  opts?: InvokeFunctionOptions,
): Promise<FunctionInvokeResponse> {
  const data: FunctionInvokeRequestDTO = { payload };
  const gameId =
    opts?.gameId ??
    (typeof window !== 'undefined' ? localStorage.getItem('game_id') || undefined : undefined);
  const env =
    opts?.env ??
    (typeof window !== 'undefined' ? localStorage.getItem('env') || undefined : undefined);
  if (gameId) data.gameId = gameId;
  if (env) data.env = env;
  if (opts?.route) data.route = opts.route;
  if (opts?.targetServiceId) data.target_service_id = opts.targetServiceId;
  if (opts?.hashKey) data.hash_key = opts.hashKey;
  return request<FunctionInvokeResponse>(
    `/api/v1/functions/${encodeURIComponent(functionId)}/invoke`,
    {
      method: 'POST',
      data,
    },
  );
}

export async function startJob(
  functionId: string,
  payload: any,
  opts?: InvokeFunctionOptions,
): Promise<FunctionInvokeResponse> {
  const data: FunctionInvokeRequestDTO = { payload, mode: 'async' };
  const gameId =
    opts?.gameId ??
    (typeof window !== 'undefined' ? localStorage.getItem('game_id') || undefined : undefined);
  const env =
    opts?.env ??
    (typeof window !== 'undefined' ? localStorage.getItem('env') || undefined : undefined);
  if (gameId) data.gameId = gameId;
  if (env) data.env = env;
  if (opts?.route) data.route = opts.route;
  if (opts?.targetServiceId) data.target_service_id = opts.targetServiceId;
  if (opts?.hashKey) data.hash_key = opts.hashKey;
  return request<FunctionInvokeResponse>(
    `/api/v1/functions/${encodeURIComponent(functionId)}/invoke`,
    {
      method: 'POST',
      data,
    },
  );
}

export async function cancelJob(job_id: string) {
  return request<void>(`/api/v1/jobs/${encodeURIComponent(job_id)}/cancel`, { method: 'POST' });
}

export async function fetchJobResult(id: string) {
  return request<{ state: string; payload?: any; error?: string }>(`/api/v1/jobs/${id}/result`, {
    method: 'GET',
  });
}

export async function listFunctionInstances(params: {
  gameId?: string;
  functionId: string;
}): Promise<{ instances: FunctionInstance[] }> {
  const res = await request<{ items?: any[]; instances?: any[] }>(
    `/api/v1/functions/${encodeURIComponent(params.functionId)}/instances`,
    {
      params: {
        game_id: params.gameId,
      },
    },
  );
  const rawItems = res?.items || res?.instances || [];
  return { instances: rawItems.map(normalizeFunctionInstance) };
}

export async function fetchFunctionUiSchema(functionId: string): Promise<FunctionUiSchemaDocument> {
  const response = await request<{
    schema?: any;
    layout?: any;
    components?: any;
    custom?: boolean;
    hasDefault?: boolean;
    uiSource?: 'custom_metadata' | 'config_file_override' | 'openapi_x_ui' | 'none' | string;
    uiSourceDetail?: string;
    updated_at?: string;
    updatedAt?: string;
  }>(`/api/v1/functions/${encodeURIComponent(functionId)}/ui`, { method: 'GET' });
  const normalized: FunctionUiSchemaDocument = {
    ...response,
    updatedAt: response.updatedAt || response.updated_at,
  };
  return normalized;
}

export async function saveFunctionUiSchema(
  functionId: string,
  uiConfig: {
    schema?: any;
    layout?: any;
    components?: any;
    clearCustom?: boolean;
  },
) {
  return request<FunctionUiSchemaDocument>(
    `/api/v1/functions/${encodeURIComponent(functionId)}/ui`,
    {
      method: 'PUT',
      data: buildFunctionUiPayload(uiConfig),
    },
  );
}

export type FunctionUIHistoryItem = {
  version: number;
  schema?: any;
  layout?: any;
  components?: any;
  message?: string;
  createdBy?: string;
  createdAt?: string;
};

export async function fetchFunctionUiHistory(functionId: string) {
  return request<{ items: FunctionUIHistoryItem[] }>(
    `/api/v1/functions/${encodeURIComponent(functionId)}/ui/history`,
    { method: 'GET' },
  );
}

export async function rollbackFunctionUiSchema(functionId: string, version: number) {
  return request<{
    appliedVersion: number;
    current?: {
      schema?: any;
      layout?: any;
      components?: any;
      custom?: boolean;
      hasDefault?: boolean;
      uiSource?: string;
      uiSourceDetail?: string;
    };
  }>(`/api/v1/functions/${encodeURIComponent(functionId)}/ui/rollback`, {
    method: 'POST',
    data: { version },
  });
}

export type FunctionRouteConfig = {
  nodes?: string[];
  path?: string;
  order?: number;
  hidden?: boolean;
};

export async function fetchFunctionRoute(functionId: string) {
  return request<{
    menu?: FunctionRouteConfig;
    source?: 'metadata' | 'default' | string;
  }>(`/api/v1/functions/${encodeURIComponent(functionId)}/route`, { method: 'GET' });
}

export async function saveFunctionRoute(functionId: string, route: FunctionRouteConfig) {
  return request<{
    menu?: FunctionRouteConfig;
    source?: 'metadata' | 'default' | string;
  }>(`/api/v1/functions/${encodeURIComponent(functionId)}/route`, {
    method: 'PUT',
    data: route,
  });
}

export async function getFunctionPermissions(functionId: string) {
  return request<{ items?: FunctionPermission[] }>(
    `/api/v1/functions/${encodeURIComponent(functionId)}/permissions`,
    {
      method: 'GET',
    },
  );
}

export async function updateFunctionPermissions(
  functionId: string,
  permissions: FunctionPermission[],
) {
  return request<{ items?: FunctionPermission[] }>(
    `/api/v1/functions/${encodeURIComponent(functionId)}/permissions`,
    {
      method: 'PUT',
      data: { permissions },
    },
  );
}

export function openJobEventSource(jobId: string) {
  return createEventSource(`/api/v1/jobs/${jobId}/stream`);
}

// Batch operations
export async function updateFunctionStatus(functionId: string, data: { enabled: boolean }) {
  return request<void>(`/api/v1/functions/${encodeURIComponent(functionId)}/status`, {
    method: 'PUT',
    data,
  });
}

export async function enableFunction(functionId: string) {
  return request<void>(`/api/v1/functions/${encodeURIComponent(functionId)}/enable`, {
    method: 'POST',
  });
}

export async function disableFunction(functionId: string) {
  return request<void>(`/api/v1/functions/${encodeURIComponent(functionId)}/disable`, {
    method: 'POST',
  });
}

export async function batchUpdateFunctions(data: { functionIds: string[]; enabled: boolean }) {
  return request<{ updated: number; failed: string[] }>('/api/v1/functions/batch-update', {
    method: 'POST',
    data: {
      function_ids: data.functionIds,
      enabled: data.enabled,
    },
  });
}

export async function copyFunction(functionId: string) {
  const response = await request<{ function_id: string; new_id: string }>(
    `/api/v1/functions/${encodeURIComponent(functionId)}/copy`,
    {
      method: 'POST',
    },
  );
  return {
    functionId: response.function_id,
    newId: response.new_id,
  };
}

export async function deleteFunction(functionId: string) {
  return request<void>(`/api/v1/functions/${encodeURIComponent(functionId)}`, {
    method: 'DELETE',
  });
}

export async function getFunctionDetail(functionId: string) {
  const response = await request<RawFunctionDescriptor>(
    `/api/v1/functions/${encodeURIComponent(functionId)}`,
  );
  return normalizeFunctionDescriptor(response);
}

export async function getFunctionHistory(functionId: string) {
  return request<
    Array<{
      id: string;
      action: string;
      operator?: string;
      timestamp: string;
      details?: any;
    }>
  >(`/api/v1/functions/${encodeURIComponent(functionId)}/history`);
}

export async function getFunctionAnalytics(functionId: string) {
  return request<{
    totalCalls: number;
    successRate: number;
    avgLatency: number;
    callsToday: number;
    callsThisWeek: number;
    callsThisMonth: number;
  }>(`/api/v1/functions/${encodeURIComponent(functionId)}/analytics`);
}

export async function updateFunction(
  functionId: string,
  data: {
    name?: string;
    description?: string;
    category?: string;
    tags?: string[];
    enabled?: boolean;
  },
) {
  return request<void>(`/api/v1/functions/${encodeURIComponent(functionId)}`, {
    method: 'PUT',
    data,
  });
}

/**
 * 获取函数的 OpenAPI 3.0.3 规范
 * @param functionId 函数 ID
 * @returns OpenAPI Operation Object
 */
export async function getFunctionOpenAPI(functionId: string) {
  const resp = await request<{ spec: OpenAPIOperation }>(
    `/api/v1/functions/${encodeURIComponent(functionId)}/openapi`,
  );
  return normalizeFunctionOpenAPIResponse(resp);
}

/**
 * 导入 OpenAPI 3.0.3 规范
 * @param spec OpenAPI 3.0.3 Document
 * @returns 导入结果
 */
export async function importOpenAPISpec(spec: any) {
  return request<OpenAPIImportResponse>('/api/v1/functions/_import', {
    method: 'POST',
    data: { spec },
  });
}
