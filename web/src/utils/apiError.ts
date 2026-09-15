/**
 * API 错误稳定码提取（T8/D2）。
 *
 * 响应契约：HTTP 状态表达错误类别（409=冲突），body.error 为前端分支
 * snake_case 稳定码，message 为用户可读文本。umi request 对非 2xx 抛
 * ResponseError（.data 即响应体），本函数从中取稳定码供渲染层分支。
 */

type ErrorLike = { data?: unknown };

/** 提取稳定错误码（如 executor_unbound / binding_stale）；无结构化 body 时返回 ''。 */
export function extractApiErrorCode(err: unknown): string {
  if (err && typeof err === 'object') {
    const data = (err as ErrorLike).data;
    if (data && typeof data === 'object') {
      const code = (data as { error?: unknown }).error;
      if (typeof code === 'string' && code) return code;
    }
  }
  return '';
}
