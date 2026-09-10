import React, { useEffect, useRef, useState } from 'react';
import { App, Card, Space, Button, Input, Select, Tag, Form, Dropdown } from 'antd';
import {
  ModalForm,
  PageContainer,
  ProTable,
  type ActionType,
  type ProColumns,
} from '@ant-design/pro-components';
import type { MenuProps } from 'antd';
import { listAdmins, type AdminRecord } from '@/services/api/permissions';
import { FormattedMessage, history, useIntl } from '@umijs/max';
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

type IntlMessage = { id: string; defaultMessage: string };

const ticketStatuses: TicketStatus[] = ['open', 'in_progress', 'resolved', 'closed'];

const priorityColors: Record<TicketPriority, string> = {
  urgent: 'red',
  high: 'volcano',
  normal: 'blue',
  low: 'default',
};

const priorityLabels: Record<TicketPriority, IntlMessage> = {
  urgent: { id: 'pages.tickets.priority.urgent', defaultMessage: '紧急' },
  high: { id: 'pages.tickets.priority.high', defaultMessage: '高' },
  normal: { id: 'pages.tickets.priority.normal', defaultMessage: '普通' },
  low: { id: 'pages.tickets.priority.low', defaultMessage: '低' },
};

const statusColors: Record<TicketStatus, string> = {
  open: 'gold',
  in_progress: 'blue',
  resolved: 'green',
  closed: 'default',
};

const statusLabels: Record<TicketStatus, IntlMessage> = {
  open: { id: 'pages.tickets.status.open', defaultMessage: '打开' },
  in_progress: { id: 'pages.tickets.status.inProgress', defaultMessage: '处理中' },
  resolved: { id: 'pages.tickets.status.resolved', defaultMessage: '已解决' },
  closed: { id: 'pages.tickets.status.closed', defaultMessage: '已关闭' },
};

function isTicketPriority(value: string): value is TicketPriority {
  return value in priorityLabels;
}

function isTicketStatus(value: string): value is TicketStatus {
  return value in statusLabels;
}

