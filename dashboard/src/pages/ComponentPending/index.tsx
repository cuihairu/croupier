import React, { useEffect, useState } from 'react';
import { PageContainer, ProTable, ProColumns } from '@ant-design/pro-components';
import { App, Button, Space, Tag } from 'antd';
import { request } from '@umijs/max';

type PendingRow = {
  function_id: string;
  display_name?: { zh?: string; en?: string };
  summary?: { zh?: string; en?: string };
  suggested_permissions?: { verbs?: string[]; scopes?: string[] };
};

const fetchPending = async (): Promise<PendingRow[]> => {
  const res = await request('/api/admin/pending', { method: 'GET' });
  return res?.pending || [];
};
const publish = async (fid: string) => {
  await request(`/api/admin/functions/${encodeURIComponent(fid)}/publish`, { method: 'POST' });
};

export default () => {
  const { message } = App.useApp();
  const [rows, setRows] = useState<PendingRow[]>([]);
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
  useEffect(() => { reload(); }, []);

  const columns: ProColumns<PendingRow>[] = [
    { title: '函数ID', dataIndex: 'function_id', width: 280, copyable: true, ellipsis: true },
    { title: '名称(zh)', dataIndex: ['display_name','zh'], width: 220, ellipsis: true },
    { title: '摘要(zh)', dataIndex: ['summary','zh'], width: 320, ellipsis: true },
    {
      title: '建议权限', dataIndex: 'suggested_permissions', width: 320, render: (_, r) => (
        <Space size="small">
          <span>verbs:</span>
          {(r.suggested_permissions?.verbs || []).map(v => <Tag key={v}>{v}</Tag>)}
          <span>scopes:</span>
          {(r.suggested_permissions?.scopes || []).map(s => <Tag key={s}>{s}</Tag>)}
        </Space>
      )
    },
    {
      title: '操作',
      valueType: 'option',
      render: (_, r) => [
        <a key="publish" onClick={async () => {
          try {
            await publish(r.function_id);
            message.success('已发布到覆盖配置');
            reload();
          } catch (e: any) {
            message.error(e?.message || '发布失败');
          }
        }}>发布</a>
      ]
    }
  ];

  return (
    <PageContainer title="待审核（发布到覆盖配置）" extra={[
      <Button key="refresh" onClick={reload}>刷新</Button>
    ]}>
      <ProTable<PendingRow>
        rowKey="function_id"
        loading={loading}
        columns={columns}
        dataSource={rows}
        pagination={{ pageSize: 10 }}
        search={false}
      />
    </PageContainer>
  );
};

