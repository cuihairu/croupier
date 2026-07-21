import type { WorkspaceConfig, WorkspacePermissions } from '@/types/workspace';

export type WorkspaceConsoleCategorySource = 'configured' | 'objectKey';

export type WorkspaceConsoleCategory = {
  key: string;
  label: string;
  source: WorkspaceConsoleCategorySource;
};

export type WorkspaceConsoleCategoryGroup = WorkspaceConsoleCategory & {
  configs: WorkspaceConfig[];
};

export type ConsoleWorkspaceMenuItem = {
  key: string;
  path: string;
  name: string;
  locale: false;
  children?: ConsoleWorkspaceMenuItem[];
};

export type ConsoleAccessUser = {
  access?: string;
  roles?: string[];
};

function normalizeText(value: unknown): string {
  return String(value ?? '').trim();
}

function normalizeToken(value: unknown): string {
  return normalizeText(value).toLowerCase();
}

function firstObjectSegment(objectKey: string): string {
  return objectKey.split('.')[0]?.trim() || '';
}

function encodePathSegment(value: string): string {
  return encodeURIComponent(value);
}

function sortWorkspaces(a: WorkspaceConfig, b: WorkspaceConfig): number {
  const aOrder = typeof a.menuOrder === 'number' ? a.menuOrder : Number.MAX_SAFE_INTEGER;
  const bOrder = typeof b.menuOrder === 'number' ? b.menuOrder : Number.MAX_SAFE_INTEGER;
  if (aOrder !== bOrder) return aOrder - bOrder;
  return (a.title || a.objectKey).localeCompare(b.title || b.objectKey, 'zh-CN');
}

function getPermissionRoles(permissions?: WorkspacePermissions): string[] {
  if (!permissions || Array.isArray(permissions)) return [];
  return Array.isArray(permissions.roles) ? permissions.roles : [];
}

function getPermissionIDs(permissions?: WorkspacePermissions): string[] {
  if (!permissions) return [];
  if (Array.isArray(permissions)) return permissions;
  return Array.isArray(permissions.permissions) ? permissions.permissions : [];
}

export function resolveWorkspaceConsoleCategory(
  config: Pick<WorkspaceConfig, 'category' | 'objectKey' | 'meta'>,
): WorkspaceConsoleCategory | null {
  const configuredCategory = normalizeText(config.category);
  if (configuredCategory) {
    return {
      key: configuredCategory,
      label: configuredCategory,
      source: 'configured',
    };
  }

  const objectKey = normalizeText(config.objectKey);
  const categoryKey = firstObjectSegment(objectKey);
  if (!categoryKey) return null;

  const objectLabel = objectKey === categoryKey ? normalizeText(config.meta?.objectLabel) : '';
  return {
    key: categoryKey,
    label: objectLabel || categoryKey,
    source: 'objectKey',
  };
}

export function getConsoleCategoryPath(categoryKey: string): string {
  return `/console/${encodePathSegment(categoryKey)}`;
}

export function getConsoleWorkspacePath(config: WorkspaceConfig | string): string {
  const objectKey =
    typeof config === 'string' ? normalizeText(config) : normalizeText(config.objectKey);
  const category =
    typeof config === 'string'
      ? resolveWorkspaceConsoleCategory({ objectKey })
      : resolveWorkspaceConsoleCategory(config);

  if (!objectKey || !category) return '/console/home';
  return `${getConsoleCategoryPath(category.key)}/${encodePathSegment(objectKey)}`;
}

export function canAccessWorkspaceMenu(config: WorkspaceConfig, user?: ConsoleAccessUser): boolean {
  const userRoles = new Set((user?.roles || []).map(normalizeToken).filter(Boolean));
  const userPermissions = new Set(
    normalizeText(user?.access).split(',').map(normalizeToken).filter(Boolean),
  );
  const isAdmin =
    userRoles.has('admin') ||
    userRoles.has('super_admin') ||
    userPermissions.has('admin') ||
    userPermissions.has('*');

  if (isAdmin) return true;

  const requiredRoles = getPermissionRoles(config.permissions).map(normalizeToken).filter(Boolean);
  if (requiredRoles.length > 0 && !requiredRoles.some((role) => userRoles.has(role))) {
    return false;
  }

  const requiredPermissions = getPermissionIDs(config.permissions)
    .map(normalizeToken)
    .filter(Boolean);
  if (
    requiredPermissions.length > 0 &&
    !requiredPermissions.some((permission) => userPermissions.has(permission))
  ) {
    return false;
  }

  return true;
}

export function groupWorkspacesByConsoleCategory(
  configs: WorkspaceConfig[],
): WorkspaceConsoleCategoryGroup[] {
  const grouped = new Map<string, WorkspaceConsoleCategoryGroup>();

  configs.forEach((config) => {
    const category = resolveWorkspaceConsoleCategory(config);
    if (!category) return;

    const existing = grouped.get(category.key);
    if (existing) {
      existing.configs.push(config);
      if (existing.label === existing.key && category.label !== category.key) {
        existing.label = category.label;
      }
      return;
    }

    grouped.set(category.key, {
      ...category,
      configs: [config],
    });
  });

  return Array.from(grouped.values())
    .map((group) => ({
      ...group,
      configs: [...group.configs].sort(sortWorkspaces),
    }))
    .sort((a, b) => {
      const aOrder = a.configs[0]?.menuOrder ?? Number.MAX_SAFE_INTEGER;
      const bOrder = b.configs[0]?.menuOrder ?? Number.MAX_SAFE_INTEGER;
      if (aOrder !== bOrder) return aOrder - bOrder;
      return a.label.localeCompare(b.label, 'zh-CN');
    });
}

export function filterWorkspacesByConsoleCategory(
  configs: WorkspaceConfig[],
  categoryKey: string,
): WorkspaceConfig[] {
  const normalizedCategoryKey = normalizeText(categoryKey);
  if (!normalizedCategoryKey) return configs;
  return configs.filter((config) => {
    const category = resolveWorkspaceConsoleCategory(config);
    return category?.key === normalizedCategoryKey;
  });
}

export function buildConsoleWorkspaceMenuItems(
  configs: WorkspaceConfig[],
): ConsoleWorkspaceMenuItem[] {
  return groupWorkspacesByConsoleCategory(configs).map((group) => {
    const categoryPath = getConsoleCategoryPath(group.key);
    const children = group.configs.map((config) => ({
      key: getConsoleWorkspacePath(config),
      path: getConsoleWorkspacePath(config),
      name: config.title || config.objectKey,
      locale: false as const,
    }));

    return {
      key: categoryPath,
      path: categoryPath,
      name: group.label,
      locale: false,
      children,
    };
  });
}
