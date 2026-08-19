import React from 'react';
import type { ProColumns } from '@ant-design/pro-components';
import { Badge, Button, Space, Tag, Tooltip, Typography } from 'antd';
import { CodeOutlined, InfoCircleOutlined, PlayCircleOutlined } from '@ant-design/icons';
import type { DirectoryPageSchema } from './schema';
import type { SummaryRow } from './types';
import { localizedText } from '@/utils/localizedText';

const { Text } = Typography;

type BuildColumnsOptions = {
  columns: DirectoryPageSchema['columns'];
  rowActions: DirectoryPageSchema['rowActions'];
  onOpenDetail: (record: SummaryRow) => void;
  onOpenSchema: (id: string) => void;
  onInvoke: (record: SummaryRow) => void;
};

const rowActionIcon = {
  info: <InfoCircleOutlined />,
  code: <CodeOutlined />,
  play: <PlayCircleOutlined />,
} as const;

export const buildDirectoryColumns = ({
  columns,
  rowActions,
  onOpenDetail,
  onOpenSchema,
  onInvoke,
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
            {record.version && <Tag color="blue">v{record.version}</Tag>}
          </Space>
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
          <Tag color={record.resource ? 'geekblue' : 'default'}>{record.resource || '未声明'}</Tag>
        ),
      } as ProColumns<SummaryRow>;
    }
    if (col.key === 'operation') {
      return {
        title: col.title,
        dataIndex: 'operation',
        width: col.width,
        render: (_, record) => (
          <Tag color={record.operation ? 'purple' : 'default'}>{record.operation || '未声明'}</Tag>
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
    if (col.key === 'enabled') {
      return {
        title: col.title,
        dataIndex: 'enabled',
        width: col.width,
        filters: [
          { text: '启用', value: true },
          { text: '禁用', value: false },
        ],
        onFilter: (value, record) => record.enabled === value,
        render: (_, record) => (
          <Badge
            status={record.enabled ? 'success' : 'default'}
            text={record.enabled ? '启用' : '禁用'}
          />
        ),
      } as ProColumns<SummaryRow>;
    }
    return {
      title: col.title,
      valueType: 'option',
      width: col.width,
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
