/**
 * @name umi 的路由配置
 * @description 只支持 path,component,routes,redirect,wrappers,name,icon 的配置
 * @param path  path 只支持两种占位符配置，第一种是动态参数 :id 的形式，第二种是 * 通配符，通配符只能出现路由字符串的最后。
 * @param component 配置 location 和 path 匹配后用于渲染的 React 组件路径。可以是绝对路径，也可以是相对路径，如果是相对路径，会从 src/pages 开始找起。
 * @param routes 配置子路由，通常在需要为多个路径增加 layout 组件时使用。
 * @param redirect 配置路由跳转
 * @param wrappers 配置路由组件的包装组件，通过包装组件可以为当前的路由组件组合进更多的功能。 比如，可以用于路由级别的权限校验
 * @param name 配置路由的标题，默认读取国际化文件 menu.ts 中 menu.xxxx 的值，如配置 name 为 login，则读取 menu.ts 中 menu.login 的取值作为标题
 * @param icon 配置路由的图标，取值参考 https://ant.design/components/icon-cn， 注意去除风格后缀和大小写，如想要配置图标为 <StepBackwardOutlined /> 则取值应为 stepBackward 或 StepBackward，如想要配置图标为 <UserOutlined /> 则取值应为 user 或者 User
 * @doc https://umijs.org/docs/guides/routes
 */
