import React, { useCallback, useEffect, useState } from 'react';
import { Card, Table, Space, Button, Input, Select, Tag, Modal, Form } from 'antd';
import { PageContainer } from '@ant-design/pro-components';
import { listTickets, createTicket, updateTicket, deleteTicket } from '@/services/api/support';
import { history, useAccess } from '@umijs/max';
import { listAdmins, type AdminRecord } from '@/services/api/permissions';

type TicketPriority = 'urgent' | 'high' | 'normal' | 'low';
type TicketStatus = 'open' | 'in_progress' | 'resolved' | 'closed';

type SupportTicket = {
  id: number;
  title: string;
  content?: string;
  priority?: TicketPriority | string;
  status?: TicketStatus | string;
  assignee?: string;
  playerId?: string;
  contact?: string;
  gameId?: string;
  env?: string;
  source?: string;
  updatedAt?: string;
};

type SupportAccess = {
  canSupportManage?: boolean;
};

const priorityColors: Record<TicketPriority, string> = {
  urgent: 'red',
  high: 'volcano',
  normal: 'blue',
  low: 'default',
};

const priorityLabels: Record<TicketPriority, string> = {
  urgent: '阻断',
  high: '严重',
  normal: '一般',
  low: '轻微',
};

const statusColors: Record<TicketStatus, string> = {
  open: 'gold',
  in_progress: 'blue',
  resolved: 'green',
  closed: 'default',
};

const statusLabels: Record<TicketStatus, string> = {
  open: '新建',
  in_progress: '处理中',
  resolved: '已修复',
  closed: '关闭',
};

function isTicketPriority(value: string): value is TicketPriority {
  return value in priorityLabels;
}

function isTicketStatus(value: string): value is TicketStatus {
  return value in statusLabels;
}

