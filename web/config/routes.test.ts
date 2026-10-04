/**
 * 菜单结构回归（OPEN-ISSUES #8/#9 落地版 8a858e0）
 *
 * #8 账号中心合并：一层「个人中心」（页内 tab），旧消息/设置路径仅重定向；
 * #9 基础配置扁平化：「系统管理」下不再上卷「基础配置」分组，
 *    旧 /system/foundation/* 路径保留 hideInMenu 重定向兼容书签。
 *
 * 断言对象是路由配置数据本身（菜单项 = 有 name 且未 hideInMenu 的节点），
 * 不渲染组件——8a858e0 落地时未带测试，此处补齐防结构回退。
 */
import routes from './routes';

type RouteNode = {
  path?: string;
  name?: string;
  redirect?: string;
  hideInMenu?: boolean;
  component?: string;
  routes?: RouteNode[];
};

/** 深度收集全部节点 */
const flatten = (nodes: RouteNode[] | undefined): RouteNode[] =>
  (nodes || []).flatMap((n) => [n, ...flatten(n.routes)]);

const ALL = flatten(routes as RouteNode[]);
const byPath = (path: string): RouteNode => {
  const hit = ALL.find((n) => n.path === path);
  if (!hit) throw new Error(`route ${path} not found`);
  return hit;
};

/** 菜单可见子项 = 有 name 且未 hideInMenu 的直接子节点 */
const visibleChildren = (node: RouteNode): RouteNode[] =>
  (node.routes || []).filter((n) => n.name && !n.hideInMenu);

describe('#9 系统管理扁平化（无「基础配置」上卷分组）', () => {
  const system = byPath('/system');

  it('可见子项直接挂页：游戏管理/环境/术语/站点/分析过滤', () => {
    expect(visibleChildren(system).map((n) => n.path)).toEqual([
      '/system/games',
      '/system/environments',
      '/system/terms',
      '/system/site',
      '/system/analytics-filters',
    ]);
  });

  it('不存在带 name 的 foundation 二级分组', () => {
    const grouped = (system.routes || []).filter(
      (n) => n.name && n.routes && n.path?.includes('foundation'),
    );
    expect(grouped).toEqual([]);
  });

  it('旧 /system/foundation/* 全部为 hideInMenu 重定向且不带 name（不出现在菜单）', () => {
    const legacy = ALL.filter((n) => n.path?.startsWith('/system/foundation'));
    expect(legacy.length).toBeGreaterThanOrEqual(4);
    for (const node of legacy) {
      expect(node.hideInMenu).toBe(true);
      expect(node.redirect).toBeTruthy();
      expect(node.name).toBeUndefined();
    }
  });
});

describe('#8 账号中心合并为单层「个人中心」', () => {
  it('个人中心是唯一可见账号菜单项（页内 tab 承载消息/安全）', () => {
    const center = byPath('/admin/account/center');
    expect(center.name).toBe('UserAccount');
    expect(center.hideInMenu).toBeUndefined();
    expect(center.component).toBe('./Profile');
  });

  it('旧消息/设置/账号路径均为无 name 的重定向（不产生菜单项）', () => {
    const legacyPaths = [
      '/admin/account',
      '/admin/account/settings',
      '/admin/account/messages',
      '/account',
      '/account/center',
      '/account/settings',
      '/account/messages',
    ];
    for (const p of legacyPaths) {
      const node = byPath(p);
      expect(node.redirect).toBeTruthy();
      expect(node.name).toBeUndefined();
    }
  });

  it('消息 tab 语义保留：messages 路由指向个人中心 notifications tab', () => {
    expect(byPath('/admin/account/messages').redirect).toBe(
      '/admin/account/center?tab=notifications',
    );
    expect(byPath('/account/settings').redirect).toBe('/admin/account/center?tab=security');
  });
});
