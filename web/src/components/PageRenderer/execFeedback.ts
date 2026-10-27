import { extractApiErrorCode, isResultUnknownError } from '@/utils/apiError';
import type { PageExecutionResult } from '@/types/dashboard';

/** 失败反馈：稳定错误码 + 用户可读文本 + 字段级明细（参数错误定位到字段）。 */
export interface ExecFailure {
  code: string;
  message: string;
  fieldErrors: Array<{ field: string; message: string }>;
  /** 结果类别（D4 超时/网络中断）：unknown=请求可能已在服务端执行、结果未知，
   * failed=服务端已应答（HTTP 错误）或本地失败、结果已知为未成功。 */
  outcome: 'failed' | 'unknown';
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

/** 从执行异常提取失败反馈（400 参数错误、403 权限不足等）。 */
export function describeExecFailure(err: unknown): ExecFailure {
  const body = errorBody(err);
  const message =
    typeof body?.message === 'string' && body.message
      ? body.message
      : typeof (err as ErrorLike | undefined)?.message === 'string'
        ? ((err as ErrorLike).message as string)
        : '';
  return {
    code: extractApiErrorCode(err),
    message,
    fieldErrors: fieldErrorsOf(body?.details),
    // 传输层失败（超时/网络中断）结果未知：可能已在服务端执行，不能按
    // 「已失败」提示盲目重试（D4 只在幂等保证或确认未执行时安全重试）。
    outcome: isResultUnknownError(err) ? 'unknown' : 'failed',
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
