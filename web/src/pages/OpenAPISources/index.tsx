import React, { useEffect, useMemo, useState } from 'react';
import { PageContainer, ProTable, type ProColumns } from '@ant-design/pro-components';
import {
  Alert,
  App,
  Button,
  Card,
  Drawer,
  Input,
  Modal,
  Popconfirm,
  Select,
  Space,
  Tag,
  Typography,
  Upload,
} from 'antd';
import type { UploadFile } from 'antd/es/upload/interface';
import {
  CloudUploadOutlined,
  DeleteOutlined,
  EditOutlined,
  LinkOutlined,
  ReloadOutlined,
} from '@ant-design/icons';
import { history, useAccess } from '@umijs/max';
import {
  bindOpenAPISourceProvider,
  createOpenAPISource,
  deleteOpenAPISourceBinding,
  getOpenAPISource,
  listOpenAPISources,
  updateOpenAPISource,
  type OpenAPISourceBinding,
  type OpenAPISourceDetail,
  type OpenAPIDocument,
  type OpenAPISourceOperation,
  type OpenAPISourceSummary,
  uploadOpenAPISourceFile,
} from '@/services/api/openapi';
import { listDescriptors, type FunctionDescriptor } from '@/services/api/functions';
import { isScopeReady, subscribeScope } from '@/stores/scope';
import type { Diagnostic } from '@/types/dashboard';

type ApiErrorLike = {
  response?: {
    data?: {
      message?: string;
      details?: Record<string, unknown>;
    };
  };
  message?: string;
};

function errorMessage(error: unknown, fallback: string): string {
  if (error instanceof Error && error.message) return error.message;
  const apiError = error as ApiErrorLike;
  if (apiError?.response?.data?.message) return apiError.response.data.message;
  return fallback;
}

function diagnosticsFromError(error: unknown): Diagnostic[] {
  const apiError = error as ApiErrorLike;
  const details = apiError?.response?.data?.details;
  const raw = details?.diagnostics;
  return Array.isArray(raw) ? raw.filter(isDiagnostic) : [];
}

function isDiagnostic(value: unknown): value is Diagnostic {
  if (!value || typeof value !== 'object') return false;
  const item = value as Partial<Diagnostic>;
  return typeof item.code === 'string' && typeof item.message === 'string';
}

function diagnosticColor(severity?: Diagnostic['severity']) {
  if (severity === 'error') return 'red';
  if (severity === 'warning') return 'orange';
  return 'blue';
}

function riskColor(risk?: string) {
  if (risk === 'danger') return 'red';
  if (risk === 'high') return 'volcano';
  if (risk === 'warning') return 'orange';
  return 'green';
}

function capabilityColor(capability?: string) {
  if (capability === 'task') return 'purple';
  if (capability === 'report') return 'geekblue';
  if (capability === 'collection_query' || capability === 'item_query') return 'cyan';
  if (capability === 'create' || capability === 'update' || capability === 'delete')
    return 'volcano';
  return 'blue';
}

function executionColor(execution?: string) {
  if (execution === 'task') return 'purple';
  return 'green';
}

function formatDate(value?: string): string {
  if (!value) return '-';
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? value : date.toLocaleString();
}

function functionLabel(fn: FunctionDescriptor): string {
  const title =
    fn.summary?.zh || fn.summary?.en || fn.displayName?.zh || fn.displayName?.en || fn.id;
  return `${title} (${fn.id})`;
}

function operationLabel(operation: OpenAPISourceOperation): string {
  return operation.summary || operation.operation || operation.operationId;
}

function proposalInboxPath(proposalKey: string, resourceKey?: string): string {
  const params = new URLSearchParams();
  if (resourceKey) params.set('resourceKey', resourceKey);
  params.set('proposalKey', proposalKey);
  return `/system/functions/proposals?${params.toString()}`;
}

