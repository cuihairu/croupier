import { request } from '@umijs/max';

// Source: croupier/internal/api/game/dto.go GameEnvItem
export type GameEnvMeta = {
  env: string;
  description?: string;
  color?: string;
};

// Source: croupier/internal/api/game/dto.go GameInfo
export type Game = {
  id?: number;
  name?: string;
  displayName?: string;
  icon?: string;
  description?: string;
  aliasName?: string;
  homepage?: string;
  status?: string;
  enabled?: boolean;
  createdAt?: string;
  updatedAt?: string;
  color?: string;
  envs?: string[];
  envMeta?: GameEnvMeta[];
  gameType?: string;
  genreCode?: string;
};

type RawGameEnvMeta = {
  env?: string;
  Env?: string;
  description?: string;
  Description?: string;
  color?: string;
  Color?: string;
};

type RawGame = {
  id?: number;
  ID?: number;
  name?: string;
  Name?: string;
  displayName?: string;
  display_name?: string;
  aliasName?: string;
  alias_name?: string;
  AliasName?: string;
  gameName?: string;
  game_name?: string;
  icon?: string;
  Icon?: string;
  description?: string;
  Description?: string;
  homepage?: string;
  Homepage?: string;
  status?: string;
  Status?: string;
  enabled?: boolean;
  Enabled?: boolean;
  createdAt?: string;
  created_at?: string;
  CreatedAt?: string;
  updatedAt?: string;
  updated_at?: string;
  UpdatedAt?: string;
  color?: string;
  Color?: string;
  envs?: string[];
  envMeta?: RawGameEnvMeta[];
  env_meta?: RawGameEnvMeta[];
  gameType?: string;
  genreCode?: string;
};

const normalizeEnvMeta = (envs: RawGameEnvMeta[] | undefined): GameEnvMeta[] | undefined => {
  if (!Array.isArray(envs)) return undefined;
  return envs
    .map((env) => {
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

function normalizeGame(raw: RawGame): Game {
  const name = raw?.name ?? raw?.Name;
  const aliasName =
    raw?.aliasName ??
    raw?.alias_name ??
    raw?.AliasName ??
    raw?.displayName ??
    raw?.display_name ??
    raw?.gameName ??
    raw?.game_name;
  const envMeta = normalizeEnvMeta(raw?.envMeta ?? raw?.env_meta);
  const envs =
    Array.isArray(raw?.envs) && raw.envs.length > 0
      ? raw.envs
      : Array.isArray(envMeta)
      ? envMeta.map((env) => env.env)
      : undefined;

  return {
    id: raw?.id ?? raw?.ID,
    name,
    displayName: aliasName ?? name,
    icon: raw?.icon ?? raw?.Icon,
    description: raw?.description ?? raw?.Description,
    aliasName,
    homepage: raw?.homepage ?? raw?.Homepage,
    status: raw?.status ?? raw?.Status,
    enabled: typeof raw?.enabled === 'boolean' ? raw.enabled : raw?.Enabled,
    createdAt: raw?.createdAt ?? raw?.created_at ?? raw?.CreatedAt,
    updatedAt: raw?.updatedAt ?? raw?.updated_at ?? raw?.UpdatedAt,
    color: raw?.color ?? raw?.Color,
    envs,
    envMeta,
    gameType: raw?.gameType,
    genreCode: raw?.genreCode,
  };
}

export async function listGamesMeta() {
  const response = await request<{ games?: RawGame[] }>('/api/v1/games');
  return { games: Array.isArray(response?.games) ? response.games.map(normalizeGame) : [] };
}

export async function listMyGames() {
  const response = await request<{ games?: RawGame[] }>('/api/v1/profile/games');
  return { games: Array.isArray(response?.games) ? response.games.map(normalizeGame) : [] };
}

export async function upsertGame(
  game: Pick<Game, 'name' | 'aliasName' | 'description'> & { config?: string },
) {
  return request<{ game: Game } | void>('/api/v1/games', {
    method: 'POST',
    data: {
      name: game.name,
      aliasName: game.aliasName,
      description: game.description,
      config: game.config,
    },
  });
}

export async function deleteGame(id: number) {
  return request<void>(`/api/v1/games/${id}`, { method: 'DELETE' });
}

export async function updateGame(
  id: number,
  game: Pick<Game, 'name' | 'aliasName' | 'description'>,
) {
  return request<void>(`/api/v1/games/${id}`, {
    method: 'PUT',
    data: {
      name: game.name,
      aliasName: game.aliasName,
      description: game.description,
    },
  });
}
