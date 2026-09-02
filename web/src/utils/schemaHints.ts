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
  if (typeof value === 'string') return normalizeLocalizedText(value);
  if (isPlainObject(value)) return normalizeLocalizedText(value as Record<string, string>);
  return undefined;
}

function asWidget(value: unknown): FormWidget | undefined {
  return typeof value === 'string' && (FORM_WIDGETS as readonly string[]).includes(value)
    ? (value as FormWidget)
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
  // x-options-source：保留字段（todo.md F9），消费实现落地前忽略

  const group = asGroupKey(schema['x-group']);
  if (group) included = true;
  return { field, included, group };
}

/** 收集顶层叶子字段（嵌套容器保留为整体字段，点路径展开待渲染器支持后启用）。 */
function collectFields(properties: Record<string, JSONValue>, out: FieldDerived[]): void {
  for (const [key, child] of Object.entries(properties)) {
    if (!isPlainObject(child)) continue;
    out.push(deriveLeaf(key, child));
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
      // 未声明的分组 key：按字段出现顺序自动补组，title 取 key 人性化
      const humanized = humanizeFieldKey(group);
      target = {
        key: group,
        title: { 'zh-CN': humanized, 'en-US': humanized },
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
export function derivePresentationSpec(schema?: JSONSchema | null): FormPresentationSpec {
  const root = isPlainObject(schema) ? schema : {};
  const properties = isPlainObject(root.properties) ? root.properties : {};

  const collected: FieldDerived[] = [];
  collectFields(properties, collected);
  const ordered = collected
    .map((item, index) => ({ item, index }))
    .sort((a, b) => {
      const orderA = a.item.field.order ?? Number.MAX_SAFE_INTEGER;
      const orderB = b.item.field.order ?? Number.MAX_SAFE_INTEGER;
      return orderA === orderB ? a.index - b.index : orderA - orderB;
    })
    .map(({ item }) => item);

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
