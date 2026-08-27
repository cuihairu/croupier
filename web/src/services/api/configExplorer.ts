import { request } from '@umijs/max';

// Source: croupier/internal/api/configexplorer/dto.go BindingDTO
export type ConfigSourceBinding = {
  id: number;
  gameId: string;
  env: string;
  name: string;
  type: 'git' | 'redis' | 'nacos' | 'db' | 'croupier';
  config: string;
  writable: boolean;
  createdAt: string;
  updatedAt: string;
};

export type ConfigSourceUpsertInput = {
  id?: number;
  gameId: string;
  env: string;
  name: string;
  type: ConfigSourceBinding['type'];
  config: string;
};

// Source: croupier/internal/api/configexplorer/dto.go EntryDTO
export type ConfigExplorerEntry = {
  name: string;
  path: string;
  dir: boolean;
  size: number;
  modTime?: string;
};

// Source: croupier/internal/api/configexplorer/dto.go FileResponse
export type ConfigExplorerFile = {
  path: string;
  format: string;
  text?: string;
  base64?: string;
  size: number;
  writable: boolean;
};

export type ConfigEmergencyWriteInput = {
  sourceId: number;
  path: string;
  content: string;
  reason: string;
};

export async function listConfigSources(params: { gameId?: string; env?: string }) {
  return request<{ items: ConfigSourceBinding[] }>('/api/v1/config-explorer/sources', {
    params,
  });
}

export async function upsertConfigSource(data: ConfigSourceUpsertInput) {
  return request('/api/v1/config-explorer/sources', {
    method: 'POST',
    data,
  });
}

export async function deleteConfigSource(id: number) {
  return request(`/api/v1/config-explorer/sources/${id}`, {
    method: 'DELETE',
  });
}

export async function listConfigTree(sourceId: number, dir: string) {
  return request<{ items: ConfigExplorerEntry[] }>('/api/v1/config-explorer/tree', {
    params: { sourceId, dir },
  });
}

export async function readConfigFile(sourceId: number, path: string) {
  return request<ConfigExplorerFile>('/api/v1/config-explorer/file', {
    params: { sourceId, path },
  });
}

export async function writeConfigFile(data: ConfigEmergencyWriteInput) {
  return request('/api/v1/config-explorer/file', {
    method: 'PUT',
    data,
  });
}
