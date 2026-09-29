/**
 * 扩展商店 shared 纯函数单测（覆盖率巡检：Extensions 簇余量第五批，
 * shared.ts 64 行 normalizeConfigBySchema 73% → 全分支收口）。
 *
 * 锁定契约：buildSchemaDefaults 四守卫翼（schema 缺省/非对象、properties
 * 缺省/非对象）+ 属性遍历（default 键提取、default:null 仍提取、无 default
 * 跳过、prop null 兜底空对象）；normalizeConfigBySchema 同组守卫翼 +
 * rawConfig 缺省 `|| {}` 右翼 + 类型归一矩阵：
 * - number：字符串 '42' → 42；integer：'3.7' → Math.trunc 3；NaN 翼保持原文
 * - boolean：'true'/'1'（含大小写与空白 trim）→ true；'false'/'0' → false；
 *   其他串（'yes'）两翼均不命中保持原文
 * - array/object：合法 JSON 串解析；非法串 catch 翼保持原文（后端校验拦截）
 * - value undefined/null 翼跳过；值类型已匹配/字段无类型/prop null → 不动
 * - schema 外多余键保留（浅拷贝语义：原入参不被变异）
 *
 * 边界（诚实）：分支 97.67% 唯一缺口是 `schema?.properties` 可选链的 null
 * 短路翼——函数头部 `!schema` 判空守卫已提前返回，防御式 `?.` 结构上不可达，
 * 登记不造假。另实证锁定：`{properties: []}` 因数组 typeof 恒 object 穿过
 * properties 守卫，返回值相等的新对象（拷贝语义）。
 */
import type { JSONValue } from '@/types/dashboard';
import { buildSchemaDefaults, normalizeConfigBySchema } from '../shared';

describe('buildSchemaDefaults 守卫翼', () => {
  it('schema 缺省 / 非对象 / properties 缺省 / properties 非对象 → {}', () => {
    expect(buildSchemaDefaults(undefined)).toEqual({});
    expect(buildSchemaDefaults('nope' as unknown as Record<string, JSONValue>)).toEqual({});
    expect(buildSchemaDefaults({})).toEqual({});
    expect(
      buildSchemaDefaults({ properties: 'x' } as unknown as Record<string, JSONValue>),
    ).toEqual({});
  });
});

describe('buildSchemaDefaults 属性遍历', () => {
  it('提取 default 键：有值/null 均提取，无 default 与 prop null 跳过', () => {
    const schema = {
      properties: {
        rate: { type: 'number', default: 5 },
        off: { type: 'boolean', default: null },
        plain: { type: 'string' },
        broken: null,
      },
    } as unknown as Record<string, JSONValue>;

    expect(buildSchemaDefaults(schema)).toStrictEqual({ rate: 5, off: null });
  });
});

describe('normalizeConfigBySchema 守卫翼', () => {
  const raw = { keep: 'me' } as Record<string, JSONValue>;

  it('schema 缺省 / 非对象 / properties 缺省 / properties 非对象 → 原样返回', () => {
    expect(normalizeConfigBySchema(raw, undefined)).toBe(raw);
    expect(normalizeConfigBySchema(raw, 42 as unknown as Record<string, JSONValue>)).toBe(raw);
    expect(normalizeConfigBySchema(raw, {})).toBe(raw);
    expect(
      normalizeConfigBySchema(raw, { properties: 'x' } as unknown as Record<string, JSONValue>),
    ).toBe(raw);
  });

  it('properties 为空数组（typeof 恒 object）穿过守卫：无键可归一 → 值相等的新对象', () => {
    const out = normalizeConfigBySchema(raw, { properties: [] } as unknown as Record<
      string,
      JSONValue
    >);
    expect(out).toEqual(raw);
    expect(out).not.toBe(raw);
  });

  it('rawConfig 缺省 → `|| {}` 右翼（schema 有效与 properties 缺省两形态）', () => {
    expect(normalizeConfigBySchema(undefined as unknown as Record<string, JSONValue>, {})).toEqual(
      {},
    );
    expect(
      normalizeConfigBySchema(undefined as unknown as Record<string, JSONValue>, {
        properties: { a: { type: 'number' } },
      }),
    ).toEqual({});
  });
});

