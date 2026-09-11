import type { RequestOptions } from '@@/plugin-request/request';
import type { RequestConfig } from '@umijs/max';
import { getIntl, history } from '@umijs/max';
import { createElement } from 'react';
// Use App.useApp() instances (see app.tsx) to avoid AntD static message warnings
import { getMessage, getNotification } from './utils/antdApp';
import { normalizeApiUrl, API_V1_PREFIX } from './utils/api';
import { extractErrorDetails } from './utils/errors';
import { getScopeHeaders, needsResolvedScope, waitForResolvedScope } from './services/core/scope';
import { setScope } from './stores/scope';
import type { JSONValue } from '@/types/dashboard';

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
function notiOpen(message: string | number | undefined, description?: string) {
  const api = getNotification();
  if (api) defer(() => api.open({ message: String(message ?? ''), description }));
}

function notiError(title: string, details: Array<{ field: string; message: string }>) {
  const api = getNotification();
  if (!api) return false;
  const items = details
    .slice(0, 20)
    .map((d, i) =>
      createElement(
        'li',
        { key: `${d.field}-${i}` },
        d.field ? createElement('code', null, d.field, ': ') : null,
        d.message,
      ),
    );
  if (details.length > 20) {
    items.push(
      createElement(
        'li',
        { key: 'more' },
        getIntl().formatMessage(
          { id: 'app.request.error.moreDetails', defaultMessage: '…以及另外 {count} 条' },
          { count: details.length - 20 },
        ),
      ),
    );
  }
  const list = createElement(
    'ul',
    { style: { margin: 0, paddingLeft: 18, maxHeight: 240, overflowY: 'auto' } },
    items,
  );
  defer(() => {
    // duration 0 = 不自动关闭，用户需逐条阅读结构化明细
    api.error({ message: title, description: list, duration: 0 });
  });
  return true;
}

type RestErrorPayload = {
  error?: string;
  message?: string;
  details?: Record<string, JSONValue>;
};

function resolveRestMessage(payload: RestErrorPayload | undefined, status?: number): string {
  const intl = getIntl();
  const code = String(payload?.error || '')
    .trim()
    .toLowerCase();
  const rawMessage = String(payload?.message || '').trim();
  // key 为后端稳定错误码（枚举契约），value 为展示文案
  const codeMessages: Record<string, string> = {
    unauthorized: intl.formatMessage({
      id: 'app.request.error.unauthorized',
      defaultMessage: '未授权',
    }),
    forbidden: intl.formatMessage({ id: 'app.request.error.forbidden', defaultMessage: '无权限' }),
    bad_request: intl.formatMessage({
      id: 'app.request.error.invalidParams',
      defaultMessage: '请求参数无效',
    }),
    validation_failed: intl.formatMessage({
      id: 'app.request.error.invalidParams',
      defaultMessage: '请求参数无效',
    }),
    internal_error: intl.formatMessage({
      id: 'app.request.error.internalError',
      defaultMessage: '服务器内部错误',
    }),
    not_found: intl.formatMessage({
      id: 'app.request.error.notFound',
      defaultMessage: '资源不存在',
    }),
    // 后端稳定码为 service_unavailable（HTTP 503），无 agent 可达/转发
    // 耗尽都归此类——保持键与后端码一致，否则 503 永远落到 default 文案
    service_unavailable: intl.formatMessage({
      id: 'app.request.error.unavailable',
      defaultMessage: '服务不可用',
    }),
    conflict: intl.formatMessage({
      id: 'app.request.error.conflict',
      defaultMessage: '资源冲突',
    }),
    rate_limited: intl.formatMessage({
      id: 'app.request.error.rateLimited',
      defaultMessage: '请求过于频繁',
    }),
    method_not_allowed: intl.formatMessage({
      id: 'app.request.error.methodNotAllowed',
      defaultMessage: '方法不被允许',
    }),
    not_implemented: intl.formatMessage({
      id: 'app.request.error.notImplemented',
      defaultMessage: '未实现',
    }),
    bad_gateway: intl.formatMessage({
      id: 'app.request.error.badGateway',
      defaultMessage: '上游服务错误',
    }),
    request_too_large: intl.formatMessage({
      id: 'app.request.error.requestTooLarge',
      defaultMessage: '请求体过大',
    }),
  };
  const fallbackByStatus: Record<number, string> = {
    400: intl.formatMessage({
      id: 'app.request.error.invalidParams',
      defaultMessage: '请求参数无效',
    }),
    401: intl.formatMessage({ id: 'app.request.error.unauthorized', defaultMessage: '未授权' }),
    403: intl.formatMessage({ id: 'app.request.error.forbidden', defaultMessage: '无权限' }),
    404: intl.formatMessage({
      id: 'app.request.error.notFound',
      defaultMessage: '资源不存在',
    }),
    409: intl.formatMessage({ id: 'app.request.error.conflict', defaultMessage: '资源冲突' }),
    422: intl.formatMessage({
      id: 'app.request.error.unprocessable',
      defaultMessage: '请求语义无效',
    }),
    500: intl.formatMessage({
      id: 'app.request.error.internalError',
      defaultMessage: '服务器内部错误',
    }),
    503: intl.formatMessage({
      id: 'app.request.error.unavailable',
      defaultMessage: '服务不可用',
    }),
  };
  if (rawMessage) return rawMessage;
  if (code && codeMessages[code]) return codeMessages[code];
  if (status && fallbackByStatus[status]) return fallbackByStatus[status];
  return getIntl().formatMessage({ id: 'app.request.error.default', defaultMessage: '请求失败' });
}

