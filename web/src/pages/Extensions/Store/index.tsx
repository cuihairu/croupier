import React, { useRef, useState } from 'react';
import {
  Alert,
  App,
  Button,
  Card,
  Dropdown,
  Form,
  Input,
  Select,
  Space,
  Tag,
  Typography,
  Upload,
} from 'antd';
import type { UploadProps } from 'antd';
import { DownOutlined, MoreOutlined, PlusOutlined, UploadOutlined } from '@ant-design/icons';
import {
  PageContainer,
  ProTable,
  type ActionType,
  type ProColumns,
} from '@ant-design/pro-components';
import { FormattedMessage, history, useAccess, useIntl } from '@umijs/max';
import {
  createExtensionCatalog,
  deleteExtensionCatalog,
  getExtensionCatalogDetail,
  importExtensionPack,
  installExtension,
  listExtensionCatalog,
  listExtensionCatalogReleases,
  publishExtensionRelease,
  updateExtensionCatalog,
  type ExtensionCatalogItem,
  type ExtensionReleaseItem,
} from '@/services/api/extensions';
import {
  adaptCatalogDetailResponse,
  adaptCatalogListResponse,
  adaptCatalogReleaseListResponse,
} from '@/services/adapters/extensions';
import { EXTENSION_ERROR_CODES } from '@/services/errors/codes';
import { mapExtensionError } from '@/services/errors/mapper';
import type { JSONValue } from '@/types/dashboard';
import CatalogDetailModal from './CatalogDetailModal';
import InstallModal from './InstallModal';
import {
  CatalogRegisterModal,
  ReleasePublishModal,
  type CatalogRegisterValues,
  type ReleasePublishValues,
} from './CatalogManageModals';
import { buildSchemaDefaults, normalizeConfigBySchema, type InstallFormValues } from './shared';

const { Text } = Typography;

