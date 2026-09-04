import { request } from '@umijs/max';

/** 执行留痕列表项（不含载荷，载荷走详情） */
export type ExecutionLogItem = {
  id: number;
  gameId: string;
  env: string;
  source: 'invoke' | 'page';
  functionId: string;
  pageKey?: string;
  bindingId?: string;
  actor: string;
  route?: string;
  status: string;
  durationMs: number;
  traceId?: string;
  truncated?: boolean;
  createdAt: string;
};

export type ExecutionLogDetail = ExecutionLogItem & {
  requestPayload?: unknown;
  responseBody?: unknown;
};

export type ExecutionLogListResponse = {
  items: ExecutionLogItem[];
  total: number;
  page: number;
  size: number;
};

/** 查询执行留痕（mine=true 只看自己的记录，无需审计权限） */
export async function listExecutionLogs(
  params: Record<string, string | number | boolean | undefined>,
): Promise<ExecutionLogListResponse> {
  return request<ExecutionLogListResponse>('/api/v1/execution-logs/', { params });
}

/** 执行留痕详情（含脱敏后的请求/响应载荷） */
export async function getExecutionLog(id: number): Promise<ExecutionLogDetail> {
  return request<ExecutionLogDetail>(`/api/v1/execution-logs/${id}`);
}
