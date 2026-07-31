/**
 * OperationPageRenderer - 操作页面渲染器
 *
 * 渲染独立操作页面，包括：
 * - 表单输入
 * - 确认对话框
 * - 结果展示
 *
 * @module components/PageRenderer/OperationPageRenderer
 */

import React, { useState, useCallback } from 'react';
import {
  ProForm,
  ProFormText,
  ProFormTextArea,
  ProFormSelect,
  ProFormDigit,
  ProFormSwitch,
  ProFormDatePicker,
} from '@ant-design/pro-components';
import {
  Card,
  Button,
  Space,
  Modal,
  message,
  Result,
  Typography,
  Descriptions,
  Tag,
} from 'antd';
import {
  PlayCircleOutlined,
  CheckCircleOutlined,
  CloseCircleOutlined,
} from '@ant-design/icons';
import type {
  OperationPageSpec,
  FormFieldSpec,
  PageFunctionBindingV2,
  ResultViewSpec,
} from '@/types/dashboard-vnext';

const { Text } = Typography;

// ---------------------------------------------------------------------------
// Props
// ---------------------------------------------------------------------------

export interface OperationPageRendererProps {
  /** 操作页面规格 */
  spec: OperationPageSpec;
  /** 页面绑定 */
  bindings: PageFunctionBindingV2[];
  /** 执行绑定函数 */
  onExecute: (bindingId: string, payload: unknown) => Promise<unknown>;
  /** 页面标题 */
  title?: string;
}

// ---------------------------------------------------------------------------
// 表单字段渲染
// ---------------------------------------------------------------------------

function renderFormField(field: FormFieldSpec): React.ReactNode {
  const label = field.label?.['zh-CN'] || field.label?.['en'] || field.key;
  const placeholder = field.placeholder?.['zh-CN'] || field.placeholder?.['en'];
  const required = field.required;
  const rules = required ? [{ required: true, message: `请输入${label}` }] : [];

  switch (field.widget) {
    case 'TextArea':
      return (
        <ProFormTextArea
          key={field.key}
          name={field.key}
          label={label}
          placeholder={placeholder}
          rules={rules}
          fieldProps={{ disabled: field.disabled }}
        />
      );
    case 'InputNumber':
      return (
        <ProFormDigit
          key={field.key}
          name={field.key}
          label={label}
          placeholder={placeholder}
          rules={rules}
          fieldProps={{ disabled: field.disabled }}
        />
      );
    case 'Switch':
      return (
        <ProFormSwitch
          key={field.key}
          name={field.key}
          label={label}
          fieldProps={{ disabled: field.disabled }}
        />
      );
    case 'Select':
      return (
        <ProFormSelect
          key={field.key}
          name={field.key}
          label={label}
          placeholder={placeholder}
          rules={rules}
          options={field.enumOptions?.map((opt) => ({
            label: opt.label['zh-CN'] || opt.label['en'] || opt.value,
            value: opt.value,
          }))}
          fieldProps={{ disabled: field.disabled }}
        />
      );
    case 'DatePicker':
      return (
        <ProFormDatePicker
          key={field.key}
          name={field.key}
          label={label}
          placeholder={placeholder}
          rules={rules}
          fieldProps={{ disabled: field.disabled }}
        />
      );
    default:
      return (
        <ProFormText
          key={field.key}
          name={field.key}
          label={label}
          placeholder={placeholder}
          rules={rules}
          fieldProps={{ disabled: field.disabled, type: field.widget === 'Password' ? 'password' : 'text' }}
        />
      );
  }
}

// ---------------------------------------------------------------------------
// 结果渲染
// ---------------------------------------------------------------------------

function renderResult(result: unknown, resultView?: ResultViewSpec): React.ReactNode {
  if (!result) {
    return null;
  }

  const data = result as any;

  // 如果有自定义字段
  if (resultView?.fields && resultView.fields.length > 0) {
    return (
      <Descriptions column={1} bordered>
        {resultView.fields.map((field) => (
          <Descriptions.Item
            key={field.key}
            label={field.title['zh-CN'] || field.title['en'] || field.key}
          >
            {data[field.key]?.toString() || '-'}
          </Descriptions.Item>
        ))}
      </Descriptions>
    );
  }

  // 默认 JSON 展示
  return (
    <pre style={{ maxHeight: 400, overflow: 'auto' }}>
      {JSON.stringify(data, null, 2)}
    </pre>
  );
}

