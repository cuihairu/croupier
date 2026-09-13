import { applyTransform, getByPath, type Transform } from './transform';
import type { JSONValue } from '@/types/dashboard';

/** 测试用宽松构造：指令节点是开放形状，走 double-cast 进 Transform */
const t = (value: Record<string, unknown>): Transform => value as unknown as Transform;

describe('applyTransform', () => {
  const root: JSONValue = {
    a: { b: { c: 7 } },
    nums: [1, 2, 3],
    items: [
      { id: 'x', n: 2, cents: 100, name: 'ax' },
      { id: 'y', n: 4, cents: 250, name: 'by' },
    ],
    scale: 10,
    flag: true,
    nil: null,
  };

  it('transform 缺省 / 无 expr 无 template → 原样返回 root', () => {
    expect(applyTransform(root, undefined)).toBe(root);
    expect(applyTransform(root, { lang: 'cel-lite' })).toBe(root);
    expect(applyTransform(root, { expr: '' })).toBe(root);
    expect(applyTransform(root, { expr: '   ' })).toBe(root);
  });

  it('expr 提取路径（命中 / 未命中）', () => {
    expect(applyTransform(root, { expr: '$.a.b.c' })).toBe(7);
    expect(applyTransform(root, { expr: '$.missing' })).toBeUndefined();
  });
});

describe('getByPath', () => {
  const obj: JSONValue = {
    a: { b: { c: 7 } },
    arr: [{ id: 0 }, { id: 2 }],
    'dash-key': 1,
    nil: null,
  };

  it('空表达式 → undefined；$ 与 $. 指向根', () => {
    expect(getByPath(obj, '')).toBeUndefined();
    expect(getByPath(obj, '$')).toBe(obj);
    expect(getByPath(obj, '$.')).toBe(obj);
  });

  it('$.a.b 与 a.b 等价（含首尾空白）', () => {
    expect(getByPath(obj, '$.a.b.c')).toBe(7);
    expect(getByPath(obj, 'a.b.c')).toBe(7);
    expect(getByPath(obj, '  $.a.b.c  ')).toBe(7);
  });

  it('中途中断：键缺失 / 标量再下钻 / null 再下钻', () => {
    expect(getByPath(obj, '$.missing')).toBeUndefined();
    expect(getByPath(obj, '$.a.b.c.d')).toBeUndefined();
    expect(getByPath(obj, '$.nil.deep')).toBeUndefined();
  });

  it('数组下标：命中 / 越界 / 在对象上取下标', () => {
    expect(getByPath(obj, '$.arr[1].id')).toBe(2);
    expect(getByPath(obj, '$.arr[5]')).toBeUndefined();
    expect(getByPath(obj, '$.a[0]')).toBeUndefined();
  });

  it('数组当对象下钻 → undefined（isObject 排除数组）', () => {
    expect(getByPath(obj, '$.arr.id')).toBeUndefined();
  });

  it('非 \w 键名走 else 分支直取；数组上取 dashed 键 → undefined', () => {
    expect(getByPath(obj, '$.dash-key')).toBe(1);
    expect(getByPath(obj, '$.dash-key.deeper')).toBeUndefined();
    // cur 是数组（typeof object 但被 isObject 排除）→ else 分支中断
    expect(getByPath(obj, '$.arr.x-y')).toBeUndefined();
  });
});

