import React, { useEffect, useState } from 'react';
import { PageContainer, ProTable, ProColumns } from '@ant-design/pro-components';
import { App, Button, Space, Tag } from 'antd';
import { history, request } from '@umijs/max';

type I18N = { zh?: string; en?: string };
type Menu = { section?: string; group?: string; path?: string; order?: number; hidden?: boolean };
type Row = { id: string; enabled?: boolean; display_name?: I18N; summary?: I18N; tags?: string[]; menu?: Menu };

async function fetchSummary(): Promise<Row[]> {
  const res: any = await request('/api/functions/summary');
  if (Array.isArray(res)) return res;
  if (res?.functions && Array.isArray(res.functions)) return res.functions;
  return [];
}

export default () => {
  const { message } = App.useApp();
  const [rows, setRows] = useState<Row[]>([]);
  const [loading, setLoading] = useState(false);
  const reload = async () => {
    setLoading(true);
    try {
      const data = await fetchSummary();
      setRows(data);
    } catch (e: any) {
      message.error(e?.message || '加载失败');
    } finally {
      setLoading(false);
    }
  };
  useEffect(() => { reload(); }, []);

  const columns: ProColumns<Row>[] = [
    { title: '函数ID', dataIndex: 'id', width: 280, copyable: true, ellipsis: true },
    { title: '名称(zh)', dataIndex: ['display_name','zh'], width: 220, ellipsis: true },
    { title: '摘要(zh)', dataIndex: ['summary','zh'], width: 320, ellipsis: true },
    { title: '标签', dataIndex: 'tags', render: (_, r) => <Space>{(r.tags||[]).map(t => <Tag key={t}>{t}</Tag>)}</Space> },
    { title: '菜单', dataIndex: 'menu', render: (_, r) => r.menu ? <span>{r.menu.section} / {r.menu.group}</span> : '-' },
    {
      title: '操作', valueType: 'option', render: (_, r) => [
        <a key="invoke" onClick={() => {
          const path = (r.menu?.path || '/GmFunctions') + `?fid=${encodeURIComponent(r.id)}`;
          history.push(path);
        }}>进入</a>
      ]
    }
  ];

  return (
    <PageContainer title="函数目录">
      <ProTable<Row>
        rowKey="id"
        loading={loading}
        columns={columns}
        dataSource={rows}
        pagination={{ pageSize: 10 }}
        search={{ filterType: 'light' }}
        toolBarRender={() => [
          <Button key="refresh" onClick={reload}>刷新</Button>
        ]}
      />
    </PageContainer>
  );
};

