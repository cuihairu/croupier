/**
 * utils/json 纯函数矩阵：
 * jsonParse（成功/失败回退默认值）、jsonStringify（循环引用回退 '{}'）、
 * deepMerge（非 plainObject 直接替换/嵌套递归/数组整体覆盖）、isValidJSON、
 * cloneDeep（原始值/Date/Array/plainObject/其他引用类型原样返回）、
 * parseInputSchema（字符串解析/补 type=object/非对象回退 null）、
 * deriveSchemaDefaults（default > example > enum 首项 > 类型占位，嵌套 object 递归）。
 */
import {
  cloneDeep,
  deepMerge,
  deriveSchemaDefaults,
  isValidJSON,
  jsonParse,
  jsonStringify,
  parseInputSchema,
} from '../json';

describe('jsonParse', () => {
  it('解析合法 JSON', () => {
    expect(jsonParse<{ a: number }>('{"a":1}')).toEqual({ a: 1 });
  });

  it('解析失败回退默认值并打日志', () => {
    const errorSpy = jest.spyOn(console, 'error').mockImplementation(() => {});
    const fallback = { a: -1 };
    expect(jsonParse('not-json', fallback)).toBe(fallback);
    expect(errorSpy).toHaveBeenCalled();
    errorSpy.mockRestore();
  });

  it('解析失败且未给默认值时返回 undefined', () => {
    const errorSpy = jest.spyOn(console, 'error').mockImplementation(() => {});
    expect(jsonParse('{')).toBeUndefined();
    errorSpy.mockRestore();
  });
});

describe('jsonStringify', () => {
  it('带缩进序列化', () => {
    expect(jsonStringify({ a: 1 }, 2)).toBe('{\n  "a": 1\n}');
  });

  it('循环引用序列化失败回退 "{}"', () => {
    const errorSpy = jest.spyOn(console, 'error').mockImplementation(() => {});
    const cyclic: Record<string, unknown> = {};
    cyclic.self = cyclic;
    expect(jsonStringify(cyclic)).toBe('{}');
    expect(errorSpy).toHaveBeenCalled();
    errorSpy.mockRestore();
  });
});

describe('deepMerge', () => {
  it('source 或 target 非 plainObject 时 source 整体生效', () => {
    expect(deepMerge({ a: 1 }, null as unknown as Partial<{ a: number }>)).toBeNull();
    const target = Object.create(null) as Record<string, unknown> & { a: number };
    expect(deepMerge(target, { a: 2 })).toEqual({ a: 2 });
  });

  it('浅层覆盖与嵌套递归合并', () => {
    expect(
      deepMerge({ a: 1, nested: { x: 1, y: 1 }, keep: 't' }, { a: 9, nested: { y: 2 } }),
    ).toEqual({ a: 9, nested: { x: 1, y: 2 }, keep: 't' });
  });

  it('source 嵌套值非 plainObject 或 target 无该键时直接覆盖（数组整体替换）', () => {
    expect(deepMerge({ list: [1, 2, 3], gone: true }, { list: [9] })).toEqual({
      list: [9],
      gone: true,
    });
    expect(deepMerge({} as Record<string, never>, { fresh: { deep: 1 } })).toEqual({
      fresh: { deep: 1 },
    });
  });
});

describe('isValidJSON', () => {
  it('合法/非法判定', () => {
    expect(isValidJSON('{"a":1}')).toBe(true);
    expect(isValidJSON('[1,2]')).toBe(true);
    expect(isValidJSON('oops')).toBe(false);
  });
});

