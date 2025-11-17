import React, { useEffect, useState } from 'react';
import { Card, Table, Tag, Button, Space } from 'antd';
import type { ColumnsType } from 'antd/es/table';
import { getMessage } from '@/utils/antdApp';
import { listMessages, markMessagesRead, type MessageItem } from '@/services/croupier';

export default function AccountMessages() {
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
    getMessage()?.success('已标记为已读');
    refresh();
  };

  const columns: ColumnsType<MessageItem> = [
    { title: '时间', dataIndex: 'created_at', key: 'created_at' },
    { title: '类型', dataIndex: 'type', key: 'type', render: (t?: string) => <Tag>{t || 'info'}</Tag> },
    { title: '标题', dataIndex: 'title', key: 'title' },
    { title: '内容', dataIndex: 'content', key: 'content' },
    { title: '状态', dataIndex: 'read', key: 'read', render: (r: boolean) => r ? <Tag>已读</Tag> : <Tag color="red">未读</Tag> },
    { title: '操作', key: 'ops', render: (_: any, rec) => (
      <Space>
        {!rec.read && (
          <Button size="small" onClick={() => markSelRead([rec.id], [rec.kind || 'direct'])}>标记已读</Button>
        )}
      </Space>
    )},
  ];

  return (
    <div style={{ padding: 24 }}>
      <Card
        title={status === 'unread' ? '未读消息' : '全部消息'}
        extra={
          <Space>
            <Button type={status === 'unread' ? 'primary' : 'default'} onClick={() => setStatus('unread')}>未读</Button>
            <Button type={status === 'all' ? 'primary' : 'default'} onClick={() => setStatus('all')}>全部</Button>
            <Button onClick={() => markSelRead()}>全部标记已读</Button>
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
    </div>
  );
}
