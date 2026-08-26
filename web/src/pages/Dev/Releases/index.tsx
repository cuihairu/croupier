import React, { useCallback, useEffect, useState } from 'react';
import {
  App,
  Button,
  Card,
  Form,
  Input,
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
import { useAccess } from '@umijs/max';
import type { UploadProps } from 'antd';
import {
  createRelease,
  listReleases,
  releasePlatformLabels,
  releaseStatusColors,
  releaseStatusLabels,
  releaseTypeLabels,
  transitionRelease,
  uploadReleaseArtifact,
  type Release,
} from '@/services/api/releases';
import { extractErrorMessage } from '@/utils/errors';

const { Text } = Typography;

function formatSize(bytes?: number): string {
  if (!bytes) return '-';
  if (bytes > 1024 * 1024) return `${(bytes / 1024 / 1024).toFixed(1)} MB`;
  return `${(bytes / 1024).toFixed(1)} KB`;
}

export default function DevReleasesPage() {
  const { message } = App.useApp();
  const access = useAccess();
  const canManage = Boolean(access.canDevManage);

  const [rows, setRows] = useState<Release[]>([]);
  const [loading, setLoading] = useState(false);
  const [page, setPage] = useState(1);
  const [size, setSize] = useState(20);
  const [total, setTotal] = useState(0);
  const [status, setStatus] = useState('');
  const [platform, setPlatform] = useState('');
  const [createOpen, setCreateOpen] = useState(false);
  const [saving, setSaving] = useState(false);
  const [form] = Form.useForm();
  const [grayTarget, setGrayTarget] = useState<Release | null>(null);
  const [grayValue, setGrayValue] = useState(10);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const res = await listReleases({ status, platform, page, pageSize: size });
      setRows(res.items || []);
      setTotal(res.total || 0);
    } catch (error) {
      message.error(extractErrorMessage(error, '加载版本列表失败'));
    } finally {
      setLoading(false);
    }
  }, [message, status, platform, page, size]);

  useEffect(() => {
    load();
  }, [load]);

  const submitCreate = async () => {
    const v = await form.validateFields();
    setSaving(true);
    try {
      await createRelease(v);
      message.success('版本已创建（草稿）');
      setCreateOpen(false);
      load();
    } catch (error) {
      message.error(extractErrorMessage(error, '创建失败'));
    } finally {
      setSaving(false);
    }
  };

  const doTransition = async (
    rel: Release,
    action: 'testing' | 'gray' | 'full' | 'archive' | 'rollback',
    grayPercent?: number,
  ) => {
    try {
      await transitionRelease(rel.id, action, grayPercent);
      message.success('状态已更新');
      load();
    } catch (error) {
      message.error(extractErrorMessage(error, '操作失败'));
    }
  };

  const uploadProps = (rel: Release): UploadProps => ({
    showUploadList: false,
    maxCount: 1,
    customRequest: async (options) => {
      const { file, onSuccess, onError } = options;
      try {
        await uploadReleaseArtifact(rel.id, file as File);
        onSuccess?.({}, new XMLHttpRequest());
        message.success('资源包已上传');
        load();
      } catch (error) {
        onError?.(error as Error);
        message.error(extractErrorMessage(error, '上传失败'));
      }
    },
  });

  const columns = [
    { title: '版本', dataIndex: 'version', width: 100 },
    {
      title: '渠道/平台',
      width: 140,
      render: (_: unknown, rel: Release) => (
        <Space size={4}>
          <Tag>{rel.channel}</Tag>
          <Tag color="geekblue">{releasePlatformLabels[rel.platform] || rel.platform}</Tag>
        </Space>
      ),
    },
    {
      title: '类型',
      dataIndex: 'type',
      width: 70,
      render: (v: string) => releaseTypeLabels[v] || v,
    },
    {
      title: '状态',
      dataIndex: 'status',
      width: 90,
      render: (v: string) => (
        <Tag color={releaseStatusColors[v] || 'default'}>{releaseStatusLabels[v] || v}</Tag>
      ),
    },
    {
      title: '灰度',
      dataIndex: 'grayPercent',
      width: 90,
      render: (v: number, rel: Release) =>
        rel.status === 'gray' ? <Text strong>{v}%</Text> : v > 0 ? `${v}%` : '-',
    },
    {
      title: '资源包',
      dataIndex: 'size',
      width: 100,
      render: (v: number, rel: Release) =>
        rel.objectKey ? (
          <Text title={rel.checksum}>{formatSize(v)}</Text>
        ) : (
          <Text type="secondary">未上传</Text>
        ),
    },
    { title: '更新时间', dataIndex: 'updatedAt', width: 170 },
    {
      title: '操作',
      render: (_: unknown, rel: Release) =>
        canManage ? (
          <Space wrap>
            {rel.status === 'draft' ? (
              <Upload {...uploadProps(rel)}>
                <Button size="small" icon={<CloudUploadOutlined />}>
                  传包
                </Button>
              </Upload>
            ) : null}
            {rel.status === 'uploading' ? (
              <Popconfirm
                title="进入内测（仅白名单设备可获取）？"
                onConfirm={() => doTransition(rel, 'testing')}
              >
                <Button size="small" type="primary">
                  内测
                </Button>
              </Popconfirm>
            ) : null}
            {rel.status === 'testing' ? (
              <Button
                size="small"
                type="primary"
                onClick={() => {
                  setGrayTarget(rel);
                  setGrayValue(10);
                }}
              >
                开始灰度
              </Button>
            ) : null}
            {rel.status === 'gray' ? (
              <>
                <Button
                  size="small"
                  onClick={() => {
                    setGrayTarget(rel);
                    setGrayValue(Math.max(rel.grayPercent, 10));
                  }}
                >
                  放量
                </Button>
                <Popconfirm title="直接全量发布？" onConfirm={() => doTransition(rel, 'full')}>
                  <Button size="small" type="primary">
                    全量
                  </Button>
                </Popconfirm>
              </>
            ) : null}
            {rel.status === 'full' ? (
              <Popconfirm
                title="回滚后客户端将取不到该版本，确认？"
                onConfirm={() => doTransition(rel, 'rollback')}
              >
                <Button size="small" danger>
                  回滚
                </Button>
              </Popconfirm>
            ) : null}
            {['draft', 'uploading', 'testing', 'gray'].includes(rel.status) ? (
              <Popconfirm title="废弃该版本？" onConfirm={() => doTransition(rel, 'archive')}>
                <Button size="small" type="text" danger>
                  废弃
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
        title="版本发布"
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
              options={Object.entries(releaseStatusLabels).map(([value, label]) => ({
                label,
                value,
              }))}
            />
            <Select
              placeholder="平台"
              value={platform || undefined}
              onChange={(v) => {
                setPlatform(v || '');
                setPage(1);
              }}
              allowClear
              style={{ width: 110 }}
              options={Object.entries(releasePlatformLabels).map(([value, label]) => ({
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
                创建版本
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
        title="创建版本"
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
        <Form form={form} layout="vertical" initialValues={{ type: 'full' }}>
          <Form.Item
            name="version"
            label="版本号"
            rules={[{ required: true, message: '如 1.5.0' }]}
          >
            <Input placeholder="1.5.0" />
          </Form.Item>
          <Space>
            <Form.Item name="channel" label="渠道" initialValue="official">
              <Input style={{ width: 140 }} />
            </Form.Item>
            <Form.Item name="platform" label="平台" rules={[{ required: true }]}>
              <Select
                style={{ width: 120 }}
                options={Object.entries(releasePlatformLabels).map(([value, label]) => ({
                  label,
                  value,
                }))}
              />
            </Form.Item>
            <Form.Item name="type" label="类型">
              <Select
                style={{ width: 100 }}
                options={Object.entries(releaseTypeLabels).map(([value, label]) => ({
                  label,
                  value,
                }))}
              />
            </Form.Item>
          </Space>
        </Form>
      </Modal>

      <Modal
        title={
          grayTarget ? `灰度放量：${grayTarget.version}（当前 ${grayTarget.grayPercent}%）` : ''
        }
        open={Boolean(grayTarget)}
        onCancel={() => setGrayTarget(null)}
        footer={
          <Space>
            <Button onClick={() => setGrayTarget(null)}>取消</Button>
            <Button
              type="primary"
              onClick={async () => {
                if (!grayTarget) return;
                await doTransition(grayTarget, 'gray', grayValue);
                setGrayTarget(null);
              }}
            >
              确认放量
            </Button>
          </Space>
        }
        destroyOnHidden
      >
        <Text>放量只增不减；减少曝光请使用回滚。设备按 hash 分桶，同一设备结果稳定。</Text>
        <Slider
          min={grayTarget ? grayTarget.grayPercent : 0}
          max={100}
          step={5}
          value={grayValue}
          onChange={setGrayValue}
          marks={{ 10: '10%', 50: '50%', 100: '100%' }}
        />
      </Modal>
    </PageContainer>
  );
}
