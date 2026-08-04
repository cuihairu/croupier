/**
 * Console API 服务
 *
 * 运行控制台专用 API，从 PublishedPageSpec 读取数据。
 */

import { request } from '@umijs/max';
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
    created_at?: string;
    createdAt?: string;
  }>;
};

type ApprovalDetailResponse = {
  id: string;
  state?: string;
  function_id?: string;
  actor?: string;
  reason?: string;
  updated_at?: string;
  updatedAt?: string;
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
  const response = await request<ConsolePageResponse>(`${BASE}/pages/${encodeURIComponent(pageKey)}`, {
    method: 'GET',
  });
  if (!response?.page) {
    throw new Error(`published page not found: ${pageKey}`);
  }
  return response.page;
}

/** 受控执行已发布页面 binding，浏览器不传 functionId/route/scope */
export async function executePageBinding(
  pageKey: string,
  bindingId: string,
  context: BindingExecutionContext = {},
): Promise<PageExecutionResult> {
  const response = await request<ConsoleExecuteBindingResponse>(
    `${BASE}/pages/${encodeURIComponent(pageKey)}/bindings/${encodeURIComponent(bindingId)}/execute`,
    {
      method: 'POST',
      data: { context },
    },
  );
  if (!response?.result) {
    throw new Error(`page binding execution returned empty result: ${bindingId}`);
  }
  return response.result;
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
  const detail = await request<ApprovalDetailResponse>(`/api/v1/approvals/${encodeURIComponent(approvalId)}`, {
    method: 'GET',
  });
  return {
    approvalId: detail.id || approvalId,
    status: normalizeApprovalStatus(detail.state),
    functionId: detail.function_id,
    actor: detail.actor,
    reason: detail.reason,
    updatedAt: detail.updatedAt || detail.updated_at,
  };
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
    timestamp: event.createdAt || event.created_at || '',
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
