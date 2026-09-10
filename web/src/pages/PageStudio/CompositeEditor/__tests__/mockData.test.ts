/** V5 预览模拟数据：schema 遍历 + faker zh_CN 值生成测试。 */
import { generateMockOutput, generateMockResponse } from '../mockData';
import type { FunctionDescriptor } from '@/services/api/functions';
import type { JSONValue } from '@/types/dashboard';

const playerListSchema = {
  type: 'object',
  properties: {
    items: {
      type: 'array',
      items: {
        type: 'object',
        properties: {
          uid: { type: 'string' },
          nickname: { type: 'string' },
          level: { type: 'integer', minimum: 1, maximum: 99 },
          vip: { type: 'boolean' },
          created_at: { type: 'string' },
        },
        required: ['uid'],
      },
    },
    total: { type: 'integer' },
    keyword: { type: 'string', enum: ['all', 'gold', 'silver'] },
  },
} as JSONValue;

function asRows(v: JSONValue | undefined): Array<Record<string, unknown>> {
  const payload = v as { items?: unknown };
  return Array.isArray(payload?.items) ? (payload.items as Array<Record<string, unknown>>) : [];
}

describe('generateMockOutput', () => {
  it('按 outputSchema 生成集合字段（3 行）与标量字段', () => {
    const out = generateMockOutput(playerListSchema) as Record<string, JSONValue>;
    expect(Object.keys(out).sort()).toEqual(['items', 'keyword', 'total']);

    const rows = asRows(out);
    expect(rows).toHaveLength(3);
    for (const row of rows) {
      expect(typeof row.uid).toBe('string');
      expect(typeof row.nickname).toBe('string');
      expect(typeof row.level).toBe('number');
      expect(typeof row.vip).toBe('boolean');
      expect(typeof row.created_at).toBe('string');
    }
    expect(typeof out.total).toBe('number');
  });

  it('行间数据有变化（uid 递增 / enum 轮换）', () => {
    const rows = asRows(generateMockOutput(playerListSchema));
    const uids = rows.map((r) => r.uid);
    expect(new Set(uids).size).toBe(3);
  });

  it('无 outputSchema 返回 undefined；rowCount 可覆盖', () => {
    expect(generateMockOutput(undefined)).toBeUndefined();
    const out = generateMockOutput(playerListSchema, { rowCount: 5 }) as Record<string, JSONValue>;
    expect(asRows(out)).toHaveLength(5);
  });

  it('顶层非 object schema（array）返回 undefined——预览侧诚实空态，不伪造顶层结构', () => {
    expect(generateMockOutput({ type: 'array' } as JSONValue)).toBeUndefined();
    expect(generateMockOutput({ type: 'string' } as JSONValue)).toBeUndefined();
  });

  it('enum 生成值在候选集内', () => {
    const out = generateMockOutput(playerListSchema) as Record<string, JSONValue>;
    expect(['all', 'gold', 'silver']).toContain(out.keyword);
  });

  it('深层嵌套 object 字段递归生成', () => {
    const schema = {
      type: 'object',
      properties: {
        player: {
          type: 'object',
          properties: { profile: { type: 'object', properties: { nickname: { type: 'string' } } } },
        },
      },
    } as JSONValue;
    const out = generateMockOutput(schema) as {
      player: { profile: { nickname: unknown } };
    };
    expect(typeof out.player.profile.nickname).toBe('string');
  });
});

describe('generateMockResponse', () => {
  it('包装为 { data } 形态（与 invokeFunction 响应同构）', () => {
    const fn = {
      id: 'player.list',
      outputSchema: playerListSchema,
    } as unknown as FunctionDescriptor;
    const resp = generateMockResponse(fn);
    expect(resp?.data).toBeDefined();
    expect(asRows(resp?.data as JSONValue)).toHaveLength(3);
  });

  it('无契约返回 undefined', () => {
    expect(generateMockResponse(undefined)).toBeUndefined();
  });
});
