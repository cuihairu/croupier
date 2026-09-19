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
      // SDK 名称 + SDK 版本同行展示：SDK 版本是排查协议兼容问题的关键信息，
      // 不再只藏在 Tooltip 里（与 /functions/sdk-distribution 的「SDK 版本」措辞一致）。
      title: 'SDK',
      dataIndex: 'sdkName',
      width: 200,
      ellipsis: true,
      render: (_, record) =>
        record.sdkName || record.sdkLang ? (
          <Space size={4}>
            <Tag color="geekblue">{record.sdkName || record.sdkLang}</Tag>
            {record.sdkVersion ? (
              <Text type="secondary">{record.sdkVersion}</Text>
            ) : (
              <Text type="secondary">-</Text>
            )}
          </Space>
        ) : (
          <Text type="secondary">-</Text>
        ),
    },
    {
      // 契约版本：游戏服务在 SDK RegisterWithAgent(serviceID, serviceVersion)
      // 时上报、随函数 descriptor 落库的版本（行粒度=函数，即该函数契约版本）；
      // server 端已对非 semver 自报值归一为 1.0.0（provider_version_invalid 告警）。
      title: intl.formatMessage({
        id: 'pages.functionsInstances.column.contractVersion',
        defaultMessage: '契约版本',
      }),
      dataIndex: 'version',
      width: 100,
      render: (_, record) => <Tag color="blue">{record.version || '-'}</Tag>,
    },
    {
      // Agent 版本：agent 进程自身上报的版本（RegisterRequest.Version）。
      title: intl.formatMessage({
        id: 'pages.functionsInstances.column.agentVersion',
        defaultMessage: 'Agent 版本',
      }),
      dataIndex: 'agentVersion',
      width: 110,
      render: (_, record) => <Tag color="cyan">{record.agentVersion || '-'}</Tag>,
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
      render: (_, record) => (
        <Tooltip
          title={intl.formatMessage(
            {
              id: 'pages.functionsInstances.column.ownerHint',
              defaultMessage:
                '维护该会话的 croupier-server 实例（多实例 HA 部署下的分片归属；单实例部署恒为「本实例」）',
            },
            {},
          )}
        >
          {record.ownerInstance ? (
            <Tag color="geekblue">{record.ownerInstance}</Tag>
          ) : (
            <span style={{ color: '#999' }}>
              {intl.formatMessage({
                id: 'pages.functionsInstances.column.ownerSelf',
                defaultMessage: '本实例',
              })}
            </span>
          )}
        </Tooltip>
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
