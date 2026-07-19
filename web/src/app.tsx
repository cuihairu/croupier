import { Footer, Question, SelectLang, AvatarDropdown, AvatarName } from '@/components';
import MessagesBell from '@/components/MessagesBell';
import { LinkOutlined, UserOutlined } from '@ant-design/icons';
import type { Settings as LayoutSettings } from '@ant-design/pro-components';
import { SettingDrawer } from '@ant-design/pro-components';
import type { RunTimeLayoutConfig } from '@umijs/max';
import { history, Link } from '@umijs/max';
import GameSelector from '@/components/GameSelector';
import defaultSettings from '../config/defaultSettings';
import { errorConfig } from './requestErrorConfig';
import { fetchCurrentUser, getMyPermissions } from '@/services/api';
import { hydrateScope } from '@/stores/scope';
import React, { useEffect } from 'react';
import { App as AntdApp, Grid } from 'antd';
import { setAppApi } from './utils/antdApp';
import { initWorkspaceAlerting } from './services/workspace/alerts';
import { listPublishedWorkspaceConfigs } from '@/services/workspaceConfig';
import { WORKSPACE_CATEGORIES } from '@/config/workspaceCategories';
import type { WorkspaceConfig, WorkspaceCategory } from '@/types/workspace';

const isDev = process.env.NODE_ENV === 'development';
const loginPath = '/user/login';

type InitialCurrentUser = {
  name?: string;
  userid?: string;
  access?: string;
  roles?: any[];
  avatar?: string;
};

/**
 * @see  https://umijs.org/zh-CN/plugins/plugin-initial-state
 * */
export async function getInitialState(): Promise<{
  settings?: Partial<LayoutSettings>;
  currentUser?: InitialCurrentUser;
  loading?: boolean;
  fetchUserInfo?: () => Promise<InitialCurrentUser | undefined>;
}> {
  const fetchUserInfo = async () => {
    try {
      const token = localStorage.getItem('token');
      if (!token) return undefined;
      const currentUser = await fetchCurrentUser();
      const roleNames = (currentUser.roles || []).map((role) =>
        typeof role === 'string' ? role.toLowerCase() : role,
      );
      let permissionIDs: string[] = [];
      try {
        const perms = await getMyPermissions();
        permissionIDs =
          (perms as any)?.permissionIDs ||
          (perms as any)?.permissionIds ||
          (perms as any)?.permission_ids ||
          [];
      } catch {
        permissionIDs = [];
      }
      const accessTokens = Array.from(new Set([...(permissionIDs || []), ...(roleNames || [])]))
        .map((t) =>
          String(t || '')
            .trim()
            .toLowerCase(),
        )
        .filter(Boolean);
      return {
        name: currentUser.username,
        userid: currentUser.username,
        access: accessTokens.join(','),
        roles: roleNames,
      } as any;
    } catch (error: any) {
      if (error?.response?.status === 401 || error?.response?.status === 400) {
        localStorage.removeItem('token');
      }
      history.push(loginPath);
      return undefined;
    }
  };

  const { location } = history;
  if (location.pathname !== loginPath) {
    const currentUser = await fetchUserInfo();
    if (currentUser) {
      hydrateScope();
    }
    return {
      fetchUserInfo,
      currentUser,
      settings: defaultSettings as Partial<LayoutSettings>,
    };
  }
  return {
    fetchUserInfo,
    settings: defaultSettings as Partial<LayoutSettings>,
  };
}

