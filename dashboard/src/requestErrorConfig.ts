import type { RequestOptions } from '@@/plugin-request/request';
import type { RequestConfig } from '@umijs/max';
import { history } from '@umijs/max';
// Use App.useApp() instances (see app.tsx) to avoid AntD static message warnings
import { getMessage, getNotification } from './utils/antdApp';
import { normalizeApiUrl, API_V1_PREFIX } from './utils/api';

// Defer message/notification to avoid calling during render (React 18 concurrent mode)
function defer(fn: () => void) {
  try {
    setTimeout(fn, 0);
  } catch {
    /* no-op */
  }
}
function msgError(text: string) {
  const api = getMessage();
  if (api) defer(() => api.error(text));
}
function msgWarn(text: string) {
  const api = getMessage();
  if (api) defer(() => api.warning(text));
}
function msgInfo(text: string) {
  const api = getMessage();
  if (api) defer(() => api.info(text));
}
function notiOpen(message: string | number | undefined, description?: string) {
  const api = getNotification();
  if (api) defer(() => api.open({ message, description } as any));
}

type RestErrorPayload = {
  error?: string;
  message?: string;
  details?: Record<string, any>;
};

function resolveRestMessage(payload: RestErrorPayload | undefined, status?: number): string {
  const code = String(payload?.error || '')
    .trim()
    .toLowerCase();
  const rawMessage = String(payload?.message || '').trim();
  const zh: Record<string, string> = {
    unauthorized: '未授权',
    forbidden: '无权限',
    bad_request: '请求参数无效',
    validation_failed: '请求参数无效',
    internal_error: '服务器内部错误',
    not_found: '资源不存在',
    unavailable: '服务不可用',
    conflict: '资源冲突',
    rate_limited: '请求过于频繁',
    method_not_allowed: '方法不被允许',
    not_implemented: '未实现',
    bad_gateway: '上游服务错误',
    request_too_large: '请求体过大',
  };
  const fallbackByStatus: Record<number, string> = {
    400: '请求参数无效',
    401: '未授权',
    403: '无权限',
    404: '资源不存在',
    409: '资源冲突',
    422: '请求语义无效',
    500: '服务器内部错误',
  };
  if (rawMessage) return rawMessage;
  if (code && zh[code]) return zh[code];
  if (status && fallbackByStatus[status]) return fallbackByStatus[status];
  return '请求失败';
}

/**
 * @name 错误处理
 * pro 自带的错误处理， 可以在这里做自己的改动
 * @doc https://umijs.org/docs/max/request#配置
 */
export const errorConfig: RequestConfig = {
  // 错误处理： umi@3 的错误处理方案。
  errorConfig: {
    // 错误接收及处理
    errorHandler: (error: any, opts: any) => {
      if (opts?.skipErrorHandler) throw error;
      const rawUrl: string | undefined = error?.response?.config?.url || error?.request?.url;
      const url = normalizeApiUrl(rawUrl);
      const status: number | undefined = error?.response?.status;
      const payload = error?.response?.data as RestErrorPayload | undefined;
      const errorCode = String(payload?.error || '')
        .trim()
        .toLowerCase();
      const message = resolveRestMessage(payload, status);

      if (status === 403) {
        msgWarn(message);
        if (history.location?.pathname !== '/403') history.push('/403');
        return;
      }
      // Silence expected 401s during boot/login for profile + messages endpoints
      if (status === 401 && url) {
        if (
          url.includes(`${API_V1_PREFIX}/profile`) ||
          url.includes(`${API_V1_PREFIX}/users/current`) ||
          url.includes(`${API_V1_PREFIX}/messages`)
        ) {
          // 清除无效 token，静默跳转（不显示警告消息）
          try {
            localStorage.removeItem('token');
          } catch {}
          history.push('/user/login');
          return;
        }
        // token 失效：跳转登录
        try {
          localStorage.removeItem('token');
        } catch {}
        msgWarn(message);
        history.push('/user/login');
        return;
      }
      if (
        payload &&
        typeof payload === 'object' &&
        (payload.error || payload.message || payload.details)
      ) {
        if (errorCode === 'unauthorized') {
          try {
            localStorage.removeItem('token');
          } catch {}
          history.push('/user/login');
        } else if (errorCode === 'forbidden') {
          if (history.location?.pathname !== '/403') history.push('/403');
        }
        msgError(message);
        return;
      }
      // 兼容极少数遗留 success/errorCode/errorMessage 格式，后续可移除
      const legacyInfo = error?.info;
      if (error.name === 'BizError' && legacyInfo) {
        const legacyMessage = String(legacyInfo.errorMessage || '请求失败');
        if (legacyInfo.showType === 3) {
          notiOpen(String(legacyInfo.errorCode || ''), legacyMessage);
          return;
        }
        msgError(legacyMessage);
        return;
      }
      if (error.response) {
        // Axios 的错误
        // 请求成功发出且服务器也响应了状态码，但状态代码超出了 2xx 的范围
        msgError(resolveRestMessage(undefined, error.response.status));
      } else if (error.request) {
        // 请求已经成功发起，但没有收到响应
        // \`error.request\` 在浏览器中是 XMLHttpRequest 的实例，
        // 而在node.js中是 http.ClientRequest 的实例
        msgError('无响应，请稍后重试');
      } else {
        // 发送请求时出了点问题
        msgError('请求异常，请稍后重试');
      }
    },
  },

  // 请求拦截器
  requestInterceptors: [
    (config: RequestOptions) => {
      const headers = {
        ...(config.headers || {}),
      } as Record<string, any>;
      const isASCII = (s?: string | null) => !!s && /^[\x00-\x7F]*$/.test(s);
      const token = localStorage.getItem('token');
      if (token) headers['Authorization'] = `Bearer ${token}`;
      const gid = localStorage.getItem('game_id');
      const env = localStorage.getItem('env');
      // HTTP header values must be ASCII per XHR spec; skip if contains non-ASCII to avoid runtime error
      if (isASCII(gid)) headers['X-Game-ID'] = gid as string;
      if (isASCII(env)) headers['X-Env'] = env as string;
      const nextUrl = typeof config.url === 'string' ? normalizeApiUrl(config.url) : config.url;
      return { ...config, headers, url: nextUrl ?? config.url };
    },
  ],
};