export default [
  {
    path: '/analytics',
    name: 'Analytics',
    icon: 'areaChart',
    access: 'canAnalyticsRead',
    routes: [
      { path: '/analytics', redirect: '/analytics/realtime' },
      { path: '/analytics/realtime', name: 'Realtime', access: 'canAnalyticsRead', component: './Analytics/Realtime' },
      { path: '/analytics/overview', name: 'Overview', access: 'canAnalyticsRead', component: './Analytics/Overview' },
      { path: '/analytics/retention', name: 'Retention', access: 'canAnalyticsRead', component: './Analytics/Retention' },
      { path: '/analytics/behavior', name: 'Behavior', access: 'canAnalyticsRead', component: './Analytics/Behavior' },
      { path: '/analytics/payments', name: 'Payments', access: 'canAnalyticsRead', component: './Analytics/Payments' },
      { path: '/analytics/levels', name: 'Levels', access: 'canAnalyticsRead', component: './Analytics/Levels' },
      { path: '/analytics/attribution', name: 'Attribution', access: 'canAnalyticsRead', component: './Analytics/Attribution' },
      { path: '/analytics/segments', name: 'Segments', access: 'canAnalyticsRead', component: './Analytics/Segments' },
    ],
  },
  // Move 运营管理 (Operations) to second position in the menu
  {
    path: '/operations',
    name: 'Operations',
    icon: 'dashboard',
    routes: [
      {
        path: '/operations',
        redirect: '/operations/approvals',
      },
      {
        path: '/operations/approvals',
        name: 'Approvals',
        access: 'canApprovalsRead',
        component: './Approvals',
      },
      {
        path: '/operations/audit',
        name: 'Audit',
        access: 'canAuditRead',
        component: './Audit',
      },
      {
        path: '/operations/operation-logs',
        name: 'OperationLogs',
        access: 'canAuditRead',
        component: './Admin/OperationLogs',
      },
      {
        path: '/operations/registry',
        name: 'Registry',
        access: 'canRegistryRead',
        component: './Registry',
        hideInMenu: true,
      },
      {
        path: '/operations/servers',
        name: 'Servers',
        access: 'canRegistryRead',
        component: './Servers',
        hideInMenu: true,
      },
      {
        path: '/operations/configs',
        name: 'Configs',
        access: 'canOpsRead',
        component: './Operations/Configs',
      },
    ],
  },
  {
    path: '/user',
    layout: false,
    routes: [
      {
        name: 'login',
        path: '/user/login',
        component: './User/Login',
      },
    ],
  },
  {
    path: '/support',
    name: 'Support',
    icon: 'customerService',
    access: 'canSupportRead',
    routes: [
      { path: '/support', redirect: '/support/tickets' },
      {
        path: '/support/tickets',
        name: 'Tickets',
        access: 'canSupportRead',
        component: './Support/Tickets',
      },
      {
        path: '/support/tickets/:id',
        name: 'TicketDetail',
        access: 'canSupportRead',
        component: './Support/Tickets/Detail',
        hideInMenu: true,
      },
      {
        path: '/support/faq',
        name: 'FAQ',
        access: 'canSupportRead',
        component: './Support/FAQ',
      },
      {
        path: '/support/bugs',
        name: 'Bugs',
        access: 'canSupportRead',
        component: './Support/Bugs',
      },
      {
        path: '/support/feedback',
        name: 'Feedback',
        access: 'canSupportRead',
        component: './Support/Feedback',
      },
    ],
  },
  {
    path: '/admin',
    name: 'AdminUsers',
    icon: 'team',
    routes: [
      {
        path: '/admin/account',
        name: 'UserAccount',
        icon: 'user',
        routes: [
          { path: '/admin/account/center', name: 'Center', component: './Profile' },
          { path: '/admin/account/settings', name: 'Settings', redirect: '/admin/account/center?tab=security' },
          { path: '/admin/account/messages', name: 'Messages', redirect: '/admin/account/center?tab=notifications' },
        ],
      },
      // Back-office user management (mirrors Security pages for convenience)
      {
        path: '/admin/permissions',
        name: 'Permissions',
        access: 'canPermissionManage',
        routes: [
          { path: '/admin/permissions', redirect: '/admin/permissions/users' },
          {
            path: '/admin/permissions/users',
            name: 'Users',
            access: 'canUserManage',
            component: './Permissions/UsersV2',
          },
          {
            path: '/admin/permissions/roles',
            name: 'Roles',
            access: 'canRoleManage',
            component: './Permissions/RolesV2',
          },
          {
            path: '/admin/permissions/config',
            name: 'Config',
            access: 'canPermissionConfig',
            component: './Permissions/Config',
          },
        ],
      },
      // Quick links to audit pages
      { path: '/admin/audit', name: 'Audit', access: 'canAuditRead', component: './Audit' },
      // Login logs shortcut page (wraps Audit with preset kind=login)
      { path: '/admin/login-logs', name: 'LoginLogs', access: 'canAuditRead', component: './Admin/LoginLogs' },
      // Operation logs (audit view focused on non-login events)
      { path: '/admin/operation-logs', name: 'OperationLogs', access: 'canAuditRead', component: './Admin/OperationLogs' },
    ],
  },
  {
    path: '/game',
    name: 'GameManagement',
    icon: 'control',
    access: 'canGamesRead',
    routes: [
      {
        path: '/game',
        redirect: '/game/environments',
      },
      // 游戏基础配置页面已下线，合并到环境/实体等页面
      {
        path: '/game/environments',
        name: 'GameEnvironments',
        access: 'canGamesRead',
        component: './GamesEnvs',
      },
      // 游戏内容管理
      {
        path: '/game/entities',
        name: 'GameEntities',
        access: 'canEntitiesRead',
        component: './Entities',
      },
      // 函数管理模块 - 重构后的统一函数管理菜单
      {
        path: '/game/functions',
        name: 'FunctionManagement',
        access: 'canFunctionsRead',
        routes: [
          {
            path: '/game/functions',
            redirect: '/game/functions/catalog',
          },
          {
            path: '/game/functions/catalog',
            name: 'FunctionCatalog',
            access: 'canFunctionsRead',
            component: './Functions/Directory',
            icon: 'unordered-list',
          },
          {
            path: '/game/functions/invoke',
            name: 'FunctionInvoke',
            access: 'canFunctionsRead',
            component: './GmFunctions',
            icon: 'play-circle',
          },
          {
            path: '/game/functions/instances',
            name: 'FunctionInstances',
            access: 'canFunctionsRead',
            component: './Functions/Instances',
            icon: 'cluster',
          },
          {
            path: '/game/functions/assignments',
            name: 'FunctionAssignments',
            access: 'canAssignmentsRead',
            component: './Assignments',
            icon: 'safety',
          },
          {
            path: '/game/functions/packs',
            name: 'FunctionPacks',
            access: 'canPacksRead',
            component: './Packs',
            icon: 'inbox',
          },
        ],
      },
      // 保持向后兼容的重定向
      {
        path: '/game/functions/old',
        name: 'GameFunctionsLegacy',
        access: 'canFunctionsRead',
        component: './GmFunctions',
        hideInMenu: true,
      },
      // 新增统一组件管理中心（主菜单已迁移到 /components）
      {
        path: '/game/component-management',
        name: 'ComponentManagement',
        access: 'canFunctionsRead',
        component: './ComponentManagement',
        hideInMenu: true,
      },
      // 游戏运营管理
      {
        path: '/game/assignments',
        name: 'GameAssignments',
        access: 'canAssignmentsRead',
        component: './Assignments',
      },
      {
        path: '/game/packs',
        name: 'GamePacks',
        access: 'canPacksRead',
        component: './Packs',
      },
    ],
  },
  {
    path: '/ops',
    name: 'Ops',
    icon: 'cluster',
    access: 'canOpsRead',
    routes: [
      { path: '/ops', redirect: '/ops/services' },
      { path: '/ops/services', name: 'Services', access: 'canOpsRead', component: './Ops/Services' },
      { path: '/ops/health', name: 'Health', access: 'canOpsRead', component: './Ops/Health' },
      { path: '/ops/nodes', name: 'Nodes', access: 'canOpsRead', component: './Ops/Nodes' },
      { path: '/ops/jobs', name: 'Jobs', access: 'canOpsRead', component: './Ops/Jobs' },
      { path: '/ops/alerts', name: 'Alerts', access: 'canOpsRead', component: './Ops/Alerts' },
      { path: '/ops/rate-limits', name: 'RateLimits', access: 'canOpsManage', component: './Ops/RateLimits' },
      { path: '/ops/backups', name: 'Backups', access: 'canOpsManage', component: './Ops/Backups' },
      { path: '/ops/mq', name: 'MQ', access: 'canOpsRead', component: './Ops/MQ' },
      { path: '/ops/certificates', name: 'Certificates', access: 'canOpsManage', component: './Ops/Certificates' },
      { path: '/ops/notifications', name: 'Notifications', access: 'canOpsManage', component: './Ops/Notifications' },
      { path: '/ops/analytics-filters', name: 'AnalyticsFilters', access: 'canOpsManage', component: './Ops/AnalyticsFilters' },
      { path: '/ops/maintenance', name: 'Maintenance', access: 'canOpsManage', component: './Ops/Maintenance' },
    ],
  },
  {
    path: '/',
    redirect: '/analytics/realtime',
  },
  // Legacy redirects for backward compatibility
  {
    path: '/account',
    redirect: '/admin/account/center',
  },
  { path: '/account/messages', redirect: '/admin/account/messages' },
  { path: '/account/center', redirect: '/admin/account/center' },
  { path: '/account/settings', redirect: '/admin/account/settings' },
  { path: '/permissions', redirect: '/admin/permissions/roles' },
  // Legacy redirects for system->admin renaming
  { path: '/system', redirect: '/admin/permissions/roles' },
  { path: '/system/permissions', redirect: '/admin/permissions/roles' },
  { path: '/system/permissions/roles', redirect: '/admin/permissions/roles' },
  { path: '/system/permissions/users', redirect: '/admin/permissions/users' },
  { path: '/system/permissions/config', redirect: '/admin/permissions/config' },
  // Legacy redirects for admin/permissions
  { path: '/admin/permissions', redirect: '/admin/permissions/roles' },
  { path: '/admin/permissions/roles', redirect: '/admin/permissions/roles' },
  { path: '/admin/permissions/users', redirect: '/admin/permissions/users' },
  { path: '/admin/permissions/config', redirect: '/admin/permissions/config' },
  // Game management legacy redirects
  {
    path: '/game-mgmt/games-meta',
    redirect: '/game/meta',
  },
  {
    path: '/game-mgmt/entities',
    redirect: '/game/entities',
  },
  {
    path: '/game-mgmt',
    redirect: '/game/meta',
  },
  {
    path: '/gm/games',
    redirect: '/game/environments',
  },
  {
    path: '/gm/functions',
    redirect: '/game/functions',
  },
  {
    path: '/gm/assignments',
    redirect: '/game/assignments',
  },
  {
    path: '/gm/packs',
    redirect: '/game/packs',
  },
  {
    path: '/gm/approvals',
    redirect: '/operations/approvals',
  },
  {
    path: '/gm/audit',
    redirect: '/operations/audit',
  },
  {
    path: '/gm/registry',
    redirect: '/operations/registry',
  },
  {
    path: '/gm',
    redirect: '/game/meta',
  },
  // Security menu removed (duplicated with AdminUsers/Permissions)
  {
    path: '/components',
    name: 'ComponentManagement',
    icon: 'appstore',
    access: 'canFunctionsRead',
    component: './ComponentManagement',
  },
  // 后台组件分配，显式展示在主菜单
  {
    path: '/assignments',
    name: 'GameAssignments',
    icon: 'branches',
    access: 'canAssignmentsRead',
    component: './Assignments',
  },
  {
    path: '*',
    layout: false,
    component: './404',
  },
];
