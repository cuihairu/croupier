import React from 'react';
import { Alert, Button, Descriptions, Modal, Space, Table, Tag, Typography } from 'antd';
import {
  BranchesOutlined,
  BulbOutlined,
  CheckCircleOutlined,
  FunctionOutlined,
  WarningOutlined,
} from '@ant-design/icons';
import type {
  AffectedPageInfo,
  CapabilityKind,
  DiagnosticInfo,
  FunctionInfo,
  ResourceCatalogItem,
  ResourceSemanticConflicts,
  ResourceSemanticVersionInfo,
  ResourceSemanticVersions,
  SemanticConflictInfo,
  SemanticProvenanceInfo,
  SemanticSource,
} from '@/types/dashboard';
import { localizedText } from '@/utils/localizedText';
import {
  affectedKindColors,
  affectedKindLabels,
  bindingFreshnessSummary,
  capabilityLabels,
  conflictSources,
  displaySemanticValue,
  pageTitleText,
  riskColors,
  sourceColors,
  sourceLabels,
  statusColors,
  statusLabels,
} from './shared';

const { Text, Title } = Typography;

/** 资源详情弹窗：基本信息 + 函数列表 + 受影响页面 + 语义信息 + 来源/版本/冲突/诊断四组表。
 * 数据由主页拉取注入；冲突决议与提案跳转经回调上抛。 */
