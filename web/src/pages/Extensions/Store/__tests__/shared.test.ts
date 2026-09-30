/**
 * Store shared 纯逻辑单测（覆盖率巡检：buildSchemaDefaults /
 * normalizeConfigBySchema 矩阵收口，Extensions 簇收尾件）。
 *
 * buildSchemaDefaults：schema 缺省/非对象、properties 缺省/非对象 → {}；
 * 仅拾取显式携带 default 键的属性（hasOwnProperty 口径，非真值判定——
 * default: false/0 也要拾取）；null 属性条目安全跳过。
 * normalizeConfigBySchema：无 schema/无 properties 时 rawConfig 原样透传
 * （rawConfig 缺省 → {}）；number/integer 数字串转换（NaN 串保留原文、
 * integer 截断）；boolean 'true'/'1'/'false'/'0'（含大小写与空白）、
 * 其他串不动；array/object JSON 串解析（坏 JSON 保留原文交后端校验）；
 * null 值跳过；未知类型不动。
 */
import { buildSchemaDefaults, normalizeConfigBySchema } from '../shared';

describe('buildSchemaDefaults', () => {
  it('schema 缺省/非对象 → {}', () => {
    expect(buildSchemaDefaults(undefined)).toEqual({});
    expect(buildSchemaDefaults(null as never)).toEqual({});
  });

  it('properties 缺省/非对象 → {}', () => {
    expect(buildSchemaDefaults({})).toEqual({});
    expect(buildSchemaDefaults({ properties: 'x' as never })).toEqual({});
  });

  it('仅拾取显式 default 键（falsy default 也拾取），null 条目跳过', () => {
    expect(
      buildSchemaDefaults({
        properties: {
          withDefault: { type: 'string', default: 'a' },
          falseDefault: { type: 'boolean', default: false },
          zeroDefault: { type: 'number', default: 0 },
          noDefault: { type: 'string' },
          nullEntry: null as never,
        },
      }),
    ).toEqual({ withDefault: 'a', falseDefault: false, zeroDefault: 0 });
  });
});

describe('normalizeConfigBySchema', () => {
  const schema = {
    properties: {
      port: { type: 'number' },
      count: { type: 'integer' },
      enabled: { type: 'boolean' },
      tags: { type: 'array' },
      extra: { type: 'object' },
      plain: { type: 'string' },
    },
  };

  it('无 schema / 无 properties：rawConfig 原样透传，rawConfig 缺省 → {}', () => {
    const raw = { a: 1 };
    expect(normalizeConfigBySchema(raw, undefined)).toBe(raw);
    expect(normalizeConfigBySchema(raw, {})).toBe(raw);
    expect(normalizeConfigBySchema(undefined as never, undefined)).toEqual({});
    // schema/properties 非对象（真值但形态错）→ 同样原样透传
    expect(normalizeConfigBySchema(raw, 'x' as never)).toBe(raw);
    expect(normalizeConfigBySchema(raw, { properties: 'x' as never })).toBe(raw);
  });

  it('键缺省（value undefined）→ 跳过归一，配置原样', () => {
    expect(normalizeConfigBySchema({}, schema)).toEqual({});
  });

  it('rawConfig 缺省但 schema 有效 → {} 起点（防御翼）', () => {
    expect(normalizeConfigBySchema(undefined as never, schema)).toEqual({});
    // 31/33 两行的 rawConfig||{} 兜底：缺省 rawConfig 配坏形态 schema
    expect(normalizeConfigBySchema(undefined as never, { properties: 'x' as never })).toEqual({});
  });

  it('属性条目 null / 缺 type → 不归一，值原样保留', () => {
    const oddSchema = { properties: { a: null as never, b: {} } };
    expect(normalizeConfigBySchema({ a: '1', b: 'x' }, oddSchema)).toEqual({
      a: '1',
      b: 'x',
    });
  });

  it('number/integer：数字串转换、integer 截断、NaN 串保留原文', () => {
    expect(normalizeConfigBySchema({ port: '8080', count: '3.7' }, schema)).toEqual({
      port: 8080,
      count: 3,
    });
    expect(normalizeConfigBySchema({ port: 'not-a-num' }, schema)).toEqual({
      port: 'not-a-num',
    });
  });

  it('boolean：true/1/false/0（含大小写空白）识别，其他串不动', () => {
    expect(normalizeConfigBySchema({ enabled: ' True ', port: '1', count: '0' }, schema)).toEqual({
      enabled: true,
      port: 1,
      count: 0,
    });
    expect(normalizeConfigBySchema({ enabled: 'yes' }, schema)).toEqual({
      enabled: 'yes',
    });
    // false/0 两臂（含空白）
    expect(normalizeConfigBySchema({ enabled: 'false' }, schema)).toEqual({
      enabled: false,
    });
    expect(normalizeConfigBySchema({ enabled: ' 0 ' }, schema)).toEqual({
      enabled: false,
    });
  });

  it('array/object：JSON 串解析，坏 JSON 保留原文（后端校验兜底）', () => {
    expect(normalizeConfigBySchema({ tags: '[1,2]', extra: '{"k":"v"}' }, schema)).toEqual({
      tags: [1, 2],
      extra: { k: 'v' },
    });
    expect(normalizeConfigBySchema({ tags: '[broken' }, schema)).toEqual({
      tags: '[broken',
    });
  });

  it('null 值跳过、未知类型/非串值不动', () => {
    expect(normalizeConfigBySchema({ enabled: null, plain: 42, tags: [3] }, schema)).toEqual({
      enabled: null,
      plain: 42,
      tags: [3],
    });
  });
});
