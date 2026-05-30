export type WorkspaceErrorCode =
  | 'unauthorized'
  | 'forbidden'
  | 'workspace_not_found'
  | 'workspace_invalid_config'
  | 'workspace_publish_failed'
  | 'workspace_version_not_found'
  | 'network_error'
  | 'unknown';

export interface WorkspaceErrorInfo {
  code: WorkspaceErrorCode;
  message: string;
  status?: number;
  requestId?: string;
  raw?: any;
}

const DEFAULT_ERROR_MESSAGE: Record<WorkspaceErrorCode, string> = {
  unauthorized: '登录已过期，请重新登录',
  forbidden: '无权限执行该操作',
  workspace_not_found: '工作台配置不存在',
  workspace_invalid_config: '工作台配置非法',
  workspace_publish_failed: '工作台发布失败',
  workspace_version_not_found: '版本不存在或已失效',
  network_error: '网络异常，请稍后重试',
  unknown: '操作失败，请稍后重试',
};

function normalizeCode(code?: string, status?: number): WorkspaceErrorCode {
  const normalized = String(code || '').toLowerCase();
  if (normalized === 'unauthorized' || status === 401) return 'unauthorized';
  if (normalized === 'forbidden' || status === 403) return 'forbidden';
  if (normalized === 'workspace_version_not_found') return 'workspace_version_not_found';
  if (normalized === 'workspace_publish_failed') return 'workspace_publish_failed';
  if (normalized === 'workspace_not_found' || normalized === 'not_found' || status === 404) {
    return 'workspace_not_found';
  }
  if (normalized === 'workspace_invalid_config' || normalized === 'bad_request' || status === 400) {
    return 'workspace_invalid_config';
  }
  if (status === 0) return 'network_error';
  return 'unknown';
}

export function parseWorkspaceError(error: any): WorkspaceErrorInfo {
  const status = error?.response?.status as number | undefined;
  const payload = error?.response?.data;
  const code = normalizeCode(payload?.error || payload?.code, status);
  const message =
    payload?.message || payload?.error || error?.message || DEFAULT_ERROR_MESSAGE[code];
  return {
    code,
    message:
      typeof message === 'string' && message.trim()
        ? message
        : DEFAULT_ERROR_MESSAGE[code] || DEFAULT_ERROR_MESSAGE.unknown,
    status,
    requestId: payload?.request_id || payload?.requestId,
    raw: error,
  };
}

export function getWorkspaceErrorMessage(error: any, fallback?: string): string {
  const parsed = parseWorkspaceError(error);
  if (parsed.message && parsed.message !== 'Request failed') return parsed.message;
  return fallback || DEFAULT_ERROR_MESSAGE[parsed.code] || DEFAULT_ERROR_MESSAGE.unknown;
}
