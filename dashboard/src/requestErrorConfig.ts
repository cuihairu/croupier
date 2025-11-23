import type { RequestOptions } from '@@/plugin-request/request';
import type { RequestConfig } from '@umijs/max';
import { history } from '@umijs/max';
// Use App.useApp() instances (see app.tsx) to avoid AntD static message warnings
import { getMessage, getNotification } from './utils/antdApp';
import { normalizeApiUrl, API_V1_PREFIX } from './utils/api';

// Defer message/notification to avoid calling during render (React 18 concurrent mode)
function defer(fn: () => void) {
  try { setTimeout(fn, 0); } catch { /* no-op */ }
}
function msgError(text: string) { const api = getMessage(); if (api) defer(() => api.error(text)); }
function msgWarn(text: string) { const api = getMessage(); if (api) defer(() => api.warning(text)); }
function msgInfo(text: string) { const api = getMessage(); if (api) defer(() => api.info(text)); }
function notiOpen(message: string | number | undefined, description?: string) {
  const api = getNotification();
  if (api) defer(() => api.open({ message, description } as any));
}

// 错误处理方案： 错误类型
enum ErrorShowType {
  SILENT = 0,
  WARN_MESSAGE = 1,
  ERROR_MESSAGE = 2,
  NOTIFICATION = 3,
  REDIRECT = 9,
}
// 与后端约定的响应数据格式
interface ResponseStructure {
  success: boolean;
  data: any;
  errorCode?: number;
  errorMessage?: string;
  showType?: ErrorShowType;
}

/**
 * @name 错误处理
 * pro 自带的错误处理， 可以在这里做自己的改动
 * @doc https://umijs.org/docs/max/request#配置
 */
export const errorConfig: RequestConfig = {
  // 错误处理： umi@3 的错误处理方案。
  errorConfig: {
    // 错误抛出：仅当明确有 success=false 时才抛出
    errorThrower: (res) => {
      const r = res as unknown as Partial<ResponseStructure>;
      if (r && Object.prototype.hasOwnProperty.call(r, 'success') && r.success === false) {
        const error: any = new Error(r.errorMessage || 'Request failed');
        error.name = 'BizError';
        error.info = { errorCode: r.errorCode, errorMessage: r.errorMessage, showType: r.showType, data: r.data };
        throw error;
      }
    },
    // 错误接收及处理
    errorHandler: (error: any, opts: any) => {
      if (opts?.skipErrorHandler) throw error;
      const rawUrl: string | undefined = error?.response?.config?.url || error?.request?.url;
      const url = normalizeApiUrl(rawUrl);
      const status: number | undefined = error?.response?.status;
      // Silence expected 401s during boot/login for current-user + messages endpoints
      if (status === 401 && url) {
        if (url.includes(`${API_V1_PREFIX}/users/current`) || url.includes(`${API_V1_PREFIX}/messages`)) {
          return;
        }
        // token 失效：跳转登录
        try { localStorage.removeItem('token'); } catch {}
        msgWarn('登录已过期，请重新登录');
        history.push('/user/login');
        return;
      }
      // 优先解析后端统一错误 { code, message, request_id }
      const payload = error?.response?.data as any;
      if (payload && typeof payload === 'object' && (payload.code || payload.message)) {
        const code = String(payload.code || '');
        let message = String(payload.message || '');
        const zh: Record<string,string> = {
          unauthorized:'未授权', forbidden:'无权限', bad_request:'请求参数无效', internal_error:'服务器内部错误',
          not_found:'资源不存在', unavailable:'服务不可用', conflict:'资源冲突', rate_limited:'请求过于频繁',
          method_not_allowed:'方法不被允许', not_implemented:'未实现', bad_gateway:'上游服务错误', request_too_large:'请求体过大',
        };
        const generic = new Set(['', 'unauthorized','forbidden','bad request','internal error','not found','service unavailable','conflict','too many login attempts','method not allowed','not implemented','bad gateway','request too large','invalid payload']);
        if (!message || generic.has(message.toLowerCase())) message = zh[code] || message || '请求失败';
        if (status === 401) msgWarn(message); else msgError(message);
        return;
      }
      // 我们的 errorThrower 抛出的错误。
      if (error.name === 'BizError') {
        const errorInfo: ResponseStructure | undefined = error.info;
        if (errorInfo) {
          const { errorMessage, errorCode } = errorInfo;
          switch (errorInfo.showType) {
            case ErrorShowType.SILENT:
              // do nothing
              break;
            case ErrorShowType.WARN_MESSAGE:
              msgWarn(errorMessage as any);
              break;
            case ErrorShowType.ERROR_MESSAGE:
              msgError(errorMessage as any);
              break;
            case ErrorShowType.NOTIFICATION:
              notiOpen(String(errorCode||''), errorMessage);
              break;
            case ErrorShowType.REDIRECT:
              // TODO: redirect
              break;
            default:
              msgError(errorMessage as any);
          }
        }
      } else if (error.response) {
        // Axios 的错误
        // 请求成功发出且服务器也响应了状态码，但状态代码超出了 2xx 的范围
        msgError(`响应错误：${error.response.status}`);
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

  // 响应拦截器
  responseInterceptors: [
    (response) => {
      // 拦截响应数据，进行个性化处理
      const { data } = response as unknown as ResponseStructure;

      if (data?.success === false) msgError('请求失败！');
      return response;
    },
  ],
};
