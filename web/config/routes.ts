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
const functionManagementRoutes = [
  {
    path: '/system/functions',
    redirect: '/system/functions/catalog',
  },
  {
    path: '/system/functions/catalog',
    name: 'FunctionCatalog',
    access: 'canFunctionsRead',
    component: './Functions/Directory',
  },
  {
    // 旧「资源/操作」页已并入资源目录（ResourceCatalog），保留重定向兼容书签与旧链接。
    path: '/system/functions/resources',
    redirect: '/system/functions/resource-catalog',
  },
  {
    path: '/system/functions/pages',
    name: 'PageStudio',
    access: 'canPageRead',
    component: './PageStudio',
    icon: 'layout',
  },
  {
    path: '/system/functions/openapi-sources',
    name: 'OpenAPISources',
    access: 'canOpenAPISourcesRead',
    component: './OpenAPISources',
    icon: 'cloudUpload',
  },
  {
    path: '/system/functions/invoke',
    name: 'FunctionInvoke',
    access: 'canFunctionsRead',
    component: './Functions/Invoke',
    hideInMenu: true,
  },
  {
    path: '/system/functions/instances',
    name: 'FunctionInstances',
    access: 'canFunctionsRead',
    component: './Functions/Instances',
    icon: 'cluster',
  },
  {
    path: '/system/functions/warnings',
    name: 'FunctionWarnings',
    access: 'canFunctionsRead',
    component: './Functions/Warnings',
    icon: 'warning',
  },
  {
    path: '/system/functions/assignments',
    name: 'FunctionAssignments',
    access: 'canAssignmentsRead',
    component: './Assignments',
    icon: 'safety',
  },
  {
    path: '/system/functions/resource-catalog',
    name: 'ResourceCatalog',
    access: 'canResourcesRead',
    component: './ResourceCatalog',
    icon: 'database',
  },
  {
    path: '/system/functions/proposals',
    access: 'canPageRead',
    component: './Proposals',
    hideInMenu: true,
  },
  {
    path: '/system/functions/:id',
    name: 'FunctionDetail',
    access: 'canFunctionsRead',
    component: './Functions/Detail',
    hideInMenu: true,
  },
];

