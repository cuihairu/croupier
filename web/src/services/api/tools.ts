import { request } from '@umijs/max';

// Source: internal/api/tool/dto.go
export type ToolItem = {
  id: number;
  name: string;
  url: string;
  description?: string;
  category: string;
  icon?: string;
  gameId?: string;
  env?: string;
  enabled: boolean;
  sort: number;
  createdBy?: string;
  updatedAt: string;
};

export type ToolCategory = 'ci' | 'repo' | 'monitor' | 'docs' | 'artifact' | 'other';

export const toolCategoryLabels: Record<ToolCategory, string> = {
  ci: 'CI/CD',
  repo: '代码仓库',
  monitor: '监控',
  docs: '文档',
  artifact: '制品库',
  other: '其他',
};

export const toolCategoryOrder: ToolCategory[] = [
  'ci',
  'repo',
  'monitor',
  'docs',
  'artifact',
  'other',
];

export async function listTools(params?: { gameId?: string; env?: string }): Promise<{
  items: ToolItem[];
}> {
  return request<{ items: ToolItem[] }>('/api/v1/tools', { params });
}

export async function createTool(payload: {
  name: string;
  url: string;
  description?: string;
  category?: ToolCategory | string;
  gameId?: string;
  env?: string;
  sort?: number;
}): Promise<ToolItem> {
  return request<ToolItem>('/api/v1/tools', { method: 'POST', data: payload });
}

export async function updateTool(
  id: number | string,
  payload: {
    name?: string;
    url?: string;
    description?: string;
    category?: string;
    sort?: number;
    enabled?: boolean;
    gameId?: string;
    env?: string;
  },
): Promise<ToolItem> {
  return request<ToolItem>(`/api/v1/tools/${encodeURIComponent(String(id))}`, {
    method: 'PUT',
    data: payload,
  });
}

export async function deleteTool(id: number | string): Promise<void> {
  return request<void>(`/api/v1/tools/${encodeURIComponent(String(id))}`, {
    method: 'DELETE',
  });
}
