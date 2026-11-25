/**
 * SUI-021：表单呈现配置的保存/发布前校验。
 *
 * 与 `internal/dashboard/spec/form_validation.go`（`ValidateFormPresentations`）逐项对齐，
 * 在提交草稿前把服务端会拒的配置提前拦下并给出可定位的错误。规则：
 *
 * - 字段 key 非空且不重复；无法在 jsonSchema 中解析的字段报 `form_field_unknown`；
 * - 分组 key 非空且不重复；分组字段必须在 `form.fields` 中声明（未声明或解析不到）
 *   报 `form_group_field_unknown`；同一分组内字段不重复（`form_group_field_duplicate`）、
 *   同一字段不被两个分组认领（`form_group_field_conflict`）；
 * - 控件名必须在受控枚举内（`form_widget_invalid`），且控件接受的 schema 类型与
 *   字段类型匹配（`form_widget_incompatible`）——DateRange/RichText 仅声明未实现，
 *   不做兼容性检查；schema 类型未知/缺省时跳过检查；类型数组按任意匹配；
 * - 字段 width 必须在 0..12（`form_width_invalid`）。
 *
 * SUI-033 命名展示变体（SUI-031 协议）：variant key 非空且唯一
 * （`form_variant_key_missing` / `form_variant_key_conflict`）、defaultVariant 必须
 * 解析到已声明变体（`form_variant_default_unknown`）、变体字段键必须能在
 * jsonSchema 解析（`form_variant_field_unknown`）、展示覆盖遵循与基础字段相同的
 * 控件/宽度规则、调用相关键覆盖非法（`form_variant_illegal_override`，与 Go 端
 * `FormVariantFieldSpec.UnmarshalJSON` 的 UnknownKeys 审计等价——浏览器 JSON
 * 解析不丢键，直接对白名单外键计数）。
 *
 * 未知字段路径由 `fieldPath.ts` 的 `unknownFieldPaths` / `resolveFieldSchema` 统一判定
 * （含分组字段与 JSON Pointer 的 `~0`/`~1` 转义）。仅当 jsonSchema 为带非空
 * `properties` 的对象时才做字段解析；根数组/标量/无 properties 的 schema 跳过，
 * 避免对合法配置误报。
 *
 * 纯函数、无副作用、幂等：仅读取输入，可在保存与发布两条路径复用。
 */
import type {
  FormFieldSpec,
  FormGroupSpec,
  FormPresentationSpec,
  FormVariantSpec,
  FormWidget,
  JSONSchema,
} from '@/types/dashboard';
import { resolveFieldSchema, unknownFieldPaths, type SchemaNode } from './fieldPath';
import { widgetCapability } from './widgetCapabilities';

/** 单条校验问题：field 指向具体条目（如 `operation.form.fields[2].width`）。 */
export interface FormValidationIssue {
  field: string;
  message: string;
  code: string;
}

/** 字段 schema 是否可解析：root 为对象且顶层 properties 非空。 */
function hasObjectProperties(schema: unknown): boolean {
  if (typeof schema !== 'object' || schema === null || Array.isArray(schema)) return false;
  const properties = (schema as SchemaNode)['properties'];
  return (
    typeof properties === 'object' &&
    properties !== null &&
    !Array.isArray(properties) &&
    Object.keys(properties as Record<string, unknown>).length > 0
  );
}

/** 字段 schema 类型（字符串或类型数组）；未知/缺省时 undefined。 */
function schemaTypesOf(node: SchemaNode | null): string[] | undefined {
  if (!node) return undefined;
  const type = node['type'];
  if (typeof type === 'string') return type ? [type] : undefined;
  if (Array.isArray(type)) {
    const list = type.filter((item): item is string => typeof item === 'string');
    return list.length ? list : undefined;
  }
  return undefined;
}

/** 控件与 schema 类型兼容性：类型数组按任意匹配；未知类型或未实现控件跳过。 */
function widgetCompatible(widget: FormWidget, types: string[] | undefined): boolean | undefined {
  if (types === undefined) return undefined;
  const capability = widgetCapability(widget);
  // DateRange/RichText 仅声明未实现，能力表没有对应规则 → 跳过兼容性检查。
  if (!capability) return undefined;
  return types.some((type) => capability.acceptedTypes.includes(type));
}

/** 按「对象且非数组」窄化任意值。 */
function asRecord(value: unknown): Record<string, unknown> | null {
  return typeof value === 'object' && value !== null && !Array.isArray(value)
    ? (value as Record<string, unknown>)
    : null;
}

