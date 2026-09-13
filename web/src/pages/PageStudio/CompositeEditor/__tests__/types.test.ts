/** types（编辑器共享工具）覆盖：VIEW_META 四视图、schemaProperties/
 * schemaRequired 形态边界、sectionParams/sectionOutputFields、
 * linkageCheck（同名匹配/game_id·env 豁免/required 才计未匹配）、
 * defaultView 操作映射、scanParamCandidates（白名单类型/白名单 prop/
 * title 恒列/children 递归/nodeTitle 兜底）、derivePageKey。 */
import {
  VIEW_META,
  schemaProperties,
  schemaRequired,
  sectionParams,
  sectionOutputFields,
  linkageCheck,
  defaultView,
  scanParamCandidates,
  derivePageKey,
} from '../types';
import type { FunctionDescriptor } from '@/services/api/functions';
import type { PageNode } from '../model';

const fn = (op: string, inputSchema?: unknown, outputSchema?: unknown): FunctionDescriptor => ({
  id: 'player.query',
  operation: op,
  resource: 'player',
  ...(inputSchema !== undefined ? { inputSchema } : {}),
  ...(outputSchema !== undefined ? { outputSchema } : {}),
});

describe('VIEW_META', () => {
  it('四视图齐备', () => {
    expect(Object.keys(VIEW_META).sort()).toEqual(['actions', 'fields', 'form', 'table']);
    expect(VIEW_META.table.label).toBe('表格');
    expect(VIEW_META.fields.hint).toBe('键值详情，单对象展示');
  });
});

describe('schemaProperties / schemaRequired', () => {
  it('正常 schema：有序属性与必填集合', () => {
    const schema = {
      properties: { b: { type: 'string' }, a: { type: 'number' } },
      required: ['a', 1, null],
    };
    expect(schemaProperties(schema)).toEqual(['b', 'a']);
    expect([...schemaRequired(schema)]).toEqual(['a']);
  });

  it('形态边界：undefined/数组/非对象 properties/required 非数组', () => {
    expect(schemaProperties(undefined)).toEqual([]);
    expect(schemaProperties([1, 2])).toEqual([]);
    expect(schemaProperties({ properties: 'nope' })).toEqual([]);
    expect(schemaProperties({ properties: [] })).toEqual([]);
    expect([...schemaRequired({ required: 'nope' })]).toEqual([]);
    expect([...schemaRequired(undefined)]).toEqual([]);
  });
});

describe('sectionParams / sectionOutputFields', () => {
  it('fn undefined：空', () => {
    expect(sectionParams(undefined)).toEqual([]);
    expect(sectionOutputFields(undefined)).toEqual([]);
  });

  it('输入参数带 required 标记；输出字段透出', () => {
    const f = fn(
      'query',
      { properties: { playerId: {}, opt: {} }, required: ['playerId'] },
      { properties: { id: {}, name: {} } },
    );
    expect(sectionParams(f)).toEqual([
      { name: 'playerId', required: true },
      { name: 'opt', required: false },
    ]);
    expect(sectionOutputFields(f)).toEqual(['id', 'name']);
  });
});

describe('linkageCheck', () => {
  const params = [
    { name: 'playerId', required: true },
    { name: 'game_id', required: true },
    { name: 'env', required: true },
    { name: 'opt', required: false },
  ];

  it('同名匹配；未匹配只计 required（game_id/env 豁免）', () => {
    const [r] = linkageCheck(
      [{ key: 'src', title: '来源', outputs: ['playerId', 'extra'] }],
      params,
    );
    expect(r).toEqual({
      depKey: 'src',
      depTitle: '来源',
      matched: ['playerId'],
      unmatchedParams: [],
    });
  });

  it('无交集：required 计入未匹配，可选与豁免不计', () => {
    const [r] = linkageCheck([{ key: 'src', title: '来源', outputs: [] }], params);
    expect(r.matched).toEqual([]);
    expect(r.unmatchedParams).toEqual(['playerId']);
  });
});

describe('defaultView', () => {
  it('操作映射三分支', () => {
    expect(defaultView(fn('list'))).toBe('table');
    expect(defaultView(fn('query'))).toBe('table');
    expect(defaultView(fn('search'))).toBe('table');
    expect(defaultView(fn('get'))).toBe('fields');
    expect(defaultView(fn('update'))).toBe('form');
    expect(defaultView(undefined)).toBe('form');
  });
});

describe('scanParamCandidates', () => {
  it('白名单类型与 prop：title 恒列、span/autoRun 存在才列；children 递归', () => {
    const nodes: PageNode[] = [
      { id: 't1', type: 'text', props: { title: '文本' }, children: [] },
      {
        id: 'c1',
        type: 'container',
        props: {},
        children: [
          { id: 'tb1', type: 'fnTable', props: { title: '表格', autoRun: true } },
          { id: 'ff1', type: 'fnFields', props: { span: 12 } },
        ],
      },
    ];
    const out = scanParamCandidates(nodes);
    // modal/container/text 排除；fnTable 出 title+autoRun（无 span 不列）；
    // fnFields 出 title+span
    expect(out.map((c) => c.key)).toEqual(['tb1.title', 'tb1.autoRun', 'ff1.title', 'ff1.span']);
    expect(out.find((c) => c.key === 'ff1.title')).toMatchObject({
      nodeTitle: 'fnFields',
      propLabel: '标题',
      current: undefined,
    });
    expect(out.find((c) => c.key === 'tb1.autoRun')).toMatchObject({ propLabel: '自动执行' });
    expect(out.find((c) => c.key === 'ff1.span')).toMatchObject({
      propLabel: '栅格宽度',
      current: 12,
    });
  });

  it('无节点标题兜底 type；无 children 递归缺省', () => {
    const out = scanParamCandidates([{ id: 'x', type: 'fnForm', props: {} }]);
    expect(out.map((c) => c.nodeTitle)).toEqual(['fnForm']);
  });
});

describe('derivePageKey', () => {
  it('资源前缀去重合并；空/无前缀回退空串', () => {
    expect(derivePageKey(['player.query', 'player.ban', 'mail.send'])).toBe('player-mail');
    expect(derivePageKey(['.odd', 'x'])).toBe('x');
    expect(derivePageKey([])).toBe('');
  });
});
