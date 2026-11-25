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
  deriveFormDefaults,
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
  it('array 按 items 派生一件示例元素（对象展开字段骨架、基元给占位值）', () => {
    expect(
      deriveSchemaDefaults({
        type: 'object',
        properties: {
          items: {
            type: 'array',
            items: {
              type: 'object',
              properties: { id: { type: 'string' }, n: { type: 'integer', minimum: 1 } },
            },
          },
          tags: { type: 'array', items: { type: 'string' } },
          free: { type: 'array' },
        },
      }),
    ).toEqual({ items: [{ id: '', n: 1 }], tags: [''], free: [] });
  });

  it('schema 缺失返回空对象；无 properties 的对象根返回空对象', () => {
    expect(deriveSchemaDefaults(undefined)).toEqual({});
    expect(deriveSchemaDefaults(null)).toEqual({});
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

  // SUI-017：根骨架跟随根类型——根数组与根标量不再套对象壳。
  describe('SUI-017 根数组与标量的种子', () => {
    it('根数组按 items 派生一件示例元素；无 items 得空数组', () => {
      expect(
        deriveSchemaDefaults({
          type: 'array',
          items: { type: 'object', properties: { id: { type: 'string' } } },
        }),
      ).toEqual([{ id: '' }]);
      expect(deriveSchemaDefaults({ type: 'array' })).toEqual([]);
      expect(deriveSchemaDefaults({ type: 'array', items: { type: 'string' } })).toEqual(['']);
    });

    it('根标量得到类型占位值：string/integer/number minimum/boolean', () => {
      expect(deriveSchemaDefaults({ type: 'string' })).toBe('');
      expect(deriveSchemaDefaults({ type: 'integer' })).toBe(0);
      expect(deriveSchemaDefaults({ type: 'number', minimum: 3 })).toBe(3);
      expect(deriveSchemaDefaults({ type: 'boolean' })).toBe(false);
    });

    it('可空联合根（SUI-009 type 数组）按主类型派生种子', () => {
      expect(deriveSchemaDefaults({ type: ['array', 'null'] })).toEqual([]);
      expect(deriveSchemaDefaults({ type: ['boolean', 'null'] })).toBe(false);
      expect(deriveSchemaDefaults({ type: ['string', 'null'], minLength: 2 })).toBe('');
    });

    it('根级 default > example > enum 首项，显式 null 按声明保留', () => {
      expect(deriveSchemaDefaults({ type: 'array', default: [] })).toEqual([]);
      expect(deriveSchemaDefaults({ type: 'boolean', default: null })).toBe(null);
      expect(deriveSchemaDefaults({ type: 'integer', default: 7, example: 9 })).toBe(7);
      expect(deriveSchemaDefaults({ type: 'string', example: 'E', enum: ['v1'] })).toBe('E');
      expect(deriveSchemaDefaults({ type: 'integer', enum: [2, 3] })).toBe(2);
    });

    it('对象根保持旧路径：根级 default 不参与派生', () => {
      expect(
        deriveSchemaDefaults({ type: 'object', default: { fromRoot: 1 }, properties: {} }),
      ).toEqual({});
    });
  });
});

// SUI-018 默认值层：表单/JSON 初值只含 schema 声明的 default；示例、枚举
// 首项与类型占位不入层（示例经「填入示例」显式填入，占位只作输入提示），
// 可选字段未填写即保持缺省。
describe('deriveFormDefaults（SUI-018 默认值层）', () => {
  it('只收声明 default：example、enum 首项与类型占位都不入层', () => {
    expect(
      deriveFormDefaults({
        type: 'object',
        properties: {
          a: { type: 'string', default: 'D', example: 'E', enum: ['v1'] },
          b: { type: 'string', example: 'E' },
          c: { type: 'string', enum: ['v1', 'v2'] },
          d: { type: 'integer' },
          e: { type: 'boolean' },
          f: { type: 'array', items: { type: 'string' } },
        },
      }),
    ).toEqual({ a: 'D' });
  });

  it('显式 null default 视为未声明（可选字段留缺省不提交 null）', () => {
    expect(
      deriveFormDefaults({
        type: 'object',
        properties: {
          a: { type: ['null', 'string'], default: null },
          b: { type: 'boolean', default: false },
          c: { type: 'integer', default: 0 },
        },
      }),
    ).toEqual({ b: false, c: 0 });
  });

  it('嵌套对象递归收集子字段声明默认；无任何声明默认的嵌套对象整体缺席', () => {
    expect(
      deriveFormDefaults({
        type: 'object',
        properties: {
          cfg: {
            type: 'object',
            properties: {
              level: { type: 'integer', default: 3 },
              note: { type: 'string' },
            },
          },
          empty: { type: 'object', properties: { x: { type: 'string' } } },
        },
      }),
    ).toEqual({ cfg: { level: 3 } });
  });

  it('对象根忽略根级 default/example（旧对象表单路径不变）', () => {
    expect(
      deriveFormDefaults({ type: 'object', default: { fromRoot: 1 }, properties: {} }),
    ).toEqual({});
  });

  it('根数组/标量无 default 时给类型骨架（JSON 模式可发送的初值）', () => {
    expect(deriveFormDefaults({ type: 'array' })).toEqual([]);
    expect(deriveFormDefaults({ type: 'string' })).toBe('');
    expect(deriveFormDefaults({ type: 'integer' })).toBe(0);
    expect(deriveFormDefaults({ type: 'number', minimum: 2 })).toBe(2);
    expect(deriveFormDefaults({ type: 'boolean' })).toBe(false);
    expect(deriveFormDefaults(undefined)).toEqual({});
    expect(deriveFormDefaults(null)).toEqual({});
  });

  it('声明 default 时根数组/标量原样取 default（含 falsy 与 null）', () => {
    expect(deriveFormDefaults({ type: 'boolean', default: false })).toBe(false);
    expect(deriveFormDefaults({ type: 'integer', default: 0 })).toBe(0);
    expect(deriveFormDefaults({ type: ['null', 'string'], default: null })).toBe(null);
    expect(deriveFormDefaults({ type: 'array', default: [] })).toEqual([]);
  });
});

// SUI-017 验收：根数组和标量经编辑（JSON 字符串）→ 取值（parse）→ 提交
// （payload 序列化）往返后保持原 JSON 类型，覆盖空数组、false、0、显式 null
// 和旧对象表单。
describe('SUI-017 根值类型往返', () => {
  const roundtrip = (schema: Parameters<typeof deriveSchemaDefaults>[0]) => {
    const seed = deriveSchemaDefaults(schema);
    // 编辑：编辑器内容即序列化文本；取值：JSON.parse 还原类型
    const edited = JSON.parse(JSON.stringify(seed, null, 2));
    return { seed, edited };
  };

  it.each([
    ['空数组', { type: 'array' }, []],
    ['false', { type: 'boolean', default: false }, false],
    ['0', { type: 'integer' }, 0],
    ['显式 null', { type: ['null', 'boolean'], default: null }, null],
    ['根数组骨架', { type: 'array', items: { type: 'string' } }, ['']],
    ['旧对象表单', { type: 'object', properties: { a: { type: 'string' } } }, { a: '' }],
  ])('%s 往返后类型不变', (_name, schema, expected) => {
    const { seed, edited } = roundtrip(schema);
    expect(seed).toEqual(expected);
    expect(edited).toEqual(expected);
    expect(Array.isArray(edited)).toBe(Array.isArray(expected));
    expect(edited === null).toBe(expected === null);
  });
});
