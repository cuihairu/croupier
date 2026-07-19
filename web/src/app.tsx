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

// ============================================================
// 动态路由注入（运行控制台左侧菜单）
// 使用 fetch 而非 import 以避免 tree-shaking 问题
// ============================================================

type WorkspaceConfig = {
  objectKey: string;
  title: string;
  category?: string;
  permissions?: { roles?: string[] };
};

/** 分类配置（内联，避免 import 被 tree-shake） */
const CATEGORIES: Record<string, { name: string; order: number }> = {
  player: { name: '玩家管理', order: 1 },
  inventory: { name: '物品管理', order: 2 },
  order: { name: '订单管理', order: 3 },
  economy: { name: '经济系统', order: 4 },
  social: { name: '社交系统', order: 5 },
  other: { name: '其他工具', order: 99 },
};

/** 缓存已加载的工作台配置 */
let _wsConfigs: WorkspaceConfig[] = [];
let _wsLoaded = false;

/**
 * 在渲染前加载工作台配置
 */
export function render(oldRender: () => void) {
  const token = localStorage.getItem('token');
  if (!token) {
    _wsLoaded = true;
    oldRender();
    return;
  }

  fetch('/api/v1/workspaces/published', {
    headers: { Authorization: `Bearer ${token}` },
  })
    .then((res) => (res.ok ? res.json() : { items: [] }))
    .then((data) => {
      _wsConfigs = Array.isArray(data?.items) ? data.items : [];
    })
    .catch(() => {
      _wsConfigs = [];
    })
    .finally(() => {
      _wsLoaded = true;
      oldRender();
    });
}

/**
 * 动态注入运行控制台路由
 */
export function patchClientRoutes({ routes }: { routes: any[] }) {
  if (!_wsLoaded || _wsConfigs.length === 0) {
    return;
  }

  // 权限过滤
  const initialState = (window as any).__INITIAL_STATE__?.currentUser;
  const userRoles: string[] = initialState?.roles || [];
  const isAdmin = userRoles.some(
    (r: string) => r.toLowerCase() === 'admin' || r.toLowerCase() === 'super_admin',
  );

  const filtered = isAdmin
    ? _wsConfigs
    : _wsConfigs.filter((c) => {
        if (!c.permissions?.roles?.length) return true;
        return c.permissions.roles.some((role) =>
          userRoles.some((ur) => ur.toLowerCase() === role.toLowerCase()),
        );
      });

  if (filtered.length === 0) return;

  // 按分类分组
  const grouped = new Map<string, WorkspaceConfig[]>();
  filtered.forEach((c) => {
    const cat = c.category || 'other';
    if (!grouped.has(cat)) grouped.set(cat, []);
    grouped.get(cat)!.push(c);
  });

  // 排序
  const sorted = Array.from(grouped.entries()).sort(
    ([a], [b]) => (CATEGORIES[a]?.order || 99) - (CATEGORIES[b]?.order || 99),
  );

  // 构建动态路由
  const dynamicRoutes = sorted.map(([cat, configs]) => ({
    path: `/console/${cat}`,
    name: CATEGORIES[cat]?.name || cat,
    routes: configs.map((c) => ({
      path: `/console/${c.objectKey}`,
      name: c.title,
      component: './Console/Workspace',
    })),
  }));

  // 找到 console 路由并注入
  const findAndPatch = (routeList: any[]): boolean => {
    for (const route of routeList) {
      if (route.path === '/console') {
        const existing = route.routes || [];
        route.routes = [...existing, { type: 'divider' }, ...dynamicRoutes];
        return true;
      }
      if (route.routes && findAndPatch(route.routes)) return true;
    }
    return false;
  };

  findAndPatch(routes);
}

