/**
 * ResourceCatalogPage - 资源能力目录。
 *
 * 这里只管理 FunctionContract 聚合后的 CapabilitySemantics：
 * identity、collection、lifecycle binding、语义来源和冲突决议。
 * 页面标题、菜单、列、按钮位置属于 Page Proposal/Page Studio。
 */

import React, { useCallback, useEffect, useState } from 'react';
import {
  Alert,
  Button,
  Card,
  Descriptions,
  Form,
  Input,
  message,
  Modal,
  Select,
  Space,
  Table,
  Tag,
  Typography,
} from 'antd';
import {
  BranchesOutlined,
  BulbOutlined,
  CheckCircleOutlined,
  EditOutlined,
  EyeOutlined,
  FunctionOutlined,
  ReloadOutlined,
  SearchOutlined,
  WarningOutlined,
} from '@ant-design/icons';
import { history } from '@umijs/max';
import type { ColumnsType } from 'antd/es/table';
import type {
  AffectedPageInfo,
  CapabilityKind,
  DiagnosticInfo,
  FunctionInfo,
  ResourceCatalogItem,
  ResourceSemanticConflicts,
  ResourceSemanticVersionInfo,
  ResourceSemanticVersions,
  ResolveSemanticConflictRequest,
  SemanticConflictInfo,
  SemanticProvenanceInfo,
  SemanticSource,
  SemanticsInfo,
  TaskSemanticInfo,
  UpdateResourceSemanticsRequest,
} from '@/types/dashboard';
import {
  getResourceDetail,
  getResourceSemanticConflicts,
  getResourceSemanticVersions,
  listResourceCatalog,
  resolveResourceSemanticConflict,
  updateResourceSemantics,
} from '@/services/dashboard';
import { localizedText } from '@/utils/localizedText';

const { Text, Title } = Typography;

const statusColors: Record<ResourceCatalogItem['status'], string> = {
  identified: 'success',
  pending: 'warning',
  conflict: 'error',
  not_executable: 'default',
};

const statusLabels: Record<ResourceCatalogItem['status'], string> = {
  identified: '已识别',
  pending: '待确认',
  conflict: '冲突',
  not_executable: '不可执行',
};

const capabilityLabels: Record<CapabilityKind, string> = {
  collection_query: '列表查询',
  item_query: '详情查询',
  create: '创建',
  update: '更新',
  delete: '删除',
  action: '动作',
  task: '任务',
  report: '报表',
};

const semanticSources: SemanticSource[] = ['platform_review', 'sdk_explicit', 'openapi_rest'];

const sourceLabels: Record<SemanticSource, string> = {
  platform_review: '平台确认',
  sdk_explicit: 'SDK 显式',
  openapi_rest: 'OpenAPI REST',
};

const sourceColors: Record<SemanticSource, string> = {
  platform_review: 'green',
  sdk_explicit: 'blue',
  openapi_rest: 'gold',
};

const riskColors: Record<string, string> = {
  danger: 'red',
  high: 'orange',
  warning: 'gold',
  safe: 'green',
};

const affectedKindLabels: Record<'draft' | 'published' | 'proposal', string> = {
  draft: '草稿',
  published: '已发布',
  proposal: '提案',
};

const affectedKindColors: Record<'draft' | 'published' | 'proposal', string> = {
  draft: 'blue',
  published: 'green',
  proposal: 'gold',
};

const emptySemanticMeta: ResourceSemanticConflicts = {
  conflicts: [],
  provenance: [],
};

const emptySemanticVersions: ResourceSemanticVersions = {
  items: [],
  total: 0,
};

const displaySemanticValue = (value?: string): string => {
  if (!value) {
    return '-';
  }
  try {
    const parsed = JSON.parse(value);
    return typeof parsed === 'string' ? parsed : JSON.stringify(parsed);
  } catch {
    return value;
  }
};

const pageTitleText = (page?: AffectedPageInfo): string =>
  localizedText(page?.title, 'zh-CN', '-') || page?.pageKey || '-';

const bindingFreshnessSummary = (page?: AffectedPageInfo): string => {
  if (!page?.bindingFreshness || page.bindingFreshness.length === 0) {
    return '无';
  }
  return page.bindingFreshness.map((item) => item.status).join(', ');
};

const conflictSources = (conflict: SemanticConflictInfo): SemanticSource[] =>
  semanticSources.filter((source) => conflict.values[source] !== undefined);

const semanticsToFormValues = (semantics?: SemanticsInfo): UpdateResourceSemanticsRequest => ({
  identityField: semantics?.identityField,
  identityFieldType: semantics?.identityFieldType || 'string',
  identityPath: semantics?.identityPath,
  collectionQueryId: semantics?.collectionQueryId,
  collectionPath: semantics?.collectionPath,
  pageFieldName: semantics?.pageFieldName,
  pageSizeFieldName: semantics?.pageSizeFieldName,
  itemsFieldName: semantics?.itemsFieldName,
  totalFieldName: semantics?.totalFieldName,
  itemQueryId: semantics?.itemQueryId,
  itemPath: semantics?.itemPath,
  createId: semantics?.createId,
  updateId: semantics?.updateId,
  deleteId: semantics?.deleteId,
  actions: semantics?.actions || [],
  tasks: semantics?.tasks || [],
  reports: semantics?.reports || [],
});