describe('cloneDeep', () => {
  it('原始值与 null 原样返回', () => {
    expect(cloneDeep(42)).toBe(42);
    expect(cloneDeep('s')).toBe('s');
    expect(cloneDeep(null)).toBeNull();
  });

  it('Date 克隆为等值新实例', () => {
    const d = new Date('2026-01-01T00:00:00Z');
    const copy = cloneDeep(d);
    expect(copy).not.toBe(d);
    expect(copy.getTime()).toBe(d.getTime());
  });

  it('数组与嵌套对象深拷贝（修改克隆不影响原对象）', () => {
    const origin = { list: [{ id: 1 }], meta: { nested: { deep: true } } };
    const copy = cloneDeep(origin);
    expect(copy).toEqual(origin);
    expect(copy).not.toBe(origin);
    copy.list[0].id = 99;
    copy.meta.nested.deep = false;
    expect(origin.list[0].id).toBe(1);
    expect(origin.meta.nested.deep).toBe(true);
  });

  it('非 plainObject 引用类型（Map 等）原样返回', () => {
    const m = new Map([['k', 'v']]);
    expect(cloneDeep(m)).toBe(m);
  });
});

describe('parseInputSchema', () => {
  it('空/非字符串输入回退 null', () => {
    expect(parseInputSchema(undefined)).toBeNull();
    expect(parseInputSchema('')).toBeNull();
    expect(parseInputSchema('   ')).toBeNull();
  });

  it('解析成功且补默认 type=object', () => {
    const schema = parseInputSchema('{"properties":{"id":{"type":"string"}}}');
    expect(schema?.type).toBe('object');
    expect(schema?.properties?.id?.type).toBe('string');
  });

  it('已有 type 时不覆盖', () => {
    expect(parseInputSchema('{"type":"array"}')?.type).toBe('array');
  });

  it('解析结果非对象（数字/null 字面量）回退 null；数组因 typeof object 被接受（现状）', () => {
    expect(parseInputSchema('42')).toBeNull();
    expect(parseInputSchema('null')).toBeNull();
    // 数组也是 typeof 'object'，现行为接受、数组上补挂 type='object'——记录现状
    const arr = parseInputSchema('[1,2]') as unknown as { type?: string; length?: number };
    expect(Array.isArray(arr)).toBe(true);
    expect(arr.type).toBe('object');
  });

  it('非法 JSON 回退 null', () => {
    const errorSpy = jest.spyOn(console, 'error').mockImplementation(() => {});
    expect(parseInputSchema('{bad')).toBeNull();
    errorSpy.mockRestore();
  });
});

describe('deriveSchemaDefaults', () => {
  it('schema 缺失或非 object 类型返回空对象', () => {
    expect(deriveSchemaDefaults(undefined)).toEqual({});
    expect(deriveSchemaDefaults(null)).toEqual({});
    expect(deriveSchemaDefaults({ type: 'string' })).toEqual({});
    expect(deriveSchemaDefaults({ type: 'object' })).toEqual({});
  });

  it('优先级 default > example > enum 首项', () => {
    expect(
      deriveSchemaDefaults({
        type: 'object',
        properties: {
          a: { type: 'string', default: 'D', example: 'E', enum: ['v1', 'v2'] },
          b: { type: 'string', example: 'E', enum: ['v1', 'v2'] },
          c: { type: 'string', enum: ['v1', 'v2'] },
        },
      }),
    ).toEqual({ a: 'D', b: 'E', c: 'v1' });
  });

  it('default/example 为 null 时落到下一优先级', () => {
    expect(
      deriveSchemaDefaults({
        type: 'object',
        properties: {
          a: { type: 'integer', default: null, example: 7 },
          b: { type: 'string', example: null },
        },
      }),
    ).toEqual({ a: 7, b: '' });
  });

  it('按类型生成占位值：number 取 minimum、boolean/array/object 嵌套、未知类型跳过', () => {
    expect(
      deriveSchemaDefaults({
        type: 'object',
        properties: {
          n: { type: 'number', minimum: 5 },
          i: { type: 'integer' },
          b: { type: 'boolean' },
          arr: { type: 'array' },
          obj: { type: 'object', properties: { inner: { type: 'string' } } },
          unknown: { type: 'who-knows' },
        },
      }),
    ).toEqual({
      n: 5,
      i: 0,
      b: false,
      arr: [],
      obj: { inner: '' },
    });
  });
});
