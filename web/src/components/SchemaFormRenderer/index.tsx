/**
 * JSON Schema form runtime adapter.
 *
 * The public contract is FormPresentationSpec. RJSF uiSchema is derived in
 * memory only and must not be persisted into SDK/OpenAPI/PageSpec snapshots.
 */

import React, { forwardRef, useImperativeHandle, useMemo, useRef } from 'react';
import Form from '@rjsf/antd';
import type CoreForm from '@rjsf/core';
import type { IChangeEvent } from '@rjsf/core';
import validator from '@rjsf/validator-ajv8';
import type { RJSFSchema, UiSchema } from '@rjsf/utils';
import type {
  FormFieldSpec,
  FormPresentationSpec,
  FormValues,
  FormWidget,
  JSONSchema,
  JSONValue,
} from '@/types/dashboard';

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

function getLocalizedText(
  value: Record<string, string> | undefined,
  fallback: string,
): string {
  if (!value) return fallback;
  return value['zh-CN'] || value.zh || value.en || value['en-US'] || fallback;
}

function getFieldSchema(schema: JSONSchema, key: string): JSONSchema {
  const properties = schema.properties;
  if (!isObject(properties)) return {};
  const child = properties[key];
  return isObject(child) ? child : {};
}

function getEnumNames(field: FormFieldSpec | undefined): string[] | undefined {
  if (!field?.enumOptions?.length) return undefined;
  return field.enumOptions.map((option) => getLocalizedText(option.label, String(option.value)));
}

function shouldUseTextarea(schema: JSONSchema): boolean {
  return schema.format === 'textarea' || Number(schema.maxLength || 0) > 120;
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
    case 'MultiSelect':
    case 'Input':
    case 'InputNumber':
    case 'DateRange':
    case 'Upload':
    case 'ImageUpload':
    case 'FileUpload':
    case 'RichText':
    case 'Cascader':
    case 'TreeSelect':
    case 'Rate':
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
) {
  const fieldSchema = getFieldSchema(rootSchema, field.key);
  const nextUi = (uiSchema[field.key] || {}) as UiSchema;
  const widget = widgetToRjsf(field.widget, fieldSchema);

  if (field.label) {
    const properties = jsonSchema.properties as Record<string, RJSFSchema> | undefined;
    if (properties?.[field.key]) {
      properties[field.key] = {
        ...properties[field.key],
        title: getLocalizedText(field.label, field.key),
      };
    }
  }
  if (field.description) {
    const properties = jsonSchema.properties as Record<string, RJSFSchema> | undefined;
    if (properties?.[field.key]) {
      properties[field.key] = {
        ...properties[field.key],
        description: getLocalizedText(field.description, ''),
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
  if (field.placeholder) nextUi['ui:placeholder'] = getLocalizedText(field.placeholder, '');
  if (widget) nextUi['ui:widget'] = widget;
  if (field.widget === 'MultiSelect') nextUi['ui:widget'] = 'select';
  if (field.widgetProps) nextUi['ui:options'] = { ...(nextUi['ui:options'] || {}), ...field.widgetProps };
  uiSchema[field.key] = nextUi;
}

function deriveRuntimeSchema(spec: FormPresentationSpec): {
  schema: RJSFSchema;
  uiSchema: UiSchema;
} {
  const schema = { ...(spec.jsonSchema || {}) } as RJSFSchema;
  const uiSchema: UiSchema = {
    'ui:submitButtonOptions': {
      submitText: getLocalizedText(spec.submitButton?.text, '提交'),
      norender: false,
    },
  };

  if (!schema.type) schema.type = 'object';
  if (!schema.properties) schema.properties = {};

  const rootSchema = spec.jsonSchema || {};
  for (const field of spec.fields || []) {
    applyFieldPresentation(schema, uiSchema, field, rootSchema);
  }

  const order = spec.fields?.map((field) => field.key).filter(Boolean);
  if (order?.length) {
    uiSchema['ui:order'] = [...order, '*'];
  }

  return { schema, uiSchema };
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

    const { schema, uiSchema } = useMemo(() => {
      const derived = deriveRuntimeSchema(spec);
      if (hideSubmit || readonly) {
        derived.uiSchema['ui:submitButtonOptions'] = {
          ...(derived.uiSchema['ui:submitButtonOptions'] || {}),
          norender: true,
        };
      }
      return derived;
    }, [hideSubmit, readonly, spec]);

    useImperativeHandle(ref, () => ({
      submit: () => {
        formRef.current?.submit();
      },
      validate: () => Boolean(formRef.current?.validateForm()),
      getValues: () => currentValuesRef.current,
    }));

    return (
      <Form
        ref={formRef as React.Ref<RJSFFormRef>}
        schema={schema}
        uiSchema={uiSchema}
        validator={validator}
        formData={initialValues || {}}
        readonly={readonly}
        disabled={disabled}
        liveValidate={false}
        omitExtraData
        noHtml5Validate
        onChange={(event: IChangeEvent<FormValues>) => {
          const next = normalizeFormValues(event.formData);
          currentValuesRef.current = next;
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
