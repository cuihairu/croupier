import React from 'react';
import { Alert, Input, Modal, Select, Space } from 'antd';
import { useIntl } from '@umijs/max';
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
  const intl = useIntl();

  return (
    <Modal
      title={
        operation
          ? intl.formatMessage(
              {
                id: 'pages.openapiSources.bindingModal.title.withOperation',
                defaultMessage: '绑定 {operationId}',
              },
              { operationId: operation.operationId },
            )
          : intl.formatMessage({
              id: 'pages.openapiSources.bindingModal.title.bindProvider',
              defaultMessage: '绑定 Provider',
            })
      }
      open={open}
      onCancel={onCancel}
      onOk={onOk}
      okText={intl.formatMessage({
        id: 'pages.openapiSources.bindingModal.button.save',
        defaultMessage: '保存 binding',
      })}
    >
      <Space orientation="vertical" size={12} style={{ width: '100%' }}>
        <Alert
          type="info"
          showIcon
          message={intl.formatMessage({
            id: 'pages.openapiSources.bindingModal.alert.message',
            defaultMessage: '当前只启用 Provider binding',
          })}
          description={intl.formatMessage({
            id: 'pages.openapiSources.bindingModal.alert.description',
            defaultMessage:
              'httpConnector 需要 allowlist、SecretRef、超时/重试和审计策略后才能开放。',
          })}
        />
        <Input
          addonBefore="bindingId"
          value={bindingId}
          onChange={(event) => onBindingIdChange(event.target.value)}
        />
        <Select
          showSearch
          placeholder={intl.formatMessage({
            id: 'pages.openapiSources.bindingModal.function.placeholder',
            defaultMessage: '选择已注册函数',
          })}
          value={functionId}
          onChange={onFunctionIdChange}
          options={functionOptions}
          optionFilterProp="label"
          style={{ width: '100%' }}
        />
        <Input
          addonBefore="providerId"
          placeholder={intl.formatMessage({
            id: 'pages.openapiSources.bindingModal.providerId.placeholder',
            defaultMessage: '可选；留空由运行时按函数路由',
          })}
          value={providerId}
          onChange={(event) => onProviderIdChange(event.target.value)}
        />
      </Space>
    </Modal>
  );
}
