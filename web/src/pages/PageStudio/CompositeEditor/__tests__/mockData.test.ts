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

  it('字段名启发式：phone/email/url/status/city/address/level/name 各归其位', () => {
    const schema = {
      type: 'object',
      properties: {
        phone: { type: 'string' },
        email: { type: 'string' },
        avatar: { type: 'string' },
        status: { type: 'string' },
        city: { type: 'string' },
        address: { type: 'string' },
        level: { type: 'string' }, // string 型等级走 Lv.N 启发式（integer 走数值分支）
        petName: { type: 'string' }, // (name)$ 结尾
        whatever: { type: 'string' }, // 默认兜底：名词+序号
      },
    } as JSONValue;
    const out = generateMockOutput(schema) as Record<string, unknown>;
    expect(typeof out.phone).toBe('string');
    expect(String(out.email)).toContain('@');
    expect(String(out.avatar)).toMatch(/^https?:\/\//);
    expect(['active', 'normal', 'enabled', 'frozen']).toContain(out.status);
    expect(typeof out.city).toBe('string');
    expect(typeof out.address).toBe('string');
    expect(out.level).toBe('Lv.1'); // index 0 → (0 % 9) + 1
    expect(typeof out.petName).toBe('string');
    expect(typeof out.whatever).toBe('string');
  });

  it('嵌套 array 字段：对象元素 3 行 / 非对象 items 空数组 / 无 properties 的 object 空对象', () => {
    const schema = {
      type: 'object',
      properties: {
        meta: {
          type: 'object',
          properties: {
            tags: { type: 'array', items: { type: 'string' } },
            broken: { type: 'array', items: 'nope' },
            emptyObj: { type: 'object' },
          },
        },
      },
    } as JSONValue;
    const out = generateMockOutput(schema) as {
      meta: { tags: unknown[]; broken: unknown[]; emptyObj: unknown };
    };
    // mockValueBySchema 的 array 分支：对象元素（string schema）→ 3 行标量
    expect(out.meta.tags).toHaveLength(3);
    // items 非对象 → []
    expect(out.meta.broken).toEqual([]);
    // object 无 properties → {}
    expect(out.meta.emptyObj).toEqual({});
  });

  it('顶层数组字段 items 无对象 properties：按 items schema 逐行生成', () => {
    const schema = {
      type: 'object',
      properties: {
        topTags: { type: 'array', items: { type: 'string' } },
        nums: { type: 'array', items: { type: 'integer' } },
      },
    } as JSONValue;
    const out = generateMockOutput(schema) as { topTags: unknown[]; nums: number[] };
    expect(out.topTags).toHaveLength(3);
    out.topTags.forEach((t) => expect(typeof t).toBe('string'));
    expect(out.nums).toHaveLength(3);
    out.nums.forEach((n) => expect(typeof n).toBe('number'));
  });

  it('形态兜底：非对象字段走启发式字符串 / 顶层数组非对象 items / fn 无有效 schema 返回 undefined', () => {
    const schema = {
      type: 'object',
      properties: {
        bad: 'nope', // 字段 schema 本身非对象 → mockValueBySchema 走启发式兜底
        badArr: { type: 'array', items: 'x' }, // 顶层数组 items 非对象 → element falsy
      },
    } as JSONValue;
    const out = generateMockOutput(schema) as Record<string, JSONValue>;
    expect(typeof out.bad).toBe('string');
    expect(out.badArr).toHaveLength(3);
    out.badArr.forEach((v) => expect(typeof v).toBe('string'));

    // fn 存在但 outputSchema 无 properties → data undefined → 响应 undefined
    const fn = { id: 'x', outputSchema: { type: 'string' } } as unknown as FunctionDescriptor;
    expect(generateMockResponse(fn)).toBeUndefined();
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
