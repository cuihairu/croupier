import type { MenuDataItem } from '@ant-design/pro-components';
import type { WorkspaceConfig } from '@/types/workspace';
import { buildConsoleWorkspaceMenuItems, type ConsoleWorkspaceMenuItem } from './navigation';

export type AppRouteMenuItem = Omit<MenuDataItem, 'children' | 'routes'> & {
  path?: string;
  key?: string;
  children?: AppRouteMenuItem[];
  routes?: AppRouteMenuItem[];
};

export type AppMenuItem = Omit<AppRouteMenuItem, 'children' | 'routes'> & {
  children?: AppMenuItem[];
};

function isConsoleHomeMenu(item: AppRouteMenuItem): boolean {
  return item.path === '/console/home' || item.key === '/console/home';
}

function getMenuChildren(item: AppRouteMenuItem): AppRouteMenuItem[] {
  return item.children || item.routes || [];
}

function withMenuChildren(item: AppRouteMenuItem, children: AppMenuItem[]): AppMenuItem {
  const { routes, ...menuItem } = item;
  return {
    ...menuItem,
    children,
  };
}

export function injectConsoleWorkspaceMenus(
  menuData: AppRouteMenuItem[],
  workspaceMenuItems: ConsoleWorkspaceMenuItem[],
): AppMenuItem[] {
  return menuData.map((item) => {
    if (item.path === '/console' || item.key === '/console') {
      const staticChildren = getMenuChildren(item).filter(isConsoleHomeMenu).map(stripRoutes);
      return withMenuChildren(item, [...staticChildren, ...workspaceMenuItems]);
    }
    const children = getMenuChildren(item);
    if (children.length > 0) {
      return withMenuChildren(item, injectConsoleWorkspaceMenus(children, workspaceMenuItems));
    }
    return item;
  });
}

function stripRoutes(item: AppRouteMenuItem): AppMenuItem {
  const { routes, children, ...menuItem } = item;
  if (!children?.length && !routes?.length) return menuItem;
  return {
    ...menuItem,
    children: getMenuChildren(item).map(stripRoutes),
  };
}

export function buildConsoleMenuData(
  menuData: AppRouteMenuItem[],
  workspaceConfigs: WorkspaceConfig[] | undefined,
): AppMenuItem[] {
  const configs = Array.isArray(workspaceConfigs) ? workspaceConfigs : [];
  const workspaceMenuItems = buildConsoleWorkspaceMenuItems(configs);
  if (workspaceMenuItems.length === 0) return menuData.map(stripRoutes);
  return injectConsoleWorkspaceMenus(menuData, workspaceMenuItems);
}
