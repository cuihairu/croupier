/**
 * 权限验证服务
 */

import { useModel } from '@umijs/max';

/**
 * 获取当前用户权限列表
 */
function getCurrentPermissions(): Set<string> {
  // 从 initialState 获取权限
  const { initialState } = useModel('@@initialState');
  const access = ((initialState?.currentUser as any)?.access as string | undefined) || '';

  return new Set(
    access
      .split(',')
      .map((p) => p.trim())
      .filter(Boolean),
  );
}

/**
 * 检查是否有指定权限
 */
export function hasPermission(permission: string): boolean {
  if (!permission) return true;

  const permissions = getCurrentPermissions();
  return permissions.has(permission);
}

/**
 * 检查是否有任意一个权限
 */
export function hasAnyPermission(permissions: string[]): boolean {
  if (!permissions || permissions.length === 0) return true;

  const userPermissions = getCurrentPermissions();
  return permissions.some((p) => userPermissions.has(p));
}

/**
 * 检查是否有所有权限
 */
export function hasAllPermissions(permissions: string[]): boolean {
  if (!permissions || permissions.length === 0) return true;

  const userPermissions = getCurrentPermissions();
  return permissions.every((p) => userPermissions.has(p));
}

/**
 * 根据权限过滤项目
 */
export function filterByPermission<T extends { permissions?: string[] }>(items: T[]): T[] {
  return items.filter((item) => {
    if (!item.permissions || item.permissions.length === 0) return true;
    return hasAnyPermission(item.permissions);
  });
}

/**
 * 权限验证 Hook
 */
export function usePermission(permission: string): boolean {
  const { initialState } = useModel('@@initialState');
  const access = ((initialState?.currentUser as any)?.access as string | undefined) || '';

  const permissions = new Set(
    access
      .split(',')
      .map((p) => p.trim())
      .filter(Boolean),
  );

  return !permission || permissions.has(permission);
}

/**
 * 批量权限验证 Hook
 */
export function usePermissions(permissions: string[]): boolean[] {
  const { initialState } = useModel('@@initialState');
  const access = ((initialState?.currentUser as any)?.access as string | undefined) || '';

  const userPermissions = new Set(
    access
      .split(',')
      .map((p) => p.trim())
      .filter(Boolean),
  );

  return permissions.map((p) => !p || userPermissions.has(p));
}

/**
 * 权限验证 Hook（任意一个）
 */
export function useAnyPermission(permissions: string[]): boolean {
  const { initialState } = useModel('@@initialState');
  const access = ((initialState?.currentUser as any)?.access as string | undefined) || '';

  if (!permissions || permissions.length === 0) return true;

  const userPermissions = new Set(
    access
      .split(',')
      .map((p) => p.trim())
      .filter(Boolean),
  );

  return permissions.some((p) => userPermissions.has(p));
}

/**
 * 权限验证 Hook（所有）
 */
export function useAllPermissions(permissions: string[]): boolean {
  const { initialState } = useModel('@@initialState');
  const access = ((initialState?.currentUser as any)?.access as string | undefined) || '';

  if (!permissions || permissions.length === 0) return true;

  const userPermissions = new Set(
    access
      .split(',')
      .map((p) => p.trim())
      .filter(Boolean),
  );

  return permissions.every((p) => userPermissions.has(p));
}
