import { request } from '@umijs/max';

export type GameEnv = { env: string; description?: string; color?: string };

export async function listGameEnvs(gameId: number) {
  const res = await request<{ envs: GameEnv[] }>(`/api/games/${gameId}/envs`);
  return { envs: Array.isArray(res?.envs) ? res.envs : [] };
}

export async function addGameEnv(gameId: number, env: string, description?: string, color?: string) {
  return request<void>(`/api/games/${gameId}/envs`, { method: 'POST', data: { env, description, color } });
}

export async function updateGameEnv(gameId: number, oldEnv: string, env?: string, description?: string, color?: string) {
  // backend expects snake_case old_env
  return request<void>(`/api/games/${gameId}/envs`, { method: 'PUT', data: { old_env: oldEnv, env, description, color } });
}

export async function deleteGameEnv(gameId: number, params: { env: string }) {
  const qs = new URLSearchParams();
  if (params.env) qs.set('env', params.env);
  const q = qs.toString();
  return request<void>(`/api/games/${gameId}/envs?${q}`, { method: 'DELETE' });
}
