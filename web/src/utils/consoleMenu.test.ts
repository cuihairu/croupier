import type { ConsoleMenuSpec } from '@/types/dashboard';
import type { MenuItem } from '@/services/api/menu';
import {
  CONSOLE_MENU_REFRESH_EVENT,
  buildConsolePagePath,
  buildConsoleMenuFromAccessibleMenus,
  buildMenuFromConsoleSpec,
  requestConsoleMenuRefresh,
  resolveConsolePageRoute,
  resolveLocalizedText,
  type RuntimeMenuItem,
} from './consoleMenu';

type ConsolePageArg = Parameters<typeof resolveConsolePageRoute>[0];

describe('requestConsoleMenuRefresh', () => {
  it('向 window 派发全局刷新事件', () => {
    const spy = jest.fn();
    window.addEventListener(CONSOLE_MENU_REFRESH_EVENT, spy);
    requestConsoleMenuRefresh();
    window.removeEventListener(CONSOLE_MENU_REFRESH_EVENT, spy);
    expect(spy).toHaveBeenCalledTimes(1);
    expect((spy.mock.calls[0][0] as Event).type).toBe(CONSOLE_MENU_REFRESH_EVENT);
  });
});

describe('resolveLocalizedText', () => {
  it('text 缺省 → fallback', () => {
    expect(resolveLocalizedText(undefined, 'zh-CN', 'fb')).toBe('fb');
  });

  it('BCP47 locale 直接命中', () => {
    expect(resolveLocalizedText({ 'zh-CN': '中文' }, 'zh-CN', 'fb')).toBe('中文');
  });

  it('下划线 locale 归一后命中（zh_CN → zh-CN）', () => {
    expect(resolveLocalizedText({ 'zh-CN': '中文' }, 'zh_CN', 'fb')).toBe('中文');
  });

  it('大小写归一命中（ZH-CN → zh-cn）', () => {
    expect(resolveLocalizedText({ 'zh-cn': '小写' }, 'ZH-CN', 'fb')).toBe('小写');
  });

  it('exact 未命中回落 localizedText 链（en-US locale 取 zh-CN 值）', () => {
    expect(resolveLocalizedText({ 'zh-CN': '中文' }, 'en-US', 'fb')).toBe('中文');
  });

  it('完全无值 → fallback', () => {
    expect(resolveLocalizedText({}, 'en-US', 'fb')).toBe('fb');
  });
});

describe('buildConsolePagePath', () => {
  it('分段 encodeURIComponent', () => {
    expect(buildConsolePagePath('a b', 'c/d')).toBe('/console/a%20b/c%2Fd');
    expect(buildConsolePagePath('ops', 'home')).toBe('/console/ops/home');
  });
});

describe('resolveConsolePageRoute', () => {
  it('page 为空 → 无规范路径、不重定向', () => {
    expect(resolveConsolePageRoute(undefined, 'cur')).toEqual({
      canonicalPath: '',
      shouldRedirect: false,
    });
    expect(resolveConsolePageRoute(null, 'cur')).toEqual({
      canonicalPath: '',
      shouldRedirect: false,
    });
  });

  it('category 缺失或 key 空白 → 同上', () => {
    expect(resolveConsolePageRoute({ pageKey: 'p' } as ConsolePageArg, 'cur')).toEqual({
      canonicalPath: '',
      shouldRedirect: false,
    });
    expect(
      resolveConsolePageRoute({ pageKey: 'p', category: { key: '   ', labels: {} } }, 'cur'),
    ).toEqual({ canonicalPath: '', shouldRedirect: false });
  });

  it('key 与当前分组一致 → 不重定向', () => {
    const page: ConsolePageArg = { pageKey: 'p', category: { key: 'cat', labels: {} } };
    expect(resolveConsolePageRoute(page, 'cat')).toEqual({
      canonicalPath: '/console/cat/p',
      shouldRedirect: false,
    });
  });

  it('key 与当前分组不一致 → 重定向到规范路径', () => {
    const page: ConsolePageArg = { pageKey: 'p', category: { key: 'cat a', labels: {} } };
    expect(resolveConsolePageRoute(page, 'other')).toEqual({
      canonicalPath: '/console/cat%20a/p',
      shouldRedirect: true,
    });
  });
});

