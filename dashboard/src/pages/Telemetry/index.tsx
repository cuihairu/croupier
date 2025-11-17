import React, { useEffect, useState, useMemo } from 'react';
import { Card, Row, Col, Statistic, Table, Space, Button, Tag, Typography, Alert, App, Spin, Progress } from 'antd';
import { LineChartOutlined, ApiOutlined, FireOutlined, UserOutlined, DollarOutlined, BugOutlined } from '@ant-design/icons';
import { useModel } from '@umijs/max';
import GameSelector from '@/components/GameSelector';

const { Title, Text } = Typography;

// 模拟API服务
const telemetryAPI = {
  // 获取游戏实时指标
  async getGameMetrics(gameId: string, env: string) {
    // 这里应该调用实际的遥测API
    return {
      dau: Math.floor(Math.random() * 10000 + 5000),
      sessionDuration: Math.floor(Math.random() * 180000 + 120000),
      revenue: Math.floor(Math.random() * 5000 + 1000),
      crashRate: Math.random() * 0.05,
      levelCompletionRate: Math.random() * 0.3 + 0.7,
      retentionD1: Math.random() * 0.2 + 0.6,
    };
  },

  // 获取链路追踪数据
  async getTraces(gameId: string, limit: number = 20) {
    const traces = [];
    for (let i = 0; i < limit; i++) {
      traces.push({
        traceId: `trace_${Math.random().toString(36).substring(7)}`,
        operationName: ['session.start', 'level.complete', 'purchase.attempt', 'match.start'][Math.floor(Math.random() * 4)],
        duration: Math.floor(Math.random() * 500 + 50),
        status: Math.random() > 0.9 ? 'error' : 'success',
        timestamp: new Date(Date.now() - Math.random() * 3600000).toISOString(),
        spans: Math.floor(Math.random() * 10 + 3),
      });
    }
    return traces.sort((a, b) => new Date(b.timestamp).getTime() - new Date(a.timestamp).getTime());
  },

  // 获取系统健康状态
  async getHealthStatus() {
    return {
      otelCollector: Math.random() > 0.1 ? 'healthy' : 'unhealthy',
      jaeger: Math.random() > 0.05 ? 'healthy' : 'unhealthy',
      prometheus: Math.random() > 0.05 ? 'healthy' : 'unhealthy',
      redis: Math.random() > 0.1 ? 'healthy' : 'unhealthy',
      clickhouse: Math.random() > 0.1 ? 'healthy' : 'unhealthy',
    };
  },
};

