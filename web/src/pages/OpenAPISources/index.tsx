import React, { useEffect, useMemo, useState } from 'react';
import { PageContainer, ProTable, type ProColumns } from '@ant-design/pro-components';
import { Alert, App, Button, Card, Space, Tag, Tooltip, Typography } from 'antd';
import type { UploadFile } from 'antd/es/upload/interface';
import { CloudUploadOutlined, EditOutlined, ReloadOutlined } from '@ant-design/icons';
import { FormattedMessage, history, useAccess, useIntl } from '@umijs/max';
import {
  bindOpenAPISourceProvider,
  createOpenAPISource,
  deleteOpenAPISourceBinding,
  getOpenAPISource,
  listOpenAPISources,
  listRuntimeSources,
  updateOpenAPISource,
  uploadOpenAPISourceFile,
  type OpenAPISourceBinding,
  type OpenAPISourceDetail,
  type OpenAPISourceOperation,
  type OpenAPISourcePipelineSummary,
  type OpenAPISourceSummary,
  type RuntimeProviderItem,
} from '@/services/api/openapi';
import { listDescriptors, type FunctionDescriptor } from '@/services/api/functions';
import { isScopeReady, subscribeScope } from '@/stores/scope';
import type { Diagnostic } from '@/types/dashboard';
import SourceDetailDrawer from './SourceDetailDrawer';
import SourceModal from './SourceModal';
import BindingModal from './BindingModal';
import PipelineSummaryModal from './PipelineSummaryModal';
import {
  diagnosticsFromError,
  errorMessage,
  formatDate,
  functionLabel,
  parseOpenAPIDocument,
  proposalInboxPath,
  type SourceModalMode,
} from './shared';

