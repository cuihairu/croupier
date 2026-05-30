import { request } from '@umijs/max';

// Source: croupier/internal/api/functioncall/dto.go Item
export type FunctionCallItem = {
  id: string;
  jobId: string;
  functionId: string;
  gameId?: string;
  env?: string;
  actorId?: string;
  actorType?: string;
  status: string;
  agentId?: string;
  serviceId?: string;
  startedAt?: string;
  finishedAt?: string;
  durationMs?: number;
  payload?: any;
  result?: any;
  errorMessage?: string;
  retryCount?: number;
  createdAt: string;
};

// Source: croupier/internal/api/functioncall/dto.go ListResponse
export type FunctionCallsListResponse = {
  calls: FunctionCallItem[];
  total: number;
  page: number;
  pageSize: number;
};

// Source: croupier/internal/api/functioncall/dto.go StatsResponse
export type FunctionCallStatsResponse = {
  total: number;
  succeeded: number;
  failed: number;
  running: number;
  cancelled: number;
  timeout: number;
  other: number;
  avgDurationMs: number;
};

// Source: croupier/internal/api/functioncall/dto.go ListRequest
export type FunctionCallsListParams = {
  functionId?: string;
  gameId?: string;
  env?: string;
  status?: string;
  actorId?: string;
  agentId?: string;
  startTime?: string;
  endTime?: string;
  page?: number;
  pageSize?: number;
};

type RawFunctionCallItem = {
  id: string;
  job_id?: string;
  jobId?: string;
  function_id?: string;
  functionId?: string;
  game_id?: string;
  gameId?: string;
  env?: string;
  actor_id?: string;
  actorId?: string;
  actor_type?: string;
  actorType?: string;
  status: string;
  agent_id?: string;
  agentId?: string;
  service_id?: string;
  serviceId?: string;
  started_at?: string;
  startedAt?: string;
  finished_at?: string;
  finishedAt?: string;
  duration_ms?: number;
  durationMs?: number;
  payload?: any;
  result?: any;
  error_msg?: string;
  errorMessage?: string;
  retry_count?: number;
  retryCount?: number;
  created_at?: string;
  createdAt?: string;
};

type RawFunctionCallsListResponse = {
  calls?: RawFunctionCallItem[];
  total?: number;
  page?: number;
  page_size?: number;
  pageSize?: number;
};

type RawFunctionCallStatsResponse = {
  total?: number;
  succeeded?: number;
  failed?: number;
  running?: number;
  cancelled?: number;
  timeout?: number;
  other?: number;
  avg_duration_ms?: number;
  avgDurationMs?: number;
};

function normalizeFunctionCallItem(item: RawFunctionCallItem): FunctionCallItem {
  return {
    id: item.id,
    jobId: item.job_id || item.jobId || '',
    functionId: item.function_id || item.functionId || '',
    gameId: item.game_id || item.gameId,
    env: item.env,
    actorId: item.actor_id || item.actorId,
    actorType: item.actor_type || item.actorType,
    status: item.status,
    agentId: item.agent_id || item.agentId,
    serviceId: item.service_id || item.serviceId,
    startedAt: item.started_at || item.startedAt,
    finishedAt: item.finished_at || item.finishedAt,
    durationMs: item.duration_ms || item.durationMs,
    payload: item.payload,
    result: item.result,
    errorMessage: item.error_msg || item.errorMessage,
    retryCount: item.retry_count || item.retryCount,
    createdAt: item.created_at || item.createdAt || '',
  };
}

/**
 * 获取函数调用历史列表
 */
export async function listFunctionCalls(params: FunctionCallsListParams = {}) {
  const response = await request<RawFunctionCallsListResponse>('/api/v1/function-calls', {
    params: {
      function_id: params.functionId,
      game_id: params.gameId,
      env: params.env,
      status: params.status,
      actor_id: params.actorId,
      agent_id: params.agentId,
      start_time: params.startTime,
      end_time: params.endTime,
      page: params.page,
      page_size: params.pageSize,
    },
  });
  return {
    calls: (response.calls || []).map(normalizeFunctionCallItem),
    total: response.total || 0,
    page: response.page || 1,
    pageSize: response.page_size || response.pageSize || params.pageSize || 20,
  };
}

/**
 * 获取单条调用历史详情
 */
export async function getFunctionCallDetail(id: string) {
  const response = await request<RawFunctionCallItem>(
    `/api/v1/function-calls/${encodeURIComponent(id)}`,
    {
      method: 'GET',
    },
  );
  return normalizeFunctionCallItem(response);
}

/**
 * 重新执行失败的调用
 */
export async function rerunFunctionCall(id: string, payload?: any) {
  const response = await request<{ job_id?: string; jobId?: string }>(
    `/api/v1/function-calls/${encodeURIComponent(id)}/rerun`,
    {
      method: 'POST',
      data: { payload },
    },
  );
  return { jobId: response.job_id || response.jobId || '' };
}

/**
 * 获取调用统计
 */
export async function getFunctionCallStats(
  params: {
    functionId?: string;
    gameId?: string;
    env?: string;
    actorId?: string;
    startTime?: string;
    endTime?: string;
  } = {},
) {
  const response = await request<RawFunctionCallStatsResponse>('/api/v1/function-calls/stats', {
    params: {
      function_id: params.functionId,
      game_id: params.gameId,
      env: params.env,
      actor_id: params.actorId,
      start_time: params.startTime,
      end_time: params.endTime,
    },
  });
  return {
    total: response.total || 0,
    succeeded: response.succeeded || 0,
    failed: response.failed || 0,
    running: response.running || 0,
    cancelled: response.cancelled || 0,
    timeout: response.timeout || 0,
    other: response.other || 0,
    avgDurationMs: response.avg_duration_ms || response.avgDurationMs || 0,
  };
}
