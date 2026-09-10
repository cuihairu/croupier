import React from 'react';
import type { ProColumns } from '@ant-design/pro-components';
import { Badge, Button, Space, Tag, Tooltip, Typography } from 'antd';
import { BugOutlined, ClusterOutlined, EyeOutlined, HistoryOutlined } from '@ant-design/icons';
import type { FunctionInstance } from '@/services/api';

const { Text } = Typography;

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
      title: '函数ID',
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
      title: '版本',
      dataIndex: 'version',
      width: 90,
      render: (_, record) => <Tag color="blue">{record.version || '-'}</Tag>,
    },
    {
      title: '环境',
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
      title: '状态',
      dataIndex: 'status',
      width: 100,
      filters: [
        { text: '运行中', value: 'running' },
        { text: '停止', value: 'stopped' },
        { text: '错误', value: 'error' },
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
              ? '运行中'
              : record.status === 'error'
                ? '错误'
                : '停止'
          }
        />
      ),
    },
    {
      title: '操作',
      width: 120,
      fixed: 'right',
      render: (_, record) => (
        <Space>
          <Tooltip title="查看详情">
            <Button
              type="link"
              size="small"
              icon={<EyeOutlined />}
              onClick={() => onDetail(record)}
            />
          </Tooltip>
          <Tooltip title="查看日志">
            <Button
              type="link"
              size="small"
              icon={<HistoryOutlined />}
              onClick={() => onLogs(record)}
            />
          </Tooltip>
          <Tooltip title="调试">
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
