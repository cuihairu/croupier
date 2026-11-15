import { request } from '@umijs/max';
import { createEventSource, type SSEStream } from '../core/http';

// Source: croupier/internal/api/ops/devops_sessions.go（AGENT-002 契约：面板经
// server 代理开/输入/调整/关 SSH 会话，输出经 SSE 流回面板；relay 未接线时
// 服务端如实报 503 devops_relay_not_configured）。scope 头（X-Game-ID/X-Env）
// 由全局请求拦截器注入，与其他 ops 端点同口径。
const SESSIONS_PATH = '/api/v1/ops/devops/sessions';

/** 列表条目（DevopsSessionItem）。 */
export type DevopsSessionItem = {
  sessionId: string;
  agentId: string;
  operator: string;
  protocol: string;
  host: string;
  port: number;
  channel?: string;
  createdAt: string;
};

export type DevopsSessionListResponse = { sessions?: DevopsSessionItem[] };

/** 开会话入参（DevopsSessionOpenRequest）：protocol 本批只交付 ssh。 */
export type DevopsSessionOpenRequest = {
  agentId: string;
  protocol: string;
  host: string;
  port: number;
  cols?: number;
  rows?: number;
};

/** 开会话结果（DevopsSessionOpenResponse）。 */
export type DevopsSessionOpenResponse = {
  sessionId: string;
  agentId: string;
  protocol: string;
  host: string;
  port: number;
  channel?: string;
};

/** 抽屉头部/终端所需的最小会话面（列表条目与 open 响应都满足）。 */
export type DevopsSessionActive = Pick<
  DevopsSessionItem,
  'sessionId' | 'agentId' | 'protocol' | 'host' | 'port'
>;

/** SSE 帧载荷（devopsSSEPayload）：data 为 base64 编码的 pty 字节流。 */
export type DevopsSessionSSEPayload = {
  sessionId?: string;
  kind?: string;
  data?: string;
  outcome?: string;
};

export async function openDevopsSession(requestBody: DevopsSessionOpenRequest) {
  return request<DevopsSessionOpenResponse>(SESSIONS_PATH, {
    method: 'POST',
    data: requestBody,
  });
}

export async function listDevopsSessions() {
  const response = await request<DevopsSessionListResponse>(SESSIONS_PATH);
  return response?.sessions || [];
}

/** 键盘数据下行。429 session_busy 为瞬态流控，不走全局 toast：skipErrorHandler。 */
export async function sendDevopsSessionInput(sessionId: string, data: string) {
  return request<void>(`${SESSIONS_PATH}/${encodeURIComponent(sessionId)}/input`, {
    method: 'POST',
    data: { data },
    skipErrorHandler: true,
  });
}

/** 窗口调整下行。瞬态失败只记日志，不弹窗。 */
export async function resizeDevopsSession(sessionId: string, cols: number, rows: number) {
  return request<void>(`${SESSIONS_PATH}/${encodeURIComponent(sessionId)}/resize`, {
    method: 'POST',
    data: { cols, rows },
    skipErrorHandler: true,
  });
}

/** 主动关闭（服务端留痕 devops.session_close，reason 默认 "panel close"）。 */
export async function closeDevopsSession(sessionId: string, reason = 'panel close') {
  return request<void>(`${SESSIONS_PATH}/${encodeURIComponent(sessionId)}`, {
    method: 'DELETE',
    data: { reason },
  });
}

/** 会话输出流：SSE，接入即先补服务端回放环（最近一屏），再接实时帧。 */
export function openDevopsSessionStream(sessionId: string): SSEStream {
  return createEventSource(`${SESSIONS_PATH}/${encodeURIComponent(sessionId)}/stream`);
}
