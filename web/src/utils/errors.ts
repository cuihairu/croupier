type ErrorPayload = {
  message?: unknown;
  error?: unknown;
  details?: unknown;
};

type RequestLikeError = {
  data?: ErrorPayload;
  info?: { data?: ErrorPayload };
  response?: { data?: ErrorPayload };
  message?: unknown;
};

function firstString(values: unknown[]): string | undefined {
  return values.find((item): item is string => typeof item === 'string' && item.trim().length > 0);
}

function errorPayloadOf(error: unknown): ErrorPayload | undefined {
  if (!error || typeof error !== 'object') return undefined;
  const err = error as RequestLikeError;
  if (err.response?.data && typeof err.response.data === 'object') return err.response.data;
  if (err.data && typeof err.data === 'object') return err.data;
  if (err.info?.data && typeof err.info.data === 'object') return err.info.data;
  return undefined;
}

/** 提取后端统一错误体的稳定错误码（error 字段），如 "mfa_required"。 */
export function extractErrorCode(error: unknown): string | undefined {
  const payload = errorPayloadOf(error);
  if (!payload) return undefined;
  if (typeof payload.error === 'string' && payload.error) return payload.error;
  return undefined;
}

/** 是否为 MFA 二次验证需求错误（登录 401 + error=mfa_required）。 */
export function isMfaRequiredError(error: unknown): boolean {
  return extractErrorCode(error) === 'mfa_required';
}

export function extractErrorMessage(error: unknown, fallback: string): string {
  if (!error || typeof error !== 'object') return fallback;
  const err = error as RequestLikeError;
  return (
    firstString([
      err.data?.message,
      err.info?.data?.message,
      err.response?.data?.message,
      err.response?.data?.error,
      err.message,
    ]) || fallback
  );
}

export type ApiErrorDetail = {
  field: string;
  message: string;
};

/**
 * 提取后端统一错误体中的 details 字段（{ 字段路径: 原因 } 或表单式数组），
 * 用于向用户展示结构化的失败原因（API Response Contract）。
 */
export function extractErrorDetails(error: unknown): ApiErrorDetail[] {
  const payload = errorPayloadOf(error);
  const raw = payload?.details;
  if (!raw) return [];
  const out: ApiErrorDetail[] = [];
  if (Array.isArray(raw)) {
    for (const item of raw) {
      if (!item || typeof item !== 'object') continue;
      const entry = item as Record<string, unknown>;
      const field = typeof entry.field === 'string' ? entry.field : '';
      const message = typeof entry.message === 'string' ? entry.message : '';
      if (field || message) out.push({ field, message });
    }
    return out;
  }
  if (typeof raw === 'object') {
    for (const [key, value] of Object.entries(raw as Record<string, unknown>)) {
      const message = typeof value === 'string' ? value : JSON.stringify(value);
      out.push({ field: key, message });
    }
  }
  return out;
}

/**
 * 把 details 渲染成 `字段: 原因` 多行文本（用于 Modal/notification 的描述区）。
 */
export function formatErrorDetails(error: unknown, separator = '\n'): string {
  return extractErrorDetails(error)
    .map(({ field, message }) => (field ? `${field}: ${message}` : message))
    .join(separator);
}

/**
 * 判断错误是否为结构化校验失败（details 非空），用于决定是否展示明细 UI。
 */
export function hasErrorDetails(error: unknown): boolean {
  return extractErrorDetails(error).length > 0;
}