describe('applyTransform template：字面量与路径拼接', () => {
  const root: JSONValue = { a: { b: { c: 7 } }, scale: 10 };

  it('原始类型字面量原样返回', () => {
    expect(applyTransform(root, t({ template: 'plain' }))).toBe('plain');
    expect(applyTransform(root, t({ template: 3 }))).toBe(3);
    expect(applyTransform(root, t({ template: true }))).toBe(true);
    expect(applyTransform(root, t({ template: null }))).toBe(null);
  });

  it('对象模板：$. 取 ctx、结果为 undefined 的键被剔除', () => {
    expect(applyTransform(root, t({ template: { v: '$.a.b.c', gone: '$.nope', k: 1 } }))).toEqual({
      v: 7,
      k: 1,
    });
    // 显式 undefined 值同样走兜底 return（非 directive / 非对象）
    expect(applyTransform(root, t({ template: { u: undefined, k: 1 } }))).toEqual({ k: 1 });
  });

  it('数组模板：undefined 结果被过滤', () => {
    expect(applyTransform(root, t({ template: ['$.a.b.c', 'lit', '$.missing'] }))).toEqual([
      7,
      'lit',
    ]);
  });

  it('多键对象即使含指令名键也按普通对象处理（isDirectiveNode 要求单键）', () => {
    expect(applyTransform(root, t({ template: { sum: '$.scale', extra: 1 } }))).toEqual({
      sum: 10,
      extra: 1,
    });
  });

  it('单键但非指令名 → 普通对象', () => {
    expect(applyTransform(root, t({ template: { bogus: '$.scale' } }))).toEqual({ bogus: 10 });
  });
});

