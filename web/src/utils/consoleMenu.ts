import type { MenuDataItem } from '@ant-design/pro-components';
import type { ConsoleMenuItem, ConsoleMenuSpec, LocalizedText } from '@/types/dashboard';
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

/**
 * 在 ConsoleMenuSpec 树中递归查找 pageKey 的规范路径。
 * URL 的 categoryKey 段是挂载菜单 key（menu_items 驱动）：页面挂到哪个
 * 菜单，规范路径就是 /console/{menuKey}/{pageKey}。树中未找到（未挂
 * 菜单的直达 URL）返回空串——页面仍按 pageKey 渲染，不做重定向。
 */
export function resolveConsolePageCanonicalPath(
  menu: ConsoleMenuSpec | null | undefined,
  pageKey: string,
): string {
  if (!pageKey) return '';
  const visit = (items: ConsoleMenuItem[]): string => {
    for (const item of items) {
      for (const child of item.children || []) {
        if (child.key === pageKey) {
          return child.path || buildConsolePagePath(item.key, pageKey);
        }
      }
      const nested = visit((item.children || []) as ConsoleMenuItem[]);
      if (nested) return nested;
    }
    return '';
  };
  return visit(((menu?.items || []) as unknown as ConsoleMenuItem[]) || []);
}

/** ConsoleMenuSpec 子节点 → 侧边栏菜单项。
 * 子项可能是挂载页面（叶子）或子菜单（菜单树任意层级嵌套）——
 * menu_items 驱动后树深不限，递归展开。 */
const toConsoleChild = (node: ConsoleMenuItem, locale: string): RuntimeMenuItem => ({
  key: node.path,
  path: node.path,
  name: resolveLocalizedText(node.title, locale, node.key),
  locale: false,
  icon: resolveMenuIcon(node.icon),
  ...(node.children?.length
    ? { children: node.children.map((child) => toConsoleChild(child, locale)) }
    : {}),
});

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
        // 分类项此前从不带 icon——后端已回填组内首个非空图标，子项透传自身 icon。
        icon: resolveMenuIcon(category.icon),
        children: (category.children || []).map((child) => toConsoleChild(child, locale)),
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