const compactSemanticsPayload = (
  values: UpdateResourceSemanticsRequest,
): UpdateResourceSemanticsRequest => {
  const payload: UpdateResourceSemanticsRequest = {};

  const assignString = (key: keyof UpdateResourceSemanticsRequest, value?: string) => {
    const trimmed = value?.trim();
    if (trimmed) {
      Object.assign(payload, { [key]: trimmed });
    }
  };
  const assignNumber = (key: keyof UpdateResourceSemanticsRequest, value?: number) => {
    if (typeof value === 'number' && value > 0) {
      Object.assign(payload, { [key]: value });
    }
  };

  assignString('identityField', values.identityField);
  assignString('identityFieldType', values.identityFieldType);
  assignString('identityPath', values.identityPath);
  assignNumber('collectionQueryId', values.collectionQueryId);
  assignString('collectionPath', values.collectionPath);
  assignString('pageFieldName', values.pageFieldName);
  assignString('pageSizeFieldName', values.pageSizeFieldName);
  assignString('itemsFieldName', values.itemsFieldName);
  assignString('totalFieldName', values.totalFieldName);
  assignNumber('itemQueryId', values.itemQueryId);
  assignString('itemPath', values.itemPath);
  assignNumber('createId', values.createId);
  assignNumber('updateId', values.updateId);
  assignNumber('deleteId', values.deleteId);
  if (values.actions) {
    const actions = values.actions
      .map((action) => {
        const functionId = action.functionId?.trim();
        if (!functionId || !action.subject) {
          return undefined;
        }
        return {
          functionId,
          subject: action.subject,
          identityInput: action.identityInput?.trim(),
        };
      })
      .filter((action): action is NonNullable<typeof action> => Boolean(action));
    Object.assign(payload, { actions });
  }
  if (values.tasks) {
    const tasks = values.tasks
      .map(compactTaskSemantic)
      .filter((task): task is TaskSemanticInfo => Boolean(task));
    Object.assign(payload, { tasks });
  }
  if (values.reports) {
    const reports = values.reports
      .map((report) => ({
        query: { functionId: report.query?.functionId?.trim() || '' },
        datasetPath: report.datasetPath?.trim() || '',
        dimensions: (report.dimensions || [])
          .map((item) => item?.trim())
          .filter((item): item is string => Boolean(item)),
        metrics: (report.metrics || [])
          .map((item) => item?.trim())
          .filter((item): item is string => Boolean(item)),
      }))
      .filter((report) => report.query.functionId);
    Object.assign(payload, { reports });
  }
  assignString('changeReason', values.changeReason);

  return payload;
};

const compactTaskSemantic = (task: TaskSemanticInfo): TaskSemanticInfo | undefined => {
  const startFunctionId = task.start?.functionId?.trim();
  const statusFunctionId = task.status?.function?.functionId?.trim();
  if (!startFunctionId || !statusFunctionId) {
    return undefined;
  }
  const next: TaskSemanticInfo = {
    start: { functionId: startFunctionId },
    taskId: {
      resultPath: task.taskId?.resultPath?.trim() || '',
      valueType: task.taskId?.valueType || 'string',
    },
    status: {
      function: { functionId: statusFunctionId },
      taskIdInput: task.status?.taskIdInput?.trim() || '',
      statePath: task.status?.statePath?.trim() || '',
    },
  };
  if (task.events?.function?.functionId) {
    next.events = {
      function: { functionId: task.events.function.functionId.trim() },
      taskIdInput: task.events.taskIdInput?.trim() || '',
      eventsPath: task.events.eventsPath?.trim() || '',
    };
  }
  if (task.result?.function?.functionId) {
    next.result = {
      function: { functionId: task.result.function.functionId.trim() },
      taskIdInput: task.result.taskIdInput?.trim() || '',
      resultPath: task.result.resultPath?.trim() || '',
    };
  }
  if (task.cancel?.function?.functionId) {
    next.cancel = {
      function: { functionId: task.cancel.function.functionId.trim() },
      taskIdInput: task.cancel.taskIdInput?.trim() || '',
    };
  }
  return next;
};