describe('applyTransform template：forEach / map / pluck', () => {
  const root: JSONValue = {
    items: [
      { id: 'x', n: 2, cents: 100, name: 'ax' },
      { id: 'y', n: 4, cents: 250, name: 'by' },
    ],
    scale: 10,
  };

  it('forEach：item ctx 用 $.、根引用用 $$.', () => {
    const out = applyTransform(
      root,
      t({
        template: {
          forEach: {
            path: '$.items',
            template: { id: '$.id', rootScale: '$$.scale' },
          },
        },
      }),
    );
    expect(out).toEqual([
      { id: 'x', rootScale: 10 },
      { id: 'y', rootScale: 10 },
    ]);
  });

  it('forEach：path 非数组 → []；template 结果 undefined 被过滤', () => {
    expect(
      applyTransform(root, t({ template: { forEach: { path: '$.scale', template: '$.x' } } })),
    ).toEqual([]);
    expect(
      applyTransform(
        root,
        t({ template: { forEach: { path: '$.items', template: '$.missing' } } }),
      ),
    ).toEqual([]);
  });

  it.each([
    ['eq', { eq: ['$.id', 'x'] }, ['x']],
    ['ne', { ne: ['$.id', 'x'] }, ['y']],
    ['gt', { gt: ['$.n', 2] }, ['y']],
    ['lt', { lt: ['$.n', 3] }, ['x']],
    ['contains', { contains: ['$.name', 'a'] }, ['x']],
    ['match（字符串模式）', { match: ['$.name', '^a'] }, ['x']],
    ['match（RegExp 实例）', { match: ['$.name', /^b/] }, ['y']],
  ] as const)('forEach where %s', (_label, where, expected) => {
    const out = applyTransform(
      root,
      t({
        template: {
          forEach: {
            path: '$.items',
            where: where as Record<string, unknown>,
            template: '$.id',
          },
        },
      }),
    );
    expect(out).toEqual(expected);
  });

  it('forEach where：参数为标量自动包装 / 根相对参数 $$.', () => {
    // 非数组参数 → [值]，与 undefined 比较恒不等 → []
    expect(
      applyTransform(
        root,
        t({
          template: {
            forEach: { path: '$.items', where: { eq: '$.id' }, template: '$.id' },
          },
        }),
      ),
    ).toEqual([]);
    // 根相对：n < $$.scale（全部成立）
    expect(
      applyTransform(
        root,
        t({
          template: {
            forEach: { path: '$.items', where: { lt: ['$.n', '$$.scale'] }, template: '$.id' },
          },
        }),
      ),
    ).toEqual(['x', 'y']);
  });

  it('forEach where：空对象 / 缺省 / 非对象 / 未知算子 → 无过滤', () => {
    for (const where of [{}, undefined, 'nope', { weird: [1, 2] }]) {
      const out = applyTransform(
        root,
        t({
          template: {
            forEach: {
              path: '$.items',
              where: where as Record<string, unknown> | undefined,
              template: '$.id',
            },
          },
        }),
      );
      expect(out).toEqual(['x', 'y']);
    }
  });

  it('forEach where：contains / match 参数缺失时按空串处理', () => {
    // contains 两侧路径均未命中 → '' includes '' → 全部保留
    expect(
      applyTransform(
        root,
        t({
          template: {
            forEach: {
              path: '$.items',
              where: { contains: ['$$.nope', '$.nope'] },
              template: '$.id',
            },
          },
        }),
      ),
    ).toEqual(['x', 'y']);
    // match 左参未命中 → '' 不匹配 'a'
    expect(
      applyTransform(
        root,
        t({
          template: {
            forEach: {
              path: '$.items',
              where: { match: ['$.nope', 'a'] },
              template: '$.id',
            },
          },
        }),
      ),
    ).toEqual([]);
    // match 右参缺失 → 空正则匹配任意串 → 全部保留
    expect(
      applyTransform(
        root,
        t({
          template: {
            forEach: { path: '$.items', where: { match: ['$.name'] }, template: '$.id' },
          },
        }),
      ),
    ).toEqual(['x', 'y']);
  });

  it('map 指令与 forEach 等价；非数组 → []', () => {
    expect(
      applyTransform(root, t({ template: { map: { path: '$.items', template: '$.id' } } })),
    ).toEqual(['x', 'y']);
    expect(
      applyTransform(root, t({ template: { map: { path: '$.nope', template: '$.id' } } })),
    ).toEqual([]);
  });

  it('pluck：值可为路径 / 字面量 / 数组 / 对象 / 嵌套指令；非数组 → []', () => {
    expect(
      applyTransform(root, t({ template: { pluck: { path: '$.items', value: '$.n' } } })),
    ).toEqual([2, 4]);
    expect(
      applyTransform(root, t({ template: { pluck: { path: '$.items', value: '$$.scale' } } })),
    ).toEqual([10, 10]);
    expect(
      applyTransform(root, t({ template: { pluck: { path: '$.items', value: 'literal' } } })),
    ).toEqual(['literal', 'literal']);
    expect(applyTransform(root, t({ template: { pluck: { path: '$.items', value: 9 } } }))).toEqual(
      [9, 9],
    );
    expect(
      applyTransform(root, t({ template: { pluck: { path: '$.items', value: [1, 2] } } })),
    ).toEqual([
      [1, 2],
      [1, 2],
    ]);
    expect(
      applyTransform(root, t({ template: { pluck: { path: '$.items', value: { k: 1 } } } })),
    ).toEqual([{ k: 1 }, { k: 1 }]);
    expect(
      applyTransform(
        root,
        t({ template: { pluck: { path: '$.items', value: { number: '$.n' } } } }),
      ),
    ).toEqual([2, 4]);
    expect(
      applyTransform(root, t({ template: { pluck: { path: '$.scale', value: '$.n' } } })),
    ).toEqual([]);
    // 显式 undefined 值 → resolveValue 兜底 undefined → 过滤
    expect(
      applyTransform(root, t({ template: { pluck: { path: '$.items', value: undefined } } })),
    ).toEqual([]);
  });
});

describe('applyTransform template：sum / avg', () => {
  const root: JSONValue = {
    nums: [1, 2, 3],
    items: [{ cents: 100 }, { cents: 250 }],
  };

  it('sum/avg 直接对数字数组', () => {
    expect(applyTransform(root, t({ template: { sum: { path: '$.nums' } } }))).toBe(6);
    expect(applyTransform(root, t({ template: { avg: { path: '$.nums' } } }))).toBe(2);
  });

  it('sum/avg 以 value 模板逐项取数', () => {
    expect(
      applyTransform(root, t({ template: { sum: { path: '$.items', value: '$.cents' } } })),
    ).toBe(350);
    expect(
      applyTransform(root, t({ template: { avg: { path: '$.items', value: '$.cents' } } })),
    ).toBe(175);
    expect(
      applyTransform(
        root,
        t({ template: { sum: { path: '$.items', value: { number: '$.cents' } } } }),
      ),
    ).toBe(350);
  });

  it('非数组 → 0；全 NaN 的 avg → 0', () => {
    expect(applyTransform(root, t({ template: { sum: { path: '$.nope' } } }))).toBe(0);
    expect(applyTransform(root, t({ template: { avg: { path: '$.items' } } }))).toBe(0);
  });
});

