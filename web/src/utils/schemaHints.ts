/**
 * Descriptor 呈现 hints 推导器。
 *
 * 将 input_schema 字段上的 x-ui-* 扩展字段（契约见
 * docs/architecture/presentation-hints.md）推导为 FormPresentationSpec。
 * 纯函数；无 hints 时输出与历史行为等价的 { jsonSchema, layout: 'vertical' }。
 */

import { normalizeLocalizedText } from '@/services/api/functions-enhanced';
import { humanizeFieldKey } from '@/utils/humanize';
import type {
  ConditionSpec,
  EnumOption,
  FormFieldSpec,
  FormGroupSpec,
  FormLayout,
  FormPresentationSpec,
  FormWidget,
  JSONSchema,
  JSONValue,
  LocalizedText,
  RemoteOptionsSpec,
} from '@/types/dashboard';

const FORM_WIDGETS: readonly FormWidget[] = [
  'Input',
  'TextArea',
  'InputNumber',
  'Password',
  'Select',
  'MultiSelect',
  'Radio',
  'Checkbox',
  'Switch',
  'DatePicker',
  'TimePicker',
  'DateRange',
  'Upload',
  'ImageUpload',
  'FileUpload',
  'RichText',
  'Code',
  'Cascader',
  'TreeSelect',
  'Color',
  'Slider',
  'Rate',
  'JSON',
  'KeyValue',
  'Array',
  'Object',
];

const FORM_LAYOUTS: readonly FormLayout[] = ['vertical', 'horizontal', 'inline', 'grid'];

const CONDITION_KINDS: readonly ConditionSpec['kind'][] = [
  'equals',
  'notEquals',
  'exists',
  'all',
  'any',
];

type SchemaNode = Record<string, JSONValue>;

