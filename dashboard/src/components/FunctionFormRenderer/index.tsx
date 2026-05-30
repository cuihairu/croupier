import React, { useEffect, useState, useCallback, useRef } from 'react';
import {
  Form,
  Input,
  InputNumber,
  Switch,
  Select,
  DatePicker,
  TimePicker,
  Rate,
  Slider,
  Cascader,
  TreeSelect,
  Checkbox,
  Radio,
  Upload,
  Button,
  Space,
  Typography,
  Card,
  Alert,
  Divider,
  Tabs,
} from 'antd';
import { UploadOutlined, InfoCircleOutlined } from '@ant-design/icons';
import { renderXUIField, XUISchemaField } from '@/components/XUISchema';
import type { FormInstance } from 'antd';

const { TextArea } = Input;
const { RangePicker } = DatePicker;
const { Option } = Select;
const { Text, Paragraph } = Typography;
const EMPTY_INITIAL_VALUES: Record<string, any> = {};
const toStableJSON = (v: any) => {
  try {
    return JSON.stringify(v ?? {});
  } catch {
    return '';
  }
};

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
  'ui:layout'?: { type?: 'grid' | 'tabs'; cols?: number };
  'ui:groups'?: Array<{ title?: string; fields: string[] }>;
  'ui:order'?: string[];
};

export interface FunctionFormRendererProps {
  schema: JSONSchema;
  uiSchema?: FormUISchema;
  initialValues?: Record<string, any>;
  onSubmit?: (values: any) => void;
  onSecondarySubmit?: (values: any) => void;
  onChange?: (changedFields: any, allValues: any) => void;
  loading?: boolean;
  secondaryLoading?: boolean;
  disabled?: boolean;
  compact?: boolean;
  showValidationErrors?: boolean;
  validateTrigger?: 'onChange' | 'onBlur' | 'onSubmit';
  submitText?: string;
  secondarySubmitText?: string;
  resetText?: string;
  showReset?: boolean;
  extra?: React.ReactNode;
  enableRawJson?: boolean;
}