/** 变体字段允许的展示覆盖键集（与 Go `variantFieldKnownKeys` 对齐，
 *  新增展示覆盖字段时两处同步）。 */
const variantFieldKnownKeys: ReadonlySet<string> = new Set([
  'key',
  'widget',
  'label',
  'placeholder',
  'description',
  'role',
  'unit',
  'perTarget',
  'width',
  'order',
  'enumOptions',
  'remoteOptions',
  'widgetProps',
]);

/** 变体字段上的非法覆盖键（白名单之外的键，排序保证诊断稳定）。 */
export function unknownVariantOverrideKeys(field: unknown): string[] {
  const record = asRecord(field);
  if (!record) return [];
  return Object.keys(record)
    .filter((key) => !variantFieldKnownKeys.has(key))
    .sort();
}

/** 未知字段判定：复用 fieldPath 的 unknownFieldPaths（含分组字段），返回
 *  `key：reason` → reason 的映射，便于按字段定位并原样带上解析失败原因。 */
function unknownReasons(
  fields: Array<{ key: string }>,
  groupFields: string[],
  schema: unknown,
): Map<string, string> {
  const entries = unknownFieldPaths(fields, groupFields, schema);
  const result = new Map<string, string>();
  for (const entry of entries) {
    const at = entry.indexOf('：');
    const key = at < 0 ? entry : entry.slice(0, at);
    const reason = at < 0 ? '' : entry.slice(at + 1);
    result.set(key, reason);
  }
  return result;
}

