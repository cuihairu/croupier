import React, { useCallback, useEffect, useState } from 'react';
import {
  App,
  Button,
  Card,
  Form,
  Input,
  InputNumber,
  Modal,
  Popconfirm,
  Select,
  Slider,
  Space,
  Table,
  Tag,
  Typography,
  Upload,
} from 'antd';
import { PageContainer } from '@ant-design/pro-components';
import { CloudUploadOutlined, PlusOutlined, ReloadOutlined } from '@ant-design/icons';
import type { UploadProps } from 'antd';
import { useAccess } from '@umijs/max';
import {
  createHotpatch,
  hotpatchFrameworkLabels,
  hotpatchStatusColors,
  hotpatchStatusLabels,
  listHotpatches,
  transitionHotpatch,
  uploadHotpatchPackage,
  type HotpatchItem,
} from '@/services/api/hotpatches';
import { extractErrorMessage } from '@/utils/errors';

const { Text } = Typography;

export default function DevHotpatchesPage() {
  const { message } = App.useApp();
  const access = useAccess();
  const canManage = Boolean(access.canDevManage);

  const [rows, setRows] = useState<HotpatchItem[]>([]);
  const [loading, setLoading] = useState(false);
  const [page, setPage] = useState(1);
  const [size, setSize] = useState(20);
  const [total, setTotal] = useState(0);
  const [status, setStatus] = useState('');
  const [framework, setFramework] = useState('');
  const [createOpen, setCreateOpen] = useState(false);
  const [saving, setSaving] = useState(false);
  const [form] = Form.useForm();
  const [rollTarget, setRollTarget] = useState<HotpatchItem | null>(null);
  const [rollValue, setRollValue] = useState(10);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const res = await listHotpatches({ status, framework, page, pageSize: size });
      setRows(res.items || []);
      setTotal(res.total || 0);
    } catch (error) {
      message.error(extractErrorMessage(error, '加载热更单失败'));
    } finally {
      setLoading(false);
    }
  }, [message, status, framework, page, size]);

  useEffect(() => {
    load();
  }, [load]);

  const submitCreate = async () => {
    const v = await form.validateFields();
    setSaving(true);
    try {
      await createHotpatch(v);
      message.success('热更单已创建（草稿），请上传补丁包');
      setCreateOpen(false);
      load();
    } catch (error) {
      message.error(extractErrorMessage(error, '创建失败'));
    } finally {
      setSaving(false);
    }
  };

  const doTransition = async (
    hp: HotpatchItem,
    action: 'approve' | 'roll' | 'applied' | 'fail' | 'rollback',
    rolloutPercent?: number,
  ) => {
    try {
      await transitionHotpatch(hp.id, action, rolloutPercent);
      message.success('状态已更新');
      load();
    } catch (error) {
      message.error(extractErrorMessage(error, '操作失败'));
    }
  };

  const uploadProps = (hp: HotpatchItem): UploadProps => ({
    showUploadList: false,
    maxCount: 1,
    customRequest: async (options) => {
      const { file, onSuccess, onError } = options;
      try {
        await uploadHotpatchPackage(hp.id, file as File);
        onSuccess?.({}, new XMLHttpRequest());
        message.success('补丁包已上传（SHA-256 已登记）');
        load();
      } catch (error) {
        onError?.(error as Error);
        message.error(extractErrorMessage(error, '上传失败'));
      }
    },
  });

  const columns = [
    { title: 'ID', dataIndex: 'id', width: 60 },
    {
      title: '框架',
      dataIndex: 'framework',
      width: 130,
      render: (v: string) => hotpatchFrameworkLabels[v] || v,
    },
    { title: '关联缺陷', dataIndex: 'bugId', width: 90, render: (v: number) => `#${v}` },
    {
      title: '状态',
      dataIndex: 'status',
      width: 100,
      render: (v: string) => (
        <Tag color={hotpatchStatusColors[v] || 'default'}>{hotpatchStatusLabels[v] || v}</Tag>
      ),
    },
    {
      title: '灰度',
      dataIndex: 'rolloutPercent',
      width: 80,
      render: (v: number, hp: HotpatchItem) =>
        hp.status === 'rolling' ? <Text strong>{v}%</Text> : v > 0 ? `${v}%` : '-',
    },
    {
      title: '补丁包',
      dataIndex: 'size',
      width: 100,
      render: (v: number, hp: HotpatchItem) =>
        hp.packageKey ? formatSize(v) : <Text type="secondary">未上传</Text>,
    },
    { title: '更新时间', dataIndex: 'updatedAt', width: 170 },
    {
      title: '操作',
      render: (_: unknown, hp: HotpatchItem) =>
        canManage ? (
          <Space wrap>
            {hp.status === 'draft' ? (
              <Upload {...uploadProps(hp)}>
                <Button size="small" icon={<CloudUploadOutlined />}>
                  传包
                </Button>
              </Upload>
            ) : null}
            {hp.status === 'draft' && hp.packageKey ? (
              <Popconfirm
                title="提交审批？（双人规则：需第二人复核后才能灰度）"
                onConfirm={() => doTransition(hp, 'approve')}
              >
                <Button size="small" type="primary">
                  提交审批
                </Button>
              </Popconfirm>
            ) : null}
            {hp.status === 'approved' ? (
              <Button
                size="small"
                type="primary"
                onClick={() => {
                  setRollTarget(hp);
                  setRollValue(10);
                }}
              >
                开始灰度
              </Button>
            ) : null}
            {hp.status === 'rolling' ? (
              <>
                <Button
                  size="small"
                  onClick={() => {
                    setRollTarget(hp);
                    setRollValue(Math.max(hp.rolloutPercent, 10));
                  }}
                >
                  放量
                </Button>
                <Button size="small" onClick={() => doTransition(hp, 'applied')}>
                  标记生效
                </Button>
              </>
            ) : null}
            {['rolling', 'failed'].includes(hp.status) ? (
              <Popconfirm
                title="回滚所有已应用节点？"
                onConfirm={() => doTransition(hp, 'rollback')}
              >
                <Button size="small" danger>
                  回滚
                </Button>
              </Popconfirm>
            ) : null}
          </Space>
        ) : (
          '-'
        ),
    },
  ];

  return (
    <PageContainer>
      <Card
        title="服务端热更新"
        extra={
          <Space wrap>
            <Select
              placeholder="状态"
              value={status || undefined}
              onChange={(v) => {
                setStatus(v || '');
                setPage(1);
              }}
              allowClear
              style={{ width: 110 }}
              options={Object.entries(hotpatchStatusLabels).map(([value, label]) => ({
                label,
                value,
              }))}
            />
            <Select
              placeholder="框架"
              value={framework || undefined}
              onChange={(v) => {
                setFramework(v || '');
                setPage(1);
              }}
              allowClear
              style={{ width: 160 }}
              options={Object.entries(hotpatchFrameworkLabels).map(([value, label]) => ({
                label,
                value,
              }))}
            />
            <Button icon={<ReloadOutlined />} onClick={load} loading={loading}>
              刷新
            </Button>
            {canManage ? (
              <Button
                type="primary"
                icon={<PlusOutlined />}
                onClick={() => {
                  form.resetFields();
                  setCreateOpen(true);
                }}
              >
                创建热更单
              </Button>
            ) : null}
          </Space>
        }
      >
        <Table
          rowKey="id"
          dataSource={rows}
          loading={loading}
          columns={columns}
          pagination={{
            current: page,
            pageSize: size,
            total,
            showSizeChanger: true,
            onChange: (p, s) => {
              setPage(p);
              setSize(s);
            },
          }}
        />
      </Card>

      <Modal
        title="创建热更单"
        open={createOpen}
        onCancel={() => setCreateOpen(false)}
        footer={
          <Space>
            <Button onClick={() => setCreateOpen(false)}>取消</Button>
            <Button type="primary" loading={saving} onClick={submitCreate}>
              创建
            </Button>
          </Space>
        }
        destroyOnHidden
      >
        <Form form={form} layout="vertical" initialValues={{ framework: 'skynet' }}>
          <Form.Item name="title" label="标题" rules={[{ required: true, message: '请输入标题' }]}>
            <Input placeholder="如：修复背包闪退" />
          </Form.Item>
          <Form.Item
            name="bugId"
            label="关联缺陷编号"
            rules={[{ required: true, message: '热更必须关联缺陷（可追溯）' }]}
          >
            <InputNumber min={1} style={{ width: '100%' }} placeholder="缺陷追踪里的 Bug ID" />
          </Form.Item>
          <Form.Item name="framework" label="目标框架" rules={[{ required: true }]}>
            <Select
              options={Object.entries(hotpatchFrameworkLabels).map(([value, label]) => ({
                label,
                value,
              }))}
            />
          </Form.Item>
        </Form>
      </Modal>

      <Modal
        title={rollTarget ? `节点灰度放量（当前 ${rollTarget.rolloutPercent}%）` : ''}
        open={Boolean(rollTarget)}
        onCancel={() => setRollTarget(null)}
        footer={
          <Space>
            <Button onClick={() => setRollTarget(null)}>取消</Button>
            <Button
              type="primary"
              onClick={async () => {
                if (!rollTarget) return;
                await doTransition(rollTarget, 'roll', rollValue);
                setRollTarget(null);
              }}
            >
              确认放量
            </Button>
          </Space>
        }
        destroyOnHidden
      >
        <Text>按节点 hash 分桶，同一节点结果稳定；放量只增不减。</Text>
        <Slider
          min={rollTarget ? rollTarget.rolloutPercent : 0}
          max={100}
          step={5}
          value={rollValue}
          onChange={setRollValue}
          marks={{ 10: '10%', 50: '50%', 100: '100%' }}
        />
      </Modal>
    </PageContainer>
  );
}

function formatSize(bytes?: number): string {
  if (!bytes) return '-';
  if (bytes > 1024 * 1024) return `${(bytes / 1024 / 1024).toFixed(1)} MB`;
  return `${(bytes / 1024).toFixed(1)} KB`;
}
