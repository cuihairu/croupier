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

describe('D5 模板默认呈现', () => {
  it('身份列固定可复制且作为常用查询条件前置', () => {
    for (const entry of gameOpsTemplates) {
      if (entry.pageSpec.type !== 'resource') continue;
      const list = entry.pageSpec.resource?.listView;
      expect(list).toBeDefined();
      const columns = list?.columns ?? [];
      // 首个身份列：固定左列 + 可复制，且是可查询的前置条件。
      expect(columns[0]).toMatchObject({ fixed: 'left', render: 'copy', filterable: true });
      // 常用（可查询）条件按列序前置，无高级条件插到身份条件之前。
      const filterableKeys = columns.filter((c) => c.filterable).map((c) => c.key);
      expect(filterableKeys[0]).toBe(columns[0].key);
      expect(filterableKeys.indexOf(columns[0].key)).toBe(0);
    }
  });

  it('危险操作位置明确：danger 行操作全部排在行操作末尾', () => {
    for (const entry of gameOpsTemplates) {
      if (entry.pageSpec.type !== 'resource') continue;
      const actions = entry.pageSpec.resource?.listView?.rowActions ?? [];
      const dangerKeys = actions.filter((a) => a.type === 'danger').map((a) => a.key);
      const nonDanger = actions.filter((a) => a.type !== 'danger');
      // danger 动作紧邻排最后，常态动作在前（不与 danger 交错）。
      expect(nonDanger.length + dangerKeys.length).toBe(actions.length);
      if (dangerKeys.length > 0) {
        expect(actions.slice(-dangerKeys.length).map((a) => a.key)).toEqual(dangerKeys);
      }
    }
  });

  // D5 收尾：模板默认呈现的量纲列语义表。每列显式声明单位（x-field-unit
  // 输出侧，列级 unit），不靠字段名猜测；金额列单位由独立 currency 列显式承载
  // （orderReview.amount→currency 枚举）。时间列由渲染端附浏览器本地时区标签，
  // 标题不硬编码 UTC。状态列走业务枚举标签（enum + render:tag）。
  const quantityUnits: Record<string, Record<string, { 'zh-CN': string; 'en-US': string }>> = {
    'player-manage': { balanceGold: { 'zh-CN': '金', 'en-US': 'gold' } },
    'ban-management': { durationHours: { 'zh-CN': '小时', 'en-US': 'hours' } },
    'mail-compensation': {
      targetCount: { 'zh-CN': '人', 'en-US': 'people' },
      perAmount: { 'zh-CN': '金', 'en-US': 'gold' },
      totalAmount: { 'zh-CN': '金', 'en-US': 'gold' },
      sentCount: { 'zh-CN': '人', 'en-US': 'people' },
      failedCount: { 'zh-CN': '人', 'en-US': 'people' },
    },
    'activity-config': { currentVersion: { 'zh-CN': '版', 'en-US': 'versions' } },
  };
  const resourceColumns = (entry: (typeof gameOpsTemplates)[number]) =>
    entry.pageSpec.resource?.listView?.columns ?? [];

  it('量纲列显式声明单位、时间列标题不硬编码时区、状态列走枚举标签', () => {
    for (const entry of gameOpsTemplates) {
      if (entry.pageSpec.type !== 'resource') continue;
      const columns = resourceColumns(entry);
      for (const [key, unit] of Object.entries(quantityUnits[entry.key] ?? {})) {
        const col = columns.find((c) => c.key === key);
        expect(col).toBeDefined();
        expect(col?.dataType).toBe('number');
        expect(col?.unit).toEqual(unit);
      }
      for (const col of columns) {
        if (col.dataType === 'datetime') {
          expect(col.title['zh-CN']).not.toMatch(/UTC/);
        }
        if (col.dataType === 'enum' && col.render === 'tag') {
          expect(col.enum?.length ?? 0).toBeGreaterThan(0);
        }
      }
    }
  });

  it('金额列单位由显式币种列承载（orderReview.amount→currency 枚举）', () => {
    const order = gameOpsTemplates.find((e) => e.key === 'order-review');
    const columns = resourceColumns(order!);
    expect(columns.some((c) => c.key === 'amount')).toBe(true);
    const currency = columns.find((c) => c.key === 'currency');
    expect(currency?.dataType).toBe('enum');
    expect(currency?.enum?.map((o) => o.value)).toEqual(['CNY', 'USD']);
  });

  it('保存回读与发布态默认表现一致：JSON 回读不丢失量纲/枚举语义', () => {
    // 模拟模板 PageSpec 落库保存→回读（JSON 序列化/反序列化），列表列语义
    // （unit/enum/render）必须原样保留；发布态渲染消费同一 spec 得一致呈现。
    for (const entry of gameOpsTemplates) {
      if (entry.pageSpec.type !== 'resource') continue;
      const saved = JSON.parse(JSON.stringify(entry.pageSpec)) as typeof entry.pageSpec;
      expect(saved.resource?.listView?.columns).toEqual(resourceColumns(entry));
      for (const col of saved.resource?.listView?.columns ?? []) {
        if (col.dataType === 'enum' && col.render === 'tag') {
          expect(col.enum?.length ?? 0).toBeGreaterThan(0);
        }
      }
    }
  });
});
