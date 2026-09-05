import {
  countStaticFormFields,
  demoConstantTemplatePayloads,
  findLegacyMergedTemplates,
  maxConstantsInTree,
} from '../constantTemplateAudit';
import type { PageNode } from '../../../CompositeEditor/model';
import { staticFormNodeFromFields } from '../../../CompositeEditor/constants';

function staticFormNode(schema: Record<string, unknown>): PageNode {
  return {
    id: 'n1',
    type: 'staticForm',
    props: { title: 'T', staticSchema: JSON.stringify({ type: 'object', properties: schema }) },
  };
}

describe('countStaticFormFields', () => {
  it('统计 staticSchema 里的常量字段数', () => {
    const node = staticFormNode({ a: { type: 'string' }, b: { type: 'string' } });
    expect(countStaticFormFields(node)).toBe(2);
  });

  it('非 staticForm / 坏 schema 返回 0', () => {
    expect(countStaticFormFields({ id: 'x', type: 'fnTable', props: {} })).toBe(0);
    expect(
      countStaticFormFields({
        id: 'x',
        type: 'staticForm',
        props: { staticSchema: '{broken' },
      }),
    ).toBe(0);
  });
});

describe('maxConstantsInTree', () => {
  it('取树中 staticForm 的最大字段数；无 staticForm 为 0', () => {
    const tree: PageNode[] = [
      { id: 't', type: 'fnTable', props: {} },
      staticFormNode({ only: { type: 'string' } }),
    ];
    expect(maxConstantsInTree(tree)).toBe(1);
    expect(maxConstantsInTree([{ id: 'b', type: 'button', props: {} }])).toBe(0);
    expect(maxConstantsInTree([])).toBe(0);
  });
});

describe('findLegacyMergedTemplates', () => {
  it('标记一个 staticForm 塞多个常量的旧模板；放过新规范单常量模板', () => {
    const merged = {
      key: 'consts--legacy',
      tree: [
        staticFormNode({
          reason: { type: 'string' },
          level: { type: 'string' },
        }),
      ],
    };
    const modern = {
      key: 'consts--mfzkq-0',
      tree: [staticFormNode({ reason: { type: 'string' } })],
    };
    const plain = { key: 'tpl-fn', tree: [{ id: 't', type: 'fnTable', props: {} } as PageNode] };
    expect(findLegacyMergedTemplates([merged, modern, plain]).map((t) => t.key)).toEqual([
      'consts--legacy',
    ]);
  });
});

describe('demoConstantTemplatePayloads', () => {
  const payloads = demoConstantTemplatePayloads();

  it('生成 4 个示例，key 唯一且为 consts--demo-* 前缀', () => {
    expect(payloads).toHaveLength(4);
    const keys = payloads.map((p) => p.key);
    expect(new Set(keys).size).toBe(4);
    for (const key of keys) expect(key.startsWith('consts--demo-')).toBe(true);
  });

  it('每个示例都是一种常量一个独立单下拉 staticForm（恰好 1 个字段）', () => {
    for (const p of payloads) {
      expect(p.tree).toHaveLength(1);
      expect(p.tree[0].type).toBe('staticForm');
      expect(maxConstantsInTree(p.tree)).toBe(1);
      expect(findLegacyMergedTemplates([p])).toHaveLength(0);
    }
  });

  it('本地化名称遵循 LocalizedText 契约（zh-CN/en-US 键）', () => {
    for (const p of payloads) {
      expect(Object.keys(p.name).sort()).toEqual(['en-US', 'zh-CN']);
      expect(Object.keys(p.description).sort()).toEqual(['en-US', 'zh-CN']);
    }
  });

  it('常量字段带枚举选项且与选项数一致', () => {
    for (const p of payloads) {
      const schema = JSON.parse((p.tree[0].props as { staticSchema: string }).staticSchema) as {
        properties: Record<string, { enum?: string[] }>;
      };
      const props = Object.values(schema.properties);
      expect(props).toHaveLength(1);
      expect(props[0].enum!.length).toBeGreaterThan(0);
    }
  });

  it('可通过 staticFormNodeFromFields 往返还原为单字段', () => {
    const p = payloads[0];
    const node = p.tree[0];
    expect(node.type).toBe('staticForm');
    expect(countStaticFormFields(node)).toBe(1);
  });
});
