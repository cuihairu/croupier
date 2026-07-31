/**
 * SchemaFormRenderer - JSON Schema 表单渲染器
 *
 * 根据 FormPresentationSpec 渲染表单，支持：
 * - JSON Schema 字段类型
 * - 自定义组件 hints
 * - 字段分组
 * - 验证规则
 *
 * @module components/SchemaFormRenderer
 */

import React, { useMemo } from 'react';
import {
  ProForm,
  ProFormText,
  ProFormTextArea,
  ProFormSelect,
  ProFormDigit,
  ProFormSwitch,
  ProFormDatePicker,
  ProFormTimePicker,
  ProFormCheckbox,
  ProFormRadio,
} from '@ant-design/pro-components';
import { Card, Typography, Space, Divider, Collapse } from 'antd';
import type {
  FormPresentationSpec,
  FormFieldSpec,
  FormGroupSpec,
  FormWidget,
} from '@/types/dashboard-vnext';

const { Text, Title } = Typography;
const { Panel } = Collapse;

// ---------------------------------------------------------------------------
// Props
// ---------------------------------------------------------------------------

export interface SchemaFormRendererProps {
  /** 表单展示规格 */
  spec: FormPresentationSpec;
  /** 表单初始值 */
  initialValues?: Record<string, unknown>;
  /** 表单提交回调 */
  onFinish?: (values: Record<string, unknown>) => Promise<boolean | void>;
  /** 表单值变化回调 */
  onValuesChange?: (changedValues: Record<string, unknown>, allValues: Record<string, unknown>) => void;
  /** 是否只读 */
  readonly?: boolean;
  /** 自定义组件映射 */
  widgetMap?: Partial<Record<FormWidget, React.ComponentType<unknown>>>;
}

// ---------------------------------------------------------------------------
// 字段类型推断
// ---------------------------------------------------------------------------

function inferWidgetFromSchema(schema: Record<string, unknown>, fieldKey: string): FormWidget {
  const type = schema.type as string;
  const format = schema.format as string;
  const enumValues = schema.enum as unknown[];

  // 有枚举值使用 Select
  if (enumValues && enumValues.length > 0) {
    return 'Select';
  }

  // 根据格式推断
  if (format === 'date') return 'DatePicker';
  if (format === 'date-time') return 'DatePicker';
  if (format === 'time') return 'TimePicker';
  if (format === 'email') return 'Input';
  if (format === 'uri') return 'Input';
  if (format === 'password') return 'Password';
  if (format === 'textarea') return 'TextArea';

  // 根据类型推断
  switch (type) {
    case 'string':
      return 'Input';
    case 'number':
    case 'integer':
      return 'InputNumber';
    case 'boolean':
      return 'Switch';
    case 'array':
      return 'Array';
    case 'object':
      return 'Object';
    default:
      return 'Input';
  }
}

// ---------------------------------------------------------------------------
// 单字段渲染
// ---------------------------------------------------------------------------

function renderSingleField(
  field: FormFieldSpec,
  schema: Record<string, unknown>,
  readonly: boolean
): React.ReactNode {
  const label = field.label?.['zh-CN'] || field.label?.['en'] || field.key;
  const placeholder = field.placeholder?.['zh-CN'] || field.placeholder?.['en'];
  const description = field.description?.['zh-CN'] || field.description?.['en'];
  const required = field.required;
  const disabled = field.disabled || readonly;

  // 验证规则
  const rules: unknown[] = [];
  if (required) {
    rules.push({ required: true, message: `请输入${label}` });
  }

  // 从 schema 获取额外验证
  const minLength = schema.minLength as number;
  const maxLength = schema.maxLength as number;
  const pattern = schema.pattern as string;
  const minimum = schema.minimum as number;
  const maximum = schema.maximum as number;

  if (minLength !== undefined) {
    rules.push({ min: minLength, message: `最少 ${minLength} 个字符` });
  }
  if (maxLength !== undefined) {
    rules.push({ max: maxLength, message: `最多 ${maxLength} 个字符` });
  }
  if (pattern) {
    rules.push({ pattern: new RegExp(pattern), message: '格式不正确' });
  }

  // 确定组件类型
  const widget = field.widget || inferWidgetFromSchema(schema, field.key);

  // 公共属性
  const commonProps = {
    key: field.key,
    name: field.key,
    label,
    placeholder,
    rules,
    tooltip: description,
    disabled,
    width: field.width ? `${field.width}px` : undefined,
  };

  // 渲染组件
  switch (widget) {
    case 'TextArea':
      return <ProFormTextArea {...commonProps} />;

    case 'InputNumber':
      return (
        <ProFormDigit
          {...commonProps}
          min={minimum}
          max={maximum}
          fieldProps={{ precision: schema.type === 'integer' ? 0 : undefined }}
        />
      );

    case 'Switch':
      return <ProFormSwitch {...commonProps} />;

    case 'Checkbox':
      return <ProFormCheckbox {...commonProps} />;

    case 'Radio':
      return (
        <ProFormRadio.Group
          {...commonProps}
          options={field.enumOptions?.map((opt) => ({
            label: opt.label['zh-CN'] || opt.label['en'] || opt.value,
            value: opt.value,
          }))}
        />
      );

    case 'Select':
      return (
        <ProFormSelect
          {...commonProps}
          options={field.enumOptions?.map((opt) => ({
            label: opt.label['zh-CN'] || opt.label['en'] || opt.value,
            value: opt.value,
          }))}
        />
      );

    case 'MultiSelect':
      return (
        <ProFormSelect
          {...commonProps}
          mode="multiple"
          options={field.enumOptions?.map((opt) => ({
            label: opt.label['zh-CN'] || opt.label['en'] || opt.value,
            value: opt.value,
          }))}
        />
      );

    case 'DatePicker':
      return <ProFormDatePicker {...commonProps} />;

    case 'TimePicker':
      return <ProFormTimePicker {...commonProps} />;

    case 'Password':
      return <ProFormText.Password {...commonProps} />;

    case 'Code':
      return (
        <ProFormTextArea
          {...commonProps}
          fieldProps={{
            style: { fontFamily: 'monospace' },
            autoSize: { minRows: 3, maxRows: 10 },
          }}
        />
      );

    case 'JSON':
      return (
        <ProFormTextArea
          {...commonProps}
          fieldProps={{
            style: { fontFamily: 'monospace' },
            autoSize: { minRows: 3, maxRows: 10 },
          }}
          rules={[
            ...rules,
            {
              validator: async (_, value) => {
                if (value) {
                  try {
                    JSON.parse(value);
                  } catch {
                    throw new Error('请输入有效的 JSON');
                  }
                }
              },
            },
          ]}
        />
      );

    default:
      return <ProFormText {...commonProps} />;
  }
}

