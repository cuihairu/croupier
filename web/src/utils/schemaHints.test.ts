import { derivePresentationSpec } from './schemaHints';
import { humanizeFieldKey } from './humanize';
import type { JSONSchema } from '@/types/dashboard';

const schema = (value: Record<string, unknown>): JSONSchema => value as unknown as JSONSchema;

describe('derivePresentationSpec', () => {
  it('无 hints 时等价历史行为：仅 jsonSchema + vertical 布局', () => {
    const input = schema({
      type: 'object',
      properties: { name: { type: 'string' } },
    });
    const spec = derivePresentationSpec(input);
    expect(spec.layout).toBe('vertical');
    expect(spec.fields).toBeUndefined();
    expect(spec.groups).toBeUndefined();
    expect(spec.jsonSchema).toBe(input);
  });

  it('提取 widget/label/placeholder，字符串 label 归一为 LocalizedText', () => {
    const spec = derivePresentationSpec(
      schema({
        type: 'object',
        properties: {
          playerId: {
            type: 'string',
            'x-widget': 'Select',
            'x-label': '玩家',
            'x-placeholder': { 'zh-CN': '选择玩家', 'en-US': 'Pick a player' },
          },
        },
      }),
    );
    expect(spec.fields).toHaveLength(1);
    const field = spec.fields![0];
    expect(field.widget).toBe('Select');
    expect(field.label).toEqual({ 'zh-CN': '玩家', 'en-US': '玩家' });
    expect(field.placeholder).toEqual({ 'zh-CN': '选择玩家', 'en-US': 'Pick a player' });
  });

  it('非法 widget/宽度/order 被忽略，不影响其余推导', () => {
    const spec = derivePresentationSpec(
      schema({
        type: 'object',
        properties: {
          a: { type: 'string', 'x-widget': 'NoSuchWidget', 'x-width': '6', 'x-order': 'x' },
          b: { type: 'string', 'x-widget': 'TextArea' },
        },
      }),
    );
    const a = spec.fields!.find((f) => f.key === 'a')!;
    expect(a.widget).toBeUndefined();
    expect(a.width).toBeUndefined();
    expect(a.order).toBeUndefined();
    const b = spec.fields!.find((f) => f.key === 'b')!;
    expect(b.widget).toBe('TextArea');
  });

  it('width 限 1-12 整数；disabled/order 合法时透传', () => {
    const spec = derivePresentationSpec(
      schema({
        type: 'object',
        properties: {
          a: { type: 'string', 'x-width': 13 },
          b: { type: 'integer', 'x-width': 6, 'x-order': 2, 'x-disabled': true },
        },
      }),
    );
    const a = spec.fields!.find((f) => f.key === 'a')!;
    expect(a.width).toBeUndefined();
    const b = spec.fields!.find((f) => f.key === 'b')!;
    expect(b.width).toBe(6);
    expect(b.order).toBe(2);
    expect(b.disabled).toBe(true);
  });

  it('x-visible-when：合法条件透传，非法被忽略', () => {
    const spec = derivePresentationSpec(
      schema({
        type: 'object',
        properties: {
          mode: { type: 'string', enum: ['single', 'batch'] },
          target: {
            type: 'string',
            'x-visible-when': { kind: 'equals', path: '/mode', value: 'single' },
          },
          bad: { type: 'string', 'x-visible-when': { kind: 'equals', path: 'mode', value: 'x' } },
        },
      }),
    );
    expect(spec.fields!.find((f) => f.key === 'target')!.visibleWhen).toEqual({
      kind: 'equals',
      path: '/mode',
      value: 'single',
    });
    expect(spec.fields!.find((f) => f.key === 'bad')!.visibleWhen).toBeUndefined();
  });

  it('x-enum-options 只补标签：非 string value 跳过，空结果不产生 hint', () => {
    const spec = derivePresentationSpec(
      schema({
        type: 'object',
        properties: {
          level: {
            type: 'integer',
            'x-enum-options': [
              { value: 1, label: '数值 value 被跳过' },
              { value: 'vip', label: { 'zh-CN': 'VIP' } },
              { value: 'svip' },
            ],
          },
        },
      }),
    );
    const field = spec.fields!.find((f) => f.key === 'level')!;
    expect(field.enumOptions).toEqual([{ value: 'vip', label: { 'zh-CN': 'VIP' } }]);
  });

  it('嵌套 object 保留为整体字段，不做点路径展开（待渲染器支持后启用）', () => {
    const spec = derivePresentationSpec(
      schema({
        type: 'object',
        properties: {
          address: {
            type: 'object',
            'x-label': '地址',
            properties: { city: { type: 'string', 'x-label': '城市' } },
          },
        },
      }),
    );
    expect(spec.fields!.map((f) => f.key)).toEqual(['address']);
    expect(spec.fields![0].label).toEqual({ 'zh-CN': '地址', 'en-US': '地址' });
  });

  it('分组：声明组收集成员，未声明 key 自动补组并人性化标题，空声明组被剪除', () => {
    const spec = derivePresentationSpec(
      schema({
        type: 'object',
        'x-ui-groups': [
          { key: 'basic', title: { 'zh-CN': '基本信息' } },
          { key: 'empty', title: { 'zh-CN': '空组' } },
        ],
        properties: {
          title: { type: 'string', 'x-group': 'basic' },
          level: { type: 'integer', 'x-group': 'undeclared' },
          solo: { type: 'string', 'x-widget': 'TextArea' },
        },
      }),
    );
    expect(spec.groups).toHaveLength(2);
    expect(spec.groups!.find((g) => g.key === 'basic')!.title).toEqual({ 'zh-CN': '基本信息' });
    expect(spec.groups!.find((g) => g.key === 'basic')!.fields).toEqual(['title']);
    const auto = spec.groups!.find((g) => g.key === 'undeclared')!;
    expect(auto.title).toEqual({ 'zh-CN': 'Undeclared', 'en-US': 'Undeclared' });
    expect(spec.groups!.find((g) => g.key === 'empty')).toBeUndefined();
  });

  it('x-order 升序稳定排序，未声明者保持定义顺序', () => {
    const spec = derivePresentationSpec(
      schema({
        type: 'object',
        properties: {
          first: { type: 'string', 'x-order': 1, 'x-label': 'f' },
          plain: { type: 'string' },
          second: { type: 'string', 'x-order': 2, 'x-label': 's' },
        },
      }),
    );
    expect(spec.fields!.map((f) => f.key)).toEqual(['first', 'second', 'plain']);
  });

  it('仅 x-group 的字段也进入 fields（分组需要成员可见）', () => {
    const spec = derivePresentationSpec(
      schema({
        type: 'object',
        'x-ui-groups': [{ key: 'g' }],
        properties: { a: { type: 'string', 'x-group': 'g' } },
      }),
    );
    expect(spec.fields!.map((f) => f.key)).toEqual(['a']);
    expect(spec.groups![0].fields).toEqual(['a']);
  });

  it('x-options-source 为保留字段，当前被忽略', () => {
    const spec = derivePresentationSpec(
      schema({
        type: 'object',
        properties: {
          player: {
            type: 'string',
            'x-options-source': { functionId: 'player.list' },
          },
        },
      }),
    );
    expect(spec.fields).toBeUndefined();
  });

  it('非对象/空 schema 安全回退', () => {
    expect(derivePresentationSpec(null)).toEqual({
      jsonSchema: {},
      layout: 'vertical',
    });
    expect(derivePresentationSpec(schema({ type: 'string' }))).toEqual({
      jsonSchema: { type: 'string' },
      layout: 'vertical',
    });
  });
});

describe('humanizeFieldKey', () => {
  it('camelCase/snake_case/点路径拆词并首字母大写', () => {
    expect(humanizeFieldKey('playerId')).toBe('Player Id');
    expect(humanizeFieldKey('batch_file')).toBe('Batch File');
    expect(humanizeFieldKey('mail.send')).toBe('Mail Send');
    expect(humanizeFieldKey('x')).toBe('X');
    expect(humanizeFieldKey('')).toBe('');
  });
});
