import { Footer, Question, SelectLang, AvatarDropdown, AvatarName } from '@/components';
import MessagesBell from '@/components/MessagesBell';
import { LinkOutlined } from '@ant-design/icons';
import type { Settings as LayoutSettings } from '@ant-design/pro-components';
import { SettingDrawer } from '@ant-design/pro-components';
import type { RunTimeLayoutConfig } from '@umijs/max';
import { FormattedMessage, getLocale, history, Link } from '@umijs/max';
import GameSelector from '@/components/GameSelector';
import defaultSettings from '../config/defaultSettings';
import { errorConfig } from './requestErrorConfig';
import { fetchCurrentUser, getMyPermissions } from '@/services/api';
import React, { useEffect } from 'react';
import { App as AntdApp, Grid } from 'antd';
import { clearAppApi, setAppApi } from './utils/antdApp';
import { getConsoleMenu } from './services/console';
import type { ProfilePermission } from '@/services/api/me';
import { loadAuthedInitialState, type InitialCurrentUser } from './services/initialState';
import type { ServerFeatures } from './services/api/features';
import { fetchSiteConfig, type SiteConfig } from './services/api/sites';
import { getScope, subscribeScope, type Scope } from './stores/scope';
import {
  buildMenuFromConsoleSpec,
  CONSOLE_MENU_REFRESH_EVENT,
  type RuntimeMenuItem,
} from './utils/consoleMenu';
import { resetAccessibleMenus } from './store/modules/menu';
import { AvatarFallback, avatarInitials } from '@/components/UserAvatar';
import { normalizeAvatarSrc } from '@/pages/Profile/shared';

const isDev = process.env.NODE_ENV === 'development';
const loginPath = '/user/login';

/**
 * 把 <AntdApp> 的 message/notification/modal 实例注册给非 React 模块
 * （utils/antdApp）。必须在 rootContainer 层包裹**全部路由**：登录页是
 * `layout: false`，不经过布局的 childrenRender——此前注册器只挂在已登录
 * 布局里，登录页上「登录成功/失败/MFA 提示」的 message 全部被静默丢弃，
 * antd 还会报 "notice in render" 告警（docs/BUGS.md BUG-022）。
 * 卸载时对称清理，避免残留指向已卸载 holder 的死实例。
 */
const AppApiRegistrar: React.FC = () => {
  const inst = AntdApp.useApp();
  useEffect(() => {
    setAppApi(inst);
    return () => clearAppApi(inst);
  }, [inst]);
  return null;
};

// umi 运行时 rootContainer：包裹整个应用（含 layout:false 的登录页）。
export const rootContainer = (container: React.ReactNode) => (
  <AntdApp>
    <AppApiRegistrar />
    {container}
  </AntdApp>
);

type PermissionResponse = {
  permissions?: ProfilePermission[];
  permissionIDs?: string[];
};

type InitialState = {
  settings?: Partial<LayoutSettings>;
  currentUser?: InitialCurrentUser;
  loading?: boolean;
  fetchUserInfo?: () => Promise<InitialCurrentUser | undefined>;
  scope?: Scope;
  consoleMenuRevision?: number;
  features?: ServerFeatures;
  siteConfig?: SiteConfig;
};

function normalizePermissionIDs(perms: PermissionResponse | undefined): string[] {
  const ids = perms?.permissionIDs || [];
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
        // 此前漏传，顶栏头像恒为占位图标（docs/BUGS.md BUG-012）。
        avatar: normalizeAvatarSrc(currentUser.avatar),
        nickname: currentUser.nickname || currentUser.username,
      };
    } catch (error: unknown) {
      const status = (error as { response?: { status?: number } })?.response?.status;
      if (status === 401 || status === 400) {
        localStorage.removeItem('token');
      }
      // 身份失效：可访问菜单缓存一并失效，避免下个登录复用旧身份的菜单
      resetAccessibleMenus();
      history.push(loginPath);
      return undefined;
    }
  };

  const { location } = history;
  if (location.pathname !== loginPath) {
    const authedState = await loadAuthedInitialState(fetchUserInfo);
    // 站点配置（公开端点，fail-open 到构建期默认）
    let siteConfig: SiteConfig | undefined;
    try {
      siteConfig = await fetchSiteConfig();
    } catch {
      siteConfig = undefined;
    }

    return {
      fetchUserInfo,
      ...authedState,
      scope: getScope(),
      settings: {
        ...(defaultSettings as Partial<LayoutSettings>),
        ...(siteConfig?.siteName ? { title: siteConfig.siteName } : {}),
      },
      siteConfig,
    };
  }
  return {
    fetchUserInfo,
    scope: getScope(),
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

  const ScopeMenuRefresher: React.FC = () => {
    useEffect(
      () =>
        subscribeScope((scope) => {
          setInitialState((previous) => ({
            ...previous,
            scope,
          }));
        }),
      [],
    );
    useEffect(() => {
      const refreshMenu = () => {
        setInitialState((previous) => ({
          ...previous,
          consoleMenuRevision: (previous?.consoleMenuRevision || 0) + 1,
        }));
      };
      window.addEventListener(CONSOLE_MENU_REFRESH_EVENT, refreshMenu);
      return () => window.removeEventListener(CONSOLE_MENU_REFRESH_EVENT, refreshMenu);
    }, []);
    return null;
  };

  return {
    // 站点配置覆盖（logo/title），未配置回退构建期默认
    ...(initialState?.siteConfig?.logoUrl ? { logo: initialState.siteConfig.logoUrl } : {}),
    actionsRender: () => [<HeaderActions key="header-actions" />],
    splitMenus: false,
    suppressSiderWhenMenuEmpty: true,
    menu: {
      locale: true,
      params: {
        authed: isAuthed,
        gameId: initialState?.scope?.gameId || '',
        env: initialState?.scope?.env || '',
        consoleMenuRevision: initialState?.consoleMenuRevision || 0,
        // 语言切换时重新请求并按新 locale 解析动态菜单标签。
        locale: typeof getLocale === 'function' ? getLocale() : '',
      },
      request: async (params, defaultMenuData) => {
        if (!params.authed) return defaultMenuData;

        // Load menu from Console API
        try {
          const locale = getLocale();
          const consoleMenu = await getConsoleMenu(locale);
          // /console 子树由 ConsoleMenuSpec 唯一驱动：后端已从 menu_items
          // （menu.AccessibleTree 权限过滤）+ 挂载页面组装，前端不再叠加
          // accessible 菜单合并层。
          return buildMenuFromConsoleSpec(
            defaultMenuData as RuntimeMenuItem[],
            consoleMenu,
            locale,
          );
        } catch (error) {
          console.error('[console-menu] failed to load dynamic runtime menu', error);
          throw error;
        }
      },
    },
    avatarProps: {
      src: initialState?.currentUser?.avatar,
      // 无头像时用姓名首字母占位，不再是清一色的通用图标（BUG-012）。
      icon: initialState?.currentUser?.avatar ? undefined : (
        <AvatarFallback
          initials={avatarInitials(
            initialState?.currentUser?.nickname || initialState?.currentUser?.name,
            initialState?.currentUser?.userid,
          )}
        />
      ),
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
            <span>
              <FormattedMessage id="app.layout.openapiDocs" defaultMessage="OpenAPI 文档" />
            </span>
          </Link>,
        ]
      : [],
    menuHeaderRender: undefined,
    childrenRender: (children) => {
      // AntdApp 上下文已在 rootContainer 层全局提供（含登录页），此处不再重复包裹。
      return (
        <>
          <ScopeMenuRefresher />
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
        </>
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
