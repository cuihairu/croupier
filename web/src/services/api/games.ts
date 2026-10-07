import { getIntl, request } from '@umijs/max';

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
  description?: string;
  color?: string;
};

type RawGame = {
  id?: number;
  gameId?: string;
  name?: string;
  displayName?: string;
  aliasName?: string;
  gameName?: string;
  icon?: string;
  description?: string;
  homepage?: string;
  status?: string;
  enabled?: boolean;
  createdAt?: string;
  updatedAt?: string;
  color?: string;
  envs?: string[];
  envMeta?: RawGameEnvMeta[];
  gameType?: string;
  genreCode?: string;
};

const normalizeEnvMeta = (envs: RawGameEnvMeta[] | undefined): GameEnvMeta[] | undefined => {
  if (!Array.isArray(envs)) return undefined;
  return envs
    .map((env) => {
      const name = env?.env;
      if (!name) return undefined;
      return {
        env: name,
      } as GameEnvMeta;
    })
    .filter((env): env is GameEnvMeta => Boolean(env?.env));
};

function normalizeGame(raw: RawGame): Game {
  const name = raw?.gameId ?? raw?.name;
  const aliasName = raw?.aliasName ?? raw?.displayName ?? raw?.gameName;
  const envMeta = normalizeEnvMeta(raw?.envMeta);
  const envs =
    Array.isArray(raw?.envs) && raw.envs.length > 0
      ? raw.envs
      : Array.isArray(envMeta)
        ? envMeta.map((env) => env.env)
        : undefined;

  return {
    id: raw?.id,
    name,
    aliasName,
    icon: raw?.icon,
    envs,
    envMeta,
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
  game: Pick<Game, 'name' | 'aliasName' | 'description'> & { config?: string; icon?: string },
) {
  return request<{ game: Game } | void>('/api/v1/games', {
    method: 'POST',
    data: {
      name: game.name,
      aliasName: game.aliasName,
      icon: game.icon,
      description: game.description,
      config: game.config,
    },
  });
}

export async function getGame(id: number | string) {
  return request<{ game: Game }>(`/api/v1/games/${id}`);
}

export async function deleteGame(id: number) {
  return request<void>(`/api/v1/games/${id}`, { method: 'DELETE' });
}

export async function updateGame(
  id: number | string,
  game: Pick<Game, 'aliasName'> & { icon?: string },
) {
  return request<{ game: Game }>(`/api/v1/games/${id}`, {
    method: 'PUT',
    data: {
      aliasName: game.aliasName,
      // icon 显式提交：空串 = 清空（渲染回落默认骰子），后端指针语义
      icon: game.icon,
    },
  });
}

// Source: croupier/internal/api/game/icon_upload.go GameIconUploadResponse
export type GameIconUploadResult = {
  key: string;
  url: string;
};

/** 游戏图标上传大小上限；与后端 IconMaxBytes 保持一致。 */
export const GAME_ICON_MAX_BYTES = 2 * 1024 * 1024;

/**
 * 上传游戏图标（multipart 字段 file，POST /api/v1/games/icons）。
 *
 * 走原生 XHR 而非 request 封装：需要 upload.onprogress 汇报进度百分比
 * （同 storage.uploadObjectMultipart 先例）。成功返回内容寻址 key 与可
 * 直接回填 icon 字段的 URL。
 */
export function uploadGameIcon(
  file: File,
  onProgress?: (percent: number) => void,
): Promise<GameIconUploadResult> {
  return new Promise((resolve, reject) => {
    const form = new FormData();
    form.append('file', file);

    const xhr = new XMLHttpRequest();
    const token = typeof window !== 'undefined' ? localStorage.getItem('token') : '';

    xhr.open('POST', '/api/v1/games/icons');
    if (token) {
      xhr.setRequestHeader('Authorization', `Bearer ${token}`);
    }
    if (onProgress) {
      xhr.upload.onprogress = (event: ProgressEvent) => {
        if (event.lengthComputable) {
          onProgress(Math.round((event.loaded / event.total) * 100));
        }
      };
    }

    xhr.onload = () => {
      let payload: (GameIconUploadResult & { message?: string }) | null = null;
      try {
        payload = xhr.responseText
          ? (JSON.parse(xhr.responseText) as GameIconUploadResult & { message?: string })
          : null;
      } catch {
        reject(
          new Error(
            getIntl().formatMessage({
              id: 'services.games.iconUpload.parseFailed',
              defaultMessage: '上传响应解析失败',
            }),
          ),
        );
        return;
      }

      if (xhr.status >= 200 && xhr.status < 300) {
        if (payload?.key && payload?.url) {
          resolve({ key: payload.key, url: payload.url });
          return;
        }
        reject(
          new Error(
            getIntl().formatMessage({
              id: 'services.games.iconUpload.invalidResponse',
              defaultMessage: '上传响应缺少图标地址',
            }),
          ),
        );
        return;
      }

      reject(
        new Error(
          payload?.message ||
            getIntl().formatMessage({
              id: 'services.games.iconUpload.failed',
              defaultMessage: '图标上传失败',
            }),
        ),
      );
    };

    xhr.onerror = () =>
      reject(
        new Error(
          getIntl().formatMessage({
            id: 'services.games.iconUpload.failed',
            defaultMessage: '图标上传失败',
          }),
        ),
      );
    xhr.send(form);
  });
}
