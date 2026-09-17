import { request } from '@umijs/max';
import type { LocalizedText } from '@/types/dashboard';
import { normalizeLocalizedText } from '@/services/api/functions-enhanced';

/** 菜单节点（GET /api/v1/menus 返回的树形 DTO，契约键 lowerCamelCase）。 */
export interface MenuItem {
  id: number;
  parentId: number | null;
  menuKey: string;
  labels: LocalizedText;
  icon?: string;
  sortOrder: number;
  permission?: string;
  isVisible: boolean;
  children: MenuItem[];
}

/** 创建菜单载荷。 */
export interface MenuCreatePayload {
  menuKey: string;
  /** 0/省略 = 顶级菜单。 */
  parentId?: number;
  labels: LocalizedText;
  icon?: string;
  sortOrder?: number;
  permission?: string;
  isVisible?: boolean;
}

/**
 * 更新菜单载荷（部分更新：仅提交出现的字段）。
 * parentId 语义：0 = 移到顶级；省略/undefined = 保持不变。
 */
export interface MenuUpdatePayload {
  menuKey?: string;
  parentId?: number;
  labels?: LocalizedText;
  icon?: string;
  sortOrder?: number;
  permission?: string;
  isVisible?: boolean;
}

type RawMenuItem = Omit<MenuItem, 'labels' | 'children'> & {
  labels?: LocalizedText | Record<string, string> | string;
  children?: RawMenuItem[];
};

/** 服务边界归一：labels 统一 BCP47 key，children 保证数组。 */
function normalizeMenuItem(raw: RawMenuItem): MenuItem {
  return {
    id: raw.id,
    parentId: raw.parentId ?? null,
    menuKey: raw.menuKey,
    labels: normalizeLocalizedText(raw.labels) ?? {},
    icon: raw.icon,
    sortOrder: raw.sortOrder ?? 0,
    permission: raw.permission,
    isVisible: raw.isVisible !== false,
    children: (raw.children ?? []).map(normalizeMenuItem),
  };
}

/** 获取当前 scope 的完整菜单树。 */
export async function listMenus(): Promise<MenuItem[]> {
  const res = await request<{ items?: RawMenuItem[] } | RawMenuItem[]>('/api/v1/menus');
  const items = Array.isArray(res) ? res : res?.items || [];
  return items.map(normalizeMenuItem);
}

/** 获取当前用户可访问的菜单树（服务端按可见性+权限继承过滤）。 */
export async function listAccessibleMenus(): Promise<MenuItem[]> {
  const res = await request<{ items?: RawMenuItem[] } | RawMenuItem[]>('/api/v1/menus/accessible');
  const items = Array.isArray(res) ? res : res?.items || [];
  return items.map(normalizeMenuItem);
}

/** 创建菜单。 */
export async function createMenu(payload: MenuCreatePayload): Promise<MenuItem> {
  const res = await request<RawMenuItem>('/api/v1/menus', { method: 'POST', data: payload });
  return normalizeMenuItem(res);
}

/** 更新菜单（部分字段）。 */
export async function updateMenu(id: number, payload: MenuUpdatePayload): Promise<MenuItem> {
  const res = await request<RawMenuItem>(`/api/v1/menus/${id}`, {
    method: 'PUT',
    data: payload,
  });
  return normalizeMenuItem(res);
}

/** 删除菜单（后端级联删除全部子孙菜单并解除页面挂载）。 */
export async function deleteMenu(id: number): Promise<void> {
  await request<void>(`/api/v1/menus/${id}`, { method: 'DELETE' });
}

/** 更新单个菜单排序。 */
export async function updateMenuSort(id: number, sortOrder: number): Promise<MenuItem> {
  const res = await request<RawMenuItem>(`/api/v1/menus/${id}/sort`, {
    method: 'PUT',
    data: { sortOrder },
  });
  return normalizeMenuItem(res);
}
