import { apiUrl } from '@/utils/api';
import { applyScopeHeaders, needsResolvedScope, waitForResolvedScope } from './scope';
import type { JSONValue } from '@/types/dashboard';

interface HttpError extends Error {
  status?: number;
  responseText?: string;
}

type JsonInit = RequestInit & {
  skipAuth?: boolean;
  skipScopeHeaders?: boolean;
};

function getToken() {
  if (typeof window === 'undefined') return undefined;
  try {
    return localStorage.getItem('token') || undefined;
  } catch {
    return undefined;
  }
}

function buildHeaders(
  init?: HeadersInit,
  opts?: { skipAuth?: boolean; includeScopeHeaders?: boolean },
) {
  const headers = new Headers(init || {});
  if (!opts?.skipAuth) {
    const token = getToken();
    if (token && !headers.has('Authorization')) headers.set('Authorization', `Bearer ${token}`);
  }
  if (opts?.includeScopeHeaders) {
    applyScopeHeaders(headers);
  }
  return headers;
}

export async function fetchJSON<T = JSONValue>(path: string, init: JsonInit = {}): Promise<T> {
  const url = apiUrl(path);
  const includeScopeHeaders = !init.skipScopeHeaders && needsResolvedScope(url);
  if (includeScopeHeaders) {
    await waitForResolvedScope(url);
  }
  const headers = buildHeaders(init.headers, {
    skipAuth: init.skipAuth,
    includeScopeHeaders,
  });
  if (!headers.has('Accept')) headers.set('Accept', 'application/json');
  if (!headers.has('Content-Type') && init.body && typeof init.body === 'string') {
    headers.set('Content-Type', 'application/json');
  }
  const resp = await fetch(url, { ...init, headers });
  if (!resp.ok) {
    const text = await resp.text().catch(() => '');
    const err = new Error(text || `Request failed: ${resp.status}`) as HttpError;
    err.status = resp.status;
    err.responseText = text;
    throw err;
  }
  if (resp.status === 204) return undefined as T;
  const data = (await resp.json().catch(() => undefined)) as T;
  return data;
}

/** SSE 流最小消费面：与 DOM EventSource 的用法子集结构兼容
 *（addEventListener / removeEventListener / close / onopen）。C-004：
 * 经 fetch 携带 Authorization 头，JWT 不再进 URL/日志/Referer。 */
export interface SSEStream {
  addEventListener(type: string, listener: (event: MessageEvent) => void): void;
  removeEventListener(type: string, listener: (event: MessageEvent) => void): void;
  close(): void;
  onopen: (() => void) | null;
  onerror: (() => void) | null;
}

type SSEListener = (event: MessageEvent) => void;

const SSE_RECONNECT_DELAY_MS = 3000;

/** 以 fetch 实现的 SSE 客户端：按 text/event-stream 帧解析（event / data /
 * retry 字段、空行分帧、`: ` 注释心跳）。连接被服务端关闭时自动重连；
 * HTTP 非 200 为致命（与 EventSource 一致），报错后停止重连。 */
class FetchSSEStream implements SSEStream {
  private readonly listeners = new Map<string, Set<SSEListener>>();
  private controller: AbortController | null = null;
  private closed = false;
  private retryMs = SSE_RECONNECT_DELAY_MS;
  onopen: (() => void) | null = null;
  onerror: (() => void) | null = null;

  constructor(
    private readonly url: string,
    private readonly headers: Headers,
  ) {
    void this.connect();
  }

  addEventListener(type: string, listener: SSEListener): void {
    let set = this.listeners.get(type);
    if (!set) {
      set = new Set<SSEListener>();
      this.listeners.set(type, set);
    }
    set.add(listener);
  }

  removeEventListener(type: string, listener: SSEListener): void {
    this.listeners.get(type)?.delete(listener);
  }

  close(): void {
    this.closed = true;
    this.controller?.abort();
  }

  private emit(type: string, data: string): void {
    // 监听方契约只读 data（消费侧按 MessageEvent 注解）；帧对象满足该面。
    const event = { data } as unknown as MessageEvent;
    for (const listener of this.listeners.get(type) ?? []) listener(event);
  }

