import { request } from '@umijs/max';

// Source: croupier/internal/api/config/dto.go ConfigItem / ListConfigsResponse
export type ConfigItem = {
  id: string;
  format: string;
  gameId: string;
  env: string;
  latestVersion: number;
  updatedAt: string;
  lastMessage: string;
  lastModifiedBy: string;
};

// Source: croupier/internal/api/config/dto.go GetConfigResponse
export type ConfigDetail = {
  id: string;
  format: string;
  content: string;
  version: number;
  gameId: string;
  env: string;
};

// Source: croupier/internal/api/config/dto.go ConfigVersionItem / ListVersionsResponse
export type ConfigVersion = {
  key: string;
  version: number;
  createdBy: string;
  createdAt: string;
  gameId: string;
  env: string;
  format: string;
  message: string;
  value: string;
};

// Source: croupier/internal/api/config/dto.go ValidateConfigResponse
export type ConfigValidationResult = {
  valid: boolean;
  errors: string[];
};

// Source: croupier/internal/api/config/dto.go SaveConfigRequest
export type SaveConfigInput = {
  gameId: string;
  env: string;
  format: string;
  content: string;
  message?: string;
  baseVersion?: number;
};

const BASE = '/api/v1/configs';

export async function listConfigs(params?: {
  gameId?: string;
  env?: string;
  format?: string;
  idLike?: string;
}) {
  return request<{ items: ConfigItem[] }>(BASE, { params });
}

export async function getConfig(id: string) {
  return request<ConfigDetail>(`${BASE}/${encodeURIComponent(id)}`);
}

export async function validateConfig(id: string, data: { format: string; content: string }) {
  return request<ConfigValidationResult>(`${BASE}/${encodeURIComponent(id)}/validate`, {
    method: 'POST',
    data,
  });
}

export async function saveConfig(id: string, data: SaveConfigInput) {
  return request<{ version: number }>(`${BASE}/${encodeURIComponent(id)}`, {
    method: 'PUT',
    data: {
      gameId: data.gameId,
      env: data.env,
      format: data.format,
      content: data.content,
      message: data.message || '',
      baseVersion: data.baseVersion || 0,
    },
  });
}

export async function listVersions(id: string) {
  return request<{ key: string; total: number; versions: ConfigVersion[] }>(
    `${BASE}/${encodeURIComponent(id)}/versions`,
  );
}

export async function getVersion(id: string, ver: number) {
  const response = await request<{ version: ConfigVersion }>(
    `${BASE}/${encodeURIComponent(id)}/versions/${ver}`,
  );
  return response.version;
}