const ResourceCatalogPage: React.FC = () => {
  const [loading, setLoading] = useState(false);
  const [detailLoading, setDetailLoading] = useState(false);
  const [data, setData] = useState<ResourceCatalogItem[]>([]);
  const [total, setTotal] = useState(0);
  const [category, setCategory] = useState<string>('');
  const [query, setQuery] = useState<string>('');
  const [selectedResource, setSelectedResource] = useState<ResourceCatalogItem | null>(null);
  const [semanticMeta, setSemanticMeta] = useState<ResourceSemanticConflicts>(emptySemanticMeta);
  const [semanticVersions, setSemanticVersions] =
    useState<ResourceSemanticVersions>(emptySemanticVersions);
  // 语义版本服务端分页：版本历史可达上万条，必须按页拉取
  const [versionPage, setVersionPage] = useState(1);
  const [versionPageSize, setVersionPageSize] = useState(5);
  const [selectedConflict, setSelectedConflict] = useState<SemanticConflictInfo | null>(null);
  const [detailVisible, setDetailVisible] = useState(false);
  const [editVisible, setEditVisible] = useState(false);
  const [resolveVisible, setResolveVisible] = useState(false);
  const [editForm] = Form.useForm<UpdateResourceSemanticsRequest>();
  const [resolveForm] = Form.useForm<ResolveSemanticConflictRequest>();

  const fetchData = useCallback(async () => {
    setLoading(true);
    try {
      const result = await listResourceCatalog({
        category: category || undefined,
        query: query || undefined,
      });
      setData(result.items);
      setTotal(result.total);
    } catch (error) {
      const errMsg = error instanceof Error ? error.message : '操作失败';
      message.error(errMsg);
    } finally {
      setLoading(false);
    }
  }, [category, query]);

  useEffect(() => {
    fetchData();
  }, [fetchData]);

  const fetchSemanticVersions = useCallback(
    async (resourceKey: string, page: number, pageSize: number) => {
      try {
        const versions = await getResourceSemanticVersions(resourceKey, {
          limit: pageSize,
          offset: (page - 1) * pageSize,
        });
        setSemanticVersions(versions);
      } catch (error) {
        const errMsg = error instanceof Error ? error.message : '未知错误';
        message.error('获取语义版本失败: ' + errMsg);
      }
    },
    [],
  );

  const loadResourceDetail = useCallback(
    async (resourceKey: string) => {
      setDetailLoading(true);
      try {
        const [detail, meta] = await Promise.all([
          getResourceDetail(resourceKey),
          getResourceSemanticConflicts(resourceKey),
        ]);
        setSelectedResource(detail);
        setSemanticMeta(meta);
        await fetchSemanticVersions(resourceKey, versionPage, versionPageSize);
        return detail;
      } catch (error) {
        const errMsg = error instanceof Error ? error.message : '未知错误';
        message.error('获取详情失败: ' + errMsg);
        return null;
      } finally {
        setDetailLoading(false);
      }
    },
    [fetchSemanticVersions, versionPage, versionPageSize],
  );

  const handleViewDetail = useCallback(
    async (resourceKey: string) => {
      const detail = await loadResourceDetail(resourceKey);
      if (detail) {
        setDetailVisible(true);
      }
    },
    [loadResourceDetail],
  );

  const handleEditSemantics = useCallback(
    async (resourceKey: string) => {
      const detail = await loadResourceDetail(resourceKey);
      if (!detail) {
        return;
      }
      editForm.setFieldsValue(semanticsToFormValues(detail.semantics));
      setEditVisible(true);
    },
    [editForm, loadResourceDetail],
  );

  const handleSaveSemantics = useCallback(async () => {
    if (!selectedResource) {
      return;
    }

    try {
      const values = await editForm.validateFields();
      await updateResourceSemantics(selectedResource.resourceKey, compactSemanticsPayload(values));
      message.success('语义更新成功');
      setEditVisible(false);
      await loadResourceDetail(selectedResource.resourceKey);
      fetchData();
    } catch (error) {
      const errMsg = error instanceof Error ? error.message : '未知错误';
      message.error('更新失败: ' + errMsg);
    }
  }, [editForm, fetchData, loadResourceDetail, selectedResource]);

  const handleOpenResolve = useCallback(
    (conflict: SemanticConflictInfo) => {
      const sources = conflictSources(conflict);
      setSelectedConflict(conflict);
      resolveForm.setFieldsValue({
        chosenSource: sources[0],
        reason: '',
      });
      setResolveVisible(true);
    },
    [resolveForm],
  );

  const handleResolveConflict = useCallback(async () => {
    if (!selectedResource || !selectedConflict) {
      return;
    }

    try {
      const values = await resolveForm.validateFields();
      await resolveResourceSemanticConflict(
        selectedResource.resourceKey,
        selectedConflict.field,
        values,
      );
      message.success('冲突已解决，相关 Proposal 已触发重算');
      setResolveVisible(false);
      await loadResourceDetail(selectedResource.resourceKey);
      fetchData();
    } catch (error) {
      const errMsg = error instanceof Error ? error.message : '未知错误';
      message.error('解决冲突失败: ' + errMsg);
    }
  }, [fetchData, loadResourceDetail, resolveForm, selectedConflict, selectedResource]);

  const handleOpenProposals = useCallback((resourceKey: string) => {
    history.push(`/system/functions/pages?resourceKey=${encodeURIComponent(resourceKey)}`);
  }, []);

  const renderFunctionSelect = (capability: CapabilityKind, placeholder: string) => (
    <Select<number>
      allowClear
      placeholder={placeholder}
      showSearch
      optionFilterProp="label"
      options={(selectedResource?.functions || [])
        .filter((fn) => fn.capability === capability)
        .map((fn) => ({
          value: fn.id,
          label: `${fn.functionId} #${fn.id}`,
          disabled: !fn.enabled,
        }))}
    />
  );

  const renderFunctionIdSelect = (
    placeholder: string,
    capability?: CapabilityKind,
    width = 260,
  ) => (
    <Select<string>
      allowClear
      placeholder={placeholder}
      showSearch
      optionFilterProp="label"
      style={{ width }}
      options={(selectedResource?.functions || [])
        .filter((fn) => !capability || fn.capability === capability)
        .map((fn) => ({
          value: fn.functionId,
          label: `${fn.functionId} #${fn.id} / ${capabilityLabels[fn.capability] || fn.capability}`,
          disabled: !fn.enabled,
        }))}
    />
  );

  const categoryOptions = data
    .map((item) => item.categoryKey)
    .filter((item): item is string => Boolean(item))
    .filter((item, index, all) => all.indexOf(item) === index)
    .sort();

  const columns: ColumnsType<ResourceCatalogItem> = [
    {
      title: '资源标识',
      dataIndex: 'resourceKey',
      key: 'resourceKey',
      render: (text: string) => <Text strong>{text}</Text>,
    },
    {
      title: '名称',
      dataIndex: 'labels',
      key: 'labels',
      render: (labels: ResourceCatalogItem['labels']) => localizedText(labels, 'zh-CN', '-'),
    },
    {
      title: '分类',
      dataIndex: 'categoryKey',
      key: 'categoryKey',
      render: (text?: string) => text || '-',
    },
    {
      title: '状态',
      dataIndex: 'status',
      key: 'status',
      render: (status: ResourceCatalogItem['status']) => (
        <Tag color={statusColors[status]}>{statusLabels[status]}</Tag>
      ),
    },
    {
      title: '函数数量',
      dataIndex: 'functions',
      key: 'functions',
      render: (functions: FunctionInfo[]) => functions?.length || 0,
    },
    {
      title: '语义版本',
      dataIndex: 'semantics',
      key: 'semantics',
      render: (semantics?: SemanticsInfo) => semantics?.version || '-',
    },
    {
      title: '诊断',
      dataIndex: 'diagnostics',
      key: 'diagnostics',
      render: (diagnostics?: DiagnosticInfo[]) => {
        if (!diagnostics || diagnostics.length === 0) {
          return <Tag color="success">无</Tag>;
        }
        const errors = diagnostics.filter((diagnostic) => diagnostic.severity === 'error').length;
        const warnings = diagnostics.filter(
          (diagnostic) => diagnostic.severity === 'warning',
        ).length;
        return (
          <Space>
            {errors > 0 && <Tag color="error">{errors} 错误</Tag>}
            {warnings > 0 && <Tag color="warning">{warnings} 警告</Tag>}
          </Space>
        );
      },
    },
    {
      title: '操作',
      key: 'action',
      render: (_, record) => (
        <Space>
          <Button
            type="link"
            icon={<EyeOutlined />}
            onClick={() => handleViewDetail(record.resourceKey)}
          >
            查看
          </Button>
          <Button
            type="link"
            icon={<EditOutlined />}
            onClick={() => handleEditSemantics(record.resourceKey)}
          >
            编辑语义
          </Button>
          <Button
            type="link"
            icon={<BulbOutlined />}
            onClick={() => handleOpenProposals(record.resourceKey)}
          >
            提案
          </Button>
        </Space>
      ),
    },
  ];

  return (
    <div>
      <Card style={{ marginBottom: 16 }}>
        <Space wrap>
          <Input
            placeholder="搜索资源"
            prefix={<SearchOutlined />}
            value={query}
            onChange={(event) => setQuery(event.target.value)}
            onPressEnter={fetchData}
            style={{ width: 220 }}
          />
          <Select
            placeholder="选择分类"
            value={category || undefined}
            onChange={(value) => setCategory(value || '')}
            allowClear
            style={{ width: 180 }}
            options={categoryOptions.map((item) => ({
              value: item,
              label: item,
            }))}
          />
          <Button type="primary" icon={<SearchOutlined />} onClick={fetchData}>
            搜索
          </Button>
          <Button icon={<ReloadOutlined />} onClick={fetchData}>
            刷新
          </Button>
        </Space>
      </Card>

      <Card title="资源能力目录">
        <Table
          columns={columns}
          dataSource={data}
          rowKey="resourceKey"
          loading={loading}
          pagination={{
            total,
            pageSize: 20,
            showSizeChanger: true,
            showTotal: (value) => `共 ${value} 条`,
          }}
        />
      </Card>

      <Modal
        title="资源详情"
        open={detailVisible}
        onCancel={() => setDetailVisible(false)}
        footer={null}
        width={1000}
      >
        {selectedResource && (
          <div>
            <Alert
              type={selectedResource.status === 'conflict' ? 'error' : 'info'}
              showIcon
              style={{ marginBottom: 16 }}
              message="Resource Catalog 只维护资源能力语义"
              description="这里补充 identity、collection 和生命周期函数绑定；页面标题、菜单、列、按钮位置属于 Page Proposal/Page Studio。"
            />

            <Descriptions column={2} bordered>
              <Descriptions.Item label="资源标识">{selectedResource.resourceKey}</Descriptions.Item>
              <Descriptions.Item label="名称">
                {localizedText(selectedResource.labels, 'zh-CN', '-')}
              </Descriptions.Item>
              <Descriptions.Item label="分类">
                {selectedResource.categoryKey || '-'}
              </Descriptions.Item>
              <Descriptions.Item label="状态">
                <Tag color={statusColors[selectedResource.status]}>
                  {statusLabels[selectedResource.status]}
                </Tag>
              </Descriptions.Item>
              <Descriptions.Item label="未解决冲突">
                {selectedResource.semantics?.unresolvedConflicts ? (
                  <Tag color="error">{selectedResource.semantics.unresolvedConflicts}</Tag>
                ) : (
                  <Tag color="success">0</Tag>
                )}
              </Descriptions.Item>
              <Descriptions.Item label="Proposal">
                <Button
                  size="small"
                  icon={<BulbOutlined />}
                  onClick={() => handleOpenProposals(selectedResource.resourceKey)}
                >
                  查看相关提案
                </Button>
              </Descriptions.Item>
            </Descriptions>

            <Title level={5} style={{ marginTop: 16 }}>
              <FunctionOutlined /> 函数列表
            </Title>
            <Table<FunctionInfo>
              dataSource={selectedResource.functions}
              rowKey="id"
              pagination={false}
              size="small"
              columns={[
                { title: '函数 ID', dataIndex: 'functionId', key: 'functionId' },
                { title: 'DB ID', dataIndex: 'id', key: 'id', width: 80 },
                { title: '版本', dataIndex: 'version', key: 'version' },
                {
                  title: '能力',
                  dataIndex: 'capability',
                  key: 'capability',
                  render: (text: CapabilityKind) => <Tag>{capabilityLabels[text] || text}</Tag>,
                },
                {
                  title: '执行方式',
                  dataIndex: 'execution',
                  key: 'execution',
                  render: (text: string) => <Tag>{text}</Tag>,
                },
                {
                  title: '风险',
                  dataIndex: 'risk',
                  key: 'risk',
                  render: (text: string) => <Tag color={riskColors[text] || 'default'}>{text}</Tag>,
                },
                {
                  title: '状态',
                  dataIndex: 'enabled',
                  key: 'enabled',
                  render: (enabled: boolean) => (
                    <Tag color={enabled ? 'success' : 'default'}>{enabled ? '启用' : '禁用'}</Tag>
                  ),
                },
              ]}
            />

            <Title level={5} style={{ marginTop: 16 }}>
              <BulbOutlined /> 受影响页面
            </Title>
            <Table<AffectedPageInfo>
              dataSource={selectedResource.affectedPages || []}
              rowKey={(record) => `${record.kind}:${record.proposalKey || record.pageKey}`}
              pagination={false}
              size="small"
              locale={{ emptyText: '当前资源还没有草稿、已发布页面或提案' }}
              columns={[
                {
                  title: '类型',
                  dataIndex: 'kind',
                  key: 'kind',
                  width: 100,
                  render: (kind: AffectedPageInfo['kind']) => (
                    <Tag color={affectedKindColors[kind]}>{affectedKindLabels[kind]}</Tag>
                  ),
                },
                {
                  title: '页面标识',
                  dataIndex: 'pageKey',
                  key: 'pageKey',
                  render: (value: string) => <Text code>{value}</Text>,
                },
                {
                  title: '标题',
                  key: 'title',
                  render: (_, record) => pageTitleText(record),
                },
                {
                  title: '状态',
                  key: 'status',
                  render: (_, record) => (
                    <Space wrap>
                      {record.status ? <Tag>{record.status}</Tag> : null}
                      {record.proposalQuality ? (
                        <Tag color="processing">{record.proposalQuality}</Tag>
                      ) : null}
                      {record.stale ? <Tag color="error">stale</Tag> : null}
                    </Space>
                  ),
                },
                {
                  title: '版本',
                  key: 'version',
                  render: (_, record) => {
                    if (record.kind === 'draft') {
                      return record.draftRevision || '-';
                    }
                    if (record.kind === 'published') {
                      return record.publishedVersion || '-';
                    }
                    return '-';
                  },
                },
                {
                  title: 'Freshness',
                  key: 'freshness',
                  render: (_, record) => bindingFreshnessSummary(record),
                },
                {
                  title: '更新时间',
                  dataIndex: 'updatedAt',
                  key: 'updatedAt',
                  render: (value?: string) => (value ? new Date(value).toLocaleString() : '-'),
                },
              ]}
            />

            {selectedResource.semantics && (
              <>
                <Title level={5} style={{ marginTop: 16 }}>
                  <CheckCircleOutlined /> 语义信息
                </Title>
                <Descriptions column={2} bordered size="small">
                  <Descriptions.Item label="版本">
                    {selectedResource.semantics.version}
                  </Descriptions.Item>
                  <Descriptions.Item label="来源">
                    <Tag>{selectedResource.semantics.source}</Tag>
                  </Descriptions.Item>
                  <Descriptions.Item label="Identity">
                    {selectedResource.semantics.identityField ||
                      (selectedResource.semantics.hasIdentity ? '已配置' : '未配置')}
                  </Descriptions.Item>
                  <Descriptions.Item label="Identity 类型">
                    {selectedResource.semantics.identityFieldType || '-'}
                  </Descriptions.Item>
                  <Descriptions.Item label="Collection Query">
                    {selectedResource.semantics.collectionQueryId ||
                      (selectedResource.semantics.hasCollection ? '已配置' : '未配置')}
                  </Descriptions.Item>
                  <Descriptions.Item label="Collection 路径">
                    {selectedResource.semantics.collectionPath || '-'}
                  </Descriptions.Item>
                  <Descriptions.Item label="Items 字段">
                    {selectedResource.semantics.itemsFieldName || '-'}
                  </Descriptions.Item>
                  <Descriptions.Item label="Total 字段">
                    {selectedResource.semantics.totalFieldName || '-'}
                  </Descriptions.Item>
                  <Descriptions.Item label="Create">
                    {selectedResource.semantics.createId ||
                      (selectedResource.semantics.hasCreate ? '已配置' : '未配置')}
                  </Descriptions.Item>
                  <Descriptions.Item label="Update">
                    {selectedResource.semantics.updateId ||
                      (selectedResource.semantics.hasUpdate ? '已配置' : '未配置')}
                  </Descriptions.Item>
                  <Descriptions.Item label="Delete">
                    {selectedResource.semantics.deleteId ||
                      (selectedResource.semantics.hasDelete ? '已配置' : '未配置')}
                  </Descriptions.Item>
                  <Descriptions.Item label="Actions">
                    {selectedResource.semantics.actions?.length ? (
                      <Space wrap>
                        {selectedResource.semantics.actions.map((action) => (
                          <Tag
                            key={`${action.functionId}:${action.subject}:${action.identityInput || ''}`}
                          >
                            {action.functionId} / {action.subject}
                            {action.identityInput ? ` / ${action.identityInput}` : ''}
                          </Tag>
                        ))}
                      </Space>
                    ) : (
                      '未配置'
                    )}
                  </Descriptions.Item>
                  <Descriptions.Item label="Tasks">
                    {selectedResource.semantics.tasks?.length ? (
                      <Space direction="vertical" size={4}>
                        {selectedResource.semantics.tasks.map((task) => (
                          <Text code key={task.start.functionId}>
                            {task.start.functionId} / status: {task.status.function.functionId}
                            {task.cancel ? ` / cancel: ${task.cancel.function.functionId}` : ''}
                          </Text>
                        ))}
                      </Space>
                    ) : (
                      '未配置'
                    )}
                  </Descriptions.Item>
                  <Descriptions.Item label="Reports">
                    {selectedResource.semantics.reports?.length ? (
                      <Space direction="vertical" size={4}>
                        {selectedResource.semantics.reports.map((report) => (
                          <Text code key={report.query.functionId}>
                            {report.query.functionId} / dataset: {report.datasetPath || '(root)'} /
                            dims: {report.dimensions.join(',')} / metrics:{' '}
                            {report.metrics.join(',')}
                          </Text>
                        ))}
                      </Space>
                    ) : (
                      '未配置'
                    )}
                  </Descriptions.Item>
                </Descriptions>
              </>
            )}

            <Title level={5} style={{ marginTop: 16 }}>
              <BranchesOutlined /> 语义来源
            </Title>
            <Table<SemanticProvenanceInfo>
              dataSource={semanticMeta.provenance}
              rowKey="field"
              pagination={false}
              size="small"
              loading={detailLoading}
              locale={{ emptyText: '暂无字段级来源记录' }}
              columns={[
                { title: '字段', dataIndex: 'field', key: 'field' },
                {
                  title: '来源',
                  dataIndex: 'source',
                  key: 'source',
                  render: (source: SemanticSource) => (
                    <Tag color={sourceColors[source]}>{sourceLabels[source]}</Tag>
                  ),
                },
                { title: '置信度', dataIndex: 'confidence', key: 'confidence' },
                { title: '状态', dataIndex: 'status', key: 'status' },
                {
                  title: '值',
                  dataIndex: 'value',
                  key: 'value',
                  render: (value?: string) => <Text code>{displaySemanticValue(value)}</Text>,
                },
                { title: '更新人', dataIndex: 'updatedBy', key: 'updatedBy' },
              ]}
            />

            <Title level={5} style={{ marginTop: 16 }}>
              <BranchesOutlined /> 语义版本
              {semanticVersions.total > 0 ? `（共 ${semanticVersions.total} 条）` : ''}
            </Title>
            <Table<ResourceSemanticVersionInfo>
              dataSource={semanticVersions.items}
              rowKey="version"
              size="small"
              loading={detailLoading}
              locale={{ emptyText: '暂无语义版本记录' }}
              pagination={{
                current: versionPage,
                pageSize: versionPageSize,
                total: semanticVersions.total,
                showSizeChanger: true,
                pageSizeOptions: [5, 10, 20, 50],
                showTotal: (t) => `共 ${t} 条`,
                onChange: (page, pageSize) => {
                  if (!selectedResource) return;
                  setVersionPage(page);
                  setVersionPageSize(pageSize);
                  fetchSemanticVersions(selectedResource.resourceKey, page, pageSize);
                },
              }}
              columns={[
                { title: '版本', dataIndex: 'version', key: 'version', width: 90 },
                {
                  title: 'Source Digest',
                  dataIndex: 'sourceDigest',
                  key: 'sourceDigest',
                  render: (value?: string) =>
                    value ? <Text code>{value.slice(0, 12)}</Text> : '-',
                },
                { title: '变更原因', dataIndex: 'changeReason', key: 'changeReason' },
                {
                  title: '创建时间',
                  dataIndex: 'createdAt',
                  key: 'createdAt',
                  render: (value: string) => (value ? new Date(value).toLocaleString() : '-'),
                },
                { title: '创建人', dataIndex: 'createdBy', key: 'createdBy' },
              ]}
            />

            <Title level={5} style={{ marginTop: 16 }}>
              <WarningOutlined /> 语义冲突
            </Title>
            <Table<SemanticConflictInfo>
              dataSource={semanticMeta.conflicts}
              rowKey="field"
              pagination={false}
              size="small"
              loading={detailLoading}
              locale={{ emptyText: '暂无未解决冲突' }}
              columns={[
                { title: '字段', dataIndex: 'field', key: 'field' },
                {
                  title: '候选值',
                  key: 'values',
                  render: (_, record) => (
                    <Space wrap>
                      {conflictSources(record).map((source) => (
                        <Tag key={source} color={sourceColors[source]}>
                          {sourceLabels[source]}: {displaySemanticValue(record.values[source])}
                        </Tag>
                      ))}
                    </Space>
                  ),
                },
                {
                  title: '决议',
                  dataIndex: 'resolution',
                  key: 'resolution',
                  render: (source?: SemanticSource) =>
                    source ? (
                      <Tag color={sourceColors[source]}>{sourceLabels[source]}</Tag>
                    ) : (
                      <Tag color="error">未解决</Tag>
                    ),
                },
                {
                  title: '操作',
                  key: 'action',
                  render: (_, record) => (
                    <Button
                      size="small"
                      type="link"
                      disabled={Boolean(record.resolution)}
                      onClick={() => handleOpenResolve(record)}
                    >
                      选择来源
                    </Button>
                  ),
                },
              ]}
            />

            {selectedResource.diagnostics && selectedResource.diagnostics.length > 0 && (
              <>
                <Title level={5} style={{ marginTop: 16 }}>
                  <WarningOutlined /> 诊断信息
                </Title>
                <Table<DiagnosticInfo>
                  dataSource={selectedResource.diagnostics}
                  rowKey={(record, index) =>
                    `${record.code}-${record.functionId || ''}-${record.field || ''}-${index || 0}`
                  }
                  pagination={false}
                  size="small"
                  columns={[
                    { title: '代码', dataIndex: 'code', key: 'code' },
                    { title: '函数', dataIndex: 'functionId', key: 'functionId' },
                    { title: '字段', dataIndex: 'field', key: 'field' },
                    {
                      title: '级别',
                      dataIndex: 'severity',
                      key: 'severity',
                      render: (text: string) => (
                        <Tag
                          color={text === 'error' ? 'red' : text === 'warning' ? 'orange' : 'blue'}
                        >
                          {text}
                        </Tag>
                      ),
                    },
                    { title: '消息', dataIndex: 'message', key: 'message' },
                  ]}
                />
              </>
            )}
          </div>
        )}
      </Modal>

      <Modal
        title="编辑语义"
        open={editVisible}
        onOk={handleSaveSemantics}
        onCancel={() => setEditVisible(false)}
        okText="保存"
        cancelText="取消"
        width={760}
      >
        <Alert
          type="warning"
          showIcon
          style={{ marginBottom: 16 }}
          message="这里只补充能力语义，不编辑页面 UI"
          description="选择当前资源下的函数数据库 ID 作为 lifecycle binding；保存后会记录 platform_review 来源、创建语义版本，并触发相关 Proposal 重算。"
        />
        <Form form={editForm} layout="vertical">
          <Form.Item label="Identity 字段" name="identityField">
            <Input placeholder="例如: id, player_id" />
          </Form.Item>
          <Form.Item label="Identity 类型" name="identityFieldType">
            <Select
              options={[
                { value: 'string', label: 'string' },
                { value: 'number', label: 'number' },
                { value: 'integer', label: 'integer' },
                { value: 'boolean', label: 'boolean' },
              ]}
            />
          </Form.Item>
          <Form.Item label="Identity 路径" name="identityPath">
            <Input placeholder="例如: id 或 /data/id" />
          </Form.Item>
          <Form.Item label="Collection Query" name="collectionQueryId">
            {renderFunctionSelect('collection_query', '选择列表查询函数')}
          </Form.Item>
          <Form.Item label="Collection 路径" name="collectionPath">
            <Input placeholder="例如: /players" />
          </Form.Item>
          <Form.Item label="分页字段" name="pageFieldName">
            <Input placeholder="默认 page" />
          </Form.Item>
          <Form.Item label="分页大小字段" name="pageSizeFieldName">
            <Input placeholder="默认 page_size" />
          </Form.Item>
          <Form.Item label="Items 字段" name="itemsFieldName">
            <Input placeholder="默认 items" />
          </Form.Item>
          <Form.Item label="Total 字段" name="totalFieldName">
            <Input placeholder="默认 total" />
          </Form.Item>
          <Form.Item label="Item Query" name="itemQueryId">
            {renderFunctionSelect('item_query', '选择详情查询函数')}
          </Form.Item>
          <Form.Item label="Item 路径" name="itemPath">
            <Input placeholder="例如: /players/{player_id}" />
          </Form.Item>
          <Form.Item label="Create" name="createId">
            {renderFunctionSelect('create', '选择创建函数')}
          </Form.Item>
          <Form.Item label="Update" name="updateId">
            {renderFunctionSelect('update', '选择更新函数')}
          </Form.Item>
          <Form.Item label="Delete" name="deleteId">
            {renderFunctionSelect('delete', '选择删除函数')}
          </Form.Item>
          <Form.List name="actions">
            {(fields, { add, remove }) => (
              <Card
                size="small"
                title="资源动作语义"
                style={{ marginBottom: 16 }}
                extra={
                  <Button size="small" onClick={() => add({ subject: 'resource_item' })}>
                    添加动作
                  </Button>
                }
              >
                <Alert
                  type="info"
                  showIcon
                  style={{ marginBottom: 12 }}
                  message="这里只描述动作需要的资源上下文"
                  description="subject 决定动作针对单行、选中集合或整个资源；按钮位置由 PageProposal 生成器决定，不在这里配置。"
                />
                {fields.map((field) => (
                  <Space
                    key={field.key}
                    align="baseline"
                    wrap
                    style={{ display: 'flex', marginBottom: 8 }}
                  >
                    <Form.Item
                      {...field}
                      label="函数"
                      name={[field.name, 'functionId']}
                      rules={[{ required: true, message: '请选择 action 函数' }]}
                    >
                      <Select<string>
                        style={{ width: 260 }}
                        placeholder="选择 action 函数"
                        showSearch
                        optionFilterProp="label"
                        options={(selectedResource?.functions || [])
                          .filter((fn) => fn.capability === 'action')
                          .map((fn) => ({
                            value: fn.functionId,
                            label: `${fn.functionId} #${fn.id}`,
                            disabled: !fn.enabled,
                          }))}
                      />
                    </Form.Item>
                    <Form.Item
                      {...field}
                      label="Subject"
                      name={[field.name, 'subject']}
                      rules={[{ required: true, message: '请选择 subject' }]}
                    >
                      <Select
                        style={{ width: 180 }}
                        options={[
                          { value: 'resource_item', label: '单个资源对象' },
                          { value: 'resource_selection', label: '选中资源集合' },
                          { value: 'none', label: '整个资源' },
                        ]}
                      />
                    </Form.Item>
                    <Form.Item
                      {...field}
                      label="Identity Input"
                      name={[field.name, 'identityInput']}
                      tooltip="resource_item/resource_selection 必填，例如 /playerId 或 /playerIds；none 可留空"
                    >
                      <Input style={{ width: 200 }} placeholder="/playerId 或 /playerIds" />
                    </Form.Item>
                    <Button danger onClick={() => remove(field.name)}>
                      删除
                    </Button>
                  </Space>
                ))}
              </Card>
            )}
          </Form.List>
          <Form.List name="tasks">
            {(fields, { add, remove }) => (
              <Card
                size="small"
                title="任务语义"
                style={{ marginBottom: 16 }}
                extra={
                  <Button
                    size="small"
                    onClick={() =>
                      add({
                        taskId: { resultPath: '/taskId', valueType: 'string' },
                        status: { taskIdInput: '/taskId', statePath: '/status' },
                      })
                    }
                  >
                    添加任务
                  </Button>
                }
              >
                <Alert
                  type="info"
                  showIcon
                  style={{ marginBottom: 12 }}
                  message="这里只描述任务生命周期能力"
                  description="start 必须是 task 能力函数；status/events/result/cancel 只声明真实函数和 taskId 输入路径，不配置页面按钮位置。当前平台没有 retry runtime，因此不提供重试语义录入。"
                />
                {fields.map((field) => (
                  <Card key={field.key} size="small" style={{ marginBottom: 12 }}>
                    <Space align="baseline" wrap style={{ display: 'flex' }}>
                      <Form.Item
                        label="Start 函数"
                        name={[field.name, 'start', 'functionId']}
                        rules={[{ required: true, message: '请选择 task start 函数' }]}
                      >
                        {renderFunctionIdSelect('选择 task 函数', 'task', 300)}
                      </Form.Item>
                      <Form.Item
                        label="TaskID Result Path"
                        name={[field.name, 'taskId', 'resultPath']}
                        rules={[{ required: true, message: '请输入 taskId 输出路径' }]}
                      >
                        <Input style={{ width: 180 }} placeholder="/taskId 或空根路径" />
                      </Form.Item>
                      <Form.Item
                        label="TaskID 类型"
                        name={[field.name, 'taskId', 'valueType']}
                        rules={[{ required: true, message: '请选择 taskId 类型' }]}
                      >
                        <Select
                          style={{ width: 140 }}
                          options={[
                            { value: 'string', label: 'string' },
                            { value: 'number', label: 'number' },
                            { value: 'integer', label: 'integer' },
                            { value: 'boolean', label: 'boolean' },
                          ]}
                        />
                      </Form.Item>
                    </Space>
                    <Space align="baseline" wrap style={{ display: 'flex' }}>
                      <Form.Item
                        label="Status 函数"
                        name={[field.name, 'status', 'function', 'functionId']}
                        rules={[{ required: true, message: '请选择 status 函数' }]}
                      >
                        {renderFunctionIdSelect('选择 status 函数', undefined, 300)}
                      </Form.Item>
                      <Form.Item
                        label="Status TaskID Input"
                        name={[field.name, 'status', 'taskIdInput']}
                        rules={[{ required: true, message: '请输入 status taskId 输入路径' }]}
                      >
                        <Input style={{ width: 180 }} placeholder="/taskId" />
                      </Form.Item>
                      <Form.Item
                        label="State Path"
                        name={[field.name, 'status', 'statePath']}
                        rules={[{ required: true, message: '请输入状态输出路径' }]}
                      >
                        <Input style={{ width: 160 }} placeholder="/status" />
                      </Form.Item>
                    </Space>
                    <Space align="baseline" wrap style={{ display: 'flex' }}>
                      <Form.Item
                        label="Events 函数"
                        name={[field.name, 'events', 'function', 'functionId']}
                      >
                        {renderFunctionIdSelect('选择 events 函数')}
                      </Form.Item>
                      <Form.Item
                        label="Events TaskID Input"
                        name={[field.name, 'events', 'taskIdInput']}
                      >
                        <Input style={{ width: 160 }} placeholder="/taskId" />
                      </Form.Item>
                      <Form.Item label="Events Path" name={[field.name, 'events', 'eventsPath']}>
                        <Input style={{ width: 160 }} placeholder="/events" />
                      </Form.Item>
                    </Space>
                    <Space align="baseline" wrap style={{ display: 'flex' }}>
                      <Form.Item
                        label="Result 函数"
                        name={[field.name, 'result', 'function', 'functionId']}
                      >
                        {renderFunctionIdSelect('选择 result 函数')}
                      </Form.Item>
                      <Form.Item
                        label="Result TaskID Input"
                        name={[field.name, 'result', 'taskIdInput']}
                      >
                        <Input style={{ width: 160 }} placeholder="/taskId" />
                      </Form.Item>
                      <Form.Item label="Result Path" name={[field.name, 'result', 'resultPath']}>
                        <Input style={{ width: 160 }} placeholder="/result" />
                      </Form.Item>
                    </Space>
                    <Space align="baseline" wrap style={{ display: 'flex' }}>
                      <Form.Item
                        label="Cancel 函数"
                        name={[field.name, 'cancel', 'function', 'functionId']}
                      >
                        {renderFunctionIdSelect('选择 cancel 函数')}
                      </Form.Item>
                      <Form.Item
                        label="Cancel TaskID Input"
                        name={[field.name, 'cancel', 'taskIdInput']}
                      >
                        <Input style={{ width: 160 }} placeholder="/taskId" />
                      </Form.Item>
                    </Space>
                    <Button danger onClick={() => remove(field.name)}>
                      删除任务语义
                    </Button>
                  </Card>
                ))}
              </Card>
            )}
          </Form.List>
          <Form.List name="reports">
            {(fields, { add, remove }) => (
              <Card
                size="small"
                title="报表语义"
                style={{ marginBottom: 16 }}
                extra={
                  <Button
                    size="small"
                    onClick={() => add({ datasetPath: '/dataset', dimensions: [], metrics: [] })}
                  >
                    添加报表
                  </Button>
                }
              >
                <Alert
                  type="info"
                  showIcon
                  style={{ marginBottom: 12 }}
                  message="这里只描述报表数据集"
                  description="datasetPath 指向查询结果中的数组；dimensions/metrics 是相对 dataset item 的 JSON Pointer。图表类型和表格展示属于 Page Proposal/Page Studio。"
                />
                {fields.map((field) => (
                  <Space
                    key={field.key}
                    align="baseline"
                    wrap
                    style={{ display: 'flex', marginBottom: 8 }}
                  >
                    <Form.Item
                      label="Query 函数"
                      name={[field.name, 'query', 'functionId']}
                      rules={[{ required: true, message: '请选择 report 函数' }]}
                    >
                      {renderFunctionIdSelect('选择 report 函数', 'report', 300)}
                    </Form.Item>
                    <Form.Item
                      label="Dataset Path"
                      name={[field.name, 'datasetPath']}
                      tooltip="根数组可留空；对象字段数组示例 /dataset 或 /data/items"
                    >
                      <Input style={{ width: 180 }} placeholder="/dataset 或空根路径" />
                    </Form.Item>
                    <Form.Item
                      label="Dimensions"
                      name={[field.name, 'dimensions']}
                      rules={[{ required: true, message: '至少填写一个维度指针' }]}
                    >
                      <Select<string>
                        mode="tags"
                        style={{ width: 260 }}
                        tokenSeparators={[',']}
                        placeholder="/date, /channel"
                      />
                    </Form.Item>
                    <Form.Item
                      label="Metrics"
                      name={[field.name, 'metrics']}
                      rules={[{ required: true, message: '至少填写一个指标指针' }]}
                    >
                      <Select<string>
                        mode="tags"
                        style={{ width: 260 }}
                        tokenSeparators={[',']}
                        placeholder="/payAmount, /userCount"
                      />
                    </Form.Item>
                    <Button danger onClick={() => remove(field.name)}>
                      删除
                    </Button>
                  </Space>
                ))}
              </Card>
            )}
          </Form.List>
          <Form.Item label="变更原因" name="changeReason">
            <Input.TextArea placeholder="说明变更原因" />
          </Form.Item>
        </Form>
      </Modal>

      <Modal
        title="解决语义冲突"
        open={resolveVisible}
        onOk={handleResolveConflict}
        onCancel={() => setResolveVisible(false)}
        okText="确认选择"
        cancelText="取消"
      >
        {selectedConflict && (
          <Form form={resolveForm} layout="vertical">
            <Descriptions column={1} bordered size="small" style={{ marginBottom: 16 }}>
              <Descriptions.Item label="字段">{selectedConflict.field}</Descriptions.Item>
              <Descriptions.Item label="候选值">
                <Space wrap>
                  {conflictSources(selectedConflict).map((source) => (
                    <Tag key={source} color={sourceColors[source]}>
                      {sourceLabels[source]}:{' '}
                      {displaySemanticValue(selectedConflict.values[source])}
                    </Tag>
                  ))}
                </Space>
              </Descriptions.Item>
            </Descriptions>
            <Form.Item
              label="采用来源"
              name="chosenSource"
              rules={[{ required: true, message: '请选择采用的语义来源' }]}
            >
              <Select
                options={conflictSources(selectedConflict).map((source) => ({
                  value: source,
                  label: sourceLabels[source],
                }))}
              />
            </Form.Item>
            <Form.Item label="决议原因" name="reason">
              <Input.TextArea placeholder="说明为什么采用该来源" />
            </Form.Item>
          </Form>
        )}
      </Modal>
    </div>
  );
};

export default ResourceCatalogPage;