describe('normalizeConfigBySchema number/integer 归一', () => {
  const schemaOf = (type: string) =>
    ({ properties: { n: { type } } }) as unknown as Record<string, JSONValue>;

  it("number：'42' → 42；NaN 翼 'abc' 保持原文", () => {
    expect(normalizeConfigBySchema({ n: '42' }, schemaOf('number'))).toEqual({ n: 42 });
    expect(normalizeConfigBySchema({ n: 'abc' }, schemaOf('number'))).toEqual({ n: 'abc' });
  });

  it("integer：'3.7' → Math.trunc 3；NaN 翼保持原文", () => {
    expect(normalizeConfigBySchema({ n: '3.7' }, schemaOf('integer'))).toEqual({ n: 3 });
    expect(normalizeConfigBySchema({ n: 'x' }, schemaOf('integer'))).toEqual({ n: 'x' });
  });

  it('值已是数字 → 不动；字段无 type / prop null → 不动', () => {
    expect(normalizeConfigBySchema({ n: 7 }, schemaOf('number'))).toEqual({ n: 7 });
    const noType = { properties: { n: {} } } as unknown as Record<string, JSONValue>;
    expect(normalizeConfigBySchema({ n: '42' }, noType)).toEqual({ n: '42' });
    const nullProp = { properties: { n: null } } as unknown as Record<string, JSONValue>;
    expect(normalizeConfigBySchema({ n: 'true' }, nullProp)).toEqual({ n: 'true' });
  });
});

describe('normalizeConfigBySchema boolean 归一', () => {
  const boolSchema = {
    properties: { flag: { type: 'boolean' } },
  } as unknown as Record<string, JSONValue>;

  it("'true'/'1'（含 trim+大小写）→ true；'false'/'0' → false", () => {
    expect(normalizeConfigBySchema({ flag: 'true' }, boolSchema)).toEqual({ flag: true });
    expect(normalizeConfigBySchema({ flag: '  TRUE  ' }, boolSchema)).toEqual({ flag: true });
    expect(normalizeConfigBySchema({ flag: '1' }, boolSchema)).toEqual({ flag: true });
    expect(normalizeConfigBySchema({ flag: 'false' }, boolSchema)).toEqual({ flag: false });
    expect(normalizeConfigBySchema({ flag: '0' }, boolSchema)).toEqual({ flag: false });
  });

  it("其他串 'yes' 两翼均不命中 → 保持原文", () => {
    expect(normalizeConfigBySchema({ flag: 'yes' }, boolSchema)).toEqual({ flag: 'yes' });
  });
});

describe('normalizeConfigBySchema array/object 归一', () => {
  it("array：合法 JSON 串 '…' 解析为数组", () => {
    const schema = {
      properties: { tags: { type: 'array' } },
    } as unknown as Record<string, JSONValue>;
    expect(normalizeConfigBySchema({ tags: '["a","b"]' }, schema)).toEqual({ tags: ['a', 'b'] });
  });

  it('object：合法 JSON 串解析为对象；非法串 catch 翼保持原文', () => {
    const schema = {
      properties: { meta: { type: 'object' } },
    } as unknown as Record<string, JSONValue>;
    expect(normalizeConfigBySchema({ meta: '{"k":1}' }, schema)).toEqual({ meta: { k: 1 } });
    expect(normalizeConfigBySchema({ meta: 'not json' }, schema)).toEqual({ meta: 'not json' });
  });
});

describe('normalizeConfigBySchema 值翼与拷贝语义', () => {
  const schema = {
    properties: {
      a: { type: 'number' },
      b: { type: 'boolean' },
      c: { type: 'array' },
    },
  } as unknown as Record<string, JSONValue>;

  it('value undefined/null 翼：逐键跳过（undefined 键不产生、null 保持）', () => {
    expect(normalizeConfigBySchema({ b: null }, schema)).toStrictEqual({ b: null });
  });

  it('schema 外多余键保留 + 原入参不被变异（浅拷贝）', () => {
    const input = { a: '42', extra: 'keep' } as Record<string, JSONValue>;
    const out = normalizeConfigBySchema(input, schema);

    expect(out).toEqual({ a: 42, extra: 'keep' });
    expect(input).toEqual({ a: '42', extra: 'keep' });
    expect(out).not.toBe(input);
  });
});
