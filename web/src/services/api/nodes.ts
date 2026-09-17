import { request } from '@umijs/max';

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
 * 获取节点元数据
 */
export async function getNodeMeta(id: string) {
  return request<{ meta: unknown }>(`/api/v1/nodes/${id}/meta`, {
    method: 'GET',
  });
}

/**
 * 更新节点元数据
 */
export async function updateNodeMeta(id: string, meta: unknown) {
  return request<{ meta: unknown }>(`/api/v1/nodes/${id}/meta`, {
    method: 'PUT',
    data: { meta },
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
