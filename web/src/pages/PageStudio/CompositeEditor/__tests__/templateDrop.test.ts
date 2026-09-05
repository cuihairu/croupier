import { planTemplateDrop } from '../templateDrop';
import type { PageNode } from './model';

const fnForm = (id: string): PageNode => ({ id, type: 'fnForm', props: {} });
const fnTable = (id: string): PageNode => ({ id, type: 'fnTable', props: {} });
const container = (id: string): PageNode => ({ id, type: 'container', props: {}, children: [] });

describe('planTemplateDrop', () => {
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

  it('落点是容器 → 装入 children', () => {
    const c = container('box');
    const plan = planTemplateDrop(nodes, 'box', null, c);
    expect(plan).toEqual({ kind: 'container', targetId: 'box' });
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
