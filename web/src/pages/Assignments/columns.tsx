import React from 'react';
import type { Dispatch, SetStateAction } from 'react';
import type { ProColumns } from '@ant-design/pro-components';
import { Badge, Button, Progress, Space, Tag, Tooltip } from 'antd';
import {
  CheckCircleOutlined,
  DeleteOutlined,
  EditOutlined,
  ExperimentOutlined,
  SettingOutlined,
} from '@ant-design/icons';
import type { AssignmentPageSchema } from './pageSchema';
import type { AssignmentGroup, AssignmentItem } from './types';
import { formatDateTime } from './utils';

type BuildColumnsOptions = {
  canWrite: boolean;
  selected: string[];
  setSelected: Dispatch<SetStateAction<string[]>>;
  listColumns: AssignmentPageSchema['listColumns'];
  rowActions: AssignmentPageSchema['rowActions'];
  setEditingAssignment: Dispatch<SetStateAction<AssignmentItem | null>>;
  setCanaryModalVisible: Dispatch<SetStateAction<boolean>>;
  onOpenDetail: (id: string) => void;
  onOpenRoute: (id: string) => void;
};

export const buildAssignmentColumns = ({
  canWrite,
  selected,
  setSelected,
  listColumns,
  rowActions,
  setEditingAssignment,
  setCanaryModalVisible,
  onOpenDetail,
  onOpenRoute,
}: BuildColumnsOptions): ProColumns<AssignmentItem>[] =>
  listColumns.map((col) => {
    if (col.key === 'id') {
      return {
        title: col.title,
        dataIndex: 'id',
        width: col.width,
        copyable: col.copyable,
        render: (_, record) => (
          <Space>
            <Badge
              status={
                record.status === 'active'
                  ? 'success'
                  : record.status === 'canary'
                  ? 'processing'
                  : 'default'
              }
            />
            <span>{record.id}</span>
          </Space>
        ),
      } as ProColumns<AssignmentItem>;
    }
    if (col.key === 'name') {
      return {
        title: col.title,
        dataIndex: 'name',
        width: col.width,
        ellipsis: true,
      } as ProColumns<AssignmentItem>;
    }
    if (col.key === 'version') {
      return {
        title: col.title,
        dataIndex: 'version',
        width: col.width,
        render: (text) => <Tag color="blue">{text || '-'}</Tag>,
      } as ProColumns<AssignmentItem>;
    }
    if (col.key === 'status') {
      return {
        title: col.title,
        dataIndex: 'status',
        width: col.width,
        render: (text) => {
          const config = {
            active: { color: 'success', text: '已启用' },
            canary: { color: 'processing', text: '灰度中' },
            disabled: { color: 'default', text: '未启用' },
          } as const;
          const c = config[text as keyof typeof config] || config.disabled;
          return <Tag color={c.color}>{c.text}</Tag>;
        },
      } as ProColumns<AssignmentItem>;
    }
    if (col.key === 'route') {
      return {
        title: col.title,
        width: col.width,
        render: (_, record) => {
          const hasRoute = !!(
            (Array.isArray(record.menuNodes) && record.menuNodes.length > 0) ||
            record.menuPath
          );
          if (!hasRoute) return <Tag color="default">未配置</Tag>;
          return (
            <Space wrap size={[4, 6]}>
              {(record.menuNodes || []).map((node) => (
                <Tag key={node} color="blue">
                  {node}
                </Tag>
              ))}
              {record.menuPath && <Tag color="geekblue">{record.menuPath}</Tag>}
              <Tag color={record.menuSource === 'metadata' ? 'green' : 'default'}>
                {record.menuSource === 'metadata' ? '已自定义' : '默认'}
              </Tag>
            </Space>
          );
        },
      } as ProColumns<AssignmentItem>;
    }
    if (col.key === 'assignedAt') {
      return {
        title: col.title,
        dataIndex: 'assignedAt',
        width: col.width,
        render: (text) => formatDateTime(text as string | number | Date | undefined),
      } as ProColumns<AssignmentItem>;
    }
    return {
      title: col.title,
      width: col.width,
      render: (_, record) => {
        const iconMap = {
          check: <CheckCircleOutlined />,
          delete: <DeleteOutlined />,
          experiment: <ExperimentOutlined />,
          setting: <SettingOutlined />,
          edit: <EditOutlined />,
        } as const;

        const isVisible = (visibleWhen: 'isActive' | 'notActive' | undefined) => {
          if (!visibleWhen) return true;
          if (visibleWhen === 'isActive') return record.status === 'active';
          return record.status !== 'active';
        };

        const runAction = (key: AssignmentPageSchema['rowActions'][number]['key']) => {
          if (key === 'enable') {
            setSelected([...selected, record.id]);
            return;
          }
          if (key === 'disable') {
            setSelected(selected.filter((id) => id !== record.id));
            return;
          }
          if (key === 'canary') {
            setEditingAssignment(record);
            setCanaryModalVisible(true);
            return;
          }
          if (key === 'detail') {
            onOpenDetail(record.id);
            return;
          }
          onOpenRoute(record.id);
        };

        return (
          <Space>
            {rowActions
              .filter((action) => (action.permission === 'write' ? canWrite : true))
              .filter((action) => isVisible(action.visibleWhen))
              .map((action) => (
                <Tooltip key={`${record.id}-${action.key}`} title={action.tooltip}>
                  <Button
                    type="link"
                    size="small"
                    danger={!!action.danger}
                    icon={iconMap[action.icon]}
                    onClick={() => runAction(action.key)}
                  />
                </Tooltip>
              ))}
          </Space>
        );
      },
    } as ProColumns<AssignmentItem>;
  });

