/**
 * @see https://umijs.org/docs/max/access#access
 * */
type AccessCurrentUser = {
  access?: string;
};

export default function access(initialState: { currentUser?: AccessCurrentUser } | undefined) {
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
  const canWorkspaceRead =
    has('workspaces:read') || has('workspaces:manage') || has('functions:manage') || has('admin');
  const canWorkspaceEdit =
    has('workspaces:edit') || has('workspaces:manage') || has('functions:manage') || has('admin');
  const canWorkspacePublish =
    has('workspaces:publish') ||
    has('workspaces:manage') ||
    has('functions:manage') ||
    has('admin');
  const canWorkspaceRollback =
    has('workspaces:rollback') ||
    has('workspaces:manage') ||
    has('functions:manage') ||
    has('admin');
  const canWorkspaceDelete =
    has('workspaces:delete') || has('workspaces:manage') || has('functions:manage') || has('admin');
  const canSystemConfigRead =
    has('games:read') ||
    has('games:manage') ||
    has('functions:read') ||
    has('functions:manage') ||
    has('ops:read') ||
    has('ops:manage') ||
    has('extension:read') ||
    has('extensions:read') ||
    has('extension:manage') ||
    has('extensions:manage') ||
    has('admin');
  return {
    canSystemConfigRead,
    canAdmin: has('admin'),
    // Game meta management
    canGamesManage: has('games:manage') || has('admin'),
    canGamesRead: has('games:read') || has('games:manage') || has('admin'),
    canRegistryRead: has('registry:read') || has('admin'),
    canAssignmentsRead: has('assignments:read') || has('admin'),
    canAssignmentsWrite: has('assignments:write') || has('admin'),
    canAuditRead: has('audit:read') || has('admin'),
    // Functions management
    canFunctionsRead: has('functions:read') || has('functions:manage') || has('admin'),
    canFunctionsManage: has('functions:manage') || has('admin'),
    // Workspace management (design/publish) - admin only
    canWorkspaceManage:
      canWorkspaceEdit || canWorkspacePublish || canWorkspaceRollback || canWorkspaceDelete,
    canWorkspaceRead,
    canWorkspaceEdit,
    canWorkspacePublish,
    canWorkspaceRollback,
    canWorkspaceDelete,
    canEntitiesRead: has('entities:read') || has('entities:manage') || has('admin'),
    // 运维管理（Ops）
    canOpsRead:
      has('ops:read') ||
      has('admin') ||
      has('registry:read') ||
      has('extension:read') ||
      has('extensions:read') ||
      has('extension:manage') ||
      has('extensions:manage'),
    canOpsManage: has('ops:manage') || has('admin'),
    // Support (客服系统)
    canSupportRead: has('support:read') || has('admin'),
    canSupportManage: has('support:manage') || has('admin'),
    // 数据分析
    canAnalyticsRead: has('analytics:read') || has('admin'),
    canAnalyticsManage: has('analytics:manage') || has('admin'),
    canAnalyticsExport: has('analytics:export') || has('admin'),
    // 扩展商店与安装管理
    canExtensionsRead:
      has('extension:read') ||
      has('extensions:read') ||
      has('extension:manage') ||
      has('extensions:manage') ||
      has('ops:read') ||
      has('ops:manage') ||
      has('admin'),
    canExtensionsManage:
      has('extension:write') ||
      has('extensions:write') ||
      has('extension:manage') ||
      has('extensions:manage') ||
      has('admin'),
    // 权限管理相关权限（与后端的 RBAC key 对齐）
    canPermissionManage:
      has('roles:read') ||
      has('roles:manage') ||
      has('users:read') ||
      has('users:manage') ||
      has('admin'),
    canRoleManage: has('roles:read') || has('roles:manage') || has('admin'),
    canUserManage: has('users:read') || has('users:manage') || has('admin'),
    canPermissionConfig: has('system:config') || has('admin'),
  };
}
