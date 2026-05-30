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
  resources?: any;
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
  const token = typeof window !== 'undefined' ? localStorage.getItem('token') : '';
  return request<NodesListResponse>('/api/v1/nodes', {
    method: 'GET',
    params,
    headers: token ? { Authorization: `Bearer ${token}` } : undefined,
  });
}

/**
 * 获取节点元数据
 */
export async function getNodeMeta(id: string) {
  const token = typeof window !== 'undefined' ? localStorage.getItem('token') : '';
  return request<{ meta: any }>(`/api/v1/nodes/${id}/meta`, {
    method: 'GET',
    headers: token ? { Authorization: `Bearer ${token}` } : undefined,
  });
}

/**
 * 更新节点元数据
 */
export async function updateNodeMeta(id: string, meta: any) {
  const token = typeof window !== 'undefined' ? localStorage.getItem('token') : '';
  return request<{ meta: any }>(`/api/v1/nodes/${id}/meta`, {
    method: 'PUT',
    data: { meta },
    headers: token ? { Authorization: `Bearer ${token}` } : undefined,
  });
}

/**
 * 排空节点
 */
export async function drainNode(id: string, timeout?: number) {
  const token = typeof window !== 'undefined' ? localStorage.getItem('token') : '';
  return request<void>(`/api/v1/nodes/${id}/drain`, {
    method: 'POST',
    data: { timeout },
    headers: token ? { Authorization: `Bearer ${token}` } : undefined,
  });
}

/**
 * 取消排空节点
 */
export async function undrainNode(id: string) {
  const token = typeof window !== 'undefined' ? localStorage.getItem('token') : '';
  return request<void>(`/api/v1/nodes/${id}/undrain`, {
    method: 'POST',
    headers: token ? { Authorization: `Bearer ${token}` } : undefined,
  });
}

/**
 * 重启节点
 */
export async function restartNode(id: string) {
  const token = typeof window !== 'undefined' ? localStorage.getItem('token') : '';
  return request<void>(`/api/v1/nodes/${id}/restart`, {
    method: 'POST',
    headers: token ? { Authorization: `Bearer ${token}` } : undefined,
  });
}

/**
 * 获取节点命令列表
 */
export async function getNodeCommands() {
  const token = typeof window !== 'undefined' ? localStorage.getItem('token') : '';
  return request<NodeCommandsResponse>('/api/v1/nodes/commands', {
    method: 'GET',
    headers: token ? { Authorization: `Bearer ${token}` } : undefined,
  });
}
