import React, { useCallback, useEffect, useState } from 'react';
import {
  App,
  Badge,
  Button,
  Card,
  Col,
  Row,
  Statistic,
  Table,
  Tag,
  Tooltip,
  Typography,
} from 'antd';
import { PageContainer } from '@ant-design/pro-components';
import type { ColumnsType } from 'antd/es/table';
import { ReloadOutlined, ClusterOutlined } from '@ant-design/icons';
import { fetchClusterInfo, type ClusterInfo, type ClusterInstanceItem } from '@/services/api/ops';
import { extractErrorMessage } from '@/utils/errors';

const { Text } = Typography;

export default function ClusterPage() {
  const { message } = App.useApp();
  const [loading, setLoading] = useState(false);
  const [info, setInfo] = useState<ClusterInfo | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      setInfo(await fetchClusterInfo());
    } catch (error) {
      message.error(extractErrorMessage(error, '加载集群信息失败'));
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
      title: '实例',
      dataIndex: 'instanceId',
      render: (_: unknown, r: ClusterInstanceItem) => (
        <span>
          <ClusterOutlined style={{ marginRight: 6 }} />
          <Text strong>{r.instanceId}</Text>
          {r.self && (
            <Tag color="blue" style={{ marginLeft: 8 }}>
              当前实例
            </Tag>
          )}
        </span>
      ),
    },
    { title: '互联地址', dataIndex: 'advertiseAddr', render: (v: string) => v || '-' },
    { title: 'Epoch', dataIndex: 'epoch', width: 80 },
    {
      title: '启动时间',
      dataIndex: 'startedAt',
      width: 180,
      render: (v: string) => (v ? new Date(v).toLocaleString() : '-'),
    },
    {
      title: 'Agent 连接',
      dataIndex: 'agentCount',
      width: 100,
      render: (v: number) => v || 0,
    },
    {
      title: '状态',
      dataIndex: 'alive',
      width: 100,
      render: (v: boolean) =>
        v ? (
          <Badge status="success" text="在线" />
        ) : (
          <Tooltip title="租约过期——实例宕机或网络分区，其名下 Agent 将自动重连到存活实例">
            <Badge status="error" text="离线" />
          </Tooltip>
        ),
    },
  ];

  return (
    <PageContainer
      subTitle="Server 多实例成员拓扑（10s 自动刷新）"
      extra={[
        <Button key="refresh" icon={<ReloadOutlined />} onClick={load} loading={loading}>
          刷新
        </Button>,
      ]}
    >
      {!info?.enabled && (
        <Card style={{ marginBottom: 16 }}>
          <Text type="secondary">
            当前为单实例部署（cluster.enabled=false）。开启多实例高可用见
            docs/architecture/server-ha-multi-instance.md。
          </Text>
        </Card>
      )}
      {info?.enabled && (
        <Row gutter={16} style={{ marginBottom: 16 }}>
          <Col span={8}>
            <Card>
              <Statistic title="实例总数" value={info.total} />
            </Card>
          </Col>
          <Col span={8}>
            <Card>
              <Statistic
                title="在线实例"
                value={info.aliveCount}
                valueStyle={{ color: info.aliveCount === info.total ? '#3f8600' : '#cf1322' }}
              />
            </Card>
          </Col>
          <Col span={8}>
            <Card>
              <Statistic
                title="Agent 连接分布"
                value={items.reduce(
                  (sum: number, it: ClusterInstanceItem) => sum + (it.agentCount || 0),
                  0,
                )}
                suffix="个"
              />
            </Card>
          </Col>
        </Row>
      )}
      <Card title="实例列表">
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
