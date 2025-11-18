import React, { useEffect, useState } from 'react';
import { Card, Table, Tag, Button, Space } from 'antd';
import { PageContainer } from '@ant-design/pro-components';
import { useIntl } from '@umijs/max';
import type { ColumnsType } from 'antd/es/table';
import { getMessage } from '@/utils/antdApp';
import { listMessages, markMessagesRead, type MessageItem } from '@/services/croupier';

export default function AccountMessages() {
  const intl = useIntl();
  const [items, setItems] = useState<MessageItem[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [size, setSize] = useState(10);
  const [loading, setLoading] = useState(false);
  const [status, setStatus] = useState<'unread' | 'all'>('unread');

  const refresh = async (p = page, s = size, st = status) => {
    setLoading(true);
    try {
      const r = await listMessages({ page: p, size: s, status: st });
      setItems(r.messages || []);
      setTotal(r.total || 0);
      setPage(r.page || p);
      setSize(r.size || s);
    } finally { setLoading(false); }
  };
  useEffect(() => { refresh(1, size, status); }, [status]);

  const markSelRead = async (ids?: number[], kinds?: ('direct'|'broadcast')[]) => {
    const target = ids != null ? ids.map((id, i) => ({ id, kind: kinds?.[i] || 'direct' })) : items.filter(x => !x.read).map(x => ({ id: x.id, kind: x.kind || 'direct' }));
    const directIds = target.filter(t => t.kind !== 'broadcast').map(t => t.id);
    const bcastIds = target.filter(t => t.kind === 'broadcast').map(t => t.id);
    const sel = directIds.concat(bcastIds);
    if (sel.length === 0) return;
    await markMessagesRead(directIds, { broadcast_ids: bcastIds });
    getMessage()?.success(intl.formatMessage({ id: 'pages.account.messages.marked.read' }));
    refresh();
  };

  const columns: ColumnsType<MessageItem> = [
    { title: intl.formatMessage({ id: 'pages.account.messages.time' }), dataIndex: 'created_at', key: 'created_at' },
    { title: intl.formatMessage({ id: 'pages.account.messages.type' }), dataIndex: 'type', key: 'type', render: (t?: string) => <Tag>{t || 'info'}</Tag> },
    { title: intl.formatMessage({ id: 'pages.account.messages.title' }), dataIndex: 'title', key: 'title' },
    { title: intl.formatMessage({ id: 'pages.account.messages.content' }), dataIndex: 'content', key: 'content' },
    { title: intl.formatMessage({ id: 'pages.account.messages.status' }), dataIndex: 'read', key: 'read', render: (r: boolean) => r ? <Tag>{intl.formatMessage({ id: 'pages.account.messages.read' })}</Tag> : <Tag color="red">{intl.formatMessage({ id: 'pages.account.messages.unread' })}</Tag> },
    { title: intl.formatMessage({ id: 'pages.account.messages.actions' }), key: 'ops', render: (_: any, rec) => (
      <Space>
        {!rec.read && (
          <Button size="small" onClick={() => markSelRead([rec.id], [rec.kind || 'direct'])}>{intl.formatMessage({ id: 'pages.account.messages.mark.read' })}</Button>
        )}
      </Space>
    )},
  ];

  return (
    <PageContainer>
      <Card
        title={status === 'unread' ? intl.formatMessage({ id: 'pages.account.messages.unread.title' }) : intl.formatMessage({ id: 'pages.account.messages.all.title' })}
        extra={
          <Space>
            <Button type={status === 'unread' ? 'primary' : 'default'} onClick={() => setStatus('unread')}>{intl.formatMessage({ id: 'pages.account.messages.unread.button' })}</Button>
            <Button type={status === 'all' ? 'primary' : 'default'} onClick={() => setStatus('all')}>{intl.formatMessage({ id: 'pages.account.messages.all.button' })}</Button>
            <Button onClick={() => markSelRead()}>{intl.formatMessage({ id: 'pages.account.messages.mark.all.read' })}</Button>
          </Space>
        }
      >
        <Table
          rowKey="id"
          columns={columns}
          dataSource={items}
          loading={loading}
          pagination={{ current: page, pageSize: size, total, onChange: (p, s) => refresh(p, s, status) }}
        />
      </Card>
    </PageContainer>
  );
}
