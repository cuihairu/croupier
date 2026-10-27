/** D3 组合页表格服务端分页可配置：编辑器属性面 → 编译 → 发布 spec 透传。
 *
 * 覆盖路径：normalizePaginationProp 纯函数（仅显式启用才透传、每页条数钳制
 * 到 [1,200]、去重排序、空值丢弃）、compileTree（fnTable 启用落位
 * section.pagination，关闭/缺失不下发，非表格组件忽略）、decompileToTree
 *（spec table.pagination 回读为编辑态 props，未配置不还原关闭态对象）、
 * 回读→再编译的分页配置稳定性，以及 FnTable 属性面板的分页配置面。
 * 组合页无资源语义上下文，生成器不推导分页：漏配不会静默启用，误配也只
 * 在显式勾选启用时才生效。 */
import { compileTree, decompileToTree, type SpecSectionLike } from '../compiler';
import { normalizePaginationProp } from '../compiler/normalize';
import { nodeId, type PageNode } from '../model';
import { getComponent, resetRegistryForTest } from '../registry';
import { registerBuiltinComponents } from '../components/builtin';

function fn(type: PageNode['type'], fid: string, extra: Record<string, unknown> = {}): PageNode {
  return { id: nodeId(type), type, props: { functionId: fid, title: fid, span: 24, ...extra } };
}

describe('normalizePaginationProp（服务端分页归一）', () => {
  it('仅显式启用才透传：缺失/非对象/数组/未勾选均按关闭处理', () => {
    expect(normalizePaginationProp(undefined)).toBeUndefined();
    expect(normalizePaginationProp(null)).toBeUndefined();
    expect(normalizePaginationProp('on')).toBeUndefined();
    expect(normalizePaginationProp([])).toBeUndefined();
    expect(normalizePaginationProp({ enabled: false })).toBeUndefined();
    expect(normalizePaginationProp({ enabled: 'yes' })).toBeUndefined();
    expect(normalizePaginationProp({ defaultSize: 10 })).toBeUndefined();
  });

  it('启用：条数钳制到 [1,200]、去重升序、空值与非法值丢弃', () => {
    expect(normalizePaginationProp({ enabled: true })).toEqual({ enabled: true });
    expect(normalizePaginationProp({ enabled: true, defaultSize: 5 })).toEqual({
      enabled: true,
      defaultSize: 5,
    });
    expect(
      normalizePaginationProp({ enabled: true, defaultSize: 0, pageSizes: [20, 10, 10, 500] }),
    ).toEqual({
      enabled: true,
      pageSizes: [10, 20, 200],
    });
    expect(normalizePaginationProp({ enabled: true, pageSizes: [0, -5, 'abc', null] })).toEqual({
      enabled: true,
    });
    // 字符串入参（数字控件可能回字符串）按数字归一
    expect(
      normalizePaginationProp({ enabled: true, defaultSize: '15', pageSizes: ['20', 'abc', '10'] }),
    ).toEqual({
      enabled: true,
      defaultSize: 15,
      pageSizes: [10, 20],
    });
  });
});

describe('compileTree（服务端分页透传）', () => {
  it('fnTable 显式启用分页 → section.pagination 落位', () => {
    const { sections, warnings } = compileTree([
      fn('fnTable', 'player.list', {
        autoRun: true,
        pagination: { enabled: true, defaultSize: 10, pageSizes: [10, 20, 50] },
      }),
    ]);
    expect(warnings).toEqual([]);
    expect(sections).toHaveLength(1);
    expect(sections[0].pagination).toEqual({
      enabled: true,
      defaultSize: 10,
      pageSizes: [10, 20, 50],
    });
  });

  it('未勾选或未配置 → 不下发分页（发布态保持既有客户端分页）', () => {
    const off = compileTree([fn('fnTable', 'player.list', { pagination: { enabled: false } })]);
    expect(off.sections[0].pagination).toBeUndefined();
    const absent = compileTree([fn('fnTable', 'player.list')]);
    expect(absent.sections[0].pagination).toBeUndefined();
  });

  it('非表格组件忽略分页配置', () => {
    const { sections } = compileTree([
      fn('fnFields', 'player.get', { pagination: { enabled: true, defaultSize: 10 } }),
    ]);
    expect(sections).toHaveLength(1);
    expect(sections[0].pagination).toBeUndefined();
  });
});

describe('decompileToTree（服务端分页回读与稳定性）', () => {
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
        pagination: { enabled: true, defaultSize: 10, pageSizes: [10, 20, 50] },
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

  it('spec table.pagination → 编辑态 props；未配置不还原关闭态对象', () => {
    const [tree, warnings] = decompileToTree(spec);
    expect(warnings).toEqual([]);
    expect(tree).toHaveLength(2);
    expect(tree[0].props.pagination).toEqual({
      enabled: true,
      defaultSize: 10,
      pageSizes: [10, 20, 50],
    });
    expect(tree[1].props.pagination).toBeUndefined();
  });

  it('回读→再编译分页配置不变（保存不漂移）', () => {
    const [tree] = decompileToTree(spec);
    const { sections } = compileTree(tree);
    const byKey = new Map(sections.map((s) => [s.key, s]));
    expect(byKey.get('player.list')?.pagination).toEqual({
      enabled: true,
      defaultSize: 10,
      pageSizes: [10, 20, 50],
    });
    expect(byKey.get('player.plain')?.pagination).toBeUndefined();
  });
});

describe('FnTable 属性面板：分页配置面', () => {
  beforeAll(() => {
    resetRegistryForTest();
    registerBuiltinComponents();
  });

  it('propSchema 暴露启用/每页条数/可选条数三字段，默认关闭', () => {
    const def = getComponent('fnTable');
    expect(def).toBeDefined();
    const schema = def!.propSchema({
      nodes: [],
      fnById: new Map(),
      fn: undefined,
      allFns: [],
    });
    const pagination = schema.properties?.pagination as
      | {
          type?: string;
          additionalProperties?: boolean;
          properties?: Record<
            string,
            { type?: string; default?: unknown; minimum?: number; maximum?: number }
          >;
        }
      | undefined;
    expect(pagination?.type).toBe('object');
    expect(pagination?.additionalProperties).toBe(false);
    expect(pagination?.properties?.enabled).toMatchObject({ type: 'boolean', default: false });
    expect(pagination?.properties?.defaultSize).toMatchObject({
      type: 'integer',
      default: 10,
      minimum: 1,
      maximum: 200,
    });
    expect(pagination?.properties?.pageSizes).toMatchObject({
      type: 'array',
      default: [10, 20, 50],
    });
  });
});
