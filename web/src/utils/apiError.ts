/**
 * API 错误稳定码提取（T8/D2）。
 *
 * 响应契约：HTTP 状态表达错误类别（409=冲突），body.error 为前端分支
 * snake_case 稳定码，message 为用户可读文本。umi request 对非 2xx 抛
 * ResponseError（.data 即响应体，部分封装挂在 .response.data），本函数从中
 * 取稳定码供渲染层分支，与 execFeedback.errorBody / utils/errors 同口径回退。
 */

type ErrorLike = { data?: unknown; response?: { data?: unknown } };

/** 提取稳定错误码（如 executor_unbound / binding_stale）；无结构化 body 时返回 ''。 */
export function extractApiErrorCode(err: unknown): string {
  if (err && typeof err === 'object') {
    const like = err as ErrorLike;
    const data = like.data ?? like.response?.data;
    if (data && typeof data === 'object') {
      const code = (data as { error?: unknown }).error;
      if (typeof code === 'string' && code) return code;
    }
  }
  return '';
}

/** 请求结果是否「未知」（D4 超时/网络中断）：传输层失败且请求可能已在服务端
 * 执行，不得按「已失败」提示盲目重试。umi request 对非 2xx 抛 ResponseError
 * （有 .data/.response，服务端已应答=结果已知——即使文案含 timeout 也不算
 * 未知）；传输层抛 TypeError（"Failed to fetch"）/ AbortError 或带 "timeout"
 * 的错误——这类请求可能已到达服务端，写操作结果未知，只能经幂等键安全重试
 * 或查询执行状态。 */
export function isResultUnknownError(err: unknown): boolean {
  if (!err || typeof err !== 'object') return false;
  const like = err as ErrorLike & { name?: unknown; message?: unknown; response?: unknown };
  if (like.data !== undefined || like.response !== undefined) return false;
  if (like.name === 'AbortError') return true;
  if (typeof like.message === 'string') {
    const m = like.message.toLowerCase();
    if (m.includes('timeout') || m.includes('failed to fetch') || m.includes('network error')) {
      return true;
    }
  }
  return false;
}