// ---------------------------------------------------------------------------
// SchemaFormRenderer 组件
// ---------------------------------------------------------------------------

const SchemaFormRenderer: React.FC<SchemaFormRendererProps> = ({
  spec,
  initialValues,
  onFinish,
  onValuesChange,
  readonly = false,
  widgetMap,
}) => {
  // 解析 JSON Schema
  const schema = useMemo(() => {
    return (spec.jsonSchema || {}) as Record<string, unknown>;
  }, [spec.jsonSchema]);

  // 获取字段定义
  const properties = useMemo(() => {
    return (schema.properties || {}) as Record<string, Record<string, unknown>>;
  }, [schema.properties]);

  // 获取必填字段
  const requiredFields = useMemo(() => {
    return (schema.required || []) as string[];
  }, [schema.required]);

  // 构建字段列表
  const fields = useMemo(() => {
    const result: FormFieldSpec[] = [];

    // 从 spec.fields 获取字段配置
    if (spec.fields && spec.fields.length > 0) {
      spec.fields.forEach((field) => {
        const fieldSchema = properties[field.key] || {};
        result.push({
          ...field,
          required: field.required ?? requiredFields.includes(field.key),
        });
      });
    } else {
      // 从 schema 自动生成字段
      Object.keys(properties).forEach((key) => {
        const fieldSchema = properties[key];
        result.push({
          key,
          label: { 'zh-CN': (fieldSchema.title as string) || key },
          description: fieldSchema.description ? { 'zh-CN': fieldSchema.description as string } : undefined,
          required: requiredFields.includes(key),
          enumOptions: (fieldSchema.enum as unknown[])?.map((v) => ({
            value: String(v),
            label: { 'zh-CN': String(v) },
          })),
        });
      });
    }

    // 排序
    result.sort((a, b) => (a.order || 0) - (b.order || 0));

    return result;
  }, [spec.fields, properties, requiredFields]);

  // 按分组组织字段
  const groupedFields = useMemo(() => {
    if (!spec.groups || spec.groups.length === 0) {
      return [{ fields }];
    }

    const groups: Array<{ group: FormGroupSpec; fields: FormFieldSpec[] }> = [];
    const groupedKeys = new Set<string>();

    spec.groups.forEach((group) => {
      const groupFields = fields.filter((f) => group.fields.includes(f.key));
      group.fields.forEach((key) => groupedKeys.add(key));
      groups.push({ group, fields: groupFields });
    });

    // 未分组的字段
    const ungroupedFields = fields.filter((f) => !groupedKeys.has(f.key));
    if (ungroupedFields.length > 0) {
      groups.unshift({
        group: { key: '__ungrouped__', fields: [] },
        fields: ungroupedFields,
      });
    }

    return groups;
  }, [fields, spec.groups]);

  return (
    <ProForm
      layout={spec.layout || 'vertical'}
      initialValues={initialValues}
      onFinish={onFinish}
      onValuesChange={onValuesChange}
      submitter={
        readonly
          ? false
          : {
              submitButtonProps: {
                children: spec.submitButton?.text?.['zh-CN'] || '提交',
              },
              resetButtonProps: {
                children: spec.cancelButton?.text?.['zh-CN'] || '重置',
              },
            }
      }
    >
      {groupedFields.length === 1 && groupedFields[0].group.key === '__ungrouped__' ? (
        // 无分组
        groupedFields[0].fields.map((field) => {
          const fieldSchema = properties[field.key] || {};
          return renderSingleField(field, fieldSchema, readonly);
        })
      ) : (
        // 有分组
        <Collapse defaultActiveKey={spec.groups?.map((g) => g.key) || []}>
          {groupedFields.map(({ group, fields: groupFields }) => (
            <Panel
              key={group.key}
              header={group.title?.['zh-CN'] || group.title?.['en'] || group.key}
              collapsible={group.collapsible}
            >
              {groupFields.map((field) => {
                const fieldSchema = properties[field.key] || {};
                return renderSingleField(field, fieldSchema, readonly);
              })}
            </Panel>
          ))}
        </Collapse>
      )}
    </ProForm>
  );
};

export default SchemaFormRenderer;