function parseOpenAPIDocument(text: string): OpenAPIDocument {
  const value: unknown = JSON.parse(text);
  if (!value || typeof value !== 'object' || Array.isArray(value)) {
    throw new Error('OpenAPI JSON 必须是对象');
  }
  const record = value as Record<string, unknown>;
  if (typeof record.openapi !== 'string' || record.openapi.trim() === '') {
    throw new Error('OpenAPI JSON 缺少 openapi 字段');
  }
  if (!record.info || typeof record.info !== 'object' || Array.isArray(record.info)) {
    throw new Error('OpenAPI JSON 缺少 info 对象');
  }
  return value as OpenAPIDocument;
}

type SourceModalMode = 'create' | 'update';

export default function OpenAPISourcesPage() {
  const { message } = App.useApp();
  const access = useAccess() as {
    canOpenAPISourcesWrite?: boolean;
  };
  const canWrite = !!access.canOpenAPISourcesWrite;
  const [loading, setLoading] = useState(false);
  const [sources, setSources] = useState<OpenAPISourceSummary[]>([]);
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
    } finally {
      setLoading(false);
    }
  };

  const loadFunctions = async () => {
    setFunctions(await listDescriptors());
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
    loadFunctions();
  }, [scopeKey]);

  const functionOptions = useMemo(
    () => functions.map((fn) => ({ label: functionLabel(fn), value: fn.id })),
    [functions],
  );

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
      message.error('没有 OpenAPI Source 写权限');
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
      message.error(errorMessage(error, '加载 OpenAPI Source 失败'));
    }
  };

  const closeSourceModal = () => {
    setSourceModalOpen(false);
    resetSourceForm();
  };

  const submitSource = async () => {
    if (!canWrite) {
      message.error('没有 OpenAPI Source 写权限');
      return;
    }
    setSourceDiagnostics([]);
    try {
      let response;
      if (isUpdatingSource) {
        if (!editingSource) {
          message.error('缺少要更新的 OpenAPI Source');
          return;
        }
        const text = rawSpec.trim();
        if (!text) {
          message.warning('请粘贴新的 OpenAPI JSON');
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
          message.warning('请上传文件或粘贴 OpenAPI JSON');
          return;
        }
        response = await createOpenAPISource(parseOpenAPIDocument(text), uploadName || undefined);
      }
      message.success(isUpdatingSource ? 'OpenAPI Source 已更新' : 'OpenAPI Source 已创建');
      setSourceModalOpen(false);
      resetSourceForm();
      await loadSources();
      await openDetail(response.source.sourceId);
    } catch (error) {
      const diagnostics = diagnosticsFromError(error);
      if (diagnostics.length > 0) {
        setSourceDiagnostics(diagnostics);
        message.error('OpenAPI Source 校验失败，请查看诊断');
        return;
      }
      message.error(errorMessage(error, '创建 OpenAPI Source 失败'));
    }
  };

  const openBindingModal = (operation: OpenAPISourceOperation) => {
    if (!canWrite) {
      message.error('没有 OpenAPI Source 写权限');
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
      message.error('没有 OpenAPI Source 写权限');
      return;
    }
    if (!detail || !bindingOperation || !bindingFunctionId) {
      message.warning('请选择要绑定的函数');
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
        Modal.success({
          title: 'Provider binding 已保存',
          content: `已生成默认页面 Proposal：${result.proposal.proposalKey}。请进入 Proposal 队列预览并发布，发布后才会出现在运行控制台菜单。`,
          okText: '打开 Proposal',
          onOk: () =>
            history.push(
              proposalInboxPath(result.proposal!.proposalKey, result.proposal!.resourceKey),
            ),
        });
      } else {
        message.warning(
          'Provider binding 已保存，但未返回可发布 Proposal。请在 Proposal 队列查看诊断。',
        );
      }
    } catch (error) {
      message.error(errorMessage(error, '保存 binding 失败'));
    }
  };

  const removeBinding = async (binding: OpenAPISourceBinding) => {
    if (!canWrite) {
      message.error('没有 OpenAPI Source 写权限');
      return;
    }
    if (!detail) return;
    await deleteOpenAPISourceBinding(detail.sourceId, binding.bindingId);
    message.success('binding 已删除');
    await openDetail(detail.sourceId);
    await loadSources();
  };

  const sourceColumns: ProColumns<OpenAPISourceSummary>[] = [
    {
      title: 'Source',
      dataIndex: 'name',
      render: (_, record) => (
        <Space direction="vertical" size={0}>
          <Typography.Text strong>{record.name}</Typography.Text>
          <Typography.Text code>{record.sourceId}</Typography.Text>
        </Space>
      ),
    },
    {
      title: '版本',
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
      title: '操作数',
      dataIndex: 'operationCount',
      width: 100,
    },
    {
      title: '诊断',
      dataIndex: 'diagnosticCount',
      width: 100,
      render: (_, record) => (
        <Tag color={record.diagnosticCount > 0 ? 'orange' : 'green'}>{record.diagnosticCount}</Tag>
      ),
    },
    {
      title: '更新时间',
      dataIndex: 'updatedAt',
      width: 180,
      render: (_, record) => formatDate(record.updatedAt),
    },
    {
      title: '操作',
      valueType: 'option',
      width: 170,
      render: (_, record) => {
        const actions = [
          <Button key="open" type="link" size="small" onClick={() => openDetail(record.sourceId)}>
            打开
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
              更新
            </Button>,
          );
        }
        return actions;
      },
    },
  ];

  const operationColumns: ProColumns<OpenAPISourceOperation>[] = [
    {
      title: 'Operation',
      dataIndex: 'operationId',
      render: (_, record) => (
        <Space direction="vertical" size={0}>
          <Typography.Text strong>{operationLabel(record)}</Typography.Text>
          <Typography.Text code>{record.operationId}</Typography.Text>
          <Typography.Text type="secondary">{`${record.method} ${record.path}`}</Typography.Text>
        </Space>
      ),
    },
    {
      title: '能力契约',
      dataIndex: 'resource',
      width: 320,
      render: (_, record) => (
        <Space direction="vertical" size={4}>
          <Space size={4} wrap>
            <Tag color={record.resource ? 'blue' : undefined}>
              {record.resource || '无 resource'}
            </Tag>
            <Tag color={record.operation ? undefined : 'default'}>
              {record.operation || '无 operation'}
            </Tag>
            <Tag color={capabilityColor(record.capability)}>
              {record.capability || '无 capability'}
            </Tag>
            <Tag color={executionColor(record.execution)}>{record.execution || '无 execution'}</Tag>
            <Tag color={record.approval?.required ? 'orange' : 'default'}>
              {record.approval?.required
                ? `approval:${record.approval.policyKey || 'required'}`
                : '无 approval'}
            </Tag>
            <Tag color={riskColor(record.risk)}>{record.risk || '无 risk'}</Tag>
          </Space>
          <Typography.Text code>{record.permission || '无 permission'}</Typography.Text>
        </Space>
      ),
    },
    {
      title: 'Provider Binding',
      dataIndex: 'bound',
      width: 260,
      render: (_, record) => (
        <Space direction="vertical" size={2}>
          <Tag color={record.bound ? 'green' : 'orange'}>{record.bound ? 'bound' : 'unbound'}</Tag>
          {record.bindingId ? <Typography.Text code>{record.bindingId}</Typography.Text> : null}
          {record.functionId ? <Typography.Text>{record.functionId}</Typography.Text> : null}
        </Space>
      ),
    },
    {
      title: '操作',
      valueType: 'option',
      width: 110,
      render: (_, record) => [
        canWrite ? (
          <Button
            key="bind"
            type="link"
            size="small"
            icon={<LinkOutlined />}
            onClick={() => openBindingModal(record)}
          >
            绑定
          </Button>
        ) : (
          <Typography.Text key="readonly" type="secondary">
            只读
          </Typography.Text>
        ),
      ],
    },
  ];

  const bindingColumns: ProColumns<OpenAPISourceBinding>[] = [
    {
      title: 'bindingId',
      dataIndex: 'bindingId',
      render: (_, record) => <Typography.Text code>{record.bindingId}</Typography.Text>,
    },
    { title: 'operationId', dataIndex: 'operationId' },
    { title: 'functionId', dataIndex: 'functionId' },
    {
      title: 'kind',
      dataIndex: 'kind',
      width: 100,
      render: (_, record) => <Tag>{record.kind}</Tag>,
    },
    {
      title: '操作',
      valueType: 'option',
      width: 110,
      render: (_, record) => [
        canWrite ? (
          <Popconfirm key="delete" title="删除此 binding？" onConfirm={() => removeBinding(record)}>
            <Button type="link" danger size="small" icon={<DeleteOutlined />}>
              删除
            </Button>
          </Popconfirm>
        ) : (
          <Typography.Text key="readonly" type="secondary">
            只读
          </Typography.Text>
        ),
      ],
    },
  ];

  const pageActions = [
    <Button key="reload" icon={<ReloadOutlined />} onClick={loadSources} loading={loading}>
      刷新
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
        上传 Source
      </Button>,
    );
  }

  return (
    <PageContainer
      title="OpenAPI Sources"
      subTitle="上传 OpenAPI 只产生能力契约和诊断；可执行性必须显式绑定 Provider，页面 UI 仍在 Page Studio 确定。"
      extra={pageActions}
    >
      <Space direction="vertical" size={16} style={{ width: '100%' }}>
        <Alert
          type="info"
          showIcon
          message="Source 不是 UI，也不是自动注册"
          description="OpenAPI Source 用于解析 FunctionSpec / ResourceSpec / OperationSpec 和 PageCandidate 诊断；Source 未绑定 Provider 前不可执行，上传文档中的 UI、菜单、路由和 renderer 私有字段会被后端拒绝。"
        />
        {!canWrite ? (
          <Alert
            type="warning"
            showIcon
            message="当前是只读模式"
            description="你可以查看 Source、operation、diagnostics 和现有 Provider binding；上传、绑定和解绑需要 OpenAPI Source 写权限。"
          />
        ) : null}
        {sourceDiagnostics.length > 0 ? (
          <Card title="最近一次诊断">
            <Space direction="vertical" size={6}>
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
                  message={`${item.code}${item.field ? ` @ ${item.field}` : ''}`}
                  description={item.message}
                />
              ))}
            </Space>
          </Card>
        ) : null}
        <Card>
          <ProTable<OpenAPISourceSummary>
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

      <Drawer
        title={detail ? detail.name : 'OpenAPI Source'}
        open={!!detail}
        onClose={() => setDetail(null)}
        extra={
          detail && canWrite ? (
            <Button icon={<EditOutlined />} onClick={() => openUpdateSourceModal(detail)}>
              更新 Source
            </Button>
          ) : null
        }
        width="86vw"
        destroyOnClose
      >
        {detail ? (
          <Space direction="vertical" size={16} style={{ width: '100%' }}>
            <Card loading={detailLoading}>
              <Space wrap>
                <Tag>{`rev ${detail.revision}`}</Tag>
                <Tag>{detail.format}</Tag>
                <Tag>{detail.openapiVersion}</Tag>
                <Tag
                  color={detail.diagnosticCount > 0 ? 'orange' : 'green'}
                >{`diagnostics ${detail.diagnosticCount}`}</Tag>
                <Typography.Text code>{detail.contentHash}</Typography.Text>
              </Space>
            </Card>
            <Card title="Diagnostics" loading={detailLoading}>
              {(detail.diagnostics || []).length === 0 ? (
                <Typography.Text type="secondary">无诊断</Typography.Text>
              ) : (
                <Space direction="vertical" size={8} style={{ width: '100%' }}>
                  {(detail.diagnostics || []).map((item) => (
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
                      message={
                        <Space>
                          <Tag color={diagnosticColor(item.severity)}>{item.severity}</Tag>
                          <Typography.Text code>{item.code}</Typography.Text>
                          {item.field ? <Typography.Text>{item.field}</Typography.Text> : null}
                        </Space>
                      }
                      description={item.message}
                    />
                  ))}
                </Space>
              )}
            </Card>
            <Card title="Operations" loading={detailLoading}>
              <ProTable<OpenAPISourceOperation>
                rowKey="operationId"
                dataSource={detail.operations || []}
                columns={operationColumns}
                search={false}
                pagination={{ pageSize: 10 }}
                options={false}
              />
            </Card>
            <Card title="Provider Bindings" loading={detailLoading}>
              <ProTable<OpenAPISourceBinding>
                rowKey="bindingId"
                dataSource={detail.bindings || []}
                columns={bindingColumns}
                search={false}
                pagination={false}
                options={false}
              />
            </Card>
            <Card title="原始 OpenAPI JSON">
              <Typography.Paragraph copyable>
                <Typography.Text code>{JSON.stringify(detail.spec || {}, null, 2)}</Typography.Text>
              </Typography.Paragraph>
            </Card>
          </Space>
        ) : null}
      </Drawer>

      <Modal
        title={isUpdatingSource ? '更新 OpenAPI Source' : '上传 OpenAPI Source'}
        open={sourceModalOpen}
        onCancel={closeSourceModal}
        onOk={submitSource}
        okText={isUpdatingSource ? '更新 revision' : '创建'}
        width={760}
      >
        <Space direction="vertical" size={12} style={{ width: '100%' }}>
          <Alert
            type={isUpdatingSource ? 'info' : 'warning'}
            showIcon
            message={isUpdatingSource ? '更新只产生新的 Source revision' : '不要在 OpenAPI 中写 UI'}
            description={
              isUpdatingSource
                ? '更新会刷新 Source 的 operations 和 diagnostics，保留现有 Provider binding；OpenAPI 不能写 UI，只允许 x-resource/x-operation/x-capability/x-execution/x-risk/x-enabled/x-permission。'
                : 'OpenAPI 不能写 UI，只允许 x-resource/x-operation/x-capability/x-execution/x-risk/x-enabled/x-permission。'
            }
          />
          <Input
            addonBefore="name"
            placeholder="可选，默认使用 info.title"
            value={uploadName}
            onChange={(event) => setUploadName(event.target.value)}
          />
          {isUpdatingSource ? null : (
            <Upload
              beforeUpload={(file) => {
                setUploadFile(file);
                return false;
              }}
              maxCount={1}
              fileList={uploadFile ? [uploadFile] : []}
              onRemove={() => {
                setUploadFile(null);
                return true;
              }}
            >
              <Button icon={<CloudUploadOutlined />}>选择 JSON/YAML 文件</Button>
            </Upload>
          )}
          <Input.TextArea
            rows={12}
            placeholder={
              isUpdatingSource
                ? '粘贴新的 OpenAPI JSON。YAML 更新请走 API raw PUT。'
                : '或粘贴 OpenAPI JSON。YAML 请使用文件上传。'
            }
            value={rawSpec}
            onChange={(event) => setRawSpec(event.target.value)}
          />
        </Space>
      </Modal>

      <Modal
        title={bindingOperation ? `绑定 ${bindingOperation.operationId}` : '绑定 Provider'}
        open={bindOpen}
        onCancel={() => setBindOpen(false)}
        onOk={submitBinding}
        okText="保存 binding"
      >
        <Space direction="vertical" size={12} style={{ width: '100%' }}>
          <Alert
            type="info"
            showIcon
            message="当前只启用 Provider binding"
            description="httpConnector 需要 allowlist、SecretRef、超时/重试和审计策略后才能开放。"
          />
          <Input
            addonBefore="bindingId"
            value={bindingId}
            onChange={(event) => setBindingId(event.target.value)}
          />
          <Select
            showSearch
            placeholder="选择已注册函数"
            value={bindingFunctionId}
            onChange={setBindingFunctionId}
            options={functionOptions}
            optionFilterProp="label"
            style={{ width: '100%' }}
          />
          <Input
            addonBefore="providerId"
            placeholder="可选；留空由运行时按函数路由"
            value={bindingProviderId}
            onChange={(event) => setBindingProviderId(event.target.value)}
          />
        </Space>
      </Modal>
    </PageContainer>
  );
}
