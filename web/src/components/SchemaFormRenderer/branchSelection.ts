/**
 * SUI-042：discriminator 分支选择。
 *
 * OpenAPI 多态（`oneOf`/`anyOf` + `discriminator`）声明的分支由
 * discriminator 属性值决定；结构化表单按所选分支合并 schema、渲染该分支
 * 字段并校验，切换分支后只保留所选分支允许的值。分支无法明确解析的组合
 * 不猜语义，交回 JSON 降级（诊断说明具体原因）。
 */
import type { FormValues, JSONSchema, JSONValue } from '@/types/dashboard';

/** 单个可解析分支：discriminator 值、展示标签与该分支自身的 schema。 */
export interface DiscriminatorBranch {
  /** discriminator 属性值（mapping 键或分支 const/enum 唯一项）。 */
  value: string;
  /** 下拉展示标签：分支 title 优先，否则 discriminator 值。 */
  label: string;
  /** 分支 subschema（已从 `$ref` 解析或原样内联）。 */
  schema: JSONSchema;
}

/** 解析结果三态：非多态根（null，走既有流程）/ 可解析 / 不可解析。 */
export type DiscriminatorResolution =
  | { status: 'resolved'; propertyName: string; branches: DiscriminatorBranch[] }
  | { status: 'unresolved'; reason: string };

interface DiscriminatorObject {
  propertyName?: unknown;
  mapping?: unknown;
}

function isObjectNode(value: JSONValue | undefined): value is Record<string, JSONValue> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

/** 按指针在文档根内解析 `#/...` 引用（`$defs`/`definitions`/`components.schemas`）。 */
function resolveInlineRef(root: JSONSchema, ref: string): JSONSchema | null {
  if (!ref.startsWith('#/')) return null;
  let node: JSONValue = root;
  for (const raw of ref.slice(2).split('/')) {
    const segment = raw.replace(/~1/g, '/').replace(/~0/g, '~');
    if (!isObjectNode(node) || !(segment in node)) return null;
    node = node[segment];
  }
  return isObjectNode(node) ? (node as JSONSchema) : null;
}

/** 读取分支 subschema 上 discriminator 属性的声明值（const 或唯一 enum 项）。 */
function declaredBranchValue(schema: JSONSchema, propertyName: string): string | null {
  const props = schema.properties;
  if (!isObjectNode(props) || !isObjectNode(props[propertyName])) return null;
  const decl = props[propertyName];
  if ('const' in decl) {
    const c = decl.const;
    return typeof c === 'string' ? c : String(c as JSONValue);
  }
  if (Array.isArray(decl.enum) && decl.enum.length === 1) {
    return String(decl.enum[0] as JSONValue);
  }
  return null;
}

/** 从分支 subschema 解析（value, schema）：mapping 引用解析失败视为不可解析分支。 */
function resolveBranchNode(
  root: JSONSchema,
  node: JSONValue,
  propertyName: string,
  mapping: Record<string, string>,
): { value: string; schema: JSONSchema } | null {
  if (typeof node === 'string') {
    // 分支是 `$ref`：mapping[value] = ref 时按映射值命名；无映射则放弃（无法定值）。
    const schema = resolveInlineRef(root, node);
    if (!schema) return null;
    const mapped = Object.entries(mapping).find(([, target]) => target === node);
    if (mapped) return { value: mapped[0], schema };
    return null;
  }
  if (!isObjectNode(node)) return null;
  const schema = node as JSONSchema;
  if (typeof node.$ref === 'string') {
    const resolved = resolveInlineRef(root, node.$ref);
    if (!resolved) return null;
    const mapped = Object.entries(mapping).find(([, target]) => target === node.$ref);
    const value = mapped ? mapped[0] : declaredBranchValue(resolved, propertyName);
    if (value === null) return null;
    return { value, schema: resolved };
  }
  const value = declaredBranchValue(schema, propertyName);
  if (value === null) return null;
  return { value, schema };
}

/**
 * 解析对象根的 discriminator 分支声明。根无 `oneOf`/`anyOf`+`discriminator`
 * 返回 null（非多态，走既有流程）；声明了 discriminator 但分支值无法明确
 * 解析（缺 propertyName、分支缺 const/enum、引用悬空、分支值冲突）时返回
 * unresolved 与具体原因——由调用方走 JSON 降级，不猜语义。
 */
