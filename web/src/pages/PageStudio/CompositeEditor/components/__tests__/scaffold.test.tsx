import { resetRegistryForTest } from '../../registry';
import { registerBuiltinComponents, viewTypeToComponent } from '../builtin';
import type { FunctionDescriptor } from '@/services/api/functions';

const listFn: FunctionDescriptor = {
  id: 'inventory.list',
  operation: 'list',
  resource: 'inventory',
  inputSchema: {
    type: 'object',
    required: ['playerId'],
    properties: { playerId: { type: 'string' }, limit: { type: 'integer' } },
  },
  outputSchema: {
    type: 'object',
    properties: { id: { type: 'string' }, name: { type: 'string' }, quantity: { type: 'integer' } },
  },
};

const formFn: FunctionDescriptor = {
  id: 'player.ban',
  operation: 'update',
  resource: 'player',
  inputSchema: {
    type: 'object',
    required: ['playerId', 'reason'],
    properties: {
      playerId: { type: 'string' },
      reason: { type: 'string' },
      days: { type: 'integer' },
    },
  },
  outputSchema: { type: 'object', properties: { ok: { type: 'boolean' } } },
};

const getFn: FunctionDescriptor = {
  id: 'player.get',
  operation: 'get',
  resource: 'player',
  inputSchema: {
    type: 'object',
    required: ['playerId'],
    properties: { playerId: { type: 'string' } },
  },
  outputSchema: {
    type: 'object',
    properties: { uid: { type: 'string' }, level: { type: 'integer' }, vip: { type: 'integer' } },
  },
};

beforeAll(() => {
  resetRegistryForTest();
  registerBuiltinComponents();
});

describe('builtin components scaffold（契约→组件默认值，防回归快照）', () => {
  it('fnTable：列=输出 schema 全选，autoRun 开', () => {
    const props = viewTypeToComponent('table') && (undefined as never);
    void props;
    const def = resetAndGet('fnTable');
    expect(def.scaffold(listFn)).toEqual({
      functionId: 'inventory.list',
      title: 'inventory.list',
      span: 24,
      autoRun: true,
      columns: ['id', 'name', 'quantity'],
    });
  });

  it('fnForm：display 默认 inline', () => {
    const def = resetAndGet('fnForm');
    expect(def.scaffold(formFn)).toEqual({
      functionId: 'player.ban',
      title: 'player.ban',
      span: 24,
      display: 'inline',
    });
  });

  it('fnFields：默认半宽 autoRun', () => {
    const def = resetAndGet('fnFields');
    expect(def.scaffold(getFn)).toEqual({
      functionId: 'player.get',
      title: 'player.get',
      span: 12,
      autoRun: true,
    });
  });

  it('基础组件 scaffold', () => {
    expect(resetAndGet('button').scaffold()).toEqual({
      title: '按钮',
      btnStyle: 'default',
      span: 6,
    });
    expect(resetAndGet('modal').scaffold()).toEqual({ title: '操作', width: 'medium' });
    expect(resetAndGet('text').scaffold()).toEqual({ content: '说明文本', level: 'p', span: 24 });
  });

  it('modal 只接受 fnForm 子节点', () => {
    const def = resetAndGet('modal');
    expect(def.allowedChildren).toEqual(['fnForm']);
  });
});

function resetAndGet(type: string) {
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  const mods = require('../../registry');
  const def = mods.getComponent(type);
  if (!def) throw new Error(`missing ${type}`);
  return def;
}