/** 校验单个 FormPresentationSpec。 */
export function validateFormPresentation(
  spec: FormPresentationSpec,
  fieldPrefix: string,
): FormValidationIssue[] {
  const issues: FormValidationIssue[] = [];
  if (!spec || typeof spec !== 'object') return issues;

  const schema = spec.jsonSchema;
  const resolvable = hasObjectProperties(schema);

  const fields = Array.isArray(spec.fields) ? spec.fields : [];
  const groups = Array.isArray(spec.groups) ? spec.groups : [];
  const groupFields = groups.flatMap((group) =>
    Array.isArray(group?.fields)
      ? group.fields.filter((item): item is string => typeof item === 'string')
      : [],
  );
  // 只跑一次共享诊断（与 renderer 的 SUI-019 告警同源）。
  const unknown = resolvable
    ? unknownReasons(
        fields.map((field) => ({ key: typeof field?.key === 'string' ? field.key : '' })),
        groupFields,
        schema,
      )
    : new Map<string, string>();

  const declaredKeys = new Set<string>();
  fields.forEach((field, index) => {
    const base = `${fieldPrefix}.fields[${index}]`;
    const rawKey = typeof field?.key === 'string' ? field.key : '';
    const key = rawKey.trim();
    if (key === '') {
      issues.push({
        field: `${base}.key`,
        message: 'form field key is required',
        code: 'form_field_key_missing',
      });
      return;
    }
    if (declaredKeys.has(key)) {
      issues.push({
        field: `${base}.key`,
        message: `form field key ${JSON.stringify(key)} is declared more than once`,
        code: 'form_field_duplicate',
      });
    }
    declaredKeys.add(key);

    const reason = unknown.get(rawKey);
    if (reason !== undefined) {
      issues.push({
        field: `${base}.key`,
        message: `form field ${JSON.stringify(rawKey)} cannot be resolved in jsonSchema: ${reason}`,
        code: 'form_field_unknown',
      });
    }

    const widget = field?.widget as FormWidget | string | undefined;
    if (widget !== undefined && widget !== '') {
      if (!widgetCapability(widget)) {
        issues.push({
          field: `${base}.widget`,
          message: `unknown form widget ${JSON.stringify(widget)}`,
          code: 'form_widget_invalid',
        });
      } else if (resolvable && reason === undefined) {
        const target = resolveFieldSchema(schema, rawKey);
        const types = schemaTypesOf(target.schema);
        if (widgetCompatible(widget as FormWidget, types) === false) {
          issues.push({
            field: `${base}.widget`,
            message: `widget ${JSON.stringify(widget)} is not compatible with schema type ${
              types?.join('/') ?? 'unknown'
            }`,
            code: 'form_widget_incompatible',
          });
        }
      }
    }

    const width = field?.width;
    if (typeof width === 'number' && !Number.isNaN(width) && (width < 0 || width > 12)) {
      issues.push({
        field: `${base}.width`,
        message: 'form field width must be 0 (unset) or between 1 and 12',
        code: 'form_width_invalid',
      });
    }
  });

  const seenGroups = new Set<string>();
  const fieldGroups = new Map<string, string>();
  groups.forEach((group, index) => {
    const base = `${fieldPrefix}.groups[${index}]`;
    const groupKey = typeof group?.key === 'string' ? group.key.trim() : '';
    if (groupKey === '') {
      issues.push({
        field: `${base}.key`,
        message: 'form group key is required',
        code: 'form_group_key_missing',
      });
    } else if (seenGroups.has(groupKey)) {
      issues.push({
        field: `${base}.key`,
        message: `form group key ${JSON.stringify(groupKey)} is declared more than once`,
        code: 'form_group_key_duplicate',
      });
    }
    seenGroups.add(groupKey);

    const groupID = groupKey === '' ? `#${index}` : groupKey;
    const rawFields = Array.isArray(group?.fields) ? group.fields : [];
    const seenInGroup = new Set<string>();
    rawFields.forEach((rawKey, itemIndex) => {
      const itemField = `${base}.fields[${itemIndex}]`;
      const key = typeof rawKey === 'string' ? rawKey.trim() : '';
      if (key === '') {
        issues.push({
          field: itemField,
          message: 'form group field key is empty',
          code: 'form_group_field_unknown',
        });
        return;
      }

      let reason = unknown.get(rawKey) ?? '';
      if (!declaredKeys.has(key)) {
        reason = reason ? `${reason}; ` : '';
        reason += 'field is not declared in form.fields';
      }
      if (reason) {
        issues.push({
          field: itemField,
          message: `form group field ${JSON.stringify(rawKey)} is invalid: ${reason}`,
          code: 'form_group_field_unknown',
        });
      }

      if (seenInGroup.has(key)) {
        issues.push({
          field: itemField,
          message: `form group field ${JSON.stringify(rawKey)} is declared more than once in group ${JSON.stringify(
            group?.key ?? '',
          )}`,
          code: 'form_group_field_duplicate',
        });
        return;
      }
      seenInGroup.add(key);

      const owner = fieldGroups.get(key);
      if (owner !== undefined && owner !== groupID) {
        issues.push({
          field: itemField,
          message: `form field ${JSON.stringify(rawKey)} is assigned to both group ${JSON.stringify(
            owner,
          )} and group ${JSON.stringify(group?.key ?? '')}`,
          code: 'form_group_field_conflict',
        });
        return;
      }
      fieldGroups.set(key, groupID);
    });
  });

  // 命名展示变体（SUI-031 协议，SUI-033 校验）——与 Go validateFormPresentation 对齐。
  const variants: FormVariantSpec[] = Array.isArray(spec.variants) ? spec.variants : [];
  const variantKeys = new Set<string>();
  variants.forEach((variant, index) => {
    const base = `${fieldPrefix}.variants[${index}]`;
    const variantRaw = asRecord(variant);
    if (!variantRaw) return;
    const variantKey = typeof variantRaw['key'] === 'string' ? variantRaw['key'].trim() : '';
    if (variantKey === '') {
      issues.push({
        field: `${base}.key`,
        message: 'form variant key is required',
        code: 'form_variant_key_missing',
      });
    } else if (variantKeys.has(variantKey)) {
      issues.push({
        field: `${base}.key`,
        message: `form variant key ${JSON.stringify(variantKey)} is declared more than once`,
        code: 'form_variant_key_conflict',
      });
    }
    if (variantKey !== '') variantKeys.add(variantKey);

    const variantFields = Array.isArray(variantRaw['fields'])
      ? (variantRaw['fields'] as unknown[]).filter(
          (item): item is FormVariantSpec['fields'][number] => typeof item === 'object' && item !== null,
        )
      : [];
    const variantGroupFields = Array.isArray(variantRaw['groups'])
      ? (variantRaw['groups'] as unknown[]).flatMap((group) => {
          const fields = asRecord(group)?.['fields'];
          return Array.isArray(fields)
            ? fields.filter((item): item is string => typeof item === 'string')
            : [];
        })
      : [];
    // 变体字段键的 schema 解析（含变体分组字段），与基础字段共用同一判定。
    const variantUnknown = resolvable
      ? unknownReasons(
          variantFields.map((item) => ({ key: (asRecord(item)?.['key'] as string | undefined) ?? '' })),
          variantGroupFields,
          schema,
        )
      : new Map<string, string>();

    const seenVariantFields = new Set<string>();
    variantFields.forEach((item, itemIndex) => {
      const vbase = `${base}.fields[${itemIndex}]`;
      const illegal = unknownVariantOverrideKeys(item);
      if (illegal.length > 0) {
        issues.push({
          field: `${vbase}.key`,
          message: `form variant field overrides call-affecting keys: ${illegal.join(', ')}`,
          code: 'form_variant_illegal_override',
        });
      }
      const rawKey = (asRecord(item)?.['key'] as string | undefined) ?? '';
      const key = rawKey.trim();
      if (key === '') {
        issues.push({
          field: `${vbase}.key`,
          message: 'form variant field key is required',
          code: 'form_variant_field_key_missing',
        });
        return;
      }
      if (seenVariantFields.has(key)) {
        issues.push({
          field: `${vbase}.key`,
          message: `form variant field key ${JSON.stringify(key)} is declared more than once`,
          code: 'form_variant_field_duplicate',
        });
      }
      seenVariantFields.add(key);

      const reason = variantUnknown.get(rawKey);
      if (reason !== undefined) {
        issues.push({
          field: `${vbase}.key`,
          message: `form variant field ${JSON.stringify(rawKey)} cannot be resolved in jsonSchema: ${reason}`,
          code: 'form_variant_field_unknown',
        });
      }

      const widget = asRecord(item)?.['widget'] as FormWidget | string | undefined;
      if (widget !== undefined && widget !== '') {
        if (!widgetCapability(widget)) {
          issues.push({
            field: `${vbase}.widget`,
            message: `unknown form widget ${JSON.stringify(widget)}`,
            code: 'form_widget_invalid',
          });
        } else if (resolvable && reason === undefined) {
          const target = resolveFieldSchema(schema, rawKey);
          const types = schemaTypesOf(target.schema);
          if (widgetCompatible(widget as FormWidget, types) === false) {
            issues.push({
              field: `${vbase}.widget`,
              message: `widget ${JSON.stringify(widget)} is not compatible with schema type ${
                types?.join('/') ?? 'unknown'
              }`,
              code: 'form_widget_incompatible',
            });
          }
        }
      }

      const width = asRecord(item)?.['width'];
      if (typeof width === 'number' && !Number.isNaN(width) && (width < 0 || width > 12)) {
        issues.push({
          field: `${vbase}.width`,
          message: 'form field width must be 0 (unset) or between 1 and 12',
          code: 'form_width_invalid',
        });
      }
    });

    const variantGroups = Array.isArray(variantRaw['groups'])
      ? (variantRaw['groups'] as unknown[]).filter(
          (item): item is FormGroupSpec => typeof item === 'object' && item !== null,
        )
      : [];
    const seenVariantGroups = new Set<string>();
    const variantFieldGroups = new Map<string, string>();
    variantGroups.forEach((group, groupIndex) => {
      const gbase = `${base}.groups[${groupIndex}]`;
      const groupKey = typeof group?.key === 'string' ? group.key.trim() : '';
      if (groupKey === '') {
        issues.push({
          field: `${gbase}.key`,
          message: 'form variant group key is required',
          code: 'form_group_key_missing',
        });
      } else if (seenVariantGroups.has(groupKey)) {
        issues.push({
          field: `${gbase}.key`,
          message: `form variant group key ${JSON.stringify(groupKey)} is declared more than once`,
          code: 'form_group_key_duplicate',
        });
      }
      seenVariantGroups.add(groupKey);

      const variantGroupID = groupKey === '' ? `#${groupIndex}` : groupKey;
      const rawFields = Array.isArray(group?.fields) ? group.fields : [];
      const seenInVariantGroup = new Set<string>();
      rawFields.forEach((rawKey, itemIndex) => {
        const itemField = `${gbase}.fields[${itemIndex}]`;
        const key = typeof rawKey === 'string' ? rawKey.trim() : '';
        if (key === '') {
          issues.push({
            field: itemField,
            message: 'form variant group field key is empty',
            code: 'form_group_field_unknown',
          });
          return;
        }

        // 变体分组的字段引用目标：jsonSchema 可解析，且字段已在基础
        // fields 或本变体 fields 声明（渲染按合并后的字段集）。
        let reason = variantUnknown.get(rawKey) ?? '';
        if (!declaredKeys.has(key) && !seenVariantFields.has(key)) {
          reason = reason ? `${reason}; ` : '';
          reason += 'field is not declared in form.fields or variant fields';
        }
        if (reason) {
          issues.push({
            field: itemField,
            message: `form variant group field ${JSON.stringify(rawKey)} is invalid: ${reason}`,
            code: 'form_group_field_unknown',
          });
        }

        if (seenInVariantGroup.has(key)) {
          issues.push({
            field: itemField,
            message: `form group field ${JSON.stringify(rawKey)} is declared more than once in group ${JSON.stringify(
              group?.key ?? '',
            )}`,
            code: 'form_group_field_duplicate',
          });
          return;
        }
        seenInVariantGroup.add(key);

        const owner = variantFieldGroups.get(key);
        if (owner !== undefined && owner !== variantGroupID) {
          issues.push({
            field: itemField,
            message: `form field ${JSON.stringify(rawKey)} is assigned to both group ${JSON.stringify(
              owner,
            )} and group ${JSON.stringify(group?.key ?? '')}`,
            code: 'form_group_field_conflict',
          });
          return;
        }
        variantFieldGroups.set(key, variantGroupID);
      });
    });
  });

  const defaultVariant = typeof spec.defaultVariant === 'string' ? spec.defaultVariant.trim() : '';
  if (defaultVariant !== '' && !variantKeys.has(defaultVariant)) {
    issues.push({
      field: `${fieldPrefix}.defaultVariant`,
      message: `form defaultVariant ${JSON.stringify(spec.defaultVariant)} does not match any declared variant`,
      code: 'form_variant_default_unknown',
    });
  }

  return issues;
}

