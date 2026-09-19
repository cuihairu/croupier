/** V5 T5.4：表达式变量空间构建测试。 */
import { buildExprVariables, buildPathRoots } from '../exprVariables';
import type { PageNode } from '../model';
import type { FunctionDescriptor } from '@/services/api/functions';
import type { JSONValue } from '@/types/dashboard';

const fn: FunctionDescriptor = {
  id: 'player.list',
  outputSchema: {
    type: 'object',
    properties: {
      items: {
        type: 'array',
        items: {
          type: 'object',
          properties: { uid: { type: 'string' }, nickname: { type: 'string' } },
        },
      },
      total: { type: 'number' },
    },
  } as JSONValue,
  inputSchema: {
    type: 'object',
    properties: { keyword: { type: 'string' } },
  } as JSONValue,
} as unknown as FunctionDescriptor;

const fnById = new Map([['player.list', fn]]);

const tableNode: PageNode = {
  id: 'n1',
  type: 'fnTable',
  props: { sectionKey: 'playerListTable', title: '玩家列表', functionId: 'player.list' },
};
const formNode: PageNode = {
  id: 'n2',
  type: 'fnForm',
  props: { sectionKey: 'mailSendForm', title: '发邮件', functionId: 'player.list' },
};
const staticNode: PageNode = {
  id: 'n3',
  type: 'staticForm',
  props: {
    sectionKey: 'filterForm',
    staticSchema: JSON.stringify({
      type: 'object',
      properties: { keyword: { type: 'string' } },
    }),
  },
};
const buttonNode: PageNode = {
  id: 'n4',
  type: 'button',
  props: { sectionKey: 'grantButton', title: '授权' },
};

describe('buildExprVariables', () => {
  it('收集树内全部已命名组件（含基础组件与弹窗子级）', () => {
    const tree: PageNode[] = [
      tableNode,
      buttonNode,
      { id: 'm1', type: 'modal', props: { sectionKey: 'sendMailModal' }, children: [formNode] },
    ];
    const vars = buildExprVariables(tree);
    expect(vars.map((v) => v.name)).toEqual([
      'playerListTable',
      'grantButton',
      'sendMailModal',
      'mailSendForm',
    ]);
    expect(vars[0].title).toBe('玩家列表');
    expect(vars[0].kind).toBe('fnTable');
  });

  it('未命名组件不进变量列表', () => {
    const vars = buildExprVariables([{ id: 'x', type: 'text', props: {} }]);
    expect(vars).toEqual([]);
  });
});

describe('buildPathRoots（§4.3 变量空间）', () => {
  it('fnTable 暴露 data/selectedRow/selectedRows；selectedRow 为 items 元素字段', () => {
    const roots = buildPathRoots(tableNode, fnById);
    expect(roots.map((r) => r.segment)).toEqual(['data', 'selectedRow', 'selectedRows']);
    const data = roots[0].children ?? [];
    expect(data.map((c) => c.segment)).toEqual(['items', 'total']);
    const selectedRow = roots[1].children ?? [];
    expect(selectedRow.map((c) => c.segment)).toEqual(['uid', 'nickname']);
  });

  it('fnForm 暴露 values（inputSchema）与 data（outputSchema）', () => {
    const roots = buildPathRoots(formNode, fnById);
    expect(roots.map((r) => r.segment)).toEqual(['values', 'data']);
    expect((roots[0].children ?? []).map((c) => c.segment)).toEqual(['keyword']);
  });

  it('staticForm 暴露 values（节点 staticSchema）', () => {
    const roots = buildPathRoots(staticNode, fnById);
    expect(roots.map((r) => r.segment)).toEqual(['values']);
    expect((roots[0].children ?? []).map((c) => c.segment)).toEqual(['keyword']);
  });

  it('基础组件无路径（仅可作动作目标）', () => {
    expect(buildPathRoots(buttonNode, fnById)).toEqual([]);
  });

  it('fnFields 暴露 data（outputSchema）', () => {
    const fieldsNode: PageNode = {
      id: 'n5',
      type: 'fnFields',
      props: { sectionKey: 'playerFields', functionId: 'player.list' },
    };
    const roots = buildPathRoots(fieldsNode, fnById);
    expect(roots.map((r) => r.segment)).toEqual(['data']);
    expect((roots[0].children ?? []).map((c) => c.segment)).toEqual(['items', 'total']);
  });

  it('嵌套 object schema 递归展开（type=object 字段下钻）', () => {
    const nestedFn: FunctionDescriptor = {
      id: 'stat.card',
      outputSchema: {
        type: 'object',
        properties: {
          summary: {
            type: 'object',
            properties: { dau: { type: 'number' }, revenue: { type: 'number' } },
          },
        },
      } as JSONValue,
    } as unknown as FunctionDescriptor;
    const node: PageNode = {
      id: 'n6',
      type: 'fnFields',
      props: { sectionKey: 'statCard', functionId: 'stat.card' },
    };
    const roots = buildPathRoots(node, new Map([['stat.card', nestedFn]]));
    const summary = (roots[0].children ?? []).find((c) => c.segment === 'summary');
    expect((summary?.children ?? []).map((c) => c.segment)).toEqual(['dau', 'revenue']);
  });

  it('staticSchema 为非法 JSON 字符串时回退空树', () => {
    const badNode: PageNode = {
      id: 'n7',
      type: 'staticForm',
      props: { sectionKey: 'badForm', staticSchema: '{oops' },
    };
    const roots = buildPathRoots(badNode, fnById);
    expect(roots).toEqual([{ segment: 'values', children: [] }]);
  });

  it('节点缺失返回空', () => {
    expect(buildPathRoots(undefined, fnById)).toEqual([]);
  });
});

