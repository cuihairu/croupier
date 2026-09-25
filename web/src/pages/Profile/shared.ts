import type { JSONValue } from '@/types/dashboard';

/** Profile 页共享常量与纯工具（Tab 键、审计元数据提取、申请模板类型）。 */

/**
 * 归一头像 URL：空串 / 空白串 → undefined。
 *
 * 后端在未设置头像时返回 `avatar: ""`。React 会把它原样渲染成 `<img src="">`，
 * 而浏览器把空 src 解释为「重新请求当前页面的 URL」——既是一次无意义的请求，
 * 也会触发 React 告警。归一为 undefined 后由 `<Avatar icon={...}>` 占位。
 *
 * 个人中心 hero 与头像弹窗共用此函数（docs/BUGS.md BUG-007）。
 */
export function normalizeAvatarSrc(value: string | null | undefined): string | undefined {
  if (typeof value !== 'string') return undefined;
  const trimmed = value.trim();
  return trimmed === '' ? undefined : trimmed;
}

export const TAB_KEYS = {
  PROFILE: 'profile',
  SECURITY: 'security',
  GAMES: 'games',
  PERMISSIONS: 'permissions',
  ACTIVITY: 'activity',
  SESSIONS: 'sessions',
  NOTIFICATIONS: 'notifications',
} as const;

export type PermissionApplyItem = {
  id: string;
  name: string;
  description?: string;
  resource: string;
  action: string;
  category?: string;
};

export type PermissionApplyTemplate = Omit<PermissionApplyItem, 'name' | 'description'> & {
  nameId: string;
  descriptionId: string;
};

export const FALLBACK_APPLY_PERMISSION_TEMPLATES: PermissionApplyTemplate[] = [
  {
    id: 'pages:edit',
    nameId: 'profile.permissions.fallback.pages.edit.name',
    descriptionId: 'profile.permissions.fallback.pages.edit.description',
    resource: 'pages',
    action: 'edit',
    category: 'page',
  },
  {
    id: 'pages:publish',
    nameId: 'profile.permissions.fallback.pages.publish.name',
    descriptionId: 'profile.permissions.fallback.pages.publish.description',
    resource: 'pages',
    action: 'publish',
    category: 'page',
  },
  {
    id: 'pages:rollback',
    nameId: 'profile.permissions.fallback.pages.rollback.name',
    descriptionId: 'profile.permissions.fallback.pages.rollback.description',
    resource: 'pages',
    action: 'rollback',
    category: 'page',
  },
  {
    id: 'functions:manage',
    nameId: 'profile.permissions.fallback.functions.manage.name',
    descriptionId: 'profile.permissions.fallback.functions.manage.description',
    resource: 'functions',
    action: 'manage',
    category: 'functions',
  },
  {
    id: 'audit:read',
    nameId: 'profile.permissions.fallback.audit.read.name',
    descriptionId: 'profile.permissions.fallback.audit.read.description',
    resource: 'audit',
    action: 'read',
    category: 'audit',
  },
  {
    id: 'ops:manage',
    nameId: 'profile.permissions.fallback.ops.manage.name',
    descriptionId: 'profile.permissions.fallback.ops.manage.description',
    resource: 'ops',
    action: 'manage',
    category: 'ops',
  },
];

export function pickAuditMetaValue(
  meta: Record<string, JSONValue> | undefined,
  keys: string[],
): string {
  if (!meta) return '';
  for (const key of keys) {
    const value = meta[key];
    if (value !== undefined && value !== null && String(value).trim() !== '') {
      return String(value);
    }
  }
  return '';
}

export interface ProfileData {
  displayName?: string;
  email?: string;
  phone?: string;
  avatar?: string;
  name?: string;
  roles?: string[];
  [key: string]: string | number | boolean | string[] | undefined;
}

export interface PasswordValues {
  current: string;
  password: string;
  confirm?: string;
}