export function resolveDiscriminatorBranches(
  schema: JSONSchema | null | undefined,
): DiscriminatorResolution | null {
  if (!isObjectNode(schema as JSONValue)) return null;
  const root = schema as JSONSchema;
  const disc = root.discriminator;
  if (!isObjectNode(disc as JSONValue)) return null;
  const { propertyName, mapping: rawMapping } = disc as DiscriminatorObject;
  if (typeof propertyName !== 'string' || propertyName === '') {
    return { status: 'unresolved', reason: 'discriminator.propertyName missing' };
  }
  const mapping: Record<string, string> = {};
  if (isObjectNode(rawMapping as JSONValue)) {
    for (const [value, target] of Object.entries(rawMapping as Record<string, JSONValue>)) {
      if (typeof target === 'string') mapping[value] = target;
    }
  }
  const variants: JSONValue[] | undefined = (
    Array.isArray(root.oneOf) ? root.oneOf : Array.isArray(root.anyOf) ? root.anyOf : undefined
  ) as JSONValue[] | undefined;
  if (!variants || variants.length === 0) {
    return {
      status: 'unresolved',
      reason: `no oneOf/anyOf variants under discriminator "${propertyName}"`,
    };
  }
  const branches: DiscriminatorBranch[] = [];
  for (const node of variants) {
    const resolvedBranch = resolveBranchNode(root, node, propertyName, mapping);
    if (!resolvedBranch) {
      return {
        status: 'unresolved',
        reason: `branch ${branches.length + 1}/${variants.length} does not declare discriminator "${propertyName}" via const/enum or mapping`,
      };
    }
    branches.push({
      value: resolvedBranch.value,
      label:
        (typeof resolvedBranch.schema.title === 'string' && resolvedBranch.schema.title) ||
        resolvedBranch.value,
      schema: resolvedBranch.schema,
    });
  }
  const seen = new Set<string>();
  for (const branch of branches) {
    if (seen.has(branch.value)) {
      return {
        status: 'unresolved',
        reason: `duplicate discriminator value "${branch.value}" across branches`,
      };
    }
    seen.add(branch.value);
  }
  return { status: 'resolved', propertyName, branches };
}

/**
 * 把所选分支合并进基础 schema（基础保留 title/description/基线属性，分支
 * 覆盖自己的 properties/required；`oneOf`/`anyOf`/`discriminator` 移除——
 * 渲染与校验只面向所选分支，见 pruneValuesToBranch）。深拷贝，不改输入。
 */
export function mergeBranchIntoSchema(
  base: JSONSchema,
  branch: DiscriminatorBranch,
  propertyName: string,
): JSONSchema {
  const merged: JSONSchema = JSON.parse(JSON.stringify(base)) as JSONSchema;
  delete merged.oneOf;
  delete merged.anyOf;
  delete merged.discriminator;
  const branchSchema = branch.schema;
  const branchProps = isObjectNode(branchSchema.properties as JSONValue)
    ? (branchSchema.properties as Record<string, JSONSchema>)
    : {};
  const baseProps = isObjectNode(merged.properties as JSONValue)
    ? (merged.properties as Record<string, JSONSchema>)
    : {};
  const props: Record<string, JSONSchema> = { ...baseProps };
  for (const [key, value] of Object.entries(branchProps)) {
    props[key] = value;
  }
  merged.properties = props;
  const baseRequired = Array.isArray(merged.required)
    ? (merged.required as JSONValue[]).filter((v): v is string => typeof v === 'string')
    : [];
  const branchRequired = Array.isArray(branchSchema.required)
    ? (branchSchema.required as JSONValue[]).filter((v): v is string => typeof v === 'string')
    : [];
  const required = Array.from(new Set([...baseRequired, ...branchRequired, propertyName]));
  if (required.length > 0) merged.required = required;
  if (!merged.type) merged.type = 'object';
  return merged;
}

/**
 * 切换分支后按合并 schema 裁剪表单值：只保留所选分支允许的顶层键，并预置
 * discriminator 属性为该分支的值（SUI-042：切换分支后只提交所选分支允许
 * 的值）。共享键（两侧都声明的属性）保留已填值。
 */
export function pruneValuesToBranch(
  values: FormValues,
  mergedSchema: JSONSchema,
  propertyName: string,
  branchValue: string,
): FormValues {
  const props = isObjectNode(mergedSchema.properties as JSONValue)
    ? (mergedSchema.properties as Record<string, JSONValue>)
    : {};
  const allowed = new Set(Object.keys(props));
  allowed.add(propertyName);
  const next: FormValues = { [propertyName]: branchValue };
  for (const [key, value] of Object.entries(values)) {
    if (allowed.has(key) && key !== propertyName) next[key] = value;
  }
  return next;
}
