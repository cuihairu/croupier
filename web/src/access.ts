/**
 * @see https://umijs.org/docs/max/access#access
 * */
type AccessCurrentUser = {
  access?: string;
  roles?: string[];
};

export default function access(initialState: { currentUser?: AccessCurrentUser } | undefined) {
  const currentUser = initialState?.currentUser;
  const acc = currentUser?.access || '';
  const perms = new Set(
    acc
      .split(',')
      .map((token) => token.trim().toLowerCase())
      .filter(Boolean),
  );
  (currentUser?.roles || [])
    .map((role) => String(role || '').trim().toLowerCase())
    .filter(Boolean)
    .forEach((role) => perms.add(role));

  const has = (p: string) => {
    const key = (p || '').toLowerCase();
    return perms.has('*') || perms.has(key);
  };
  const isAdmin = has('admin') || has('admin:all') || has('super_admin');
  const hasAny = (...keys: string[]) => keys.some((key) => has(key)) || isAdmin;

  const canWorkspaceRead =
    hasAny('workspace:read', 'workspaces:read', 'workspaces:manage', 'functions:manage');
  const canWorkspaceEdit =
    hasAny('workspace:edit', 'workspaces:edit', 'workspaces:manage', 'functions:manage');
  const canWorkspacePublish =
    hasAny('workspace:publish', 'workspaces:publish', 'workspaces:manage', 'functions:manage');
  const canWorkspaceRollback =
    hasAny('workspace:rollback', 'workspaces:rollback', 'workspaces:manage', 'functions:manage');
  const canWorkspaceDelete =
    hasAny('workspace:delete', 'workspaces:delete', 'workspaces:manage', 'functions:manage');
  const canConsoleRead =
    canWorkspaceRead || hasAny('function:invoke', 'functions:read', 'functions:manage');
  const canSystemConfigRead =
    hasAny(
      'games:read',
      'games:manage',
      'functions:read',
      'functions:manage',
      'ops:read',
      'ops:manage',
      'extension:read',
      'extensions:read',
      'extension:manage',
      'extensions:manage',
    );
  return {
    canSystemConfigRead,
    canAdmin: isAdmin,
    // Game meta management
    canGamesManage: hasAny('games:manage'),
    canGamesRead: hasAny('games:read', 'games:manage'),
    canRegistryRead: hasAny('registry:read'),
    canAssignmentsRead: hasAny('assignments:read'),
    canAssignmentsWrite: hasAny('assignments:write'),
    canAuditRead: hasAny('audit:read'),
    // Functions management
    canFunctionsRead: hasAny('functions:read', 'functions:manage'),
    canFunctionsManage: hasAny('functions:manage'),
    // Runtime console reads published workspaces; it is not the workspace editor.
    canConsoleRead,
    // Workspace management (design/publish) - admin only
    canWorkspaceManage:
      canWorkspaceEdit || canWorkspacePublish || canWorkspaceRollback || canWorkspaceDelete,
    canWorkspaceRead,
    canWorkspaceEdit,
    canWorkspacePublish,
    canWorkspaceRollback,
    canWorkspaceDelete,
    canEntitiesRead: hasAny('entities:read', 'entities:manage'),
    // 运维管理（Ops）
    canOpsRead: hasAny(
      'ops:read',
      'registry:read',
      'extension:read',
      'extensions:read',
      'extension:manage',
      'extensions:manage',
    ),
    canOpsManage: hasAny('ops:manage'),
    // Support (客服系统)
    canSupportRead: hasAny('support:read'),
    canSupportManage: hasAny('support:manage'),
    // 数据分析
    canAnalyticsRead: hasAny('analytics:read'),
    canAnalyticsManage: hasAny('analytics:manage'),
    canAnalyticsExport: hasAny('analytics:export'),
    // 扩展商店与安装管理
    canExtensionsRead: hasAny(
      'extension:read',
      'extensions:read',
      'extension:manage',
      'extensions:manage',
      'ops:read',
      'ops:manage',
    ),
    canExtensionsManage: hasAny(
      'extension:write',
      'extensions:write',
      'extension:manage',
      'extensions:manage',
    ),
    // 权限管理相关权限（与后端的 RBAC key 对齐）
    canPermissionManage: hasAny('roles:read', 'roles:manage', 'users:read', 'users:manage'),
    canRoleManage: hasAny('roles:read', 'roles:manage'),
    canUserManage: hasAny('users:read', 'users:manage'),
    canPermissionConfig: hasAny('system:config'),
  };
}