// ---------------------------------------------------------------------------
// OperationPageRenderer 组件
// ---------------------------------------------------------------------------

const OperationPageRenderer: React.FC<OperationPageRendererProps> = ({
  spec,
  bindings,
  onExecute,
  title,
}) => {
  const [loading, setLoading] = useState(false);
  const [result, setResult] = useState<unknown>(null);
  const [error, setError] = useState<string | null>(null);
  const [confirmVisible, setConfirmVisible] = useState(false);
  const [pendingValues, setPendingValues] = useState<unknown>(null);

  // 查找主绑定
  const mainBinding = bindings.find((b) => b.usage === 'action') || bindings[0];

  // 处理表单提交
  const handleSubmit = useCallback(
    async (values: unknown) => {
      if (!mainBinding) {
        message.error('未配置操作绑定');
        return;
      }

      // 如果需要确认
      if (spec.confirm) {
        setPendingValues(values);
        setConfirmVisible(true);
        return;
      }

      // 直接执行
      setLoading(true);
      setError(null);
      setResult(null);

      try {
        const response = await onExecute(mainBinding.id, values);
        setResult(response);

        if (spec.resultView?.successMessage) {
          message.success(
            spec.resultView.successMessage['zh-CN'] || '操作成功'
          );
        } else {
          message.success('操作成功');
        }
      } catch (err: any) {
        setError(err.message || '操作失败');

        if (spec.resultView?.errorMessage) {
          message.error(
            spec.resultView.errorMessage['zh-CN'] || '操作失败'
          );
        } else {
          message.error('操作失败');
        }
      } finally {
        setLoading(false);
      }
    },
    [mainBinding, spec.confirm, spec.resultView, onExecute]
  );

  // 处理确认后执行
  const handleConfirm = useCallback(async () => {
    if (!mainBinding || !pendingValues) {
      return;
    }

    setConfirmVisible(false);
    setLoading(true);
    setError(null);
    setResult(null);

    try {
      const response = await onExecute(mainBinding.id, pendingValues);
      setResult(response);
      message.success('操作成功');
    } catch (err: any) {
      setError(err.message || '操作失败');
      message.error('操作失败');
    } finally {
      setLoading(false);
      setPendingValues(null);
    }
  }, [mainBinding, pendingValues, onExecute]);

  // 重置
  const handleReset = useCallback(() => {
    setResult(null);
    setError(null);
  }, []);

  return (
    <div>
      {/* 表单 */}
      <Card title={title || '执行操作'}>
        <ProForm
          onFinish={handleSubmit}
          submitter={{
            submitButtonProps: { loading },
            resetButtonProps: { onClick: handleReset },
          }}
        >
          {spec.form.fields?.map(renderFormField)}
        </ProForm>
      </Card>

      {/* 确认对话框 */}
      {spec.confirm && (
        <Modal
          title={spec.confirm.title['zh-CN'] || '确认操作'}
          open={confirmVisible}
          onOk={handleConfirm}
          onCancel={() => {
            setConfirmVisible(false);
            setPendingValues(null);
          }}
          okText={spec.confirm.confirmText['zh-CN'] || '确定'}
          cancelText={spec.confirm.cancelText?.['zh-CN'] || '取消'}
          confirmLoading={loading}
        >
          {spec.confirm.description && (
            <p>{spec.confirm.description['zh-CN']}</p>
          )}
          {pendingValues && (
            <pre style={{ maxHeight: 200, overflow: 'auto', background: '#f5f5f5', padding: 12 }}>
              {JSON.stringify(pendingValues, null, 2)}
            </pre>
          )}
        </Modal>
      )}

      {/* 结果展示 */}
      {(result || error) && (
        <Card title="执行结果" style={{ marginTop: 16 }}>
          {error ? (
            <Result
              status="error"
              title="操作失败"
              subTitle={error}
              icon={<CloseCircleOutlined />}
            />
          ) : (
            <Result
              status="success"
              title="操作成功"
              icon={<CheckCircleOutlined />}
              extra={renderResult(result, spec.resultView)}
            />
          )}
        </Card>
      )}
    </div>
  );
};

export default OperationPageRenderer;
