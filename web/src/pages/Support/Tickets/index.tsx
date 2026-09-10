import React, { useEffect, useRef, useState } from 'react';
import { App, Card, Space, Button, Input, Select, Tag, Modal, Form, Dropdown } from 'antd';
import {
  ModalForm,
  PageContainer,
  ProTable,
  type ActionType,
  type ProColumns,
} from '@ant-design/pro-components';
import type { MenuProps } from 'antd';
import { listAdmins, type AdminRecord } from '@/services/api/permissions';
import { history } from '@umijs/max';
import {
  listTickets,
  createTicket,
  updateTicket,
  deleteTicket,
  transitionTicket,
  type TicketPayload,
} from '@/services/api/support';
import { useAccess } from '@umijs/max';
import { extractErrorMessage } from '@/utils/errors';
import { formatDateTime } from '@/utils/format';

type TicketPriority = 'urgent' | 'high' | 'normal' | 'low';
type TicketStatus = 'open' | 'in_progress' | 'resolved' | 'closed';

type SupportTicket = {
  id: number;
  title: string;
  content?: string;
  category?: string;
  priority?: TicketPriority | string;
  status?: TicketStatus | string;
  assignee?: string;
  tags?: string[];
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

const ticketStatuses: TicketStatus[] = ['open', 'in_progress', 'resolved', 'closed'];

const priorityColors: Record<TicketPriority, string> = {
  urgent: 'red',
  high: 'volcano',
  normal: 'blue',
  low: 'default',
};

const priorityLabels: Record<TicketPriority, string> = {
  urgent: '紧急',
  high: '高',
  normal: '普通',
  low: '低',
};

const statusColors: Record<TicketStatus, string> = {
  open: 'gold',
  in_progress: 'blue',
  resolved: 'green',
  closed: 'default',
};

const statusLabels: Record<TicketStatus, string> = {
  open: '打开',
  in_progress: '处理中',
  resolved: '已解决',
  closed: '已关闭',
};

function isTicketPriority(value: string): value is TicketPriority {
  return value in priorityLabels;
}

function isTicketStatus(value: string): value is TicketStatus {
  return value in statusLabels;
}

export default function SupportTicketsPage() {
  const { message } = App.useApp();
  const actionRef = useRef<ActionType | undefined>(undefined);
  const [q, setQ] = useState('');
  const [status, setStatus] = useState<string>('');
  const [priority, setPriority] = useState<string>('');
  const [category, setCategory] = useState<string>('');
  const [assignee, setAssignee] = useState<string>('');
  const [gameId, setGameId] = useState<string>('');
  const [env, setEnv] = useState<string>('');
  const [open, setOpen] = useState(false);
  const [editing, setEditing] = useState<SupportTicket | null>(null);
  const access = (useAccess?.() || {}) as SupportAccess;
  const [users, setUsers] = useState<AdminRecord[]>([]);

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

  // destroyOnHidden 使弹窗每次关闭即卸载表单，重开时按最新 initialValues
  // 重新挂载，新增/编辑切换不会残留上一次的预填值
  const openAdd = () => {
    setEditing(null);
    setOpen(true);
  };
  const openEdit = (rec: SupportTicket) => {
    setEditing(rec);
    setOpen(true);
  };
  const onFinish = async (v: TicketPayload) => {
    // 原实现无本地弹错（全局请求拦截器已 toast），失败时弹窗保持开启
    try {
      if (editing) {
        await updateTicket(editing.id, v);
      } else {
        await createTicket(v);
      }
      actionRef.current?.reload();
      return true;
    } catch {
      return false;
    }
  };
  const onDelete = (rec: SupportTicket) => {
    Modal.confirm({
      title: '删除工单',
      content: `确定删除工单“${rec.title}”？`,
      onOk: async () => {
        await deleteTicket(rec.id);
        actionRef.current?.reload();
      },
    });
  };

  const transition = async (rec: SupportTicket, status: TicketStatus) => {
    await transitionTicket(rec.id, { status });
    actionRef.current?.reload();
  };

  const transitionMenu = (rec: SupportTicket): MenuProps['items'] =>
    ticketStatuses
      .filter((s) => s !== rec.status)
      .map((s) => ({
        key: s,
        label: statusLabels[s],
      }));

  const columns: ProColumns<SupportTicket>[] = [
    { title: '标题', dataIndex: 'title' },
    { title: '分类', dataIndex: 'category' },
    { title: '优先级', dataIndex: 'priority', render: (_, row) => priTag(row.priority) },
    { title: '状态', dataIndex: 'status', render: (_, row) => stTag(row.status) },
    { title: '处理人', dataIndex: 'assignee' },
    {
      title: '游戏/环境',
      render: (_, r: SupportTicket) => `${r.gameId || ''}/${r.env || ''}`,
    },
    {
      title: '更新时间',
      dataIndex: 'updatedAt',
      render: (_, row) => formatDateTime(row.updatedAt ?? ''),
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
          {access.canSupportManage && (
            <Dropdown
              menu={{
                items: transitionMenu(r),
                onClick: ({ key }) => {
                  const nextStatus = String(key);
                  if (isTicketStatus(nextStatus)) {
                    transition(r, nextStatus);
                  }
                },
              }}
              trigger={['click']}
            >
              <Button size="small">流转为</Button>
            </Dropdown>
          )}
        </Space>
      ),
    },
  ];

  return (
    <PageContainer>
      <Card
        title="工单系统"
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
              onChange={(v) => {
                setStatus(v);
                // 筛选变化回第 1 页：params 变化与 setPageInfo 的双触发由
                // ProTable 内部 debounce + abort 合并，不会出现错序数据
                actionRef.current?.setPageInfo?.({ current: 1 });
              }}
              allowClear
              style={{ width: 140 }}
              options={[
                { label: '打开', value: 'open' },
                { label: '处理中', value: 'in_progress' },
                { label: '已解决', value: 'resolved' },
                { label: '已关闭', value: 'closed' },
              ]}
            />
            <Select
              placeholder="优先级"
              value={priority}
              onChange={(v) => {
                setPriority(v);
                actionRef.current?.setPageInfo?.({ current: 1 });
              }}
              allowClear
              style={{ width: 140 }}
              options={[
                { label: '低', value: 'low' },
                { label: '普通', value: 'normal' },
                { label: '高', value: 'high' },
                { label: '紧急', value: 'urgent' },
              ]}
            />
            <Input
              placeholder="分类"
              value={category}
              onChange={(e) => setCategory(e.target.value)}
              style={{ width: 120 }}
            />
            <Input
              placeholder="处理人"
              value={assignee}
              onChange={(e) => setAssignee(e.target.value)}
              style={{ width: 120 }}
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
                // 回第 1 页并重查：已在第 1 页时 setPageInfo 不触发请求，
                // 由 reload 兜底；非第 1 页时双触发经 debounce + abort 合并
                actionRef.current?.setPageInfo?.({ current: 1 });
                actionRef.current?.reload();
              }}
            >
              查询
            </Button>
            {access.canSupportManage && <Button onClick={openAdd}>新建工单</Button>}
          </Space>
        }
      >
        <ProTable<SupportTicket>
          actionRef={actionRef}
          rowKey="id"
          columns={columns}
          search={false}
          options={false}
          toolBarRender={false}
          params={{ q, status, priority, category, assignee, gameId, env }}
          request={async ({
            current = 1,
            pageSize = 20,
            q: qFilter,
            status: statusFilter,
            priority: priorityFilter,
            category: categoryFilter,
            assignee: assigneeFilter,
            gameId: gameIdFilter,
            env: envFilter,
          }) => {
            try {
              const res = await listTickets({
                q: qFilter ?? '',
                status: statusFilter ?? '',
                priority: priorityFilter ?? '',
                category: categoryFilter ?? '',
                assignee: assigneeFilter ?? '',
                gameId: gameIdFilter ?? '',
                env: envFilter ?? '',
                page: current,
                size: pageSize,
              });
              return { data: res.tickets || [], total: res.total || 0, success: true };
            } catch (error) {
              message.error(extractErrorMessage(error, '加载工单失败'));
              return { data: [], total: 0, success: false };
            }
          }}
          pagination={{ pageSize: 20, showSizeChanger: true }}
        />

        <ModalForm<TicketPayload>
          title={editing ? '编辑工单' : '新建工单'}
          open={open}
          onOpenChange={setOpen}
          modalProps={{ destroyOnHidden: true }}
          width={520}
          layout="vertical"
          submitter={{ searchConfig: { submitText: '确定' } }}
          initialValues={editing ?? { priority: 'normal', status: 'open' }}
          onFinish={onFinish}
        >
          <Form.Item label="标题" name="title" rules={[{ required: true, message: '请输入标题' }]}>
            {' '}
            <Input />{' '}
          </Form.Item>
          <Form.Item label="内容" name="content">
            {' '}
            <Input.TextArea rows={4} />{' '}
          </Form.Item>
          <Form.Item label="分类" name="category">
            {' '}
            <Input />{' '}
          </Form.Item>
          <Form.Item label="优先级" name="priority">
            {' '}
            <Select
              options={[
                { label: '低', value: 'low' },
                { label: '普通', value: 'normal' },
                { label: '高', value: 'high' },
                { label: '紧急', value: 'urgent' },
              ]}
            />{' '}
          </Form.Item>
          <Form.Item label="状态" name="status">
            {' '}
            <Select
              options={[
                { label: '打开', value: 'open' },
                { label: '处理中', value: 'in_progress' },
                { label: '已解决', value: 'resolved' },
                { label: '已关闭', value: 'closed' },
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
          <Form.Item label="标签" name="tags">
            {' '}
            <Input placeholder="," />{' '}
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
            <Input />{' '}
          </Form.Item>
        </ModalForm>
      </Card>
    </PageContainer>
  );
}
