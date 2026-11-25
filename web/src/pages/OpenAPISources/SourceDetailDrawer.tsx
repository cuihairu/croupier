import React from 'react';
import { ProTable, type ProColumns } from '@ant-design/pro-components';
import { Button, Card, Drawer, Popconfirm, Space, Tag, Typography } from 'antd';
import { DeleteOutlined, EditOutlined, LinkOutlined } from '@ant-design/icons';
import { FormattedMessage, useIntl } from '@umijs/max';
import type {
  OpenAPISourceBinding,
  OpenAPISourceDetail,
  OpenAPISourceOperation,
} from '@/services/api/openapi';
import type { Diagnostic } from '@/types/dashboard';
import DiagnosticsList from './DiagnosticsList';
import { capabilityColor, executionColor, operationLabel, riskColor } from './shared';

const { Text } = Typography;

/** SUI-011/SUI-026：请求兼容层级 → 颜色/本地化文案。manual=缺少请求映射
 * （无法自动映射的媒体类型/discriminator），具体原因列在标签下方。 */
const COMPATIBILITY_COLOR: Record<string, string> = {
  direct: 'green',
  degraded: 'orange',
  manual: 'red',
};

const COMPATIBILITY_LABEL: Record<string, { id: string; defaultMessage: string }> = {
  direct: { id: 'pages.openapiSources.drawer.compatibility.direct', defaultMessage: '直接' },
  degraded: {
    id: 'pages.openapiSources.drawer.compatibility.degraded',
    defaultMessage: '降级（JSON 编辑）',
  },
  manual: {
    id: 'pages.openapiSources.drawer.compatibility.manual',
    defaultMessage: '手动（无法自动映射）',
  },
};

function RequestCompatibilityCell({
  record,
  diagnostics,
}: {
  record: OpenAPISourceOperation;
  diagnostics: Diagnostic[];
}) {
  const intl = useIntl();
  const tier = record.requestCompatibility;
  if (!tier) return <Text type="secondary">-</Text>;
  const label = COMPATIBILITY_LABEL[tier]
    ? intl.formatMessage(COMPATIBILITY_LABEL[tier])
    : tier;
  const reasons = diagnostics.filter(
    (item) =>
      item.functionId === record.functionId &&
      typeof item.code === 'string' &&
      item.code.startsWith('openapi_schema_compatibility'),
  );
  return (
    <Space orientation="vertical" size={2}>
      <Tag color={COMPATIBILITY_COLOR[tier] ?? 'default'}>{label}</Tag>
      {reasons.slice(0, 3).map((item, index) => (
        <Text key={`${item.code}-${index}`} type="secondary" style={{ fontSize: 12 }}>
          {item.message}
        </Text>
      ))}
    </Space>
  );
}

/** Source 详情抽屉：revision/format 标签 + 诊断列表 + Operations 表 +
 * Provider Bindings 表 + 原始 JSON。数据由页面拉取注入。 */
