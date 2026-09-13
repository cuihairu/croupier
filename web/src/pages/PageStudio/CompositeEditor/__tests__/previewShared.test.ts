/** previewShared（预览共享工具）覆盖：payloadOf 归一四形态（空/信封
 * result/data 优先/非对象回退自身）、itemsOf 数组与非数组、findIn 按递归
 * 查找（命中根/命中嵌套子树/未命中）。 */
import { payloadOf, itemsOf, findIn } from '../previewShared';
import type { PageNode } from '../model';

describe('payloadOf', () => {
  it('undefined/空值：空对象', () => {
    expect(payloadOf(undefined)).toEqual({});
    expect(payloadOf(null)).toEqual({});
  });

  it('信封形态：result 优先', () => {
    expect(payloadOf({ result: { id: 1 }, data: { id: 2 } })).toEqual({ id: 1 });
  });

  it('data 形态（无 result）', () => {
    expect(payloadOf({ data: { items: [1] } })).toEqual({ items: [1] } as never);
  });

  it('inner 非对象（原始值信封）：回退自身', () => {
    expect(payloadOf({ result: 'plain' })).toEqual({ result: 'plain' });
  });

  it('裸 payload：原样', () => {
    expect(payloadOf({ id: 'p1' })).toEqual({ id: 'p1' });
  });
});

describe('itemsOf', () => {
  it('items 数组：透出', () => {
    expect(itemsOf({ items: [{ id: 1 }, { id: 2 }] })).toEqual([{ id: 1 }, { id: 2 }] as never);
  });

  it('items 非数组 / 缺省：空表', () => {
    expect(itemsOf({ items: 'nope' })).toEqual([]);
    expect(itemsOf({})).toEqual([]);
  });
});

describe('findIn', () => {
  const tree: PageNode[] = [
    {
      id: 'a',
      type: 'container',
      props: {},
      children: [
        { id: 'a1', type: 'text', props: {} },
        {
          id: 'a2',
          type: 'modal',
          props: {},
          children: [{ id: 'deep', type: 'button', props: {} }],
        },
      ],
    },
    { id: 'b', type: 'text', props: {} },
  ];

  it('命中根级', () => {
    expect(findIn(tree, 'b')?.type).toBe('text');
  });

  it('命中嵌套子树（弹窗内）', () => {
    expect(findIn(tree, 'deep')?.type).toBe('button');
  });

  it('命中 children 内层节点', () => {
    expect(findIn(tree, 'a1')?.id).toBe('a1');
  });

  it('未命中：undefined', () => {
    expect(findIn(tree, 'zz')).toBeUndefined();
  });
});
