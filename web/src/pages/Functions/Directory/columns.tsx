import React from 'react';
import type { ProColumns } from '@ant-design/pro-components';
import { Badge, Button, Space, Tag, Tooltip, Typography } from 'antd';
import { CodeOutlined, InfoCircleOutlined, PlayCircleOutlined } from '@ant-design/icons';
import type { DirectoryPageSchema } from './schema';
import type { SummaryRow } from './types';
import { localizedText } from '@/utils/localizedText';
import VersionFloorSelect from './VersionFloorSelect';

const { Text } = Typography;

/** 模块级文案助手接收 intl 的最小结构（@umijs/max 未导出 IntlShape 类型） */
type IntlFormatter = {
  formatMessage: (
    descriptor: { id: string; defaultMessage: string },
    values?: Record<string, string | number>,
  ) => string;
};

type BuildColumnsOptions = {
  intl: IntlFormatter;
  columns: DirectoryPageSchema['columns'];
  rowActions: DirectoryPageSchema['rowActions'];
  /** 当前数据集出现的全部函数版本（离散值，用于列过滤选项） */
  versions: string[];
  /** #26：functionId → 历史版本列表（门槛下拉选项，服务端聚合） */
  versionIndex: Record<string, string[]>;
  onFloorChange: (functionId: string, minVersion: string | undefined) => void;
  onOpenDetail: (record: SummaryRow) => void;
  onOpenSchema: (id: string) => void;
  onInvoke: (record: SummaryRow) => void;
  /** 点击「开放范围」摘要直达函数开放范围页 */
  onOpenAssignments: () => void;
};

const rowActionIcon = {
  info: <InfoCircleOutlined />,
  code: <CodeOutlined />,
  play: <PlayCircleOutlined />,
} as const;

