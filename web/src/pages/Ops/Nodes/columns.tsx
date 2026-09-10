import React from 'react';
import type { ColumnsType } from 'antd/es/table';
import { Button, Space, Tag } from 'antd';
import { FormattedMessage } from '@umijs/max';
import type { NodeRow } from './shared';

/** 模块级文案助手接收 intl 的最小结构（@umijs/max 未导出 IntlShape 类型） */
type IntlFormatter = {
  formatMessage: (
    descriptor: { id: string; defaultMessage: string },
    values?: Record<string, string | number>,
  ) => string;
};

type BuildNodeColumnsOptions = {
  intl: IntlFormatter;
  onDetail: (record: NodeRow) => void;
  onDrain: (agentId: string) => void;
  onRestart: (agentId: string) => void;
  onCron: (record: NodeRow) => void;
};

// 主表格列 - 只显示关键信息
export function buildNodeColumns({
  intl,
  onDetail,
  onDrain,
  onRestart,
  onCron,
}: BuildNodeColumnsOptions): ColumnsType<NodeRow> {
  return [
    {
      title: intl.formatMessage({ id: 'pages.opsNodes.column.agentId', defaultMessage: '节点 ID' }),
      dataIndex: 'agentId',
      width: 200,
      ellipsis: true,
    },
    {
      title: intl.formatMessage({ id: 'pages.opsNodes.column.game', defaultMessage: '游戏' }),
      dataIndex: 'gameId',
      width: 100,
    },
    {
      title: intl.formatMessage({ id: 'pages.opsNodes.column.env', defaultMessage: '环境' }),
      dataIndex: 'env',
      width: 80,
    },
    { title: 'IP', dataIndex: 'ip', width: 130, ellipsis: true },
    {
      title: intl.formatMessage({
        id: 'pages.opsNodes.column.ownerInstance',
        defaultMessage: '归属实例',
      }),
      dataIndex: ['labels', 'ownerInstance'],
      width: 130,
      ellipsis: true,
      render: (v: unknown) =>
        typeof v === 'string' && v ? (
          <Tag color="geekblue">{v}</Tag>
        ) : (
          <span style={{ color: '#999' }}>
            <FormattedMessage id="pages.opsNodes.column.ownerSelf" defaultMessage="本实例" />
          </span>
        ),
    },
    {
      // 三方对账（agent 视角）：agent 自报"我连着的实例"（注册响应带回，
      // 心跳续报）。与归属列不一致 = 路由漂移/半开连接信号。
      title: intl.formatMessage({
        id: 'pages.opsNodes.column.reportedOwner',
        defaultMessage: 'agent 自报',
      }),
      dataIndex: ['labels', 'reportedOwner'],
      width: 150,
      ellipsis: true,
      render: (v: unknown, record) => {
        const reported = typeof v === 'string' && v ? v : '';
        if (!reported) return <span style={{ color: '#999' }}>—</span>;
        const claimed = record.labels?.ownerInstance || '';
        if (claimed && reported !== claimed) {
          return (
            <Tag
              color="red"
              title={intl.formatMessage(
                {
                  id: 'pages.opsNodes.column.reportedMismatch',
                  defaultMessage: `agent 自报 ${reported}，归属表 ${claimed}——疑似漂移/半开`,
                },
                { reported, claimed },
              )}
            >
              {reported} ≠ {claimed}
            </Tag>
          );
        }
        return <Tag color="green">{reported}</Tag>;
      },
    },
    {
      title: intl.formatMessage({
        id: 'pages.opsNodes.column.health',
        defaultMessage: '健康状态',
      }),
      dataIndex: 'healthy',
      width: 90,
      render: (v, record) =>
        record.nodeStatus === 'stale' || record.nodeStatus === 'offline' ? (
          <Tag color="red">
            <FormattedMessage id="pages.opsNodes.column.statusOffline" defaultMessage="离线" />
          </Tag>
        ) : v ? (
          <Tag color="green">
            <FormattedMessage id="pages.opsNodes.column.statusHealthy" defaultMessage="健康" />
          </Tag>
        ) : (
          <Tag color="default">
            <FormattedMessage id="pages.opsNodes.column.statusUnhealthy" defaultMessage="异常" />
          </Tag>
        ),
    },
    {
      title: intl.formatMessage({
        id: 'pages.opsNodes.column.opsStatus',
        defaultMessage: '运维状态',
      }),
      dataIndex: 'nodeStatus',
      width: 100,
      render: (v: string, record: NodeRow) => {
        const statusMap: Record<string, { color: string; text: string }> = {
          active: {
            color: 'green',
            text: intl.formatMessage({
              id: 'pages.opsNodes.column.nodeStatus.online',
              defaultMessage: '在线',
            }),
          },
          online: {
            color: 'green',
            text: intl.formatMessage({
              id: 'pages.opsNodes.column.nodeStatus.online',
              defaultMessage: '在线',
            }),
          },
          drained: {
            color: 'orange',
            text: intl.formatMessage({
              id: 'pages.opsNodes.column.nodeStatus.drained',
              defaultMessage: '已下线',
            }),
          },
          stale: {
            color: 'red',
            text: intl.formatMessage({
              id: 'pages.opsNodes.column.nodeStatus.offline',
              defaultMessage: '离线',
            }),
          },
          offline: {
            color: 'red',
            text: intl.formatMessage({
              id: 'pages.opsNodes.column.nodeStatus.offline',
              defaultMessage: '离线',
            }),
          },
          restarting: {
            color: 'blue',
            text: intl.formatMessage({
              id: 'pages.opsNodes.column.nodeStatus.restarting',
              defaultMessage: '重启中',
            }),
          },
        };
        const s = statusMap[v] || {
          color: 'default',
          text:
            v ||
            intl.formatMessage({
              id: 'pages.opsNodes.column.nodeStatus.unknown',
              defaultMessage: '未知',
            }),
        };
        // active/online 对运维语义等价（都健康在线），区别只在连接归属
        // 实例——active=本实例直连，online=集群对端持有（HA 转发可达），
        // 归属信息放 title 提示而不是状态文案。
        const owner = record.labels?.ownerInstance;
        if (v === 'online' && owner) {
          return (
            <Tag
              color={s.color}
              title={intl.formatMessage(
                {
                  id: 'pages.opsNodes.column.ownerClusterHeld',
                  defaultMessage: `连接由集群实例 ${owner} 持有`,
                },
                { owner },
              )}
            >
              {s.text}
            </Tag>
          );
        }
        return <Tag color={s.color}>{s.text}</Tag>;
      },
    },
    {
      title: intl.formatMessage({ id: 'pages.opsNodes.column.actions', defaultMessage: '操作' }),
      width: 220,
      fixed: 'right',
      render: (_, r) => (
        <Space>
          <Button size="small" onClick={() => onDetail(r)}>
            <FormattedMessage id="pages.opsNodes.column.buttonDetail" defaultMessage="详情" />
          </Button>
          <Button size="small" onClick={() => onDrain(r.agentId)}>
            <FormattedMessage id="pages.opsNodes.column.buttonDrain" defaultMessage="下线" />
          </Button>
          <Button size="small" onClick={() => onRestart(r.agentId)}>
            <FormattedMessage id="pages.opsNodes.column.buttonRestart" defaultMessage="重启" />
          </Button>
          <Button size="small" onClick={() => onCron(r)}>
            <FormattedMessage id="pages.opsNodes.column.buttonCron" defaultMessage="定时任务" />
          </Button>
        </Space>
      ),
    },
  ];
}