// ---------------------------------------------------------------------------
// 遍历整页 PageSpec 的所有 FormPresentationSpec（与 Go ValidateFormPresentations 对齐）
// ---------------------------------------------------------------------------

/** 把未知形态的表单对象归一成 FormPresentationSpec 后校验。 */
function validateLooseForm(value: unknown, fieldPrefix: string, out: FormValidationIssue[]): void {
  const form = asRecord(value);
  if (!form) return;
  const rawSchema = form['jsonSchema'];
  const rawFields = form['fields'];
  const rawGroups = form['groups'];
  const rawVariants = form['variants'];
  const defaultVariant = form['defaultVariant'];
  const spec: FormPresentationSpec = {
    jsonSchema: (asRecord(rawSchema) as JSONSchema | null) ?? {},
    fields: Array.isArray(rawFields)
      ? rawFields.filter((item): item is FormFieldSpec => typeof item === 'object' && item !== null)
      : [],
    groups: Array.isArray(rawGroups)
      ? rawGroups.filter((item): item is FormGroupSpec => typeof item === 'object' && item !== null)
      : [],
    variants: Array.isArray(rawVariants)
      ? rawVariants.filter((item): item is FormVariantSpec => typeof item === 'object' && item !== null)
      : [],
    defaultVariant: typeof defaultVariant === 'string' ? defaultVariant : undefined,
  };
  out.push(...validateFormPresentation(spec, fieldPrefix));
}