type BuildCategoryColumnsOptions = {
  categoryColumns: AssignmentPageSchema['categoryColumns'];
  onBatchAssign: (category: string, assign: boolean) => void;
};

export const buildCategoryColumns = ({
  categoryColumns,
  onBatchAssign,
}: BuildCategoryColumnsOptions): ProColumns<AssignmentGroup>[] =>
  categoryColumns.map((col) => {
    if (col.key === 'category') {
      return {
        title: col.title,
        dataIndex: 'category',
        width: col.width,
      } as ProColumns<AssignmentGroup>;
    }
    if (col.key === 'count') {
      return {
        title: col.title,
        dataIndex: 'items',
        width: col.width,
        render: (_, record) => record.items.length,
      } as ProColumns<AssignmentGroup>;
    }
    if (col.key === 'activeCount') {
      return {
        title: col.title,
        dataIndex: 'activeCount',
        width: col.width,
        render: (text) => <Tag color="green">{text}</Tag>,
      } as ProColumns<AssignmentGroup>;
    }
    if (col.key === 'activeRate') {
      return {
        title: col.title,
        width: col.width,
        render: (_, record) => {
          const percent =
            record.items.length > 0
              ? Math.round((record.activeCount / record.items.length) * 100)
              : 0;
          return (
            <Progress
              percent={percent}
              size="small"
              status={percent === 100 ? 'success' : undefined}
            />
          );
        },
      } as ProColumns<AssignmentGroup>;
    }
    return {
      title: col.title,
      width: col.width,
      render: (_, record) => (
        <Space>
          <Button
            size="small"
            type="primary"
            ghost
            onClick={() => onBatchAssign(record.category, true)}
          >
            全部启用
          </Button>
          <Button size="small" danger onClick={() => onBatchAssign(record.category, false)}>
            全部禁用
          </Button>
        </Space>
      ),
    } as ProColumns<AssignmentGroup>;
  });

type BuildRouteColumnsOptions = {
  routeColumns: AssignmentPageSchema['routeColumns'];
  onEditRoute: (id: string) => void;
};

export const buildRouteColumns = ({
  routeColumns,
  onEditRoute,
}: BuildRouteColumnsOptions): ProColumns<AssignmentItem>[] =>
  routeColumns.map((col) => {
    if (col.key === 'id') {
      return {
        title: col.title,
        dataIndex: 'id',
        width: col.width,
        copyable: col.copyable,
      } as ProColumns<AssignmentItem>;
    }
    if (col.key === 'name') {
      return {
        title: col.title,
        dataIndex: 'name',
        width: col.width,
        ellipsis: true,
      } as ProColumns<AssignmentItem>;
    }
    if (col.key === 'route') {
      return {
        title: col.title,
        width: col.width,
        render: (_, record) => (
          <Space wrap size={[4, 6]}>
            {Array.isArray(record.menuNodes) && record.menuNodes.length > 0 ? (
              record.menuNodes.map((node) => (
                <Tag key={node} color="blue">
                  {node}
                </Tag>
              ))
            ) : (
              <Tag>未分组</Tag>
            )}
            {record.menuPath ? (
              <Tag color="geekblue">{record.menuPath}</Tag>
            ) : (
              <Tag color="default">默认调用页</Tag>
            )}
            <Tag color={record.menuSource === 'metadata' ? 'green' : 'default'}>
              {record.menuSource === 'metadata' ? '已自定义' : '默认'}
            </Tag>
          </Space>
        ),
      } as ProColumns<AssignmentItem>;
    }
    return {
      title: col.title,
      width: col.width,
      render: (_, record) => (
        <Button
          type="link"
          size="small"
          icon={<EditOutlined />}
          onClick={() => onEditRoute(record.id)}
        >
          编辑路由
        </Button>
      ),
    } as ProColumns<AssignmentItem>;
  });