export default [
  // ==================== 审批中心 ====================
  // 高频待办入口：函数调用/页面发布的双人规则审批（与静态配置分离）。
  {
    path: '/approvals',
    name: 'Approvals',
    icon: 'audit',
    access: 'canAuditRead',
    component: './Approvals',
  },

  // ==================== 平台配置 ====================
  {
    path: '/system',
    name: 'SystemConfig',
    icon: 'control',
    access: 'canSystemConfigRead',
    routes: [
      {
        path: '/system',
        redirect: '/system/foundation/environments',
      },
      {
        path: '/system/foundation',
        name: 'SystemFoundation',
        routes: [
          {
            path: '/system/foundation/environments',
            name: 'GameEnvironments',
            access: 'canGamesRead',
            component: './GamesEnvs',
          },
          {
            // 术语字典是 Console 展示文案的生成期数据源（非运维健康类配置），
            // 归属系统管理-基础配置。
            path: '/system/foundation/terms',
            name: 'Terms',
            access: 'canSystemConfigRead',
            component: './Ops/Terms',
          },
          {
            path: '/system/foundation/site',
            name: 'SiteSettings',
            access: 'canSystemConfigRead',
            component: './System/SiteSettings',
          },
        ],
      },
      {
        path: '/system/extensions',
        name: 'Extensions',
        access: 'canExtensionsRead',
        routes: [
          {
            path: '/system/extensions/store',
            name: 'ExtensionsStore',
            access: 'canExtensionsRead',
            component: './Extensions/Store',
          },
          {
            path: '/system/extensions/installations',
            name: 'ExtensionsInstallations',
            access: 'canExtensionsRead',
            component: './Extensions/Installations',
          },
          {
            path: '/system/extensions/agent-sync',
            name: 'ExtensionsAgentSync',
            access: 'canExtensionsRead',
            component: './Extensions/AgentSync',
          },
        ],
      },
    ],
  },
  {
    path: '/system/functions',
    name: 'FunctionsAndPages',
    icon: 'function',
    access: 'canFunctionsAndPagesRead',
    routes: functionManagementRoutes,
  },
  {
    path: '/console',
    name: 'ControlConsole',
    icon: 'appstore',
    access: 'canConsoleRead',
    hideInMenu: false,
    routes: [
      {
        path: '/console',
        redirect: '/console/home',
      },
      {
        path: '/console/home',
        name: 'ConsoleHome',
        access: 'canConsoleRead',
        component: './Console',
        hideInMenu: true,
      },
      {
        path: '/console/templates/player-manage',
        name: 'PlayerManageTemplate',
        access: 'canConsoleRead',
        component: './Console/TemplatePlayerManage',
        hideInMenu: true,
      },
      {
        path: '/console/:categoryKey/:pageKey',
        name: 'ConsolePage',
        access: 'canConsoleRead',
        component: './Console/Page',
        hideInMenu: true,
      },
      {
        path: '/console/:categoryKey',
        name: 'ConsoleCategory',
        access: 'canConsoleRead',
        component: './Console',
        hideInMenu: true,
      },
    ],
  },
  {
    path: '/analytics',
    name: 'Analytics',
    icon: 'areaChart',
    access: 'canAnalyticsRead',
    routes: [
      { path: '/analytics', redirect: '/analytics/realtime' },
      {
        path: '/analytics/realtime',
        name: 'Realtime',
        access: 'canAnalyticsRead',
        component: './Analytics/Realtime',
      },
      {
        path: '/analytics/overview',
        name: 'Overview',
        access: 'canAnalyticsRead',
        component: './Analytics/Overview',
      },
      {
        path: '/analytics/retention',
        name: 'Retention',
        access: 'canAnalyticsRead',
        component: './Analytics/Retention',
      },
      {
        path: '/analytics/behavior',
        name: 'Behavior',
        access: 'canAnalyticsRead',
        component: './Analytics/Behavior',
      },
      {
        path: '/analytics/payments',
        name: 'Payments',
        access: 'canAnalyticsRead',
        component: './Analytics/Payments',
      },
      {
        path: '/analytics/levels',
        name: 'Levels',
        access: 'canAnalyticsRead',
        component: './Analytics/Levels',
      },
      {
        path: '/analytics/invocations',
        name: 'Invocations',
        access: 'canAnalyticsRead',
        component: './Analytics/Invocations',
      },
      {
        path: '/analytics/warehouse',
        name: 'Warehouse',
        access: 'canAnalyticsRead',
        component: './Analytics/Warehouse',
      },
    ],
  },
  // Dev (研发协作: 缺陷追踪/任务安排)
  {
    path: '/dev',
    name: 'Dev',
    icon: 'project',
    access: 'canDevRead',
    routes: [
      { path: '/dev', redirect: '/dev/bugs' },
      {
        path: '/dev/bugs',
        name: 'Bugs',
        access: 'canDevRead',
        component: './Dev/Bugs',
      },
      {
        path: '/dev/tools',
        name: 'DevTools',
        access: 'canDevRead',
        component: './Dev/Tools',
      },
      {
        path: '/dev/releases',
        name: 'Releases',
        access: 'canDevRead',
        component: './Dev/Releases',
      },
      {
        // Excel 在线编辑器：游戏业务数值表（道具/活动/数值）的策划创作
        // 入口，编译为 ConfigVersion。
        path: '/dev/excel-config',
        name: 'ExcelConfig',
        access: 'canDevRead',
        component: './System/ExcelConfig',
      },
      {
        // 游戏业务配置（ConfigVersion 版本化）管理：schema/diff/版本历史，
        // 与 Excel 编辑器构成 创作→版本 链路。
        path: '/dev/configs',
        name: 'Configs',
        access: 'canDevRead',
        component: './Operations/Configs',
      },
      {
        // 在线配置浏览器：只读浏览各配置中心（git/redis/nacos/db/croupier）
        // + 可写源应急编辑。平台不参与各项目配置流程。
        path: '/dev/config-explorer',
        name: 'ConfigExplorer',
        access: 'canDevRead',
        component: './Dev/ConfigExplorer',
      },
      {
        path: '/dev/hotpatches',
        name: 'Hotpatches',
        access: 'canDevManage',
        component: './Dev/Hotpatches',
      },
      {
        path: '/dev/traces',
        name: 'Traces',
        access: 'canDevRead',
        component: './Telemetry/Traces',
      },
    ],
  },
  // Ops (运维)
  {
    path: '/ops',
    name: 'Ops',
    icon: 'tool',
    access: 'canOpsRead',
    routes: [
      { path: '/ops', redirect: '/ops/nodes' },
      {
        path: '/ops/servers',
        redirect: '/ops/nodes',
        hideInMenu: true,
      },
      { path: '/ops/nodes', name: 'Nodes', access: 'canOpsRead', component: './Ops/Nodes' },
      {
        // LB 监控（管道用开源：haproxy exporter → prometheus → 平台原生渲染；
        // docs/operations/load-balancing.md「LB 监控」）
        path: '/ops/lb',
        name: 'LBMonitor',
        access: 'canOpsRead',
        component: './Ops/LBMonitor',
      },
      {
        // Server 多实例成员拓扑（在线/离线/agent 分布）
        path: '/ops/cluster',
        name: 'Cluster',
        access: 'canOpsRead',
        component: './Ops/Cluster',
      },
      { path: '/ops/jobs', name: 'Jobs', access: 'canOpsRead', component: './Ops/Jobs' },
      {
        // cron 定时调度管理（/api/v1/schedules）
        path: '/ops/schedules',
        name: 'Schedules',
        access: 'canOpsManage',
        component: './Ops/Schedules',
      },
      {
        path: '/ops/alerts',
        name: 'Alerts',
        access: 'canOpsRead',
        component: './Ops/Alerts',
      },
      {
        path: '/ops/rate-limits',
        name: 'RateLimits',
        access: 'canOpsManage',
        component: './Ops/RateLimits',
      },
      {
        path: '/ops/backups',
        name: 'Backups',
        access: 'canOpsManage',
        component: './Ops/Backups',
      },
      {
        path: '/ops/certificates',
        name: 'Certificates',
        access: 'canOpsManage',
        component: './Ops/Certificates',
      },
      {
        path: '/ops/notifications',
        name: 'Notifications',
        access: 'canOpsManage',
        component: './Ops/Notifications',
      },
      {
        path: '/ops/analytics-filters',
        name: 'AnalyticsFilters',
        access: 'canOpsManage',
        component: './Ops/AnalyticsFilters',
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
    path: '/account',
    redirect: '/admin/account/center',
  },
  {
    path: '/account/center',
    redirect: '/admin/account/center',
  },
  {
    path: '/account/settings',
    redirect: '/admin/account/center?tab=security',
  },
  {
    path: '/account/messages',
    redirect: '/admin/account/messages',
  },
  {
    path: '/operations/extensions/store',
    redirect: '/system/extensions/store',
  },
  {
    path: '/operations/extensions/installations',
    redirect: '/system/extensions/installations',
  },
  {
    path: '/operations/extensions/agent-sync',
    redirect: '/system/extensions/agent-sync',
  },
  {
    path: '/analytics/attribution',
    redirect: '/analytics/overview',
    hideInMenu: true,
  },
  {
    path: '/analytics/segments',
    redirect: '/analytics/behavior',
    hideInMenu: true,
  },
  {
    path: '/ops/services',
    redirect: '/ops/nodes',
    hideInMenu: true,
  },
  {
    path: '/ops/registry',
    redirect: '/ops/nodes',
    hideInMenu: true,
  },
  {
    path: '/ops/health',
    redirect: '/ops/nodes',
    hideInMenu: true,
  },
  {
    path: '/ops/mq',
    redirect: '/ops/jobs',
    hideInMenu: true,
  },
  {
    path: '/ops/maintenance',
    redirect: '/ops/alerts',
    hideInMenu: true,
  },
  {
    path: '/system/component-management',
    redirect: '/system/functions/catalog',
    hideInMenu: true,
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
        // 缺陷追踪已迁移至「研发」域（/dev/bugs）；旧路径保留重定向
        path: '/support/bugs',
        redirect: '/dev/bugs',
        hideInMenu: true,
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
    name: 'AccessControl',
    icon: 'team',
    routes: [
      {
        path: '/admin',
        redirect: '/admin/permissions',
      },
      {
        path: '/admin/account',
        name: 'UserAccount',
        icon: 'user',
        routes: [
          { path: '/admin/account/center', name: 'Center', component: './Profile' },
          {
            path: '/admin/account/settings',
            hideInMenu: true,
            redirect: '/admin/account/center?tab=security',
          },
          {
            path: '/admin/account/messages',
            name: 'Messages',
            redirect: '/admin/account/center?tab=notifications',
          },
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
      // Login logs shortcut page (wraps Audit with preset kind=login)
      {
        path: '/admin/login-logs',
        name: 'LoginLogs',
        access: 'canAuditRead',
        component: './Admin/LoginLogs',
      },
      {
        path: '/admin/operation-logs',
        name: 'OperationLogs',
        access: 'canAuditRead',
        component: './Admin/OperationLogs',
      },
    ],
  },
  {
    path: '/',
    component: './Welcome',
  },
  {
    path: '/403',
    layout: false,
    component: './403',
  },
  {
    path: '*',
    layout: false,
    component: './404',
  },
];
