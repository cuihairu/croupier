import React from 'react';
import { Alert, Button, Card, Descriptions, Space, Tabs, Typography } from 'antd';
import type { FunctionDescriptor } from '@/services/api/functions';
import type { JSONSchema } from '@/types/dashboard';
import { JsonViewer } from './DetailSections';

const { Text } = Typography;

export type ConfigTabProps = {
  functionId: string;
  activeSubTab: string;
  onSubTabChange: (key: string) => void;
  jsonViewData: {
    descriptor_from_detail_api?: FunctionDescriptor | null;
    descriptor_from_index_api?: FunctionDescriptor | null;
    openapi_operation?: unknown;
  };
  onJsonCopySuccess: () => void;
  onJsonCopyError: () => void;
  formDescriptor: Partial<FunctionDescriptor>;
  parsedInputSchema?: JSONSchema;
  onOpenPageStudio: () => void;
};

export default function DetailConfigTab({
  functionId,
  activeSubTab,
  onSubTabChange,
  jsonViewData,
  onJsonCopySuccess,
  onJsonCopyError,
  formDescriptor,
  parsedInputSchema,
  onOpenPageStudio,
}: ConfigTabProps) {
  const outputSchema =
    typeof formDescriptor.outputSchema === 'string'
      ? formDescriptor.outputSchema
      : JSON.stringify(formDescriptor.outputSchema || {}, null, 2);

  const jsonTabItems = [
    {
      key: 'json-detail',
      label: 'Detail API',
      children: (
        <JsonViewer
          data={jsonViewData.descriptor_from_detail_api || {}}
          onCopySuccess={onJsonCopySuccess}
          onCopyError={onJsonCopyError}
        />
      ),
    },
    {
      key: 'json-index',
      label: 'Descriptor Index',
      children: (
        <JsonViewer
          data={jsonViewData.descriptor_from_index_api || {}}
          onCopySuccess={onJsonCopySuccess}
          onCopyError={onJsonCopyError}
        />
      ),
    },
    {
      key: 'json-openapi',
      label: 'OpenAPI',
      children: (
        <JsonViewer
          data={jsonViewData.openapi_operation || {}}
          onCopySuccess={onJsonCopySuccess}
          onCopyError={onJsonCopyError}
        />
      ),
    },
  ];

  const configTabItems = [
    {
      key: 'json',
      label: '元数据只读',
      children: (
        <>
          <Alert
            message="函数元数据"
            description="按来源拆分查看：详情接口、描述符索引和 OpenAPI。这一页只用于核对能力契约，不用于搭页面。"
            type="info"
            showIcon
          />
          <Tabs style={{ marginTop: 16 }} type="card" size="small" items={jsonTabItems} />
        </>
      ),
    },
    {
      key: 'schema',
      label: '契约 Schema',
      children: (
        <Space direction="vertical" size={16} style={{ width: '100%' }}>
          <Alert
            message="函数注册只提供契约，不保存页面 UI"
            description="这里展示 input/output JSON Schema，用于核对调用参数和返回结构。默认业务页面由 PageProposal 生成，接受后进入 Page Studio 编辑和发布。"
            type="info"
            showIcon
            action={
              <Button type="primary" size="small" onClick={onOpenPageStudio}>
                查看页面候选
              </Button>
            }
          />
          <Descriptions bordered size="small" column={1}>
            <Descriptions.Item label="Function ID">
              <Text code>{functionId || '-'}</Text>
            </Descriptions.Item>
            <Descriptions.Item label="Resource">
              <Text>{formDescriptor.resource || '-'}</Text>
            </Descriptions.Item>
            <Descriptions.Item label="Operation">
              <Text>{formDescriptor.operation || '-'}</Text>
            </Descriptions.Item>
          </Descriptions>
          <Card size="small" title="Input JSON Schema">
            <JsonViewer
              data={parsedInputSchema || {}}
              onCopySuccess={onJsonCopySuccess}
              onCopyError={onJsonCopyError}
            />
          </Card>
          <Card size="small" title="Output JSON Schema">
            <JsonViewer
              data={outputSchema ? outputSchema : {}}
              onCopySuccess={onJsonCopySuccess}
              onCopyError={onJsonCopyError}
            />
          </Card>
        </Space>
      ),
    },
  ];

  return (
    <>
      <Alert
        type="warning"
        showIcon
        style={{ marginBottom: 16 }}
        message="这里配置的是单个函数，不是整个业务页面"
        description={
          <Space wrap>
            <span>
              函数层只确认能力契约；分类、菜单、列表、详情、动作位置和表单展示都由
              PageProposal/PageSpec 决定。
            </span>
            <Button type="primary" size="small" onClick={onOpenPageStudio}>
              查看资源/页面候选
            </Button>
          </Space>
        }
      />
      <Tabs
        activeKey={activeSubTab}
        onChange={onSubTabChange}
        type="card"
        size="small"
        items={configTabItems}
      />
    </>
  );
}
