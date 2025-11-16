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

async function fetchCurrentAccess(): Promise<{ roles: string[]; accessSet: Set<string> }> {
  try {
    const me: any = await request('/api/auth/me', { method: 'GET' });
    const roles: string[] = me?.roles || [];
    // try extended access CSV if available
    const accessCsv: string | undefined = (me?.access as string) || '';
    const set = new Set<string>((accessCsv || '').split(',').map((s) => s.trim()).filter(Boolean));
    // basic roles expansion
    if (roles.includes('admin')) {
      set.add('*');
    }
    if (roles.includes('operator') || roles.includes('admin')) {
      set.add('functions:read');
    }
    return { roles, accessSet: set };
  } catch {
    return { roles: [], accessSet: new Set<string>() };
  }
}

function buildDynamicMenu(funcs: FuncItem[], accessSet: Set<string>) {
  const has = (p: string) => accessSet.has('*') || accessSet.has(p);
  // Group by section -> group
  const sections: Record<string, any> = {};
  for (const f of funcs) {
    const m = f.menu || {};
    if (m.hidden) continue;
    // permission gate: admin|functions:read|function:{id}:read|function:{id}:invoke
    const fid = f.id;
    if (
      !has('functions:read') &&
      !has(`function:${fid}:read`) &&
      !has(`function:${fid}:invoke`) &&
      !has('*')
    ) {
      continue;
    }
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
        const [funcs, me] = await Promise.all([fetchFunctions(), fetchCurrentAccess()]);
        const dyn = buildDynamicMenu(funcs, me.accessSet);
        // Merge: keep existing menu first, then dynamic sections
        return [...(defaultMenuData || []), ...dyn];
      },
    },
  };
};
