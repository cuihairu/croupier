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

// 玩家类型定义
export interface Player {
  id: number;
  username: string;
  nickname: string;
  email: string;
  phone: string;
  gameId: string;
  status: number; // 1:active 0:banned 2:suspended
  balance: number; // 游戏货币
  level: number;
  vip: number;
  createdAt: string;
  updatedAt: string;
}

export interface PlayersListResponse {
  items: Player[];
  total: number;
  page: number;
  pageSize: number;
}

export interface PlayerCreateRequest {
  username: string;
  password: string;
  nickname?: string;
  email?: string;
  phone?: string;
  gameId: string;
}

export interface PlayerUpdateRequest {
  nickname?: string;
  email?: string;
  phone?: string;
  status?: number;
  level?: number;
  vip?: number;
}

export interface PlayerBalanceRequest {
  amount: number;
  reason: string;
}

// RESTful: 获取玩家列表
export async function listPlayers(params: {
  page?: number;
  pageSize?: number;
  gameId?: string;
  search?: string;
  status?: number;
  level?: number;
  vip?: number;
}) {
  const token = typeof window !== 'undefined' ? localStorage.getItem('token') : '';
  const resp = await request<ApiResponse<PlayersListResponse>>('/api/v1/players', {
    method: 'GET',
    params,
    headers: token ? { Authorization: `Bearer ${token}` } : undefined,
  });
  return unwrap(resp);
}

// RESTful: 创建玩家
export async function createPlayer(params: PlayerCreateRequest) {
  const token = typeof window !== 'undefined' ? localStorage.getItem('token') : '';
  const resp = await request<ApiResponse<Player>>('/api/v1/players', {
    method: 'POST',
    data: params,
    headers: token ? { Authorization: `Bearer ${token}` } : undefined,
  });
  return unwrap(resp);
}

// RESTful: 获取玩家详情
export async function getPlayer(id: string) {
  const token = typeof window !== 'undefined' ? localStorage.getItem('token') : '';
  const resp = await request<ApiResponse<Player>>(`/api/v1/players/${id}`, {
    method: 'GET',
    headers: token ? { Authorization: `Bearer ${token}` } : undefined,
  });
  return unwrap(resp);
}

// RESTful: 更新玩家信息
export async function updatePlayer(id: string, params: PlayerUpdateRequest) {
  const token = typeof window !== 'undefined' ? localStorage.getItem('token') : '';
  const resp = await request<ApiResponse<Player>>(`/api/v1/players/${id}`, {
    method: 'PUT',
    data: params,
    headers: token ? { Authorization: `Bearer ${token}` } : undefined,
  });
  return unwrap(resp);
}

// RESTful: 删除玩家
export async function deletePlayer(id: string) {
  const token = typeof window !== 'undefined' ? localStorage.getItem('token') : '';
  return request(`/api/v1/players/${id}`, {
    method: 'DELETE',
    headers: token ? { Authorization: `Bearer ${token}` } : undefined,
  });
}

// RESTful: 调整玩家余额
export async function adjustPlayerBalance(id: string, params: PlayerBalanceRequest) {
  const token = typeof window !== 'undefined' ? localStorage.getItem('token') : '';
  const resp = await request<ApiResponse<Player>>(`/api/v1/players/${id}/balance`, {
    method: 'POST',
    data: params,
    headers: token ? { Authorization: `Bearer ${token}` } : undefined,
  });
  return unwrap(resp);
}