/** 遍历一个 PageSpec-like 对象的所有表单位置（resource/operation/task/report/composite）。 */
export function collectFormValidationIssues(page: unknown): FormValidationIssue[] {
  const issues: FormValidationIssue[] = [];
  const root = asRecord(page);
  if (!root) return issues;

  const resource = asRecord(root['resource']);
  if (resource) {
    validateLooseForm(resource['createForm'], 'resource.createForm', issues);
    validateLooseForm(resource['updateForm'], 'resource.updateForm', issues);
    const actions = resource['actions'];
    if (Array.isArray(actions)) {
      actions.forEach((action, index) => {
        validateLooseForm(asRecord(action)?.['form'], `resource.actions[${index}].form`, issues);
      });
    }
  }

  validateLooseForm(asRecord(root['operation'])?.['form'], 'operation.form', issues);
  validateLooseForm(asRecord(root['task'])?.['form'], 'task.form', issues);
  validateLooseForm(asRecord(root['report'])?.['queryForm'], 'report.queryForm', issues);

  const composite = asRecord(root['composite']);
  if (composite) {
    const sections = composite['sections'];
    if (Array.isArray(sections)) {
      sections.forEach((section, index) => {
        validateLooseForm(asRecord(section)?.['form'], `composite.sections[${index}].form`, issues);
      });
    }
  }

  return issues;
}
