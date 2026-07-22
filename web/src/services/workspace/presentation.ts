import type { WorkspaceConfig } from '@/types/workspace';
import {
  getConsoleCategoryLocaleId,
  groupWorkspacesByConsoleCategory,
  resolveWorkspaceConsoleCategory,
} from '@/services/workspace/navigation';

export type WorkspaceObjectGroup = {
  key: string;
  label: string;
  locale?: string;
  configs: WorkspaceConfig[];
};

export type WorkspaceLabelDescriptor = {
  id?: string;
  defaultMessage: string;
};

export function resolveWorkspaceObjectLabel(config: WorkspaceConfig): string {
  return String(config.meta?.objectLabel || config.objectKey || '未命名对象').trim();
}

export function resolveWorkspaceCategoryLabel(category?: string): WorkspaceLabelDescriptor {
  const normalized = String(category || '').trim();
  if (!normalized) return { defaultMessage: '' };
  return {
    id: getConsoleCategoryLocaleId(normalized),
    defaultMessage: normalized,
  };
}

export function resolveWorkspaceConsoleCategoryLabel(
  config: WorkspaceConfig,
): WorkspaceLabelDescriptor {
  const category = resolveWorkspaceConsoleCategory(config);
  if (!category) return { defaultMessage: '' };
  return category.source === 'configured'
    ? resolveWorkspaceCategoryLabel(category.key)
    : {
        id: getConsoleCategoryLocaleId(category.key),
        defaultMessage: category.label,
      };
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
    label: group.label,
    locale: getConsoleCategoryLocaleId(group.key),
    configs: group.configs,
  }));
}