export const FunctionFormRenderer: React.FC<FunctionFormRendererProps> = ({
  schema,
  uiSchema,
  initialValues = EMPTY_INITIAL_VALUES,
  onSubmit,
  onSecondarySubmit,
  onChange,
  loading = false,
  secondaryLoading = false,
  disabled = false,
  compact = false,
  showValidationErrors = true,
  validateTrigger = 'onChange',
  submitText = '执行函数',
  secondarySubmitText = '作为任务执行',
  resetText = '重置',
  showReset = true,
  extra,
  enableRawJson = true,
}) => {
  const [form] = Form.useForm();
  const [formData, setFormData] = useState(initialValues);
  const [inputMode, setInputMode] = useState<'form' | 'json'>('form');
  const [rawJson, setRawJson] = useState<string>('');
  const [rawJsonError, setRawJsonError] = useState<string>('');
  const initialValuesRef = useRef<string>(toStableJSON(initialValues));

  useEffect(() => {
    const nextKey = toStableJSON(initialValues);
    if (nextKey === initialValuesRef.current) return;
    initialValuesRef.current = nextKey;
    form.setFieldsValue(initialValues);
    setFormData(initialValues);
  }, [initialValues, form]);

  useEffect(() => {
    const hasFields = schema?.properties && Object.keys(schema.properties || {}).length > 0;
    if (enableRawJson && !hasFields) {
      setInputMode((prev) => (prev === 'json' ? prev : 'json'));
    } else if (!enableRawJson) {
      setInputMode((prev) => (prev === 'form' ? prev : 'form'));
    }
  }, [schema, enableRawJson]);

  const handleValuesChange = useCallback(
    (changedFields: any, allValues: any) => {
      setFormData(allValues);
      onChange?.(changedFields, allValues);
    },
    [onChange],
  );

  const handleFinish = useCallback(
    (values: any) => {
      if (enableRawJson && inputMode === 'json') {
        const text = (rawJson || '').trim();
        if (!text) {
          setRawJsonError('请输入 JSON 参数（或切换回表单模式）');
          return;
        }
        try {
          const parsed = JSON.parse(text);
          setRawJsonError('');
          onSubmit?.(parsed);
        } catch (e: any) {
          setRawJsonError(e?.message || 'JSON 解析失败');
        }
        return;
      }
      setRawJsonError('');
      onSubmit?.(values);
    },
    [enableRawJson, inputMode, rawJson, onSubmit],
  );

  const buildPayloadFromCurrent = useCallback(async (): Promise<any | undefined> => {
    if (enableRawJson && inputMode === 'json') {
      const text = (rawJson || '').trim();
      if (!text) {
        setRawJsonError('请输入 JSON 参数（或切换回表单模式）');
        return undefined;
      }
      try {
        const parsed = JSON.parse(text);
        setRawJsonError('');
        return parsed;
      } catch (e: any) {
        setRawJsonError(e?.message || 'JSON 解析失败');
        return undefined;
      }
    }
    setRawJsonError('');
    const values = await form.validateFields();
    return values;
  }, [enableRawJson, inputMode, rawJson, form]);

  const handleSecondaryClick = useCallback(async () => {
    if (!onSecondarySubmit) return;
    try {
      const payload = await buildPayloadFromCurrent();
      if (payload === undefined) return;
      onSecondarySubmit(payload);
    } catch {
      // antd Form will show field errors
    }
  }, [onSecondarySubmit, buildPayloadFromCurrent]);

  const handleReset = useCallback(() => {
    form.resetFields();
    setFormData(initialValues);
    setRawJson('');
    setRawJsonError('');
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
    const schemaKeys = Object.keys(props);
    let fieldOrder = schemaKeys;
    if (Array.isArray(uiSchema?.['ui:order']) && uiSchema['ui:order'].length > 0) {
      const uiOrder = uiSchema['ui:order'].filter((k) =>
        Object.prototype.hasOwnProperty.call(props, k),
      );
      const remaining = schemaKeys.filter((k) => !uiOrder.includes(k));
      fieldOrder = [...uiOrder, ...remaining];
    }
    if (fieldOrder.length === 0) {
      fieldOrder = schemaKeys;
    }

    // Handle grouped layout
    const groups = uiSchema?.['ui:groups'] || [];
    const layoutType = uiSchema?.['ui:layout']?.type || 'vertical';
    const cols = Math.max(1, Math.min(4, uiSchema?.['ui:layout']?.cols || 1));

    if (groups.length > 0) {
      // Grouped layout
      if (layoutType === 'tabs') {
        // Tabs layout implementation
        const tabItems = groups.map((group, groupIndex) => {
          const tabFields: React.ReactNode[] = [];

          group.fields.forEach((fieldName) => {
            if (props[fieldName]) {
              tabFields.push(
                renderXUIField(
                  fieldName,
                  props[fieldName],
                  uiFields[fieldName] || {},
                  formData,
                  form,
                  [fieldName],
                  required.includes(fieldName),
                  validateField,
                ),
              );
            }
          });

          return {
            key: `tab-${groupIndex}`,
            label: group.title || `标签 ${groupIndex + 1}`,
            children: <div style={{ paddingTop: 16 }}>{tabFields}</div>,
          };
        });

        // Add remaining fields to a separate tab
        const groupedFields = new Set(groups.flatMap((g) => g.fields));
        const remainingFields = fieldOrder.filter((field) => !groupedFields.has(field));

        if (remainingFields.length > 0) {
          const remainingTabFields: React.ReactNode[] = [];

          remainingFields.forEach((fieldName) => {
            if (props[fieldName]) {
              remainingTabFields.push(
                renderXUIField(
                  fieldName,
                  props[fieldName],
                  uiFields[fieldName] || {},
                  formData,
                  form,
                  [fieldName],
                  required.includes(fieldName),
                  validateField,
                ),
              );
            }
          });

          tabItems.push({
            key: 'tab-remaining',
            label: '其他参数',
            children: <div style={{ paddingTop: 16 }}>{remainingTabFields}</div>,
          });
        }

        // Render tabs
        items.push(
          <Tabs
            key="form-tabs"
            defaultActiveKey={tabItems[0]?.key}
            items={tabItems}
            size="small"
            style={{ marginBottom: 16 }}
          />,
        );
      } else {
        // Section groups
        groups.forEach((group, groupIndex) => {
          if (group.title) {
            items.push(
              <Divider key={`group-title-${groupIndex}`} orientation="left">
                {group.title}
              </Divider>,
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
                  validateField,
                ),
              );
            }
          });
        });

        // Add remaining fields not in groups
        const groupedFields = new Set(groups.flatMap((g) => g.fields));
        const remainingFields = fieldOrder.filter((field) => !groupedFields.has(field));

        if (remainingFields.length > 0) {
          items.push(
            <Divider key="remaining-title" orientation="left">
              其他参数
            </Divider>,
          );
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
                  validateField,
                ),
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
              validateField,
            ),
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
          <Text strong style={{ fontSize: 16 }}>
            {schema.title}
          </Text>
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
        {enableRawJson ? (
          <Tabs
            activeKey={inputMode}
            onChange={(k) => {
              setInputMode(k as any);
              setRawJsonError('');
            }}
            size="small"
            items={[
              {
                key: 'form',
                label: '表单',
                children: renderFormItems(),
              },
              {
                key: 'json',
                label: 'JSON',
                children: (
                  <div>
                    <Text type="secondary">直接输入要发送的请求参数（JSON）。</Text>
                    <TextArea
                      rows={10}
                      placeholder='例如：{"player_id":"123","reason":"test"}'
                      value={rawJson}
                      onChange={(e) => {
                        setRawJson(e.target.value);
                        setRawJsonError('');
                      }}
                      style={{ marginTop: 8, fontFamily: 'monospace' }}
                    />
                    {rawJsonError && (
                      <Alert
                        style={{ marginTop: 8 }}
                        type="error"
                        showIcon
                        message="JSON 无效"
                        description={rawJsonError}
                      />
                    )}
                  </div>
                ),
              },
            ]}
          />
        ) : (
          renderFormItems()
        )}

        {extra && <div style={{ marginTop: 16 }}>{extra}</div>}

        <div style={{ marginTop: 24 }}>
          <Space>
            <Button type="primary" htmlType="submit" loading={loading} disabled={disabled}>
              {submitText}
            </Button>
            {onSecondarySubmit && (
              <Button
                onClick={handleSecondaryClick}
                loading={secondaryLoading}
                disabled={disabled || loading}
              >
                {secondarySubmitText}
              </Button>
            )}
            {showReset && (
              <Button onClick={handleReset} disabled={disabled || loading}>
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