describe('buildExprVariables title 兜底链', () => {
  it('无 title 有 content：title 兜底取 content 文本', () => {
    const contentOnly: PageNode = {
      id: 'n-content',
      type: 'button',
      props: { sectionKey: 'grantBtn', content: '一键授权说明' },
    };
    const vars = buildExprVariables([contentOnly]);
    expect(vars).toEqual([{ name: 'grantBtn', title: '一键授权说明', kind: 'button' }]);
  });

  it('title 非字符串且无 content：title 为 undefined', () => {
    const plain: PageNode = {
      id: 'n-plain',
      type: 'button',
      props: { sectionKey: 'plainBtn', title: 42 },
    };
    const vars = buildExprVariables([plain]);
    expect(vars).toEqual([{ name: 'plainBtn', kind: 'button' }]);
  });
});

describe('schemaTree / itemsElementTree 残缺形态矩阵', () => {
  const fieldsNode: PageNode = {
    id: 'nf',
    type: 'fnFields',
    props: { sectionKey: 'fields', functionId: 'demo.fn' },
  };
  const tableNode: PageNode = {
    id: 'nt',
    type: 'fnTable',
    props: { sectionKey: 'tbl', functionId: 'demo.fn' },
  };

  it('array 字段缺 items / items 非对象：字段无下钻子段', () => {
    const arrFn: FunctionDescriptor = {
      id: 'demo.fn',
      outputSchema: {
        type: 'object',
        properties: {
          tags: { type: 'array' },
          ids: { type: 'array', items: 'oops' },
          total: { type: 'number' },
        },
      } as JSONValue,
    } as unknown as FunctionDescriptor;
    const roots = buildPathRoots(fieldsNode, new Map([['demo.fn', arrFn]]));
    const data = roots[0].children ?? [];
    expect(data.map((c) => c.segment)).toEqual(['tags', 'ids', 'total']);
    // array 字段缺元素 schema → 不展开 children
    expect(data.every((c) => c.children === undefined)).toBe(true);
  });

  it('fnTable selectedRow：outputSchema 逐步残缺 → selectedRow 无候选字段', () => {
    // 1) 无 outputSchema：properties 缺省
    const noSchema: FunctionDescriptor = { id: 'demo.fn' } as unknown as FunctionDescriptor;
    // 2) properties 无 items 数组字段
    const noItems: FunctionDescriptor = {
      id: 'demo.fn',
      outputSchema: {
        type: 'object',
        properties: { total: { type: 'number' } },
      } as JSONValue,
    } as unknown as FunctionDescriptor;
    // 3) items 数组字段缺元素 schema
    const noElem: FunctionDescriptor = {
      id: 'demo.fn',
      outputSchema: {
        type: 'object',
        properties: { items: { type: 'array', items: { type: 'string' } } },
      } as JSONValue,
    } as unknown as FunctionDescriptor;
    for (const desc of [noSchema, noItems, noElem]) {
      const roots = buildPathRoots(tableNode, new Map([['demo.fn', desc]]));
      expect(roots.map((r) => r.segment)).toEqual(['data', 'selectedRow', 'selectedRows']);
      expect(roots[1].children ?? []).toEqual([]);
    }
  });
});

describe('staticFormTreeOf：staticSchema 对象形态（遗留编辑器数据）', () => {
  it('staticSchema 为对象（非 JSON 字符串）：直接按 schema 展开路径', () => {
    const objSchemaNode: PageNode = {
      id: 'n-obj-schema',
      type: 'staticForm',
      props: {
        sectionKey: 'objForm',
        staticSchema: { type: 'object', properties: { kw: { type: 'string' } } },
      },
    };
    const roots = buildPathRoots(objSchemaNode, fnById);
    expect(roots.map((r) => r.segment)).toEqual(['values']);
    expect((roots[0].children ?? []).map((c) => c.segment)).toEqual(['kw']);
  });
});