export default function SourceDetailDrawer({
  detail,
  detailLoading,
  canWrite,
  onClose,
  onUpdateSource,
  onBindOperation,
  onRemoveBinding,
}: {
  detail: OpenAPISourceDetail | null;
  detailLoading: boolean;
  canWrite: boolean;
  onClose: () => void;
  onUpdateSource: (record: OpenAPISourceDetail) => void;
  onBindOperation: (operation: OpenAPISourceOperation) => void;
  onRemoveBinding: (binding: OpenAPISourceBinding) => void;
}) {
  const intl = useIntl();

  // SUI-026：manual（缺少请求映射）等层级的具体原因随 source diagnostics，
  // 按 functionId 关联到对应 operation 行展示。
  const diagnostics: Diagnostic[] = detail?.diagnostics ?? [];

  const operationColumns: ProColumns<OpenAPISourceOperation>[] = [
    {
      title: 'Operation',
      dataIndex: 'operationId',
      render: (_, record) => (
        <Space orientation="vertical" size={0}>
          <Text strong>{operationLabel(record)}</Text>
          <Text code>{record.operationId}</Text>
          <Text type="secondary">{`${record.method} ${record.path}`}</Text>
        </Space>
      ),
    },
    {
      title: intl.formatMessage({
        id: 'pages.openapiSources.drawer.column.capabilityContract',
        defaultMessage: '能力契约',
      }),
      dataIndex: 'resource',
      width: 320,
      render: (_, record) => (
        <Space orientation="vertical" size={4}>
          <Space size={4} wrap>
            <Tag color={record.resource ? 'blue' : undefined}>
              {record.resource ||
                intl.formatMessage({
                  id: 'pages.openapiSources.drawer.tag.noResource',
                  defaultMessage: '无 resource',
                })}
            </Tag>
            <Tag color={record.operation ? undefined : 'default'}>
              {record.operation ||
                intl.formatMessage({
                  id: 'pages.openapiSources.drawer.tag.noOperation',
                  defaultMessage: '无 operation',
                })}
            </Tag>
            <Tag color={capabilityColor(record.capability)}>
              {record.capability ||
                intl.formatMessage({
                  id: 'pages.openapiSources.drawer.tag.noCapability',
                  defaultMessage: '无 capability',
                })}
            </Tag>
            <Tag color={executionColor(record.execution)}>
              {record.execution ||
                intl.formatMessage({
                  id: 'pages.openapiSources.drawer.tag.noExecution',
                  defaultMessage: '无 execution',
                })}
            </Tag>
            <Tag color={record.approval?.required ? 'orange' : 'default'}>
              {record.approval?.required
                ? `approval:${record.approval.policyKey || 'required'}`
                : intl.formatMessage({
                    id: 'pages.openapiSources.drawer.tag.noApproval',
                    defaultMessage: '无 approval',
                  })}
            </Tag>
            <Tag color={riskColor(record.risk)}>
              {record.risk ||
                intl.formatMessage({
                  id: 'pages.openapiSources.drawer.tag.noRisk',
                  defaultMessage: '无 risk',
                })}
            </Tag>
          </Space>
          <Text code>
            {record.permission ||
              intl.formatMessage({
                id: 'pages.openapiSources.drawer.tag.noPermission',
                defaultMessage: '无 permission',
              })}
          </Text>
        </Space>
      ),
    },
    {
      title: intl.formatMessage({
        id: 'pages.openapiSources.drawer.column.requestCompatibility',
        defaultMessage: '请求兼容性',
      }),
      dataIndex: 'requestCompatibility',
      width: 150,
      render: (_, record) => (
        <RequestCompatibilityCell record={record} diagnostics={diagnostics} />
      ),
    },
    {
      title: 'Provider Binding',
      dataIndex: 'bound',
      width: 260,
      render: (_, record) => (
        <Space orientation="vertical" size={2}>
          <Tag color={record.bound ? 'green' : 'orange'}>{record.bound ? 'bound' : 'unbound'}</Tag>
          {record.bindingId ? <Text code>{record.bindingId}</Text> : null}
          {record.functionId ? <Text>{record.functionId}</Text> : null}
        </Space>
      ),
    },
    {
      title: intl.formatMessage({
        id: 'pages.openapiSources.column.actions',
        defaultMessage: '操作',
      }),
      valueType: 'option',
      width: 110,
      render: (_, record) => [
        canWrite ? (
          <Button
            key="bind"
            type="link"
            size="small"
            icon={<LinkOutlined />}
            onClick={() => onBindOperation(record)}
          >
            <FormattedMessage id="pages.openapiSources.drawer.button.bind" defaultMessage="绑定" />
          </Button>
        ) : (
          <Text key="readonly" type="secondary">
            <FormattedMessage
              id="pages.openapiSources.drawer.text.readonly"
              defaultMessage="只读"
            />
          </Text>
        ),
      ],
    },
  ];

  const bindingColumns: ProColumns<OpenAPISourceBinding>[] = [
    {
      title: 'bindingId',
      dataIndex: 'bindingId',
      render: (_, record) => <Text code>{record.bindingId}</Text>,
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
      title: intl.formatMessage({
        id: 'pages.openapiSources.column.actions',
        defaultMessage: '操作',
      }),
      valueType: 'option',
      width: 110,
      render: (_, record) => [
        canWrite ? (
          <Popconfirm
            key="delete"
            title={intl.formatMessage({
              id: 'pages.openapiSources.drawer.popconfirm.deleteBinding',
              defaultMessage: '删除此 binding？',
            })}
            onConfirm={() => onRemoveBinding(record)}
          >
            <Button type="link" danger size="small" icon={<DeleteOutlined />}>
              <FormattedMessage
                id="pages.openapiSources.drawer.button.delete"
                defaultMessage="删除"
              />
            </Button>
          </Popconfirm>
        ) : (
          <Text key="readonly" type="secondary">
            <FormattedMessage
              id="pages.openapiSources.drawer.text.readonly"
              defaultMessage="只读"
            />
          </Text>
        ),
      ],
    },
  ];

  return (
    <Drawer
      title={detail ? detail.name : 'OpenAPI Source'}
      open={!!detail}
      onClose={onClose}
      extra={
        detail && canWrite ? (
          <Button icon={<EditOutlined />} onClick={() => onUpdateSource(detail)}>
            <FormattedMessage
              id="pages.openapiSources.drawer.button.updateSource"
              defaultMessage="更新 Source"
            />
          </Button>
        ) : null
      }
      size="86vw"
      destroyOnHidden
    >
      {detail ? (
        <Space orientation="vertical" size={16} style={{ width: '100%' }}>
          <Card loading={detailLoading}>
            <Space wrap>
              <Tag>{`rev ${detail.revision}`}</Tag>
              <Tag>{detail.format}</Tag>
              <Tag>{detail.openapiVersion}</Tag>
              <Tag
                color={detail.diagnosticCount > 0 ? 'orange' : 'green'}
              >{`diagnostics ${detail.diagnosticCount}`}</Tag>
              <Text code>{detail.contentHash}</Text>
            </Space>
          </Card>
          <Card title="Diagnostics" loading={detailLoading}>
            {(detail.diagnostics || []).length === 0 ? (
              <Text type="secondary">
                <FormattedMessage
                  id="pages.openapiSources.drawer.empty.diagnostics"
                  defaultMessage="无诊断"
                />
              </Text>
            ) : (
              <DiagnosticsList items={detail.diagnostics || []} />
            )}
          </Card>
          <Card title="Operations" loading={detailLoading}>
            <ProTable<OpenAPISourceOperation>
              scroll={{ x: 'max-content' }}
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
              scroll={{ x: 'max-content' }}
              rowKey="bindingId"
              dataSource={detail.bindings || []}
              columns={bindingColumns}
              search={false}
              pagination={false}
              options={false}
            />
          </Card>
          <Card
            title={intl.formatMessage({
              id: 'pages.openapiSources.drawer.card.rawJson',
              defaultMessage: '原始 OpenAPI JSON',
            })}
          >
            <Typography.Paragraph copyable>
              <Text code>{JSON.stringify(detail.spec || {}, null, 2)}</Text>
            </Typography.Paragraph>
          </Card>
        </Space>
      ) : null}
    </Drawer>
  );
}
