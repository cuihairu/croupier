import type { MenuDataItem } from '@ant-design/pro-components';
import type { WorkspaceConfig } from '@/types/workspace';

export type AppRouteMenuItem = Omit<MenuDataItem, 'children' | 'routes'> & {
  path?: string;
  key?: string;
  children?: AppRouteMenuItem[];
  routes?: AppRouteMenuItem[];
};

export type AppMenuItem = Omit<AppRouteMenuItem, 'children' | 'routes'> & {
  children?: AppMenuItem[];
};

function unsupportedWorkspaceMenu(): never {
  throw new Error('旧 WorkspaceConfig 动态菜单已删除；运行控制台必须使用 ConsoleMenuSpec。');
}

export function injectConsoleWorkspaceMenus(): AppMenuItem[] {
  return unsupportedWorkspaceMenu();
}

export function buildConsoleMenuData(
  _menuData: AppRouteMenuItem[],
  _workspaceConfigs: WorkspaceConfig[] | undefined,
): AppMenuItem[] {
  return unsupportedWorkspaceMenu();
}
