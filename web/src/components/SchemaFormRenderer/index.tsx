/**
 * JSON Schema form runtime adapter.
 *
 * The public contract is FormPresentationSpec. RJSF uiSchema is derived in
 * memory only and must not be persisted into SDK/OpenAPI/PageSpec snapshots.
 */

import { Segmented, Select } from 'antd';
import React, {
  useCallback,
  forwardRef,
  useEffect,
  useImperativeHandle,
  useMemo,
  useRef,
  useState,
} from 'react';
import Form from '@rjsf/antd';
import type CoreForm from '@rjsf/core';
import type { IChangeEvent } from '@rjsf/core';
import validator from '@rjsf/validator-ajv8';
import { getIntl } from '@umijs/max';
import type { ErrorSchema, RJSFValidationError, RJSFSchema, UiSchema } from '@rjsf/utils';
import type {
  FormFieldSpec,
  FormPresentationSpec,
  FormValues,
  FormWidget,
  JSONSchema,
  JSONValue,
} from '@/types/dashboard';
import { currentUiLocale } from '@/utils/uiLocale';
import { localizedText } from '@/utils/localizedText';
import { humanizeFieldKey } from '@/utils/humanize';
import { deriveDefaultPresentationSpec } from '@/utils/schemaHints';
import { resolveVariantPresentation } from '@/utils/formVariants';
import { primarySchemaType } from '@/utils/schemaDialect';
import { matchesCondition } from '@/components/PageRenderer/sectionCondition';
import {
  deleteAtPath,
  isPointerFieldKey,
  resolveFieldSchema,
  setAtPath,
  toggleRequiredAt,
  uiSchemaNodeAt,
  unknownFieldPaths,
  valueAtFieldPath,
} from '@/utils/fieldPath';
import { customWidgets } from './widgets';
import { uploadFields, uploadWidgets } from './widgets-upload';
import { customTemplates } from './templates';
import {
  mergeBranchIntoSchema,
  pruneValuesToBranch,
  resolveDiscriminatorBranches,
} from './branchSelection';
import type { DiscriminatorBranch, DiscriminatorResolution } from './branchSelection';
import SchemaFormErrorBoundary from './ErrorBoundary';
import JsonRootEditor, { isJsonRootSpec } from './JsonRootEditor';

export interface SchemaFormRendererHandle {
  submit: () => void;
  validate: () => boolean;
  /** 实时表单值（含隐藏字段）——仅用于状态镜像/回显，禁止直接作提交取值 */
  getValues: () => FormValues;
  /** SUI-014 统一提交取值：与原生提交（onFinish）共用同一投影规则
   *  （projectSubmitValues，隐藏键按当前值剔除）。调试页、资源创建/编辑、
   *  行操作等命令式提交一律取此接口，禁止用 getValues 拼提交 payload。 */
  getSubmitValues: () => FormValues;
}

export interface SchemaFormRendererProps {
  spec: FormPresentationSpec;
  initialValues?: FormValues;
  onFinish?: (values: FormValues) => Promise<boolean | void> | boolean | void;
  onValuesChange?: (changedValues: FormValues, allValues: FormValues) => void;
  readonly?: boolean;
  disabled?: boolean;
  hideSubmit?: boolean;
  /** RJSF 容器元素标签：默认 'form'。嵌入 ProForm（ModalForm/DrawerForm）
   * 时传 'div' 避免嵌套 <form>，仍由同一 RJSF 引擎收集/校验动态参数。 */
  containerTagName?: 'form' | 'div';
  /** D4：外部字段级错误（服务端参数校验的 details），按字段名注入到控件下方
   * （RJSF extraErrors），与本地校验并存、不替代本地校验。键为 schema 顶层
   * 属性名（嵌套属性请按 RJSF ErrorSchema 嵌套传入）。省略或空对象时不注入。 */
  externalErrors?: Record<string, string[]>;
}

type JsonObject = Record<string, JSONValue>;
type RJSFFormRef = CoreForm<FormValues, RJSFSchema, Record<string, never>>;

function isObject(value: unknown): value is JsonObject {
  return !!value && typeof value === 'object' && !Array.isArray(value);
}

function normalizeJsonValue(value: unknown): JSONValue {
  if (value === null) return null;
  if (typeof value === 'string' || typeof value === 'number' || typeof value === 'boolean') {
    return value;
  }
  if (Array.isArray(value)) {
    return value.map((item) => normalizeJsonValue(item));
  }
  if (isObject(value)) {
    return Object.fromEntries(
      Object.entries(value)
        .filter(([, item]) => item !== undefined)
        .map(([key, item]) => [key, normalizeJsonValue(item)]),
    );
  }
  return null;
}

function normalizeFormValues(value: unknown): FormValues {
  if (!isObject(value)) return {};
  // SUI-022：undefined 值键直接剔除，不折成 null。rjsf v6 清空 leaf 后键以
  // undefined 滞留在 state.formData（AJV 校验时被 JSON 序列化剥掉，但这里的
  // 投影若折成 null，null 会复活进提交 payload 并撞服务端 type 校验）。
  return Object.fromEntries(
    Object.entries(value)
      .filter(([, item]) => item !== undefined)
      .map(([key, item]) => [key, normalizeJsonValue(item)]),
  );
}

/** 按字段标识取字段 schema：裸顶层 key（旧契约）与 JSON Pointer 同归 resolveFieldSchema。 */
function getFieldSchema(schema: JSONSchema, key: string): JSONSchema {
  return (resolveFieldSchema(schema, key).schema ?? {}) as JSONSchema;
}

/** SUI-023：按枚举值关联可见标签——enumOptions 声明顺序与 schema.enum 无关，
 *  以 String(value) 归一做键匹配；匹配不上的枚举值回退原始值字符串（不伪造标签）。
 *  无 schema.enum 时按声明顺序兜底（旧契约，等价旧行为）。 */
function getEnumNames(
  field: FormFieldSpec | undefined,
  schemaEnum?: unknown[],
): string[] | undefined {
  if (!field?.enumOptions?.length) return undefined;
  const labelByValue = new Map<string, string>();
  for (const option of field.enumOptions) {
    labelByValue.set(
      String(option.value),
      localizedText(option.label, currentLocale(), String(option.value)),
    );
  }
  if (!schemaEnum?.length) {
    return field.enumOptions.map(
      (option) => labelByValue.get(String(option.value)) ?? String(option.value),
    );
  }
  return schemaEnum.map((value) => labelByValue.get(String(value)) ?? String(value));
}

