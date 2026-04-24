import React, { useState, useEffect } from 'react';
import {
  Modal,
  Form,
  Input,
  Divider,
  Button,
  Space,
  Typography,
  App,
  Tabs,
  Alert,
  Card,
} from 'antd';
import type { FormInstance } from 'antd/es/form';
import FormRender from 'form-render';
import {
  CheckCircleOutlined,
  ExclamationCircleOutlined,
  EditOutlined,
  CodeOutlined,
  FormOutlined,
} from '@ant-design/icons';
import JSONSchemaEditor from '@/components/JSONSchemaEditor';
import UISchemaEditor from '@/components/UISchemaEditor';

const { Text } = Typography;
const { TextArea } = Input;

export interface EntityValidationResult {
  valid: boolean;
  errors?: string[];
  warnings?: string[];
}

export interface XEntityFormProps<T = any> {
  // Modal props
  visible: boolean;
  onCancel: () => void;
  title?: string;
  width?: number;

  // Entity data
  entity?: T | null;
  onSubmit: (data: any) => Promise<void>;
  onValidate?: (data: any) => Promise<EntityValidationResult>;

  // Form configuration
  basicFields?: Array<{
    name: string;
    label: string;
    required?: boolean;
    type?: 'input' | 'textarea' | 'array';
    placeholder?: string;
    rules?: any[];
  }>;

  // Schema editing
  enableSchemaEditing?: boolean;
  schemaFormSchema?: any;
  schemaLabel?: string;
  uiSchemaLabel?: string;

  // Customization
  submitButtonText?: string;
  validateButtonText?: string;
  cancelButtonText?: string;
  successMessage?: string;
  failureMessage?: string;
  extraFooterButtons?: React.ReactNode[];

  // Optional custom content area rendered after basic fields.
  // Provides access to inner antd Form instance for advanced use cases
  // such as file upload components that set form values.
  customContent?: (form: FormInstance) => React.ReactNode;

  // Initial values transformer
  getInitialValues?: (entity: T) => any;
  getSchemaData?: (entity: T) => any;
  getUiSchemaData?: (entity: T) => any;
}

