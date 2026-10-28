/**
 * Console API 服务
 *
 * 运行控制台专用 API，从 PublishedPageSpec 读取数据。
 */

import { request } from '@umijs/max';
import {
  acquireExecuteIdempotencyKey,
  executeIntentScopeKey,
  releaseExecuteIdempotencyKey,
} from '@/services/executeIdempotency';
import type {
  ApprovalStatusResult,
  BindingExecutionContext,
  ConsoleMenuSpec,
  JSONValue,
  PageExecutionResult,
  PublishedPageSpec,
  TaskEvent,
  TaskStatusResult,
} from '@/types/dashboard';

const BASE = '/api/v1/console';

type ConsolePagesResponse = {
  items?: PublishedPageSpec[];
};

type ConsolePageResponse = {
  page?: PublishedPageSpec;
};

type ConsoleExecuteBindingResponse = {
  result?: PageExecutionResult;
  /** 幂等重放：结果取自服务端已落地记录而非本次新下发。 */
  replayed?: boolean;
};

type TaskDetailResponse = {
  id: string;
  status?: string;
  progress?: number;
  message?: string;
  result?: JSONValue;
  error?: string;
};

type TaskEventsResponse = {
  items?: Array<{
    type?: string;
    progress?: number;
    message?: string;
    payload?: JSONValue;
    createdAt?: string;
  }>;
};

type ApprovalDetailResponse = {
  id: string;
  state?: string;
  functionId?: string;
  actor?: string;
  reason?: string;
  continuation?: boolean;
  resultKind?: 'sync' | 'task';
  taskId?: string;
  result?: JSONValue | string;
  updatedAt?: string;
};

type ApprovalGetResponse =
  | ApprovalDetailResponse
  | {
      approval?: ApprovalDetailResponse;
    };

/** 获取运行控制台菜单 */
export async function getConsoleMenu(lang?: string): Promise<ConsoleMenuSpec> {
  return request<ConsoleMenuSpec>(`${BASE}/menu`, {
    method: 'GET',
    params: lang ? { lang } : undefined,
  });
}

/** 获取已发布页面列表 */
export async function listPublishedPages(): Promise<PublishedPageSpec[]> {
  const response = await request<ConsolePagesResponse>(`${BASE}/pages`, {
    method: 'GET',
  });
  return Array.isArray(response?.items) ? response.items : [];
}

/** 获取单个已发布页面 */
export async function getPublishedPage(pageKey: string): Promise<PublishedPageSpec> {
  const response = await request<ConsolePageResponse>(
    `${BASE}/pages/${encodeURIComponent(pageKey)}`,
    {
      method: 'GET',
    },
  );
  if (!response?.page) {
    throw new Error(`published page not found: ${pageKey}`);
  }
  return response.page;
}

/**
 * 意图闭环判定：仅同步结果算执行已完成，释放该作用域的幂等键——之后同内容
 * 提交是新的意图（新 key 真实执行）。审批待处理与任务已提交结果未落定，保留
 * key：同内容重复提交在保留窗口内由服务端重放同一审批单/任务号，而非新建
 * （窗口到期后按新意图处理）。
 */
function finishExecuteIntent(scopeKey: string, result: PageExecutionResult): void {
  if (result.kind === 'sync') {
    releaseExecuteIdempotencyKey(scopeKey);
  }
}

/** 受控执行已发布页面 binding，浏览器不传 functionId/route/scope。
 * 同一提交意图（同页面/绑定/上下文且上一次未成功）复用同一 Idempotency-Key：
 * 服务端重放已落地结果或判结果未知，不重复下发副作用。 */
export async function executePageBinding(
  pageKey: string,
  bindingId: string,
  context: BindingExecutionContext = {},
): Promise<PageExecutionResult> {
  const scopeKey = executeIntentScopeKey({ pageKey, bindingId, context });
  const response = await request<ConsoleExecuteBindingResponse>(
    `${BASE}/pages/${encodeURIComponent(pageKey)}/bindings/${encodeURIComponent(bindingId)}/execute`,
    {
      method: 'POST',
      data: { context },
      skipErrorHandler: true,
      headers: { 'Idempotency-Key': acquireExecuteIdempotencyKey(scopeKey) },
    },
  );
  if (!response?.result) {
    throw new Error(`page binding execution returned empty result: ${bindingId}`);
  }
  finishExecuteIntent(scopeKey, response.result);
  return response.replayed ? { ...response.result, replayed: true } : response.result;
}

/** 草稿预览执行（B1）：请求体与组合页预览端点同形（页面 key + 区块请求 +
 * 模板快照），服务端按同一生成链重建草稿 PageSpec 并跑发布级 selector 校验，
 * 再走与已发布页面相同的执行门禁（权限 / 审批 / 契约快照）后下发执行；
 * 不落库，浏览器不提交 functionId / route / scope。幂等语义与已发布执行一致。 */
