import React, { useEffect, useState } from 'react';
import { PageContainer, ProTable, ProColumns } from '@ant-design/pro-components';
import { App, Button, Space, Tag } from 'antd';
import {
  listPendingFunctions,
  publishPendingFunction,
  type PendingFunctionRow,
} from '@/services/api';

const fetchPending = async (): Promise<PendingFunctionRow[]> => {
  return listPendingFunctions();
};
const publish = async (fid: string) => {
  await publishPendingFunction(fid);
};

export default () => {
  const { message } = App.useApp();
  const [rows, setRows] = useState<PendingFunctionRow[]>([]);
  const [loading, setLoading] = useState(false);

  const reload = async () => {
    setLoading(true);
    try {
      const data = await fetchPending();
      setRows(data);
    } catch (e: any) {
      message.error(e?.message || '加载失败');
    } finally {
      setLoading(false);
    }
  };
  useEffect(() => {
    reload();
  }, []);

  const columns: ProColumns<PendingFunctionRow>[] = [
    { title: '函数ID', dataIndex: 'functionId', width: 280, copyable: true, ellipsis: true },
    { title: '名称(zh)', dataIndex: ['displayName', 'zh'], width: 220, ellipsis: true },
    { title: '摘要(zh)', dataIndex: ['summary', 'zh'], width: 320, ellipsis: true },
    {
      title: '建议权限',
      dataIndex: 'suggestedPermissions',
      width: 320,
      render: (_, r) => (
        <Space size="small">
          <span>verbs:</span>
          {(r.suggestedPermissions?.verbs || []).map((v) => (
            <Tag key={v}>{v}</Tag>
          ))}
          <span>scopes:</span>
          {(r.suggestedPermissions?.scopes || []).map((s) => (
            <Tag key={s}>{s}</Tag>
          ))}
        </Space>
      ),
    },
    {
      title: '操作',
      valueType: 'option',
      render: (_, r) => [
        <a
          key="publish"
          onClick={async () => {
            try {
              await publish(r.functionId);
              message.success('已发布到覆盖配置');
              reload();
            } catch (e: any) {
              message.error(e?.message || '发布失败');
            }
          }}
        >
          发布
        </a>,
      ],
    },
  ];

  return (
    <PageContainer
      title="待审核（发布到覆盖配置）"
      extra={[
        <Button key="refresh" onClick={reload}>
          刷新
        </Button>,
      ]}
    >
      <ProTable<PendingFunctionRow>
        rowKey="functionId"
        loading={loading}
        columns={columns}
        dataSource={rows}
        pagination={{ pageSize: 10 }}
        search={false}
      />
    </PageContainer>
  );
};