export default function XEntityForm<T = any>({
  visible,
  onCancel,
  title,
  width = 800,
  entity,
  onSubmit,
  onValidate,
  basicFields = [],
  enableSchemaEditing = false,
  schemaFormSchema,
  schemaLabel = 'Schema',
  uiSchemaLabel = 'UI Schema',
  submitButtonText,
  validateButtonText = 'Validate',
  cancelButtonText = 'Cancel',
  successMessage,
  failureMessage,
  extraFooterButtons = [],
  customContent,
  getInitialValues,
  getSchemaData,
  getUiSchemaData,
}: XEntityFormProps<T>) {
  const [form] = Form.useForm();
  // Use App context message API to avoid calling static message in render
  const { message } = App.useApp();
  const [schemaData, setSchemaData] = useState<any>({});
  const [uiSchemaData, setUiSchemaData] = useState<any>({});
  const [validationResult, setValidationResult] = useState<EntityValidationResult | null>(null);
  const [loading, setLoading] = useState(false);
  const [previewData, setPreviewData] = useState<Record<string, any>>({});

  const isEditing = !!entity;

  // Reset form when entity changes or modal opens/closes
  useEffect(() => {
    if (visible) {
      if (entity && getInitialValues) {
        form.setFieldsValue(getInitialValues(entity));
      } else {
        form.resetFields();
      }

      if (entity && getSchemaData) {
        setSchemaData(getSchemaData(entity));
      } else {
        setSchemaData({});
      }

      if (entity && getUiSchemaData) {
        setUiSchemaData(getUiSchemaData(entity));
      } else {
        setUiSchemaData({});
      }

      setValidationResult(null);
      setPreviewData({});
    }
  }, [visible, entity, form, getInitialValues, getSchemaData, getUiSchemaData]);

  const handleValidate = async () => {
    if (!onValidate) return;

    try {
      const values = await form.validateFields();
      setLoading(true);

      const entityData = {
        ...values,
        ...(enableSchemaEditing && {
          schema: schemaData,
          uiSchema: uiSchemaData,
        }),
      };

      const result = await onValidate(entityData);
      setValidationResult(result);

      if (result.valid) {
        message.success('Validation passed');
      } else {
        message.warning('Validation failed');
      }
    } catch (error: any) {
      message.error(error?.message || 'Validation failed');
    } finally {
      setLoading(false);
    }
  };

  const handleSubmit = async () => {
    try {
      const values = await form.validateFields();
      setLoading(true);

      const entityData = {
        ...values,
        ...(enableSchemaEditing && {
          schema: schemaData,
          uiSchema: uiSchemaData,
        }),
      };

      await onSubmit(entityData);
      message.success(successMessage || `${isEditing ? 'Updated' : 'Created'} successfully`);
      onCancel();
    } catch (error: any) {
      message.error(
        error?.message || failureMessage || `Failed to ${isEditing ? 'update' : 'create'}`,
      );
    } finally {
      setLoading(false);
    }
  };

  // Build footer buttons
  const footerButtons = [
    ...extraFooterButtons,
    ...(onValidate
      ? [
          <Button key="validate" onClick={handleValidate} loading={loading}>
            {validateButtonText}
          </Button>,
        ]
      : []),
    <Button key="cancel" onClick={onCancel}>
      {cancelButtonText}
    </Button>,
    <Button key="submit" type="primary" onClick={handleSubmit} loading={loading}>
      {submitButtonText || (isEditing ? 'Update' : 'Create')}
    </Button>,
  ];

  const resolveUISchema = (data: any) => {
    if (!data || typeof data !== 'object') {
      return undefined;
    }
    if (data.fields && typeof data.fields === 'object') {
      return data.fields;
    }
    if (data.properties && typeof data.properties === 'object') {
      return data.properties;
    }
    return data;
  };
  const previewSchema = schemaData && typeof schemaData === 'object' ? schemaData : {};
  const previewSchemaProperties =
    previewSchema?.properties && typeof previewSchema.properties === 'object'
      ? previewSchema.properties
      : {};
  const hasPreviewFields = Object.keys(previewSchemaProperties || {}).length > 0;
  const previewUISchema = resolveUISchema(uiSchemaData);

  return (
    <Modal
      title={title || (isEditing ? 'Edit Entity' : 'Create Entity')}
      open={visible}
      onCancel={onCancel}
      width={width}
      footer={footerButtons}
    >
      {/* Basic Fields Form */}
      <Form form={form} layout="vertical">
        {basicFields.map((field) => {
          const { name, label, required, type = 'input', placeholder, rules = [] } = field;
          const fieldRules = required
            ? [{ required: true, message: `Please enter ${label.toLowerCase()}` }, ...rules]
            : rules;

          let inputComponent;
          switch (type) {
            case 'textarea':
              inputComponent = <TextArea rows={2} placeholder={placeholder} />;
              break;
            case 'array':
              inputComponent = <Input placeholder={placeholder} />;
              break;
            default:
              inputComponent = <Input placeholder={placeholder} />;
          }

          return (
            <Form.Item key={name} name={name} label={label} rules={fieldRules}>
              {inputComponent}
            </Form.Item>
          );
        })}
        {/* Custom content hook (e.g., uploaders, previews). Placed inside Form so Form.Item can register fields. */}
        {typeof customContent === 'function' ? customContent(form) : null}
      </Form>

      {/* Schema Editing Section */}
      {enableSchemaEditing && (
        <>
          <Divider orientation="left">
            <Space>
              <EditOutlined />
              Schema & UI Configuration
            </Space>
          </Divider>

          <Tabs
            defaultActiveKey="schema"
            items={[
              {
                key: 'schema',
                label: (
                  <span>
                    <EditOutlined />
                    JSON Schema
                  </span>
                ),
                children: <JSONSchemaEditor value={schemaData} onChange={setSchemaData} />,
              },
              {
                key: 'ui',
                label: (
                  <span>
                    <FormOutlined />
                    UI Schema
                  </span>
                ),
                children: (
                  <UISchemaEditor
                    value={uiSchemaData}
                    onChange={setUiSchemaData}
                    jsonSchema={schemaData}
                  />
                ),
              },
              {
                key: 'preview',
                label: (
                  <span>
                    <CodeOutlined />
                    Preview
                  </span>
                ),
                children: (
                  <Space direction="vertical" style={{ width: '100%' }}>
                    <Alert
                      message="Preview"
                      description="This is how the form will appear with your schema configuration"
                      type="info"
                    />
                    <Card size="small" title="Form Preview">
                      {hasPreviewFields ? (
                        <FormRender
                          schema={previewSchema}
                          uiSchema={previewUISchema || {}}
                          formData={previewData}
                          onChange={setPreviewData}
                          displayType="row"
                          labelWidth={120}
                        />
                      ) : (
                        <Alert
                          message="Schema is empty"
                          description="Add fields to the JSON Schema to preview the form."
                          type="warning"
                          showIcon
                        />
                      )}
                    </Card>
                    <Card size="small" title="JSON Schema">
                      <pre style={{ fontSize: 12, background: '#f5f5f5', padding: 8 }}>
                        {JSON.stringify(schemaData, null, 2)}
                      </pre>
                    </Card>
                    <Card size="small" title="UI Schema">
                      <pre style={{ fontSize: 12, background: '#f5f5f5', padding: 8 }}>
                        {JSON.stringify(uiSchemaData, null, 2)}
                      </pre>
                    </Card>
                  </Space>
                ),
              },
            ]}
          />
        </>
      )}

      {/* Validation Result */}
      {validationResult && (
        <div style={{ marginTop: 16 }}>
          <Divider>Validation Result</Divider>
          <Space direction="vertical" style={{ width: '100%' }}>
            <Text>
              {validationResult.valid ? (
                <>
                  <CheckCircleOutlined style={{ color: 'green' }} /> Valid
                </>
              ) : (
                <>
                  <ExclamationCircleOutlined style={{ color: 'orange' }} /> Invalid
                </>
              )}
            </Text>
            {validationResult.errors?.length && (
              <div>
                <Text type="danger">Errors:</Text>
                <ul>
                  {validationResult.errors.map((error, idx) => (
                    <li key={idx}>
                      <Text type="danger">{error}</Text>
                    </li>
                  ))}
                </ul>
              </div>
            )}
            {validationResult.warnings?.length && (
              <div>
                <Text type="warning">Warnings:</Text>
                <ul>
                  {validationResult.warnings.map((warning, idx) => (
                    <li key={idx}>
                      <Text type="warning">{warning}</Text>
                    </li>
                  ))}
                </ul>
              </div>
            )}
          </Space>
        </div>
      )}
    </Modal>
  );
}
