/**
 * 呈现字段的 JSON Pointer 路径寻址（SUI-019）。
 *
 * FormPresentationSpec 的字段标识（`FormFieldSpec.key` 与 `FormGroupSpec.fields`）
 * 有两种形态，渲染侧一律经本模块解析，禁止再各自按顶层属性名取值：
 *
 * 1. **兼容旧顶层 key**：不含前导 `/` 的裸 key，即 schema 根 `properties` 下的
 *    属性名（Go 生成器 `buildFormFields` 与历史前端 `derivePresentationSpec`
 *    只产出这一形态，行为不变）。点号分隔的嵌套路径不是指针——按裸 key 原样
 *    当作字面属性名处理，不与 `/a/b` 混用。
 * 2. **JSON Pointer（RFC 6901）**：`/address/city`、`/tags/-/name`。数组项用
 *    `-`（首项语义：指向 `items` 的子属性）；`~1` 转义 `/`、`~0` 转义 `~`，
 *    因此键名含点或斜杠也能唯一定位。
 *
 * 解析失败（路径任一段不在 schema 中）不静默降级为空 schema：调用方拿到
 * `container === null` + `reason`，据此产出诊断（见 `unknownFieldPaths`）。
 */

import { primarySchemaType } from './schemaDialect';

/** schema / uiSchema 节点：键值均可变的对象。 */
export type SchemaNode = Record<string, unknown>;

/** 指针形态判定：前导 `/`。 */
export function isPointerFieldKey(key: string): boolean {
  return key.startsWith('/');
}

/** 把字段标识拆成 token 序列（裸 key = 单个 token）。 */
export function fieldPathTokens(key: string): string[] {
  if (!isPointerFieldKey(key)) return [key];
  return key
    .slice(1)
    .split('/')
    .map((token) => token.replace(/~1/g, '/').replace(/~0/g, '~'));
}

/** token 序列还原成指针形态（裸 key 原样返回）。 */
export function joinFieldPath(key: string, tokens: string[]): string {
  if (!isPointerFieldKey(key)) return key;
  return `/${tokens.map((token) => token.replace(/~/g, '~0').replace(/\//g, '~1')).join('/')}`;
}

function asObject(value: unknown): SchemaNode | null {
  return typeof value === 'object' && value !== null && !Array.isArray(value)
    ? (value as SchemaNode)
    : null;
}

function propertiesOf(node: unknown): SchemaNode | null {
  return asObject((asObject(node) ?? {})['properties']);
}

function isArraySchema(node: unknown): boolean {
  if (typeof node !== 'object' || node === null) return false;
  const object = node as SchemaNode;
  return primarySchemaType({ type: object['type'] as string | string[] | undefined }) === 'array';
}

/** 非末段 token 的下一节点：数组 schema 取 `items`，对象取对应属性。 */
function childNode(node: unknown, token: string): unknown | undefined {
  const object = asObject(node);
  if (!object) return undefined;
  if (isArraySchema(object)) return object['items'];
  const properties = propertiesOf(node);
  if (!properties || !Object.prototype.hasOwnProperty.call(properties, token)) return undefined;
  return properties[token];
}

export interface FieldSchemaTarget {
  /** 原始字段标识 */
  key: string;
  /** 解析路径的 token 序列 */
  tokens: string[];
  /** uiSchema 侧同路径的段序列：穿过数组的段归为 `-`（rjsf 数组项 ui 约定） */
  uiTokens: string[];
  /** 持有该字段 schema 的 `properties` 容器；未知路径为 null */
  container: SchemaNode | null;
  /** 容器的宿主 schema 节点（`properties` 与 `required` 都在其上）；未知路径为 null */
  parent: SchemaNode | null;
  /** 字段在容器中的属性名；未知路径为 null */
  property: string | null;
  /** 字段自身 schema；未知路径为 null */
  schema: SchemaNode | null;
  /** 未知路径原因（诊断用；解析成功时无） */
  reason?: string;
}

/** 在 schema 中定位字段：裸 key 与指针两条路径同返回形态。 */
export function resolveFieldSchema(root: unknown, key: string): FieldSchemaTarget {
  const tokens = fieldPathTokens(key);
  // 未知路径的 uiTokens 用裸 token 兜底：调用方拿到 container === null 时不再走 ui 注入
  const missing = { key, tokens, uiTokens: tokens, container: null, parent: null, property: null, schema: null };
  if (tokens.length === 0 || tokens[0] === '') return { ...missing, reason: '路径为空' };

  let current: unknown = asObject(root) ?? {};
  const uiPath: string[] = [];
  for (let i = 0; i < tokens.length - 1; i += 1) {
    uiPath.push(isArraySchema(current) ? '-' : tokens[i]);
    const next = childNode(current, tokens[i]);
    if (next === undefined) {
      return { ...missing, reason: `路径段 "${tokens[i]}" 在 schema 中不存在` };
    }
    current = next;
  }
  uiPath.push(tokens[tokens.length - 1]);

  const properties = propertiesOf(current);
  const last = tokens[tokens.length - 1];
  if (!properties || !Object.prototype.hasOwnProperty.call(properties, last)) {
    return {
      ...missing,
      reason: `字段 "${last}" 不在 ${joinFieldPath(key, tokens)} 的 properties 中`,
    };
  }
  return {
    key,
    tokens,
    uiTokens: uiPath,
    container: properties,
    parent: current as SchemaNode,
    property: last,
    schema: properties[last] as SchemaNode,
  };
}

