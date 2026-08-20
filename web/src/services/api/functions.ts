import { request } from '@umijs/max';
import {
  normalizeFunctionInstance,
  type FunctionInstance,
  type LocalizedText,
  type RawFunctionInstance,
  normalizeLocalizedText as normalizeLocalizedTextUnified,
} from './functions-enhanced';
import { normalizeFunctionOpenAPIResponse, type OpenAPIOperation } from './openapi';
import type { JSONValue } from '@/types/dashboard';

// Source: backend function descriptor endpoints and registry-derived descriptors.
// Primary backend references: croupier/internal/api/function/dto.go and internal/logic/function/descriptors logic.
export type FunctionDescriptor = {
  id: string;
  type?: 'function';
  version?: string;
  name?: string;
  description?: LocalizedText;
  resource?: string;
  displayName?: LocalizedText;
  summary?: LocalizedText;
  operation?: string;
  tags?: string[];
  risk?: string;
  params?: JSONValue;
  auth?: Record<string, JSONValue>;
  outputs?: JSONValue;
  schema?: JSONValue;
  operations?: JSONValue;
  inputSchema?: JSONValue; // JSON Schema for request body (from proto)
  outputSchema?: JSONValue; // JSON Schema for response body (from proto)
};

type RawLocalizedText = Record<string, string | undefined>;

type RawFunctionDescriptor = Omit<FunctionDescriptor, 'displayName' | 'summary' | 'description'> & {
  displayName?: RawLocalizedText | string;
  summary?: RawLocalizedText | string;
  description?: RawLocalizedText | string;
  input?: JSONValue;
  output?: JSONValue;
  descriptor?: {
    input?: JSONValue;
    output?: JSONValue;
    schema?: JSONValue;
  };
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
  gameId: string;
  env: string;
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
  mode?: 'async';
};

// JSON request body sent to POST /api/v1/functions/:id/invoke.
// Source: croupier/internal/api/function/dto.go FunctionInvokeRequest
type FunctionInvokeRequestDTO = {
  payload: JSONValue;
  route?: 'lb' | 'broadcast' | 'targeted' | 'hash';
  targetServiceId?: string;
  hashKey?: string;
  mode?: 'async';
};

type RawFunctionRegistrationWarning = {
  key: string;
  gameId?: string;
  env?: string;
  agentId?: string;
  functionId?: string;
  version?: string;
  code: string;
  message: string;
  count: number;
  firstSeen?: string;
  lastSeen?: string;
};

function normalizeFunctionRegistrationWarning(
  raw: RawFunctionRegistrationWarning,
): FunctionRegistrationWarning {
  return {
    key: raw.key,
    gameId: raw.gameId ?? '',
    env: raw.env ?? '',
    agentId: raw.agentId ?? '',
    functionId: raw.functionId ?? '',
    version: raw.version,
    code: raw.code,
    message: raw.message,
    count: raw.count,
    firstSeen: raw.firstSeen ?? '',
    lastSeen: raw.lastSeen ?? '',
  };
}

// 统一本地化归一（BCP47 契约），与 functions-enhanced 共用同一实现
const normalizeLocalizedText = normalizeLocalizedTextUnified;

// Exported for testing
export function normalizeFunctionDescriptor(raw: RawFunctionDescriptor): FunctionDescriptor {
  const nested = raw.descriptor;
  const inputSchema = raw.inputSchema ?? raw.input ?? nested?.input;
  const outputSchema = raw.outputSchema ?? raw.output ?? nested?.output;
  return {
    ...raw,
    displayName: normalizeLocalizedText(raw.displayName),
    summary: normalizeLocalizedText(raw.summary),
    description: normalizeLocalizedText(raw.description),
    inputSchema,
    outputSchema,
    schema: raw.schema ?? nested?.schema,
  };
}

/**
 * 将详情接口的基础函数信息与嵌套 descriptor 投影为页面统一使用的描述符。
 * 详情接口返回 { name, description, descriptor: { input, output, schema } }，
 * 而列表/注册接口通常直接返回 input/output；两者必须在 API 层归一化。
 */
export function normalizeFunctionDetail(raw: RawFunctionDescriptor): FunctionDescriptor {
  const normalized = normalizeFunctionDescriptor(raw);
  return {
    ...normalized,
    displayName: normalized.displayName || normalizeLocalizedText(raw.name),
    summary: normalized.summary || normalizeLocalizedText(raw.description),
  };
}

// Canonical frontend projection of invoke response.
// Backend references:
// - croupier/internal/api/function/dto.go FunctionInvokeResponse
// - croupier/internal/api/function/helpers.go functionInvoke(...)
export type FunctionInvokeResponse = {
  result?: JSONValue;
  error?: string;
  duration?: number;
  timestamp?: string;
  taskId?: string;
  taskID?: string;
  /** OTel trace id of this invocation, for Jaeger/Grafana lookup. */
  traceId?: string;
  approvalRequired?: boolean;
  approvalId?: string;
  approvalWorkflow?: string;
};

export async function listDescriptors() {
  // 契约：GET /api/v1/functions/descriptors -> { functions: [...] }
  const response = await request<{ functions?: RawFunctionDescriptor[] } | RawFunctionDescriptor[]>(
    '/api/v1/functions/descriptors',
  );
  const raw = Array.isArray(response) ? response : (response.functions ?? []);
  return raw.map(normalizeFunctionDescriptor);
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
        functionId: params?.functionId,
        agentId: params?.agentId,
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
  payload: JSONValue,
  opts?: InvokeFunctionOptions,
): Promise<FunctionInvokeResponse> {
  const data: FunctionInvokeRequestDTO = { payload };
  if (opts?.route) data.route = opts.route;
  if (opts?.targetServiceId) data.targetServiceId = opts.targetServiceId;
  if (opts?.hashKey) data.hashKey = opts.hashKey;
  if (opts?.mode) data.mode = opts.mode;
  return request<FunctionInvokeResponse>(
    `/api/v1/functions/${encodeURIComponent(functionId)}/invoke`,
    {
      method: 'POST',
      data,
    },
  );
}

