import React from 'react';
import type { ProColumns } from '@ant-design/pro-components';
import { Badge, Button, Space, Tag, Tooltip, Typography } from 'antd';
import { InfoCircleOutlined, PlayCircleOutlined, SettingOutlined } from '@ant-design/icons';
import type { DirectoryPageSchema } from './schema';
import type { SummaryRow } from './types';

const { Text } = Typography;

type BuildColumnsOptions = {
  columns: DirectoryPageSchema['columns'];
  rowActions: DirectoryPageSchema['rowActions'];
  onOpenDetail: (record: SummaryRow) => void;
  onOpenUI: (id: string) => void;
  onInvoke: (record: SummaryRow) => void;
};

const rowActionIcon = {
  info: <InfoCircleOutlined />,
  setting: <SettingOutlined />,
  play: <PlayCircleOutlined />,
} as const;

export const buildDirectoryColumns = ({
  columns,
  rowActions,
  onOpenDetail,
  onOpenUI,
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
        render: (_, record) => record.displayName?.zh || record.displayName?.en || record.id,
      } as ProColumns<SummaryRow>;
    }
    if (col.key === 'summary') {
      return {
        title: col.title,
        dataIndex: 'summary',
        width: col.width,
        ellipsis: true,
        render: (_, record) => record.summary?.zh || record.summary?.en || '-',
      } as ProColumns<SummaryRow>;
    }
    if (col.key === 'category') {
      return {
        title: col.title,
        dataIndex: 'category',
        width: col.width,
        filters: true,
        onFilter: (value, record) => record.category === value,
        render: (_, record) => (
          <Tag color={record.category ? 'geekblue' : 'default'}>{record.category || '未分类'}</Tag>
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
                if (action.key === 'ui') return onOpenUI(record.id);
                return onInvoke(record);
              }}
            />
          </Tooltip>
        )),
    } as ProColumns<SummaryRow>;
  });
