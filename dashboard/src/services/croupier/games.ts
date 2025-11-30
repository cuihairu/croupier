import { request } from "@umijs/max";

export type GameEnvMeta = {
  env: string;
  description?: string;
  color?: string;
};

export type Game = {
  id?: number;
  name?: string; // ASCII-safe scope id
  display_name?: string;
  icon?: string;
  description?: string;
  alias_name?: string;
  homepage?: string;
  status?: string;
  enabled?: boolean;
  created_at?: string;
  updated_at?: string;
  color?: string;
  envs?: string[];
  envMeta?: GameEnvMeta[];
};

const normalizeEnvMeta = (envs: any): GameEnvMeta[] | undefined => {
  if (!Array.isArray(envs)) return undefined;
  return envs
    .map((env: any) => {
      const name = env?.env ?? env?.Env;
      if (!name) return undefined;
      return {
        env: name,
        description: env?.description ?? env?.Description,
        color: env?.color ?? env?.Color,
      } as GameEnvMeta;
    })
    .filter((env): env is GameEnvMeta => Boolean(env?.env));
};

// Normalize server payload shape to our frontend's camel/snake case keys.
function normalizeGame(g: any): Game {
  const slug = g?.gameId ?? g?.game_id ?? g?.name ?? g?.Name;
  const alias = g?.alias_name ?? g?.AliasName ?? g?.gameName ?? g?.game_name;
  const normalized: Game = {
    id: g?.id ?? g?.ID,
    name: slug,
    display_name: alias ?? slug,
    icon: g?.icon ?? g?.Icon,
    description: g?.description ?? g?.Description,
    alias_name: alias,
    homepage: g?.homepage ?? g?.Homepage,
    status: g?.status ?? g?.Status,
    created_at: g?.created_at ?? g?.CreatedAt,
    updated_at: g?.updated_at ?? g?.UpdatedAt,
    enabled:
      typeof g?.enabled === "boolean" ? g.enabled : Boolean(g?.Enabled ?? true),
    color: g?.color ?? g?.Color,
    envs: Array.isArray(g?.envs) ? g.envs : undefined,
    envMeta: normalizeEnvMeta(g?.envMeta ?? g?.env_meta),
  };
  if (
    (!normalized.envs || normalized.envs.length === 0) &&
    Array.isArray(normalized.envMeta)
  ) {
    normalized.envs = normalized.envMeta.map((env) => env.env);
  }
  return normalized;
}

export async function listGamesMeta() {
  const res = await request<any>("/api/v1/games");
  const games = Array.isArray(res?.games) ? res.games.map(normalizeGame) : [];
  return { games } as { games: Game[] };
}

// Only games allowed by current user's scope (empty scope => all games)
export async function listMyGames() {
  const res = await request<any>("/api/v1/profile/games");
  const payload = res?.games ?? res?.data?.games;
  const games = Array.isArray(payload) ? payload.map(normalizeGame) : [];
  return { games } as { games: Game[] };
}

export async function upsertGame(g: Game) {
  // POST /api/v1/games: id==0/undefined -> create; else update
  return request<{ id: number } | void>("/api/v1/games", {
    method: "POST",
    data: g,
  });
}

export async function deleteGame(id: number) {
  return request<void>(`/api/v1/games/${id}`, { method: "DELETE" });
}
