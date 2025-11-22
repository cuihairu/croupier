import React, { useEffect, useState, useCallback } from 'react';
import { Form, Input, InputNumber, Switch, Select, DatePicker, TimePicker, Rate, Slider, Cascader, TreeSelect, Checkbox, Radio, Upload, Button, Space, Typography, Card, Alert, Divider } from 'antd';
import { UploadOutlined, InfoCircleOutlined } from '@ant-design/icons';
import { renderXUIField, XUISchemaField } from '@/components/XUISchema';
import type { FormInstance } from 'antd';

const { TextArea } = Input;
const { RangePicker } = DatePicker;
const { Option } = Select;
const { Text, Paragraph } = Typography;

export type JSONSchemaProperty = {
  type: 'string' | 'number' | 'integer' | 'boolean' | 'array' | 'object';
  title?: string;
  description?: string;
  default?: any;
  enum?: any[];
  minLength?: number;
  maxLength?: number;
  minimum?: number;
  maximum?: number;
  pattern?: string;
  format?: string;
  items?: JSONSchemaProperty;
  properties?: Record<string, JSONSchemaProperty>;
  required?: string[];
};

export type JSONSchema = {
  type: 'object';
  properties?: Record<string, JSONSchemaProperty>;
  required?: string[];
  title?: string;
  description?: string;
};

export type FormUISchema = {
  fields?: Record<string, XUISchemaField>;
  'ui:layout'?: { type?: 'grid'; cols?: number };
  'ui:groups'?: Array<{ title?: string; fields: string[] }>;
  'ui:order'?: string[];
};

export interface FunctionFormRendererProps {
  schema: JSONSchema;
  uiSchema?: FormUISchema;
  initialValues?: Record<string, any>;
  onSubmit?: (values: any) => void;
  onChange?: (changedFields: any, allValues: any) => void;
  loading?: boolean;
  disabled?: boolean;
  compact?: boolean;
  showValidationErrors?: boolean;
  validateTrigger?: 'onChange' | 'onBlur' | 'onSubmit';
  submitText?: string;
  resetText?: string;
  showReset?: boolean;
  extra?: React.ReactNode;
}

