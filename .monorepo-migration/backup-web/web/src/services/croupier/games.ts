import { request } from '@umijs/max';

export type Game = {
  id?: number;
  name?: string;
  icon?: string;         // URL
  description?: string;
  alias_name?: string;
  homepage?: string;
  status?: string;       // dev|test|running|online|offline|maintenance
  enabled?: boolean;
  created_at?: string;
  updated_at?: string;
  envs?: string[];
};

// Normalize server payload shape to our frontend's camel/snake case keys.
function normalizeGame(g: any): Game {
  return {
    id: g?.id ?? g?.ID,
    name: g?.name ?? g?.Name,
    icon: g?.icon ?? g?.Icon,
    description: g?.description ?? g?.Description,
    alias_name: g?.alias_name ?? g?.AliasName,
    homepage: g?.homepage ?? g?.Homepage,
    status: g?.status ?? g?.Status,
    created_at: g?.created_at ?? g?.CreatedAt,
    updated_at: g?.updated_at ?? g?.UpdatedAt,
    enabled: (g?.enabled ?? g?.Enabled) as any,
    envs: Array.isArray(g?.envs) ? g.envs : undefined,
  } as Game;
}

export async function listGamesMeta() {
  const res = await request<any>('/api/games');
  const games = Array.isArray(res?.games) ? res.games.map(normalizeGame) : [];
  return { games } as { games: Game[] };
}

// Only games allowed by current user's scope (empty scope => all games)
export async function listMyGames() {
  const res = await request<any>('/api/me/games');
  const games = Array.isArray(res?.games)
    ? res.games.map((g: any) => ({ ...normalizeGame(g), envs: Array.isArray(g?.envs) ? g.envs : [] }))
    : [];
  return { games } as { games: Game[] };
}

export async function upsertGame(g: Game) {
  // POST /api/games: id==0/undefined -> create; else update
  return request<{ id: number } | void>('/api/games', { method: 'POST', data: g });
}

export async function deleteGame(id: number) {
  return request<void>(`/api/games/${id}`, { method: 'DELETE' });
}
