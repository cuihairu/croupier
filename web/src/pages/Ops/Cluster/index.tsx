import React, { useCallback, useEffect, useRef, useState } from 'react';
import { App, Badge, Button, Card, Col, Row, Table, Tag, Tooltip, Typography } from 'antd';
import { PageContainer, StatisticCard } from '@ant-design/pro-components';
import { FormattedMessage, useIntl } from '@umijs/max';
import type { ColumnsType } from 'antd/es/table';
import { ReloadOutlined, ClusterOutlined } from '@ant-design/icons';
import { fetchClusterInfo, type ClusterInfo, type ClusterInstanceItem } from '@/services/api/ops';
import { extractErrorMessage } from '@/utils/errors';
import { formatDateTime } from '@/utils/format';

const { Text } = Typography;

export default function ClusterPage() {
  const { message } = App.useApp();
  const intl = useIntl();
  // useIntl 在测试 mock 下每次渲染返回新引用，直接进 useCallback 依赖会让请求
  // effect 无限重建；经 ref 转发后回调依赖稳定，执行时仍读取最新实例
  const intlRef = useRef(intl);
  intlRef.current = intl;
  const [loading, setLoading] = useState(false);
  const [info, setInfo] = useState<ClusterInfo | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      setInfo(await fetchClusterInfo());
    } catch (error) {
      message.error(
        extractErrorMessage(
          error,
          intlRef.current.formatMessage({
            id: 'pages.opsCluster.error.loadFailed',
            defaultMessage: '加载集群信息失败',
          }),
        ),
      );
    } finally {
      setLoading(false);
    }
  }, [message]);

  useEffect(() => {
    load();
    const t = setInterval(load, 10_000); // 10s 自动刷新在线状态
    return () => clearInterval(t);
  }, [load]);

  const items = info?.items || [];

  const columns: ColumnsType<ClusterInstanceItem> = [
    {
      title: intl.formatMessage({ id: 'pages.opsCluster.column.instance', defaultMessage: '实例' }),
      dataIndex: 'instanceId',
      render: (_: unknown, r: ClusterInstanceItem) => (
        <span>
          <ClusterOutlined style={{ marginRight: 6 }} />
          <Text strong>{r.instanceId}</Text>
          {r.self && (
            <Tag color="blue" style={{ marginLeft: 8 }}>
              <FormattedMessage
                id="pages.opsCluster.tag.currentInstance"
                defaultMessage="当前实例"
              />
            </Tag>
          )}
        </span>
      ),
    },
    {
      title: intl.formatMessage({
        id: 'pages.opsCluster.column.advertiseAddr',
        defaultMessage: '互联地址',
      }),
      dataIndex: 'advertiseAddr',
      render: (v: string) => v || '-',
    },
    { title: 'Epoch', dataIndex: 'epoch', width: 80 },
    {
      title: intl.formatMessage({
        id: 'pages.opsCluster.column.startedAt',
        defaultMessage: '启动时间',
      }),
      dataIndex: 'startedAt',
      width: 180,
      render: (v: string) => formatDateTime(v ?? ''),
    },
    {
      title: intl.formatMessage({
        id: 'pages.opsCluster.column.agentConnections',
        defaultMessage: 'Agent 连接',
      }),
      dataIndex: 'agentCount',
      width: 100,
      render: (v: number) => v || 0,
    },
    {
      title: intl.formatMessage({ id: 'pages.opsCluster.column.status', defaultMessage: '状态' }),
      dataIndex: 'alive',
      width: 100,
      render: (v: boolean) =>
        v ? (
          <Badge
            status="success"
            text={intl.formatMessage({
              id: 'pages.opsCluster.badge.online',
              defaultMessage: '在线',
            })}
          />
        ) : (
          <Tooltip
            title={intl.formatMessage({
              id: 'pages.opsCluster.tooltip.leaseExpired',
              defaultMessage: '租约过期——实例宕机或网络分区，其名下 Agent 将自动重连到存活实例',
            })}
          >
            <Badge
              status="error"
              text={intl.formatMessage({
                id: 'pages.opsCluster.badge.offline',
                defaultMessage: '离线',
              })}
            />
          </Tooltip>
        ),
    },
  ];

  return (
    <PageContainer
      subTitle={intl.formatMessage({
        id: 'pages.opsCluster.subTitle',
        defaultMessage: 'Server 多实例成员拓扑（10s 自动刷新）',
      })}
      extra={[
        <Button key="refresh" icon={<ReloadOutlined />} onClick={load} loading={loading}>
          <FormattedMessage id="pages.opsCluster.button.refresh" defaultMessage="刷新" />
        </Button>,
      ]}
    >
      {!info?.enabled && (
        <Card style={{ marginBottom: 16 }}>
          <Text type="secondary">
            <FormattedMessage
              id="pages.opsCluster.hint.singleInstance"
              defaultMessage="当前为单实例部署（cluster.enabled=false）。开启多实例高可用见 docs/architecture/server-ha-multi-instance.md。"
            />
          </Text>
        </Card>
      )}
      {info?.enabled && (
        <Row gutter={16} style={{ marginBottom: 16 }}>
          <Col span={8}>
            <StatisticCard
              statistic={{
                title: intl.formatMessage({
                  id: 'pages.opsCluster.statistic.instanceTotal',
                  defaultMessage: '实例总数',
                }),
                value: info.total,
              }}
            />
          </Col>
          <Col span={8}>
            <StatisticCard
              statistic={{
                title: intl.formatMessage({
                  id: 'pages.opsCluster.statistic.onlineInstances',
                  defaultMessage: '在线实例',
                }),
                value: info.aliveCount,
                styles: {
                  content: { color: info.aliveCount === info.total ? '#3f8600' : '#cf1322' },
                },
              }}
            />
          </Col>
          <Col span={8}>
            <StatisticCard
              statistic={{
                title: intl.formatMessage({
                  id: 'pages.opsCluster.statistic.agentConnections',
                  defaultMessage: 'Agent 连接分布',
                }),
                value: items.reduce(
                  (sum: number, it: ClusterInstanceItem) => sum + (it.agentCount || 0),
                  0,
                ),
                suffix: intl.formatMessage({
                  id: 'pages.opsCluster.statistic.agentUnit',
                  defaultMessage: '个',
                }),
              }}
            />
          </Col>
        </Row>
      )}
      <Card
        title={intl.formatMessage({
          id: 'pages.opsCluster.card.instanceList',
          defaultMessage: '实例列表',
        })}
      >
        <Table<ClusterInstanceItem>
          rowKey="instanceId"
          loading={loading}
          columns={columns}
          dataSource={items}
          pagination={false}
          size="middle"
        />
      </Card>
    </PageContainer>
  );
}