export default function SupportBugsPage() {
  const [list, setList] = useState<SupportTicket[]>([]);
  const [loading, setLoading] = useState(false);
  const [page, setPage] = useState(1);
  const [size, setSize] = useState(20);
  const [total, setTotal] = useState(0);
  const [q, setQ] = useState('');
  const [status, setStatus] = useState<string>('');
  const [priority, setPriority] = useState<string>('');
  const [assignee, setAssignee] = useState<string>('');
  const [gameId, setGameId] = useState<string>('');
  const [env, setEnv] = useState<string>('');
  const [open, setOpen] = useState(false);
  const [editing, setEditing] = useState<SupportTicket | null>(null);
  const [form] = Form.useForm();
  const access = (useAccess?.() || {}) as SupportAccess;
  const [users, setUsers] = useState<AdminRecord[]>([]);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const res = await listTickets({
        q,
        status,
        priority,
        category: 'bug',
        assignee,
        gameId,
        env,
        page,
        size,
      });
      setList(res.tickets || []);
      setTotal(res.total || 0);
    } finally {
      setLoading(false);
    }
  }, [q, status, priority, assignee, gameId, env, page, size]);
  useEffect(() => {
    load();
  }, [load]);
  useEffect(() => {
    (async () => {
      try {
        const res = await listAdmins({ page: 1, pageSize: 200 });
        setUsers(res.items || []);
      } catch {}
    })();
  }, []);

  const priTag = (v?: string) => {
    if (!v) return '-';
    return (
      <Tag color={isTicketPriority(v) ? priorityColors[v] : 'default'}>
        {isTicketPriority(v) ? priorityLabels[v] : v}
      </Tag>
    );
  };
  const stTag = (v?: string) => {
    if (!v) return '-';
    return (
      <Tag color={isTicketStatus(v) ? statusColors[v] : 'default'}>
        {isTicketStatus(v) ? statusLabels[v] : v}
      </Tag>
    );
  };

  const openAdd = () => {
    setEditing(null);
    form.resetFields();
    setOpen(true);
  };
  const openEdit = (rec: SupportTicket) => {
    setEditing(rec);
    form.setFieldsValue(rec);
    setOpen(true);
  };
  const onSubmit = async () => {
    const v = await form.validateFields();
    const payload = { ...v, category: 'bug' };
    if (editing) {
      await updateTicket(editing.id, payload);
    } else {
      await createTicket(payload);
    }
    setOpen(false);
    load();
  };
  const onDelete = (rec: SupportTicket) => {
    Modal.confirm({
      title: '删除缺陷',
      content: `确定删除缺陷“${rec.title}”？`,
      onOk: async () => {
        await deleteTicket(rec.id);
        load();
      },
    });
  };

  return (
    <PageContainer>
      <Card
        title="缺陷列表"
        extra={
          <Space>
            <Input
              placeholder="关键词"
              value={q}
              onChange={(e) => setQ(e.target.value)}
              style={{ width: 180 }}
            />
            <Select
              placeholder="状态"
              value={status}
              onChange={setStatus}
              allowClear
              style={{ width: 140 }}
              options={[
                { label: '新建', value: 'open' },
                { label: '处理中', value: 'in_progress' },
                { label: '已修复', value: 'resolved' },
                { label: '关闭', value: 'closed' },
              ]}
            />
            <Select
              placeholder="严重级别"
              value={priority}
              onChange={setPriority}
              allowClear
              style={{ width: 160 }}
              options={[
                { label: '轻微', value: 'low' },
                { label: '一般', value: 'normal' },
                { label: '严重', value: 'high' },
                { label: '阻断', value: 'urgent' },
              ]}
            />
            <Select
              placeholder="处理人"
              allowClear
              value={assignee || undefined}
              onChange={(v) => setAssignee(v || '')}
              style={{ width: 160 }}
              options={users.map((u) => ({ label: u.username, value: u.username }))}
            />
            <Input
              placeholder="游戏"
              value={gameId}
              onChange={(e) => setGameId(e.target.value)}
              style={{ width: 120 }}
            />
            <Input
              placeholder="环境"
              value={env}
              onChange={(e) => setEnv(e.target.value)}
              style={{ width: 120 }}
            />
            <Button
              type="primary"
              onClick={() => {
                setPage(1);
                load();
              }}
            >
              查询
            </Button>
            {access.canSupportManage && <Button onClick={openAdd}>新建缺陷</Button>}
          </Space>
        }
      >
        <Table
          rowKey="id"
          loading={loading}
          dataSource={list}
          columns={[
            { title: '标题', dataIndex: 'title' },
            { title: '严重级别', dataIndex: 'priority', render: priTag },
            { title: '状态', dataIndex: 'status', render: stTag },
            { title: '处理人', dataIndex: 'assignee' },
            {
              title: '游戏/环境',
              render: (_, r: SupportTicket) => `${r.gameId || ''}/${r.env || ''}`,
            },
            {
              title: '更新时间',
              dataIndex: 'updatedAt',
              render: (v?: string) => (v ? new Date(v).toLocaleString() : '-'),
            },
            {
              title: '操作',
              render: (_, r: SupportTicket) => (
                <Space>
                  <Button size="small" onClick={() => history.push(`/support/tickets/${r.id}`)}>
                    查看详情
                  </Button>
                  {access.canSupportManage && (
                    <Button size="small" onClick={() => openEdit(r)}>
                      编辑
                    </Button>
                  )}
                  {access.canSupportManage && (
                    <Button size="small" danger onClick={() => onDelete(r)}>
                      删除
                    </Button>
                  )}
                </Space>
              ),
            },
          ]}
          pagination={{
            current: page,
            pageSize: size,
            total,
            showSizeChanger: true,
            onChange: (p, ps) => {
              setPage(p);
              setSize(ps || 20);
            },
          }}
        />

        <Modal
          title={editing ? '编辑缺陷' : '新建缺陷'}
          open={open}
          onOk={onSubmit}
          onCancel={() => setOpen(false)}
          destroyOnHidden
        >
          <Form
            form={form}
            layout="vertical"
            initialValues={{ priority: 'normal', status: 'open' }}
          >
            <Form.Item
              label="标题"
              name="title"
              rules={[{ required: true, message: '请输入标题' }]}
            >
              {' '}
              <Input />{' '}
            </Form.Item>
            <Form.Item label="内容" name="content">
              {' '}
              <Input.TextArea rows={4} />{' '}
            </Form.Item>
            <Form.Item label="严重级别" name="priority">
              {' '}
              <Select
                options={[
                  { label: '轻微', value: 'low' },
                  { label: '一般', value: 'normal' },
                  { label: '严重', value: 'high' },
                  { label: '阻断', value: 'urgent' },
                ]}
              />{' '}
            </Form.Item>
            <Form.Item label="状态" name="status">
              {' '}
              <Select
                options={[
                  { label: '新建', value: 'open' },
                  { label: '处理中', value: 'in_progress' },
                  { label: '已修复', value: 'resolved' },
                  { label: '关闭', value: 'closed' },
                ]}
              />{' '}
            </Form.Item>
            <Form.Item label="处理人" name="assignee">
              {' '}
              <Select
                allowClear
                showSearch
                options={users.map((u) => ({ label: u.username, value: u.username }))}
              />{' '}
            </Form.Item>
            <Form.Item label="玩家ID" name="playerId">
              {' '}
              <Input />{' '}
            </Form.Item>
            <Form.Item label="联系方式" name="contact">
              {' '}
              <Input />{' '}
            </Form.Item>
            <Form.Item label="游戏" name="gameId">
              {' '}
              <Input />{' '}
            </Form.Item>
            <Form.Item label="环境" name="env">
              {' '}
              <Input />{' '}
            </Form.Item>
            <Form.Item label="来源" name="source">
              {' '}
              <Input defaultValue="bug" />{' '}
            </Form.Item>
          </Form>
        </Modal>
      </Card>
    </PageContainer>
  );
}
