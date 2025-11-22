import React, { useEffect, useState, useMemo } from 'react';
import { ProTable, ProColumns } from '@ant-design/pro-components';
import { Button, Space, Tag, Badge, Tooltip, Typography, Popconfirm } from 'antd';
import {
  PlayCircleOutlined,
  InfoCircleOutlined,
  EditOutlined,
  DeleteOutlined,
  ReloadOutlined,
  EyeOutlined,
  StopOutlined
} from '@ant-design/icons';
import { history } from '@umijs/max';

const { Text } = Typography;

export type FunctionItem = {
  id: string;
  version?: string;
  enabled?: boolean;
  display_name?: { zh?: string; en?: string };
  summary?: { zh?: string; en?: string };
  description?: { zh?: string; en?: string };
  tags?: string[];
  category?: string;
  author?: string;
  created_at?: string;
  updated_at?: string;
  menu?: {
    section?: string;
    group?: string;
    path?: string;
    order?: number;
    hidden?: boolean;
  };
  instances?: number;
  permissions?: string[];
};

export interface FunctionListTableProps {
  data: FunctionItem[];
  loading?: boolean;
  onRefresh?: () => void;
  onViewDetail?: (record: FunctionItem) => void;
  onInvoke?: (record: FunctionItem) => void;
  onEdit?: (record: FunctionItem) => void;
  onDelete?: (record: FunctionItem) => void;
  onToggleStatus?: (record: FunctionItem) => void;
  showActions?: {
    view?: boolean;
    invoke?: boolean;
    edit?: boolean;
    delete?: boolean;
    toggle?: boolean;
  };
  selectable?: boolean;
  onSelectionChange?: (selectedRows: FunctionItem[]) => void;
  pagination?: {
    current?: number;
    pageSize?: number;
    total?: number;
    showSizeChanger?: boolean;
    showQuickJumper?: boolean;
  };
  searchable?: boolean;
  filters?: boolean;
  compact?: boolean;
}

