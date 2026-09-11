import React from 'react';
import type { ProColumns } from '@ant-design/pro-components';
import { Badge, Button, Space, Tag, Tooltip, Typography } from 'antd';
import { BugOutlined, ClusterOutlined, EyeOutlined, HistoryOutlined } from '@ant-design/icons';
import { getIntl } from '@umijs/max';
import type { FunctionInstance } from '@/services/api';

const { Text } = Typography;

// 展示文案经 getIntl 求值（SelectLang 切换语言会整页刷新重新求值，先例
// services/api/bugs.ts；调用方 index.tsx 按任务约束不改，无法走 intl 参数注入）
const intl = getIntl();

type BuildInstanceColumnsOptions = {
  onDetail: (record: FunctionInstance) => void;
  onLogs: (record: FunctionInstance) => void;
  onDebug: (record: FunctionInstance) => void;
};

// 列表只保留识别实例所需的关键列；Agent ID / Service ID / 地址 /
// 最后心跳等长字段与完整元信息收纳到「详情」Drawer，避免横向溢出。
export function buildInstanceColumns({
  onDetail,
  onLogs,
  onDebug,
}: BuildInstanceColumnsOptions): ProColumns<FunctionInstance>[] {
  return [
    {
      title: intl.formatMessage({
        id: 'pages.functionsInstances.column.functionId',
        defaultMessage: '函数ID',
      }),
      dataIndex: 'functionId',
      width: 260,
      ellipsis: true,
      render: (_, record) => (
        <Space>
          <ClusterOutlined />
          <Text code copyable>
            {record.functionId}
          </Text>
        </Space>
      ),
    },
    {
      title: 'SDK',
      dataIndex: 'sdkName',
      width: 170,
      ellipsis: true,
      render: (_, record) =>
        record.sdkName || record.sdkLang ? (
          <Tooltip title={`${record.sdkLang || ''} ${record.sdkVersion || ''}`.trim()}>
            <Tag color="geekblue">{record.sdkName || record.sdkLang}</Tag>
          </Tooltip>
        ) : (
          <Text type="secondary">-</Text>
        ),
    },
    {
      title: intl.formatMessage({
        id: 'pages.functionsInstances.column.version',
        defaultMessage: '版本',
      }),
      dataIndex: 'version',
      width: 90,
      render: (_, record) => <Tag color="blue">{record.version || '-'}</Tag>,
    },
    {
      title: intl.formatMessage({
        id: 'pages.functionsInstances.column.env',
        defaultMessage: '环境',
      }),
      dataIndex: 'env',
      width: 150,
      ellipsis: true,
      render: (_, record) => (
        <Space>
          <Tag color="purple">{record.gameId || '-'}</Tag>
          <Tag>{record.env || '-'}</Tag>
        </Space>
      ),
    },
    {
      // 跨实例聚合标注：远端条目显示归属的对端实例，本地直连显示「本实例」
      //（单实例部署 cluster=nil 时全部为本地）。语义与 /ops/nodes 归属列一致。
      title: intl.formatMessage({
        id: 'pages.functionsInstances.column.ownerInstance',
        defaultMessage: '归属实例',
      }),
      dataIndex: 'ownerInstance',
      width: 120,
      ellipsis: true,
      render: (_, record) =>
        record.ownerInstance ? (
          <Tag color="geekblue">{record.ownerInstance}</Tag>
        ) : (
          <span style={{ color: '#999' }}>
            {intl.formatMessage({
              id: 'pages.functionsInstances.column.ownerSelf',
              defaultMessage: '本实例',
            })}
          </span>
        ),
    },
    {
      title: intl.formatMessage({
        id: 'pages.functionsInstances.column.status',
        defaultMessage: '状态',
      }),
      dataIndex: 'status',
      width: 100,
      filters: [
        {
          text: intl.formatMessage({
            id: 'pages.functionsInstances.status.running',
            defaultMessage: '运行中',
          }),
          value: 'running',
        },
        {
          text: intl.formatMessage({
            id: 'pages.functionsInstances.status.stopped',
            defaultMessage: '停止',
          }),
          value: 'stopped',
        },
        {
          text: intl.formatMessage({
            id: 'pages.functionsInstances.status.error',
            defaultMessage: '错误',
          }),
          value: 'error',
        },
      ],
      onFilter: (value, record) => record.status === value,
      render: (_, record) => (
        <Badge
          status={
            record.healthy || record.status === 'running'
              ? 'success'
              : record.status === 'error'
                ? 'error'
                : 'default'
          }
          text={
            record.healthy || record.status === 'running'
              ? intl.formatMessage({
                  id: 'pages.functionsInstances.status.running',
                  defaultMessage: '运行中',
                })
              : record.status === 'error'
                ? intl.formatMessage({
                    id: 'pages.functionsInstances.status.error',
                    defaultMessage: '错误',
                  })
                : intl.formatMessage({
                    id: 'pages.functionsInstances.status.stopped',
                    defaultMessage: '停止',
                  })
          }
        />
      ),
    },
    {
      title: intl.formatMessage({
        id: 'pages.functionsInstances.column.actions',
        defaultMessage: '操作',
      }),
      width: 120,
      fixed: 'right',
      render: (_, record) => (
        <Space>
          <Tooltip
            title={intl.formatMessage({
              id: 'pages.functionsInstances.column.rowAction.detail',
              defaultMessage: '查看详情',
            })}
          >
            <Button
              type="link"
              size="small"
              icon={<EyeOutlined />}
              onClick={() => onDetail(record)}
            />
          </Tooltip>
          <Tooltip
            title={intl.formatMessage({
              id: 'pages.functionsInstances.column.rowAction.logs',
              defaultMessage: '查看日志',
            })}
          >
            <Button
              type="link"
              size="small"
              icon={<HistoryOutlined />}
              onClick={() => onLogs(record)}
            />
          </Tooltip>
          <Tooltip
            title={intl.formatMessage({
              id: 'pages.functionsInstances.column.rowAction.debug',
              defaultMessage: '调试',
            })}
          >
            <Button
              type="link"
              size="small"
              icon={<BugOutlined />}
              onClick={() => onDebug(record)}
            />
          </Tooltip>
        </Space>
      ),
    },
  ];
}