describe('applyTransform template：数值指令', () => {
  const root: JSONValue = { scale: 10, strNum: '42', bad: 'abc' };

  it('number：字符串数字 / 路径 / NaN → undefined / null → 0', () => {
    expect(applyTransform(root, t({ template: { number: '$.strNum' } }))).toBe(42);
    expect(applyTransform(root, t({ template: { number: '$.scale' } }))).toBe(10);
    expect(applyTransform(root, t({ template: { number: '$.bad' } }))).toBeUndefined();
    expect(applyTransform(root, t({ template: { number: null } }))).toBe(0);
  });

  it('toFixed：指定位数 / 缺省位数回退 0 / 值缺失 → undefined', () => {
    expect(applyTransform(root, t({ template: { toFixed: { value: 3.14159, digits: 2 } } }))).toBe(
      3.14,
    );
    expect(applyTransform(root, t({ template: { toFixed: { value: 3.14159 } } }))).toBe(3);
    expect(applyTransform(root, t({ template: { toFixed: { digits: 2 } } }))).toBeUndefined();
  });

  it('msFromSec / isoFromMs / isoFromSec：命中与缺失', () => {
    expect(applyTransform(root, t({ template: { msFromSec: 2 } }))).toBe(2000);
    expect(applyTransform(root, t({ template: { msFromSec: '$.nope' } }))).toBeUndefined();
    expect(applyTransform(root, t({ template: { isoFromMs: 0 } }))).toBe(
      '1970-01-01T00:00:00.000Z',
    );
    expect(applyTransform(root, t({ template: { isoFromMs: '$.nope' } }))).toBeUndefined();
    expect(applyTransform(root, t({ template: { isoFromSec: 1 } }))).toBe(
      '1970-01-01T00:00:01.000Z',
    );
    expect(applyTransform(root, t({ template: { isoFromSec: '$.nope' } }))).toBeUndefined();
  });

  it('mul / div：命中、by 非法、值缺失、除零', () => {
    expect(applyTransform(root, t({ template: { mul: { value: 3, by: 4 } } }))).toBe(12);
    expect(applyTransform(root, t({ template: { mul: { value: 3 } } }))).toBeUndefined();
    expect(applyTransform(root, t({ template: { mul: { by: 2 } } }))).toBeUndefined();
    expect(applyTransform(root, t({ template: { div: { value: 10, by: 4 } } }))).toBe(2.5);
    expect(applyTransform(root, t({ template: { div: { value: 10, by: 0 } } }))).toBeUndefined();
    expect(applyTransform(root, t({ template: { div: { value: 10 } } }))).toBeUndefined();
    expect(applyTransform(root, t({ template: { div: { by: 2 } } }))).toBeUndefined();
  });

  it('add / sub：命中与缺省补 0', () => {
    expect(applyTransform(root, t({ template: { add: { a: 1, b: 2 } } }))).toBe(3);
    expect(applyTransform(root, t({ template: { add: { a: '$.scale' } } }))).toBe(10);
    expect(applyTransform(root, t({ template: { add: {} } }))).toBe(0);
    expect(applyTransform(root, t({ template: { sub: { a: 5, b: 2 } } }))).toBe(3);
    expect(applyTransform(root, t({ template: { sub: {} } }))).toBe(0);
  });

  it('指令键存在但值为 undefined → 穿透所有分支返回 undefined', () => {
    expect(applyTransform(root, t({ template: { number: undefined } }))).toBeUndefined();
  });
});
