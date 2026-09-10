import React, { useRef, useState } from 'react';
import { Card, Space, Button, Input, Select, Modal, Form } from 'antd';
import {
  ModalForm,
  PageContainer,
  ProTable,
  type ActionType,
  type ProColumns,
} from '@ant-design/pro-components';
import {
  listFeedback,
  createFeedback,
  convertFeedbackToTicket,
  updateFeedback,
  deleteFeedback,
  type FeedbackPayload,
} from '@/services/api/support';
import { getMessage } from '@/utils/antdApp';
import { extractErrorMessage } from '@/utils/errors';
import { formatDateTime } from '@/utils/format';
import { useAccess } from '@umijs/max';
import type { JSONValue } from '@/types/dashboard';

interface FeedbackItem {
  id: number;
  playerId?: string;
  contact?: string;
  category?: string;
  priority?: string;
  status?: string;
  gameId?: string;
  env?: string;
  content?: string;
  updatedAt?: string;
  [key: string]: JSONValue | undefined;
}

interface AccessState {
  canSupportManage?: boolean;
}

export default function SupportFeedbackPage() {
  const actionRef = useRef<ActionType | undefined>(undefined);
  const [q, setQ] = useState('');
  const [category, setCategory] = useState('');
  const [status, setStatus] = useState('');
  const [gameId, setGameId] = useState('');
  const [pendingOnly, setPendingOnly] = useState(true);
  const [open, setOpen] = useState(false);
  const [editing, setEditing] = useState<FeedbackItem | null>(null);
  const access: AccessState = useAccess?.() || {};

  // destroyOnHidden 使弹窗每次关闭即卸载表单，重开时按最新 initialValues
  // 重新挂载，新增/编辑切换不会残留上一次的预填值
  const openAdd = () => {
    setEditing(null);
    setOpen(true);
  };
  const openEdit = (rec: FeedbackItem) => {
    setEditing(rec);
    setOpen(true);
  };
  const onFinish = async (v: FeedbackPayload) => {
    try {
      if (editing) {
        await updateFeedback(editing.id, v);
      } else {
        await createFeedback(v);
      }
      actionRef.current?.reload();
      return true;
    } catch {
      // 原实现无本地弹错（全局请求拦截器已 toast），失败时弹窗保持开启
      return false;
    }
  };
  const onDelete = (rec: FeedbackItem) => {
    Modal.confirm({
      title: '删除反馈',
      onOk: async () => {
        await deleteFeedback(rec.id);
        actionRef.current?.reload();
      },
    });
  };

  const columns: ProColumns<FeedbackItem>[] = [
    { title: '玩家ID', dataIndex: 'playerId' },
    { title: '联系方式', dataIndex: 'contact' },
    { title: '分类', dataIndex: 'category' },
    { title: '优先级', dataIndex: 'priority' },
    { title: '状态', dataIndex: 'status' },
    {
      title: '游戏/环境',
      render: (_: unknown, r: FeedbackItem) => `${r.gameId || ''}/${r.env || ''}`,
    },
    { title: '内容', dataIndex: 'content', ellipsis: true },
    {
      title: '更新时间',
      dataIndex: 'updatedAt',
      render: (_, row) => formatDateTime(row.updatedAt ?? ''),
    },
    {
      title: '操作',
      render: (_: unknown, r: FeedbackItem) => (
        <Space>
          <Button
            size="small"
            disabled={r.status === 'triaged'}
            onClick={async () => {
              try {
                const res = await convertFeedbackToTicket(r.id, {
                  note: `来源反馈#${r.id} 玩家:${r.playerId || '未知'}`,
                });
                getMessage()?.success(
                  res.alreadyConverted
                    ? `该反馈已转过工单 #${res.ticketId}`
                    : `已转工单 #${res.ticketId}`,
                );
                actionRef.current?.reload();
              } catch (e) {
                getMessage()?.error(extractErrorMessage(e, '转工单失败'));
              }
            }}
          >
            {r.status === 'triaged' ? '已转工单' : '转工单'}
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
  ];

  return (
    <PageContainer>
      <Card
        title="玩家反馈"
        extra={
          <Space>
            <Input
              placeholder="关键词"
              value={q}
              onChange={(e) => setQ(e.target.value)}
              style={{ width: 200 }}
            />
            <Input
              placeholder="分类"
              value={category}
              onChange={(e) => setCategory(e.target.value)}
              style={{ width: 140 }}
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
                { label: '新建', value: 'new' },
                { label: '已分流', value: 'triaged' },
                { label: '已关闭', value: 'closed' },
              ]}
            />
            <Input
              placeholder="游戏"
              value={gameId}
              onChange={(e) => setGameId(e.target.value)}
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
            {access.canSupportManage && <Button onClick={openAdd}>新建反馈</Button>}
          </Space>
        }
      >
        <ProTable<FeedbackItem>
          actionRef={actionRef}
          rowKey="id"
          columns={columns}
          search={false}
          options={false}
          toolBarRender={false}
          params={{ q, category, status, gameId, pendingOnly }}
          request={async ({
            current = 1,
            pageSize = 20,
            q: qFilter,
            category: categoryFilter,
            status: statusFilter,
            gameId: gameIdFilter,
            pendingOnly: pendingOnlyFlag,
          }) => {
            try {
              const res = await listFeedback({
                q: qFilter ?? '',
                category: categoryFilter ?? '',
                status: statusFilter ?? '',
                gameId: gameIdFilter ?? '',
                page: current,
                size: pageSize,
                // 分诊队列定位：默认隐藏已转工单的反馈，避免与工单列表重复
                excludeStatus: pendingOnlyFlag && !statusFilter ? 'triaged' : undefined,
              });
              return {
                data: (res.feedback || []) as unknown as FeedbackItem[],
                total: res.total || 0,
                success: true,
              };
            } catch (error) {
              getMessage()?.error(extractErrorMessage(error, '加载反馈失败'));
              return { data: [], total: 0, success: false };
            }
          }}
          pagination={{ pageSize: 20, showSizeChanger: true }}
        />

        <ModalForm<FeedbackPayload>
          title={editing ? '编辑反馈' : '新建反馈'}
          open={open}
          onOpenChange={setOpen}
          modalProps={{ destroyOnHidden: true }}
          width={520}
          submitter={{ searchConfig: { submitText: '确定' } }}
          initialValues={editing ?? { priority: 'normal', status: 'new' }}
          onFinish={onFinish}
        >
          <Form.Item label="玩家ID" name="playerId">
            <Input />
          </Form.Item>
          <Form.Item label="联系方式" name="contact">
            <Input />
          </Form.Item>
          <Form.Item
            label="内容"
            name="content"
            rules={[{ required: true, message: '请输入内容' }]}
          >
            <Input.TextArea rows={4} />
          </Form.Item>
          <Form.Item label="分类" name="category">
            <Input />
          </Form.Item>
          <Form.Item label="优先级" name="priority">
            <Select
              options={[
                { label: '低', value: 'low' },
                { label: '普通', value: 'normal' },
                { label: '高', value: 'high' },
              ]}
            />
          </Form.Item>
          <Form.Item label="状态" name="status">
            <Select
              options={[
                { label: '新建', value: 'new' },
                { label: '已分流', value: 'triaged' },
                { label: '已关闭', value: 'closed' },
              ]}
            />
          </Form.Item>
          <Form.Item label="附件(JSON)" name="attach">
            <Input.TextArea rows={2} />
          </Form.Item>
          <Form.Item label="游戏" name="gameId">
            <Input />
          </Form.Item>
          <Form.Item label="环境" name="env">
            <Input />
          </Form.Item>
        </ModalForm>
      </Card>
    </PageContainer>
  );
}