// ProLayout 支持的api https://procomponents.ant.design/components/layout
export const layout: RunTimeLayoutConfig = ({ initialState, setInitialState }) => {
  const isAuthed = !!initialState?.currentUser;

  const HeaderActions: React.FC = () => {
    const screens = Grid.useBreakpoint();
    const isMobile = !screens.md;
    if (!isAuthed) {
      return (
        <>
          <Question key="doc" />
          <SelectLang key="SelectLang" />
        </>
      );
    }
    if (isMobile) {
      return (
        <>
          <GameSelector key="scope-mobile" variant="mobile" />
          <MessagesBell key="msgs-mobile" />
        </>
      );
    }
    return (
      <>
        <GameSelector key="scope" variant="header" />
        <MessagesBell key="msgs" />
        <Question key="doc" />
        <SelectLang key="SelectLang" />
      </>
    );
  };

  const AppApiRegistrar: React.FC = () => {
    const inst = AntdApp.useApp();
    useEffect(() => {
      setAppApi({ message: inst.message, notification: inst.notification });
    }, [inst]);
    useEffect(() => {
      initWorkspaceAlerting();
    }, []);
    return null;
  };
  return {
    actionsRender: () => [<HeaderActions key="header-actions" />] as any,
    splitMenus: false,
    suppressSiderWhenMenuEmpty: true,
    avatarProps: {
      src: initialState?.currentUser?.avatar,
      icon: initialState?.currentUser?.avatar ? undefined : <UserOutlined />,
      title: <AvatarName />,
      render: (_, avatarChildren) => {
        return <AvatarDropdown menu>{avatarChildren}</AvatarDropdown>;
      },
    },
    footerRender: () => <Footer />,
    onPageChange: () => {
      const { location } = history;
      if (!initialState?.currentUser && location.pathname !== loginPath) {
        history.push(loginPath);
      }
    },
    links: isDev
      ? [
          <Link key="openapi" to="/umi/plugin/openapi" target="_blank">
            <LinkOutlined />
            <span>OpenAPI 文档</span>
          </Link>,
        ]
      : [],
    menuHeaderRender: undefined,
    childrenRender: (children) => {
      return (
        <AntdApp>
          <AppApiRegistrar />
          {children}
          {isDev && (
            <SettingDrawer
              disableUrlParams
              enableDarkTheme
              settings={initialState?.settings}
              onSettingChange={(settings) => {
                setInitialState((preInitialState) => ({
                  ...preInitialState,
                  settings,
                }));
              }}
            />
          )}
        </AntdApp>
      );
    },
    ...initialState?.settings,
  };
};

/**
 * @name request 配置
 * @doc https://umijs.org/docs/max/request#配置
 */
export const request = {
  ...errorConfig,
};

/**
 * 动态注入运行控制台路由
 * 根据已发布的工作台配置，按分类生成菜单
 */
export async function patchClientRoutes({ routes }: { routes: any[] }) {
  try {
    // 获取已发布的工作台
    const configs = await listPublishedWorkspaceConfigs();
    if (!Array.isArray(configs) || configs.length === 0) {
      return;
    }

    // 按分类分组
    const grouped = new Map<WorkspaceCategory, WorkspaceConfig[]>();
    configs.forEach((config) => {
      const category = config.category || 'other';
      if (!grouped.has(category)) {
        grouped.set(category, []);
      }
      grouped.get(category)!.push(config);
    });

    // 按分类排序
    const sortedCategories = Array.from(grouped.entries()).sort(
      ([a], [b]) =>
        (WORKSPACE_CATEGORIES[a]?.order || 99) - (WORKSPACE_CATEGORIES[b]?.order || 99),
    );

    // 构建动态子路由
    const dynamicRoutes = sortedCategories.map(([category, categoryConfigs]) => {
      const categoryConfig = WORKSPACE_CATEGORIES[category] || WORKSPACE_CATEGORIES.other;
      return {
        path: `/console/${category}`,
        name: categoryConfig.name,
        icon: categoryConfig.icon,
        routes: categoryConfigs.map((config) => ({
          path: `/console/${config.objectKey}`,
          name: config.title,
          component: './Console/Workspace',
        })),
      };
    });

    // 找到 console 路由，添加动态子路由
    const consoleRoute = findRoute(routes, '/console');
    if (consoleRoute) {
      // 保留原有的静态路由
      const existingRoutes = consoleRoute.routes || [];
      // 添加分类分隔符
      consoleRoute.routes = [
        ...existingRoutes,
        { type: 'divider' },
        ...dynamicRoutes,
      ];
    }
  } catch (error) {
    console.error('Failed to load workspace routes:', error);
  }
}

/**
 * 递归查找路由
 */
function findRoute(routes: any[], path: string): any {
  for (const route of routes) {
    if (route.path === path) {
      return route;
    }
    if (route.routes) {
      const found = findRoute(route.routes, path);
      if (found) return found;
    }
  }
  return null;
}
