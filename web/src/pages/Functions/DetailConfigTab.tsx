import React from 'react';
import {
  Alert,
  Button,
  Card,
  Space,
  Tabs,
} from 'antd';
import FunctionFormManager from '@/components/FunctionFormManager';
import type { FormilySchema } from '@/components/formily/schema/types';
import type { FunctionDescriptor } from '@/services/api/functions';
import type { JSONSchema } from '@/types/dashboard';
import { JsonViewer } from './DetailSections';

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
  onSaveForm: (formConfig: { schema?: FormilySchema; clearCustom?: boolean }) => Promise<void>;
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
  onSaveForm,
  onOpenPageStudio,
}: ConfigTabProps) {
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
      key: 'ui',
      label: '函数表单',
      children: (
        <FunctionFormManager
          functionId={functionId}
          descriptor={formDescriptor}
          jsonSchema={parsedInputSchema}
          onSave={onSaveForm}
        />
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
              “函数表单”只影响当前函数的入参展示；如果你要把多个函数组装成实际可操作页面，请进入 Page Studio。
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
