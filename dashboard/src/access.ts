/**
 * @see https://umijs.org/docs/max/access#access
 * */
export default function access(initialState: { currentUser?: API.CurrentUser } | undefined) {
  const acc = ((initialState?.currentUser as any)?.access as string | undefined) || '';
  const perms = new Set(
    acc
      .split(',')
      .map((token) => token.trim().toLowerCase())
      .filter(Boolean),
  );
  const has = (p: string) => {
    const key = (p || '').toLowerCase();
    return perms.has('*') || perms.has(key);
  };
  return {
    canAdmin: has('admin'),
    // Game meta management
    canGamesManage: has('games:manage') || has('admin'),
    canGamesRead: has('games:read') || has('games:manage') || has('admin'),
    canRegistryRead: has('registry:read') || has('admin'),
    canAssignmentsRead: has('assignments:read') || has('admin'),
    canAssignmentsWrite: has('assignments:write') || has('admin'),
    canPacksReload: has('packs:reload') || has('admin'),
    canPacksExport: has('packs:export') || has('admin'),
    canAuditRead: has('audit:read') || has('admin'),
    // Functions management
    canFunctionsRead: has('functions:read') || has('functions:manage') || has('admin'),
    canFunctionsManage: has('functions:manage') || has('admin'),
    canEntitiesRead: has('entities:read') || has('entities:manage') || has('admin'),
    canPacksRead: has('packs:read') || has('packs:manage') || has('admin'),
    // 运维管理（Ops）
    canOpsRead: has('ops:read') || has('admin') || has('registry:read'),
    canOpsManage: has('ops:manage') || has('admin'),
    // Support (客服系统)
    canSupportRead: has('support:read') || has('admin'),
    canSupportManage: has('support:manage') || has('admin'),
    // 数据分析
    canAnalyticsRead: has('analytics:read') || has('admin'),
    canAnalyticsManage: has('analytics:manage') || has('admin'),
    canAnalyticsExport: has('analytics:export') || has('admin'),
    // 权限管理相关权限（与后端的 RBAC key 对齐）
    canPermissionManage: has('roles:read') || has('roles:manage') || has('users:read') || has('users:manage') || has('admin'),
    canRoleManage: has('roles:read') || has('roles:manage') || has('admin'),
    canUserManage: has('users:read') || has('users:manage') || has('admin'),
    canPermissionConfig: has('system:config') || has('admin'),
  };
}