/** schema 级 x-enum-options → 与 schema.enum 对齐的本地化标签数组（缺项回退原始值）。
 *  SUI-023：按枚举值关联标签——数值/布尔（含 false）值与字符串同权，String 归一
 *  做键；hints 排列顺序与 enum 无关。rjsf optionsList 按 enum 下标取 enumNames，
 *  输出表必须与值域同序同长。 */
function asEnumNamesFromSchemaHints(child: Record<string, unknown>): string[] | undefined {
  const values = Array.isArray(child.enum) ? child.enum : [];
  if (!values.length) return undefined;
  const hints = Array.isArray(child['x-enum-options']) ? child['x-enum-options'] : [];
  if (!hints.length) return undefined;
  const labelByValue = new Map<string, string>();
  for (const item of hints) {
    if (!item || typeof item !== 'object') continue;
    const entry = item as Record<string, unknown>;
    if (!['string', 'number', 'boolean'].includes(typeof entry.value)) continue;
    const label = localizedText(
      entry.label as string | Record<string, string>,
      currentLocale(),
      String(entry.value),
    );
    if (label) labelByValue.set(String(entry.value), label);
  }
  if (!labelByValue.size) return undefined;
  return values.map((value) => labelByValue.get(String(value)) ?? String(value));
}

function shouldUseTextarea(schema: JSONSchema): boolean {
  return schema.format === 'textarea' || Number(schema.maxLength || 0) > 120;
}

// ---------------------------------------------------------------------------
// AJV 校验错误本地化（F6）：跟随平台 locale，title 用派生后 schema 的
// 人工标题（x-label > schema.title > humanize，见 F5）。
// ---------------------------------------------------------------------------

const ERROR_TEMPLATES: Record<string, { zh: string; en: string }> = {
  required: { zh: '「{title}」为必填项', en: '"{title}" is required' },
  minLength: { zh: '至少需要 {limit} 个字符', en: 'must be at least {limit} characters' },
  maxLength: { zh: '最多允许 {limit} 个字符', en: 'must be at most {limit} characters' },
  minimum: { zh: '不能小于 {limit}', en: 'must be greater than or equal to {limit}' },
  maximum: { zh: '不能大于 {limit}', en: 'must be less than or equal to {limit}' },
  minItems: { zh: '至少需要 {limit} 项', en: 'must have at least {limit} items' },
  maxItems: { zh: '最多允许 {limit} 项', en: 'must have at most {limit} items' },
  pattern: { zh: '格式不正确', en: 'does not match the expected pattern' },
  format: { zh: '格式不正确（{format}）', en: 'invalid format ({format})' },
  type: { zh: '类型应为 {type}', en: 'must be of type {type}' },
  enum: { zh: '可选值：{values}', en: 'allowed values: {values}' },
  oneOf: { zh: '不满足任一允许的组合', en: 'does not match any of the allowed variants' },
  anyOf: { zh: '不满足任一允许的组合', en: 'does not match any of the allowed variants' },
  const: { zh: '必须为 {allowedValue}', en: 'must be equal to {allowedValue}' },
};

