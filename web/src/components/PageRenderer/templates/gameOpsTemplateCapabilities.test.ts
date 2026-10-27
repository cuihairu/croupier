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
