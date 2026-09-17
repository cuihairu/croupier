import type { MenuDataItem } from '@ant-design/pro-components';
import type { MenuItem } from '@/services/api/menu';
import type { ConsoleMenuSpec, LocalizedText, PublishedPageSpec } from '@/types/dashboard';
import { localizedText } from '@/utils/localizedText';
import { resolveMenuIcon } from '@/utils/menuIcon';

export const CONSOLE_MENU_REFRESH_EVENT = 'console-menu:refresh';

export type RuntimeMenuItem = MenuDataItem & {
  children?: RuntimeMenuItem[];
};

export function requestConsoleMenuRefresh(): void {
  if (typeof window === 'undefined') return;
  window.dispatchEvent(new Event(CONSOLE_MENU_REFRESH_EVENT));
}

export function resolveLocalizedText(
  text: LocalizedText | undefined,
  locale: string,
  fallback: string,
): string {
  if (!text) return fallback;
  const normalizedLocale = locale.replace('_', '-');
  const exact = text[normalizedLocale] || text[normalizedLocale.toLowerCase()];
  if (exact) return exact;
  return localizedText(text, locale, fallback);
}

export function buildConsolePagePath(categoryKey: string, pageKey: string): string {
  return `/console/${encodeURIComponent(categoryKey)}/${encodeURIComponent(pageKey)}`;
}

export function resolveConsolePageRoute(
  page: Pick<PublishedPageSpec, 'category' | 'pageKey'> | null | undefined,
  currentCategoryKey: string,
): { canonicalPath: string; shouldRedirect: boolean } {
  const actualCategoryKey = page?.category?.key?.trim() || '';
  if (!page || !actualCategoryKey) {
    return { canonicalPath: '', shouldRedirect: false };
  }

  const canonicalPath = buildConsolePagePath(actualCategoryKey, page.pageKey);
  return {
    canonicalPath,
    shouldRedirect: actualCategoryKey !== currentCategoryKey,
  };
}

/**
 * 运行控制台动态菜单只来自 ConsoleMenuSpec。
 * 这里只合并到静态控制台根菜单，不引入 locale key 或旧 workspace 来源。
 */
export function buildMenuFromConsoleSpec(
  defaultMenuData: RuntimeMenuItem[],
  consoleMenu: ConsoleMenuSpec,
  locale: string,
): MenuDataItem[] {
  return defaultMenuData.map((item): RuntimeMenuItem => {
    if (item.path === '/console' || item.key === '/console') {
      const homeChild = (item.children || []).find((child) => child.path === '/console/home');
      const dynamicChildren: RuntimeMenuItem[] = (consoleMenu?.items || []).map((category) => ({
        key: category.path,
        path: category.path,
        name: resolveLocalizedText(category.title, locale, category.key),
        locale: false,
        // 后端分类项此前从不带 icon、前端也丢弃子项 page.icon——两端字段
        // 位置互错导致菜单永远无图标。分类取组内首个非空页面图标（后端
        // 已回填），子项透传自身 icon。
        icon: resolveMenuIcon(category.icon),
        children: (category.children || []).map((page) => ({
          key: page.path,
          path: page.path,
          name: resolveLocalizedText(page.title, locale, page.key),
          locale: false,
          icon: resolveMenuIcon(page.icon),
        })),
      }));

      return {
        ...item,
        children: [...(homeChild ? [homeChild] : []), ...dynamicChildren],
      };
    }

    if (item.children && item.children.length > 0) {
      return {
        ...item,
        children: buildMenuFromConsoleSpec(item.children, consoleMenu, locale) as RuntimeMenuItem[],
      };
    }

    return item;
  });
}

/**
 * 用用户可访问菜单树（menu_items，T-M7）驱动 /console 动态子树：
 * 菜单节点 → 子菜单/分组；已发布页面按其分类 key（=迁移后的菜单 key）
 * 挂到同名菜单下。无权限菜单服务端已过滤，这里不再出现。
 *
 * pagesByMenuKey：consoleMenu 里各分类（key=menuKey）及其页面；
 * 未匹配到任何菜单的分类页面追加到「未分类」分组兜底，不静默丢弃。
 */
export function buildConsoleMenuFromAccessibleMenus(
  defaultMenuData: RuntimeMenuItem[],
  menus: MenuItem[],
  consoleMenu: ConsoleMenuSpec,
  locale: string,
): RuntimeMenuItem[] {
  const categories = new Map<string, ConsoleMenuSpec['items'][number]>();
  for (const category of consoleMenu?.items || []) {
    categories.set(category.key, category);
  }
  const consumedKeys = new Set<string>();

  const toRuntimeItem = (menu: MenuItem): RuntimeMenuItem => {
    const category = categories.get(menu.menuKey);
    if (category) consumedKeys.add(category.key);
    const childMenus = (menu.children || []).map(toRuntimeItem);
    const pages: RuntimeMenuItem[] = (category?.children || []).map((page) => ({
      key: page.path,
      path: page.path,
      name: resolveLocalizedText(page.title, locale, page.key),
      locale: false,
      icon: resolveMenuIcon(page.icon),
    }));
    const children = [...childMenus, ...pages];
    return {
      key: `/console/${menu.menuKey}`,
      // 叶子菜单（无子菜单无页面）也可点：落到分类路由空态页
      path: children.length > 0 ? undefined : `/console/${encodeURIComponent(menu.menuKey)}`,
      name: resolveLocalizedText(menu.labels, locale, menu.menuKey),
      locale: false,
      icon: resolveMenuIcon(menu.icon || category?.icon),
      children: children.length > 0 ? children : undefined,
    };
  };

  const dynamicChildren = menus.map(toRuntimeItem);

  // 迁移期兜底：分类页面挂到了尚未创建/不可见的菜单下时不丢弃
  const orphans = (consoleMenu?.items || [])
    .filter((category) => !consumedKeys.has(category.key) && (category.children || []).length > 0)
    .map((category): RuntimeMenuItem => ({
      key: category.path,
      path: category.path,
      name: resolveLocalizedText(category.title, locale, category.key),
      locale: false,
      icon: resolveMenuIcon(category.icon),
      children: (category.children || []).map((page) => ({
        key: page.path,
        path: page.path,
        name: resolveLocalizedText(page.title, locale, page.key),
        locale: false,
        icon: resolveMenuIcon(page.icon),
      })),
    }));

  return defaultMenuData.map((item): RuntimeMenuItem => {
    if (item.path === '/console' || item.key === '/console') {
      const homeChild = (item.children || []).find((child) => child.path === '/console/home');
      return {
        ...item,
        children: [...(homeChild ? [homeChild] : []), ...dynamicChildren, ...orphans],
      };
    }
    if (item.children && item.children.length > 0) {
      return {
        ...item,
        children: buildConsoleMenuFromAccessibleMenus(
          item.children,
          menus,
          consoleMenu,
          locale,
        ) as RuntimeMenuItem[],
      };
    }
    return item;
  });
}