export const FunctionFormRenderer: React.FC<FunctionFormRendererProps> = ({
  schema,
  uiSchema,
  initialValues = {},
  onSubmit,
  onChange,
  loading = false,
  disabled = false,
  compact = false,
  showValidationErrors = true,
  validateTrigger = 'onChange',
  submitText = '执行函数',
  resetText = '重置',
  showReset = true,
  extra
}) => {
  const [form] = Form.useForm();
  const [formData, setFormData] = useState(initialValues);

  useEffect(() => {
    form.setFieldsValue(initialValues);
    setFormData(initialValues);
  }, [initialValues, form]);

  const handleValuesChange = useCallback((changedFields: any, allValues: any) => {
    setFormData(allValues);
    onChange?.(changedFields, allValues);
  }, [onChange]);

  const handleFinish = useCallback((values: any) => {
    onSubmit?.(values);
  }, [onSubmit]);

  const handleReset = useCallback(() => {
    form.resetFields();
    setFormData(initialValues);
  }, [form, initialValues]);

  const validateField = useCallback((rule: any, value: any, property: JSONSchemaProperty) => {
    // String validation
    if (property.type === 'string') {
      if (property.minLength !== undefined && value && value.length < property.minLength) {
        return Promise.reject(new Error(`最小长度为 ${property.minLength}`));
      }
      if (property.maxLength !== undefined && value && value.length > property.maxLength) {
        return Promise.reject(new Error(`最大长度为 ${property.maxLength}`));
      }
      if (property.pattern && value && !new RegExp(property.pattern).test(value)) {
        return Promise.reject(new Error(`格式不正确`));
      }
    }

    // Number validation
    if (property.type === 'number' || property.type === 'integer') {
      if (property.minimum !== undefined && value !== undefined && value < property.minimum) {
        return Promise.reject(new Error(`最小值为 ${property.minimum}`));
      }
      if (property.maximum !== undefined && value !== undefined && value > property.maximum) {
        return Promise.reject(new Error(`最大值为 ${property.maximum}`));
      }
      if (property.type === 'integer' && value !== undefined && !Number.isInteger(value)) {
        return Promise.reject(new Error(`必须为整数`));
      }
    }

    // Array validation
    if (property.type === 'array' && value) {
      if (!Array.isArray(value)) {
        return Promise.reject(new Error(`必须为数组`));
      }
    }

    // Object validation
    if (property.type === 'object' && value) {
      if (typeof value !== 'object' || Array.isArray(value)) {
        return Promise.reject(new Error(`必须为对象`));
      }
    }

    return Promise.resolve();
  }, []);

  const renderFormItems = useCallback(() => {
    const items = [];
    const props = schema.properties || {};
    const required = schema.required || [];
    const uiFields = uiSchema?.fields || {};

    // Apply order if specified
    let fieldOrder = Object.keys(props);
    if (uiSchema?.['ui:order']) {
      fieldOrder = uiSchema['ui:order'];
    }

    // Handle grouped layout
    const groups = uiSchema?.['ui:groups'] || [];
    const layoutType = uiSchema?.['ui:layout']?.type || 'vertical';
    const cols = Math.max(1, Math.min(4, uiSchema?.['ui:layout']?.cols || 1));

    if (groups.length > 0) {
      // Grouped layout
      if (layoutType === 'tabs') {
        // TODO: Implement tabs layout if needed
      } else {
        // Section groups
        groups.forEach((group, groupIndex) => {
          if (group.title) {
            items.push(
              <Divider key={`group-title-${groupIndex}`} orientation="left">
                {group.title}
              </Divider>
            );
          }

          group.fields.forEach((fieldName) => {
            if (props[fieldName]) {
              items.push(
                renderXUIField(
                  fieldName,
                  props[fieldName],
                  uiFields[fieldName] || {},
                  formData,
                  form,
                  [fieldName],
                  required.includes(fieldName),
                  validateField
                )
              );
            }
          });
        });

        // Add remaining fields not in groups
        const groupedFields = new Set(groups.flatMap(g => g.fields));
        const remainingFields = fieldOrder.filter(field => !groupedFields.has(field));

        if (remainingFields.length > 0) {
          items.push(<Divider key="remaining-title" orientation="left">其他参数</Divider>);
          remainingFields.forEach((fieldName) => {
            if (props[fieldName]) {
              items.push(
                renderXUIField(
                  fieldName,
                  props[fieldName],
                  uiFields[fieldName] || {},
                  formData,
                  form,
                  [fieldName],
                  required.includes(fieldName),
                  validateField
                )
              );
            }
          });
        }
      }
    } else {
      // Standard layout
      fieldOrder.forEach((fieldName) => {
        if (props[fieldName]) {
          items.push(
            renderXUIField(
              fieldName,
              props[fieldName],
              uiFields[fieldName] || {},
              formData,
              form,
              [fieldName],
              required.includes(fieldName),
              validateField
            )
          );
        }
      });
    }

    return items;
  }, [schema, uiSchema, formData, form, validateField]);

  return (
    <Card size={compact ? 'small' : 'default'}>
      {schema.title && (
        <div style={{ marginBottom: 16 }}>
          <Text strong style={{ fontSize: 16 }}>{schema.title}</Text>
          {schema.description && (
            <Paragraph type="secondary" style={{ marginTop: 4, marginBottom: 0 }}>
              {schema.description}
            </Paragraph>
          )}
        </div>
      )}

      <Form
        form={form}
        layout="vertical"
        initialValues={initialValues}
        onValuesChange={handleValuesChange}
        onFinish={handleFinish}
        disabled={disabled}
        requiredMark={showValidationErrors}
        validateTrigger={validateTrigger}
      >
        {renderFormItems()}

        {extra && (
          <div style={{ marginTop: 16 }}>
            {extra}
          </div>
        )}

        <div style={{ marginTop: 24 }}>
          <Space>
            <Button
              type="primary"
              htmlType="submit"
              loading={loading}
              disabled={disabled}
            >
              {submitText}
            </Button>
            {showReset && (
              <Button
                onClick={handleReset}
                disabled={disabled || loading}
              >
                {resetText}
              </Button>
            )}
          </Space>
        </div>
      </Form>

      {showValidationErrors && (
        <Alert
          message="表单验证提示"
          description={
            <div>
              <p>• 标有红色星号(*)的字段为必填项</p>
              <p>• 表单会在输入时实时验证</p>
              <p>• 鼠标悬停在字段上可查看详细说明</p>
            </div>
          }
          type="info"
          showIcon
          style={{ marginTop: 16 }}
        />
      )}
    </Card>
  );
};

export default FunctionFormRenderer;