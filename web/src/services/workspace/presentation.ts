import type { WorkspaceConfig } from '@/types/workspace';
import {
  groupWorkspacesByConsoleCategory,
  resolveWorkspaceConsoleCategory,
} from '@/services/workspace/navigation';

const BUILTIN_CATEGORY_LABELS: Record<string, string> = {
  gameplay: '玩法域',
  economy: '经济域',
  social: '社交域',
  support: '客服域',
  ops: '运维域',
};

export type WorkspaceObjectGroup = {
  key: string;
  label: string;
  configs: WorkspaceConfig[];
};

export function resolveWorkspaceObjectLabel(config: WorkspaceConfig): string {
  return String(config.meta?.objectLabel || config.objectKey || '未命名对象').trim();
}

export function resolveWorkspaceCategoryLabel(category?: string): string {
  const normalized = String(category || '').trim();
  if (!normalized) return '';
  return BUILTIN_CATEGORY_LABELS[normalized] || normalized;
}

export function resolveWorkspaceConsoleCategoryLabel(config: WorkspaceConfig): string {
  const category = resolveWorkspaceConsoleCategory(config);
  if (!category) return '';
  return category.source === 'configured'
    ? resolveWorkspaceCategoryLabel(category.key)
    : category.label;
}

export function groupWorkspacesByObject(configs: WorkspaceConfig[]): WorkspaceObjectGroup[] {
  const grouped = new Map<string, WorkspaceObjectGroup>();

  configs.forEach((config) => {
    const key = String(config.objectKey || '').trim() || 'unknown';
    const label = resolveWorkspaceObjectLabel(config);
    const existing = grouped.get(key);
    if (existing) {
      existing.configs.push(config);
      if (existing.label === key && label !== key) {
        existing.label = label;
      }
      return;
    }

    grouped.set(key, {
      key,
      label,
      configs: [config],
    });
  });

  return Array.from(grouped.values()).sort((a, b) => a.label.localeCompare(b.label, 'zh-CN'));
}

export function groupWorkspacesByCategory(configs: WorkspaceConfig[]): WorkspaceObjectGroup[] {
  return groupWorkspacesByConsoleCategory(configs).map((group) => ({
    key: group.key,
    label: group.source === 'configured' ? resolveWorkspaceCategoryLabel(group.key) : group.label,
    configs: group.configs,
  }));
}