export const FunctionListTable: React.FC<FunctionListTableProps> = ({
  data = [],
  loading = false,
  onRefresh,
  onViewDetail,
  onInvoke,
  onEdit,
  onDelete,
  onToggleStatus,
  showActions = {
    view: true,
    invoke: true,
    edit: false,
    delete: false,
    toggle: false
  },
  selectable = false,
  onSelectionChange,
  pagination = {
    current: 1,
    pageSize: 10,
    showSizeChanger: true,
    showQuickJumper: true
  },
  searchable = true,
  filters = true,
  compact = false
}) => {
  const [selectedRows, setSelectedRows] = useState<FunctionItem[]>([]);

  // Process data for display
  const processedData = useMemo(() => {
    return data.map(row => ({
      ...row,
      displayName: row.display_name?.zh || row.display_name?.en || row.id,
      displaySummary: row.summary?.zh || row.summary?.en || '-',
      categoryName: row.category || '未分类'
    }));
  }, [data]);

  // Handle selection change
  const handleSelectionChange = (rows: FunctionItem[]) => {
    setSelectedRows(rows);
    if (onSelectionChange) {
      onSelectionChange(rows);
    }
  };

  // Get categories for filter
  const categories = useMemo(() => {
    const cats = [...new Set(data.map(item => item.category).filter(Boolean))];
    return cats.map(cat => ({ text: cat || '未分类', value: cat }));
  }, [data]);

  const columns: ProColumns<FunctionItem>[] = [
    {
      title: '函数ID',
      dataIndex: 'id',
      width: compact ? 200 : 250,
      copyable: true,
      ellipsis: true,
      render: (_, record) => (
        <Space direction="vertical" size="small">
          <Space>
            <Badge status={record.enabled ? 'success' : 'default'} />
            <Text code>{record.id}</Text>
          </Space>
          {record.version && <Tag color="blue" size="small">v{record.version}</Tag>}
        </Space>
      )
    },
    {
      title: '函数名称',
      dataIndex: 'display_name',
      width: compact ? 150 : 200,
      ellipsis: true,
      render: (_, record) => (
        <Text strong>{record.display_name?.zh || record.display_name?.en || record.id}</Text>
      )
    },
    {
      title: '函数描述',
      dataIndex: 'summary',
      width: compact ? 200 : 300,
      ellipsis: true,
      hideInSearch: compact,
      render: (_, record) => (
        <Text type="secondary">
          {record.summary?.zh || record.summary?.en || '-'}
        </Text>
      )
    }
  ];

  // Add category column if not compact
  if (!compact) {
    columns.push({
      title: '分类',
      dataIndex: 'category',
      width: 120,
      filters: filters ? categories : undefined,
      onFilter: filters ? (value, record) => record.category === value : undefined,
      render: (_, record) => (
        <Tag color={record.category ? 'geekblue' : 'default'}>
          {record.category || '未分类'}
        </Tag>
      )
    });
  }

  // Add tags column
  if (!compact) {
    columns.push({
      title: '标签',
      dataIndex: 'tags',
      width: 200,
      render: (_, record) => (
        <Space wrap>
          {(record.tags || []).slice(0, 3).map(tag => (
            <Tag key={tag} size="small">{tag}</Tag>
          ))}
          {(record.tags || []).length > 3 && (
            <Tag size="small">+{(record.tags || []).length - 3}</Tag>
          )}
        </Space>
      )
    });
  }

  // Add status column
  columns.push({
    title: '状态',
    dataIndex: 'enabled',
    width: 80,
    filters: filters ? [
      { text: '启用', value: true },
      { text: '禁用', value: false }
    ] : undefined,
    onFilter: filters ? (value, record) => record.enabled === value : undefined,
    render: (_, record) => (
      <Badge
        status={record.enabled ? 'success' : 'default'}
        text={record.enabled ? '启用' : '禁用'}
      />
    )
  });

  // Add actions column
  const actions = [];
  if (showActions.view) {
    actions.push(
      <Tooltip key="detail" title="查看详情">
        <Button
          type="link"
          size="small"
          icon={<InfoCircleOutlined />}
          onClick={() => onViewDetail?.(record)}
        />
      </Tooltip>
    );
  }
  if (showActions.invoke) {
    actions.push(
      <Tooltip key="invoke" title="调用函数">
        <Button
          type="link"
          size="small"
          icon={<PlayCircleOutlined />}
          onClick={() => onInvoke?.(record)}
        />
      </Tooltip>
    );
  }
  if (showActions.edit) {
    actions.push(
      <Tooltip key="edit" title="编辑">
        <Button
          type="link"
          size="small"
          icon={<EditOutlined />}
          onClick={() => onEdit?.(record)}
        />
      </Tooltip>
    );
  }
  if (showActions.toggle) {
    const toggleIcon = record.enabled ? <StopOutlined /> : <PlayCircleOutlined />;
    const toggleTitle = record.enabled ? '禁用' : '启用';
    actions.push(
      <Popconfirm
        key="toggle"
        title={`确定要${toggleTitle}此函数吗？`}
        onConfirm={() => onToggleStatus?.(record)}
      >
        <Tooltip title={toggleTitle}>
          <Button
            type="link"
            size="small"
            icon={toggleIcon}
            danger={record.enabled}
          />
        </Tooltip>
      </Popconfirm>
    );
  }
  if (showActions.delete) {
    actions.push(
      <Popconfirm
        key="delete"
        title="确定要删除此函数吗？此操作不可恢复！"
        onConfirm={() => onDelete?.(record)}
      >
        <Tooltip title="删除">
          <Button
            type="link"
            size="small"
            icon={<DeleteOutlined />}
            danger
          />
        </Tooltip>
      </Popconfirm>
    );
  }

  columns.push({
    title: '操作',
    valueType: 'option',
    width: actions.length * 40 + 20,
    render: (_, record) => <Space>{actions.map(action => action)}</Space>
  });

  const rowSelection = selectable ? {
    onChange: handleSelectionChange,
    getCheckboxProps: (record: FunctionItem) => ({
      disabled: false
    })
  } : undefined;

  return (
    <ProTable<FunctionItem>
      rowKey="id"
      loading={loading}
      columns={columns}
      dataSource={processedData}
      pagination={{
        ...pagination,
        total: pagination.total || data.length,
        showTotal: pagination.total ? (total) => `共 ${total} 个函数` : undefined
      }}
      search={{
        labelWidth: 'auto',
        collapseRender: searchable ? undefined : false
      }}
      dateFormatter="string"
      headerTitle="函数列表"
      toolBarRender={() => {
        const tools = [];
        if (onRefresh) {
          tools.push(
            <Button key="refresh" icon={<ReloadOutlined />} onClick={onRefresh}>
              刷新
            </Button>
          );
        }
        return tools;
      }}
      rowSelection={rowSelection}
      options={{
        density: compact,
        fullScreen: true,
        reload: !!onRefresh,
        setting: true
      }}
    />
  );
};

export default FunctionListTable;