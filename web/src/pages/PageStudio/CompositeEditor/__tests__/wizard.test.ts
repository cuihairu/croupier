import { buildWizardTree, autoParams } from '../wizard';
import type { FunctionDescriptor } from '@/services/api/functions';
import { getComponent } from '../registry';
import { registerBuiltinComponents } from '../components/builtin';
import { parseAction } from '../actions';
import { compileTree } from '../compiler';

const tableFn: FunctionDescriptor = {
  id: 'player.list',
  operation: 'list',
  resource: 'player',
  inputSchema: { type: 'object', properties: { limit: { type: 'integer' } } },
  outputSchema: {
    type: 'object',
    properties: {
      items: { type: 'array' },
      playerId: { type: 'string' },
      level: { type: 'integer' },
    },
  },
};

const actionFn: FunctionDescriptor = {
  id: 'mail.send',
  operation: 'update',
  resource: 'mail',
  inputSchema: {
    type: 'object',
    required: ['playerId', 'title'],
    properties: {
      playerId: { type: 'string' },
      title: { type: 'string' },
      extra: { type: 'string' },
    },
  },
  outputSchema: { type: 'object', properties: { ok: { type: 'boolean' } } },
};

function scaffold(
  type: 'fnTable' | 'fnForm' | 'fnFields' | 'button' | 'modal' | 'container' | 'text',
  fn?: FunctionDescriptor,
) {
  return getComponent(type)!.scaffold(fn);
}

beforeAll(() => {
  registerBuiltinComponents();
});

describe('buildWizardTree（快速开始向导生成骨架）', () => {
  it('结构：表格 + 弹窗（含表单），表单成功刷新指向表格', () => {
    const { tree, tableId, modalId } = buildWizardTree(tableFn, actionFn, scaffold);
    expect(tree).toHaveLength(2);
    const table = tree[0];
    const modal = tree[1];
    expect(table.type).toBe('fnTable');
    expect(table.id).toBe(tableId);
    expect(table.props.autoRun).toBe(true);
    expect(modal.type).toBe('modal');
    expect(modal.id).toBe(modalId);
    const form = modal.children![0];
    expect(form.type).toBe('fnForm');
    expect(parseAction(form.props.onSuccessRefresh)).toEqual({
      kind: 'refreshNode',
      target: tableId,
    });
  });

  it('行操作：目标=弹窗，同名字段自动映射（playerId），无同名字段不映射', () => {
    const { tree, modalId } = buildWizardTree(tableFn, actionFn, scaffold);
    const ras = tree[0].props.rowActions as Array<Record<string, unknown>>;
    expect(ras).toHaveLength(1);
    expect(ras[0].targetSection).toBe(modalId);
    expect(ras[0].params).toEqual({ playerId: 'playerId' }); // extra/title 无同名行字段
    expect(autoParams(tableFn, actionFn)).toEqual({ playerId: 'playerId' });
    // 无交集时为空映射对象（仍可打开弹窗手填）
    const noOverlap: FunctionDescriptor = {
      ...actionFn,
      inputSchema: { type: 'object', properties: { uid: { type: 'string' } } },
    };
    expect(autoParams(tableFn, noOverlap)).toEqual({});
  });

  it('向导产物可直接编译发布（rowActions/dialog/onSuccessRefresh 全链）', () => {
    const { tree } = buildWizardTree(tableFn, actionFn, scaffold);
    const { sections, warnings } = compileTree(tree);
    expect(warnings).toEqual([]);
    expect(sections).toHaveLength(2);
    expect(sections[0]).toMatchObject({ functionId: 'player.list', view: 'table' });
    expect(sections[0].rowActions).toEqual([
      { label: 'mail.send', targetSection: 'mail.send', params: { playerId: 'playerId' } },
    ]);
    expect(sections[1]).toMatchObject({
      functionId: 'mail.send',
      view: 'form',
      display: 'dialog',
      onSuccessRefresh: ['player.list'],
    });
  });
});
