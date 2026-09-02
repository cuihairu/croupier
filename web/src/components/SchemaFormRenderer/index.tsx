/**
 * JSON Schema form runtime adapter.
 *
 * The public contract is FormPresentationSpec. RJSF uiSchema is derived in
 * memory only and must not be persisted into SDK/OpenAPI/PageSpec snapshots.
 */

import React, {
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
import { getLocale } from '@umijs/max';
import type { RJSFValidationError, RJSFSchema, UiSchema } from '@rjsf/utils';
import type {
  FormFieldSpec,
  FormPresentationSpec,
  FormValues,
  FormWidget,
  ConditionSpec,
  JSONSchema,
  JSONValue,
} from '@/types/dashboard';
import { localizedText } from '@/utils/localizedText';
import { humanizeFieldKey } from '@/utils/humanize';
import { customWidgets } from './widgets';

export interface SchemaFormRendererHandle {
  submit: () => void;
  validate: () => boolean;
  getValues: () => FormValues;
}

export interface SchemaFormRendererProps {
  spec: FormPresentationSpec;
  initialValues?: FormValues;
  onFinish?: (values: FormValues) => Promise<boolean | void> | boolean | void;
  onValuesChange?: (changedValues: FormValues, allValues: FormValues) => void;
  readonly?: boolean;
  disabled?: boolean;
  hideSubmit?: boolean;
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
      Object.entries(value).map(([key, item]) => [key, normalizeJsonValue(item)]),
    );
  }
  return null;
}

function normalizeFormValues(value: unknown): FormValues {
  if (!isObject(value)) return {};
  return Object.fromEntries(
    Object.entries(value).map(([key, item]) => [key, normalizeJsonValue(item)]),
  );
}

function getFieldSchema(schema: JSONSchema, key: string): JSONSchema {
  const properties = schema.properties;
  if (!isObject(properties)) return {};
  const child = properties[key];
  return isObject(child) ? child : {};
}

function valueAtPointer(value: JSONValue | undefined, pointer: string): JSONValue | undefined {
  if (value === undefined || !pointer.startsWith('/')) return undefined;
  let current = value;
  for (const token of pointer.slice(1).split('/')) {
    const key = token.replace(/~1/g, '/').replace(/~0/g, '~');
    if (Array.isArray(current)) {
      const index = Number(key);
      if (!Number.isInteger(index) || index < 0 || index >= current.length) return undefined;
      current = current[index];
      continue;
    }
    if (!isObject(current) || !Object.prototype.hasOwnProperty.call(current, key)) return undefined;
    current = current[key];
  }
  return current;
}

function sameJsonValue(left: JSONValue | undefined, right: JSONValue): boolean {
  return JSON.stringify(left) === JSON.stringify(right);
}

function matchesCondition(condition: ConditionSpec | undefined, values: FormValues): boolean {
  if (!condition) return true;
  switch (condition.kind) {
    case 'equals':
      return sameJsonValue(valueAtPointer(values, condition.path), condition.value);
    case 'notEquals':
      return !sameJsonValue(valueAtPointer(values, condition.path), condition.value);
    case 'exists':
      return valueAtPointer(values, condition.path) !== undefined;
    case 'all':
      return condition.conditions.every((item) => matchesCondition(item, values));
    case 'any':
      return condition.conditions.some((item) => matchesCondition(item, values));
  }
}

