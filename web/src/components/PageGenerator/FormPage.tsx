/**
 * 表单页生成器
 *
 * 根据配置动态生成表单页面
 * 支持两种模式：
 * 1. 简化模式：使用 fields 配置
 * 2. 高级模式：使用 formilySchema（复用现有的 Formily）
 */

import React, { useState } from 'react';
import { PageContainer } from '@ant-design/pro-components';
import {
  Form,
  Input,
  InputNumber,
  Select,
  DatePicker,
  Switch,
  Radio,
  Checkbox,
  Button,
  Space,
  message,
} from 'antd';
import { useNavigate } from '@umijs/max';
import type { PageConfig, FormFieldConfig } from './types';
import SchemaRenderer from '@/components/formily/SchemaRenderer';

interface FormPageProps {
  config: PageConfig;
}

const FormPage: React.FC<FormPageProps> = ({ config }) => {
  const [form] = Form.useForm();
  const navigate = useNavigate();
  const [submitting, setSubmitting] = useState(false);

  const { fields, formilySchema, layout, labelCol, wrapperCol, submitText, resetText, showReset } =
    config.ui.form || {};

  // 如果使用 Formily Schema（高级模式）
  if (formilySchema) {
    return (
      <PageContainer title={config.title}>
        <SchemaRenderer
          schema={formilySchema}
          onSubmit={handleFormilySubmit}
          loading={submitting}
        />
      </PageContainer>
    );
  }

  // 简化模式：使用 fields 配置
  const renderField = (field: FormFieldConfig) => {
    switch (field.type) {
      case 'input':
        return <Input placeholder={field.placeholder} disabled={field.disabled} />;

      case 'textarea':
        return (
          <Input.TextArea placeholder={field.placeholder} disabled={field.disabled} rows={4} />
        );

      case 'number':
        return (
          <InputNumber
            placeholder={field.placeholder}
            disabled={field.disabled}
            style={{ width: '100%' }}
          />
        );

      case 'select':
        return (
          <Select placeholder={field.placeholder} disabled={field.disabled}>
            {field.options?.map((opt) => (
              <Select.Option key={opt.value} value={opt.value}>
                {opt.label}
              </Select.Option>
            ))}
          </Select>
        );

      case 'date':
        return <DatePicker disabled={field.disabled} style={{ width: '100%' }} />;

      case 'switch':
        return <Switch disabled={field.disabled} />;

      case 'radio':
        return (
          <Radio.Group disabled={field.disabled}>
            {field.options?.map((opt) => (
              <Radio key={opt.value} value={opt.value}>
                {opt.label}
              </Radio>
            ))}
          </Radio.Group>
        );

      case 'checkbox':
        return (
          <Checkbox.Group disabled={field.disabled}>
            {field.options?.map((opt) => (
              <Checkbox key={opt.value} value={opt.value}>
                {opt.label}
              </Checkbox>
            ))}
          </Checkbox.Group>
        );

      default:
        return <Input />;
    }
  };

  // 生成校验规则
  const generateRules = (field: FormFieldConfig) => {
    const rules: any[] = [];

    if (field.required) {
      rules.push({
        required: true,
        message: `请输入${field.label}`,
      });
    }

    if (field.rules) {
      field.rules.forEach((rule) => {
        if (rule.type === 'email') {
          rules.push({
            type: 'email',
            message: rule.message || '请输入有效的邮箱地址',
          });
        } else if (rule.type === 'url') {
          rules.push({
            type: 'url',
            message: rule.message || '请输入有效的URL',
          });
        } else if (rule.type === 'pattern' && rule.pattern) {
          rules.push({
            pattern: new RegExp(rule.pattern),
            message: rule.message || '格式不正确',
          });
        } else if (rule.min !== undefined) {
          rules.push({
            min: rule.min,
            message: rule.message || `最小长度为 ${rule.min}`,
          });
        } else if (rule.max !== undefined) {
          rules.push({
            max: rule.max,
            message: rule.message || `最大长度为 ${rule.max}`,
          });
        }
      });
    }

    return rules;
  };

  // 处理表单提交
  const handleSubmit = async (values: any) => {
    setSubmitting(true);

    try {
      // 调用数据源
      if (config.dataSource.type === 'function') {
        const { invokeFunction } = await import('@/services/api');
        await invokeFunction(config.dataSource.functionId!, values);
      } else if (config.dataSource.type === 'api') {
        const response = await fetch(config.dataSource.apiEndpoint!, {
          method: config.dataSource.method || 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify(values),
        });
        if (!response.ok) throw new Error('提交失败');
      }

      message.success('提交成功');
      form.resetFields();

      // 处理成功后的行为
      // TODO: 支持配置化的成功后行为
    } catch (error: any) {
      message.error(error.message || '提交失败');
      console.error('Form submit error:', error);
    } finally {
      setSubmitting(false);
    }
  };

  // 处理 Formily 表单提交
  const handleFormilySubmit = async (values: any) => {
    await handleSubmit(values);
  };

  return (
    <PageContainer title={config.title}>
      <Form
        form={form}
        layout={layout || 'horizontal'}
        labelCol={labelCol || { span: 6 }}
        wrapperCol={wrapperCol || { span: 14 }}
        onFinish={handleSubmit}
        initialValues={
          fields?.reduce((acc, field) => {
            if (field.defaultValue !== undefined) {
              acc[field.key] = field.defaultValue;
            }
            return acc;
          }, {} as any) || {}
        }
      >
        {fields?.map((field) => (
          <Form.Item
            key={field.key}
            name={field.key}
            label={field.label}
            rules={generateRules(field)}
            tooltip={field.tooltip}
          >
            {renderField(field)}
          </Form.Item>
        ))}

        <Form.Item wrapperCol={{ offset: labelCol?.span || 6, span: wrapperCol?.span || 14 }}>
          <Space>
            <Button type="primary" htmlType="submit" loading={submitting}>
              {submitText || '提交'}
            </Button>
            {showReset !== false && (
              <Button onClick={() => form.resetFields()}>{resetText || '重置'}</Button>
            )}
          </Space>
        </Form.Item>
      </Form>
    </PageContainer>
  );
};

export default FormPage;