export default function ExtensionsStorePage() {
  const access = useAccess();
  const intl = useIntl();
  const { message, modal } = App.useApp();
  const actionRef = useRef<ActionType | undefined>(undefined);

  const [keywordDraft, setKeywordDraft] = useState('');
  const [kindDraft, setKindDraft] = useState<string | undefined>(undefined);
  const [statusDraft, setStatusDraft] = useState<string | undefined>(undefined);
  const [keyword, setKeyword] = useState('');
  const [kind, setKind] = useState<string | undefined>(undefined);
  const [status, setStatus] = useState<string | undefined>(undefined);

  const [detailOpen, setDetailOpen] = useState(false);
  const [detailLoading, setDetailLoading] = useState(false);
  const [detailItem, setDetailItem] = useState<ExtensionCatalogItem | undefined>(undefined);
  const [detailReleases, setDetailReleases] = useState<ExtensionReleaseItem[]>([]);
  const [detailCapabilities, setDetailCapabilities] = useState<string[]>([]);

  const [installOpen, setInstallOpen] = useState(false);
  const [installing, setInstalling] = useState(false);
  const [installItem, setInstallItem] = useState<ExtensionCatalogItem | undefined>(undefined);
  const [installConfigSchema, setInstallConfigSchema] = useState<
    Record<string, JSONValue> | undefined
  >(undefined);
  const [installForm] = Form.useForm<InstallFormValues>();

  // 管理动作（#46 批次 3 写路径 UI 接线）：登记 / 上下架 / 发布版本 / 删除
  const [registerOpen, setRegisterOpen] = useState(false);
  const [registerSubmitting, setRegisterSubmitting] = useState(false);
  const [publishOpen, setPublishOpen] = useState(false);
  const [publishSubmitting, setPublishSubmitting] = useState(false);
  const [publishItem, setPublishItem] = useState<ExtensionCatalogItem | undefined>(undefined);
  // pack(.tgz) 导入（#46 批次 6）：上传即登记 + 发布版本
  const [importing, setImporting] = useState(false);

  const openDetail = async (item: ExtensionCatalogItem) => {
    setDetailOpen(true);
    setDetailLoading(true);
    setDetailItem(undefined);
    setDetailReleases([]);
    setDetailCapabilities([]);
    try {
      const resp = await getExtensionCatalogDetail(item.id);
      const vm = adaptCatalogDetailResponse(resp, item);
      setDetailItem(vm.item);
      setDetailReleases(vm.releases);
      setDetailCapabilities(vm.capabilities);
    } finally {
      setDetailLoading(false);
    }
  };

  const openInstall = async (item: ExtensionCatalogItem) => {
    setInstallItem(item);
    setInstallOpen(true);
    setInstallConfigSchema(undefined);
    setDetailLoading(true);
    try {
      const [detailResp, releaseResp] = await Promise.all([
        getExtensionCatalogDetail(item.id),
        listExtensionCatalogReleases(item.id),
      ]);
      const detailVM = adaptCatalogDetailResponse(detailResp, item);
      const releaseVM = adaptCatalogReleaseListResponse(releaseResp);
      const releases = releaseVM.releases;
      const latestVersion =
        detailVM.item?.latestVersion || releases[0]?.version || item.latestVersion || '';
      const schema =
        detailResp?.manifest &&
        typeof detailResp.manifest === 'object' &&
        detailResp.manifest.configSchema &&
        typeof detailResp.manifest.configSchema === 'object'
          ? (detailResp.manifest.configSchema as Record<string, JSONValue>)
          : undefined;
      setInstallConfigSchema(schema);
      installForm.setFieldsValue({
        releaseVersion: latestVersion,
        scopeType: 'system',
        scopeId: 'global',
        targetType: 'agent_group',
        targetId: 'default',
        config: buildSchemaDefaults(schema),
        configJson: '{}',
      });
      setDetailReleases(releases);
    } finally {
      setDetailLoading(false);
    }
  };

  const handleInstall = async () => {
    if (!installItem) return;
    const values = await installForm.validateFields();
    let config: Record<string, JSONValue> = normalizeConfigBySchema(
      values.config || {},
      installConfigSchema,
    );
    if (values.configJson && values.configJson.trim()) {
      try {
        config = { ...config, ...JSON.parse(values.configJson) };
      } catch {
        message.error(
          intl.formatMessage({
            id: 'pages.extensionsStore.install.configJsonInvalid',
            defaultMessage: '配置 JSON 格式不正确',
          }),
        );
        return;
      }
    }

    setInstalling(true);
    try {
      await installExtension({
        extensionId: installItem.id,
        releaseVersion: values.releaseVersion,
        scopeType: values.scopeType,
        scopeId: values.scopeId,
        targetType: values.targetType,
        targetId: values.targetId,
        config,
      });
      message.success(
        intl.formatMessage(
          {
            id: 'pages.extensionsStore.install.submitted',
            defaultMessage: `已提交安装：${installItem.displayName || installItem.name}`,
          },
          { name: installItem.displayName || installItem.name },
        ),
      );
      setInstallOpen(false);
      setInstallItem(undefined);
      actionRef.current?.reload();
    } catch (err) {
      const uiErr = mapExtensionError(err as Error);
      const details = uiErr.details || {};
      if (uiErr.code === EXTENSION_ERROR_CODES.EXTENSION_ALREADY_INSTALLED) {
        const existedID = details.installationId || '-';
        const scopeType = details.scopeType || '-';
        const scopeID = details.scopeId || '-';
        const targetType = details.targetType || '-';
        const targetID = details.targetId || '-';
        const releaseVersion = details.releaseVersion || '-';
        message.error(
          intl.formatMessage(
            {
              id: 'pages.extensionsStore.install.error.alreadyInstalled',
              defaultMessage: `该扩展已安装（实例 ${existedID}）。范围 ${scopeType}:${scopeID}，目标 ${targetType}:${targetID}，版本 ${releaseVersion}`,
            },
            {
              installationId: String(existedID),
              scopeType: String(scopeType),
              scopeId: String(scopeID),
              targetType: String(targetType),
              targetId: String(targetID),
              releaseVersion: String(releaseVersion),
            },
          ),
        );
        return;
      }
      if (uiErr.code === EXTENSION_ERROR_CODES.MISSING_DEPENDENCY) {
        message.error(
          intl.formatMessage(
            {
              id: 'pages.extensionsStore.install.error.missingDependency',
              defaultMessage: `缺少依赖扩展：${details.dependency || 'unknown'}`,
            },
            { dependency: String(details.dependency || 'unknown') },
          ),
        );
        return;
      }
      if (uiErr.code === EXTENSION_ERROR_CODES.VERSION_MISMATCH) {
        message.error(
          intl.formatMessage(
            {
              id: 'pages.extensionsStore.install.error.versionMismatch',
              defaultMessage: `依赖版本不匹配：${details.dependency || 'unknown'}，要求 ${
                details.requiredVersion || '-'
              }，当前 ${details.currentVersion || '-'}`,
            },
            {
              dependency: String(details.dependency || 'unknown'),
              requiredVersion: String(details.requiredVersion || '-'),
              currentVersion: String(details.currentVersion || '-'),
            },
          ),
        );
        return;
      }
      if (uiErr.code === EXTENSION_ERROR_CODES.DEPENDENCY_CYCLE) {
        message.error(
          intl.formatMessage(
            {
              id: 'pages.extensionsStore.install.error.dependencyCycle',
              defaultMessage: `检测到循环依赖：${details.dependency || 'unknown'}`,
            },
            { dependency: String(details.dependency || 'unknown') },
          ),
        );
        return;
      }
      message.error(uiErr.message);
    } finally {
      setInstalling(false);
    }
  };

  /** HTTP 状态读取（409 冲突分支按状态码分支，body message 兜底透出） */
  const statusOf = (err: unknown): number =>
    (err as { response?: { status?: number } })?.response?.status ?? 0;

  // pack(.tgz) 导入（#46 批次 6）：上传即服务端登记 + 发布版本。
  const importUploadProps: UploadProps = {
    accept: '.tgz',
    showUploadList: false,
    disabled: importing,
    customRequest: async (options) => {
      const { file, onSuccess, onError } = options;
      setImporting(true);
      try {
        const resp = await importExtensionPack(file as File);
        onSuccess?.(resp, new XMLHttpRequest());
        message.success(
          intl.formatMessage(
            {
              id: 'pages.extensionsStore.manage.importOk',
              defaultMessage: '已导入并发布版本 {version}',
            },
            { version: resp.release.version },
          ),
        );
        actionRef.current?.reload();
      } catch (err) {
        onError?.(err as Error);
        if (statusOf(err) === 409) {
          message.error(
            intl.formatMessage({
              id: 'pages.extensionsStore.manage.importConflict',
              defaultMessage: '导入失败：该版本已存在',
            }),
          );
          return;
        }
        message.error(mapExtensionError(err as Error).message);
      } finally {
        setImporting(false);
      }
    },
  };

  const handleRegister = async (values: CatalogRegisterValues) => {
    setRegisterSubmitting(true);
    try {
      await createExtensionCatalog({
        extensionId: values.extensionId.trim(),
        name: values.name?.trim(),
        displayName: values.displayName?.trim(),
        vendor: values.vendor?.trim(),
        kind: values.kind,
        summary: values.summary?.trim(),
        iconUrl: values.iconUrl?.trim(),
        homepageUrl: values.homepageUrl?.trim(),
        status: values.status,
      });
      message.success(
        intl.formatMessage({
          id: 'pages.extensionsStore.manage.registerOk',
          defaultMessage: '已登记到目录',
        }),
      );
      setRegisterOpen(false);
      actionRef.current?.reload();
    } catch (err) {
      if (statusOf(err) === 409) {
        message.error(
          intl.formatMessage({
            id: 'pages.extensionsStore.manage.registerConflict',
            defaultMessage: '登记失败：该扩展 ID 已存在于目录',
          }),
        );
        return;
      }
      message.error(mapExtensionError(err as Error).message);
    } finally {
      setRegisterSubmitting(false);
    }
  };

  const toggleStatus = async (item: ExtensionCatalogItem) => {
    const target = item.status === 'active' ? 'delisted' : 'active';
    try {
      await updateExtensionCatalog(item.id, { status: target });
      message.success(
        intl.formatMessage(
          {
            id: 'pages.extensionsStore.manage.statusChanged',
            defaultMessage: '已{action}：{name}',
          },
          {
            action:
              target === 'active'
                ? intl.formatMessage({
                    id: 'pages.extensionsStore.manage.activate',
                    defaultMessage: '上架',
                  })
                : intl.formatMessage({
                    id: 'pages.extensionsStore.manage.delist',
                    defaultMessage: '下架',
                  }),
            name: item.displayName || item.name,
          },
        ),
      );
      actionRef.current?.reload();
    } catch (err) {
      message.error(mapExtensionError(err as Error).message);
    }
  };

  const confirmDelete = (item: ExtensionCatalogItem) => {
    modal.confirm({
      title: intl.formatMessage({
        id: 'pages.extensionsStore.manage.deleteConfirmTitle',
        defaultMessage: '删除目录条目',
      }),
      content: intl.formatMessage(
        {
          id: 'pages.extensionsStore.manage.deleteConfirmContent',
          defaultMessage: '确定删除 {name}？存在活跃安装实例时删除会被拒绝（须先卸载）。',
        },
        { name: item.displayName || item.name },
      ),
      okButtonProps: { danger: true },
      onOk: async () => {
        try {
          await deleteExtensionCatalog(item.id);
          message.success(
            intl.formatMessage({
              id: 'pages.extensionsStore.manage.deleteOk',
              defaultMessage: '已删除（含历史版本）',
            }),
          );
          actionRef.current?.reload();
        } catch (err) {
          if (statusOf(err) === 409) {
            message.error(
              intl.formatMessage({
                id: 'pages.extensionsStore.manage.deleteConflict',
                defaultMessage: '删除失败：存在活跃安装实例，请先卸载',
              }),
            );
            return;
          }
          message.error(mapExtensionError(err as Error).message);
        }
      },
    });
  };

  const openPublish = (item: ExtensionCatalogItem) => {
    setPublishItem(item);
    setPublishOpen(true);
  };

  const handlePublish = async (values: ReleasePublishValues) => {
    if (!publishItem) return;
    let manifest: Record<string, JSONValue>;
    try {
      manifest = JSON.parse(values.manifestJson) as Record<string, JSONValue>;
    } catch {
      // 表单 validator 已拦截，此处兜底
      message.error(
        intl.formatMessage({
          id: 'pages.extensionsStore.manage.manifestJsonInvalid',
          defaultMessage: 'Manifest JSON 格式不正确',
        }),
      );
      return;
    }
    setPublishSubmitting(true);
    try {
      await publishExtensionRelease(publishItem.id, {
        version: values.version.trim(),
        releaseChannel: values.releaseChannel,
        minCoreVersion: values.minCoreVersion?.trim(),
        packageRef: values.packageRef?.trim(),
        checksum: values.checksum?.trim(),
        changelog: values.changelog?.trim(),
        manifest,
      });
      message.success(
        intl.formatMessage({
          id: 'pages.extensionsStore.manage.publishOk',
          defaultMessage: '版本已发布',
        }),
      );
      setPublishOpen(false);
      setPublishItem(undefined);
      actionRef.current?.reload();
    } catch (err) {
      if (statusOf(err) === 409) {
        message.error(
          intl.formatMessage({
            id: 'pages.extensionsStore.manage.publishConflict',
            defaultMessage: '发布失败：该版本已存在',
          }),
        );
        return;
      }
      message.error(mapExtensionError(err as Error).message);
    } finally {
      setPublishSubmitting(false);
    }
  };

  const columns: ProColumns<ExtensionCatalogItem>[] = [
    {
      title: intl.formatMessage({
        id: 'pages.extensionsStore.column.extension',
        defaultMessage: '扩展',
      }),
      dataIndex: 'displayName',
      key: 'displayName',
      render: (_, row) => (
        <Space orientation="vertical" size={0}>
          <Text strong>{row.displayName || row.name}</Text>
          <Text type="secondary">{row.id}</Text>
        </Space>
      ),
    },
    {
      title: intl.formatMessage({
        id: 'pages.extensionsStore.column.kind',
        defaultMessage: '类型',
      }),
      dataIndex: 'kind',
      key: 'kind',
      width: 120,
      render: (_, row) => <Tag>{row.kind || '-'}</Tag>,
    },
    {
      title: intl.formatMessage({
        id: 'pages.extensionsStore.column.version',
        defaultMessage: '版本',
      }),
      dataIndex: 'latestVersion',
      key: 'latestVersion',
      width: 130,
      render: (_, row) => row.latestVersion || '-',
    },
    {
      title: intl.formatMessage({
        id: 'pages.extensionsStore.column.status',
        defaultMessage: '状态',
      }),
      dataIndex: 'status',
      key: 'status',
      width: 120,
      render: (_, row) => (
        <Tag color={row.status === 'active' ? 'green' : 'default'}>{row.status}</Tag>
      ),
    },
    {
      title: intl.formatMessage({
        id: 'pages.extensionsStore.column.tags',
        defaultMessage: '标签',
      }),
      dataIndex: 'tags',
      key: 'tags',
      width: 220,
      render: (_, row) => (
        <Space wrap>
          {row.defaultInstall && (
            <Tag color="gold">
              <FormattedMessage
                id="pages.extensionsStore.column.tagDefaultInstall"
                defaultMessage="默认安装"
              />
            </Tag>
          )}
          {(row.tags || []).slice(0, 3).map((tag) => (
            <Tag key={tag}>{tag}</Tag>
          ))}
          {(row.tags || []).length === 0 && !row.defaultInstall && <Text type="secondary">-</Text>}
        </Space>
      ),
    },
    {
      title: intl.formatMessage({
        id: 'pages.extensionsStore.column.installed',
        defaultMessage: '已安装',
      }),
      dataIndex: 'installed',
      key: 'installed',
      width: 100,
      render: (_, row) =>
        row.installed ? (
          <Tag color="blue">
            <FormattedMessage id="pages.extensionsStore.column.installedYes" defaultMessage="是" />
          </Tag>
        ) : (
          <Tag>
            <FormattedMessage id="pages.extensionsStore.column.installedNo" defaultMessage="否" />
          </Tag>
        ),
    },
    {
      title: intl.formatMessage({
        id: 'pages.extensionsStore.column.actions',
        defaultMessage: '操作',
      }),
      key: 'actions',
      width: 280,
      render: (_, row) => (
        <Space>
          <Button size="small" onClick={() => openDetail(row)}>
            <FormattedMessage id="pages.extensionsStore.action.detail" defaultMessage="详情" />
          </Button>
          <Button
            size="small"
            type="primary"
            disabled={!access.canExtensionsManage || row.installed}
            onClick={() => openInstall(row)}
          >
            {row.installed ? (
              <FormattedMessage
                id="pages.extensionsStore.action.alreadyInstalled"
                defaultMessage="已安装"
              />
            ) : (
              <FormattedMessage id="pages.extensionsStore.action.install" defaultMessage="安装" />
            )}
          </Button>
          {access.canExtensionsManage && (
            <Dropdown
              trigger={['click']}
              menu={{
                items: [
                  row.status === 'active'
                    ? {
                        key: 'delist',
                        label: (
                          <FormattedMessage
                            id="pages.extensionsStore.manage.delist"
                            defaultMessage="下架"
                          />
                        ),
                      }
                    : {
                        key: 'activate',
                        label: (
                          <FormattedMessage
                            id="pages.extensionsStore.manage.activate"
                            defaultMessage="上架"
                          />
                        ),
                      },
                  {
                    key: 'publish',
                    label: (
                      <FormattedMessage
                        id="pages.extensionsStore.manage.publish"
                        defaultMessage="发布版本"
                      />
                    ),
                  },
                  { type: 'divider' as const },
                  {
                    key: 'delete',
                    label: (
                      <FormattedMessage
                        id="pages.extensionsStore.manage.delete"
                        defaultMessage="删除"
                      />
                    ),
                    danger: true,
                  },
                ],
                onClick: ({ key }) => {
                  if (key === 'activate' || key === 'delist') void toggleStatus(row);
                  if (key === 'publish') openPublish(row);
                  if (key === 'delete') confirmDelete(row);
                },
              }}
            >
              <Button size="small" icon={<MoreOutlined />} aria-label="more-actions" />
            </Dropdown>
          )}
        </Space>
      ),
    },
  ];

  return (
    <PageContainer
      title={intl.formatMessage({
        id: 'pages.extensionsStore.page.title',
        defaultMessage: '扩展商店',
      })}
      subTitle={intl.formatMessage({
        id: 'pages.extensionsStore.page.subTitle',
        defaultMessage: '浏览和安装可用扩展',
      })}
    >
      <Alert
        type="info"
        showIcon
        style={{ marginBottom: 16 }}
        title={intl.formatMessage({
          id: 'pages.extensionsStore.alert.positioning.message',
          defaultMessage: '商店只负责发现与安装扩展物料',
        })}
        description={intl.formatMessage({
          id: 'pages.extensionsStore.alert.positioning.description',
          defaultMessage:
            '安装后扩展的能力注册与页面模板即进入本租户可用范围；生效状态排查与卸载在安装列表中完成，页面编排仍在 Page Studio。',
        })}
        action={
          <Button onClick={() => history.push('/system/extensions/installations')}>
            <FormattedMessage
              id="pages.extensionsStore.button.viewInstallations"
              defaultMessage="查看安装列表"
            />
          </Button>
        }
      />
      <Card>
        <Space style={{ marginBottom: 16 }} wrap>
          <Input
            style={{ width: 220 }}
            placeholder={intl.formatMessage({
              id: 'pages.extensionsStore.filter.keywordPlaceholder',
              defaultMessage: '关键字',
            })}
            allowClear
            value={keywordDraft}
            onChange={(e) => setKeywordDraft(e.target.value)}
          />
          <Select
            style={{ width: 150 }}
            allowClear
            placeholder={intl.formatMessage({
              id: 'pages.extensionsStore.filter.kindPlaceholder',
              defaultMessage: '类型',
            })}
            value={kindDraft}
            onChange={setKindDraft}
            options={[
              { label: 'ui', value: 'ui' },
              { label: 'integration', value: 'integration' },
              { label: 'analytics', value: 'analytics' },
              { label: 'ops', value: 'ops' },
            ]}
          />
          <Select
            style={{ width: 150 }}
            allowClear
            placeholder={intl.formatMessage({
              id: 'pages.extensionsStore.filter.statusPlaceholder',
              defaultMessage: '状态',
            })}
            value={statusDraft}
            onChange={setStatusDraft}
            options={[
              { label: 'active', value: 'active' },
              { label: 'inactive', value: 'inactive' },
            ]}
          />
          <Button
            type="primary"
            onClick={() => {
              // 提交筛选草稿并回第 1 页；params 变化与 setPageInfo 的双触发
              // 由 ProTable 内部 debounce + abort 合并；reload 兜底「筛选值
              // 未变时点击查询也重查」的原语义（params 不变不会触发请求）
              actionRef.current?.setPageInfo?.({ current: 1 });
              setKeyword(keywordDraft.trim());
              setKind(kindDraft);
              setStatus(statusDraft);
              actionRef.current?.reload();
            }}
          >
            <FormattedMessage id="pages.extensionsStore.filter.search" defaultMessage="查询" />
          </Button>
          <Button
            onClick={() => {
              setKeywordDraft('');
              setKindDraft(undefined);
              setStatusDraft(undefined);
              actionRef.current?.setPageInfo?.({ current: 1 });
              setKeyword('');
              setKind(undefined);
              setStatus(undefined);
            }}
          >
            <FormattedMessage id="pages.extensionsStore.filter.reset" defaultMessage="重置" />
          </Button>
          {access.canExtensionsManage && (
            <>
              <Button
                type="primary"
                ghost
                icon={<PlusOutlined />}
                onClick={() => setRegisterOpen(true)}
              >
                <FormattedMessage
                  id="pages.extensionsStore.manage.register"
                  defaultMessage="登记扩展"
                />
              </Button>
              <Upload {...importUploadProps}>
                <Button icon={<UploadOutlined />} loading={importing}>
                  <FormattedMessage
                    id="pages.extensionsStore.manage.import"
                    defaultMessage="导入扩展包"
                  />
                </Button>
              </Upload>
            </>
          )}
        </Space>

        <ProTable<ExtensionCatalogItem>
          actionRef={actionRef}
          scroll={{ x: 1000 }}
          rowKey="id"
          columns={columns}
          search={false}
          options={false}
          toolBarRender={false}
          params={{ keyword, kind, status }}
          request={async ({ current = 1, pageSize = 10, keyword: kw, kind: kd, status: st }) => {
            try {
              const resp = await listExtensionCatalog({
                keyword: kw ?? '',
                kind: kd,
                status: st,
                page: current,
                pageSize,
              });
              const vm = adaptCatalogListResponse(resp);
              return { data: vm.items, total: vm.total, success: true };
            } catch {
              // 原实现不本地弹错（全局请求拦截器已 toast），保持该语义
              return { data: [], total: 0, success: false };
            }
          }}
          pagination={{ pageSize: 10, showSizeChanger: true }}
        />
      </Card>

      <CatalogDetailModal
        open={detailOpen}
        loading={detailLoading}
        item={detailItem}
        capabilities={detailCapabilities}
        releases={detailReleases}
        onClose={() => setDetailOpen(false)}
      />

      <CatalogRegisterModal
        open={registerOpen}
        confirmLoading={registerSubmitting}
        onClose={() => setRegisterOpen(false)}
        onSubmit={(values) => void handleRegister(values)}
      />

      <ReleasePublishModal
        item={publishItem}
        open={publishOpen}
        confirmLoading={publishSubmitting}
        onClose={() => {
          setPublishOpen(false);
          setPublishItem(undefined);
        }}
        onSubmit={(values) => void handlePublish(values)}
      />

      <InstallModal
        open={installOpen}
        form={installForm}
        item={installItem}
        releases={detailReleases}
        configSchema={installConfigSchema}
        installing={installing}
        onCancel={() => setInstallOpen(false)}
        onOk={handleInstall}
      />
    </PageContainer>
  );
}
