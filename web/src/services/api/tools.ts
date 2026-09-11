import { getIntl, request } from '@umijs/max';

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

// 展示 label 经 getIntl 解析（SelectLang 切换语言会整页刷新重新求值）；
// Map key / options value 为后端枚举契约，保持不动
const intl = getIntl();

export const toolCategoryLabels: Record<ToolCategory, string> = {
  ci: 'CI/CD',
  repo: intl.formatMessage({
    id: 'services.tools.categoryLabel.repo',
    defaultMessage: '代码仓库',
  }),
  monitor: intl.formatMessage({
    id: 'services.tools.categoryLabel.monitor',
    defaultMessage: '监控',
  }),
  docs: intl.formatMessage({ id: 'services.tools.categoryLabel.docs', defaultMessage: '文档' }),
  artifact: intl.formatMessage({
    id: 'services.tools.categoryLabel.artifact',
    defaultMessage: '制品库',
  }),
  other: intl.formatMessage({ id: 'services.tools.categoryLabel.other', defaultMessage: '其他' }),
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
