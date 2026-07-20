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
import { listPublishedWorkspaceConfigs } from './services/workspaceConfig';
import {
  buildConsoleWorkspaceMenuItems,
  canAccessWorkspaceMenu,
  type ConsoleWorkspaceMenuItem,
} from './services/workspace/navigation';

const isDev = process.env.NODE_ENV === 'development';
const loginPath = '/user/login';

type InitialCurrentUser = {
  name?: string;
  userid?: string;
  access?: string;
  roles?: string[];
  avatar?: string;
};

type PermissionResponse = {
  permissionIDs?: string[];
  permissionIds?: string[];
  permission_ids?: string[];
};

type AppMenuItem = {
  key?: string;
  path?: string;
  name?: string;
  children?: AppMenuItem[];
  routes?: AppMenuItem[];
  [key: string]: unknown;
};

function normalizePermissionIDs(perms: PermissionResponse | undefined): string[] {
  const ids = perms?.permissionIDs || perms?.permissionIds || perms?.permission_ids || [];
  return Array.isArray(ids) ? ids : [];
}

function injectConsoleWorkspaceMenus(
  menuData: AppMenuItem[],
  workspaceMenuItems: ConsoleWorkspaceMenuItem[],
): AppMenuItem[] {
  if (workspaceMenuItems.length === 0) return menuData;
  return menuData.map((item) => {
    if (item.path === '/console' || item.key === '/console') {
      const staticChildren = (item.children || []).filter((child) => child.path === '/console/home');
      return {
        ...item,
        children: [...staticChildren, ...workspaceMenuItems],
      };
    }
    if (item.children) {
      return {
        ...item,
        children: injectConsoleWorkspaceMenus(item.children, workspaceMenuItems),
      };
    }
    return item;
  });
}

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
        const perms = (await getMyPermissions()) as PermissionResponse;
        permissionIDs = normalizePermissionIDs(perms);
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
      };
    } catch (error: unknown) {
      const status = (error as { response?: { status?: number } })?.response?.status;
      if (status === 401 || status === 400) {
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
    actionsRender: () => [<HeaderActions key="header-actions" />],
    splitMenus: false,
    suppressSiderWhenMenuEmpty: true,
    menu: {
      request: async (_params, defaultMenuData) => {
        if (!initialState?.currentUser) return defaultMenuData;
        try {
          const configs = await listPublishedWorkspaceConfigs({ skipErrorHandler: true });
          const visibleConfigs = configs.filter((config) =>
            canAccessWorkspaceMenu(config, initialState.currentUser),
          );
          return injectConsoleWorkspaceMenus(
            defaultMenuData as AppMenuItem[],
            buildConsoleWorkspaceMenuItems(visibleConfigs),
          );
        } catch {
          return defaultMenuData;
        }
      },
    },
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
