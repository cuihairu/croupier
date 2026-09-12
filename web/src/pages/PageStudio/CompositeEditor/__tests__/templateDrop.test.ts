import { planTemplateDrop } from '../templateDrop';
import { resetRegistryForTest } from '../registry';
import { registerBuiltinComponents } from '../components/builtin';
import type { PageNode } from './model';

const fnForm = (id: string): PageNode => ({ id, type: 'fnForm', props: {} });
const fnTable = (id: string): PageNode => ({ id, type: 'fnTable', props: {} });
const container = (id: string): PageNode => ({ id, type: 'container', props: {}, children: [] });
/** V2：页签容器（children=页 container，props.activeTab=编辑期激活页）。 */
const tabs = (id: string, pageIds: string[], activeTab?: string): PageNode => ({
  id,
  type: 'tabs',
  props: activeTab ? { activeTab } : {},
  children: pageIds.map((pid) => ({ id: pid, type: 'container', props: {}, children: [] })),
});

describe('planTemplateDrop', () => {
  // 容器落点契约校验依赖组件注册表（allowedChildren 声明）
  beforeAll(() => {
    resetRegistryForTest();
    registerBuiltinComponents();
  });

  const nodes = [fnForm('a'), fnTable('b'), fnForm('c')];

  it('空模板直接拒绝', () => {
    const plan = planTemplateDrop([], 'canvas-root', null);
    expect(plan.kind).toBe('blocked');
  });

  it('弹窗占位卡：仅 fnForm 模板可入', () => {
    const allForms = [fnForm('a'), fnForm('b')];
    const plan = planTemplateDrop(allForms, 'modal-drop:m1', null);
    expect(plan).toEqual({ kind: 'modal', targetId: 'm1' });
  });

  it('弹窗占位卡：混入非 fnForm 节点被拦截（V1 边界）', () => {
    const plan = planTemplateDrop(nodes, 'modal-drop:m1', null);
    expect(plan.kind).toBe('blocked');
    if (plan.kind === 'blocked') expect(plan.reason).toContain('V1');
  });

  it('编辑中弹窗：同弹窗占位卡规则', () => {
    const plan = planTemplateDrop(nodes, 'canvas-root', 'm2');
    expect(plan.kind).toBe('blocked');
  });

  it('落点是容器且子类型均合法 → 装入 children', () => {
    const c = container('box');
    const plan = planTemplateDrop(
      [fnTable('b'), { id: 'x', type: 'button', props: {} }],
      'box',
      null,
      c,
    );
    expect(plan).toEqual({ kind: 'container', targetId: 'box' });
  });

  it('落点是容器但含不允许的子类型 → 拦截（allowedChildren 契约）', () => {
    const c = container('box');
    const plan = planTemplateDrop(nodes, 'box', null, c); // fnForm 不在容器 allowedChildren
    expect(plan.kind).toBe('blocked');
    if (plan.kind === 'blocked') expect(plan.reason).toContain('fnForm');
  });

  it('落点是节点 → 链式插入（afterId=节点）', () => {
    const t = fnTable('t1');
    const plan = planTemplateDrop(nodes, 't1', null, t);
    expect(plan).toEqual({ kind: 'after', afterId: 't1' });
  });

  it('根级（canvas-root）→ 顺序追加', () => {
    const plan = planTemplateDrop(nodes, 'canvas-root', null);
    expect(plan).toEqual({ kind: 'after', afterId: undefined });
  });
});

describe('planTemplateDrop V2：页签容器（tabs）落激活页', () => {
  beforeAll(() => {
    resetRegistryForTest();
    registerBuiltinComponents();
  });

  const tpl = [fnTable('t'), { id: 'x', type: 'button', props: {} }];

  it('合法子类型 → 装入激活页（activeTab 命中）', () => {
    const t = tabs('tb1', ['p1', 'p2'], 'p2');
    const plan = planTemplateDrop(tpl, 'tb1', null, t);
    expect(plan).toEqual({ kind: 'container', targetId: 'p2' });
  });

  it('activeTab 未命中/未设 → 首页兜底', () => {
    const t = tabs('tb2', ['p1', 'p2']);
    const plan = planTemplateDrop(tpl, 'tb2', null, t);
    expect(plan).toEqual({ kind: 'container', targetId: 'p1' });
  });

  it('含页签页不允许的子类型 → 拦截并点名类型（契约同容器）', () => {
    const t = tabs('tb3', ['p1']);
    const plan = planTemplateDrop([fnTable('t'), fnForm('f')], 'tb3', null, t);
    expect(plan.kind).toBe('blocked');
    if (plan.kind === 'blocked') expect(plan.reason).toContain('fnForm');
  });

  it('无页 → 拦截（异常形态兜底）', () => {
    const t = tabs('tb4', []);
    const plan = planTemplateDrop(tpl, 'tb4', null, t);
    expect(plan.kind).toBe('blocked');
  });
});