export default function OpenAPISourcesPage() {
  const { message, modal } = App.useApp();
  const intl = useIntl();
  const access = useAccess() as {
    canOpenAPISourcesWrite?: boolean;
  };
  const canWrite = !!access.canOpenAPISourcesWrite;
  const [loading, setLoading] = useState(false);
  const [sources, setSources] = useState<OpenAPISourceSummary[]>([]);
  const [runtimeSources, setRuntimeSources] = useState<RuntimeProviderItem[]>([]);
  const [detail, setDetail] = useState<OpenAPISourceDetail | null>(null);
  const [detailLoading, setDetailLoading] = useState(false);
  const [functions, setFunctions] = useState<FunctionDescriptor[]>([]);
  const [uploadName, setUploadName] = useState('');
  const [rawSpec, setRawSpec] = useState('');
  const [uploadFile, setUploadFile] = useState<UploadFile | null>(null);
  const [sourceModalOpen, setSourceModalOpen] = useState(false);
  const [sourceModalMode, setSourceModalMode] = useState<SourceModalMode>('create');
  const [editingSource, setEditingSource] = useState<OpenAPISourceDetail | null>(null);
  const [bindOpen, setBindOpen] = useState(false);
  const [bindingOperation, setBindingOperation] = useState<OpenAPISourceOperation | null>(null);
  const [bindingFunctionId, setBindingFunctionId] = useState<string>();
  const [bindingProviderId, setBindingProviderId] = useState('');
  const [bindingId, setBindingId] = useState('');
  const [sourceDiagnostics, setSourceDiagnostics] = useState<Diagnostic[]>([]);
  // T7/D4：上传即成页摘要——create/update 响应携带时弹摘要 Modal（计数 + CTA）。
  const [pipelineSummary, setPipelineSummary] = useState<OpenAPISourcePipelineSummary | null>(null);
  const [scopeKey, setScopeKey] = useState('');
  const isUpdatingSource = sourceModalMode === 'update';

  // Subscribe to scope changes so we reload when game/env changes.
  useEffect(() => {
    const off = subscribeScope((scope) => {
      setScopeKey(`${scope.gameId || ''}:${scope.env || ''}`);
    });
    return off;
  }, []);

  const loadSources = async () => {
    setLoading(true);
    try {
      const response = await listOpenAPISources();
      setSources(response.items || []);
    } catch (err) {
      message.error(
        errorMessage(
          err,
          intl.formatMessage({
            id: 'pages.openAPISources.error.loadFailed',
            defaultMessage: '加载 OpenAPI 来源失败',
          }),
        ),
      );
      setSources([]);
    } finally {
      setLoading(false);
    }
  };

  // 运行时导入与 Source 同 scope；失败不阻塞页面（区块显示为空即可）。
  const loadRuntimeSources = async () => {
    try {
      const response = await listRuntimeSources();
      setRuntimeSources(response.items || []);
    } catch {
      setRuntimeSources([]);
    }
  };

  const loadFunctions = async () => {
    try {
      setFunctions(await listDescriptors());
    } catch {
      setFunctions([]);
    }
  };

  const openDetail = async (sourceId: string) => {
    setDetailLoading(true);
    setSourceDiagnostics([]);
    try {
      const response = await getOpenAPISource(sourceId);
      setDetail(response.source);
      setSourceDiagnostics(response.source.diagnostics || []);
    } finally {
      setDetailLoading(false);
    }
  };

  useEffect(() => {
    // Skip initial request until GameSelector has validated the scope.
    if (!isScopeReady()) return;
    loadSources();
    loadRuntimeSources();
    loadFunctions();
  }, [scopeKey]);

  // 绑定候选 = SDK 描述符函数 + 运行时 provider 导入函数（标注来源 agent）。
  // 运行时函数与描述符重复时以描述符为准（它们已带完整契约）。
  const functionOptions = useMemo(() => {
    const descriptorIds = new Set(functions.map((fn) => fn.id));
    const runtimeOptions = runtimeSources.flatMap((provider) =>
      provider.functions
        .filter((fn) => !descriptorIds.has(fn))
        .map((fn) => ({
          label: `${fn}（${intl.formatMessage(
            { id: 'pages.openapiSources.bindingModal.function.runtimeAgent' },
            { agent: provider.agentId },
          )}）`,
          value: fn,
        })),
    );
    return [
      ...functions.map((fn) => ({ label: functionLabel(fn), value: fn.id })),
      ...runtimeOptions,
    ];
  }, [functions, runtimeSources, intl]);

  const resetSourceForm = () => {
    setUploadName('');
    setRawSpec('');
    setUploadFile(null);
    setEditingSource(null);
    setSourceModalMode('create');
  };

  const openCreateSourceModal = () => {
    resetSourceForm();
    setSourceDiagnostics([]);
    setSourceModalOpen(true);
  };

  const openUpdateSourceModal = async (record: OpenAPISourceSummary | OpenAPISourceDetail) => {
    if (!canWrite) {
      message.error(
        intl.formatMessage({
          id: 'pages.openapiSources.error.noWritePermission',
          defaultMessage: '没有 OpenAPI Source 写权限',
        }),
      );
      return;
    }
    setSourceDiagnostics([]);
    try {
      const response =
        detail?.sourceId === record.sourceId
          ? { source: detail }
          : await getOpenAPISource(record.sourceId);
      setEditingSource(response.source);
      setSourceModalMode('update');
      setUploadName(response.source.name);
      setRawSpec(JSON.stringify(response.source.spec || {}, null, 2));
      setUploadFile(null);
      setSourceModalOpen(true);
    } catch (error) {
      message.error(
        errorMessage(
          error,
          intl.formatMessage({
            id: 'pages.openapiSources.error.loadSourceFailed',
            defaultMessage: '加载 OpenAPI Source 失败',
          }),
        ),
      );
    }
  };

  const closeSourceModal = () => {
    setSourceModalOpen(false);
    resetSourceForm();
  };

  const submitSource = async () => {
    if (!canWrite) {
      message.error(
        intl.formatMessage({
          id: 'pages.openapiSources.error.noWritePermission',
          defaultMessage: '没有 OpenAPI Source 写权限',
        }),
      );
      return;
    }
    setSourceDiagnostics([]);
    try {
      let response;
      if (isUpdatingSource) {
        if (!editingSource) {
          message.error(
            intl.formatMessage({
              id: 'pages.openapiSources.message.missingUpdateTarget',
              defaultMessage: '缺少要更新的 OpenAPI Source',
            }),
          );
          return;
        }
        const text = rawSpec.trim();
        if (!text) {
          message.warning(
            intl.formatMessage({
              id: 'pages.openapiSources.message.pasteJson',
              defaultMessage: '请粘贴新的 OpenAPI JSON',
            }),
          );
          return;
        }
        response = await updateOpenAPISource(
          editingSource.sourceId,
          parseOpenAPIDocument(text),
          uploadName || undefined,
        );
      } else if (uploadFile?.originFileObj) {
        response = await uploadOpenAPISourceFile(
          uploadFile.originFileObj,
          uploadName || uploadFile.name,
        );
      } else {
        const text = rawSpec.trim();
        if (!text) {
          message.warning(
            intl.formatMessage({
              id: 'pages.openapiSources.message.uploadOrPaste',
              defaultMessage: '请上传文件或粘贴 OpenAPI JSON',
            }),
          );
          return;
        }
        response = await createOpenAPISource(parseOpenAPIDocument(text), uploadName || undefined);
      }
      if (response.summary) {
        // T7/D4：上传即成页——摘要 Modal 展示单请求生成的契约/组件/提案
        // 计数与诊断，CTA 直达编辑器/提案收件箱（与 binding 保存后的
        // proposal CTA 同一交互模式）；Modal 已表达成功，不再叠加 toast。
        setPipelineSummary(response.summary);
      } else {
        message.success(
          isUpdatingSource
            ? intl.formatMessage({
                id: 'pages.openapiSources.message.sourceUpdated',
                defaultMessage: 'OpenAPI Source 已更新',
              })
            : intl.formatMessage({
                id: 'pages.openapiSources.message.sourceCreated',
                defaultMessage: 'OpenAPI Source 已创建',
              }),
        );
      }
      setSourceModalOpen(false);
      resetSourceForm();
      await loadSources();
      await openDetail(response.source.sourceId);
    } catch (error) {
      const diagnostics = diagnosticsFromError(error);
      if (diagnostics.length > 0) {
        setSourceDiagnostics(diagnostics);
        message.error(
          intl.formatMessage({
            id: 'pages.openapiSources.message.validationFailed',
            defaultMessage: 'OpenAPI Source 校验失败，请查看诊断',
          }),
        );
        return;
      }
      message.error(
        errorMessage(
          error,
          intl.formatMessage({
            id: 'pages.openapiSources.error.createSourceFailed',
            defaultMessage: '创建 OpenAPI Source 失败',
          }),
        ),
      );
    }
  };

  const openBindingModal = (operation: OpenAPISourceOperation) => {
    if (!canWrite) {
      message.error(
        intl.formatMessage({
          id: 'pages.openapiSources.error.noWritePermission',
          defaultMessage: '没有 OpenAPI Source 写权限',
        }),
      );
      return;
    }
    setBindingOperation(operation);
    setBindingFunctionId(operation.functionId);
    setBindingProviderId('');
    setBindingId(operation.bindingId || operation.operationId);
    setBindOpen(true);
  };

  const submitBinding = async () => {
    if (!canWrite) {
      message.error(
        intl.formatMessage({
          id: 'pages.openapiSources.error.noWritePermission',
          defaultMessage: '没有 OpenAPI Source 写权限',
        }),
      );
      return;
    }
    if (!detail || !bindingOperation || !bindingFunctionId) {
      message.warning(
        intl.formatMessage({
          id: 'pages.openapiSources.message.selectFunction',
          defaultMessage: '请选择要绑定的函数',
        }),
      );
      return;
    }
    try {
      const result = await bindOpenAPISourceProvider(detail.sourceId, {
        operationId: bindingOperation.operationId,
        functionId: bindingFunctionId,
        providerId: bindingProviderId.trim() || undefined,
        bindingId: bindingId.trim() || undefined,
      });
      setBindOpen(false);
      await openDetail(detail.sourceId);
      await loadSources();
      if (result.proposal) {
        modal.success({
          title: intl.formatMessage({
            id: 'pages.openapiSources.binding.savedModal.title',
            defaultMessage: 'Provider binding 已保存',
          }),
          content: intl.formatMessage(
            {
              id: 'pages.openapiSources.binding.savedModal.content',
              defaultMessage:
                '已生成默认页面 Proposal：{proposalKey}。请进入 Proposal 队列预览并发布，发布后才会出现在运行控制台菜单。',
            },
            { proposalKey: result.proposal.proposalKey },
          ),
          okText: intl.formatMessage({
            id: 'pages.openapiSources.binding.savedModal.okText',
            defaultMessage: '打开 Proposal',
          }),
          onOk: () =>
            history.push(
              proposalInboxPath(result.proposal!.proposalKey, result.proposal!.resourceKey),
            ),
        });
      } else {
        message.warning(
          intl.formatMessage({
            id: 'pages.openapiSources.binding.savedWithoutProposal',
            defaultMessage:
              'Provider binding 已保存，但未返回可发布 Proposal。请在 Proposal 队列查看诊断。',
          }),
        );
      }
    } catch (error) {
      message.error(
        errorMessage(
          error,
          intl.formatMessage({
            id: 'pages.openapiSources.binding.saveFailed',
            defaultMessage: '保存 binding 失败',
          }),
        ),
      );
    }
  };

  const removeBinding = async (binding: OpenAPISourceBinding) => {
    if (!canWrite) {
      message.error(
        intl.formatMessage({
          id: 'pages.openapiSources.error.noWritePermission',
          defaultMessage: '没有 OpenAPI Source 写权限',
        }),
      );
      return;
    }
    if (!detail) return;
    await deleteOpenAPISourceBinding(detail.sourceId, binding.bindingId);
    message.success(
      intl.formatMessage({
        id: 'pages.openapiSources.binding.deleted',
        defaultMessage: 'binding 已删除',
      }),
    );
    await openDetail(detail.sourceId);
    await loadSources();
  };

  const sourceColumns: ProColumns<OpenAPISourceSummary>[] = [
    {
      title: 'Source',
      dataIndex: 'name',
      render: (_, record) => (
        <Space orientation="vertical" size={0}>
          <Typography.Text strong>{record.name}</Typography.Text>
          <Typography.Text code>{record.sourceId}</Typography.Text>
        </Space>
      ),
    },
    {
      title: intl.formatMessage({
        id: 'pages.openapiSources.column.version',
        defaultMessage: '版本',
      }),
      dataIndex: 'revision',
      width: 100,
      render: (_, record) => <Tag>{`rev ${record.revision}`}</Tag>,
    },
    {
      title: 'OpenAPI',
      dataIndex: 'openapiVersion',
      width: 140,
      render: (_, record) => <Tag>{record.openapiVersion || '-'}</Tag>,
    },
    {
      title: intl.formatMessage({
        id: 'pages.openapiSources.column.operationCount',
        defaultMessage: '操作数',
      }),
      dataIndex: 'operationCount',
      width: 100,
    },
    {
      title: intl.formatMessage({
        id: 'pages.openapiSources.column.diagnosticCount',
        defaultMessage: '诊断',
      }),
      dataIndex: 'diagnosticCount',
      width: 100,
      render: (_, record) => (
        <Tag color={record.diagnosticCount > 0 ? 'orange' : 'green'}>{record.diagnosticCount}</Tag>
      ),
    },
    {
      title: intl.formatMessage({
        id: 'pages.openapiSources.column.updatedAt',
        defaultMessage: '更新时间',
      }),
      dataIndex: 'updatedAt',
      width: 180,
      render: (_, record) => formatDate(record.updatedAt),
    },
    {
      title: intl.formatMessage({
        id: 'pages.openapiSources.column.actions',
        defaultMessage: '操作',
      }),
      valueType: 'option',
      width: 170,
      render: (_, record) => {
        const actions = [
          <Button key="open" type="link" size="small" onClick={() => openDetail(record.sourceId)}>
            <FormattedMessage id="pages.openapiSources.button.open" defaultMessage="打开" />
          </Button>,
        ];
        if (canWrite) {
          actions.push(
            <Button
              key="update"
              type="link"
              size="small"
              icon={<EditOutlined />}
              onClick={() => openUpdateSourceModal(record)}
            >
              <FormattedMessage id="pages.openapiSources.button.update" defaultMessage="更新" />
            </Button>,
          );
        }
        return actions;
      },
    },
  ];

  const runtimeColumns: ProColumns<RuntimeProviderItem>[] = [
    {
      title: intl.formatMessage({
        id: 'pages.openapiSources.runtime.column.provider',
        defaultMessage: 'Provider',
      }),
      dataIndex: 'name',
      render: (_, record) => (
        <Space orientation="vertical" size={0}>
          <Typography.Text strong>{record.name}</Typography.Text>
          <Typography.Text code>{record.providerId}</Typography.Text>
        </Space>
      ),
    },
    {
      title: intl.formatMessage({
        id: 'pages.openapiSources.runtime.column.agent',
        defaultMessage: '来源 Agent',
      }),
      dataIndex: 'agentId',
      width: 200,
      render: (_, record) => (
        <Typography.Text copyable={{ text: record.agentId }}>{record.agentId}</Typography.Text>
      ),
    },
    {
      title: intl.formatMessage({
        id: 'pages.openapiSources.runtime.column.functionCount',
        defaultMessage: '函数数',
      }),
      dataIndex: 'functionCount',
      width: 110,
      render: (_, record) => (
        <Tooltip
          title={
            record.functions.length > 0
              ? record.functions.join('\n')
              : intl.formatMessage({
                  id: 'pages.openapiSources.runtime.emptyFunctions',
                  defaultMessage: '无函数',
                })
          }
        >
          <Tag>{record.functionCount}</Tag>
        </Tooltip>
      ),
    },
    {
      title: intl.formatMessage({
        id: 'pages.openapiSources.runtime.column.version',
        defaultMessage: '版本',
      }),
      dataIndex: 'version',
      width: 160,
      render: (_, record) => <Tag>{record.version || '-'}</Tag>,
    },
    {
      title: intl.formatMessage({
        id: 'pages.openapiSources.runtime.column.lastSeen',
        defaultMessage: '最近心跳',
      }),
      dataIndex: 'lastSeenUnix',
      width: 160,
      render: (_, record) =>
        record.lastSeenUnix > 0
          ? formatDate(new Date(record.lastSeenUnix * 1000).toISOString())
          : '-',
    },
  ];

  const pageActions = [
    <Button
      key="reload"
      icon={<ReloadOutlined />}
      onClick={() => {
        loadSources();
        loadRuntimeSources();
      }}
      loading={loading}
    >
      <FormattedMessage id="pages.openapiSources.button.refresh" defaultMessage="刷新" />
    </Button>,
  ];
  if (canWrite) {
    pageActions.push(
      <Button
        key="create"
        type="primary"
        icon={<CloudUploadOutlined />}
        onClick={openCreateSourceModal}
      >
        <FormattedMessage id="pages.openapiSources.button.upload" defaultMessage="上传 Source" />
      </Button>,
    );
  }

  return (
    <PageContainer
      title="OpenAPI Sources"
      subTitle={intl.formatMessage({
        id: 'pages.openapiSources.page.subTitle',
        defaultMessage:
          '上传 OpenAPI 只产生能力契约和诊断；可执行性必须显式绑定 Provider，页面 UI 仍在 Page Studio 确定。',
      })}
      extra={pageActions}
    >
      <Space orientation="vertical" size={16} style={{ width: '100%' }}>
        <Alert
          type="info"
          showIcon
          title={intl.formatMessage({
            id: 'pages.openapiSources.alert.notUi.message',
            defaultMessage: 'Source 不是 UI，也不是自动注册',
          })}
          description={intl.formatMessage({
            id: 'pages.openapiSources.alert.notUi.description',
            defaultMessage:
              'OpenAPI Source 上传即生成 FunctionContract 物料、组件模板与页面提案；未绑定运行时执行器的契约为 unbound（可编排发布、执行被阻断），同名运行时注册自动绑定，不同名可在编辑器抽屉或本页绑定；文档中的 UI、菜单、路由和 renderer 私有字段仍被后端拒绝。',
          })}
          action={
            <Button type="primary" onClick={() => history.push('/functions/pages')}>
              <FormattedMessage
                id="pages.openapiSources.button.openPageStudio"
                defaultMessage="进入 Page Studio"
              />
            </Button>
          }
        />
        {!canWrite ? (
          <Alert
            type="warning"
            showIcon
            title={intl.formatMessage({
              id: 'pages.openapiSources.alert.readOnly.message',
              defaultMessage: '当前是只读模式',
            })}
            description={intl.formatMessage({
              id: 'pages.openapiSources.alert.readOnly.description',
              defaultMessage:
                '你可以查看 Source、operation、diagnostics 和现有 Provider binding；上传、绑定和解绑需要 OpenAPI Source 写权限。',
            })}
          />
        ) : null}
        {sourceDiagnostics.length > 0 ? (
          <Card
            title={intl.formatMessage({
              id: 'pages.openapiSources.card.latestDiagnostics',
              defaultMessage: '最近一次诊断',
            })}
          >
            <Space orientation="vertical" size={6}>
              {sourceDiagnostics.map((item) => (
                <Alert
                  key={`${item.code}:${item.field || ''}:${item.message}`}
                  type={
                    item.severity === 'error'
                      ? 'error'
                      : item.severity === 'warning'
                        ? 'warning'
                        : 'info'
                  }
                  showIcon
                  title={`${item.code}${item.field ? ` @ ${item.field}` : ''}`}
                  description={item.message}
                />
              ))}
            </Space>
          </Card>
        ) : null}
        <Card
          title={intl.formatMessage({
            id: 'pages.openapiSources.runtime.cardTitle',
            defaultMessage: '运行时导入',
          })}
          extra={
            <Typography.Text type="secondary">
              {intl.formatMessage({
                id: 'pages.openapiSources.runtime.cardDescription',
                defaultMessage:
                  'Agent 侧 providers.yaml（type: openapi）注册的函数来源，标注导入 Agent',
              })}
            </Typography.Text>
          }
        >
          <ProTable<RuntimeProviderItem>
            scroll={{ x: 800 }}
            rowKey="providerId"
            dataSource={runtimeSources}
            columns={runtimeColumns}
            search={false}
            pagination={false}
            options={false}
            locale={{
              emptyText: intl.formatMessage({
                id: 'pages.openapiSources.runtime.empty',
                defaultMessage: '当前 scope 没有运行中的 openapi provider',
              }),
            }}
          />
        </Card>
        <Card>
          <ProTable<OpenAPISourceSummary>
            scroll={{ x: 900 }}
            rowKey="sourceId"
            dataSource={sources}
            loading={loading}
            columns={sourceColumns}
            search={false}
            pagination={{ pageSize: 20 }}
            options={false}
          />
        </Card>
      </Space>

      <SourceDetailDrawer
        detail={detail}
        detailLoading={detailLoading}
        canWrite={canWrite}
        onClose={() => setDetail(null)}
        onUpdateSource={openUpdateSourceModal}
        onBindOperation={openBindingModal}
        onRemoveBinding={removeBinding}
      />

      <SourceModal
        open={sourceModalOpen}
        mode={sourceModalMode}
        name={uploadName}
        onNameChange={setUploadName}
        specText={rawSpec}
        onSpecChange={setRawSpec}
        file={uploadFile}
        onFileChange={setUploadFile}
        onCancel={closeSourceModal}
        onOk={submitSource}
      />

      <BindingModal
        open={bindOpen}
        operation={bindingOperation}
        bindingId={bindingId}
        onBindingIdChange={setBindingId}
        functionId={bindingFunctionId}
        onFunctionIdChange={setBindingFunctionId}
        providerId={bindingProviderId}
        onProviderIdChange={setBindingProviderId}
        functionOptions={functionOptions}
        onCancel={() => setBindOpen(false)}
        onOk={submitBinding}
      />

      <PipelineSummaryModal summary={pipelineSummary} onClose={() => setPipelineSummary(null)} />
    </PageContainer>
  );
}