const ResourceDetailModal: React.FC<{
  open: boolean;
  resource: ResourceCatalogItem | null;
  semanticMeta: ResourceSemanticConflicts;
  semanticVersions: ResourceSemanticVersions;
  loading: boolean;
  versionPage: number;
  versionPageSize: number;
  onClose: () => void;
  onOpenProposals: (resourceKey: string) => void;
  onResolveConflict: (conflict: SemanticConflictInfo) => void;
  onVersionPageChange: (page: number, pageSize: number) => void;
}> = ({
  open,
  resource,
  semanticMeta,
  semanticVersions,
  loading,
  versionPage,
  versionPageSize,
  onClose,
  onOpenProposals,
  onResolveConflict,
  onVersionPageChange,
}) => (
  <Modal title="资源详情" open={open} onCancel={onClose} footer={null} width={1000}>
    {resource && (
      <div>
        <Alert
          type={resource.status === 'conflict' ? 'error' : 'info'}
          showIcon
          style={{ marginBottom: 16 }}
          message="Resource Catalog 只维护资源能力语义"
          description="这里补充 identity、collection 和生命周期函数绑定；页面标题、菜单、列、按钮位置属于 Page Proposal/Page Studio。"
        />

        <Descriptions column={2} bordered>
          <Descriptions.Item label="资源标识">{resource.resourceKey}</Descriptions.Item>
          <Descriptions.Item label="名称">
            {localizedText(resource.labels, 'zh-CN', '-')}
          </Descriptions.Item>
          <Descriptions.Item label="分类">{resource.categoryKey || '-'}</Descriptions.Item>
          <Descriptions.Item label="状态">
            <Tag color={statusColors[resource.status]}>{statusLabels[resource.status]}</Tag>
          </Descriptions.Item>
          <Descriptions.Item label="未解决冲突">
            {resource.semantics?.unresolvedConflicts ? (
              <Tag color="error">{resource.semantics.unresolvedConflicts}</Tag>
            ) : (
              <Tag color="success">0</Tag>
            )}
          </Descriptions.Item>
          <Descriptions.Item label="Proposal">
            <Button
              size="small"
              icon={<BulbOutlined />}
              onClick={() => onOpenProposals(resource.resourceKey)}
            >
              查看相关提案
            </Button>
          </Descriptions.Item>
        </Descriptions>

        <Title level={5} style={{ marginTop: 16 }}>
          <FunctionOutlined /> 函数列表
        </Title>
        <Table<FunctionInfo>
          dataSource={resource.functions}
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
          dataSource={resource.affectedPages || []}
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

        {resource.semantics && (
          <>
            <Title level={5} style={{ marginTop: 16 }}>
              <CheckCircleOutlined /> 语义信息
            </Title>
            <Descriptions column={2} bordered size="small">
              <Descriptions.Item label="版本">{resource.semantics.version}</Descriptions.Item>
              <Descriptions.Item label="来源">
                <Tag>{resource.semantics.source}</Tag>
              </Descriptions.Item>
              <Descriptions.Item label="Identity">
                {resource.semantics.identityField ||
                  (resource.semantics.hasIdentity ? '已配置' : '未配置')}
              </Descriptions.Item>
              <Descriptions.Item label="Identity 类型">
                {resource.semantics.identityFieldType || '-'}
              </Descriptions.Item>
              <Descriptions.Item label="Collection Query">
                {resource.semantics.collectionQueryId ||
                  (resource.semantics.hasCollection ? '已配置' : '未配置')}
              </Descriptions.Item>
              <Descriptions.Item label="Collection 路径">
                {resource.semantics.collectionPath || '-'}
              </Descriptions.Item>
              <Descriptions.Item label="Items 字段">
                {resource.semantics.itemsFieldName || '-'}
              </Descriptions.Item>
              <Descriptions.Item label="Total 字段">
                {resource.semantics.totalFieldName || '-'}
              </Descriptions.Item>
              <Descriptions.Item label="Create">
                {resource.semantics.createId ||
                  (resource.semantics.hasCreate ? '已配置' : '未配置')}
              </Descriptions.Item>
              <Descriptions.Item label="Update">
                {resource.semantics.updateId ||
                  (resource.semantics.hasUpdate ? '已配置' : '未配置')}
              </Descriptions.Item>
              <Descriptions.Item label="Delete">
                {resource.semantics.deleteId ||
                  (resource.semantics.hasDelete ? '已配置' : '未配置')}
              </Descriptions.Item>
              <Descriptions.Item label="Actions">
                {resource.semantics.actions?.length ? (
                  <Space wrap>
                    {resource.semantics.actions.map((action) => (
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
                {resource.semantics.tasks?.length ? (
                  <Space orientation="vertical" size={4}>
                    {resource.semantics.tasks.map((task) => (
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
                {resource.semantics.reports?.length ? (
                  <Space orientation="vertical" size={4}>
                    {resource.semantics.reports.map((report) => (
                      <Text code key={report.query.functionId}>
                        {report.query.functionId} / dataset: {report.datasetPath || '(root)'} /
                        dims: {report.dimensions.join(',')} / metrics: {report.metrics.join(',')}
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
          loading={loading}
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
          loading={loading}
          locale={{ emptyText: '暂无语义版本记录' }}
          pagination={{
            current: versionPage,
            pageSize: versionPageSize,
            total: semanticVersions.total,
            showSizeChanger: true,
            pageSizeOptions: [5, 10, 20, 50],
            showTotal: (t) => `共 ${t} 条`,
            onChange: (page, pageSize) => {
              onVersionPageChange(page, pageSize);
            },
          }}
          columns={[
            { title: '版本', dataIndex: 'version', key: 'version', width: 90 },
            {
              title: 'Source Digest',
              dataIndex: 'sourceDigest',
              key: 'sourceDigest',
              render: (value?: string) => (value ? <Text code>{value.slice(0, 12)}</Text> : '-'),
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
          loading={loading}
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
                  onClick={() => onResolveConflict(record)}
                >
                  选择来源
                </Button>
              ),
            },
          ]}
        />

        {resource.diagnostics && resource.diagnostics.length > 0 && (
          <>
            <Title level={5} style={{ marginTop: 16 }}>
              <WarningOutlined /> 诊断信息
            </Title>
            <Table<DiagnosticInfo>
              dataSource={resource.diagnostics}
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
                    <Tag color={text === 'error' ? 'red' : text === 'warning' ? 'orange' : 'blue'}>
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
);

export default ResourceDetailModal;