function getEnumNames(field: FormFieldSpec | undefined): string[] | undefined {
  if (!field?.enumOptions?.length) return undefined;
  return field.enumOptions.map((option) =>
    localizedText(option.label, 'zh-CN', String(option.value)),
  );
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

/** 平台当前 locale；umi 运行时外（单测/异常）回退 zh-CN。 */
function currentLocale(): string {
  try {
    return getLocale() || 'zh-CN';
  } catch {
    return 'zh-CN';
  }
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
      return 'date';
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
    case 'MultiSelect':
    case 'Input':
    case 'InputNumber':
    case 'DateRange':
    case 'Upload':
    case 'ImageUpload':
    case 'FileUpload':
    case 'RichText':
    case 'KeyValue':
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
  const fieldSchema = getFieldSchema(rootSchema, field.key);
  const nextUi = (uiSchema[field.key] || {}) as UiSchema;
  const widget = widgetToRjsf(field.widget, fieldSchema);

  if (field.label) {
    const properties = jsonSchema.properties as Record<string, RJSFSchema> | undefined;
    if (properties?.[field.key]) {
      properties[field.key] = {
        ...properties[field.key],
        title: localizedText(field.label, 'zh-CN', field.key),
      };
    }
  }
  if (field.description) {
    const properties = jsonSchema.properties as Record<string, RJSFSchema> | undefined;
    if (properties?.[field.key]) {
      properties[field.key] = {
        ...properties[field.key],
        description: localizedText(field.description, 'zh-CN', ''),
      };
    }
  }
  if (field.enumOptions?.length) {
    const properties = jsonSchema.properties as Record<string, RJSFSchema> | undefined;
    if (properties?.[field.key]) {
      properties[field.key] = {
        ...properties[field.key],
        enumNames: getEnumNames(field),
      };
    }
  }
  if (field.disabled) nextUi['ui:disabled'] = true;
  if (field.placeholder) nextUi['ui:placeholder'] = localizedText(field.placeholder, 'zh-CN', '');
  if (widget) nextUi['ui:widget'] = widget;
  if (field.widget === 'MultiSelect') {
    // F3：多选 = 内置 select + multiple；enum 由 schema enum/enumNames 提供
    nextUi['ui:widget'] = 'select';
    nextUi['ui:options'] = { ...(nextUi['ui:options'] || {}), multiple: true };
  }
  if (field.widgetProps)
    nextUi['ui:options'] = { ...(nextUi['ui:options'] || {}), ...field.widgetProps };
  uiSchema[field.key] = nextUi;
  return field.visible !== false;
}

function cloneSchema(schema: JSONSchema): RJSFSchema {
  return JSON.parse(JSON.stringify(schema || {})) as RJSFSchema;
}

export function deriveRuntimeSchema(
  spec: FormPresentationSpec,
  values: FormValues,
): {
  schema: RJSFSchema;
  uiSchema: UiSchema;
  formContext: Record<string, unknown>;
} {
  const schema = cloneSchema(spec.jsonSchema);
  const uiSchema: UiSchema = {
    'ui:submitButtonOptions': {
      submitText: localizedText(spec.submitButton?.text, 'zh-CN', '提交'),
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

  if (!schema.type) schema.type = 'object';
  if (!schema.properties) schema.properties = {};

  const rootSchema = spec.jsonSchema || {};
  const hiddenFields = new Set<string>();
  for (const field of spec.fields || []) {
    const visible =
      matchesCondition(field.visibleWhen, values) &&
      applyFieldPresentation(schema, uiSchema, field, rootSchema);
    if (!visible) hiddenFields.add(field.key);
  }

  if (hiddenFields.size > 0) {
    const properties = schema.properties as Record<string, RJSFSchema> | undefined;
    for (const key of hiddenFields) {
      delete properties?.[key];
      delete uiSchema[key];
    }
    if (Array.isArray(schema.required)) {
      schema.required = schema.required.filter((key) => !hiddenFields.has(key));
    }
  }

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

  const order = spec.fields
    ?.map((field) => field.key)
    .filter((key) => Boolean(key) && !hiddenFields.has(key));
  if (order?.length) {
    uiSchema['ui:order'] = [...order, '*'];
  }

  return { schema, uiSchema, formContext };
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
    },
    ref,
  ) => {
    const formRef = useRef<RJSFFormRef | null>(null);
    const currentValuesRef = useRef<FormValues>(initialValues || {});
    const [formValues, setFormValues] = useState<FormValues>(initialValues || {});

    useEffect(() => {
      currentValuesRef.current = initialValues || {};
      setFormValues(initialValues || {});
    }, [initialValues]);

    const { schema, uiSchema, formContext } = useMemo(() => {
      const derived = deriveRuntimeSchema(spec, formValues);
      if (hideSubmit || readonly) {
        derived.uiSchema['ui:submitButtonOptions'] = {
          ...(derived.uiSchema['ui:submitButtonOptions'] || {}),
          norender: true,
        };
      }
      return derived;
    }, [formValues, hideSubmit, readonly, spec]);

    useImperativeHandle(ref, () => ({
      submit: () => {
        formRef.current?.submit();
      },
      validate: () => Boolean(formRef.current?.validateForm()),
      getValues: () => currentValuesRef.current,
    }));

    const transformErrors = (errors: RJSFValidationError[]): RJSFValidationError[] =>
      localizeFormErrors(errors, schema, currentLocale());

    return (
      <Form
        ref={formRef as React.Ref<RJSFFormRef>}
        schema={schema}
        uiSchema={uiSchema}
        formContext={formContext}
        validator={validator}
        widgets={customWidgets}
        formData={formValues}
        readonly={readonly}
        disabled={disabled}
        liveValidate={false}
        omitExtraData
        noHtml5Validate
        transformErrors={transformErrors}
        onChange={(event: IChangeEvent<FormValues>) => {
          const next = normalizeFormValues(event.formData);
          currentValuesRef.current = next;
          setFormValues(next);
          onValuesChange?.({}, next);
        }}
        onSubmit={async (event: IChangeEvent<FormValues>) => {
          const next = normalizeFormValues(event.formData);
          currentValuesRef.current = next;
          await onFinish?.(next);
        }}
      />
    );
  },
);

SchemaFormRenderer.displayName = 'SchemaFormRenderer';

export default SchemaFormRenderer;
