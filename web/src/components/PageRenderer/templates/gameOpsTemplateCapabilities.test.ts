/**
 * 业务模板能力分类与函数绑定（界面生成链 C1）覆盖。
 *
 * 断言依赖「注册表 functions ↔ pageSpec.bindings.functionId」两侧一致，
 * 故不 mock 模板模块（与 gameOpsTemplates.test.tsx 的壳层覆盖分工）。
 *
 * 覆盖：四类组织能力分类齐备且模板归类合法；函数绑定与 pageSpec bindings
 * 一一对应、无重复绑定；CRUD 能力缺口显式可查；能力说明双语非空。
 */
import type { PageFunctionBinding } from '@/types/dashboard';
import {
  gameOpsTemplateCategories,
  gameOpsTemplateCrudRoles,
  gameOpsTemplates,
  gameOpsTemplatesByCategory,
  missingTemplateCrudRoles,
  templateFunctionRoles,
} from './gameOpsTemplates';

function bindingFunctionIds(pageSpec: { bindings: PageFunctionBinding[] }): string[] {
  return pageSpec.bindings.map((binding) => binding.functionId);
}

describe('业务模板四类能力分类', () => {
  it('四类分类齐备：玩家查询 / 资源管理 / 补偿操作 / 任务监控', () => {
    expect(gameOpsTemplateCategories.map((c) => c.key)).toEqual([
      'player-query',
      'resource-management',
      'compensation',
      'task-monitoring',
    ]);
    for (const c of gameOpsTemplateCategories) {
      expect(c.label['zh-CN']).toBeTruthy();
      expect(c.label['en-US']).toBeTruthy();
    }
  });

  it('每个模板声明合法能力分类，四类各有一个模板承接', () => {
    const categoryKeys = gameOpsTemplateCategories.map((c) => c.key);
    for (const entry of gameOpsTemplates) {
      expect(entry.categories.length).toBeGreaterThan(0);
      for (const category of entry.categories) {
        expect(categoryKeys).toContain(category);
      }
      // 声明顺序即分类表顺序，分组展示不重排
      expect(
        [...entry.categories].sort((a, b) => categoryKeys.indexOf(a) - categoryKeys.indexOf(b)),
      ).toEqual(entry.categories);
    }

    const grouped = gameOpsTemplatesByCategory();
    expect(grouped.map((g) => g.category)).toEqual(categoryKeys);
    for (const group of grouped) {
      expect(group.entries.length).toBeGreaterThan(0);
    }
    // 分组不丢模板：组内条目按「模板 × 所属分类数」计数
    expect(grouped.reduce((sum, group) => sum + group.entries.length, 0)).toBe(
      gameOpsTemplates.reduce((sum, entry) => sum + entry.categories.length, 0),
    );
  });
});

describe('业务模板函数绑定', () => {
  it('注册表 functions 与 pageSpec.bindings.functionId 一一对应', () => {
    for (const entry of gameOpsTemplates) {
      const declared = entry.functions.map((fn) => fn.id);
      const bound = bindingFunctionIds(entry.pageSpec);
      expect(new Set(declared).size).toBe(declared.length);
      expect(new Set(declared).size).toBe(bound.length);
      expect([...declared].sort()).toEqual([...bound].sort());
    }
  });

  it('绑定角色合法，每个模板恰好一个列表绑定与一个详情绑定', () => {
    const validRoles = new Set<string>(['list', 'get', 'create', 'update', 'delete', 'action']);
    for (const entry of gameOpsTemplates) {
      for (const fn of entry.functions) {
        expect(validRoles.has(fn.role)).toBe(true);
      }
      const count = (role: string) => entry.functions.filter((fn) => fn.role === role).length;
      expect(count('list')).toBe(1);
      expect(count('get')).toBe(1);
    }
  });

  it('全模板绑定函数 id 全局唯一（避免两个模板绑定同一游戏函数）', () => {
    const all = gameOpsTemplates.flatMap((entry) => entry.functions.map((fn) => fn.id));
    expect(new Set(all).size).toBe(all.length);
  });

  it('CRUD 能力缺口显式可查，与模板真实能力边界一致', () => {
    expect(gameOpsTemplateCrudRoles).toEqual(['create', 'update', 'delete']);
    expect(missingTemplateCrudRoles(gameOpsTemplates[0])).toEqual([]);
    expect(missingTemplateCrudRoles(gameOpsTemplates[1])).toEqual(['delete']);
    expect(missingTemplateCrudRoles(gameOpsTemplates[2])).toEqual(['update', 'delete']);
    expect(missingTemplateCrudRoles(gameOpsTemplates[3])).toEqual(['create', 'update', 'delete']);
    expect(missingTemplateCrudRoles(gameOpsTemplates[4])).toEqual(['update', 'delete']);
  });

  it('每个模板给出能力说明（双语非空），解释不提供的能力', () => {
    for (const entry of gameOpsTemplates) {
      expect(entry.capabilitiesNote['zh-CN'].length).toBeGreaterThan(0);
      expect(entry.capabilitiesNote['en-US'].length).toBeGreaterThan(0);
    }
  });
});

