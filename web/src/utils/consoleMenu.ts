import type { MenuDataItem } from '@ant-design/pro-components';
import type { ConsoleMenuSpec, LocalizedText, PublishedPageSpec } from '@/types/dashboard';

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
  return (
    text[normalizedLocale] ||
    text[normalizedLocale.toLowerCase()] ||
    text['zh-CN'] ||
    text['en-US'] ||
    Object.values(text).find((value) => value.trim() !== '') ||
    fallback
  );
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
  if (!consoleMenu?.items || consoleMenu.items.length === 0) {
    return defaultMenuData;
  }

  return defaultMenuData.map((item): RuntimeMenuItem => {
    if (item.path === '/console' || item.key === '/console') {
      const homeChild = (item.children || []).find((child) => child.path === '/console/home');
      const dynamicChildren: RuntimeMenuItem[] = consoleMenu.items.map((category) => ({
        key: category.path,
        path: category.path,
        name: resolveLocalizedText(category.title, locale, category.key),
        locale: false,
        icon: category.icon,
        children: (category.children || []).map((page) => ({
          key: page.path,
          path: page.path,
          name: resolveLocalizedText(page.title, locale, page.key),
          locale: false,
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
