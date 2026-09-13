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

  it('x-options-source 提取为 remoteOptions（F9）', () => {
    const spec = derivePresentationSpec(
      schema({
        type: 'object',
        properties: {
          player: {
            type: 'string',
            'x-widget': 'Select',
            'x-options-source': {
              functionId: 'player.list',
              labelPath: '/items/*/name',
              valuePath: '/items/*/id',
            },
          },
          bad: { type: 'string', 'x-options-source': {} },
        },
      }),
    );
    const player = spec.fields!.find((f) => f.key === 'player')!;
    expect(player.remoteOptions).toEqual({
      functionId: 'player.list',
      labelPath: '/items/*/name',
      valuePath: '/items/*/id',
    });
    // 非法（缺 functionId）被忽略
    expect(spec.fields!.find((f) => f.key === 'bad')!.remoteOptions).toBeUndefined();
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

describe('derivePresentationSpec hints 边界（追加）', () => {
  it('x-widget-props 只保留已定义键；空对象不产生 hint', () => {
    const spec = derivePresentationSpec(
      schema({
        type: 'object',
        properties: {
          a: { type: 'string', 'x-widget-props': { size: 'large', clear: undefined, prefix: 'p' } },
        },
      }),
    );
    expect(spec.fields![0].widgetProps).toEqual({ size: 'large', prefix: 'p' });

    const noHint = derivePresentationSpec(
      schema({
        type: 'object',
        properties: { b: { type: 'string', 'x-widget-props': {} } },
      }),
    );
    expect(noHint.fields).toBeUndefined();
  });

  it('x-ui-layout 合法值生效并触发 fields；非法回退 vertical', () => {
    const horizontal = derivePresentationSpec(
      schema({
        type: 'object',
        'x-ui-layout': 'horizontal',
        properties: { a: { type: 'string' }, b: { type: 'integer' } },
      }),
    );
    expect(horizontal.layout).toBe('horizontal');
    expect(horizontal.fields!.map((f) => f.key)).toEqual(['a', 'b']);
    expect(horizontal.groups).toBeUndefined();

    const bad = derivePresentationSpec(
      schema({
        type: 'object',
        'x-ui-layout': 'diagonal',
        properties: { a: { type: 'string' } },
      }),
    );
    expect(bad.layout).toBe('vertical');
    expect(bad.fields).toBeUndefined();
  });

  it('x-visible-when：exists / notEquals / all / any 组合条件', () => {
    const spec = derivePresentationSpec(
      schema({
        type: 'object',
        properties: {
          a: { type: 'string', 'x-visible-when': { kind: 'exists', path: '/b' } },
          b: { type: 'string', 'x-visible-when': { kind: 'notEquals', path: '/a', value: 'x' } },
          c: {
            type: 'string',
            'x-visible-when': {
              kind: 'any',
              conditions: [
                { kind: 'equals', path: '/a', value: 1 },
                { kind: 'equals', path: 'bad', value: 2 },
              ],
            },
          },
          d: {
            type: 'string',
            'x-visible-when': {
              kind: 'all',
              conditions: [
                { kind: 'exists', path: '/a' },
                { kind: 'notEquals', path: '/b', value: false },
              ],
            },
          },
        },
      }),
    );
    expect(spec.fields!.find((f) => f.key === 'a')!.visibleWhen).toEqual({
      kind: 'exists',
      path: '/b',
    });
    expect(spec.fields!.find((f) => f.key === 'b')!.visibleWhen).toEqual({
      kind: 'notEquals',
      path: '/a',
      value: 'x',
    });
    // 无效子条件被过滤，仅保留合法分支
    expect(spec.fields!.find((f) => f.key === 'c')!.visibleWhen).toEqual({
      kind: 'any',
      conditions: [{ kind: 'equals', path: '/a', value: 1 }],
    });
    expect(spec.fields!.find((f) => f.key === 'd')!.visibleWhen).toEqual({
      kind: 'all',
      conditions: [
        { kind: 'exists', path: '/a' },
        { kind: 'notEquals', path: '/b', value: false },
      ],
    });
  });

  it('x-visible-when 非法形状全部忽略（未知 kind / 缺值 / 空组合 / 全无效 / 超深）', () => {
    const deep = (): Record<string, unknown> => {
      let node: Record<string, unknown> = { kind: 'exists', path: '/a' };
      for (let i = 0; i < 6; i += 1) node = { kind: 'all', conditions: [node] };
      return node;
    };
    const spec = derivePresentationSpec(
      schema({
        type: 'object',
        properties: {
          a: { type: 'string', 'x-visible-when': { kind: 'mystery', path: '/x', value: 1 } },
          b: { type: 'string', 'x-visible-when': { kind: 'notEquals', path: '/x' } },
          c: { type: 'string', 'x-visible-when': { kind: 'all', conditions: [] } },
          d: {
            type: 'string',
            'x-visible-when': {
              kind: 'any',
              conditions: [{ kind: 'equals', path: 'no-slash', value: 1 }],
            },
          },
          e: { type: 'string', 'x-visible-when': deep() },
          f: { type: 'string', 'x-visible-when': { kind: 'exists', path: 'nope' } },
          g: { type: 'string', 'x-visible-when': { kind: 42 } },
        },
      }),
    );
    expect(spec.fields).toBeUndefined();
  });

  it('x-enum-options：空数组 / 全部无效条目 → 不产生 hint', () => {
    const spec = derivePresentationSpec(
      schema({
        type: 'object',
        properties: {
          a: { type: 'string', 'x-enum-options': [] },
          b: {
            type: 'string',
            'x-enum-options': [{ value: 1 }, 'x', null, { value: 'v' }, { value: 'w', label: 42 }],
          },
        },
      }),
    );
    expect(spec.fields).toBeUndefined();
  });

  it('x-options-source：trim functionId、searchParam 透传；空串 labelPath 忽略', () => {
    const spec = derivePresentationSpec(
      schema({
        type: 'object',
        properties: {
          a: {
            type: 'string',
            'x-options-source': {
              functionId: '  f1  ',
              searchParam: 'kw',
              labelPath: '',
              valuePath: '/v',
            },
          },
        },
      }),
    );
    expect(spec.fields![0].remoteOptions).toEqual({
      functionId: 'f1',
      searchParam: 'kw',
      valuePath: '/v',
    });
  });

  it('x-options-source：functionId 空白 / 非对象 → 忽略', () => {
    const spec = derivePresentationSpec(
      schema({
        type: 'object',
        properties: {
          a: { type: 'string', 'x-options-source': { functionId: '   ' } },
          b: { type: 'string', 'x-options-source': 'nope' },
        },
      }),
    );
    expect(spec.fields).toBeUndefined();
  });

  it('数值 hints 边界：Infinity order / 越界与小数 width / 非布尔 disabled', () => {
    const spec = derivePresentationSpec(
      schema({
        type: 'object',
        properties: {
          drop: {
            type: 'string',
            'x-order': Infinity,
            'x-width': 0,
            'x-disabled': 'yes',
          },
          lo: { type: 'string', 'x-order': 1.5, 'x-width': 1 },
          hi: { type: 'string', 'x-width': 12, 'x-description': '说明文案' },
          frac: { type: 'string', 'x-width': 1.5 },
        },
      }),
    );
    // 存在任一 hint 时 fields 收录全部顶层字段（含无 hint 字段），按 order 升序
    expect(spec.fields!.map((f) => f.key)).toEqual(['lo', 'drop', 'hi', 'frac']);
    const drop = spec.fields!.find((f) => f.key === 'drop')!;
    expect(drop.order).toBeUndefined();
    expect(drop.width).toBeUndefined();
    expect(drop.disabled).toBeUndefined();
    expect(spec.fields!.find((f) => f.key === 'lo')!.width).toBe(1);
    expect(spec.fields!.find((f) => f.key === 'lo')!.order).toBe(1.5);
    const hi = spec.fields!.find((f) => f.key === 'hi')!;
    expect(hi.width).toBe(12);
    expect(hi.description).toEqual({ 'zh-CN': '说明文案', 'en-US': '说明文案' });
    expect(spec.fields!.find((f) => f.key === 'frac')!.width).toBeUndefined();
  });

  it('文本 hints 非法类型（数字 / 布尔 / 数组）被忽略', () => {
    const spec = derivePresentationSpec(
      schema({
        type: 'object',
        properties: {
          a: { type: 'string', 'x-label': 42, 'x-description': true, 'x-placeholder': [] },
        },
      }),
    );
    expect(spec.fields).toBeUndefined();
  });

  it('properties 非对象安全回退；标量属性值跳过', () => {
    const badProps = derivePresentationSpec(schema({ properties: 'nope' }));
    expect(badProps.jsonSchema).toEqual({ properties: 'nope' });
    expect(badProps.fields).toBeUndefined();

    const spec = derivePresentationSpec(
      schema({
        type: 'object',
        'x-ui-layout': 'inline',
        properties: { a: 'scalar', b: 5, c: { type: 'string' } },
      }),
    );
    expect(spec.fields!.map((f) => f.key)).toEqual(['c']);
  });

  it('x-ui-groups：非法条目跳过；collapsible/collapsed/title 透传', () => {
    const spec = derivePresentationSpec(
      schema({
        type: 'object',
        'x-ui-groups': [
          'not-object',
          42,
          { key: 7 },
          { key: '   ' },
          { key: 'g1', title: '标题', collapsible: true, collapsed: true },
          { key: 'g2', collapsed: 'yes' },
        ],
        properties: {
          a: { type: 'string', 'x-group': 'g1' },
          b: { type: 'string', 'x-widget': 'Input', 'x-group': 'g2' },
        },
      }),
    );
    expect(spec.groups).toHaveLength(2);
    const g1 = spec.groups!.find((g) => g.key === 'g1')!;
    expect(g1.title).toEqual({ 'zh-CN': '标题', 'en-US': '标题' });
    expect(g1.collapsible).toBe(true);
    expect(g1.collapsed).toBe(true);
    expect(g1.fields).toEqual(['a']);
    // collapsed 非布尔 → 忽略；已声明组未给 title → 不自动人性化
    const g2 = spec.groups!.find((g) => g.key === 'g2')!;
    expect(g2.collapsed).toBeUndefined();
    expect(g2.title).toBeUndefined();
    expect(g2.fields).toEqual(['b']);
  });

  it('x-group 空白字符串不产生 hint', () => {
    const spec = derivePresentationSpec(
      schema({
        type: 'object',
        properties: { a: { type: 'string', 'x-group': '   ' } },
      }),
    );
    expect(spec.fields).toBeUndefined();
  });
});
