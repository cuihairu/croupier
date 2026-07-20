import type { MenuDataItem } from '@ant-design/pro-components';
import type { WorkspaceConfig } from '@/types/workspace';
import { buildConsoleWorkspaceMenuItems, type ConsoleWorkspaceMenuItem } from './navigation';

export type AppMenuItem = MenuDataItem & {
  path?: string;
  key?: string;
  children?: AppMenuItem[];
};

function isConsoleHomeMenu(item: AppMenuItem): boolean {
  return item.path === '/console/home' || item.key === '/console/home';
}

export function injectConsoleWorkspaceMenus(
  menuData: AppMenuItem[],
  workspaceMenuItems: ConsoleWorkspaceMenuItem[],
): AppMenuItem[] {
  return menuData.map((item) => {
    if (item.path === '/console' || item.key === '/console') {
      const staticChildren = (item.children || []).filter(isConsoleHomeMenu);
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

export function buildConsoleMenuData(
  menuData: AppMenuItem[],
  workspaceConfigs: WorkspaceConfig[] | undefined,
): AppMenuItem[] {
  const configs = Array.isArray(workspaceConfigs) ? workspaceConfigs : [];
  const workspaceMenuItems = buildConsoleWorkspaceMenuItems(configs);
  if (workspaceMenuItems.length === 0) return menuData;
  return injectConsoleWorkspaceMenus(menuData, workspaceMenuItems);
}
