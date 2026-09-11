import React from 'react';
import { Alert, Button, Descriptions, Modal, Space, Table, Tag, Typography } from 'antd';
import {
  BranchesOutlined,
  BulbOutlined,
  CheckCircleOutlined,
  FunctionOutlined,
  WarningOutlined,
} from '@ant-design/icons';
import { FormattedMessage, useIntl } from '@umijs/max';
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
import { formatDateTime } from '@/utils/format';
import {
  affectedKindColors,
  affectedKindLabels,
  bindingFreshnessSummary,
  capabilityLabels,
  conflictSources,
  displaySemanticValue,
  formatLabelText,
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
}) => {
  const intl = useIntl();

  return (
    <Modal
      title={intl.formatMessage({
        id: 'pages.resourceCatalog.detail.title',
        defaultMessage: '资源详情',
      })}
      open={open}
      onCancel={onClose}
      footer={null}
      width={1000}
    >
      {resource && (
        <div>
          <Alert
            type={resource.status === 'conflict' ? 'error' : 'info'}
            showIcon
            style={{ marginBottom: 16 }}
            message={intl.formatMessage({
              id: 'pages.resourceCatalog.detail.alert.message',
              defaultMessage: 'Resource Catalog 只维护资源能力语义',
            })}
            description={intl.formatMessage({
              id: 'pages.resourceCatalog.detail.alert.description',
              defaultMessage:
                '这里补充 identity、collection 和生命周期函数绑定；页面标题、菜单、列、按钮位置属于 Page Proposal/Page Studio。',
            })}
          />

          <Descriptions column={2} bordered>
            <Descriptions.Item
              label={intl.formatMessage({
                id: 'pages.resourceCatalog.detail.desc.resourceKey',
                defaultMessage: '资源标识',
              })}
            >
              {resource.resourceKey}
            </Descriptions.Item>
            <Descriptions.Item
              label={intl.formatMessage({
                id: 'pages.resourceCatalog.detail.desc.name',
                defaultMessage: '名称',
              })}
            >
              {localizedText(resource.labels, 'zh-CN', '-')}
            </Descriptions.Item>
            <Descriptions.Item
              label={intl.formatMessage({
                id: 'pages.resourceCatalog.detail.desc.category',
                defaultMessage: '分类',
              })}
            >
              {resource.categoryKey || '-'}
            </Descriptions.Item>
            <Descriptions.Item
              label={intl.formatMessage({
                id: 'pages.resourceCatalog.detail.desc.status',
                defaultMessage: '状态',
              })}
            >
              <Tag color={statusColors[resource.status]}>
                {formatLabelText(intl, statusLabels[resource.status])}
              </Tag>
            </Descriptions.Item>
            <Descriptions.Item
              label={intl.formatMessage({
                id: 'pages.resourceCatalog.detail.desc.conflicts',
                defaultMessage: '未解决冲突',
              })}
            >
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
                <FormattedMessage
                  id="pages.resourceCatalog.detail.button.viewProposals"
                  defaultMessage="查看相关提案"
                />
              </Button>
            </Descriptions.Item>
          </Descriptions>

          <Title level={5} style={{ marginTop: 16 }}>
            <FunctionOutlined />{' '}
            <FormattedMessage
              id="pages.resourceCatalog.detail.section.functions.title"
              defaultMessage="函数列表"
            />
          </Title>
          <Table<FunctionInfo>
            dataSource={resource.functions}
            rowKey="id"
            pagination={false}
            size="small"
            columns={[
              {
                title: intl.formatMessage({
                  id: 'pages.resourceCatalog.detail.column.functionId',
                  defaultMessage: '函数 ID',
                }),
                dataIndex: 'functionId',
                key: 'functionId',
              },
              { title: 'DB ID', dataIndex: 'id', key: 'id', width: 80 },
              {
                title: intl.formatMessage({
                  id: 'pages.resourceCatalog.detail.column.version',
                  defaultMessage: '版本',
                }),
                dataIndex: 'version',
                key: 'version',
              },
              {
                title: intl.formatMessage({
                  id: 'pages.resourceCatalog.detail.column.capability',
                  defaultMessage: '能力',
                }),
                dataIndex: 'capability',
                key: 'capability',
                render: (text: CapabilityKind) => (
                  <Tag>{formatLabelText(intl, capabilityLabels[text]) || text}</Tag>
                ),
              },
              {
                title: intl.formatMessage({
                  id: 'pages.resourceCatalog.detail.column.execution',
                  defaultMessage: '执行方式',
                }),
                dataIndex: 'execution',
                key: 'execution',
                render: (text: string) => <Tag>{text}</Tag>,
              },
              {
                title: intl.formatMessage({
                  id: 'pages.resourceCatalog.detail.column.risk',
                  defaultMessage: '风险',
                }),
                dataIndex: 'risk',
                key: 'risk',
                render: (text: string) => <Tag color={riskColors[text] || 'default'}>{text}</Tag>,
              },
              {
                title: intl.formatMessage({
                  id: 'pages.resourceCatalog.detail.column.status',
                  defaultMessage: '状态',
                }),
                dataIndex: 'enabled',
                key: 'enabled',
                render: (enabled: boolean) => (
                  <Tag color={enabled ? 'success' : 'default'}>
                    {enabled
                      ? intl.formatMessage({
                          id: 'pages.resourceCatalog.detail.tag.enabled',
                          defaultMessage: '启用',
                        })
                      : intl.formatMessage({
                          id: 'pages.resourceCatalog.detail.tag.disabled',
                          defaultMessage: '禁用',
                        })}
                  </Tag>
                ),
              },
            ]}
          />

          <Title level={5} style={{ marginTop: 16 }}>
            <BulbOutlined />{' '}
            <FormattedMessage
              id="pages.resourceCatalog.detail.section.affectedPages.title"
              defaultMessage="受影响页面"
            />
          </Title>
          <Table<AffectedPageInfo>
            dataSource={resource.affectedPages || []}
            rowKey={(record) => `${record.kind}:${record.proposalKey || record.pageKey}`}
            pagination={false}
            size="small"
            locale={{
              emptyText: intl.formatMessage({
                id: 'pages.resourceCatalog.detail.section.affectedPages.empty',
                defaultMessage: '当前资源还没有草稿、已发布页面或提案',
              }),
            }}
            columns={[
              {
                title: intl.formatMessage({
                  id: 'pages.resourceCatalog.detail.column.kind',
                  defaultMessage: '类型',
                }),
                dataIndex: 'kind',
                key: 'kind',
                width: 100,
                render: (kind: AffectedPageInfo['kind']) => (
                  <Tag color={affectedKindColors[kind]}>
                    {formatLabelText(intl, affectedKindLabels[kind])}
                  </Tag>
                ),
              },
              {
                title: intl.formatMessage({
                  id: 'pages.resourceCatalog.detail.column.pageKey',
                  defaultMessage: '页面标识',
                }),
                dataIndex: 'pageKey',
                key: 'pageKey',
                render: (value: string) => <Text code>{value}</Text>,
              },
              {
                title: intl.formatMessage({
                  id: 'pages.resourceCatalog.detail.column.title',
                  defaultMessage: '标题',
                }),
                key: 'title',
                render: (_, record) => pageTitleText(record),
              },
              {
                title: intl.formatMessage({
                  id: 'pages.resourceCatalog.detail.column.status',
                  defaultMessage: '状态',
                }),
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
                title: intl.formatMessage({
                  id: 'pages.resourceCatalog.detail.column.version',
                  defaultMessage: '版本',
                }),
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
                render: (_, record) => bindingFreshnessSummary(intl, record),
              },
              {
                title: intl.formatMessage({
                  id: 'pages.resourceCatalog.detail.column.updatedAt',
                  defaultMessage: '更新时间',
                }),
                dataIndex: 'updatedAt',
                key: 'updatedAt',
                render: (value?: string) => formatDateTime(value ?? ''),
              },
            ]}
          />

          {resource.semantics && (
            <>
              <Title level={5} style={{ marginTop: 16 }}>
                <CheckCircleOutlined />{' '}
                <FormattedMessage
                  id="pages.resourceCatalog.detail.section.semantics.title"
                  defaultMessage="语义信息"
                />
              </Title>
              <Descriptions column={2} bordered size="small">
                <Descriptions.Item
                  label={intl.formatMessage({
                    id: 'pages.resourceCatalog.detail.desc.version',
                    defaultMessage: '版本',
                  })}
                >
                  {resource.semantics.version}
                </Descriptions.Item>
                <Descriptions.Item
                  label={intl.formatMessage({
                    id: 'pages.resourceCatalog.detail.desc.source',
                    defaultMessage: '来源',
                  })}
                >
                  <Tag>{resource.semantics.source}</Tag>
                </Descriptions.Item>
                <Descriptions.Item label="Identity">
                  {resource.semantics.identityField ||
                    (resource.semantics.hasIdentity
                      ? intl.formatMessage({
                          id: 'pages.resourceCatalog.detail.tag.configured',
                          defaultMessage: '已配置',
                        })
                      : intl.formatMessage({
                          id: 'pages.resourceCatalog.detail.tag.notConfigured',
                          defaultMessage: '未配置',
                        }))}
                </Descriptions.Item>
                <Descriptions.Item
                  label={intl.formatMessage({
                    id: 'pages.resourceCatalog.detail.desc.identityType',
                    defaultMessage: 'Identity 类型',
                  })}
                >
                  {resource.semantics.identityFieldType || '-'}
                </Descriptions.Item>
                <Descriptions.Item label="Collection Query">
                  {resource.semantics.collectionQueryId ||
                    (resource.semantics.hasCollection
                      ? intl.formatMessage({
                          id: 'pages.resourceCatalog.detail.tag.configured',
                          defaultMessage: '已配置',
                        })
                      : intl.formatMessage({
                          id: 'pages.resourceCatalog.detail.tag.notConfigured',
                          defaultMessage: '未配置',
                        }))}
                </Descriptions.Item>
                <Descriptions.Item
                  label={intl.formatMessage({
                    id: 'pages.resourceCatalog.detail.desc.collectionPath',
                    defaultMessage: 'Collection 路径',
                  })}
                >
                  {resource.semantics.collectionPath || '-'}
                </Descriptions.Item>
                <Descriptions.Item
                  label={intl.formatMessage({
                    id: 'pages.resourceCatalog.detail.desc.itemsField',
                    defaultMessage: 'Items 字段',
                  })}
                >
                  {resource.semantics.itemsFieldName || '-'}
                </Descriptions.Item>
                <Descriptions.Item
                  label={intl.formatMessage({
                    id: 'pages.resourceCatalog.detail.desc.totalField',
                    defaultMessage: 'Total 字段',
                  })}
                >
                  {resource.semantics.totalFieldName || '-'}
                </Descriptions.Item>
                <Descriptions.Item label="Create">
                  {resource.semantics.createId ||
                    (resource.semantics.hasCreate
                      ? intl.formatMessage({
                          id: 'pages.resourceCatalog.detail.tag.configured',
                          defaultMessage: '已配置',
                        })
                      : intl.formatMessage({
                          id: 'pages.resourceCatalog.detail.tag.notConfigured',
                          defaultMessage: '未配置',
                        }))}
                </Descriptions.Item>
                <Descriptions.Item label="Update">
                  {resource.semantics.updateId ||
                    (resource.semantics.hasUpdate
                      ? intl.formatMessage({
                          id: 'pages.resourceCatalog.detail.tag.configured',
                          defaultMessage: '已配置',
                        })
                      : intl.formatMessage({
                          id: 'pages.resourceCatalog.detail.tag.notConfigured',
                          defaultMessage: '未配置',
                        }))}
                </Descriptions.Item>
                <Descriptions.Item label="Delete">
                  {resource.semantics.deleteId ||
                    (resource.semantics.hasDelete
                      ? intl.formatMessage({
                          id: 'pages.resourceCatalog.detail.tag.configured',
                          defaultMessage: '已配置',
                        })
                      : intl.formatMessage({
                          id: 'pages.resourceCatalog.detail.tag.notConfigured',
                          defaultMessage: '未配置',
                        }))}
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
                    intl.formatMessage({
                      id: 'pages.resourceCatalog.detail.tag.notConfigured',
                      defaultMessage: '未配置',
                    })
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
                    intl.formatMessage({
                      id: 'pages.resourceCatalog.detail.tag.notConfigured',
                      defaultMessage: '未配置',
                    })
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
                    intl.formatMessage({
                      id: 'pages.resourceCatalog.detail.tag.notConfigured',
                      defaultMessage: '未配置',
                    })
                  )}
                </Descriptions.Item>
              </Descriptions>
            </>
          )}

          <Title level={5} style={{ marginTop: 16 }}>
            <BranchesOutlined />{' '}
            <FormattedMessage
              id="pages.resourceCatalog.detail.section.provenance.title"
              defaultMessage="语义来源"
            />
          </Title>
          <Table<SemanticProvenanceInfo>
            dataSource={semanticMeta.provenance}
            rowKey="field"
            pagination={false}
            size="small"
            loading={loading}
            locale={{
              emptyText: intl.formatMessage({
                id: 'pages.resourceCatalog.detail.section.provenance.empty',
                defaultMessage: '暂无字段级来源记录',
              }),
            }}
            columns={[
              {
                title: intl.formatMessage({
                  id: 'pages.resourceCatalog.detail.column.field',
                  defaultMessage: '字段',
                }),
                dataIndex: 'field',
                key: 'field',
              },
              {
                title: intl.formatMessage({
                  id: 'pages.resourceCatalog.detail.column.source',
                  defaultMessage: '来源',
                }),
                dataIndex: 'source',
                key: 'source',
                render: (source: SemanticSource) => (
                  <Tag color={sourceColors[source]}>
                    {formatLabelText(intl, sourceLabels[source])}
                  </Tag>
                ),
              },
              {
                title: intl.formatMessage({
                  id: 'pages.resourceCatalog.detail.column.confidence',
                  defaultMessage: '置信度',
                }),
                dataIndex: 'confidence',
                key: 'confidence',
              },
              {
                title: intl.formatMessage({
                  id: 'pages.resourceCatalog.detail.column.status',
                  defaultMessage: '状态',
                }),
                dataIndex: 'status',
                key: 'status',
              },
              {
                title: intl.formatMessage({
                  id: 'pages.resourceCatalog.detail.column.value',
                  defaultMessage: '值',
                }),
                dataIndex: 'value',
                key: 'value',
                render: (value?: string) => <Text code>{displaySemanticValue(value)}</Text>,
              },
              {
                title: intl.formatMessage({
                  id: 'pages.resourceCatalog.detail.column.updatedBy',
                  defaultMessage: '更新人',
                }),
                dataIndex: 'updatedBy',
                key: 'updatedBy',
              },
            ]}
          />

          <Title level={5} style={{ marginTop: 16 }}>
            <BranchesOutlined />{' '}
            <FormattedMessage
              id="pages.resourceCatalog.detail.section.versions.title"
              defaultMessage="语义版本"
            />
            {semanticVersions.total > 0
              ? intl.formatMessage(
                  {
                    id: 'pages.resourceCatalog.detail.section.versions.total',
                    defaultMessage: '（共 {total} 条）',
                  },
                  { total: semanticVersions.total },
                )
              : ''}
          </Title>
          <Table<ResourceSemanticVersionInfo>
            dataSource={semanticVersions.items}
            rowKey="version"
            size="small"
            loading={loading}
            locale={{
              emptyText: intl.formatMessage({
                id: 'pages.resourceCatalog.detail.section.versions.empty',
                defaultMessage: '暂无语义版本记录',
              }),
            }}
            pagination={{
              current: versionPage,
              pageSize: versionPageSize,
              total: semanticVersions.total,
              showSizeChanger: true,
              pageSizeOptions: [5, 10, 20, 50],
              showTotal: (t) =>
                intl.formatMessage(
                  {
                    id: 'pages.resourceCatalog.detail.pagination.total',
                    defaultMessage: '共 {total} 条',
                  },
                  { total: t },
                ),
              onChange: (page, pageSize) => {
                onVersionPageChange(page, pageSize);
              },
            }}
            columns={[
              {
                title: intl.formatMessage({
                  id: 'pages.resourceCatalog.detail.column.version',
                  defaultMessage: '版本',
                }),
                dataIndex: 'version',
                key: 'version',
                width: 90,
              },
              {
                title: 'Source Digest',
                dataIndex: 'sourceDigest',
                key: 'sourceDigest',
                render: (value?: string) => (value ? <Text code>{value.slice(0, 12)}</Text> : '-'),
              },
              {
                title: intl.formatMessage({
                  id: 'pages.resourceCatalog.detail.column.changeReason',
                  defaultMessage: '变更原因',
                }),
                dataIndex: 'changeReason',
                key: 'changeReason',
              },
              {
                title: intl.formatMessage({
                  id: 'pages.resourceCatalog.detail.column.createdAt',
                  defaultMessage: '创建时间',
                }),
                dataIndex: 'createdAt',
                key: 'createdAt',
                render: (value: string) => formatDateTime(value ?? ''),
              },
              {
                title: intl.formatMessage({
                  id: 'pages.resourceCatalog.detail.column.createdBy',
                  defaultMessage: '创建人',
                }),
                dataIndex: 'createdBy',
                key: 'createdBy',
              },
            ]}
          />

          <Title level={5} style={{ marginTop: 16 }}>
            <WarningOutlined />{' '}
            <FormattedMessage
              id="pages.resourceCatalog.detail.section.conflicts.title"
              defaultMessage="语义冲突"
            />
          </Title>
          <Table<SemanticConflictInfo>
            dataSource={semanticMeta.conflicts}
            rowKey="field"
            pagination={false}
            size="small"
            loading={loading}
            locale={{
              emptyText: intl.formatMessage({
                id: 'pages.resourceCatalog.detail.section.conflicts.empty',
                defaultMessage: '暂无未解决冲突',
              }),
            }}
            columns={[
              {
                title: intl.formatMessage({
                  id: 'pages.resourceCatalog.detail.column.field',
                  defaultMessage: '字段',
                }),
                dataIndex: 'field',
                key: 'field',
              },
              {
                title: intl.formatMessage({
                  id: 'pages.resourceCatalog.detail.column.candidateValues',
                  defaultMessage: '候选值',
                }),
                key: 'values',
                render: (_, record) => (
                  <Space wrap>
                    {conflictSources(record).map((source) => (
                      <Tag key={source} color={sourceColors[source]}>
                        {formatLabelText(intl, sourceLabels[source])}:{' '}
                        {displaySemanticValue(record.values[source])}
                      </Tag>
                    ))}
                  </Space>
                ),
              },
              {
                title: intl.formatMessage({
                  id: 'pages.resourceCatalog.detail.column.resolution',
                  defaultMessage: '决议',
                }),
                dataIndex: 'resolution',
                key: 'resolution',
                render: (source?: SemanticSource) =>
                  source ? (
                    <Tag color={sourceColors[source]}>
                      {formatLabelText(intl, sourceLabels[source])}
                    </Tag>
                  ) : (
                    <Tag color="error">
                      <FormattedMessage
                        id="pages.resourceCatalog.detail.tag.unresolved"
                        defaultMessage="未解决"
                      />
                    </Tag>
                  ),
              },
              {
                title: intl.formatMessage({
                  id: 'pages.resourceCatalog.detail.column.actions',
                  defaultMessage: '操作',
                }),
                key: 'action',
                render: (_, record) => (
                  <Button
                    size="small"
                    type="link"
                    disabled={Boolean(record.resolution)}
                    onClick={() => onResolveConflict(record)}
                  >
                    <FormattedMessage
                      id="pages.resourceCatalog.detail.button.chooseSource"
                      defaultMessage="选择来源"
                    />
                  </Button>
                ),
              },
            ]}
          />

          {resource.diagnostics && resource.diagnostics.length > 0 && (
            <>
              <Title level={5} style={{ marginTop: 16 }}>
                <WarningOutlined />{' '}
                <FormattedMessage
                  id="pages.resourceCatalog.detail.section.diagnostics.title"
                  defaultMessage="诊断信息"
                />
              </Title>
              <Table<DiagnosticInfo>
                dataSource={resource.diagnostics}
                rowKey={(record, index) =>
                  `${record.code}-${record.functionId || ''}-${record.field || ''}-${index || 0}`
                }
                pagination={false}
                size="small"
                columns={[
                  {
                    title: intl.formatMessage({
                      id: 'pages.resourceCatalog.detail.column.code',
                      defaultMessage: '代码',
                    }),
                    dataIndex: 'code',
                    key: 'code',
                  },
                  {
                    title: intl.formatMessage({
                      id: 'pages.resourceCatalog.detail.column.function',
                      defaultMessage: '函数',
                    }),
                    dataIndex: 'functionId',
                    key: 'functionId',
                  },
                  {
                    title: intl.formatMessage({
                      id: 'pages.resourceCatalog.detail.column.field',
                      defaultMessage: '字段',
                    }),
                    dataIndex: 'field',
                    key: 'field',
                  },
                  {
                    title: intl.formatMessage({
                      id: 'pages.resourceCatalog.detail.column.severity',
                      defaultMessage: '级别',
                    }),
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
                  {
                    title: intl.formatMessage({
                      id: 'pages.resourceCatalog.detail.column.message',
                      defaultMessage: '消息',
                    }),
                    dataIndex: 'message',
                    key: 'message',
                  },
                ]}
              />
            </>
          )}
        </div>
      )}
    </Modal>
  );
};

export default ResourceDetailModal;
