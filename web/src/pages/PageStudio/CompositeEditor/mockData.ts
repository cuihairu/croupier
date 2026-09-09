/** V5 预览模拟数据：按函数 outputSchema 动态合成假数据（不调用真实函数）。
 *
 * 结构遍历（object/array/enum/type）+ @faker-js/faker zh_CN 值生成——
 * 字段名启发式映射到 faker API（uid/nickname/email/phone/time/city…），
 * 组装表达式/联动时无需真实 agent 在线，全链可用假数据走通。
 */
import { fakerZH_CN as faker } from '@faker-js/faker';
import type { FunctionDescriptor } from '@/services/api/functions';
import type { JSONSchema, JSONValue } from '@/types/dashboard';

type SchemaObj = Record<string, JSONValue>;

function asSchemaObj(v: unknown): SchemaObj | undefined {
  return v && typeof v === 'object' && !Array.isArray(v) ? (v as SchemaObj) : undefined;
}

/** 字段名启发式 → faker 生成器（zh_CN，可读中文数据）。 */
function mockStringByField(field: string, index: number): string {
  const f = field.toLowerCase();
  if (/(uid|uuid|_id|^id$)/.test(f)) return `u-${1001 + index}`;
  if (/(nickname|nick)/.test(f)) return faker.person.fullName();
  if (/(name|title|label)$/.test(f)) return `${faker.word.noun()}${index + 1}`;
  if (/(time|_at|date|day)$/.test(f)) {
    return faker.date.recent({ days: 7 + index }).toISOString();
  }
  if (/(phone|mobile)/.test(f)) return faker.phone.number();
  if (/(mail|email)/.test(f)) return faker.internet.email();
  if (/(url|link|avatar|icon)$/.test(f)) return faker.internet.url();
  if (/(status|state)$/.test(f)) {
    return faker.helpers.arrayElement(['active', 'normal', 'enabled', 'frozen']);
  }
  if (/(region|area|city|country)$/.test(f)) return faker.location.city();
  if (/(address|addr)$/.test(f)) return faker.location.streetAddress();
  if (/(level|grade|rank)$/.test(f)) return `Lv.${(index % 9) + 1}`;
  return `${faker.word.noun()}${index + 1}`;
}

/** 按 schema 生成单个值；index 用于数组行间变化。 */
function mockValueBySchema(
  field: string,
  schema: JSONValue | undefined,
  index: number,
  depth: number,
): JSONValue {
  const s = asSchemaObj(schema);
  if (!s) return mockStringByField(field, index);

  // enum：候选轮换
  if (Array.isArray(s.enum) && s.enum.length > 0) {
    return s.enum[index % s.enum.length] as JSONValue;
  }
  switch (s.type) {
    case 'integer':
    case 'number': {
      const min = typeof s.minimum === 'number' ? s.minimum : 1;
      const max = typeof s.maximum === 'number' ? s.maximum : 999;
      return faker.number.int({ min, max: Math.max(min, max) });
    }
    case 'boolean':
      return index % 2 === 0;
    case 'array': {
      const items = s.items;
      const element = asSchemaObj(items);
      if (!element) return [];
      return [0, 1, 2].map((i) => mockValueBySchema(field, items, index * 3 + i, depth + 1));
    }
    case 'object': {
      const props = asSchemaObj(s.properties);
      if (!props || depth >= 3) return {};
      const out: Record<string, JSONValue> = {};
      for (const [k, ps] of Object.entries(props)) {
        out[k] = mockValueBySchema(k, ps, index, depth + 1);
      }
      return out;
    }
    default:
      return mockStringByField(field, index);
  }
}

/**
 * 生成函数输出假数据：顶层遍历 outputSchema.properties；数组字段（items 等
 * 集合语义）生成 3 行元素数据，其余字段按启发式生成单值。
 * 无 outputSchema 时返回 undefined（调用方回退真实调用或空态）。
 */
export function generateMockOutput(
  outputSchema: JSONSchema | JSONValue | undefined,
  options?: { rowCount?: number },
): JSONValue | undefined {
  const s = asSchemaObj(outputSchema);
  const props = asSchemaObj(s?.properties);
  if (!props) return undefined;
  const rowCount = options?.rowCount ?? 3;

  const out: Record<string, JSONValue> = {};
  for (const [field, ps] of Object.entries(props)) {
    const p = asSchemaObj(ps);
    if (p?.type === 'array') {
      const element = asSchemaObj(p.items);
      const elementProps = element ? asSchemaObj(element.properties) : undefined;
      if (elementProps) {
        out[field] = Array.from({ length: rowCount }, (_, i) => {
          const row: Record<string, JSONValue> = {};
          for (const [k, v] of Object.entries(elementProps)) {
            row[k] = mockValueBySchema(k, v, i, 1);
          }
          return row;
        });
      } else {
        out[field] = Array.from({ length: rowCount }, (_, i) =>
          mockValueBySchema(field, p.items, i, 1),
        );
      }
      continue;
    }
    out[field] = mockValueBySchema(field, ps, 0, 1);
  }
  return out;
}

/** 从函数契约生成模拟响应（与 invokeFunction 返回同形态：{ data }）。 */
export function generateMockResponse(
  fn: FunctionDescriptor | undefined,
): { data: JSONValue } | undefined {
  if (!fn) return undefined;
  const data = generateMockOutput(fn.outputSchema);
  return data === undefined ? undefined : { data };
}
