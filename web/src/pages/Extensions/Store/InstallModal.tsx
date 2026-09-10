import React from 'react';
import type { FormInstance } from 'antd';
import { Form, Input, Modal, Select, Space, Typography } from 'antd';
import type { ExtensionCatalogItem, ExtensionReleaseItem } from '@/services/api/extensions';
import type { JSONValue } from '@/types/dashboard';
import type { InstallFormValues } from './shared';
import SchemaFields from './SchemaFields';

/** 安装扩展弹窗：版本/scope/target/配置 JSON + manifest.configSchema 表单。
 * Form 实例由页面持有（预填时序不变），提交经 onOk 上抛。 */
export default function InstallModal({
  open,
  form,
  item,
  releases,
  configSchema,
  installing,
  onCancel,
  onOk,
}: {
  open: boolean;
  form: FormInstance<InstallFormValues>;
  item: ExtensionCatalogItem | undefined;
  releases: ExtensionReleaseItem[];
  configSchema: Record<string, JSONValue> | undefined;
  installing: boolean;
  onCancel: () => void;
  onOk: () => void;
}) {
  return (
    <Modal
      open={open}
      onCancel={onCancel}
      onOk={onOk}
      okButtonProps={{ loading: installing }}
      title={`安装扩展: ${item?.displayName || item?.name || ''}`}
      width={720}
    >
      <Form form={form} layout="vertical">
        <Form.Item
          name="releaseVersion"
          label="版本"
          rules={[{ required: true, message: '请选择版本' }]}
        >
          <Select
            placeholder="选择版本"
            options={(releases || []).map((r) => ({ label: r.version, value: r.version }))}
          />
        </Form.Item>
        {releases.length === 0 && (
          <Typography.Text type="warning">当前扩展没有可用发布版本，暂不可安装。</Typography.Text>
        )}
        <Space style={{ width: '100%' }} size="middle">
          <Form.Item
            name="scopeType"
            label="Scope Type"
            style={{ flex: 1 }}
            rules={[{ required: true, message: '请输入 scopeType' }]}
          >
            <Input placeholder="system" />
          </Form.Item>
          <Form.Item
            name="scopeId"
            label="Scope ID"
            style={{ flex: 1 }}
            rules={[{ required: true, message: '请输入 scopeId' }]}
          >
            <Input placeholder="global" />
          </Form.Item>
        </Space>
        <Space style={{ width: '100%' }} size="middle">
          <Form.Item
            name="targetType"
            label="Target Type"
            style={{ flex: 1 }}
            rules={[{ required: true, message: '请输入 targetType' }]}
          >
            <Input placeholder="agent_group" />
          </Form.Item>
          <Form.Item name="targetId" label="Target ID" style={{ flex: 1 }}>
            <Input placeholder="default" />
          </Form.Item>
        </Space>
        <Form.Item name="configJson" label="配置 JSON">
          <Input.TextArea rows={5} placeholder='{"enabled": true}' />
        </Form.Item>
        {configSchema?.properties && typeof configSchema.properties === 'object' && (
          <SchemaFields schema={configSchema} />
        )}
      </Form>
    </Modal>
  );
}