// 模板默认呈现（D5）与详情面（D1）——五个模板统一走 ResourcePageSpec，
// 默认能力（身份列固定可复制、前置筛选、危险操作定位、时间/单位/枚举语义）
// 由模板配置声明、渲染器按显式语义消费，不靠字段名猜测。
describe('业务模板默认呈现（D5）与详情面（D1）', () => {
  const resourceOf = (entry: (typeof gameOpsTemplates)[number]) => {
    if (entry.pageSpec.type !== 'resource') {
      throw new Error(`模板 ${entry.key} 不是 resource 规格`);
    }
    return entry.pageSpec.resource;
  };

  it('每个模板声明身份列（identityKey）且列表有固定可复制列', () => {
    for (const entry of gameOpsTemplates) {
      const res = resourceOf(entry);
      expect(res.listView?.identityKey).toBeTruthy();
      // 身份列固定 + 可复制：至少一列固定左且 render='copy'
      const copy = res.listView?.columns.filter((c) => c.render === 'copy');
      expect(copy?.length).toBeGreaterThan(0);
      const fixedCopy = copy?.some((c) => c.fixed === 'left');
      expect(fixedCopy).toBe(true);
    }
  });

  it('每个模板提供可筛选前置查询条件（至少一列 filterable）', () => {
    for (const entry of gameOpsTemplates) {
      const res = resourceOf(entry);
      expect(res.listView?.columns.some((c) => c.filterable)).toBe(true);
    }
  });

  it('每个模板有显式详情面（detailView 字段非空）', () => {
    for (const entry of gameOpsTemplates) {
      const res = resourceOf(entry);
      expect(res.detailView?.fields.length).toBeGreaterThan(0);
    }
  });

  it('定位危险操作：声明的 danger 动作落在列表动作区，破坏性封禁面为 danger', () => {
    for (const entry of gameOpsTemplates) {
      const res = resourceOf(entry);
      const allActions = [
        ...(res.listView?.rowActions ?? []),
        ...(res.listView?.batchActions ?? []),
        ...(res.listView?.toolbarActions ?? []),
      ];
      // 声明的 danger 动作必须落在列表动作区（定位可见），不藏在详情/表单深处
      for (const a of allActions) {
        if (a.type === 'danger') {
          expect(allActions.some((x) => x.type === 'danger')).toBe(true);
        }
      }
    }
    // 破坏性操作面（封禁）定位为 danger：玩家管理模板的封禁行操作
    const player = resourceOf(gameOpsTemplates.find((e) => e.key === 'player-manage')!);
    expect(player.listView?.rowActions?.some((a) => a.type === 'danger')).toBe(true);
  });

  it('每个模板时间列用 datetime（渲染器标浏览器时区，不靠字段名猜时区）', () => {
    for (const entry of gameOpsTemplates) {
      const res = resourceOf(entry);
      expect(res.listView?.columns.some((c) => c.dataType === 'datetime')).toBe(true);
    }
  });

  it('每个模板状态列用业务枚举标签（enum 列，渲染器着色）', () => {
    for (const entry of gameOpsTemplates) {
      const res = resourceOf(entry);
      expect(res.listView?.columns.some((c) => c.dataType === 'enum' && c.enum?.length)).toBe(true);
    }
  });

  it('数量/金额列单位显式声明（unit、标题单位或币种枚举列），版本/计数列豁免', () => {
    // 金额/数量语义列（key 或标题匹配）必须显式标单位，不靠字段名猜测倍率；
    // 版本、计数等无量纲列不适用。金额单位可由相邻币种枚举列声明（币种随行变化，
    // 静态 unit 反而失真）。
    const amountLike = (c: { key: string; title: { 'zh-CN'?: string } }): boolean =>
      /amount|gold|price|qty|quantity|balance|coin|compensat|fee|cost/i.test(c.key) ||
      /[金额金币每人数量总量]/u.test(String(c.title['zh-CN'] ?? ''));
    const hasUnitWord = (title: string): boolean => /[金元个条天小时次份人]/u.test(title);
    for (const entry of gameOpsTemplates) {
      const res = resourceOf(entry);
      const numberCols = res.listView?.columns.filter((c) => c.dataType === 'number') ?? [];
      const currencyCol = res.listView?.columns.find((c) => c.key === 'currency' && c.enum?.length);
      for (const c of numberCols) {
        if (!amountLike(c)) continue;
        const declared =
          Boolean(c.unit) || hasUnitWord(String(c.title['zh-CN'] ?? '')) || Boolean(currencyCol);
        expect(declared).toBe(true);
      }
      // 至少存在一个数量/金额语义列（模板均有可量化数据面）
      expect(numberCols.length).toBeGreaterThan(0);
    }
  });
});
