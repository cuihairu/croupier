/**
 * 表单布局渲染器
 *
 * 显示表单，用于数据提交。
 *
 * @module components/WorkspaceRenderer/renderers/FormRenderer
 */

import React, { useState } from 'react';
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
  Alert,
} from 'antd';
import type { FormLayout } from '@/types/workspace';
import { invokeFunction } from '@/services/functionInvoke';
import type { RendererProps } from './types';
import { RendererEmpty, RendererModeNotice, isTemplatePreviewContext } from './state';

export type FormRendererProps = RendererProps<FormLayout>;

/**
 * 表单布局渲染器组件
 */
export default function FormRenderer({ layout, objectKey, context }: FormRendererProps) {
  const isTemplatePreview = isTemplatePreviewContext(context);
  const [form] = Form.useForm();
  const [loading, setLoading] = useState(false);
  const effectiveFields = React.useMemo(() => {
    const fields = Array.isArray(layout.fields) ? layout.fields : [];
    if (fields.length > 0) return fields;
    if (!isTemplatePreview) return fields;
    return [
      { key: 'name', label: '名称', type: 'input', required: true, placeholder: '请输入名称' },
      { key: 'count', label: '数量', type: 'number', required: true },
    ] as any[];
  }, [isTemplatePreview, layout.fields]);

  if (!layout.submitFunction && !isTemplatePreview) {
    return (
      <RendererEmpty description="当前表单未绑定提交函数，请在布局配置中选择 submitFunction" />
    );
  }

  // 处理提交
  const handleSubmit = async (values: any) => {
    if (!layout.submitFunction) {
      if (isTemplatePreview) {
        message.info('模板预览模式：提交函数待绑定');
        return;
      }
      message.error('未配置提交函数');
      return;
    }

    setLoading(true);
    try {
      // 使用函数调用服务
      await invokeFunction(layout.submitFunction, values);

      message.success('提交成功');

      // 重置表单
      if (layout.showReset !== false) {
        form.resetFields();
      }
    } catch (error: any) {
      message.error(error.message || '提交失败');
    } finally {
      setLoading(false);
    }
  };

  // 处理重置
  const handleReset = () => {
    form.resetFields();
  };

  return (
    <>
      <RendererModeNotice
        context={context}
        sampleTitle="表单预览"
        sampleDescription="当前表单正在使用示例字段帮助你预览录入结构，正式运行仍需要真实 submitFunction。"
      />
      {!layout.submitFunction && isTemplatePreview && (
        <Alert
          type="info"
          showIcon
          style={{ marginBottom: 12 }}
          message="模板预览"
          description="当前表单未绑定提交函数，应用模板后可在编辑器中绑定。"
        />
      )}
      {!layout.submitFunction && !isTemplatePreview && (
        <RendererEmpty description="当前表单未绑定提交函数，请在布局配置中选择 submitFunction" />
      )}
      <Form
        form={form}
        layout={layout.formLayout || 'horizontal'}
        labelCol={{ span: 6 }}
        wrapperCol={{ span: 14 }}
        onFinish={handleSubmit}
        style={{ maxWidth: 800, margin: '20px auto' }}
      >
        {(effectiveFields || []).map((field) => (
          <Form.Item
            key={field.key}
            name={field.key}
            label={field.label}
            rules={[
              { required: field.required, message: `请输入${field.label}` },
              ...(field.rules || []).map((rule) => ({
                type: rule.type as any,
                pattern: rule.pattern ? new RegExp(rule.pattern) : undefined,
                min: rule.min,
                max: rule.max,
                message: rule.message,
              })),
            ]}
            tooltip={field.tooltip}
            initialValue={field.defaultValue}
          >
            {renderField(field)}
          </Form.Item>
        ))}

        <Form.Item wrapperCol={{ offset: 6, span: 14 }}>
          <Space>
            <Button type="primary" htmlType="submit" loading={loading} disabled={!layout.submitFunction && !isTemplatePreview}>
              {layout.submitText || '提交'}
            </Button>
            {layout.showReset !== false && <Button onClick={handleReset}>重置</Button>}
          </Space>
        </Form.Item>
      </Form>
    </>
  );
}

/**
 * 渲染表单字段
 */
function renderField(field: any): React.ReactNode {
  switch (field.type) {
    case 'input':
      return <Input placeholder={field.placeholder} disabled={field.disabled} />;

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
          {(field.options || []).map((opt: any) => (
            <Select.Option key={opt.value} value={opt.value}>
              {opt.label}
            </Select.Option>
          ))}
        </Select>
      );

    case 'date':
      return (
        <DatePicker
          placeholder={field.placeholder}
          disabled={field.disabled}
          style={{ width: '100%' }}
        />
      );

    case 'datetime':
      return (
        <DatePicker
          showTime
          placeholder={field.placeholder}
          disabled={field.disabled}
          style={{ width: '100%' }}
        />
      );

    case 'textarea':
      return <Input.TextArea rows={4} placeholder={field.placeholder} disabled={field.disabled} />;

    case 'switch':
      return <Switch disabled={field.disabled} />;

    case 'radio':
      return (
        <Radio.Group disabled={field.disabled}>
          {(field.options || []).map((opt: any) => (
            <Radio key={opt.value} value={opt.value}>
              {opt.label}
            </Radio>
          ))}
        </Radio.Group>
      );

    case 'checkbox':
      return (
        <Checkbox.Group disabled={field.disabled}>
          {(field.options || []).map((opt: any) => (
            <Checkbox key={opt.value} value={opt.value}>
              {opt.label}
            </Checkbox>
          ))}
        </Checkbox.Group>
      );

    default:
      return <Input placeholder={field.placeholder} disabled={field.disabled} />;
  }
}
