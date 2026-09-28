/**
 * 本 worktree 功能词条字典回归（zh/en 契约对）：
 * - #7/#8/#9 菜单结构扁平化/个人中心合并（8a858e0，menu.ts）
 * - #19/#20 用户管理与密码策略（a43365f，permissionsUsers.ts 的
 *   form.mustChangePassword / form.passwordExpires.*；pages.ts 的
 *   pages.login.mustChange.* 强改密弹窗词条）
 *
 * 三本词典都是平铺 Record<string, string>，最终 spread 进 zh-CN.ts / en-US.ts
 * 聚合注册表。漏配、空值、命名空间漂移 tsc 全都抓不住（formatMessage 只要求
 * string id），线上会直接渲染 key 名或空白——与 directoryHints.test.ts 同型的
 * 存在性回归，锚点用例同时防特性词条被误删回退。
 *
 * 覆盖口径：这 6 个文件此前无任何测试 import（0%），本套件 import 即全覆盖。
 *
 * 范围（假设注明）：zh-CN/en-US 是 LocalizedText 契约的唯一强制对
 * （CLAUDE.md「Localized Text Contract」）。其余 6 个 locale 按仓库既有惯例
 * 只维护子集（menu 55 键 / pages 65 键，由各语种维护节奏决定），不在契约
 * 对内，不参与本断言。
 */
import zhMenu from '../zh-CN/menu';
import enMenu from '../en-US/menu';
import zhPages from '../zh-CN/pages';
import enPages from '../en-US/pages';
import zhUsers from '../zh-CN/permissionsUsers';
import enUsers from '../en-US/permissionsUsers';

type Dict = Record<string, string>;

const PAIRS: Array<{ name: string; zh: Dict; en: Dict; prefixes: string[] }> = [
  {
    name: '菜单词典 menu（#7/#8/#9）',
    zh: zhMenu as unknown as Dict,
    en: enMenu as unknown as Dict,
    prefixes: ['menu.'],
  },
  {
    name: '页面词典 pages（含 #20 强改密）',
    zh: zhPages as unknown as Dict,
    en: enPages as unknown as Dict,
    // pages.ts 历史上还承载 profile.* 词条（早于本 worktree），同属平铺注册表
    prefixes: ['pages.', 'profile.'],
  },
  {
    name: '用户管理词典 permissionsUsers（#19/#20）',
    zh: zhUsers as unknown as Dict,
    en: enUsers as unknown as Dict,
    prefixes: ['pages.permissionsUsers.'],
  },
];

const sortedKeys = (dict: Dict): string[] => Object.keys(dict).sort();

describe.each(PAIRS)('$name', ({ zh, en, prefixes }) => {
  it('zh-CN 与 en-US 键集一致（无单侧漏配/多余键）', () => {
    expect(sortedKeys(en)).toEqual(sortedKeys(zh));
  });

  it('全部词条为非空字符串（空值线上渲染空白）', () => {
    for (const [key, value] of [...Object.entries(zh), ...Object.entries(en)]) {
      expect(typeof value).toBe('string');
      expect(value.trim().length).toBeGreaterThan(0);
    }
  });

  it('键带命名空间前缀（聚合注册表内防跨词典撞键/自造短 key）', () => {
    for (const key of [...Object.keys(zh), ...Object.keys(en)]) {
      expect(prefixes.some((prefix) => key.startsWith(prefix))).toBe(true);
    }
  });
});

describe('本 worktree 特性词条锚点（防误删回退）', () => {
  // [词条, # 归属] —— 均由 8a858e0 / a43365f 引入（git pickaxe 可溯）
  const ANCHORS: Array<{ key: string; zh: Dict; en: Dict }> = [
    {
      key: 'menu.AccessControl.UserAccount',
      zh: zhMenu as unknown as Dict,
      en: enMenu as unknown as Dict,
    },
    {
      key: 'pages.login.mustChange.title',
      zh: zhPages as unknown as Dict,
      en: enPages as unknown as Dict,
    },
    {
      key: 'pages.login.mustChange.submit',
      zh: zhPages as unknown as Dict,
      en: enPages as unknown as Dict,
    },
    {
      key: 'pages.permissionsUsers.form.mustChangePassword',
      zh: zhUsers as unknown as Dict,
      en: enUsers as unknown as Dict,
    },
    {
      key: 'pages.permissionsUsers.form.passwordExpires.never',
      zh: zhUsers as unknown as Dict,
      en: enUsers as unknown as Dict,
    },
  ];

  it.each(ANCHORS)('$key 在 zh-CN 与 en-US 均存在且非空', ({ key, zh, en }) => {
    expect(typeof zh[key]).toBe('string');
    expect(zh[key].trim().length).toBeGreaterThan(0);
    expect(typeof en[key]).toBe('string');
    expect(en[key].trim().length).toBeGreaterThan(0);
  });
});

describe('跨词典唯一性', () => {
  it('三本词典键两两不相交（spread 聚合时不互相覆盖）', () => {
    const menuKeys = Object.keys(zhMenu as unknown as Dict);
    const pagesKeys = Object.keys(zhPages as unknown as Dict);
    const userKeys = Object.keys(zhUsers as unknown as Dict);

    expect(menuKeys.filter((key) => pagesKeys.includes(key))).toEqual([]);
    expect(menuKeys.filter((key) => userKeys.includes(key))).toEqual([]);
    expect(pagesKeys.filter((key) => userKeys.includes(key))).toEqual([]);
  });
});
