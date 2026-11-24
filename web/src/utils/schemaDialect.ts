/**
 * Schema 方言归一与能力判定（SUI-009）——前端侧。
 *
 * 与 Go 侧 internal/dashboard/spec/schema_dialect.go 对同一批样例判定一致：
 * 归一契约把 OpenAPI 3.0 `nullable: true` 与 3.1 `type: [T, "null"]` 统一为
 * `type` 数组（"null" 恒在末位），主类型 = 数组首个非 null 元素。表单控件
 * 派生、必填判定与能力分类都从这里取主类型/可空标记，避免 `type === 'string'`
 * 这类严格相等在 `["string","null"]`（或多类型联合）上失效。
 *
 * 能力判定与 Go `ClassifySchemaSupport` 同规则（条件 schema / 元组 /
 * 多类型联合 → degraded），样例集锁定于支持矩阵文档并被两端单测锁定。
 */

export type SchemaCapability = 'supported' | 'degraded' | 'unsupported';

type SchemaType = string | string[] | undefined;

type SchemaLike = {
  type?: SchemaType;
  nullable?: boolean;
  [key: string]: unknown;
};

function isObject(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

/** 主类型：type 字符串，或 type 数组首个非 null 元素；无 type 时 undefined。 */
export function primarySchemaType(schema: SchemaLike | undefined): string | undefined {
  const t = schema?.type;
  if (typeof t === 'string') return t;
  if (Array.isArray(t)) {
    const nonNull = t.filter((item) => item !== 'null');
    return nonNull.length ? nonNull[0] : 'null';
  }
  return undefined;
}

/** 该字段是否接受显式 null（type 数组含 "null"，或遗留 3.0 `nullable: true`）。 */
export function isNullableSchema(schema: SchemaLike | undefined): boolean {
  if (Array.isArray(schema?.type)) return (schema?.type as string[]).includes('null');
  return schema?.nullable === true;
}

function isMultiTypeUnion(type: unknown): boolean {
  if (!Array.isArray(type)) return false;
  const nonNull = type.filter((item) => item !== 'null');
  return nonNull.length > 1;
}

function hasConditionalSchema(node: Record<string, unknown>): boolean {
  return ['if', 'then', 'else', 'dependentSchemas'].some((key) => key in node);
}

function hasTuple(node: Record<string, unknown>): boolean {
  if ('prefixItems' in node) return true;
  return Array.isArray(node.items);
}

/** 能力判定：与 Go `ClassifySchemaSupport` 同规则，递归全部子 schema 位置。 */
export function classifySchemaSupport(schema: unknown): SchemaCapability {
  if (!isObject(schema)) return 'supported';
  if (hasConditionalSchema(schema) || hasTuple(schema) || isMultiTypeUnion(schema.type)) {
    return 'degraded';
  }
  for (const sub of subschemasOf(schema)) {
    if (classifySchemaSupport(sub) !== 'supported') return 'degraded';
  }
  return 'supported';
}

/** 返回一个 schema 键下的所有子 schema 节点（与 Go `subschemasOf` 同位置）。 */
function subschemasOf(node: Record<string, unknown>): unknown[] {
  const mapKeys = [
    'properties',
    'patternProperties',
    '$defs',
    'definitions',
    'dependentSchemas',
    'dependencies',
  ];
  const arrayKeys = ['allOf', 'anyOf', 'oneOf', 'prefixItems', 'items'];
  const singleKeys = [
    'additionalProperties',
    'contains',
    'propertyNames',
    'if',
    'then',
    'else',
    'not',
  ];

  const out: unknown[] = [];
  for (const key of mapKeys) {
    const value = node[key];
    if (isObject(value)) {
      for (const sub of Object.values(value)) if (isObject(sub)) out.push(sub);
    }
  }
  for (const key of arrayKeys) {
    const value = node[key];
    if (isObject(value)) {
      out.push(value);
    } else if (Array.isArray(value)) {
      for (const sub of value) if (isObject(sub)) out.push(sub);
    }
  }
  for (const key of singleKeys) {
    const value = node[key];
    if (isObject(value)) out.push(value);
  }
  return out;
}
