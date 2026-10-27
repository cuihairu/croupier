import { extractApiErrorCode } from '@/utils/apiError';
import type { PageExecutionResult } from '@/types/dashboard';

/** 失败反馈：稳定错误码 + 用户可读文本 + 字段级明细（参数错误定位到字段）。 */
export interface ExecFailure {
  code: string;
  message: string;
  fieldErrors: Array<{ field: string; message: string }>;
  /** 权限类失败的结构化原因（required/allowedRoles），不落字段明细。 */
  reason?: string;
}

/** 执行结果类别：审批待处理与任务已提交都不等于业务执行成功（D4）。 */
export type ExecOutcome = 'approval' | 'task' | 'success';

type ErrorLike = {
  data?: unknown;
  message?: string;
  response?: { data?: unknown };
};

/** umi request 抛出的 ResponseError：结构化体在 .data，部分封装挂在 .response.data。 */
function errorBody(err: unknown): Record<string, unknown> | undefined {
  if (!err || typeof err !== 'object') return undefined;
  const like = err as ErrorLike;
  const data = like.data ?? like.response?.data;
  return data && typeof data === 'object' ? (data as Record<string, unknown>) : undefined;
}

/** 字段级错误明细：details 支持 {字段: 文案} 映射与 [{field, message}] 数组两种形态。 */
function fieldErrorsOf(details: unknown): ExecFailure['fieldErrors'] {
  if (!details || typeof details !== 'object') return [];
  if (Array.isArray(details)) {
    return details
      .filter((item): item is Record<string, unknown> => !!item && typeof item === 'object')
      .map((item) => ({
        field: String(item.field ?? item.key ?? ''),
        message: String(item.message ?? item.error ?? ''),
      }))
      .filter((item) => item.field || item.message);
  }
  return Object.entries(details as Record<string, unknown>)
    .map(([field, value]) => ({
      field,
      message: typeof value === 'string' ? value : JSON.stringify(value),
    }))
    .filter((item) => item.field);
}

function stringList(value: unknown): string[] {
  if (!Array.isArray(value)) return [];
  return value
    .map((item) => (typeof item === 'string' ? item.trim() : ''))
    .filter((item) => item.length > 0);
}

function firstString(details: Record<string, unknown>, keys: string[]): string {
  for (const key of keys) {
    const value = details[key];
    if (typeof value === 'string' && value.trim()) return value.trim();
  }
  return '';
}

/** 权限类失败把服务端 details 收敛为可读原因（不暴露原始 key）。 */
export function describePermissionReason(code: string, details: unknown): string | undefined {
  if (!['forbidden', 'permission_denied'].includes(code)) return undefined;
  if (!details || typeof details !== 'object' || Array.isArray(details)) return undefined;
  const record = details as Record<string, unknown>;
  const parts: string[] = [];
  const required = stringList(record.required ?? record.requiredPermissions ?? record.permissions);
  if (required.length > 0) parts.push(`缺少权限：${required.join('、')}`);
  const roles = stringList(record.allowedRoles);
  if (roles.length > 0) parts.push(`需要角色：${roles.join('、')}`);
  const functionId = firstString(record, ['functionId']);
  if (functionId && parts.length === 0) parts.push(`函数：${functionId}`);
  return parts.length > 0 ? parts.join('；') : undefined;
}

/** 从执行异常提取失败反馈（400 参数错误、403 权限不足等）。 */
export function describeExecFailure(err: unknown): ExecFailure {
  const body = errorBody(err);
  const code = extractApiErrorCode(err);
  const message =
    typeof body?.message === 'string' && body.message
      ? body.message
      : typeof (err as ErrorLike | undefined)?.message === 'string'
        ? ((err as ErrorLike).message as string)
        : '';
  return {
    code,
    message,
    reason: describePermissionReason(code, body?.details),
    fieldErrors: describePermissionReason(code, body?.details) ? [] : fieldErrorsOf(body?.details),
  };
}

/** 结果体是否自带顶层 error 标记（执行器返回错误体而非抛异常）。 */
export function isErrorExecResult(result: unknown): boolean {
  return !!result && typeof result === 'object' && !!(result as { error?: unknown }).error;
}

/** 执行结果分类：审批待处理 / 任务已提交 / 业务执行成功。
 * 顶层 error 标记与空结果不算成功（调用方先用 isErrorExecResult 区分）；
 * 空结果在 preview / 旧响应拒收时返回，同样不该触发 success 侧效应。 */
export function describeExecOutcome(result: unknown): ExecOutcome {
  const r = result as PageExecutionResult | null | undefined;
  if (!r || typeof r !== 'object') return 'success';
  if (r.kind === 'approval') return 'approval';
  if (r.kind === 'task') return 'task';
  return 'success';
}
