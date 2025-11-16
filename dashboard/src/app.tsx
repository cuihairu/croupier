import type { RunTimeLayoutConfig } from '@umijs/max';
import { request } from '@umijs/max';

type I18N = { zh?: string; en?: string };
type MenuMeta = { section?: string; group?: string; path?: string; order?: number; icon?: string; badge?: string; hidden?: boolean };
type FuncItem = { id: string; display_name?: I18N; menu?: MenuMeta };

async function fetchFunctions(): Promise<FuncItem[]> {
  try {
    const res: any = await request('/api/functions/summary', { method: 'GET' });
    if (Array.isArray(res)) return res;
    if (res && Array.isArray(res.functions)) return res.functions;
    return [];
  } catch {
    return [];
  }
}

function buildDynamicMenu(funcs: FuncItem[]) {
  // Group by section -> group
  const sections: Record<string, any> = {};
  for (const f of funcs) {
    const m = f.menu || {};
    if (m.hidden) continue;
    const section = m.section || 'Function Management';
    const group = m.group || 'Uncategorized';
    sections[section] = sections[section] || {};
    sections[section][group] = sections[section][group] || [];
    sections[section][group].push({
      name: (f.display_name?.zh || f.display_name?.en || f.id),
      path: (m.path || '/GmFunctions') + `?fid=${encodeURIComponent(f.id)}`,
      icon: m.icon,
      order: m.order || 0,
    });
  }
  // Compose menu tree
  const menu: any[] = [];
  Object.keys(sections).forEach((section) => {
    const groups = sections[section];
    const children: any[] = [];
    Object.keys(groups).forEach((g) => {
      const items = groups[g].sort((a: any, b: any) => a.order - b.order);
      children.push({
        name: g,
        path: `/${encodeURIComponent(section)}/${encodeURIComponent(g)}`,
        routes: items,
      });
    });
    menu.push({
      name: section,
      path: `/${encodeURIComponent(section)}`,
      routes: children,
    });
  });
  return menu;
}

export const layout: RunTimeLayoutConfig = () => {
  return {
    // Merge dynamic menus with existing route-based menus
    menu: {
      // request is supported by ProLayout; when present, it overrides default menuData
      // We'll merge default with dynamic by returning concatenation.
      request: async (params, defaultMenuData) => {
        const funcs = await fetchFunctions();
        const dyn = buildDynamicMenu(funcs);
        // Merge: keep existing menu first, then dynamic sections
        return [...(defaultMenuData || []), ...dyn];
      },
    },
  };
};

