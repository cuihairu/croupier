import { request } from '@umijs/max';

export type GameEnv = { env: string; description?: string; color?: string };

interface RawGameEnv {
  env?: string;
  Env?: string;
  description?: string;
  Description?: string;
  color?: string;
  Color?: string;
}

export async function listGameEnvs(gameId: number) {
  const res = await request<{ envs?: RawGameEnv[] }>(`/api/v1/games/${gameId}/envs`);
  const payload = res?.envs;
  const envs = Array.isArray(payload)
    ? payload.map((item: RawGameEnv) => ({
        env: String(item?.env ?? item?.Env ?? ''),
        description: String(item?.description ?? item?.Description ?? ''),
        color: String(item?.color ?? item?.Color ?? ''),
      }))
    : [];
  return { envs };
}

export async function addGameEnv(
  gameId: number,
  env: string,
  description?: string,
  color?: string,
) {
  return request<void>(`/api/v1/games/${gameId}/envs`, {
    method: 'POST',
    data: { name: env, type: description || color || '' },
  });
}

export async function updateGameEnv(
  gameId: number,
  oldEnv: string,
  env?: string,
  description?: string,
  color?: string,
) {
  return request<void>(`/api/v1/games/${gameId}/envs/${encodeURIComponent(oldEnv)}`, {
    method: 'PUT',
    data: { name: env, type: description || color || '' },
  });
}

export async function deleteGameEnv(gameId: number, params: { env: string }) {
  return request<void>(`/api/v1/games/${gameId}/envs/${encodeURIComponent(params.env)}`, {
    method: 'DELETE',
  });
}
