import { request } from '@umijs/max';

type ApiResponse<T> = T | { data?: T } | { Data?: T } | null;

function unwrap<T>(resp: ApiResponse<T>): T {
  if (resp && typeof resp === 'object') {
    const anyResp = resp as any;
    if (anyResp.data) return anyResp.data as T;
    if (anyResp.Data) return anyResp.Data as T;
  }
  return resp as T;
}

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
  const resp = await request<ApiResponse<NodesListResponse>>('/api/v1/nodes', {
    method: 'GET',
    params,
    headers: token ? { Authorization: `Bearer ${token}` } : undefined,
  });
  return unwrap(resp);
}

/**
 * 获取节点元数据
 */
export async function getNodeMeta(id: string) {
  const token = typeof window !== 'undefined' ? localStorage.getItem('token') : '';
  const resp = await request<ApiResponse<{ meta: any }>>(`/api/v1/nodes/${id}/meta`, {
    method: 'GET',
    headers: token ? { Authorization: `Bearer ${token}` } : undefined,
  });
  return unwrap(resp);
}

/**
 * 更新节点元数据
 */
export async function updateNodeMeta(id: string, meta: any) {
  const token = typeof window !== 'undefined' ? localStorage.getItem('token') : '';
  const resp = await request<ApiResponse<{ meta: any }>>(`/api/v1/nodes/${id}/meta`, {
    method: 'PUT',
    data: { meta },
    headers: token ? { Authorization: `Bearer ${token}` } : undefined,
  });
  return unwrap(resp);
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
  const resp = await request<ApiResponse<NodeCommandsResponse>>('/api/v1/nodes/commands', {
    method: 'GET',
    headers: token ? { Authorization: `Bearer ${token}` } : undefined,
  });
  return unwrap(resp);
}
