import { request } from '@umijs/max';
import type { NodeCronJob } from './ops';

// ============================================================================
// 类型定义
// ============================================================================

export interface Node {
  id: string;
  name: string;
  type: string; // server, agent, edge
  status: string;
  ip: string;
  port: number;
  resources?: unknown;
  updatedAt: string;
}

export interface NodesListParams {
  type?: string;
  status?: string;
}

export interface NodesListResponse {
  items: Node[];
}

export interface NodeCommand {
  name: string;
  description: string;
}

export interface NodeCommandsResponse {
  items: NodeCommand[];
}

// ============================================================================
// API 函数
// ============================================================================

/**
 * 获取节点列表
 */
export async function listNodes(params?: NodesListParams) {
  return request<NodesListResponse>('/api/v1/nodes', {
    method: 'GET',
    params,
  });
}

/**
 * 排空节点
 */
export async function drainNode(id: string, timeout?: number) {
  return request<void>(`/api/v1/nodes/${id}/drain`, {
    method: 'POST',
    data: { timeout },
  });
}

/**
 * 取消排空节点
 */
export async function undrainNode(id: string) {
  return request<void>(`/api/v1/nodes/${id}/undrain`, {
    method: 'POST',
  });
}

/**
 * 重启节点
 */
export async function restartNode(id: string) {
  return request<void>(`/api/v1/nodes/${id}/restart`, {
    method: 'POST',
  });
}

/**
 * 获取节点命令列表
 */
export async function getNodeCommands() {
  return request<NodeCommandsResponse>('/api/v1/nodes/commands', {
    method: 'GET',
  });
}

// ============================================================================
// 宿主机定时任务聚合（#24：/ops/schedules 来源分组展示）
// ============================================================================

/** 单条宿主机定时任务复用 ops.ts 的 NodeCronJob（agent 采集：crontab / 计划任务） */

/** 单节点的宿主机任务采集报告；不可达节点 ok=false 并携带原因 */
export interface NodeCronJobsReport {
  nodeId: string;
  nodeName: string;
  status: string;
  ok: boolean;
  error?: string;
  jobs: NodeCronJob[];
}

export interface NodeCronJobsAllResponse {
  items: NodeCronJobsReport[];
  total: number;
}

/**
 * 聚合全部节点的宿主机定时任务（GET /api/v1/nodes/cron-jobs）。
 * 服务端并行经 agent 会话采集并缓存 30s；离线节点不中断整体。
 */
export async function listNodesCronJobsAll(): Promise<NodeCronJobsAllResponse> {
  return request<NodeCronJobsAllResponse>('/api/v1/nodes/cron-jobs', {
    method: 'GET',
  });
}
