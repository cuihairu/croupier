import React, { useEffect, useState, useMemo } from 'react';
import {
  Card,
  Table,
  Tag,
  Badge,
  Space,
  Typography,
  Button,
  Tooltip,
  Statistic,
  Row,
  Col,
  Alert,
  Progress,
} from 'antd';
import {
  ClusterOutlined,
  ReloadOutlined,
  CheckCircleOutlined,
  ExclamationCircleOutlined,
  CloseCircleOutlined,
  InfoCircleOutlined,
  ApiOutlined,
  DatabaseOutlined,
} from '@ant-design/icons';
import { getRegistryServices, type RegistryService } from '@/services/api';

const { Text, Title, Paragraph } = Typography;

export type RegistryStats = {
  totalServices: number;
  healthyServices: number;
  unhealthyServices: number;
  totalFunctions: number;
  totalGames: number;
  coveragePercentage: number;
};

export interface RegistryViewerProps {
  gameId?: string;
  env?: string;
  autoRefresh?: boolean;
  refreshInterval?: number;
  compact?: boolean;
  showStats?: boolean;
  showHealthCheck?: boolean;
  onServiceClick?: (service: RegistryService) => void;
  onRefresh?: (services: RegistryService[]) => void;
}

export const RegistryViewer: React.FC<RegistryViewerProps> = ({
  gameId,
  env,
  autoRefresh = false,
  refreshInterval = 30000,
  compact = false,
  showStats = true,
  showHealthCheck = true,
  onServiceClick,
  onRefresh,
}) => {
  const [services, setServices] = useState<RegistryService[]>([]);
  const [stats, setStats] = useState<RegistryStats | null>(null);
  const [loading, setLoading] = useState(false);

  const fetchData = async () => {
    setLoading(true);
    try {
      const params: any = {};
      if (gameId) params.gameId = gameId;
      if (env) params.env = env;

      const res = await getRegistryServices(params);
      const serviceList = res?.services || [];

      setServices(serviceList);

      // Calculate statistics
      const totalServices = serviceList.length;
      const healthyServices = serviceList.filter((s) => s.status === 'healthy').length;
      const unhealthyServices = serviceList.filter((s) => s.status === 'unhealthy').length;
      const totalFunctions = serviceList.reduce((sum, s) => sum + (s.functionsCount || 0), 0);

      const games = new Set(serviceList.map((s) => s.gameId).filter(Boolean));
      const coveragePercentage =
        totalServices > 0 ? Math.round((healthyServices / totalServices) * 100) : 0;

      setStats({
        totalServices,
        healthyServices,
        unhealthyServices,
        totalFunctions,
        totalGames: games.size,
        coveragePercentage,
      });

      onRefresh?.(serviceList);
    } catch (error) {
      console.error('Failed to fetch registry data:', error);
      setServices([]);
      setStats(null);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchData();

    let intervalId: NodeJS.Timeout;
    if (autoRefresh && refreshInterval > 0) {
      intervalId = setInterval(fetchData, refreshInterval);
    }

    return () => {
      if (intervalId) {
        clearInterval(intervalId);
      }
    };
  }, [gameId, env, autoRefresh, refreshInterval]);

  const getStatusBadge = (status: string) => {
    const statusConfig = {
      healthy: { status: 'success' as const, text: '健康' },
      unhealthy: { status: 'error' as const, text: '异常' },
      unknown: { status: 'default' as const, text: '未知' },
    };

    const config = statusConfig[status as keyof typeof statusConfig] || statusConfig.unknown;
    return <Badge {...config} />;
  };

  const getStatusColor = (status: string) => {
    switch (status) {
      case 'healthy':
        return '#52c41a';
      case 'unhealthy':
        return '#ff4d4f';
      default:
        return '#d9d9d9';
    }
  };

  const formatLastSeen = (lastSeen: string) => {
    const date = new Date(lastSeen);
    const now = new Date();
    const diffMs = now.getTime() - date.getTime();
    const diffMins = Math.floor(diffMs / 60000);

    if (diffMins < 1) return '刚刚';
    if (diffMins < 60) return `${diffMins}分钟前`;
    if (diffMins < 1440) return `${Math.floor(diffMins / 60)}小时前`;
    return date.toLocaleString('zh-CN');
  };

  const processedServices = useMemo(() => {
    return services.map((service) => ({
      ...service,
      lastSeenText: formatLastSeen(service.lastSeen),
      healthPercentage:
        service.status === 'healthy' ? 100 : service.status === 'unhealthy' ? 0 : 50,
    }));
  }, [services]);

  const columns = [
    {
      title: '服务状态',
      dataIndex: 'status',
      width: 100,
      render: (_, record: RegistryService) => (
        <Space direction="vertical" size="small">
          {getStatusBadge(record.status)}
          {!compact && record.version && (
            <Tag size="small" color="blue">
              {record.version}
            </Tag>
          )}
        </Space>
      ),
    },
    {
      title: '服务ID',
      dataIndex: 'serviceId',
      width: 200,
      ellipsis: true,
      render: (_, record: RegistryService) => (
        <Button
          type="link"
          size="small"
          onClick={() => onServiceClick?.(record)}
          style={{ padding: 0 }}
        >
          {record.serviceId}
        </Button>
      ),
    },
    {
      title: '地址',
      dataIndex: 'addr',
      width: 250,
      ellipsis: true,
      copyable: true,
    },
    {
      title: '函数数量',
      dataIndex: 'functionsCount',
      width: 100,
      render: (count: number) => <Tag color={count > 0 ? 'green' : 'default'}>{count || 0}</Tag>,
      sorter: (a: RegistryService, b: RegistryService) =>
        (a.functionsCount || 0) - (b.functionsCount || 0),
    },
    ...(compact
      ? []
      : [
          {
            title: '游戏环境',
            dataIndex: 'gameId',
            width: 120,
            render: (_: any, record: RegistryService) => (
              <Space direction="vertical" size="small">
                {record.gameId && <Tag color="geekblue">{record.gameId}</Tag>}
                {record.env && <Tag color="orange">{record.env}</Tag>}
              </Space>
            ),
          },
        ]),
    {
      title: '最后心跳',
      dataIndex: 'lastSeen',
      width: 150,
      render: (_, record: RegistryService) => (
        <Text type="secondary">{formatLastSeen(record.lastSeen)}</Text>
      ),
    },
  ];

  return (
    <Space direction="vertical" style={{ width: '100%' }} size="middle">
      {/* Statistics Panel */}
      {showStats && stats && (
        <Card size={compact ? 'small' : 'default'}>
          <Row gutter={16}>
            <Col span={6}>
              <Statistic
                title="总服务数"
                value={stats.totalServices}
                prefix={<ClusterOutlined />}
              />
            </Col>
            <Col span={6}>
              <Statistic
                title="健康服务"
                value={stats.healthyServices}
                valueStyle={{ color: '#3f8600' }}
                prefix={<CheckCircleOutlined />}
              />
            </Col>
            <Col span={6}>
              <Statistic title="总函数数" value={stats.totalFunctions} prefix={<ApiOutlined />} />
            </Col>
            <Col span={6}>
              <Statistic title="游戏数" value={stats.totalGames} prefix={<DatabaseOutlined />} />
            </Col>
          </Row>

          {/* Health Progress */}
          {showHealthCheck && (
            <div style={{ marginTop: 16 }}>
              <div
                style={{
                  display: 'flex',
                  justifyContent: 'space-between',
                  alignItems: 'center',
                  marginBottom: 8,
                }}
              >
                <Text strong>服务健康状况</Text>
                <Text>{stats.coveragePercentage}%</Text>
              </div>
              <Progress
                percent={stats.coveragePercentage}
                status={
                  stats.coveragePercentage >= 90
                    ? 'success'
                    : stats.coveragePercentage >= 70
                    ? 'normal'
                    : 'exception'
                }
                strokeColor={{
                  '0%': '#108ee9',
                  '100%': '#87d068',
                }}
              />
            </div>
          )}

          {/* Alerts */}
          {stats.unhealthyServices > 0 && (
            <Alert
              message={`${stats.unhealthyServices} 个服务异常`}
              description="请检查服务状态和网络连接，确保所有服务正常运行。"
              type="warning"
              showIcon
              style={{ marginTop: 16 }}
            />
          )}

          {stats.coveragePercentage < 70 && (
            <Alert
              message="服务覆盖率偏低"
              description={`当前健康服务覆盖率为 ${stats.coveragePercentage}%，建议检查服务配置和部署情况。`}
              type="error"
              showIcon
              style={{ marginTop: 16 }}
            />
          )}
        </Card>
      )}

      {/* Services Table */}
      <Card
        size={compact ? 'small' : 'default'}
        title="服务注册表"
        extra={
          <Space>
            {autoRefresh && (
              <Text type="secondary" style={{ fontSize: '12px' }}>
                自动刷新: {refreshInterval / 1000}秒
              </Text>
            )}
            <Button size="small" icon={<ReloadOutlined />} onClick={fetchData} loading={loading}>
              刷新
            </Button>
          </Space>
        }
      >
        <Table
          columns={columns}
          dataSource={processedServices}
          rowKey="serviceId"
          loading={loading}
          size={compact ? 'small' : 'middle'}
          pagination={{
            pageSize: compact ? 5 : 10,
            showSizeChanger: !compact,
            showQuickJumper: !compact,
            showTotal: compact ? false : (total) => `共 ${total} 个服务`,
          }}
          scroll={compact ? undefined : { x: 1000 }}
          rowClassName={(record) => {
            if (record.status === 'unhealthy') return 'unhealthy-row';
            return '';
          }}
        />
      </Card>

      {/* Additional Information */}
      {!compact && services.length === 0 && !loading && (
        <Card size="default">
          <div style={{ textAlign: 'center', padding: '40px 0' }}>
            <ClusterOutlined style={{ fontSize: 48, color: '#d9d9d9', marginBottom: 16 }} />
            <Title level={4} type="secondary">
              暂无注册服务
            </Title>
            <Paragraph type="secondary">
              当前没有可用的服务注册信息。请检查服务是否正常运行并成功注册到注册中心。
            </Paragraph>
            <Button type="primary" icon={<ReloadOutlined />} onClick={fetchData}>
              刷新数据
            </Button>
          </div>
        </Card>
      )}
    </Space>
  );
};

export default RegistryViewer;