  private async connect(): Promise<void> {
    for (;;) {
      if (this.closed) return;
      this.controller = new AbortController();
      try {
        const resp = await fetch(this.url, {
          headers: this.headers,
          signal: this.controller.signal,
        });
        if (resp.status !== 200) {
          // 非 200 视为致命：报错后不再重连（EventSource 语义）。
          this.fireError();
          return;
        }
        this.onopen?.();
        this.emit('open', '');
        await this.readStream(resp);
      } catch {
        // 网络错误或 close() 主动中止：统一走重连/退出判断。
      }
      if (this.closed) return;
      this.fireError();
      await this.waitRetry();
    }
  }

  private fireError(): void {
    this.onerror?.();
    this.emit('error', '');
  }

  private async waitRetry(): Promise<void> {
    await new Promise<void>((resolve) => {
      const timer = setTimeout(resolve, this.retryMs);
      this.controller?.signal.addEventListener(
        'abort',
        () => {
          clearTimeout(timer);
          resolve();
        },
        { once: true },
      );
    });
  }

  private async readStream(resp: Response): Promise<void> {
    const reader = resp.body?.getReader();
    if (!reader) return;
    const decoder = new TextDecoder();
    let buffer = '';
    for (;;) {
      const { done, value } = await reader.read();
      if (done) return;
      buffer += decoder.decode(value, { stream: true });
      // 归一化行尾：SSE 规范允许 CRLF / LF / CR 三种行分隔。服务端可能
      // 混用（如 \r\n\n 或纯 \r），统一折成 \n 后按空行 \n\n 分帧，避免
      // 事件延迟/粘连/漏解析。
      buffer = buffer.replace(/\r\n/g, '\n').replace(/\r/g, '\n');
      const frames = buffer.split('\n\n');
      buffer = frames.pop() ?? '';
      for (const frame of frames) this.dispatchFrame(frame);
    }
  }

  private dispatchFrame(frame: string): void {
    let event = 'message';
    const data: string[] = [];
    for (const line of frame.split('\n')) {
      if (!line || line.startsWith(':')) continue; // 空行 / 注释心跳
      const colon = line.indexOf(':');
      const field = colon === -1 ? line : line.slice(0, colon);
      let value = colon === -1 ? '' : line.slice(colon + 1);
      if (value.startsWith(' ')) value = value.slice(1);
      if (field === 'event') event = value;
      else if (field === 'data') data.push(value);
      else if (field === 'retry') {
        const ms = Number(value);
        if (Number.isFinite(ms) && ms > 0) this.retryMs = ms;
      }
    }
    if (data.length > 0) this.emit(event, data.join('\n'));
  }
}

export function createEventSource(
  path: string,
  opts?: { params?: Record<string, string | number | undefined>; attachToken?: boolean },
): SSEStream {
  const origin =
    typeof window !== 'undefined' && window.location ? window.location.origin : 'http://localhost';
  const urlObj = new URL(apiUrl(path), origin);
  const params = new URLSearchParams(urlObj.search || '');
  if (opts?.params) {
    Object.entries(opts.params).forEach(([k, v]) => {
      if (v === undefined || v === null) return;
      params.set(k, String(v));
    });
  }
  urlObj.search = params.toString();
  // C-004：EventSource 无法自定义 header，改走 fetch 流——JWT 经
  // Authorization: Bearer 头传递，token 不再出现在 URL/日志/Referer。
  if (typeof fetch !== 'function') {
    throw new Error('fetch is not available for SSE stream');
  }
  const headers = new Headers();
  if (opts?.attachToken !== false) {
    const token = getToken();
    if (token) headers.set('Authorization', `Bearer ${token}`);
  }
  return new FetchSSEStream(urlObj.toString(), headers);
}

export function buildDownloadUrl(
  path: string,
  params?: Record<string, string | number | undefined>,
) {
  const origin =
    typeof window !== 'undefined' && window.location ? window.location.origin : 'http://localhost';
  const urlObj = new URL(apiUrl(path), origin);
  if (params) {
    const search = new URLSearchParams(urlObj.search || '');
    Object.entries(params).forEach(([k, v]) => {
      if (v === undefined || v === null) return;
      search.set(k, String(v));
    });
    urlObj.search = search.toString();
  }
  return urlObj.toString();
}