export default function SupportTicketsPage() {
  const { message, modal } = App.useApp();
  const intl = useIntl();
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
    const known = isTicketPriority(v);
    return (
      <Tag color={known ? priorityColors[v] : 'default'}>
        {known ? intl.formatMessage(priorityLabels[v]) : v}
      </Tag>
    );
  };
  const stTag = (v?: string) => {
    if (!v) return '-';
    const known = isTicketStatus(v);
    return (
      <Tag color={known ? statusColors[v] : 'default'}>
        {known ? intl.formatMessage(statusLabels[v]) : v}
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
    modal.confirm({
      title: intl.formatMessage({
        id: 'pages.tickets.deleteConfirm.title',
        defaultMessage: '删除工单',
      }),
      content: intl.formatMessage(
        { id: 'pages.tickets.deleteConfirm.content', defaultMessage: '确定删除工单“{title}”？' },
        { title: rec.title },
      ),
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
        label: intl.formatMessage(statusLabels[s]),
      }));

  const columns: ProColumns<SupportTicket>[] = [
    {
      title: intl.formatMessage({ id: 'pages.tickets.field.title', defaultMessage: '标题' }),
      dataIndex: 'title',
    },
    {
      title: intl.formatMessage({ id: 'pages.tickets.field.category', defaultMessage: '分类' }),
      dataIndex: 'category',
    },
    {
      title: intl.formatMessage({ id: 'pages.tickets.field.priority', defaultMessage: '优先级' }),
      dataIndex: 'priority',
      render: (_, row) => priTag(row.priority),
    },
    {
      title: intl.formatMessage({ id: 'pages.tickets.field.status', defaultMessage: '状态' }),
      dataIndex: 'status',
      render: (_, row) => stTag(row.status),
    },
    {
      title: intl.formatMessage({ id: 'pages.tickets.field.assignee', defaultMessage: '处理人' }),
      dataIndex: 'assignee',
    },
    {
      title: intl.formatMessage({ id: 'pages.tickets.field.gameEnv', defaultMessage: '游戏/环境' }),
      render: (_, r: SupportTicket) => `${r.gameId || ''}/${r.env || ''}`,
    },
    {
      title: intl.formatMessage({
        id: 'pages.tickets.field.updatedAt',
        defaultMessage: '更新时间',
      }),
      dataIndex: 'updatedAt',
      render: (_, row) => formatDateTime(row.updatedAt ?? ''),
    },
    {
      title: intl.formatMessage({ id: 'pages.tickets.field.actions', defaultMessage: '操作' }),
      render: (_, r: SupportTicket) => (
        <Space>
          <Button size="small" onClick={() => history.push(`/support/tickets/${r.id}`)}>
            <FormattedMessage id="pages.tickets.action.viewDetail" defaultMessage="查看详情" />
          </Button>
          {access.canSupportManage && (
            <Button size="small" onClick={() => openEdit(r)}>
              <FormattedMessage id="pages.tickets.action.edit" defaultMessage="编辑" />
            </Button>
          )}
          {access.canSupportManage && (
            <Button size="small" danger onClick={() => onDelete(r)}>
              <FormattedMessage id="pages.tickets.action.delete" defaultMessage="删除" />
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
              <Button size="small">
                <FormattedMessage id="pages.tickets.action.transitionTo" defaultMessage="流转为" />
              </Button>
            </Dropdown>
          )}
        </Space>
      ),
    },
  ];

  return (
    <PageContainer>
      <Card
        title={intl.formatMessage({ id: 'pages.tickets.card.title', defaultMessage: '工单系统' })}
        extra={
          <Space>
            <Input
              placeholder={intl.formatMessage({
                id: 'pages.tickets.search.keyword',
                defaultMessage: '关键词',
              })}
              value={q}
              onChange={(e) => setQ(e.target.value)}
              style={{ width: 180 }}
            />
            <Select
              placeholder={intl.formatMessage({
                id: 'pages.tickets.field.status',
                defaultMessage: '状态',
              })}
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
                { label: intl.formatMessage(statusLabels.open), value: 'open' },
                { label: intl.formatMessage(statusLabels.in_progress), value: 'in_progress' },
                { label: intl.formatMessage(statusLabels.resolved), value: 'resolved' },
                { label: intl.formatMessage(statusLabels.closed), value: 'closed' },
              ]}
            />
            <Select
              placeholder={intl.formatMessage({
                id: 'pages.tickets.field.priority',
                defaultMessage: '优先级',
              })}
              value={priority}
              onChange={(v) => {
                setPriority(v);
                actionRef.current?.setPageInfo?.({ current: 1 });
              }}
              allowClear
              style={{ width: 140 }}
              options={[
                { label: intl.formatMessage(priorityLabels.low), value: 'low' },
                { label: intl.formatMessage(priorityLabels.normal), value: 'normal' },
                { label: intl.formatMessage(priorityLabels.high), value: 'high' },
                { label: intl.formatMessage(priorityLabels.urgent), value: 'urgent' },
              ]}
            />
            <Input
              placeholder={intl.formatMessage({
                id: 'pages.tickets.field.category',
                defaultMessage: '分类',
              })}
              value={category}
              onChange={(e) => setCategory(e.target.value)}
              style={{ width: 120 }}
            />
            <Input
              placeholder={intl.formatMessage({
                id: 'pages.tickets.field.assignee',
                defaultMessage: '处理人',
              })}
              value={assignee}
              onChange={(e) => setAssignee(e.target.value)}
              style={{ width: 120 }}
            />
            <Input
              placeholder={intl.formatMessage({
                id: 'pages.tickets.field.gameId',
                defaultMessage: '游戏',
              })}
              value={gameId}
              onChange={(e) => setGameId(e.target.value)}
              style={{ width: 120 }}
            />
            <Input
              placeholder={intl.formatMessage({
                id: 'pages.tickets.field.env',
                defaultMessage: '环境',
              })}
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
              <FormattedMessage id="pages.tickets.search.submit" defaultMessage="查询" />
            </Button>
            {access.canSupportManage && (
              <Button onClick={openAdd}>
                <FormattedMessage id="pages.tickets.action.create" defaultMessage="新建工单" />
              </Button>
            )}
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
              message.error(
                extractErrorMessage(
                  error,
                  intl.formatMessage({
                    id: 'pages.tickets.loadFailed',
                    defaultMessage: '加载工单失败',
                  }),
                ),
              );
              return { data: [], total: 0, success: false };
            }
          }}
          pagination={{ pageSize: 20, showSizeChanger: true }}
        />

        <ModalForm<TicketPayload>
          title={
            editing
              ? intl.formatMessage({
                  id: 'pages.tickets.modal.editTitle',
                  defaultMessage: '编辑工单',
                })
              : intl.formatMessage({
                  id: 'pages.tickets.modal.createTitle',
                  defaultMessage: '新建工单',
                })
          }
          open={open}
          onOpenChange={setOpen}
          modalProps={{ destroyOnHidden: true }}
          width={520}
          layout="vertical"
          submitter={{
            searchConfig: {
              submitText: intl.formatMessage({
                id: 'pages.tickets.modal.submit',
                defaultMessage: '确定',
              }),
            },
          }}
          initialValues={editing ?? { priority: 'normal', status: 'open' }}
          onFinish={onFinish}
        >
          <Form.Item
            label={intl.formatMessage({ id: 'pages.tickets.field.title', defaultMessage: '标题' })}
            name="title"
            rules={[
              {
                required: true,
                message: intl.formatMessage({
                  id: 'pages.tickets.form.titleRequired',
                  defaultMessage: '请输入标题',
                }),
              },
            ]}
          >
            {' '}
            <Input />{' '}
          </Form.Item>
          <Form.Item
            label={intl.formatMessage({
              id: 'pages.tickets.field.content',
              defaultMessage: '内容',
            })}
            name="content"
          >
            {' '}
            <Input.TextArea rows={4} />{' '}
          </Form.Item>
          <Form.Item
            label={intl.formatMessage({
              id: 'pages.tickets.field.category',
              defaultMessage: '分类',
            })}
            name="category"
          >
            {' '}
            <Input />{' '}
          </Form.Item>
          <Form.Item
            label={intl.formatMessage({
              id: 'pages.tickets.field.priority',
              defaultMessage: '优先级',
            })}
            name="priority"
          >
            {' '}
            <Select
              options={[
                { label: intl.formatMessage(priorityLabels.low), value: 'low' },
                { label: intl.formatMessage(priorityLabels.normal), value: 'normal' },
                { label: intl.formatMessage(priorityLabels.high), value: 'high' },
                { label: intl.formatMessage(priorityLabels.urgent), value: 'urgent' },
              ]}
            />{' '}
          </Form.Item>
          <Form.Item
            label={intl.formatMessage({ id: 'pages.tickets.field.status', defaultMessage: '状态' })}
            name="status"
          >
            {' '}
            <Select
              options={[
                { label: intl.formatMessage(statusLabels.open), value: 'open' },
                { label: intl.formatMessage(statusLabels.in_progress), value: 'in_progress' },
                { label: intl.formatMessage(statusLabels.resolved), value: 'resolved' },
                { label: intl.formatMessage(statusLabels.closed), value: 'closed' },
              ]}
            />{' '}
          </Form.Item>
          <Form.Item
            label={intl.formatMessage({
              id: 'pages.tickets.field.assignee',
              defaultMessage: '处理人',
            })}
            name="assignee"
          >
            {' '}
            <Select
              allowClear
              showSearch
              options={users.map((u) => ({ label: u.username, value: u.username }))}
            />{' '}
          </Form.Item>
          <Form.Item
            label={intl.formatMessage({ id: 'pages.tickets.field.tags', defaultMessage: '标签' })}
            name="tags"
          >
            {' '}
            <Input placeholder="," />{' '}
          </Form.Item>
          <Form.Item
            label={intl.formatMessage({
              id: 'pages.tickets.field.playerId',
              defaultMessage: '玩家ID',
            })}
            name="playerId"
          >
            {' '}
            <Input />{' '}
          </Form.Item>
          <Form.Item
            label={intl.formatMessage({
              id: 'pages.tickets.field.contact',
              defaultMessage: '联系方式',
            })}
            name="contact"
          >
            {' '}
            <Input />{' '}
          </Form.Item>
          <Form.Item
            label={intl.formatMessage({ id: 'pages.tickets.field.gameId', defaultMessage: '游戏' })}
            name="gameId"
          >
            {' '}
            <Input />{' '}
          </Form.Item>
          <Form.Item
            label={intl.formatMessage({ id: 'pages.tickets.field.env', defaultMessage: '环境' })}
            name="env"
          >
            {' '}
            <Input />{' '}
          </Form.Item>
          <Form.Item
            label={intl.formatMessage({ id: 'pages.tickets.field.source', defaultMessage: '来源' })}
            name="source"
          >
            {' '}
            <Input />{' '}
          </Form.Item>
        </ModalForm>
      </Card>
    </PageContainer>
  );
}