export async function startTask(
  functionId: string,
  payload: JSONValue,
  opts?: InvokeFunctionOptions,
): Promise<FunctionInvokeResponse> {
  const data: FunctionInvokeRequestDTO = { payload, mode: 'async' };
  if (opts?.route) data.route = opts.route;
  if (opts?.targetServiceId) data.targetServiceId = opts.targetServiceId;
  if (opts?.hashKey) data.hashKey = opts.hashKey;
  return request<FunctionInvokeResponse>(
    `/api/v1/functions/${encodeURIComponent(functionId)}/invoke`,
    {
      method: 'POST',
      data,
    },
  );
}

// Cancel a running task. Backed by the server's task cancel route, NOT a
// legacy /jobs route. See internal/handler/routes.go registerTaskRoutes.
export async function cancelTask(taskId: string) {
  return request<void>(`/api/v1/tasks/${encodeURIComponent(taskId)}/cancel`, { method: 'POST' });
}

// Fetch the final result/state of a task. The server has no /tasks/:id/result
// endpoint; the task detail endpoint GET /api/v1/tasks/:id already carries the
// final `result` and `error`, so we project it into the {state, payload, error}
// shape that callers consume.
export async function fetchTaskResult(taskId: string) {
  const detail = await request<{ status?: string; result?: JSONValue; error?: string }>(
    `/api/v1/tasks/${encodeURIComponent(taskId)}`,
    { method: 'GET' },
  );
  return { state: detail.status, payload: detail.result, error: detail.error };
}

export async function listFunctionInstances(params: {
  functionId: string;
}): Promise<{ instances: FunctionInstance[] }> {
  const res = await request<{ items?: RawFunctionInstance[]; instances?: RawFunctionInstance[] }>(
    `/api/v1/functions/${encodeURIComponent(params.functionId)}/instances`,
    { method: 'GET' },
  );
  const rawItems = res?.items || res?.instances || [];
  return { instances: rawItems.map(normalizeFunctionInstance) };
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

// A single task event emitted by GET /api/v1/tasks/:id/events.
// Source: croupier/internal/api/task/dto.go EventItem.
export type TaskEvent = {
  seq: number;
  type: string;
  progress: number;
  message: string;
  payload: JSONValue;
  createdAt: string;
};

export interface TaskEventHandlers {
  onEvent?: (event: TaskEvent) => void;
  onDone?: () => void;
  onError?: (err: unknown) => void;
}

export interface TaskEventSubscription {
  close(): void;
}

// Subscribe to a task's event stream by polling GET /api/v1/tasks/:id/events.
//
// The legacy frontend opened an SSE EventSource at /api/v1/jobs/:id/stream, but
// the server never exposed that route: it only offers the polled JSON endpoint
// GET /api/v1/tasks/:id/events (returns {items, next_seq, done}). This helper
// adapts the frontend to the real endpoint: it polls, replays new events via
// onEvent, fires onDone when the server marks the stream done, and stops on
// error. Returns a handle whose close() cancels polling.
//
// Server reference: internal/handler/routes.go registerTaskRoutes and
// internal/api/task/service.go Service.Events.
export function subscribeTaskEvents(
  taskId: string,
  handlers: TaskEventHandlers = {},
): TaskEventSubscription {
  let afterSeq = 0;
  let stopped = false;
  let timer: ReturnType<typeof setTimeout> | null = null;
  const intervalMs = 1500;

  const poll = async () => {
    if (stopped) return;
    try {
      const res = await request<{
        items?: TaskEvent[];
        nextSeq?: number;
        done?: boolean;
      }>(`/api/v1/tasks/${encodeURIComponent(taskId)}/events`, {
        params: { afterSeq },
      });
      if (stopped) return;
      const items = res.items || [];
      for (const it of items) handlers.onEvent?.(it);
      afterSeq = res?.nextSeq ?? afterSeq;
      if (res.done) {
        handlers.onDone?.();
        return;
      }
    } catch (err) {
      if (!stopped) handlers.onError?.(err);
      return;
    }
    if (stopped) return;
    timer = setTimeout(poll, intervalMs);
  };

  void poll();

  return {
    close() {
      stopped = true;
      if (timer) clearTimeout(timer);
    },
  };
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
      functionIds: data.functionIds,
      enabled: data.enabled,
    },
  });
}

export async function copyFunction(functionId: string) {
  const response = await request<{ functionId: string; newId: string }>(
    `/api/v1/functions/${encodeURIComponent(functionId)}/copy`,
    {
      method: 'POST',
    },
  );
  return {
    functionId: response.functionId,
    newId: response.newId,
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
  return normalizeFunctionDetail(response);
}

// Source: croupier/internal/api/function/dto.go FunctionHistoryItem
export type FunctionHistoryItemDTO = {
  id: string;
  action: string;
  operator?: string;
  timestamp: string;
  details?: JSONValue;
};

export async function getFunctionHistory(
  functionId: string,
  params?: { limit?: number; offset?: number },
): Promise<{ items: Array<FunctionHistoryItemDTO>; total: number }> {
  const response = await request<{
    items?: Array<FunctionHistoryItemDTO>;
    total?: number;
  }>(`/api/v1/functions/${encodeURIComponent(functionId)}/history`, { params });
  const items = Array.isArray(response?.items) ? response.items : [];
  return { items, total: response?.total ?? items.length };
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
    resource?: string;
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