describe('buildMenuFromConsoleSpec', () => {
  const menu = (): RuntimeMenuItem[] => [
    {
      key: '/console',
      path: '/console',
      children: [{ key: '/console/home', path: '/console/home', name: '首页' }],
    },
    { key: '/ops', path: '/ops', children: [{ key: '/ops/a', path: '/ops/a', name: 'A' }] },
    { key: '/leaf', path: '/leaf' },
    { key: '/empty', path: '/empty', children: [] },
  ];

  const spec = (): ConsoleMenuSpec => ({
    items: [
      {
        key: 'ops',
        path: '/console/ops',
        title: { 'zh-CN': '运营', 'en-US': 'Ops' },
        locale: false,
        icon: 'appstore',
        children: [
          {
            key: 'p1',
            path: '/console/ops/p1',
            title: { 'zh-CN': '页一', 'en-US': 'P1' },
            locale: false,
          },
        ],
      },
      { key: 'bare', path: '/console/bare', title: { 'zh-CN': '裸类目' }, locale: false },
    ],
  });

  it('console 根节点：保留 home 子项 + 追加动态类目（含页级 children）', () => {
    const out = buildMenuFromConsoleSpec(menu(), spec(), 'zh-CN');
    const consoleNode = out[0];
    expect(consoleNode.children?.map((c) => c.path)).toEqual([
      '/console/home',
      '/console/ops',
      '/console/bare',
    ]);
    const ops = consoleNode.children![1];
    expect(ops.name).toBe('运营');
    // icon 字符串经 resolveMenuIcon 解析为 ReactNode（ProLayout 需要），
    // 未知名兜底 AppstoreOutlined
    expect(ops.icon).toBeTruthy();
    expect(ops.locale).toBe(false);
    expect(ops.children?.[0].name).toBe('页一');
    expect(ops.children?.[0].path).toBe('/console/ops/p1');
    // 无 children 的类目 → 空数组
    expect(consoleNode.children![2].children).toEqual([]);
    // en-US locale 取英文标签
    const outEn = buildMenuFromConsoleSpec(menu(), spec(), 'en-US');
    expect(outEn[0].children![1].name).toBe('Ops');
  });

  it('title 缺失回落 key', () => {
    const noTitle = { items: [{ key: 'k', path: '/console/k', locale: false }] } as ConsoleMenuSpec;
    const out = buildMenuFromConsoleSpec(menu(), noTitle, 'zh-CN');
    expect(out[0].children![1].name).toBe('k');
  });

  it('key 命中（path 不同）同样走 console 分支', () => {
    const byKey: RuntimeMenuItem[] = [{ key: '/console', path: '/whatever' }];
    const out = buildMenuFromConsoleSpec(byKey, spec(), 'zh-CN');
    expect(out[0].children?.map((c) => c.path)).toEqual(['/console/ops', '/console/bare']);
  });

  it('无 home 子项时动态类目打头', () => {
    const noHome: RuntimeMenuItem[] = [{ path: '/console' }];
    const out = buildMenuFromConsoleSpec(noHome, spec(), 'zh-CN');
    expect(out[0].children?.[0].path).toBe('/console/ops');
  });

  it('空 items → console 子项清空', () => {
    const out = buildMenuFromConsoleSpec([{ path: '/console' }], { items: [] }, 'zh-CN');
    expect(out[0].children).toEqual([]);
  });

  it('非 console 节点：有 children 递归、无 children / 空 children 原样返回', () => {
    const base = menu();
    const out = buildMenuFromConsoleSpec(base, spec(), 'zh-CN');
    // /ops 递归（内部无 console 节点 → 结构不变）
    expect(out[1].children?.map((c) => c.key)).toEqual(['/ops/a']);
    // 叶子与空 children 保持原引用
    expect(out[2]).toBe(base[2]);
    expect(out[3]).toBe(base[3]);
  });

  it('consoleMenu 缺省 / items 缺省 → 动态子项为空', () => {
    const out = buildMenuFromConsoleSpec(
      [{ path: '/console', children: [{ path: '/console/home' }] }],
      undefined as unknown as ConsoleMenuSpec,
      'zh-CN',
    );
    expect(out[0].children).toEqual([{ path: '/console/home' }]);

    const noItems = buildMenuFromConsoleSpec(
      [{ path: '/console' }],
      {} as ConsoleMenuSpec,
      'zh-CN',
    );
    expect(noItems[0].children).toEqual([]);
  });

  it('深层嵌套中的 console 节点也会被替换', () => {
    const deep: RuntimeMenuItem[] = [
      {
        key: '/wrap',
        path: '/wrap',
        children: [{ key: '/console', path: '/console' }],
      },
    ];
    const out = buildMenuFromConsoleSpec(deep, spec(), 'zh-CN');
    expect(out[0].children![0].children?.map((c) => c.path)).toEqual([
      '/console/ops',
      '/console/bare',
    ]);
  });
});

