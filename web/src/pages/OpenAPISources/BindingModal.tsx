import React from 'react';
import { Alert, Input, Modal, Select, Space } from 'antd';
import type { OpenAPISourceOperation } from '@/services/api/openapi';

/** Provider binding 弹窗：选择已注册函数 + 可选 providerId/bindingId，
 * 表单值由页面持有（受控），保存经回调上抛。 */
export default function BindingModal({
  open,
  operation,
  bindingId,
  onBindingIdChange,
  functionId,
  onFunctionIdChange,
  providerId,
  onProviderIdChange,
  functionOptions,
  onCancel,
  onOk,
}: {
  open: boolean;
  operation: OpenAPISourceOperation | null;
  bindingId: string;
  onBindingIdChange: (value: string) => void;
  functionId: string | undefined;
  onFunctionIdChange: (value: string | undefined) => void;
  providerId: string;
  onProviderIdChange: (value: string) => void;
  functionOptions: Array<{ label: string; value: string }>;
  onCancel: () => void;
  onOk: () => void;
}) {
  return (
    <Modal
      title={operation ? `绑定 ${operation.operationId}` : '绑定 Provider'}
      open={open}
      onCancel={onCancel}
      onOk={onOk}
      okText="保存 binding"
    >
      <Space orientation="vertical" size={12} style={{ width: '100%' }}>
        <Alert
          type="info"
          showIcon
          message="当前只启用 Provider binding"
          description="httpConnector 需要 allowlist、SecretRef、超时/重试和审计策略后才能开放。"
        />
        <Input
          addonBefore="bindingId"
          value={bindingId}
          onChange={(event) => onBindingIdChange(event.target.value)}
        />
        <Select
          showSearch
          placeholder="选择已注册函数"
          value={functionId}
          onChange={onFunctionIdChange}
          options={functionOptions}
          optionFilterProp="label"
          style={{ width: '100%' }}
        />
        <Input
          addonBefore="providerId"
          placeholder="可选；留空由运行时按函数路由"
          value={providerId}
          onChange={(event) => onProviderIdChange(event.target.value)}
        />
      </Space>
    </Modal>
  );
}