function isPlainObject(value: unknown): value is SchemaNode {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

function asLocalizedText(value: unknown): LocalizedText | undefined {
  // 裸字符串按平台契约双语展开（zh-CN/en-US），与 Go 生成器 hintLocalized 同规
  // （SUI-025 共享预期结果 fixture 断言两侧一致）。
  if (typeof value === 'string') {
    const text = value.trim();
    return text ? { 'zh-CN': text, 'en-US': text } : undefined;
  }
  if (isPlainObject(value)) return normalizeLocalizedText(value as Record<string, string>);
  return undefined;
}

function asWidget(value: unknown): FormWidget | undefined {
  return typeof value === 'string' && (FORM_WIDGETS as readonly string[]).includes(value)
    ? (value as FormWidget)
    : undefined;
}

const FIELD_ROLES: readonly NonNullable<FormFieldSpec['role']>[] = [
  'identity',
  'status',
  'reason',
  'expiry',
  'amount',
  'quantity',
];

function asFieldRole(value: unknown): FormFieldSpec['role'] | undefined {
  return typeof value === 'string' && (FIELD_ROLES as readonly string[]).includes(value)
    ? (value as FormFieldSpec['role'])
    : undefined;
}

function asLayout(value: unknown): FormLayout | undefined {
  return typeof value === 'string' && (FORM_LAYOUTS as readonly string[]).includes(value)
    ? (value as FormLayout)
    : undefined;
}

function asIntInRange(value: unknown, min: number, max: number): number | undefined {
  if (typeof value !== 'number' || !Number.isInteger(value) || value < min || value > max) {
    return undefined;
  }
  return value;
}

function asOrder(value: unknown): number | undefined {
  return typeof value === 'number' && Number.isFinite(value) ? value : undefined;
}

function validateCondition(value: unknown, depth = 0): ConditionSpec | undefined {
  if (!isPlainObject(value) || depth > 4) return undefined;
  const kind = value.kind;
  if (typeof kind !== 'string' || !(CONDITION_KINDS as readonly string[]).includes(kind)) {
    return undefined;
  }
  switch (kind) {
    case 'equals':
    case 'notEquals':
      if (typeof value.path !== 'string' || !value.path.startsWith('/')) return undefined;
      if (!('value' in value)) return undefined;
      return { kind, path: value.path, value: value.value as JSONValue };
    case 'exists':
      if (typeof value.path !== 'string' || !value.path.startsWith('/')) return undefined;
      return { kind, path: value.path };
    case 'all':
    case 'any': {
      if (!Array.isArray(value.conditions) || value.conditions.length === 0) return undefined;
      const conditions = value.conditions
        .map((item) => validateCondition(item, depth + 1))
        .filter((item): item is ConditionSpec => item !== undefined);
      if (!conditions.length) return undefined;
      return { kind, conditions };
    }
    default:
      return undefined;
  }
}

function asEnumOptions(value: unknown): EnumOption[] | undefined {
  if (!Array.isArray(value) || !value.length) return undefined;
  const options: EnumOption[] = [];
  for (const item of value) {
    if (!isPlainObject(item)) continue;
    // 取值域以 schema enum 为准，hints 只补标签：非 string value 一律跳过
    if (typeof item.value !== 'string') continue;
    const label = asLocalizedText(item.label);
    if (!label) continue;
    options.push({ value: item.value, label });
  }
  return options.length ? options : undefined;
}

function asWidgetProps(value: unknown): Record<string, JSONValue> | undefined {
  if (!isPlainObject(value)) return undefined;
  const entries = Object.entries(value).filter(([, v]) => v !== undefined);
  return entries.length ? (Object.fromEntries(entries) as Record<string, JSONValue>) : undefined;
}

function asGroupKey(value: unknown): string | undefined {
  return typeof value === 'string' && value.trim() ? value.trim() : undefined;
}

function asRemoteOptions(value: unknown): RemoteOptionsSpec | undefined {
  if (!isPlainObject(value)) return undefined;
  const functionId = value.functionId;
  if (typeof functionId !== 'string' || !functionId.trim()) return undefined;
  const spec: RemoteOptionsSpec = { functionId: functionId.trim() };
  if (typeof value.labelPath === 'string' && value.labelPath) spec.labelPath = value.labelPath;
  if (typeof value.valuePath === 'string' && value.valuePath) spec.valuePath = value.valuePath;
  if (typeof value.searchParam === 'string' && value.searchParam) {
    spec.searchParam = value.searchParam;
  }
  return spec;
}

function schemaPrimaryType(node: SchemaNode): string {
  const raw = node['type'];
  if (typeof raw === 'string') return raw;
  if (Array.isArray(raw)) {
    // 数组形态需全为 string（与 Go schemaTypeFromObject 同规），取首个非 null
    if (!raw.every((item) => typeof item === 'string')) return '';
    const first = (raw as string[]).find((item) => item !== 'null');
    return first ?? 'null';
  }
  return '';
}

function nodeHasEnum(value: JSONValue | undefined): boolean {
  return Array.isArray(value) && value.length > 0;
}

/**
 * 类型缺省控件（x-widget 缺省时），与 Go 生成器 defaultWidgetForSchema 同表：
 * integer/number→InputNumber、boolean→Switch、string 按格式/长度→DatePicker/
 * TimePicker/TextArea、数组项 enum→MultiSelect、enum→Select、其余交给渲染端
 * 默认。类型缺省不算 hint（不翻转 deriveLeaf 的 included）。
 */
function typeDefaultWidgetForSchema(schema: SchemaNode): FormWidget | undefined {
  switch (schemaPrimaryType(schema)) {
    case 'integer':
    case 'number':
      return 'InputNumber';
    case 'boolean':
      return 'Switch';
    case 'string': {
      const format = typeof schema.format === 'string' ? schema.format : '';
      if (format === 'date' || format === 'date-time') return 'DatePicker';
      if (format === 'time') return 'TimePicker';
      const maxLength = typeof schema.maxLength === 'number' && Number.isFinite(schema.maxLength) ? schema.maxLength : 0;
      if (format === 'textarea' || maxLength > 120) return 'TextArea';
      break;
    }
    case 'array': {
      const items = isPlainObject(schema['items']) ? schema['items'] : null;
      if (items && nodeHasEnum(items['enum'])) return 'MultiSelect';
      break;
    }
    default:
      break;
  }
  if (nodeHasEnum(schema['enum'])) return 'Select';
  return undefined;
}

interface FieldDerived {
  field: FormFieldSpec;
  /** 有任一可用 hint（含 x-group）才进入 fields 输出 */
  included: boolean;
  group?: string;
}

function deriveLeaf(key: string, schema: SchemaNode): FieldDerived {
  const field: FormFieldSpec = { key };
  let included = false;

  const widget = asWidget(schema['x-widget']);
  if (widget) {
    field.widget = widget;
    included = true;
  }
  const label = asLocalizedText(schema['x-label']);
  if (label) {
    field.label = label;
    included = true;
  }
  const placeholder = asLocalizedText(schema['x-placeholder']);
  if (placeholder) {
    field.placeholder = placeholder;
    included = true;
  }
  const description = asLocalizedText(schema['x-description']);
  if (description) {
    field.description = description;
    included = true;
  }
  const role = asFieldRole(schema['x-field-role']);
  if (role) {
    field.role = role;
    included = true;
  }
  const unit = asLocalizedText(schema['x-field-unit']);
  if (unit) {
    field.unit = unit;
    included = true;
  }
  if (schema['x-field-per-target'] === true) {
    field.perTarget = true;
    included = true;
  }
  const width = asIntInRange(schema['x-width'], 1, 12);
  if (width !== undefined) {
    field.width = width;
    included = true;
  }
  const order = asOrder(schema['x-order']);
  if (order !== undefined) {
    field.order = order;
    included = true;
  }
  if (typeof schema['x-disabled'] === 'boolean') {
    field.disabled = schema['x-disabled'];
    included = true;
  }
  const visibleWhen = validateCondition(schema['x-visible-when']);
  if (visibleWhen) {
    field.visibleWhen = visibleWhen;
    included = true;
  }
  const enumOptions = asEnumOptions(schema['x-enum-options']);
  if (enumOptions) {
    field.enumOptions = enumOptions;
    included = true;
  }
  const widgetProps = asWidgetProps(schema['x-widget-props']);
  if (widgetProps) {
    field.widgetProps = widgetProps;
    included = true;
  }
  const remoteOptions = asRemoteOptions(schema['x-options-source']);
  if (remoteOptions) {
    field.remoteOptions = remoteOptions;
    included = true;
  }

  const group = asGroupKey(schema['x-group']);
  if (group) included = true;

  // SUI-025：x-widget 缺省时按类型补缺省控件（不算 hint，不翻转 included）
  if (!field.widget) {
    const typeDefault = typeDefaultWidgetForSchema(schema);
    if (typeDefault) field.widget = typeDefault;
  }
  return { field, included, group };
}

/** 嵌套呈现字段的递归深度上限（防御病态 schema）。 */
const NESTED_FIELD_DEPTH = 8;

/** JSON Pointer 段转义（`~` → `~0`、`/` → `~1`，与 fieldPath 解析同规）。 */
function escapePointerToken(key: string): string {
  return key.replace(/~/g, '~0').replace(/\//g, '~1');
}

/**
 * 收集叶子字段（SUI-019）：
 * - 顶层（`prefix` 为空）产出裸 key，与 Go 生成器 buildFormFields 的历史契约一致；
 * - 嵌套对象子字段与数组项字段以 JSON Pointer 标识（`/address/city`、`/tags/-/label`），
 *   只有携带 hints 的嵌套字段才进入 fields——无 hints 的嵌套结构仍由 rjsf 按
 *   schema 整体渲染（容器保持为整体字段，不静默拆分）。
 */
function collectFields(
  properties: Record<string, JSONValue>,
  out: FieldDerived[],
  prefix = '',
  depth = 0,
): void {
  if (depth > NESTED_FIELD_DEPTH) return;
  for (const [key, child] of Object.entries(properties)) {
    if (!isPlainObject(child)) continue;
    // 顶层用裸 key；其下子层指针从 `/<escaped-key>` 起算
    const identifier = prefix ? `${prefix}/${escapePointerToken(key)}` : key;
    const childPrefix = prefix ? identifier : `/${escapePointerToken(key)}`;
    const derived = deriveLeaf(identifier, child);
    // 嵌套字段仅在携带 hints 时产出；顶层一律保留（保证 ui:order 完整）
    if (!prefix || derived.included) out.push(derived);
    const childProperties = isPlainObject(child['properties']) ? child['properties'] : null;
    if (childProperties) collectFields(childProperties, out, childPrefix, depth + 1);
    const items = child['items'];
    if (isPlainObject(items)) {
      // 数组项：`-` 段指向 items 的子属性（首项语义），渲染侧同解析
      const itemProperties = isPlainObject(items['properties']) ? items['properties'] : null;
      if (itemProperties) collectFields(itemProperties, out, `${childPrefix}/-`, depth + 1);
    }
  }
}

function deriveGroups(root: SchemaNode, ordered: FieldDerived[]): FormGroupSpec[] {
  const declared = new Map<string, FormGroupSpec>();
  if (Array.isArray(root['x-ui-groups'])) {
    for (const entry of root['x-ui-groups']) {
      if (!isPlainObject(entry) || typeof entry.key !== 'string' || !entry.key.trim()) continue;
      const group: FormGroupSpec = { key: entry.key.trim(), fields: [] };
      const title = asLocalizedText(entry.title);
      if (title) group.title = title;
      if (typeof entry.collapsible === 'boolean') group.collapsible = entry.collapsible;
      if (typeof entry.collapsed === 'boolean') group.collapsed = entry.collapsed;
      declared.set(group.key, group);
    }
  }

  const groups: FormGroupSpec[] = [...declared.values()];
  const byKey = new Map(groups.map((group) => [group.key, group]));
  for (const { field, group } of ordered) {
    if (!group) continue;
    let target = byKey.get(group);
    if (!target) {
      // 未声明的分组 key：按字段出现顺序自动补组，title 取 key 人性化。
      // D7：归一为默认语言单 key，不再强制双写（渲染端按回退链取值）。
      const humanized = humanizeFieldKey(group);
      target = {
        key: group,
        title: { 'zh-CN': humanized },
        fields: [],
      };
      byKey.set(group, target);
      groups.push(target);
    }
    target.fields.push(field.key);
  }
  // 无成员的声明组不渲染
  const populated = groups.filter((group) => group.fields.length);
  return populated;
}

/**
 * 从 input_schema 推导 FormPresentationSpec。
 * 无任何 hints 时等价于历史行为：{ jsonSchema, layout: 'vertical' }；
 * 存在任一 hint 时 fields 收录全部顶层字段（保证 ui:order 完整）。
 */
function sortCollectedFields(collected: FieldDerived[]): FieldDerived[] {
  return collected
    .map((item, index) => ({ item, index }))
    .sort((a, b) => {
      const orderA = a.item.field.order ?? Number.MAX_SAFE_INTEGER;
      const orderB = b.item.field.order ?? Number.MAX_SAFE_INTEGER;
      return orderA === orderB ? a.index - b.index : orderA - orderB;
    })
    .map(({ item }) => item);
}

export function derivePresentationSpec(schema?: JSONSchema | null): FormPresentationSpec {
  const root = isPlainObject(schema) ? schema : {};
  const properties = isPlainObject(root.properties) ? root.properties : {};

  const collected: FieldDerived[] = [];
  collectFields(properties, collected);
  const ordered = sortCollectedFields(collected);

  const layout = asLayout(root['x-ui-layout']) ?? 'vertical';
  const groups = deriveGroups(root, ordered);
  const hasHints =
    ordered.some(({ included }) => included) || groups.length > 0 || layout !== 'vertical';

  const spec: FormPresentationSpec = { jsonSchema: root as JSONSchema, layout };
  if (hasHints) {
    spec.fields = ordered.map(({ field }) => field);
    if (groups.length) spec.groups = groups;
  }
  return spec;
}

/**
 * SUI-025：默认派生（无配置时的标准纵向表单）。与 derivePresentationSpec 的
 * 区别仅在 fields 兜底：即使无任何 hints 也产出全部顶层字段（带类型缺省
 * 控件），供渲染端在 spec 未携带 fields 时合成标准表单——与 Go 生成器发布页
 * （deriveFormPresentation + DefaultFormPresentation）表现一致。嵌套字段仍
 * 仅在有 hints 时产出；Go 侧同源契约见 testdata/form_presentation_parity.json。
 */
export function deriveDefaultPresentationSpec(schema?: JSONSchema | null): FormPresentationSpec {
  const spec = derivePresentationSpec(schema);
  const root = isPlainObject(schema) ? schema : {};
  const properties = isPlainObject(root.properties) ? root.properties : {};
  if (spec.fields || !Object.keys(properties).length) return spec;
  const collected: FieldDerived[] = [];
  collectFields(properties, collected);
  spec.fields = sortCollectedFields(collected).map(({ field }) => field);
  return spec;
}
