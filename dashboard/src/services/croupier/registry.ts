import { request } from '@umijs/max';

export async function fetchRegistry() {
  return request<{ agents: any[]; functions: any[]; assignments?: Record<string, string[]>; coverage?: any[] }>('/api/v1/registry');
}

export type ServerAgent = {
  agent_id: string;
  game_id: string;
  env: string;
  rpc_addr: string;
  ip?: string;
  type?: string; // e.g., 'agent'
  version?: string;
  functions: number;
  healthy: boolean;
  expires_in_sec: number;
};
