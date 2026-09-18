import type { ConsoleMenuSpec } from '@/types/dashboard';
import {
  CONSOLE_MENU_REFRESH_EVENT,
  buildConsolePagePath,
  buildMenuFromConsoleSpec,
  requestConsoleMenuRefresh,
  resolveConsolePageCanonicalPath,
  resolveLocalizedText,
  type RuntimeMenuItem,
} from './consoleMenu';

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

describe('resolveConsolePageCanonicalPath', () => {
  const menuTree = (): ConsoleMenuSpec =>
    ({
      items: [
        {
          key: 'player',
          path: '/console/player',
          title: {},
          children: [
            { key: 'resource--player', path: '/console/player/resource--player', title: {} },
          ],
        },
        {
          key: 'ops',
          path: '/console/ops',
          title: {},
          children: [
            {
              key: 'audit',
              path: '/console/audit',
              title: {},
              children: [{ key: 'deep.page', path: '/console/audit/deep.page', title: {} }],
            },
          ],
        },
      ],
    }) as unknown as ConsoleMenuSpec;

  it('menu 为空/pageKey 为空 → 返回空串（不重定向）', () => {
    expect(resolveConsolePageCanonicalPath(undefined, 'p')).toBe('');
    expect(resolveConsolePageCanonicalPath(null, 'p')).toBe('');
    expect(resolveConsolePageCanonicalPath({ items: [] }, 'p')).toBe('');
    expect(resolveConsolePageCanonicalPath(menuTree(), '')).toBe('');
  });

  it('页面挂在根级菜单 → 返回菜单树中的 path', () => {
    expect(resolveConsolePageCanonicalPath(menuTree(), 'resource--player')).toBe(
      '/console/player/resource--player',
    );
  });

  it('页面挂在深层子菜单 → 递归命中', () => {
    expect(resolveConsolePageCanonicalPath(menuTree(), 'deep.page')).toBe(
      '/console/audit/deep.page',
    );
  });

  it('页面未挂任何菜单（直达 URL）→ 空串，不做重定向', () => {
    expect(resolveConsolePageCanonicalPath(menuTree(), 'unmounted.page')).toBe('');
  });

  it('path 缺失时按父菜单 key 兜底构造（转义一致）', () => {
    const loose = {
      items: [
        {
          key: 'player ops',
          path: '/console/player%20ops',
          title: {},
          children: [{ key: 'p', title: {} }],
        },
      ],
    } as unknown as ConsoleMenuSpec;
    expect(resolveConsolePageCanonicalPath(loose, 'p')).toBe('/console/player%20ops/p');
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

  it('子项为子菜单（带 children）时递归展开任意层级', () => {
    // menu_items 驱动后 consoleMenu 子项可能是子菜单而非页面
    const nested = {
      items: [
        {
          key: 'root',
          path: '/console/root',
          title: { 'zh-CN': '运营' },
          locale: false,
          children: [
            {
              key: 'sub-audit',
              path: '/console/sub-audit',
              title: { 'zh-CN': '审计中心' },
              locale: false,
              children: [
                { key: 'page-x', path: '/console/sub-audit/page-x', title: {}, locale: false },
              ],
            },
            { key: 'page-y', path: '/console/root/page-y', title: {}, locale: false },
          ],
        },
      ],
    } as unknown as ConsoleMenuSpec;
    const out = buildMenuFromConsoleSpec([{ path: '/console' }], nested, 'zh-CN');
    const root = out[0].children![0];
    expect(root.children!.map((c) => c.path)).toEqual([
      '/console/sub-audit',
      '/console/root/page-y',
    ]);
    // 子菜单展开嵌套 children；叶子页面无 children 键
    expect(root.children![0].children?.[0].path).toBe('/console/sub-audit/page-x');
    expect(root.children![1]).not.toHaveProperty('children');
  });
});
