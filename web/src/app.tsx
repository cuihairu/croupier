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
import React, { useEffect } from 'react';
import { App as AntdApp, Grid } from 'antd';
import { setAppApi } from './utils/antdApp';
import { initWorkspaceAlerting } from './services/workspace/alerts';
import { buildConsoleMenuData, type AppRouteMenuItem } from './services/workspace/menu';
import type { WorkspaceConfig } from './types/workspace';
import { loadAuthedInitialState, type InitialCurrentUser } from './services/initialState';

const isDev = process.env.NODE_ENV === 'development';
const loginPath = '/user/login';

type PermissionResponse = {
  permissionIDs?: string[];
  permissionIds?: string[];
  permission_ids?: string[];
};

type InitialState = {
  settings?: Partial<LayoutSettings>;
  currentUser?: InitialCurrentUser;
  loading?: boolean;
  fetchUserInfo?: () => Promise<InitialCurrentUser | undefined>;
  workspaceConfigs?: WorkspaceConfig[];
};

function normalizePermissionIDs(perms: PermissionResponse | undefined): string[] {
  const ids = perms?.permissionIDs || perms?.permissionIds || perms?.permission_ids || [];
  return Array.isArray(ids) ? ids : [];
}

/**
 * @see  https://umijs.org/zh-CN/plugins/plugin-initial-state
 * */
export async function getInitialState(): Promise<InitialState> {
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
    const authedState = await loadAuthedInitialState(fetchUserInfo);

    return {
      fetchUserInfo,
      ...authedState,
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
      locale: true,
      params: {
        workspaceConfigs: initialState?.workspaceConfigs || [],
        authed: isAuthed,
      },
      request: async (params, defaultMenuData) => {
        if (!params.authed) return defaultMenuData;
        return buildConsoleMenuData(
          defaultMenuData as AppRouteMenuItem[],
          params.workspaceConfigs as WorkspaceConfig[],
        );
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
