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
  const spec: FormPresentationSpec = {
    jsonSchema: (asRecord(rawSchema) as JSONSchema | null) ?? {},
    fields: Array.isArray(rawFields)
      ? rawFields.filter((item): item is FormFieldSpec => typeof item === 'object' && item !== null)
      : [],
    groups: Array.isArray(rawGroups)
      ? rawGroups.filter((item): item is FormGroupSpec => typeof item === 'object' && item !== null)
      : [],
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