export default function TelemetryPage() {
  const { message } = App.useApp();
  const [gameId, setGameId] = useState<string | undefined>(localStorage.getItem('game_id') || undefined);
  const [env, setEnv] = useState<string | undefined>(localStorage.getItem('env') || undefined);
  const [loading, setLoading] = useState(false);
  const [metrics, setMetrics] = useState<any>({});
  const [traces, setTraces] = useState<any[]>([]);
  const [health, setHealth] = useState<any>({});

  const { initialState } = useModel('@@initialState');
  const roles = useMemo(() => {
    const acc = (initialState as any)?.currentUser?.access as string | undefined;
    return (acc ? acc.split(',') : []).filter(Boolean);
  }, [initialState]);

  const canView = roles.includes('*') || roles.includes('telemetry:read');

  async function loadData() {
    if (!gameId || !canView) return;

    setLoading(true);
    try {
      const [metricsData, tracesData, healthData] = await Promise.all([
        telemetryAPI.getGameMetrics(gameId, env || 'production'),
        telemetryAPI.getTraces(gameId),
        telemetryAPI.getHealthStatus(),
      ]);

      setMetrics(metricsData);
      setTraces(tracesData);
      setHealth(healthData);
    } catch (error) {
      message.error('Failed to load telemetry data');
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    loadData();
    // 定时刷新数据
    const interval = setInterval(loadData, 30000); // 30秒刷新一次
    return () => clearInterval(interval);
  }, [gameId, env, canView]);

  useEffect(() => {
    const onStorage = () => {
      setGameId(localStorage.getItem('game_id') || undefined);
      setEnv(localStorage.getItem('env') || undefined);
    };
    window.addEventListener('storage', onStorage);
    return () => window.removeEventListener('storage', onStorage);
  }, []);

  const traceColumns = [
    {
      title: 'Trace ID',
      dataIndex: 'traceId',
      key: 'traceId',
      render: (traceId: string) => <Text code copyable>{traceId}</Text>,
    },
    {
      title: 'Operation',
      dataIndex: 'operationName',
      key: 'operationName',
      render: (name: string) => <Tag color="blue">{name}</Tag>,
    },
    {
      title: 'Duration',
      dataIndex: 'duration',
      key: 'duration',
      render: (duration: number) => `${duration}ms`,
      sorter: (a: any, b: any) => a.duration - b.duration,
    },
    {
      title: 'Spans',
      dataIndex: 'spans',
      key: 'spans',
    },
    {
      title: 'Status',
      dataIndex: 'status',
      key: 'status',
      render: (status: string) => (
        <Tag color={status === 'success' ? 'green' : 'red'}>
          {status.toUpperCase()}
        </Tag>
      ),
    },
    {
      title: 'Timestamp',
      dataIndex: 'timestamp',
      key: 'timestamp',
      render: (timestamp: string) => new Date(timestamp).toLocaleString(),
    },
  ];

  const getHealthColor = (status: string) => {
    return status === 'healthy' ? 'green' : 'red';
  };

  if (!canView) {
    return (
      <Card title="Telemetry Dashboard">
        <Alert
          message="Access Denied"
          description="You don't have permission to view telemetry data."
          type="error"
          showIcon
        />
      </Card>
    );
  }

  return (
    <div>
      <Card
        title="游戏遥测监控台"
        extra={<GameSelector />}
        actions={[
          <Button key="refresh" onClick={loadData} loading={loading}>
            刷新数据
          </Button>,
          <Button key="export" type="link">
            导出报告
          </Button>,
        ]}
      >
        <Space direction="vertical" style={{ width: '100%' }} size="large">
          <div>
            <Text>Game: <b>{gameId || '-'}</b> / Env: <b>{env || '-'}</b></Text>
          </div>

          {/* 系统健康状态 */}
          <Card size="small" title="系统健康状态">
            <Row gutter={16}>
              {Object.entries(health).map(([service, status]) => (
                <Col span={4} key={service}>
                  <Tag color={getHealthColor(status as string)}>
                    {service}: {status}
                  </Tag>
                </Col>
              ))}
            </Row>
          </Card>

          {/* 核心指标 */}
          <Row gutter={16}>
            <Col span={4}>
              <Card>
                <Statistic
                  title="日活跃用户"
                  value={metrics.dau || 0}
                  prefix={<UserOutlined />}
                  valueStyle={{ color: '#3f8600' }}
                />
              </Card>
            </Col>
            <Col span={4}>
              <Card>
                <Statistic
                  title="平均会话时长"
                  value={(metrics.sessionDuration || 0) / 1000 / 60}
                  suffix="分钟"
                  precision={1}
                  prefix={<LineChartOutlined />}
                  valueStyle={{ color: '#1890ff' }}
                />
              </Card>
            </Col>
            <Col span={4}>
              <Card>
                <Statistic
                  title="今日收入"
                  value={metrics.revenue || 0}
                  prefix={<DollarOutlined />}
                  suffix="USD"
                  valueStyle={{ color: '#cf1322' }}
                />
              </Card>
            </Col>
            <Col span={4}>
              <Card>
                <Statistic
                  title="崩溃率"
                  value={(metrics.crashRate || 0) * 100}
                  suffix="%"
                  precision={2}
                  prefix={<BugOutlined />}
                  valueStyle={{ color: metrics.crashRate > 0.02 ? '#cf1322' : '#3f8600' }}
                />
              </Card>
            </Col>
            <Col span={4}>
              <Card>
                <Statistic
                  title="关卡完成率"
                  value={(metrics.levelCompletionRate || 0) * 100}
                  suffix="%"
                  precision={1}
                  prefix={<FireOutlined />}
                  valueStyle={{ color: '#1890ff' }}
                />
              </Card>
            </Col>
            <Col span={4}>
              <Card>
                <Statistic
                  title="次日留存率"
                  value={(metrics.retentionD1 || 0) * 100}
                  suffix="%"
                  precision={1}
                  prefix={<ApiOutlined />}
                  valueStyle={{ color: '#3f8600' }}
                />
              </Card>
            </Col>
          </Row>

          {/* 性能进度条 */}
          <Row gutter={16}>
            <Col span={8}>
              <Card size="small" title="关卡完成率">
                <Progress
                  percent={(metrics.levelCompletionRate || 0) * 100}
                  status={metrics.levelCompletionRate > 0.8 ? 'success' : 'active'}
                />
              </Card>
            </Col>
            <Col span={8}>
              <Card size="small" title="次日留存率">
                <Progress
                  percent={(metrics.retentionD1 || 0) * 100}
                  status={metrics.retentionD1 > 0.6 ? 'success' : 'exception'}
                />
              </Card>
            </Col>
            <Col span={8}>
              <Card size="small" title="系统稳定性">
                <Progress
                  percent={(1 - (metrics.crashRate || 0)) * 100}
                  status={metrics.crashRate < 0.01 ? 'success' : 'exception'}
                />
              </Card>
            </Col>
          </Row>

          {/* 链路追踪表格 */}
          <Card title="最近的链路追踪" size="small">
            <Table
              columns={traceColumns}
              dataSource={traces}
              rowKey="traceId"
              pagination={{ pageSize: 10 }}
              loading={loading}
              size="small"
            />
          </Card>

          {/* 操作面板 */}
          <Card size="small" title="快速操作">
            <Space>
              <Button type="primary" href="/telemetry/traces" target="_blank">
                查看Jaeger
              </Button>
              <Button href="/telemetry/metrics" target="_blank">
                查看Prometheus
              </Button>
              <Button href="/telemetry/dashboards" target="_blank">
                Grafana仪表板
              </Button>
              <Button onClick={() => message.info('导出功能开发中')}>
                导出数据
              </Button>
            </Space>
          </Card>
        </Space>
      </Card>
    </div>
  );
}