export const buildDirectoryColumns = ({
  intl,
  columns,
  rowActions,
  versions,
  versionIndex,
  onFloorChange,
  onOpenDetail,
  onOpenSchema,
  onInvoke,
  onOpenAssignments,
}: BuildColumnsOptions): ProColumns<SummaryRow>[] =>
  columns.map((col) => {
    if (col.key === 'id') {
      return {
        title: col.title,
        dataIndex: 'id',
        width: col.width,
        copyable: col.copyable,
        ellipsis: true,
        render: (_, record) => (
          <Space>
            <Badge status={record.enabled ? 'success' : 'default'} />
            <Text code>{record.id}</Text>
          </Space>
        ),
      } as ProColumns<SummaryRow>;
    }
    if (col.key === 'version') {
      // 版本是独立且重要的信息：单独成列并支持按版本过滤（不再挤在函数 ID 后面）
      return {
        title: col.title,
        dataIndex: 'version',
        width: col.width,
        filters: versions.map((v) => ({ text: `v${v}`, value: v })),
        onFilter: (value, record) => record.version === value,
        render: (_, record) => (record.version ? <Tag color="blue">v{record.version}</Tag> : '-'),
      } as ProColumns<SummaryRow>;
    }
    if (col.key === 'minVersion') {
      // 函数级最低 SDK 版本门槛（#26）：行内下拉直接修改，选项只来自
      // 服务端函数历史版本索引——点击单元格即下拉选择，不再跳转设置面板
      // 手输。清空下拉 = 清除门槛（走 DELETE）。
      return {
        title: col.title,
        dataIndex: 'minVersion',
        width: col.width,
        render: (_, record) => (
          <VersionFloorSelect
            value={record.minVersion}
            options={versionIndex[record.id] ?? []}
            onChange={(next) => onFloorChange(record.id, next)}
          />
        ),
      } as ProColumns<SummaryRow>;
    }
    if (col.key === 'displayName') {
      return {
        title: col.title,
        dataIndex: 'displayName',
        width: col.width,
        ellipsis: true,
        render: (_, record) => localizedText(record.displayName, 'zh-CN', record.id),
      } as ProColumns<SummaryRow>;
    }
    if (col.key === 'summary') {
      return {
        title: col.title,
        dataIndex: 'summary',
        width: col.width,
        ellipsis: true,
        render: (_, record) => {
          const text = localizedText(record.summary, 'zh-CN', '');
          if (!text) return '-';
          const truncated = text.length > 50 ? text.slice(0, 50) + '...' : text;
          return (
            <Tooltip title={text}>
              <span>{truncated}</span>
            </Tooltip>
          );
        },
      } as ProColumns<SummaryRow>;
    }
    if (col.key === 'resource') {
      return {
        title: col.title,
        dataIndex: 'resource',
        width: col.width,
        filters: true,
        onFilter: (value, record) => record.resource === value,
        render: (_, record) => (
          <Tag color={record.resource ? 'geekblue' : 'default'}>
            {record.resource ||
              intl.formatMessage({
                id: 'pages.functionsDirectory.desc.undeclared',
                defaultMessage: '未声明',
              })}
          </Tag>
        ),
      } as ProColumns<SummaryRow>;
    }
    if (col.key === 'operation') {
      return {
        title: col.title,
        dataIndex: 'operation',
        width: col.width,
        render: (_, record) => (
          <Tag color={record.operation ? 'purple' : 'default'}>
            {record.operation ||
              intl.formatMessage({
                id: 'pages.functionsDirectory.desc.undeclared',
                defaultMessage: '未声明',
              })}
          </Tag>
        ),
      } as ProColumns<SummaryRow>;
    }
    if (col.key === 'tags') {
      return {
        title: col.title,
        dataIndex: 'tags',
        width: col.width,
        render: (_, record) => (
          <Space wrap>
            {(record.tags || []).slice(0, 3).map((tag) => (
              <Tag key={tag}>{tag}</Tag>
            ))}
            {(record.tags || []).length > 3 && <Tag>+{(record.tags || []).length - 3}</Tag>}
          </Space>
        ),
      } as ProColumns<SummaryRow>;
    }
    if (col.key === 'assignments') {
      // 开放范围摘要：open/total 环境白名单计数，点击直达「函数开放范围」页。
      // total=0 → 该游戏未配置白名单（默认开放）；undefined → 白名单拉取失败。
      return {
        title: col.title,
        dataIndex: 'assignmentScope',
        width: col.width,
        render: (_, record) => {
          const scope = record.assignmentScope;
          if (!scope) {
            return (
              <Tooltip
                title={intl.formatMessage({
                  id: 'pages.functionsDirectory.assignments.unknownTooltip',
                  defaultMessage: '开放范围数据拉取失败，请刷新重试',
                })}
              >
                <Text type="secondary">-</Text>
              </Tooltip>
            );
          }
          if (scope.total === 0) {
            return (
              <Tooltip
                title={intl.formatMessage({
                  id: 'pages.functionsDirectory.assignments.tooltip',
                  defaultMessage:
                    '该游戏尚未保存过环境白名单：默认开放全部函数。点击进入「函数开放范围」配置。',
                })}
              >
                <Button type="link" size="small" onClick={onOpenAssignments}>
                  {intl.formatMessage({
                    id: 'pages.functionsDirectory.assignments.defaultOpen',
                    defaultMessage: '默认开放',
                  })}
                </Button>
              </Tooltip>
            );
          }
          return (
            <Tooltip
              title={intl.formatMessage(
                {
                  id: 'pages.functionsDirectory.assignments.tooltipScoped',
                  defaultMessage:
                    '已保存白名单的 {total} 个环境中开放 {open} 个。统计口径：未保存白名单的环境默认开放全部函数。点击进入「函数开放范围」调整。',
                },
                { open: scope.open, total: scope.total },
              )}
            >
              <Button type="link" size="small" onClick={onOpenAssignments}>
                {scope.open === 0
                  ? intl.formatMessage({
                      id: 'pages.functionsDirectory.assignments.none',
                      defaultMessage: '未开放',
                    })
                  : intl.formatMessage(
                      {
                        id: 'pages.functionsDirectory.assignments.openScopes',
                        defaultMessage: '开放 {open}/{total} 环境',
                      },
                      { open: scope.open, total: scope.total },
                    )}
              </Button>
            </Tooltip>
          );
        },
      } as ProColumns<SummaryRow>;
    }
    if (col.key === 'enabled') {
      return {
        title: col.title,
        dataIndex: 'enabled',
        width: col.width,
        filters: [
          {
            text: intl.formatMessage({
              id: 'pages.functionsDirectory.state.enabled',
              defaultMessage: '启用',
            }),
            value: true,
          },
          {
            text: intl.formatMessage({
              id: 'pages.functionsDirectory.state.disabled',
              defaultMessage: '禁用',
            }),
            value: false,
          },
        ],
        onFilter: (value, record) => record.enabled === value,
        render: (_, record) => (
          <Badge
            status={record.enabled ? 'success' : 'default'}
            text={
              record.enabled
                ? intl.formatMessage({
                    id: 'pages.functionsDirectory.state.enabled',
                    defaultMessage: '启用',
                  })
                : intl.formatMessage({
                    id: 'pages.functionsDirectory.state.disabled',
                    defaultMessage: '禁用',
                  })
            }
          />
        ),
      } as ProColumns<SummaryRow>;
    }
    return {
      title: col.title,
      valueType: 'option',
      width: col.width,
      fixed: 'right',
      render: (_, record) =>
        rowActions.map((action) => (
          <Tooltip key={`${record.id}-${action.key}`} title={action.tooltip}>
            <Button
              type="link"
              size="small"
              icon={rowActionIcon[action.icon]}
              onClick={() => {
                if (action.key === 'detail') return onOpenDetail(record);
                if (action.key === 'schema') return onOpenSchema(record.id);
                return onInvoke(record);
              }}
            />
          </Tooltip>
        )),
    } as ProColumns<SummaryRow>;
  });
