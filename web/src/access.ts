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

  const canPageRead = hasAny('pages:read', 'pages:edit', 'pages:publish', 'pages:rollback');
  const canPageEdit = hasAny('pages:edit');
  const canPagePublish = hasAny('pages:publish');
  const canPageRollback = hasAny('pages:rollback');
  const canPageDelete = hasAny('pages:delete');
  const canConsoleRead = hasAny('console:read', 'pages:read', 'function:invoke');
  const canResourcesRead = hasAny('resources:read', 'resources:diagnose', 'functions:read', 'functions:manage');
  const canFunctionsRead = hasAny('functions:read', 'functions:manage');
  const canOpenAPISourcesRead = hasAny(
    'openapi_sources:read',
    'openapi_sources:write',
    'resources:read',
    'resources:diagnose',
    'functions:read',
    'functions:manage',
    'pages:read',
    'pages:edit',
  );
  const canOpenAPISourcesWrite = hasAny(
    'openapi_sources:write',
    'resources:diagnose',
    'functions:manage',
    'pages:edit',
  );
  const canSystemConfigRead =
    hasAny(
      'games:read',
      'games:manage',
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
    canFunctionsAndPagesRead: canFunctionsRead || canPageRead || canResourcesRead,
    canFunctionsRead,
    canFunctionsManage: hasAny('functions:manage'),
    // Runtime console reads published PageSpec snapshots.
    canConsoleRead,
    canPageManage: canPageEdit || canPagePublish || canPageRollback || canPageDelete,
    canPageRead,
    canPageEdit,
    canPagePublish,
    canPageRollback,
    canPageDelete,
    canResourcesRead,
    canOpenAPISourcesRead,
    canOpenAPISourcesWrite,
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
