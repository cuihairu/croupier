import React, { useRef, useState } from 'react';
import { App, Card, Space, Button, Input, Switch, Modal, Form } from 'antd';
import {
  ModalForm,
  PageContainer,
  ProTable,
  type ActionType,
  type ProColumns,
} from '@ant-design/pro-components';
import { listFAQ, createFAQ, updateFAQ, deleteFAQ, type FAQPayload } from '@/services/api/support';
import { useAccess } from '@umijs/max';
import type { JSONValue } from '@/types/dashboard';
import { extractErrorMessage } from '@/utils/errors';
import { formatDateTime } from '@/utils/format';

interface FAQItem {
  id: number;
  question: string;
  answer: string;
  category?: string;
  tags?: string[];
  visible?: boolean;
  sort?: number;
  updatedAt?: string;
  [key: string]: JSONValue | undefined;
}

interface AccessState {
  canSupportManage?: boolean;
}

export default function SupportFAQPage() {
  const { message } = App.useApp();
  const actionRef = useRef<ActionType | undefined>(undefined);
  const [q, setQ] = useState('');
  const [category, setCategory] = useState('');
  const [visible, setVisible] = useState<string>('');
  const [open, setOpen] = useState(false);
  const [editing, setEditing] = useState<FAQItem | null>(null);
  const access: AccessState = useAccess?.() || {};

  // destroyOnHidden 使弹窗每次关闭即卸载表单，重开时按最新 initialValues
  // 重新挂载，新增/编辑切换不会残留上一次的预填值
  const openAdd = () => {
    setEditing(null);
    setOpen(true);
  };
  const openEdit = (rec: FAQItem) => {
    setEditing(rec);
    setOpen(true);
  };
  const onFinish = async (v: FAQPayload) => {
    try {
      if (editing) {
        await updateFAQ(editing.id, v);
      } else {
        await createFAQ(v);
      }
      actionRef.current?.reload();
      return true;
    } catch {
      // 原实现无本地弹错（全局拦截器已 toast），失败时弹窗保持开启
      return false;
    }
  };
  const onDelete = (rec: FAQItem) => {
    Modal.confirm({
      title: '删除 FAQ',
      onOk: async () => {
        await deleteFAQ(rec.id);
        actionRef.current?.reload();
      },
    });
  };

  const columns: ProColumns<FAQItem>[] = [
    { title: '问题', dataIndex: 'question', ellipsis: true },
    { title: '分类', dataIndex: 'category' },
    { title: '标签', dataIndex: 'tags' },
    { title: '可见', dataIndex: 'visible', render: (_, row) => (row.visible ? '是' : '否') },
    { title: '排序', dataIndex: 'sort' },
    {
      title: '更新时间',
      dataIndex: 'updatedAt',
      render: (_, row) => formatDateTime(row.updatedAt ?? ''),
    },
    {
      title: '操作',
      render: (_: unknown, r: FAQItem) => (
        <Space>
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
        title="常见问题（FAQ）"
        extra={
          <Space>
            <Input
              placeholder="关键词"
              value={q}
              onChange={(e) => {
                setQ(e.target.value);
                // 原实现筛选变化即回第 1 页，保持该语义；params 变化与
                // setPageInfo 的双触发由 ProTable 内部 debounce + abort 合并
                actionRef.current?.setPageInfo?.({ current: 1 });
              }}
              style={{ width: 200 }}
            />
            <Input
              placeholder="分类"
              value={category}
              onChange={(e) => {
                setCategory(e.target.value);
                actionRef.current?.setPageInfo?.({ current: 1 });
              }}
              style={{ width: 140 }}
            />
            <Input
              placeholder="是否可见(true/false)"
              value={visible}
              onChange={(e) => {
                setVisible(e.target.value);
                actionRef.current?.setPageInfo?.({ current: 1 });
              }}
              style={{ width: 180 }}
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
            {access.canSupportManage && <Button onClick={openAdd}>新建 FAQ</Button>}
          </Space>
        }
      >
        <ProTable<FAQItem>
          actionRef={actionRef}
          rowKey="id"
          columns={columns}
          search={false}
          options={false}
          toolBarRender={false}
          params={{ q, category, visible }}
          request={async ({
            current = 1,
            pageSize = 10,
            q: qFilter,
            category: categoryFilter,
            visible: visibleFilter,
          }) => {
            try {
              const res = await listFAQ({
                q: qFilter ?? '',
                category: categoryFilter ?? '',
                visible: visibleFilter ?? '',
                page: current,
                pageSize,
              });
              const items = (res.faq || res.items || []) as unknown as FAQItem[];
              return { data: items, total: res.total ?? items.length, success: true };
            } catch (error) {
              message.error(extractErrorMessage(error, '加载 FAQ 失败'));
              return { data: [], total: 0, success: false };
            }
          }}
          pagination={{
            pageSize: 10,
            showSizeChanger: true,
            pageSizeOptions: [10, 20, 50],
            showTotal: (t) => `共 ${t} 条`,
          }}
        />
        <ModalForm<FAQPayload>
          title={editing ? '编辑 FAQ' : '新建 FAQ'}
          open={open}
          onOpenChange={setOpen}
          modalProps={{ destroyOnHidden: true }}
          width={520}
          submitter={{ searchConfig: { submitText: '确定' } }}
          initialValues={editing ?? { visible: true, sort: 0 }}
          onFinish={onFinish}
        >
          <Form.Item
            label="问题"
            name="question"
            rules={[{ required: true, message: '请输入问题' }]}
          >
            {' '}
            <Input.TextArea rows={3} />{' '}
          </Form.Item>
          <Form.Item label="答案" name="answer" rules={[{ required: true, message: '请输入答案' }]}>
            {' '}
            <Input.TextArea rows={6} />{' '}
          </Form.Item>
          <Form.Item label="分类" name="category">
            {' '}
            <Input />{' '}
          </Form.Item>
          <Form.Item label="标签" name="tags">
            {' '}
            <Input placeholder="," />{' '}
          </Form.Item>
          <Form.Item label="可见" name="visible" valuePropName="checked">
            {' '}
            <Switch />{' '}
          </Form.Item>
          <Form.Item label="排序" name="sort">
            {' '}
            <Input type="number" />{' '}
          </Form.Item>
        </ModalForm>
      </Card>
    </PageContainer>
  );
}