describe('buildConsoleMenuFromAccessibleMenus', () => {
  const menu = (): RuntimeMenuItem[] => [
    {
      key: '/console',
      path: '/console',
      children: [{ key: '/console/home', path: '/console/home', name: '首页' }],
    },
    { key: '/ops', path: '/ops', children: [{ key: '/ops/a', path: '/ops/a', name: 'A' }] },
  ];

  const spec = (): ConsoleMenuSpec => ({
    items: [
      {
        key: 'resource',
        path: '/console/resource',
        title: { 'zh-CN': '资源管理' },
        locale: false,
        icon: 'appstore',
        children: [
          {
            key: 'p1',
            path: '/console/resource/p1',
            title: { 'zh-CN': '玩家管理' },
            locale: false,
          },
        ],
      },
      {
        key: 'legacy',
        path: '/console/legacy',
        title: { 'zh-CN': '遗留分类' },
        locale: false,
        children: [
          {
            key: 'p2',
            path: '/console/legacy/p2',
            title: { 'zh-CN': '遗留页' },
            locale: false,
          },
        ],
      },
    ],
  });

  const menuNode = (
    id: number,
    menuKey: string,
    children: MenuItem[] = [],
    icon?: string,
  ): MenuItem => ({
    id,
    parentId: null,
    menuKey,
    labels: { 'zh-CN': `菜单${menuKey}` },
    icon,
    sortOrder: id,
    isVisible: true,
    children,
  });

  it('菜单树驱动 console 子树：页面挂同名菜单下，home 保留', () => {
    const menus = [menuNode(1, 'resource', [menuNode(2, 'player')], 'DatabaseOutlined')];
    const out = buildConsoleMenuFromAccessibleMenus(menu(), menus, spec(), 'zh-CN');
    const consoleItem = out[0];
    // home + 菜单驱动子树（legacy 分类无同名菜单 → 兜底分组仍在）
    expect(consoleItem.children?.map((c) => c.name)).toEqual(['首页', '菜单resource', '遗留分类']);
    const resource = consoleItem.children![1];
    expect(resource.icon).toBeTruthy();
    // 子菜单 + 页面（页面在子菜单之后）
    expect(resource.children?.map((c) => c.name)).toEqual(['菜单player', '玩家管理']);
    // 叶子子菜单无页面 → path 落分类空态路由
    expect(resource.children![0].path).toBe('/console/player');
    // 页面路径透传 consoleMenu 已构建好的 path
    expect(resource.children![1].path).toBe('/console/resource/p1');
  });

  it('叶子菜单（无子无页面）path 落到分类路由', () => {
    const menus = [menuNode(1, 'bare')];
    const out = buildConsoleMenuFromAccessibleMenus(menu(), menus, spec(), 'zh-CN');
    const bare = out[0].children![1];
    expect(bare.name).toBe('菜单bare');
    expect(bare.path).toBe('/console/bare');
    expect(bare.children).toBeUndefined();
  });

  it('分类页面无同名菜单 → 兜底分组不丢弃', () => {
    const menus = [menuNode(1, 'resource')];
    const out = buildConsoleMenuFromAccessibleMenus(menu(), menus, spec(), 'zh-CN');
    const names = out[0].children!.map((c) => c.name);
    expect(names).toEqual(['首页', '菜单resource', '遗留分类']);
    const legacy = out[0].children![2];
    expect(legacy.children?.[0].path).toBe('/console/legacy/p2');
  });

  it('可访问菜单为空 → 退化为分类兜底分组（保持旧可见性，不静默丢页面）', () => {
    const out = buildConsoleMenuFromAccessibleMenus(menu(), [], spec(), 'zh-CN');
    expect(out[0].children?.map((c) => c.name)).toEqual(['首页', '资源管理', '遗留分类']);
  });

  it('深层嵌套的 console 节点同样被替换', () => {
    const deep: RuntimeMenuItem[] = [
      { key: '/wrap', path: '/wrap', children: [{ key: '/console', path: '/console' }] },
    ];
    const out = buildConsoleMenuFromAccessibleMenus(
      deep,
      [menuNode(1, 'resource')],
      spec(),
      'zh-CN',
    );
    expect(out[0].children![0].children?.map((c) => c.name)).toEqual(['菜单resource', '遗留分类']);
  });
});
