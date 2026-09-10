import React, { useRef, useState } from 'react';
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
  Tag,
  Typography,
  Upload,
} from 'antd';
import {
  ModalForm,
  PageContainer,
  ProTable,
  type ActionType,
  type ProColumns,
} from '@ant-design/pro-components';
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

/** 热更单表单值：gameId 由请求拦截器 X-Game-ID header 注入，表单不产生该字段 */
type HotpatchFormValues = {
  title: string;
  bugId: number;
  framework: string;
};

export default function DevHotpatchesPage() {
  const { message } = App.useApp();
  const access = useAccess();
  const canManage = Boolean(access.canDevManage);

  const [status, setStatus] = useState('');
  const [framework, setFramework] = useState('');
  const [createOpen, setCreateOpen] = useState(false);
  const [rollTarget, setRollTarget] = useState<HotpatchItem | null>(null);
  const [rollValue, setRollValue] = useState(10);
  const actionRef = useRef<ActionType | undefined>(undefined);
  // 刷新按钮的 loading 转由表格加载态驱动
  const [tableLoading, setTableLoading] = useState(false);

  const reload = () => actionRef.current?.reload();

  const onFinish = async (v: HotpatchFormValues) => {
    try {
      // createHotpatch 契约要求 gameId 必填，但实际路由依赖 X-Game-ID header；
      // 原实现 body 即不含 gameId（Go json 解析缺省同为零值 ""），显式空串等价
      await createHotpatch({ ...v, gameId: '' });
      message.success('热更单已创建（草稿），请上传补丁包');
      reload();
      return true;
    } catch (error) {
      message.error(extractErrorMessage(error, '创建失败'));
      return false;
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
      reload();
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
        reload();
      } catch (error) {
        onError?.(error as Error);
        message.error(extractErrorMessage(error, '上传失败'));
      }
    },
  });

  const columns: ProColumns<HotpatchItem>[] = [
    { title: 'ID', dataIndex: 'id', width: 60 },
    {
      title: '框架',
      dataIndex: 'framework',
      width: 130,
      render: (_, hp) => hotpatchFrameworkLabels[hp.framework] || hp.framework,
    },
    { title: '关联缺陷', dataIndex: 'bugId', width: 90, render: (_, hp) => `#${hp.bugId}` },
    {
      title: '状态',
      dataIndex: 'status',
      width: 100,
      render: (_, hp) => (
        <Tag color={hotpatchStatusColors[hp.status] || 'default'}>
          {hotpatchStatusLabels[hp.status] || hp.status}
        </Tag>
      ),
    },
    {
      title: '灰度',
      dataIndex: 'rolloutPercent',
      width: 80,
      render: (_, hp) =>
        hp.status === 'rolling' ? (
          <Text strong>{hp.rolloutPercent}%</Text>
        ) : hp.rolloutPercent > 0 ? (
          `${hp.rolloutPercent}%`
        ) : (
          '-'
        ),
    },
    {
      title: '补丁包',
      dataIndex: 'size',
      width: 100,
      render: (_, hp) =>
        hp.packageKey ? formatSize(hp.size) : <Text type="secondary">未上传</Text>,
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
                // 筛选变化回第 1 页：params 变化与 setPageInfo 的双触发由
                // ProTable 内部 debounce + abort 合并，不会出现错序数据
                actionRef.current?.setPageInfo?.({ current: 1 });
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
                actionRef.current?.setPageInfo?.({ current: 1 });
              }}
              allowClear
              style={{ width: 160 }}
              options={Object.entries(hotpatchFrameworkLabels).map(([value, label]) => ({
                label,
                value,
              }))}
            />
            <Button icon={<ReloadOutlined />} onClick={reload} loading={tableLoading}>
              刷新
            </Button>
            {canManage ? (
              <Button type="primary" icon={<PlusOutlined />} onClick={() => setCreateOpen(true)}>
                创建热更单
              </Button>
            ) : null}
          </Space>
        }
      >
        <ProTable<HotpatchItem>
          actionRef={actionRef}
          rowKey="id"
          columns={columns}
          search={false}
          options={false}
          toolBarRender={false}
          params={{ status, framework }}
          request={async ({ current = 1, pageSize = 20, status: statusFilter, framework: fw }) => {
            try {
              const res = await listHotpatches({
                status: statusFilter ?? '',
                framework: fw ?? '',
                page: current,
                pageSize,
              });
              return { data: res.items || [], total: res.total || 0, success: true };
            } catch (error) {
              message.error(extractErrorMessage(error, '加载热更单失败'));
              return { data: [], total: 0, success: false };
            }
          }}
          onLoadingChange={(loading) => setTableLoading(loading === true)}
          pagination={{ pageSize: 20, showSizeChanger: true }}
        />
      </Card>

      <ModalForm<HotpatchFormValues>
        title="创建热更单"
        open={createOpen}
        onOpenChange={setCreateOpen}
        modalProps={{ destroyOnHidden: true }}
        width={520}
        layout="vertical"
        submitter={{ searchConfig: { submitText: '创建' } }}
        initialValues={{ framework: 'skynet' }}
        onFinish={onFinish}
      >
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
      </ModalForm>

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
