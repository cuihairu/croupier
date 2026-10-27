/** D2 组合页表格行选择可配置：编辑器属性面 → 编译 → 发布 spec 透传。
 *
 * 覆盖路径：normalizeSelectionProp 纯函数（仅显式 multiple=true 才透传、
 * identityKey 空值丢弃并 trim）、compileTree（fnTable 启用落位
 * section.selection，关闭/缺失不下发，非表格组件忽略）、decompileToTree
 * （spec table.selection 回读为编辑态 props，未启用不还原关闭态对象）、
 * 回读→再编译的选择配置稳定性，以及 FnTable 属性面板的行选择配置面。
 * 组合页无资源语义上下文，生成器不推导行选择：漏配不会静默启用多选，
 * 缺省时发布态保持既有单选 radio（rowSelected 事件语义不变）。 */
import type { FunctionDescriptor } from '@/services/api/functions';
import { compileTree, decompileToTree, type SpecSectionLike } from '../compiler';
import { normalizeSelectionProp } from '../compiler/normalize';
import { nodeId, type PageNode } from '../model';
import { getComponent, resetRegistryForTest } from '../registry';
import { registerBuiltinComponents } from '../components/builtin';

function fn(type: PageNode['type'], fid: string, extra: Record<string, unknown> = {}): PageNode {
  return { id: nodeId(type), type, props: { functionId: fid, title: fid, span: 24, ...extra } };
}

describe('normalizeSelectionProp（行选择归一）', () => {
  it('仅显式 multiple=true 才透传：缺失/非对象/数组/未勾选均按未启用处理', () => {
    expect(normalizeSelectionProp(undefined)).toBeUndefined();
    expect(normalizeSelectionProp(null)).toBeUndefined();
    expect(normalizeSelectionProp('on')).toBeUndefined();
    expect(normalizeSelectionProp([])).toBeUndefined();
    expect(normalizeSelectionProp({ multiple: false })).toBeUndefined();
    expect(normalizeSelectionProp({ multiple: 'yes' })).toBeUndefined();
    expect(normalizeSelectionProp({ identityKey: 'uid' })).toBeUndefined();
  });

  it('启用：identityKey 仅接受非空字符串并 trim，空值与非法值丢弃', () => {
    expect(normalizeSelectionProp({ multiple: true })).toEqual({ multiple: true });
    expect(normalizeSelectionProp({ multiple: true, identityKey: 'uid' })).toEqual({
      multiple: true,
      identityKey: 'uid',
    });
    expect(normalizeSelectionProp({ multiple: true, identityKey: '  uid  ' })).toEqual({
      multiple: true,
      identityKey: 'uid',
    });
    expect(normalizeSelectionProp({ multiple: true, identityKey: '   ' })).toEqual({
      multiple: true,
    });
    expect(normalizeSelectionProp({ multiple: true, identityKey: 7 })).toEqual({
      multiple: true,
    });
    expect(normalizeSelectionProp({ multiple: true, identityKey: null })).toEqual({
      multiple: true,
    });
    expect(normalizeSelectionProp({ multiple: true, identityKey: undefined })).toEqual({
      multiple: true,
    });
  });
});

describe('compileTree（行选择透传）', () => {
  it('fnTable 显式启用多选 → section.selection 落位', () => {
    const { sections, warnings } = compileTree([
      fn('fnTable', 'player.list', {
        autoRun: true,
        selection: { multiple: true, identityKey: 'uid' },
      }),
    ]);
    expect(warnings).toEqual([]);
    expect(sections).toHaveLength(1);
    expect(sections[0].selection).toEqual({ multiple: true, identityKey: 'uid' });
  });

  it('未勾选或未配置 → 不下发行选择（发布态保持既有单选 radio）', () => {
    const off = compileTree([fn('fnTable', 'player.list', { selection: { multiple: false } })]);
    expect(off.sections[0].selection).toBeUndefined();
    const absent = compileTree([fn('fnTable', 'player.list')]);
    expect(absent.sections[0].selection).toBeUndefined();
  });

  it('非表格组件忽略行选择配置', () => {
    const { sections } = compileTree([
      fn('fnFields', 'player.get', { selection: { multiple: true, identityKey: 'uid' } }),
    ]);
    expect(sections).toHaveLength(1);
    expect(sections[0].selection).toBeUndefined();
  });
});