export async function executeDraftCompositeBinding(
  pageKey: string,
  bindingId: string,
  sections: readonly Record<string, unknown>[],
  context: BindingExecutionContext = {},
  componentTemplates?: readonly unknown[],
): Promise<PageExecutionResult> {
  const scopeKey = executeIntentScopeKey({ pageKey, bindingId, context });
  const response = await request<ConsoleExecuteBindingResponse>(
    '/api/v1/versioning/pages/composite/preview/execute',
    {
      method: 'POST',
      data: {
        pageKey,
        bindingId,
        sections: [...sections],
        ...(componentTemplates && componentTemplates.length > 0 ? { componentTemplates } : {}),
        context,
      },
      skipErrorHandler: true,
      headers: { 'Idempotency-Key': acquireExecuteIdempotencyKey(scopeKey) },
    },
  );
  if (!response?.result) {
    throw new Error(`draft composite execution returned empty result: ${bindingId}`);
  }
  finishExecuteIntent(scopeKey, response.result);
  return response.replayed ? { ...response.result, replayed: true } : response.result;
}

/** 查询异步任务状态，供 TaskPageRenderer 轮询使用 */
export async function queryTaskStatus(taskId: string): Promise<TaskStatusResult> {
  const [detail, events] = await Promise.all([
    request<TaskDetailResponse>(`/api/v1/tasks/${encodeURIComponent(taskId)}`, {
      method: 'GET',
    }),
    request<TaskEventsResponse>(`/api/v1/tasks/${encodeURIComponent(taskId)}/events`, {
      method: 'GET',
    }),
  ]);
  return {
    taskId: detail.id || taskId,
    status: normalizeTaskStatus(detail.status),
    progress: detail.progress,
    message: detail.message,
    result: detail.result,
    error: detail.error,
    events: (events.items || []).map(normalizeTaskEvent),
  };
}

/** 取消异步任务 */
export async function cancelTask(taskId: string): Promise<void> {
  await request<void>(`/api/v1/tasks/${encodeURIComponent(taskId)}/cancel`, {
    method: 'POST',
  });
}

/** 查询审批状态，供 Operation/Task 页面展示等待态 */
export async function queryApprovalStatus(approvalId: string): Promise<ApprovalStatusResult> {
  const response = await request<ApprovalGetResponse>(
    `/api/v1/approvals/${encodeURIComponent(approvalId)}`,
    {
      method: 'GET',
    },
  );
  const detail =
    'approval' in response && response.approval
      ? response.approval
      : (response as ApprovalDetailResponse);
  return {
    approvalId: detail.id || approvalId,
    status: normalizeApprovalStatus(detail.state),
    functionId: detail.functionId,
    actor: detail.actor,
    reason: detail.reason,
    continuation: detail.continuation,
    resultKind: detail.resultKind,
    taskId: detail.taskId,
    result: normalizeApprovalResult(detail.result),
    updatedAt: detail.updatedAt || detail.updatedAt,
  };
}

function normalizeApprovalResult(result?: JSONValue | string): JSONValue | undefined {
  if (result === undefined || result === '') {
    return undefined;
  }
  if (typeof result !== 'string') {
    return result;
  }
  try {
    return JSON.parse(result) as JSONValue;
  } catch {
    return result;
  }
}

function normalizeTaskStatus(status?: string): TaskStatusResult['status'] {
  switch (status) {
    case 'queued':
    case 'dispatching':
      return 'pending';
    case 'running':
      return 'running';
    case 'succeeded':
    case 'completed':
      return 'completed';
    case 'failed':
    case 'timed_out':
      return 'failed';
    case 'cancel_requested':
    case 'cancelled':
      return 'cancelled';
    default:
      return 'pending';
  }
}

function normalizeTaskEvent(event: NonNullable<TaskEventsResponse['items']>[number]): TaskEvent {
  return {
    timestamp: event.createdAt || event.createdAt || '',
    type: normalizeTaskEventType(event.type),
    message: event.message || '',
    data: event.payload,
  };
}

function normalizeTaskEventType(type?: string): TaskEvent['type'] {
  switch (type) {
    case 'failed':
    case 'error':
      return 'error';
    case 'cancel_requested':
    case 'cancelled':
    case 'warning':
      return 'warning';
    case 'progress':
      return 'progress';
    default:
      return 'info';
  }
}

function normalizeApprovalStatus(status?: string): ApprovalStatusResult['status'] {
  switch (status) {
    case 'approved':
      return 'approved';
    case 'rejected':
      return 'rejected';
    case 'expired':
      return 'expired';
    default:
      return 'pending';
  }
}
