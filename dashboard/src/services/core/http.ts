import { apiUrl } from '@/utils/api';

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

function getScopeValue(key: string) {
  if (typeof window === 'undefined') return undefined;
  try {
    return localStorage.getItem(key) || undefined;
  } catch {
    return undefined;
  }
}

function isASCII(s?: string | null) {
  return !!s && /^[\x00-\x7F]*$/.test(s);
}

function buildHeaders(init?: HeadersInit, opts?: { skipAuth?: boolean; skipScopeHeaders?: boolean }) {
  const headers = new Headers(init || {});
  if (!opts?.skipAuth) {
    const token = getToken();
    if (token && !headers.has('Authorization')) headers.set('Authorization', `Bearer ${token}`);
  }
  if (!opts?.skipScopeHeaders) {
    const gid = getScopeValue('game_id');
    const env = getScopeValue('env');
    if (isASCII(gid) && !headers.has('X-Game-ID')) headers.set('X-Game-ID', gid as string);
    if (isASCII(env) && !headers.has('X-Env')) headers.set('X-Env', env as string);
  }
  return headers;
}

export async function fetchJSON<T = any>(path: string, init: JsonInit = {}): Promise<T> {
  const url = apiUrl(path);
  const headers = buildHeaders(init.headers, {
    skipAuth: init.skipAuth,
    skipScopeHeaders: init.skipScopeHeaders,
  });
  if (!headers.has('Accept')) headers.set('Accept', 'application/json');
  if (!headers.has('Content-Type') && init.body && typeof init.body === 'string') {
    headers.set('Content-Type', 'application/json');
  }
  const resp = await fetch(url, { ...init, headers });
  if (!resp.ok) {
    const text = await resp.text().catch(() => '');
    const err = new Error(text || `Request failed: ${resp.status}`);
    (err as any).status = resp.status;
    (err as any).responseText = text;
    throw err;
  }
  if (resp.status === 204) return undefined as T;
  const data = (await resp.json().catch(() => undefined)) as T;
  return data;
}

export function createEventSource(
  path: string,
  opts?: { params?: Record<string, string | number | undefined>; attachToken?: boolean },
) {
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
  if (opts?.attachToken !== false) {
    const token = getToken();
    if (token) params.set('token', token);
  }
  urlObj.search = params.toString();
  return new EventSource(urlObj.toString());
}

export function buildDownloadUrl(path: string, params?: Record<string, string | number | undefined>) {
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