function resolveFieldTitle(error: RJSFValidationError, schema: RJSFSchema): string {
  const rawPath = String(error.property ?? '')
    .replace(/^\./, '')
    .replace(/['\[\]]/g, '');
  const segments =
    error.name === 'required'
      ? [...rawPath.split('.').filter(Boolean), String(error.params?.missingProperty ?? '')]
      : rawPath.split('.').filter(Boolean);
  const leafKey = segments[segments.length - 1] ?? '';
  // 沿 property 路径下钻，取路径上最后一个人工 title
  let title = '';
  let node: RJSFSchema = schema;
  for (const segment of segments) {
    if (!segment) continue;
    const props = node.properties as Record<string, RJSFSchema> | undefined;
    const child = props?.[segment];
    if (!child) break;
    if (typeof child.title === 'string' && child.title) title = child.title;
    node = child;
  }
  return title || leafKey || '该字段';
}

/** 平台当前 locale；统一走 @/utils/uiLocale（SUI-024，renderer 内外同一入口）。 */
function currentLocale(): string {
  return currentUiLocale();
}

/** 读取 Form 实时值：优先 rjsf class 内部 state（规避延迟 onChange 回调竞态），回退镜像。 */
function getLiveFormValues(
  form: CoreForm<FormValues, RJSFSchema, Record<string, never>> | null,
  fallback: FormValues,
): FormValues {
  const state = (form as unknown as { state?: { formData?: FormValues } } | null)?.state?.formData;
  return (state !== undefined ? normalizeFormValues(state) : fallback) as FormValues;
}

/** AJV 错误 → 平台语言消息；未收录关键字保留原始 message。 */
export function localizeFormErrors(
  errors: RJSFValidationError[],
  schema: RJSFSchema,
  locale: string,
): RJSFValidationError[] {
  const isZh = !locale.toLowerCase().startsWith('en');
  return errors.map((error) => {
    const template = ERROR_TEMPLATES[error.name ?? ''];
    if (!template) return error;
    let text = isZh ? template.zh : template.en;
    // minLength:1（必填字符串=非空的派生约束）报「不能为空」而非「至少 1 个字符」
    if (error.name === 'minLength' && error.params?.limit === 1) {
      text = isZh ? '「{title}」不能为空' : '"{title}" cannot be empty';
    }
    text = text
      .replaceAll('{title}', resolveFieldTitle(error, schema))
      .replaceAll('{limit}', String(error.params?.limit ?? ''))
      .replaceAll('{type}', String(error.params?.type ?? ''))
      .replaceAll('{format}', String(error.params?.format ?? ''))
      .replaceAll('{allowedValue}', String(error.params?.allowedValue ?? ''));
    if (error.name === 'enum' && Array.isArray(error.params?.allowedValues)) {
      text = text.replaceAll('{values}', error.params.allowedValues.map(String).join('、'));
    }
    return { ...error, message: text };
  });
}

function widgetToRjsf(widget?: FormWidget, schema?: JSONSchema): string | undefined {
  if (!widget && schema && shouldUseTextarea(schema)) return 'textarea';
  switch (widget) {
    case 'TextArea':
    case 'Code':
    case 'JSON':
      return 'textarea';
    case 'Password':
      return 'password';
    case 'Radio':
      return 'radio';
    case 'Checkbox':
      return 'checkbox';
    case 'Switch':
      return 'checkbox';
    case 'DatePicker':
      // format:date-time 解析到 date-time 覆盖控件（DateTimeWidget，带显式
      // 偏移序列化）；format:date 解析 date。类型缺省派生（SUI-025）把两种
      // format 都派生成 DatePicker hint，靠这里按 schema format 分派。
      return schema?.format === 'date-time' ? 'date-time' : 'date';
    case 'TimePicker':
      return 'time';
    case 'Color':
      return 'color';
    case 'Slider':
      return 'range';
    case 'Select':
      return 'select';
    case 'TreeSelect':
      return 'treeSelect';
    case 'Cascader':
      return 'cascader';
    case 'Rate':
      return 'rate';
    case 'Upload':
    case 'ImageUpload':
    case 'FileUpload':
      return 'upload';
    case 'KeyValue':
      // object 类型走 ObjectField（忽略 ui:widget），KeyValue 以 ui:field 接入
      return undefined;
    case 'MultiSelect':
    case 'Input':
    case 'InputNumber':
    case 'DateRange':
    case 'RichText':
    case 'Array':
    case 'Object':
    default:
      return undefined;
  }
}

function applyFieldPresentation(
  jsonSchema: RJSFSchema,
  uiSchema: UiSchema,
  field: FormFieldSpec,
  rootSchema: JSONSchema,
): boolean {
  const target = resolveFieldSchema(jsonSchema, field.key);
  const fieldSchema = getFieldSchema(rootSchema, field.key);
  // SUI-019：指针标识的 ui 元数据注入嵌套分支（数组项走 '-'），裸 key 行为不变
  const nextUi = uiSchemaNodeAt(uiSchema as Record<string, unknown>, target.uiTokens);
  const widget = widgetToRjsf(field.widget, fieldSchema);

  if (field.label && target.container && target.property) {
    target.container[target.property] = {
      ...(target.container[target.property] as Record<string, unknown>),
      title: localizedText(field.label, currentLocale(), field.key),
    };
  }
  if (field.description && target.container && target.property) {
    target.container[target.property] = {
      ...(target.container[target.property] as Record<string, unknown>),
      description: localizedText(field.description, currentLocale(), ''),
    };
  }
  if (field.enumOptions?.length && target.container && target.property) {
    const names = getEnumNames(
      field,
      Array.isArray(fieldSchema.enum) ? fieldSchema.enum : undefined,
    );
    target.container[target.property] = {
      ...(target.container[target.property] as Record<string, unknown>),
      enumNames: names,
    };
    // rjsf v5 的 optionsList 只从 ui:options.enumNames 取下拉标签；只写 schema 级
    // enumNames 时下拉会露原始枚举值（GUX-017 实测发现）。schema 级保留给
    // CompositeEditor 等自行消费 schema 的面板读取。
    nextUi['ui:options'] = {
      ...(nextUi['ui:options'] || {}),
      enumNames: names,
    };
  }
  if (field.disabled) nextUi['ui:disabled'] = true;
  if (field.placeholder)
    nextUi['ui:placeholder'] = localizedText(field.placeholder, currentLocale(), '');
  if (widget) nextUi['ui:widget'] = widget;
  if (field.widget === 'KeyValue') nextUi['ui:field'] = 'keyValue';
  if (field.widget === 'MultiSelect') {
    // F3：多选 = 内置 select + multiple；enum 由 schema enum/enumNames 提供
    nextUi['ui:widget'] = 'select';
    nextUi['ui:options'] = { ...(nextUi['ui:options'] || {}), multiple: true };
  }
  if (field.remoteOptions) {
    // F9：远程选项源随 ui:options 进入 widget（Select/TreeSelect 消费，
    // widget 名仍由 x-widget 决定，不在此强制）
    nextUi['ui:options'] = {
      ...(nextUi['ui:options'] || {}),
      remoteOptions: field.remoteOptions,
    };
  }
  if (field.widgetProps)
    nextUi['ui:options'] = { ...(nextUi['ui:options'] || {}), ...field.widgetProps };
  mergeFieldValidity(jsonSchema, field);
  return field.visible !== false;
}

// FormFieldSpec.required / validationRules 生效（3.1 修复）：编辑器可发布
// 这些字段（versioning manual-merge），此前渲染器静默丢弃 = 配置假成功。
// min/max 按字段类型映射到 ajv 关键字（array→Items、number/integer→imum、
// 其余→Length）；pattern 直接进 schema.pattern；custom 无法表达为 JSON
// Schema，显式告警而非静默吞掉。required:false 可撤销 schema 原有必填。
// 指针标识（SUI-019）下 required/校验规则落在字段所属宿主 schema 上，
// 与 resolveFieldSchema 同解析路径（裸 key 行为不变）。
function mergeFieldValidity(schema: RJSFSchema, field: FormFieldSpec): void {
  const target = resolveFieldSchema(schema, field.key);
  const child = target.schema;
  const required: string[] = Array.isArray(target.parent?.['required'])
    ? [...(target.parent['required'] as string[])]
    : [];
  const requiredKey = target.property ?? field.key;
  let changed = false;
  const addRequired = () => {
    if (!required.includes(requiredKey)) {
      required.push(requiredKey);
      changed = true;
    }
  };
  if (field.required === true) addRequired();
  if (field.required === false) {
    const index = required.indexOf(requiredKey);
    if (index >= 0) {
      required.splice(index, 1);
      changed = true;
    }
  }
  if (child) {
    for (const rule of field.validationRules ?? []) {
      switch (rule.type) {
        case 'required':
          addRequired();
          break;
        case 'min':
        case 'max': {
          if (typeof rule.value !== 'number') break;
          // SUI-009：type 数组（可空/联合）以主类型判定控件语义。
          const childType = primarySchemaType(child);
          const isArray = childType === 'array';
          const isNumber = childType === 'number' || childType === 'integer';
          if (rule.type === 'min') {
            if (isArray) child['minItems'] = rule.value;
            else if (isNumber) child['minimum'] = rule.value;
            else child['minLength'] = rule.value;
          } else {
            if (isArray) child['maxItems'] = rule.value;
            else if (isNumber) child['maximum'] = rule.value;
            else child['maxLength'] = rule.value;
          }
          changed = true;
          break;
        }
        case 'pattern':
          if (typeof rule.value === 'string') {
            child['pattern'] = rule.value;
            changed = true;
          }
          break;
        case 'custom':
          // eslint-disable-next-line no-console
          console.warn(
            `[SchemaFormRenderer] field "${field.key}" validation rule "custom" is not supported and ignored`,
          );
          break;
      }
    }
  }
  if (changed && target.parent) {
    target.parent['required'] = required;
  }
}

/** spec.fields[].defaultValue 种入表单初值（仅补未提供值的字段；
 *  显式传入的 initialValues 优先，空串/null 视为已提供的值不覆盖）。
 *  指针标识（SUI-019）按路径定位/写入，穿过数组的路径不写（数组整体提交）。 */
export function applySpecDefaults(spec: FormPresentationSpec, values: FormValues): FormValues {
  const fields = spec.fields ?? [];
  if (!fields.some((field) => field.defaultValue !== undefined)) return values;
  let next = values;
  for (const field of fields) {
    if (field.defaultValue === undefined) continue;
    if (isPointerFieldKey(field.key)) {
      if (valueAtFieldPath(next, field.key) === undefined) {
        next = setAtPath(next, field.key, field.defaultValue) as unknown as FormValues;
      }
      continue;
    }
    if (next[field.key] === undefined) {
      next = { ...next, [field.key]: field.defaultValue };
    }
  }
  return next;
}

function cloneSchema(schema: JSONSchema): RJSFSchema {
  return JSON.parse(JSON.stringify(schema || {})) as RJSFSchema;
}

/** 统一可见性谓词（SUI-014）：`visible === false` 或 visibleWhen 不满足即隐藏。
 *  渲染（deriveRuntimeSchema）与提交投影（projectSubmitValues）共用同一判定，
 *  保证「看到的」与「提交的」对同一份值恒等，两侧永不漂移。 */
function isFieldHidden(field: FormFieldSpec, values: FormValues): boolean {
  return field.visible === false || !matchesCondition(field.visibleWhen, values);
}

/** 当前值下的隐藏字段 key 列表（与 deriveRuntimeSchema 的 hiddenKeys 同源）。 */
export function computeHiddenKeys(spec: FormPresentationSpec, values: FormValues): string[] {
  return (spec.fields ?? [])
    .filter((field) => isFieldHidden(field, values))
    .map((field) => field.key);
}

/** SUI-014 唯一提交投影规则：按当前值计算隐藏键并从 payload 剔除，其余键
 *  原样保留（隐藏字段值继续留在表单态，可见性切回时恢复）。原生提交
 *  （onSubmit）与命令式 getSubmitValues() 共用，提交 payload 只经此生成。
 *  指针标识（SUI-019）按路径剔除（deleteAtPath），穿越数组的路径不裁——
 *  数组按整体提交/裁剪。裸 key 行为不变。 */
export function projectSubmitValues(values: FormValues, spec: FormPresentationSpec): FormValues {
  const hidden = new Set(computeHiddenKeys(spec, values));
  if (hidden.size === 0) return values;
  const pointerKeys = [...hidden].filter(isPointerFieldKey);
  const bareKeys = [...hidden].filter((key) => !isPointerFieldKey(key));
  if (pointerKeys.length === 0) {
    return Object.fromEntries(
      Object.entries(values).filter(([key]) => !hidden.has(key)),
    ) as FormValues;
  }
  let next: FormValues = bareKeys.length
    ? (Object.fromEntries(
        Object.entries(values).filter(([key]) => !bareKeys.includes(key)),
      ) as FormValues)
    : values;
  for (const key of pointerKeys) {
    next = deleteAtPath(next, key) as unknown as FormValues;
  }
  return next;
}

/** 必填字符串补 minLength:1（递归：嵌套对象与数组 items 同样生效）。
 *  SUI-009：可空字符串（type:["string","null"]）主类型仍是 string，
 *  必填非空规则照常（null 是显式值，不豁免非空必填）。
 *  只沿 schema 关键字下钻（properties/items/definitions 等），不误入
 *  `default`/`examples` 里形似 schema 的数据对象。 */
function enforceRequiredStringMinLength(node: Record<string, unknown>): void {
  const properties = node['properties'];
  const required = node['required'];
  if (isObject(properties) && Array.isArray(required)) {
    for (const key of required) {
      if (typeof key !== 'string') continue;
      const child = properties[key];
      if (
        isObject(child) &&
        primarySchemaType(child as JSONSchema) === 'string' &&
        child['minLength'] === undefined
      ) {
        child['minLength'] = 1;
      }
    }
  }
  if (isObject(properties)) {
    for (const child of Object.values(properties)) {
      if (isObject(child)) enforceRequiredStringMinLength(child as Record<string, unknown>);
    }
  }
  for (const keyword of ['items', 'additionalProperties'] as const) {
    const child = node[keyword];
    if (isObject(child)) {
      enforceRequiredStringMinLength(child as Record<string, unknown>);
    } else if (Array.isArray(child)) {
      for (const item of child) {
        if (isObject(item)) enforceRequiredStringMinLength(item as Record<string, unknown>);
      }
    }
  }
  for (const keyword of ['definitions', '$defs'] as const) {
    const defs = node[keyword];
    if (!isObject(defs)) continue;
    for (const child of Object.values(defs)) {
      if (isObject(child)) enforceRequiredStringMinLength(child as Record<string, unknown>);
    }
  }
  for (const keyword of ['allOf', 'anyOf', 'oneOf'] as const) {
    const list = node[keyword];
    if (!Array.isArray(list)) continue;
    for (const item of list) {
      if (isObject(item)) enforceRequiredStringMinLength(item as Record<string, unknown>);
    }
  }
}

export function deriveRuntimeSchema(
  spec: FormPresentationSpec,
  values: FormValues,
): {
  schema: RJSFSchema;
  uiSchema: UiSchema;
  formContext: Record<string, unknown>;
  /** F8：当前因 visibleWhen 隐藏的字段 key（值保留在表单中，提交时剔除） */
  hiddenKeys: string[];
} {
  const schema = cloneSchema(spec.jsonSchema);
  const uiSchema: UiSchema = {
    'ui:submitButtonOptions': {
      // 纯函数无法 useIntl：getIntl 调用点求值（测试 mock 返回 defaultMessage）
      submitText: localizedText(
        spec.submitButton?.text,
        currentLocale(),
        getIntl().formatMessage({
          id: 'component.schemaFormRenderer.submitFallback',
          defaultMessage: '提交',
        }),
      ),
      norender: false,
    },
  };

  // 从 spec.layout 推导 formContext 布局配置
  const formContext: Record<string, unknown> = {};
  switch (spec.layout) {
    case 'horizontal':
      formContext.labelCol = { span: 6 };
      formContext.wrapperCol = { span: 18 };
      formContext.labelAlign = 'right';
      break;
    case 'inline':
      formContext.labelCol = { flex: '80px' };
      formContext.wrapperCol = { flex: 'auto' };
      formContext.labelAlign = 'right';
      break;
    case 'grid':
      formContext.colSpan = 12;
      formContext.rowGutter = 16;
      break;
    case 'vertical':
    default:
      // vertical 是默认布局，不需要额外配置
      break;
  }

  // F7：分组与字段宽度元数据注入 formContext，供 GroupedObjectFieldTemplate 消费
  if (spec.groups?.length) {
    formContext.__groups = spec.groups;
    formContext.__fieldGroups = Object.fromEntries(
      spec.groups.flatMap((group) => group.fields.map((fieldKey) => [fieldKey, group.key])),
    );
  }
  const fieldWidths: Record<string, number> = {};
  for (const field of spec.fields || []) {
    if (typeof field.width === 'number') fieldWidths[field.key] = field.width;
  }
  if (Object.keys(fieldWidths).length) formContext.__fieldWidths = fieldWidths;

  if (!schema.type) schema.type = 'object';
  if (!schema.properties) schema.properties = {};

  const rootSchema = spec.jsonSchema || {};
  const hiddenFields = new Set<string>();
  for (const field of spec.fields || []) {
    // 呈现副作用对所有字段生效（隐藏字段仍需 ui 元数据），可见性判定与
    // computeHiddenKeys/projectSubmitValues 走同一谓词（SUI-014）
    applyFieldPresentation(schema, uiSchema, field, rootSchema);
    if (isFieldHidden(field, values)) hiddenFields.add(field.key);
  }

  // F8：条件隐藏改为 ui:widget hidden——不再从 schema.properties 删除，
  // 切换联动时保留用户已填数据；仅从 required 摘除以豁免校验。指针标识
  // （SUI-019）ui 注入嵌套分支、required 摘除落在宿主 schema 上。
  if (hiddenFields.size > 0) {
    const unresolved: string[] = [];
    for (const key of hiddenFields) {
      const target = resolveFieldSchema(schema, key);
      if (!target.container || !target.property) {
        unresolved.push(key);
        continue;
      }
      // rjsf SchemaField 从 uiSchema 的 uiOptions 读 hidden，schema 保持纯净
      const nextUi = uiSchemaNodeAt(uiSchema as Record<string, unknown>, target.uiTokens);
      nextUi['ui:widget'] = 'hidden';
      toggleRequiredAt(target, target.property, false);
    }
    // schema 里没有的隐藏键沿用旧行为：只从根 required 摘除（键名可能来自
    // versioning 合并的历史配置），不注入 ui 分支。
    if (unresolved.length && Array.isArray(schema.required)) {
      const drop = new Set(unresolved);
      const filtered = schema.required.filter((key) => !drop.has(key));
      if (filtered.length !== schema.required.length) schema.required = filtered;
    }
  }

  // 必填字符串 = 非空：AJV required 只查键存在，"" 是合法值直接出网——游戏侧
  // 才报错（BUG-038 关联观察：demo inventory.grant 的 templateId 空串提交，
  // 修复前表现为 15s 超时）。required 且 type:string 且未显式声明 minLength
  // 的字段补 minLength:1；显式声明（含 validationRules 映射产物）不覆盖。
  // SUI-019：递归覆盖嵌套对象（指针字段 required 并入宿主 required 后同样生效）。
  enforceRequiredStringMinLength(schema as Record<string, unknown>);

  // label 兜底链（F5）：x-label hint（fields）> schema.title > key 人性化，
  // 覆盖 spec.fields 之外的字段，避免英文 key 裸奔。
  const props = schema.properties as Record<string, RJSFSchema> | undefined;
  if (props) {
    for (const [key, child] of Object.entries(props)) {
      if (child && typeof child === 'object' && !child.title) {
        child.title = humanizeFieldKey(key);
      }
    }
  }

  // 枚举标签兜底（GUX-017 实测发现）：spec 未提供 fields 时（资源模板
  // createForm 直接给 jsonSchema），rjsf v5 optionsList 只认 ui:options.enumNames，
  // schema 级 enumNames / x-enum-options 不会被消费，下拉露原始枚举值。
  // 标签来源 x-enum-options（平台 hints 契约）> schema.enumNames > 原始值。
  if (props) {
    for (const [key, child] of Object.entries(props)) {
      if (!child || typeof child !== 'object' || !Array.isArray(child.enum)) continue;
      const existing = (uiSchema[key] || {}) as UiSchema;
      const existingOptions = existing['ui:options'] as Record<string, unknown> | undefined;
      if (existingOptions && Array.isArray(existingOptions.enumNames)) continue;
      const hinted = asEnumNamesFromSchemaHints(child as unknown as Record<string, unknown>);
      const names =
        hinted ??
        (Array.isArray(child.enumNames)
          ? child.enum.map((value, i) =>
              typeof child.enumNames[i] === 'string'
                ? (child.enumNames[i] as string)
                : String(value),
            )
          : undefined);
      if (!names) continue;
      existing['ui:options'] = { ...(existingOptions || {}), enumNames: names };
      uiSchema[key] = existing;
    }
  }

  // ui:order 跳过隐藏字段，但隐藏字段仍保留在 schema 中（保值）。
  // SUI-019 边界：ui:order 只接受根属性名（rjsf 契约），指针标识不参与排序。
  const order = spec.fields
    ?.map((field) => field.key)
    .filter((key) => Boolean(key) && !hiddenFields.has(key) && !isPointerFieldKey(key));
  if (order?.length) {
    uiSchema['ui:order'] = [...order, '*'];
  }

  // 诊断（SUI-019）：呈现配置里定位不到的字段不静默丢弃，console.warn 报出原因；
  // 升级为保存/发布期阻断校验见 SUI-021。
  const unknown = unknownFieldPaths(
    (spec.fields ?? []).map((field) => ({ key: field.key })),
    spec.groups?.flatMap((group) => group.fields) ?? [],
    spec.jsonSchema,
  );
  for (const reason of unknown) {
    // eslint-disable-next-line no-console
    console.warn(`[SchemaFormRenderer] presentation field path unknown: ${reason}`);
  }

  return { schema, uiSchema, formContext, hiddenKeys: [...hiddenFields] };
}

/**
 * SUI-025：spec 未携带 fields（无配置 = 标准纵向表单）时按 jsonSchema 兜底
 * 合成字段（含类型缺省控件），渲染、初值种子与提交投影共用同一有效 spec——
 * 调试页与发布页对同一 schema 的无配置形态表现一致。layout/groups 仅在 spec
 * 未声明时回填（发布 spec 的 layout 恒有值；派生 groups 只在 hints 存在时
 * 产出，届时 fields 已存在，不会走到兜底）。
 *
 * SUI-035：渲染端解析链落地——defaultVariant → 基础呈现 → 派生纵向。发布的
 * `defaultVariant` 经 resolveVariantPresentation 合并（SUI-031 overlay 语义，
 * 与编辑器预览/运行时切换同一合并规则）；key 解析不到已声明变体（陈旧缓存）
 * 原样回落基础；无变体配置的旧页面合并为恒等。
 *
 * SUI-036：链首的显式选择在本组件内落地——切换器维护会话内 `explicitKey`，
 * 非空时压过 `defaultVariant` 进入同一解析；schema 恒不变，表单态按 schema
 * 路径寻址，切换保留输入与错误状态、提交 payload 恒等。
 */
function effectivePresentationSpec(
  spec: FormPresentationSpec,
  explicitKey?: string,
): FormPresentationSpec {
  const resolved = resolveVariantPresentation(spec, explicitKey || spec.defaultVariant);
  if (resolved.fields?.length) return resolved;
  const derived = deriveDefaultPresentationSpec(resolved.jsonSchema);
  if (!derived.fields?.length) return resolved;
  return {
    ...resolved,
    layout: resolved.layout ?? derived.layout,
    fields: derived.fields,
    groups: resolved.groups ?? derived.groups,
  };
}

/**
 * SUI-042：按所选分支构造有效呈现 spec。schema 换成分支合并产物后，fields
 * 走与 SUI-025 相同的兜底判定：spec 未配置 fields（或 fields 是对原 schema
 * 的派生产物）时按合并 schema 重新派生，分支专属字段获得类型缺省控件与标签
 * 兜底；显式配置的 fields 原样保留（配置面如何呈现分支字段由作者决定，不猜）。
 */
function branchMergedSpec(
  resolvedSpec: FormPresentationSpec,
  resolution: DiscriminatorResolution & { propertyName: string },
  branch: DiscriminatorBranch,
): FormPresentationSpec {
  const mergedJsonSchema = mergeBranchIntoSchema(
    resolvedSpec.jsonSchema as JSONSchema,
    branch,
    resolution.propertyName,
  );
  const base: FormPresentationSpec = { ...resolvedSpec, jsonSchema: mergedJsonSchema };
  const deriveOn = (schema: JSONSchema): FormPresentationSpec | null => {
    const derived = deriveDefaultPresentationSpec(schema);
    if (!derived.fields?.length) return null;
    return {
      ...base,
      layout: base.layout ?? derived.layout,
      fields: derived.fields,
      groups: base.groups ?? derived.groups,
    };
  };
  if (!base.fields?.length) return deriveOn(mergedJsonSchema) ?? base;
  // fields 与「对原 schema 的派生结果」逐字节一致 → 视为 SUI-025 派生产物
  const originalDerived = deriveDefaultPresentationSpec(resolvedSpec.jsonSchema as JSONSchema);
  if (
    originalDerived.fields?.length &&
    JSON.stringify(originalDerived.fields) === JSON.stringify(resolvedSpec.fields)
  ) {
    return deriveOn(mergedJsonSchema) ?? base;
  }
  return base;
}

/** SUI-027：同页多实例字段 ID 隔离。rjsf 以 `{idPrefix}_{路径}` 生成控件
 * id/label htmlFor/aria 关联，默认前缀 `root`——同页挂载两个同 schema 表单
 * 时两实例产出相同 id，label 与错误说明跨实例串扰。这里维护存活前缀集合：
 * 首个实例保留默认 `root`（既有用例与页面选择器 `#root_*` 不变），并发实例
 * 依次领取 `root-2`/`root-3`…；实例卸载即归还，串行重渲染各自回到 `root`。
 * 前缀在实例生命周期内稳定（useRef 首渲染领取），重渲染不换号。 */
const liveIdPrefixes = new Set<string>();

function nextIdPrefix(): string {
  let n = 1;
  let candidate = 'root';
  while (liveIdPrefixes.has(candidate)) {
    n += 1;
    candidate = `root-${n}`;
  }
  liveIdPrefixes.add(candidate);
  return candidate;
}

const SchemaFormRenderer = forwardRef<SchemaFormRendererHandle, SchemaFormRendererProps>(
  (
    {
      spec,
      initialValues,
      onFinish,
      onValuesChange,
      readonly = false,
      disabled = false,
      hideSubmit = false,
      containerTagName = 'form',
      externalErrors,
    },
    ref,
  ) => {
    const formRef = useRef<RJSFFormRef | null>(null);
    // SUI-027：实例级字段 ID 前缀（首个实例保持默认 `root`，并发实例领
    // `root-2`/`root-3`…；卸载归还、重渲染稳定，见上方 allocator 注释）。
    const idPrefixRef = useRef<string | null>(null);
    if (idPrefixRef.current === null) idPrefixRef.current = nextIdPrefix();
    const idPrefix = idPrefixRef.current;
    useEffect(() => {
      const allocated = idPrefix;
      return () => {
        liveIdPrefixes.delete(allocated);
      };
    }, [idPrefix]);
    // SUI-026：声明非对象根类型（数组/标量/null）的 schema 走 JSON 编辑形态，
    // 与对象表单同一 handle 契约、同一校验器、同一绑定执行链。
    const jsonRoot = useMemo(() => isJsonRootSpec(spec.jsonSchema), [spec]);
    const jsonRootRef = useRef<SchemaFormRendererHandle | null>(null);
    const initialValuesJson = useMemo(
      () => JSON.stringify(normalizeFormValues(initialValues || {})),
      [initialValues],
    );

    // SUI-024：reload=false 的语言切换经 languagechange 事件广播（umi
    // plugin-locale setLocale 非刷新路径发系统事件）；订阅后派生 schema/uiSchema
    // 随 locale 重算（文案换语言），formData 镜像不动（镜像即实时值），已填写
    // 数据不重置。刷新式切换（默认）整页重载，本机制不参与。
    const [localeTick, setLocaleTick] = useState(0);
    useEffect(() => {
      const bump = (): void => setLocaleTick((tick) => tick + 1);
      window.addEventListener('languagechange', bump);
      return () => window.removeEventListener('languagechange', bump);
    }, []);

    // SUI-036：命名风格的会话内显式选择。初值空 = 跟随发布 defaultVariant；
    // schema 恒取基础，切风格只换 uiSchema 派生，输入与错误态不重置。
    const variantOptions = useMemo(
      () =>
        (spec.variants || [])
          .filter((variant) => variant && typeof variant.key === 'string' && variant.key !== '')
          .map((variant) => ({
            value: variant.key,
            label: localizedText(variant.title, currentLocale(), variant.key),
            // title 固定为 key（antd6 默认用 label 文本做 title）——切换项
            // 的稳定标识，测试与无障碍提示与界面语言无关
            title: variant.key,
          })),
      // localeTick 入参：语言切换后重算风格名
      // eslint-disable-next-line react-hooks/exhaustive-deps -- 刻意的失效触发器
      [spec.variants, localeTick],
    );
    const [explicitVariant, setExplicitVariant] = useState<string | undefined>(undefined);
    const variantKeySet = useMemo(
      () => (spec.variants || []).map((variant) => variant.key).join('\n'),
      [spec.variants],
    );
    useEffect(() => {
      // 换页面/换函数：显式选择复位回发布默认（陈旧 key 虽能安全回落，但
      // 切换器上会显示不存在的选项）
      setExplicitVariant(undefined);
    }, [variantKeySet]);
    // 解析链首的显式选择压过发布默认；SUI-035 的同源解析继续处理回退。
    const resolvedSpec = useMemo(
      () => effectivePresentationSpec(spec, explicitVariant),
      // localeTick 不参与解析（风格名才随语言变），spec/显式 key 变才重解析
      [spec, explicitVariant],
    );
    // SUI-042：discriminator 分支层——可解析多态根按所选分支合并 schema 渲染，
    // 不可解析组合不猜语义、交回 JSON 降级（诊断说明原因）。非多态根返回
    // null，既有流程零变化。
    const branchResolution = useMemo(
      () => resolveDiscriminatorBranches(resolvedSpec.jsonSchema),
      [resolvedSpec],
    );
    // SUI-042：不可解析分支也走 JSON 编辑器；imperative handle 与提交路径
    // 需与渲染形态同源，不能只看非对象根判定。
    const branchDegraded = branchResolution?.status === 'unresolved';
    const jsonEditor = jsonRoot || branchDegraded;
    const [explicitBranch, setExplicitBranch] = useState<string | undefined>(undefined);
    const branchKeys = useMemo(
      () =>
        branchResolution?.status === 'resolved'
          ? branchResolution.branches.map((branch) => branch.value).join('\n')
          : '',
      [branchResolution],
    );
    useEffect(() => {
      // 换页面/换函数（分支集变化）：显式分支复位到首个声明分支
      setExplicitBranch(undefined);
    }, [branchKeys]);
    const selectedBranch = useMemo(() => {
      if (branchResolution?.status !== 'resolved') return null;
      const branches = branchResolution.branches;
      const seeded = (initialValues as FormValues | undefined)?.[branchResolution.propertyName];
      const seededBranch =
        typeof seeded === 'string' ? branches.find((branch) => branch.value === seeded) : undefined;
      return (
        branches.find((branch) => branch.value === explicitBranch) ??
        seededBranch ??
        branches[0] ??
        null
      );
    }, [branchResolution, explicitBranch, initialValues]);
    const branchSpec = useMemo(() => {
      if (branchResolution?.status !== 'resolved' || !selectedBranch) return resolvedSpec;
      return branchMergedSpec(resolvedSpec, branchResolution, selectedBranch);
    }, [branchResolution, resolvedSpec, selectedBranch]);
    const seededInitial = useMemo(() => {
      const values = applySpecDefaults(branchSpec, normalizeFormValues(initialValues || {}));
      // discriminator 是合并 schema 的必填属性：显式初值/默认配置之外的分支值
      // 由选择器补齐，保证首屏提交 payload 自带所选分支标识。
      if (branchResolution?.status === 'resolved' && selectedBranch) {
        const key = branchResolution.propertyName;
        if (values[key] === undefined || values[key] !== selectedBranch.value) {
          return { ...values, [key]: selectedBranch.value };
        }
      }
      return values;
    }, [branchSpec, branchResolution, selectedBranch, initialValues]);
    const currentValuesRef = useRef<FormValues>(seededInitial);
    const [formValues, setFormValues] = useState<FormValues>(seededInitial);
    const lastInitJsonRef = useRef<string | null>(null);

    useEffect(() => {
      // 外部重置初值（JSON 变化）才同步；自身 onChange 回声由 JSON 相等跳过
      if (lastInitJsonRef.current === initialValuesJson) return;
      lastInitJsonRef.current = initialValuesJson;
      currentValuesRef.current = seededInitial;
      setFormValues(currentValuesRef.current);
    }, [initialValuesJson, seededInitial]);

    // SUI-042：切换分支——按目标分支构造合并 spec，裁剪表单值（只保留所选
    // 分支允许的键，预置 discriminator 值），再补默认值。共享键已填值保留。
    const handleBranchChange = useCallback(
      (key: string) => {
        setExplicitBranch(key);
        if (branchResolution?.status !== 'resolved') return;
        const branch = branchResolution.branches.find((item) => item.value === key);
        if (!branch) return;
        const nextSpec = branchMergedSpec(resolvedSpec, branchResolution, branch);
        const pruned = pruneValuesToBranch(
          currentValuesRef.current,
          nextSpec.jsonSchema as JSONSchema,
          branchResolution.propertyName,
          key,
        );
        const next = applySpecDefaults(nextSpec, pruned);
        currentValuesRef.current = next;
        setFormValues(next);
        onValuesChange?.({}, next);
      },
      [branchResolution, resolvedSpec, onValuesChange],
    );
    const { schema, uiSchema, formContext } = useMemo(() => {
      const derived = deriveRuntimeSchema(branchSpec, formValues);
      if (hideSubmit || readonly) {
        derived.uiSchema['ui:submitButtonOptions'] = {
          ...(derived.uiSchema['ui:submitButtonOptions'] || {}),
          norender: true,
        };
      }
      return derived;
      // localeTick 入参：语言切换 tick 触发派生重算（currentLocale() 在纯函数内读取）
      // eslint-disable-next-line react-hooks/exhaustive-deps -- localeTick 是刻意的失效触发器
    }, [formValues, hideSubmit, readonly, branchSpec, localeTick]);
    // handler 引用稳定（见下方 rjsf 注释），spec 经 ref 读取当前值
    const specRef = useRef(resolvedSpec);
    specRef.current = branchSpec;

    useImperativeHandle(
      ref,
      () => ({
        submit: () => {
          if (jsonEditor) {
            jsonRootRef.current?.submit();
            return;
          }
          formRef.current?.submit();
        },
        validate: () =>
          jsonEditor
            ? Boolean(jsonRootRef.current?.validate())
            : Boolean(formRef.current?.validateForm()),
        getValues: () =>
          jsonEditor
            ? (jsonRootRef.current?.getValues() ?? {})
            : getLiveFormValues(formRef.current, currentValuesRef.current),
        getSubmitValues: () =>
          jsonEditor
            ? (jsonRootRef.current?.getSubmitValues() ?? {})
            : projectSubmitValues(
                getLiveFormValues(formRef.current, currentValuesRef.current),
                specRef.current,
              ),
      }),
      [jsonEditor],
    );

    // rjsf Form 在任意 props 变化时会把内部 state 重置回 props.formData；
    // 因此所有 handler 必须引用稳定，保证 props 仅在内容真正变化时才不等，
    // 否则镜像滞后（React 19 延迟回调）会把 Form state 回滚到旧值。
    const handleChangeEvent = useCallback(
      (event: IChangeEvent<FormValues>) => {
        const next = normalizeFormValues(event.formData);
        currentValuesRef.current = next;
        setFormValues(next);
        onValuesChange?.({}, next);
      },
      [onValuesChange],
    );
    const handleSubmitEvent = useCallback(
      async (event: IChangeEvent<FormValues>) => {
        // SUI-014：提交 payload 经唯一投影 projectSubmitValues 生成（隐藏键按
        // 当前值剔除）；隐藏字段值保留在表单态（currentValuesRef 不动），可见性
        // 切回时恢复。与命令式 getSubmitValues() 完全同一取值逻辑。
        const next = projectSubmitValues(normalizeFormValues(event.formData), specRef.current);
        await onFinish?.(next);
      },
      [onFinish],
    );
    const transformErrors = useCallback(
      (errors: RJSFValidationError[]): RJSFValidationError[] =>
        localizeFormErrors(errors, schema, currentLocale()),
      [schema],
    );
    const widgets = useMemo(() => ({ ...customWidgets, ...uploadWidgets }), []);
    // 2.2：spec 内容变化时复位错误边界（换函数/换页面不残留上一次的崩溃态）
    const boundaryKey = useMemo(() => {
      try {
        // 崩溃边界以基础 spec 为键（SUI-036）：风格切换只变解析产物，键不
        // 变 → 不重挂载，rjsf 内部错误态保留；换页面/换函数（基础 spec 变）
        // 仍复位。
        return JSON.stringify(spec);
      } catch {
        return `spec-${Date.now()}`;
      }
    }, [spec]);

    // D4：外部字段错误按 RJSF ErrorSchema 注入——字段自身的错误键是 __errors，
    // 但 @rjsf/utils 的 ErrorSchema 类型只声明了嵌套结构，运行时约定需断言收敛
    const extraErrors = useMemo<ErrorSchema | undefined>(() => {
      if (!externalErrors) return undefined;
      const schema: Record<string, { __errors: string[] }> = {};
      for (const [field, messages] of Object.entries(externalErrors)) {
        if (!field || !messages || messages.length === 0) continue;
        schema[field] = { __errors: messages };
      }
      return Object.keys(schema).length > 0 ? (schema as unknown as ErrorSchema) : undefined;
    }, [externalErrors]);

    if (jsonRoot) {
      // SUI-026：JSON 根形态不经 rjsf（对象表单引擎表达不了非对象根），由
      // JsonRootEditor 提供同契约编辑入口。
      return (
        <JsonRootEditor
          ref={jsonRootRef}
          spec={resolvedSpec}
          initialValues={initialValues}
          onFinish={onFinish}
          onValuesChange={onValuesChange}
          readonly={readonly}
          disabled={disabled}
          hideSubmit={hideSubmit}
        />
      );
    }

    if (branchDegraded) {
      // SUI-042：discriminator 已声明但分支无法明确解析——结构化表单不猜
      // 语义，降级为完整请求 JSON 编辑（同一执行与校验链），诊断给出原因。
      const degradeReason = getIntl().formatMessage(
        {
          id: 'component.schemaFormRenderer.branch.degraded',
          defaultMessage:
            '多态分支无法明确解析：{reason}；结构化表单不猜测语义，请编辑完整请求 JSON——提交仍经本页面绑定的同一执行与 schema 校验链。',
        },
        { reason: branchResolution.reason },
      );
      return (
        <JsonRootEditor
          ref={jsonRootRef}
          spec={resolvedSpec}
          initialValues={initialValues}
          onFinish={onFinish}
          onValuesChange={onValuesChange}
          readonly={readonly}
          disabled={disabled}
          hideSubmit={hideSubmit}
          reasonOverride={degradeReason}
        />
      );
    }

    const showBranchSelector =
      branchResolution?.status === 'resolved' && branchResolution.branches.length > 1;

    return (
      <SchemaFormErrorBoundary key={boundaryKey}>
        {showBranchSelector && selectedBranch && (
          // SUI-042：分支切换器——选中分支决定合并后的 schema（结构化渲染
          // 与校验都只面向所选分支）；切换经 handleBranchChange 裁剪表单值。
          <Select
            size="small"
            data-testid="branch-select"
            style={{ marginBottom: 12, minWidth: 200 }}
            value={selectedBranch.value}
            options={branchResolution.branches.map((branch) => ({
              value: branch.value,
              label: branch.label,
            }))}
            onChange={handleBranchChange}
            aria-label={getIntl().formatMessage({
              id: 'component.schemaFormRenderer.branchLabel',
              defaultMessage: 'Branch',
            })}
          />
        )}
        {variantOptions.length > 1 && (
          // SUI-036：风格切换器（发布默认 + 会话内显式选择，同一解析链）。
          // 切换只重派 uiSchema，表单实例不重挂载——输入与错误态保留。
          <Segmented
            size="small"
            data-testid="variant-switch"
            value={explicitVariant || spec.defaultVariant || undefined}
            options={variantOptions}
            onChange={(key) => setExplicitVariant(String(key))}
            aria-label={getIntl().formatMessage({
              id: 'component.schemaFormRenderer.variantSwitchLabel',
              defaultMessage: 'Style',
            })}
          />
        )}
        <Form
          ref={formRef as React.Ref<RJSFFormRef>}
          schema={schema}
          uiSchema={uiSchema}
          idPrefix={idPrefix}
          tagName={containerTagName}
          formContext={formContext}
          validator={validator}
          widgets={widgets}
          fields={uploadFields}
          extraErrors={extraErrors}
          templates={customTemplates}
          formData={formValues}
          readonly={readonly}
          disabled={disabled}
          liveValidate={false}
          omitExtraData
          noHtml5Validate
          transformErrors={transformErrors}
          onChange={handleChangeEvent}
          onSubmit={handleSubmitEvent}
        />
      </SchemaFormErrorBoundary>
    );
  },
);

SchemaFormRenderer.displayName = 'SchemaFormRenderer';

export default SchemaFormRenderer;
