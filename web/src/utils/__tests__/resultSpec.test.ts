/**
 * utils/resultSpec：outputSchema → 结构化结果视图规格推导（F10）。
 *
 * 分支矩阵：顶层 object+properties → 字段卡片；array+items.properties →
 * 表格列；title 缺失走 humanize 兜底（LocalizedText 契约形态）；其余形态
 * 返回 undefined → JSON viewer 兜底；isArrayOfObjects 数据形态判定。
 */
import { deriveResultSpec, isArrayOfObjects } from '../resultSpec';
import type { JSONSchema } from '@/types/dashboard';

describe('deriveResultSpec', () => {
  it('顶层 object + properties → 字段卡片（title 优先，缺失 humanize 兜底）', () => {
    const schema = {
      type: 'object',
      properties: {
        player: { type: 'string', title: '玩家' },
        created_at: { type: 'integer' },
      },
    } as unknown as JSONSchema;
    expect(deriveResultSpec(schema)).toEqual({
      spec: {
        fields: [
          { key: 'player', title: { 'zh-CN': '玩家', 'en-US': '玩家' }, dataType: 'string' },
          {
            key: 'created_at',
            title: { 'zh-CN': 'Created At', 'en-US': 'Created At' },
            dataType: 'integer',
          },
        ],
      },
      shape: 'object',
    });
  });

  it('无 type 但有 properties 仍按 object 处理；type 缺省字段按 string', () => {
    const schema = { properties: { hits: {} } } as unknown as JSONSchema;
    expect(deriveResultSpec(schema)).toEqual({
      spec: {
        fields: [{ key: 'hits', title: { 'zh-CN': 'Hits', 'en-US': 'Hits' }, dataType: 'string' }],
      },
      shape: 'object',
    });
  });

  it('顶层 array + items.properties → 表格列（arrayOfObjects）', () => {
    const schema = {
      type: 'array',
      items: { type: 'object', properties: { id: { type: 'integer' } } },
    } as unknown as JSONSchema;
    const derived = deriveResultSpec(schema);
    expect(derived?.shape).toBe('arrayOfObjects');
    expect(derived?.spec.fields).toEqual([
      { key: 'id', title: { 'zh-CN': 'Id', 'en-US': 'Id' }, dataType: 'integer' },
    ]);
  });

  it('无 type 但有 items 仍按 arrayOfObjects 处理', () => {
    const schema = { items: { properties: { name: { type: 'string' } } } } as unknown as JSONSchema;
    expect(deriveResultSpec(schema)?.shape).toBe('arrayOfObjects');
  });

  it('object 空 properties / array 空 items.properties → undefined（JSON viewer 兜底）', () => {
    expect(
      deriveResultSpec({ type: 'object', properties: {} } as unknown as JSONSchema),
    ).toBeUndefined();
    expect(
      deriveResultSpec({ type: 'array', items: { properties: {} } } as unknown as JSONSchema),
    ).toBeUndefined();
  });

  it('标量类型 / 无 schema / 损坏形态 → undefined', () => {
    expect(deriveResultSpec({ type: 'string' } as unknown as JSONSchema)).toBeUndefined();
    expect(deriveResultSpec(null)).toBeUndefined();
    expect(deriveResultSpec(undefined)).toBeUndefined();
    expect(deriveResultSpec('not-a-schema' as unknown as JSONSchema)).toBeUndefined();
  });
});

describe('isArrayOfObjects', () => {
  it('非空且全为对象 → true；空数组/混入标量/null → false', () => {
    expect(isArrayOfObjects([{ a: 1 }, { b: 2 }])).toBe(true);
    expect(isArrayOfObjects([])).toBe(false);
    expect(isArrayOfObjects([{ a: 1 }, 'scalar'])).toBe(false);
    expect(isArrayOfObjects(null)).toBe(false);
    expect(isArrayOfObjects(undefined)).toBe(false);
  });
});
