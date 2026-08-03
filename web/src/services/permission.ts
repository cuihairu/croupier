/**
 * 权限验证服务
 */

import { useModel } from '@umijs/max';

interface CurrentUser {
  access?: string;
  [key: string]: string | number | boolean | undefined;
}

/**
 * 解析权限字符串为 Set
 */
function parsePermissions(access: string | undefined): Set<string> {
  return new Set(
    (access || '')
      .split(',')
      .map((p) => p.trim())
      .filter(Boolean),
  );
}

/**
 * 检查是否有指定权限（纯函数，接受 access 参数）
 */
function checkPermission(access: string | undefined, permission: string): boolean {
  if (!permission) return true;
  return parsePermissions(access).has(permission);
}

/**
 * 检查是否有任意一个权限（纯函数）
 */
function checkAnyPermission(access: string | undefined, permissions: string[]): boolean {
  if (!permissions || permissions.length === 0) return true;
  const userPermissions = parsePermissions(access);
  return permissions.some((p) => userPermissions.has(p));
}

/**
 * 检查是否有所有权限（纯函数）
 */
function checkAllPermissions(access: string | undefined, permissions: string[]): boolean {
  if (!permissions || permissions.length === 0) return true;
  const userPermissions = parsePermissions(access);
  return permissions.every((p) => userPermissions.has(p));
}

/**
 * 根据权限过滤项目（纯函数）
 */
export function filterByPermission<T extends { permissions?: string[] }>(
  items: T[],
  access: string | undefined,
): T[] {
  return items.filter((item) => {
    if (!item.permissions || item.permissions.length === 0) return true;
    return checkAnyPermission(access, item.permissions);
  });
}

/**
 * 获取当前用户的 access 字符串（Hook）
 */
export function useCurrentUserAccess(): string {
  const { initialState } = useModel('@@initialState');
  const currentUser = initialState?.currentUser as CurrentUser | undefined;
  return (currentUser?.access as string | undefined) || '';
}

/**
 * 权限验证 Hook
 */
export function usePermission(permission: string): boolean {
  const access = useCurrentUserAccess();
  return checkPermission(access, permission);
}

/**
 * 批量权限验证 Hook
 */
export function usePermissions(permissions: string[]): boolean[] {
  const access = useCurrentUserAccess();
  return permissions.map((p) => checkPermission(access, p));
}

/**
 * 权限验证 Hook（任意一个）
 */
export function useAnyPermission(permissions: string[]): boolean {
  const access = useCurrentUserAccess();
  return checkAnyPermission(access, permissions);
}

/**
 * 权限验证 Hook（所有）
 */
export function useAllPermissions(permissions: string[]): boolean {
  const access = useCurrentUserAccess();
  return checkAllPermissions(access, permissions);
}