describe('decompileToTree（行选择回读与稳定性）', () => {
  const spec: SpecSectionLike[] = [
    {
      key: 'player.list',
      functionId: 'player.list',
      view: 'table',
      title: { 'zh-CN': '玩家' },
      span: 24,
      autoRun: true,
      table: {
        columns: [{ key: 'uid' }],
        selection: { multiple: true, identityKey: 'uid' },
      },
    },
    {
      key: 'player.plain',
      functionId: 'player.list',
      view: 'table',
      title: { 'zh-CN': '全部' },
      span: 24,
      autoRun: true,
    },
  ];

  it('spec table.selection → 编辑态 props；未启用不还原关闭态对象', () => {
    const [tree, warnings] = decompileToTree(spec);
    expect(warnings).toEqual([]);
    expect(tree).toHaveLength(2);
    expect(tree[0].props.selection).toEqual({ multiple: true, identityKey: 'uid' });
    expect(tree[1].props.selection).toBeUndefined();
  });

  it('回读→再编译行选择配置不变（保存不漂移）', () => {
    const [tree] = decompileToTree(spec);
    const { sections } = compileTree(tree);
    const byKey = new Map(sections.map((s) => [s.key, s]));
    expect(byKey.get('player.list')?.selection).toEqual({
      multiple: true,
      identityKey: 'uid',
    });
    expect(byKey.get('player.plain')?.selection).toBeUndefined();
  });
});

describe('FnTable 属性面板：行选择配置面', () => {
  beforeAll(() => {
    resetRegistryForTest();
    registerBuiltinComponents();
  });

  it('propSchema 暴露启用多选/行身份列两字段，多选默认关闭', () => {
    const def = getComponent('fnTable');
    expect(def).toBeDefined();
    const schema = def!.propSchema({
      nodes: [],
      fnById: new Map(),
      fn: undefined,
      allFns: [],
    });
    const selection = schema.properties?.selection as
      | {
          type?: string;
          additionalProperties?: boolean;
          properties?: Record<string, { type?: string; default?: unknown; description?: string }>;
        }
      | undefined;
    expect(selection?.type).toBe('object');
    expect(selection?.additionalProperties).toBe(false);
    expect(selection?.properties?.multiple).toMatchObject({ type: 'boolean', default: false });
    expect(selection?.properties?.identityKey).toMatchObject({ type: 'string' });
  });

  it('有输出 schema 时行身份列限定为输出字段（enum+enumNames）', () => {
    const listFn: FunctionDescriptor = {
      id: 'player.list',
      outputSchema: {
        type: 'object',
        properties: { uid: { type: 'string' }, nickname: { type: 'string' } },
      },
    };
    const def = getComponent('fnTable');
    const schema = def!.propSchema({
      nodes: [],
      fnById: new Map([['player.list', listFn]]),
      fn: listFn,
      allFns: [],
    });
    const identityKey = (schema.properties?.selection?.properties?.identityKey ?? {}) as {
      enum?: unknown[];
      enumNames?: unknown[];
      description?: string;
    };
    expect(identityKey.enum).toEqual(['uid', 'nickname']);
    expect(identityKey.enumNames).toEqual(['uid', 'nickname']);
    expect(identityKey.description).toBeUndefined();
  });

  it('无输出 schema 时行身份列改为说明文案（不伪装成可选值）', () => {
    const def = getComponent('fnTable');
    const schema = def!.propSchema({
      nodes: [],
      fnById: new Map(),
      fn: undefined,
      allFns: [],
    });
    const identityKey = (schema.properties?.selection?.properties?.identityKey ?? {}) as {
      enum?: unknown[];
      description?: string;
    };
    expect(identityKey.enum).toBeUndefined();
    expect(identityKey.description).toBeTruthy();
  });
});
