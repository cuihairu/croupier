/**
 * 权限控制组件
 */

import React from 'react';
import { useAnyPermission } from '@/services/permission';

export interface PermissionGuardProps {
  permissions?: string[];
  children: React.ReactNode;
  fallback?: React.ReactNode;
}

/**
 * 权限守卫组件
 */
export function PermissionGuard({ permissions, children, fallback = null }: PermissionGuardProps) {
  const hasPermission = useAnyPermission(permissions || []);

  if (!hasPermission) {
    return <>{fallback}</>;
  }

  return <>{children}</>;
}

export default PermissionGuard;