/**
 * @name 错误处理
 * pro 自带的错误处理， 可以在这里做自己的改动
 * @doc https://umijs.org/docs/max/request#配置
 */
interface ErrorHandlerOptions {
  skipErrorHandler?: boolean;
  [key: string]: string | number | boolean | undefined;
}

interface RequestError extends Error {
  response?: {
    status?: number;
    config?: { url?: string };
    data?: RestErrorPayload;
  };
  request?: {
    url?: string;
  };
  info?: {
    errorMessage?: string;
    errorCode?: string;
    showType?: number;
  };
}

export const errorConfig: RequestConfig = {
  // 错误处理： umi@3 的错误处理方案。
  errorConfig: {
    // 错误接收及处理
    errorHandler: (error: Error, opts: ErrorHandlerOptions) => {
      if (opts?.skipErrorHandler) throw error;
      const reqError = error as RequestError;
      const rawUrl: string | undefined = reqError?.response?.config?.url || reqError?.request?.url;
      const url = normalizeApiUrl(rawUrl);
      const status: number | undefined = reqError?.response?.status;
      const payload = reqError?.response?.data as RestErrorPayload | undefined;
      const errorCode = String(payload?.error || '')
        .trim()
        .toLowerCase();
      const message = resolveRestMessage(payload, status);

      if (status === 403) {
        // Scope 相关的 403 不应跳转到 /403 页面，而是清除无效 scope 让 GameSelector 重新选择
        if (
          errorCode === 'scope_not_authorized' ||
          errorCode === 'invalid_game_scope' ||
          errorCode === 'scope_required'
        ) {
          // 同时清除内存和持久化的 scope
          setScope({ gameId: undefined, env: undefined }, { persist: true, emit: true });
          msgWarn(
            getIntl().formatMessage({
              id: 'app.request.error.invalidScope',
              defaultMessage: '当前选择的游戏环境无效，请重新选择',
            }),
          );
          return;
        }
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
        // 结构化校验明细（details）用常驻 notification 展示，方便用户逐条定位
        const details = extractErrorDetails(reqError);
        if (details.length > 0 && notiError(message, details)) return;
        msgError(message);
        return;
      }
      // 兼容极少数遗留 success/errorCode/errorMessage 格式，后续可移除
      const legacyInfo = reqError?.info;
      if (error.name === 'BizError' && legacyInfo) {
        const legacyMessage = String(
          legacyInfo.errorMessage ||
            getIntl().formatMessage({
              id: 'app.request.error.default',
              defaultMessage: '请求失败',
            }),
        );
        if (legacyInfo.showType === 3) {
          notiOpen(String(legacyInfo.errorCode || ''), legacyMessage);
          return;
        }
        msgError(legacyMessage);
        return;
      }
      if (reqError?.response) {
        // Axios 的错误
        // 请求成功发出且服务器也响应了状态码，但状态代码超出了 2xx 的范围
        msgError(resolveRestMessage(undefined, reqError.response.status));
      } else if (reqError?.request) {
        // 请求已经成功发起，但没有收到响应
        // `error.request` 在浏览器中是 XMLHttpRequest 的实例，
        // 而在node.js中是 http.ClientRequest 的实例
        msgError(
          getIntl().formatMessage({
            id: 'app.request.error.noResponse',
            defaultMessage: '无响应，请稍后重试',
          }),
        );
      } else {
        // 发送请求时出了点问题
        msgError(
          getIntl().formatMessage({
            id: 'app.request.error.exception',
            defaultMessage: '请求异常，请稍后重试',
          }),
        );
      }
    },
  },

  // 请求拦截器
  requestInterceptors: [
    async (config: RequestOptions) => {
      const nextUrl = typeof config.url === 'string' ? normalizeApiUrl(config.url) : config.url;
      // /profile/games is intentionally outside the scoped paths: GameSelector
      // uses it to validate scope, so waiting here would deadlock startup.
      await waitForResolvedScope(nextUrl);
      const headers = { ...(config.headers || {}) } as Record<string, string>;
      const token = localStorage.getItem('token');
      if (token) headers.Authorization = `Bearer ${token}`;
      if (needsResolvedScope(nextUrl)) {
        Object.keys(headers).forEach((key) => {
          if (key.toLowerCase() === 'x-game-id' || key.toLowerCase() === 'x-env')
            delete headers[key];
        });
        const scope = getScopeHeaders();
        if (scope) {
          headers['X-Game-ID'] = scope.gameID;
          headers['X-Env'] = scope.env;
        }
      }
      return { ...config, headers, url: nextUrl ?? config.url };
    },
  ],
};