/** 按 uiTokens 取（不存在即逐层创建）uiSchema 分支，返回最深层节点。 */
export function uiSchemaNodeAt(uiSchema: SchemaNode, tokens: string[]): SchemaNode {
  let current: SchemaNode = uiSchema;
  for (const token of tokens) {
    const next = asObject(current[token]);
    if (next === null) {
      const created: SchemaNode = {};
      current[token] = created;
      current = created;
    } else {
      current = next;
    }
  }
  return current;
}

/** 从 required 列表中摘除/追加字段名（required 属于 parent 宿主 schema）。 */
export function toggleRequiredAt(
  target: FieldSchemaTarget,
  key: string,
  add: boolean,
): boolean {
  const parent = target.parent;
  if (!parent) return false;
  const required: string[] = Array.isArray(parent['required']) ? [...(parent['required'] as string[])] : [];
  const index = required.indexOf(key);
  let changed = false;
  if (add && index < 0) {
    required.push(key);
    changed = true;
  }
  if (!add && index >= 0) {
    required.splice(index, 1);
    changed = true;
  }
  if (changed) parent['required'] = required;
  return changed;
}

/** 数组项形态（含 `-` 或数组下标 token）：路径穿过数组，值裁剪不适用。 */
export function crossesArray(tokens: string[]): boolean {
  return tokens.some((token) => token === '-' || /^\d+$/.test(token));
}

/** 按字段标识从表单值中取值：裸 key 直接取，指针逐段取（数组按整数下标）。 */
export function valueAtFieldPath(
  values: Record<string, unknown> | undefined,
  key: string,
): unknown {
  if (!values) return undefined;
  if (!isPointerFieldKey(key)) return values[key];
  let current: unknown = values;
  for (const token of fieldPathTokens(key)) {
    if (Array.isArray(current)) {
      const index = Number(token);
      if (!Number.isInteger(index) || index < 0 || index >= current.length) return undefined;
      current = current[index];
      continue;
    }
    const object = asObject(current);
    if (!object || !Object.prototype.hasOwnProperty.call(object, token)) return undefined;
    current = object[token];
  }
  return current;
}

/** 按字段标识从值中剔除该键（不可变）。路径穿过数组时原样返回——item 级
 *  呈现提示不能裁掉整个数组（SUI-019 边界：数组项按数组整体提交/裁剪）。 */
export function deleteAtPath(
  values: Record<string, unknown>,
  key: string,
): Record<string, unknown> {
  const tokens = fieldPathTokens(key);
  if (tokens.length === 1) {
    if (!Object.prototype.hasOwnProperty.call(values, tokens[0])) return values;
    const next = { ...values };
    delete next[tokens[0]];
    return next;
  }
  if (isPointerFieldKey(key) && crossesArray(tokens)) return values;

  const chain: { parent: Record<string, unknown>; token: string }[] = [];
  let current: SchemaNode | undefined = values;
  for (const token of tokens.slice(0, -1)) {
    const object = asObject(current);
    if (!object || !Object.prototype.hasOwnProperty.call(object, token)) return values;
    chain.push({ parent: object, token });
    const next: SchemaNode | null = asObject(object[token]);
    if (!next) return values;
    current = next;
  }
  if (!current || !Object.prototype.hasOwnProperty.call(current, tokens[tokens.length - 1])) {
    return values;
  }
  // 自最深层逐层回卷，保证未触及的兄弟分支共享引用
  let rebuilt: Record<string, unknown> = { ...current };
  delete rebuilt[tokens[tokens.length - 1]];
  for (let i = chain.length - 1; i >= 0; i -= 1) {
    const { parent, token } = chain[i];
    rebuilt = { ...parent, [token]: rebuilt };
  }
  return rebuilt;
}

/** 按字段标识写入值（不可变）。路径穿过数组时原样返回。 */
export function setAtPath(
  values: Record<string, unknown>,
  key: string,
  value: unknown,
): Record<string, unknown> {
  const tokens = fieldPathTokens(key);
  if (tokens.length === 1) return { ...values, [tokens[0]]: value };
  if (isPointerFieldKey(key) && crossesArray(tokens)) return values;

  const chain: { parent: Record<string, unknown>; token: string }[] = [];
  let current: SchemaNode = values;
  for (const token of tokens.slice(0, -1)) {
    const object: SchemaNode = asObject(current) ?? {};
    chain.push({ parent: object, token });
    current = asObject(object[token]) ?? {};
  }
  let rebuilt: Record<string, unknown> = { ...current, [tokens[tokens.length - 1]]: value };
  for (let i = chain.length - 1; i >= 0; i -= 1) {
    const { parent, token } = chain[i];
    rebuilt = { ...parent, [token]: rebuilt };
  }
  return rebuilt;
}

/** 诊断：呈现配置里定位不到的字段标识（spec.fields 与 groups.fields 全量核对）。 */
export function unknownFieldPaths(
  fields: Array<{ key: string }>,
  groupFields: string[],
  schema: unknown,
): string[] {
  const reasons: string[] = [];
  for (const key of [...fields.map((field) => field.key), ...groupFields]) {
    if (!key) continue;
    const target = resolveFieldSchema(schema, key);
    if (!target.container) reasons.push(`${key}${target.reason ? `：${target.reason}` : ''}`);
  }
  return reasons;
}
