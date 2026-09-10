import React, { useRef, useState } from 'react';
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
import { FormattedMessage, useAccess, useIntl } from '@umijs/max';
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

/** 版本表单值：gameId 由请求拦截器 X-Game-ID header 注入，表单不产生该字段 */
type ReleaseFormValues = {
  version: string;
  channel: string;
  platform: string;
  type?: string;
};

function formatSize(bytes?: number): string {
  if (!bytes) return '-';
  if (bytes > 1024 * 1024) return `${(bytes / 1024 / 1024).toFixed(1)} MB`;
  return `${(bytes / 1024).toFixed(1)} KB`;
}

export default function DevReleasesPage() {
  const { message } = App.useApp();
  const access = useAccess();
  const intl = useIntl();
  const canManage = Boolean(access.canDevManage);

  const [status, setStatus] = useState('');
  const [platform, setPlatform] = useState('');
  const [createOpen, setCreateOpen] = useState(false);
  const [grayTarget, setGrayTarget] = useState<Release | null>(null);
  const [grayValue, setGrayValue] = useState(10);
  const actionRef = useRef<ActionType | undefined>(undefined);
  // 刷新按钮的 loading 转由表格加载态驱动
  const [tableLoading, setTableLoading] = useState(false);

  const reload = () => actionRef.current?.reload();

  const onFinish = async (v: ReleaseFormValues) => {
    try {
      // createRelease 契约要求 gameId 必填，但实际路由依赖 X-Game-ID header；
      // 原实现 body 即不含 gameId（Go json 解析缺省同为零值 ""），显式空串等价
      await createRelease({ ...v, gameId: '' });
      message.success(
        intl.formatMessage({
          id: 'pages.devReleases.success.created',
          defaultMessage: '版本已创建（草稿）',
        }),
      );
      reload();
      return true;
    } catch (error) {
      message.error(
        extractErrorMessage(
          error,
          intl.formatMessage({
            id: 'pages.devReleases.error.createFailed',
            defaultMessage: '创建失败',
          }),
        ),
      );
      return false;
    }
  };

  const doTransition = async (
    rel: Release,
    action: 'testing' | 'gray' | 'full' | 'archive' | 'rollback',
    grayPercent?: number,
  ) => {
    try {
      await transitionRelease(rel.id, action, grayPercent);
      message.success(
        intl.formatMessage({
          id: 'pages.devReleases.success.statusUpdated',
          defaultMessage: '状态已更新',
        }),
      );
      reload();
    } catch (error) {
      message.error(
        extractErrorMessage(
          error,
          intl.formatMessage({
            id: 'pages.devReleases.error.operationFailed',
            defaultMessage: '操作失败',
          }),
        ),
      );
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
        message.success(
          intl.formatMessage({
            id: 'pages.devReleases.success.artifactUploaded',
            defaultMessage: '资源包已上传',
          }),
        );
        reload();
      } catch (error) {
        onError?.(error as Error);
        message.error(
          extractErrorMessage(
            error,
            intl.formatMessage({
              id: 'pages.devReleases.error.uploadFailed',
              defaultMessage: '上传失败',
            }),
          ),
        );
      }
    },
  });

  const columns: ProColumns<Release>[] = [
    {
      title: intl.formatMessage({
        id: 'pages.devReleases.column.version',
        defaultMessage: '版本',
      }),
      dataIndex: 'version',
      width: 100,
    },
    {
      title: intl.formatMessage({
        id: 'pages.devReleases.column.channelPlatform',
        defaultMessage: '渠道/平台',
      }),
      width: 140,
      render: (_: unknown, rel: Release) => (
        <Space size={4}>
          <Tag>{rel.channel}</Tag>
          <Tag color="geekblue">{releasePlatformLabels[rel.platform] || rel.platform}</Tag>
        </Space>
      ),
    },
    {
      title: intl.formatMessage({
        id: 'pages.devReleases.column.type',
        defaultMessage: '类型',
      }),
      dataIndex: 'type',
      width: 70,
      render: (_, rel) => releaseTypeLabels[rel.type] || rel.type,
    },
    {
      title: intl.formatMessage({
        id: 'pages.devReleases.column.status',
        defaultMessage: '状态',
      }),
      dataIndex: 'status',
      width: 90,
      render: (_, rel) => (
        <Tag color={releaseStatusColors[rel.status] || 'default'}>
          {releaseStatusLabels[rel.status] || rel.status}
        </Tag>
      ),
    },
    {
      title: intl.formatMessage({
        id: 'pages.devReleases.column.gray',
        defaultMessage: '灰度',
      }),
      dataIndex: 'grayPercent',
      width: 90,
      render: (_, rel) =>
        rel.status === 'gray' ? (
          <Text strong>{rel.grayPercent}%</Text>
        ) : rel.grayPercent > 0 ? (
          `${rel.grayPercent}%`
        ) : (
          '-'
        ),
    },
    {
      title: intl.formatMessage({
        id: 'pages.devReleases.column.artifact',
        defaultMessage: '资源包',
      }),
      dataIndex: 'size',
      width: 100,
      render: (_, rel) =>
        rel.objectKey ? (
          <Text title={rel.checksum}>{formatSize(rel.size)}</Text>
        ) : (
          <Text type="secondary">
            <FormattedMessage
              id="pages.devReleases.column.artifactNotUploaded"
              defaultMessage="未上传"
            />
          </Text>
        ),
    },
    {
      title: intl.formatMessage({
        id: 'pages.devReleases.column.updatedAt',
        defaultMessage: '更新时间',
      }),
      dataIndex: 'updatedAt',
      width: 170,
    },
    {
      title: intl.formatMessage({
        id: 'pages.devReleases.column.actions',
        defaultMessage: '操作',
      }),
      render: (_: unknown, rel: Release) =>
        canManage ? (
          <Space wrap>
            {rel.status === 'draft' ? (
              <Upload {...uploadProps(rel)}>
                <Button size="small" icon={<CloudUploadOutlined />}>
                  <FormattedMessage
                    id="pages.devReleases.action.uploadArtifact"
                    defaultMessage="传包"
                  />
                </Button>
              </Upload>
            ) : null}
            {rel.status === 'uploading' ? (
              <Popconfirm
                title={intl.formatMessage({
                  id: 'pages.devReleases.confirm.testing',
                  defaultMessage: '进入内测（仅白名单设备可获取）？',
                })}
                onConfirm={() => doTransition(rel, 'testing')}
              >
                <Button size="small" type="primary">
                  <FormattedMessage id="pages.devReleases.action.testing" defaultMessage="内测" />
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
                <FormattedMessage
                  id="pages.devReleases.action.startRollout"
                  defaultMessage="开始灰度"
                />
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
                  <FormattedMessage id="pages.devReleases.action.rollout" defaultMessage="放量" />
                </Button>
                <Popconfirm
                  title={intl.formatMessage({
                    id: 'pages.devReleases.confirm.full',
                    defaultMessage: '直接全量发布？',
                  })}
                  onConfirm={() => doTransition(rel, 'full')}
                >
                  <Button size="small" type="primary">
                    <FormattedMessage id="pages.devReleases.action.full" defaultMessage="全量" />
                  </Button>
                </Popconfirm>
              </>
            ) : null}
            {rel.status === 'full' ? (
              <Popconfirm
                title={intl.formatMessage({
                  id: 'pages.devReleases.confirm.rollback',
                  defaultMessage: '回滚后客户端将取不到该版本，确认？',
                })}
                onConfirm={() => doTransition(rel, 'rollback')}
              >
                <Button size="small" danger>
                  <FormattedMessage id="pages.devReleases.action.rollback" defaultMessage="回滚" />
                </Button>
              </Popconfirm>
            ) : null}
            {['draft', 'uploading', 'testing', 'gray'].includes(rel.status) ? (
              <Popconfirm
                title={intl.formatMessage({
                  id: 'pages.devReleases.confirm.archive',
                  defaultMessage: '废弃该版本？',
                })}
                onConfirm={() => doTransition(rel, 'archive')}
              >
                <Button size="small" type="text" danger>
                  <FormattedMessage id="pages.devReleases.action.archive" defaultMessage="废弃" />
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
        title={<FormattedMessage id="pages.devReleases.card.title" defaultMessage="版本发布" />}
        extra={
          <Space wrap>
            <Select
              placeholder={intl.formatMessage({
                id: 'pages.devReleases.filter.status',
                defaultMessage: '状态',
              })}
              value={status || undefined}
              onChange={(v) => {
                setStatus(v || '');
                // 筛选变化回第 1 页：params 变化与 setPageInfo 的双触发由
                // ProTable 内部 debounce + abort 合并，不会出现错序数据
                actionRef.current?.setPageInfo?.({ current: 1 });
              }}
              allowClear
              style={{ width: 110 }}
              options={Object.entries(releaseStatusLabels).map(([value, label]) => ({
                label,
                value,
              }))}
            />
            <Select
              placeholder={intl.formatMessage({
                id: 'pages.devReleases.filter.platform',
                defaultMessage: '平台',
              })}
              value={platform || undefined}
              onChange={(v) => {
                setPlatform(v || '');
                actionRef.current?.setPageInfo?.({ current: 1 });
              }}
              allowClear
              style={{ width: 110 }}
              options={Object.entries(releasePlatformLabels).map(([value, label]) => ({
                label,
                value,
              }))}
            />
            <Button icon={<ReloadOutlined />} onClick={reload} loading={tableLoading}>
              <FormattedMessage id="pages.devReleases.action.refresh" defaultMessage="刷新" />
            </Button>
            {canManage ? (
              <Button type="primary" icon={<PlusOutlined />} onClick={() => setCreateOpen(true)}>
                <FormattedMessage id="pages.devReleases.action.create" defaultMessage="创建版本" />
              </Button>
            ) : null}
          </Space>
        }
      >
        <ProTable<Release>
          actionRef={actionRef}
          rowKey="id"
          columns={columns}
          search={false}
          options={false}
          toolBarRender={false}
          params={{ status, platform }}
          request={async ({
            current = 1,
            pageSize = 20,
            status: statusFilter,
            platform: platformFilter,
          }) => {
            try {
              const res = await listReleases({
                status: statusFilter ?? '',
                platform: platformFilter ?? '',
                page: current,
                pageSize,
              });
              return { data: res.items || [], total: res.total || 0, success: true };
            } catch (error) {
              message.error(
                extractErrorMessage(
                  error,
                  intl.formatMessage({
                    id: 'pages.devReleases.error.loadFailed',
                    defaultMessage: '加载版本列表失败',
                  }),
                ),
              );
              return { data: [], total: 0, success: false };
            }
          }}
          onLoadingChange={(loading) => setTableLoading(loading === true)}
          pagination={{ pageSize: 20, showSizeChanger: true }}
        />
      </Card>

      <ModalForm<ReleaseFormValues>
        title={
          <FormattedMessage id="pages.devReleases.createForm.title" defaultMessage="创建版本" />
        }
        open={createOpen}
        onOpenChange={setCreateOpen}
        modalProps={{ destroyOnHidden: true }}
        width={520}
        layout="vertical"
        submitter={{
          searchConfig: {
            submitText: intl.formatMessage({
              id: 'pages.devReleases.createForm.submit',
              defaultMessage: '创建',
            }),
          },
        }}
        initialValues={{ type: 'full' }}
        onFinish={onFinish}
      >
        <Form.Item
          name="version"
          label={intl.formatMessage({
            id: 'pages.devReleases.field.version',
            defaultMessage: '版本号',
          })}
          rules={[
            {
              required: true,
              message: intl.formatMessage({
                id: 'pages.devReleases.field.versionRequired',
                defaultMessage: '如 1.5.0',
              }),
            },
          ]}
        >
          <Input placeholder="1.5.0" />
        </Form.Item>
        <Space>
          <Form.Item
            name="channel"
            label={intl.formatMessage({
              id: 'pages.devReleases.field.channel',
              defaultMessage: '渠道',
            })}
            initialValue="official"
          >
            <Input style={{ width: 140 }} />
          </Form.Item>
          <Form.Item
            name="platform"
            label={intl.formatMessage({
              id: 'pages.devReleases.field.platform',
              defaultMessage: '平台',
            })}
            rules={[{ required: true }]}
          >
            <Select
              style={{ width: 120 }}
              options={Object.entries(releasePlatformLabels).map(([value, label]) => ({
                label,
                value,
              }))}
            />
          </Form.Item>
          <Form.Item
            name="type"
            label={intl.formatMessage({
              id: 'pages.devReleases.field.type',
              defaultMessage: '类型',
            })}
          >
            <Select
              style={{ width: 100 }}
              options={Object.entries(releaseTypeLabels).map(([value, label]) => ({
                label,
                value,
              }))}
            />
          </Form.Item>
        </Space>
      </ModalForm>

      <Modal
        title={
          grayTarget
            ? intl.formatMessage(
                {
                  id: 'pages.devReleases.modal.rolloutTitle',
                  defaultMessage: `灰度放量：${grayTarget.version}（当前 ${grayTarget.grayPercent}%）`,
                },
                { version: grayTarget.version, percent: grayTarget.grayPercent },
              )
            : ''
        }
        open={Boolean(grayTarget)}
        onCancel={() => setGrayTarget(null)}
        footer={
          <Space>
            <Button onClick={() => setGrayTarget(null)}>
              <FormattedMessage id="pages.devReleases.modal.cancel" defaultMessage="取消" />
            </Button>
            <Button
              type="primary"
              onClick={async () => {
                if (!grayTarget) return;
                await doTransition(grayTarget, 'gray', grayValue);
                setGrayTarget(null);
              }}
            >
              <FormattedMessage
                id="pages.devReleases.modal.confirmRollout"
                defaultMessage="确认放量"
              />
            </Button>
          </Space>
        }
        destroyOnHidden
      >
        <Text>
          <FormattedMessage
            id="pages.devReleases.modal.rolloutHint"
            defaultMessage="放量只增不减；减少曝光请使用回滚。设备按 hash 分桶，同一设备结果稳定。"
          />
        </Text>
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
