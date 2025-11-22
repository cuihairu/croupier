import React, { useEffect, useState, useMemo } from 'react';
import { PageContainer, ProTable, ProColumns } from '@ant-design/pro-components';
import { App, Button, Space, Tag, Card, Badge, Tooltip, Typography, Statistic, Row, Col, Alert } from 'antd';
import {
  ClusterOutlined,
  ReloadOutlined,
  CheckCircleOutlined,
  CloseCircleOutlined,
  ExclamationCircleOutlined,
  InfoCircleOutlined
} from '@ant-design/icons';
import { request } from '@umijs/max';
import { listFunctionInstances } from '@/services/croupier';

const { Text, Title } = Typography;

type FunctionInstance = {
  agent_id: string;
  service_id: string;
  addr: string;
  version: string;
  function_id: string;
  status?: 'running' | 'stopped' | 'error';
  last_heartbeat?: string;
  functions_count?: number;
};

type CoverageData = {
  total_functions: number;
  covered_functions: number;
  coverage_percentage: number;
  total_instances: number;
  active_instances: number;
  inactive_instances: number;
};

export default () => {
  const { message } = App.useApp();
  const [instances, setInstances] = useState<FunctionInstance[]>([]);
  const [loading, setLoading] = useState(false);
  const [coverage, setCoverage] = useState<CoverageData | null>(null);

  const fetchData = async () => {
    setLoading(true);
    try {
      const res = await request('/api/function_instances');
      setInstances(res?.instances || []);

      // Calculate coverage statistics
      const functionsMap = new Map<string, number>();
      let activeCount = 0;
      let inactiveCount = 0;

      res?.instances?.forEach((instance: FunctionInstance) => {
        functionsMap.set(instance.function_id, (functionsMap.get(instance.function_id) || 0) + 1);
        if (instance.status === 'running') {
          activeCount++;
        } else {
          inactiveCount++;
        }
      });

      const totalFunctions = functionsMap.size;
      const coveredFunctions = Array.from(functionsMap.values()).filter(count => count > 0).length;

      setCoverage({
        total_functions: totalFunctions,
        covered_functions: coveredFunctions,
        coverage_percentage: totalFunctions > 0 ? Math.round((coveredFunctions / totalFunctions) * 100) : 0,
        total_instances: res?.instances?.length || 0,
        active_instances: activeCount,
        inactive_instances: inactiveCount
      });
    } catch (e: any) {
      message.error(e?.message || '加载失败');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => { fetchData(); }, []);

  // Process data for display
  const processedData = useMemo(() => {
    return instances.map(instance => ({
      ...instance,
      statusColor: instance.status === 'running' ? 'success' : instance.status === 'error' ? 'error' : 'default',
      statusText: instance.status === 'running' ? '运行中' : instance.status === 'error' ? '错误' : '停止',
      lastSeen: instance.last_heartbeat ? new Date(instance.last_heartbeat).toLocaleString('zh-CN') : '未知'
    }));
  }, [instances]);

  // Get unique functions for filter
  const functionFilters = useMemo(() => {
    const functions = [...new Set(instances.map(instance => instance.function_id))];
    return functions.map(funcId => ({ text: funcId, value: funcId }));
  }, [instances]);

  const columns: ProColumns<FunctionInstance>[] = [
    {
      title: 'Agent ID',
      dataIndex: 'agent_id',
      width: 200,
      copyable: true,
      ellipsis: true,
      render: (_, record) => (
        <Space>
          <ClusterOutlined />
          <Text code>{record.agent_id}</Text>
        </Space>
      )
    },
    {
      title: 'Service ID',
      dataIndex: 'service_id',
      width: 200,
      copyable: true,
      ellipsis: true,
      render: (_, record) => <Text code>{record.service_id}</Text>
    },
    {
      title: '地址',
      dataIndex: 'addr',
      width: 250,
      ellipsis: true,
      copyable: true
    },
    {
      title: '函数ID',
      dataIndex: 'function_id',
      width: 280,
      filters: functionFilters,
      onFilter: (value, record) => record.function_id === value,
      render: (_, record) => <Text code copyable>{record.function_id}</Text>
    },
    {
      title: '版本',
      dataIndex: 'version',
      width: 100,
      render: (_, record) => <Tag color="blue">{record.version}</Tag>
    },
    {
      title: '状态',
      dataIndex: 'status',
      width: 100,
      filters: [
        { text: '运行中', value: 'running' },
        { text: '停止', value: 'stopped' },
        { text: '错误', value: 'error' }
      ],
      onFilter: (value, record) => record.status === value,
      render: (_, record) => (
        <Badge
          status={record.status === 'running' ? 'success' : record.status === 'error' ? 'error' : 'default'}
          text={record.status === 'running' ? '运行中' : record.status === 'error' ? '错误' : '停止'}
        />
      )
    },
    {
      title: '最后心跳',
      dataIndex: 'last_heartbeat',
      width: 180,
      render: (_, record) => (
        <Text type="secondary">{record.last_heartbeat ? new Date(record.last_heartbeat).toLocaleString('zh-CN') : '未知'}</Text>
      )
    }
  ];

  return (
    <PageContainer
      title="函数实例管理"
      subTitle="监控和管理各个Agent实例上的函数注册情况"
      extra={[
        <Button key="refresh" icon={<ReloadOutlined />} onClick={fetchData}>
          刷新
        </Button>
      ]}
    >
      {/* Coverage Statistics */}
      {coverage && (
        <Card style={{ marginBottom: 16 }}>
          <Title level={5} style={{ marginBottom: 16 }}>
            <InfoCircleOutlined /> 覆盖率分析
          </Title>
          <Row gutter={16}>
            <Col span={6}>
              <Statistic
                title="总函数数"
                value={coverage.total_functions}
                prefix={<ClusterOutlined />}
              />
            </Col>
            <Col span={6}>
              <Statistic
                title="已覆盖函数"
                value={coverage.covered_functions}
                valueStyle={{ color: '#3f8600' }}
                prefix={<CheckCircleOutlined />}
              />
            </Col>
            <Col span={6}>
              <Statistic
                title="覆盖率"
                value={coverage.coverage_percentage}
                suffix="%"
                valueStyle={{
                  color: coverage.coverage_percentage >= 80 ? '#3f8600' :
                         coverage.coverage_percentage >= 60 ? '#fa8c16' : '#cf1322'
                }}
              />
            </Col>
            <Col span={6}>
              <Statistic
                title="活跃实例"
                value={coverage.active_instances}
                suffix={`/ ${coverage.total_instances}`}
                valueStyle={{ color: '#3f8600' }}
                prefix={<CheckCircleOutlined />}
              />
            </Col>
          </Row>

          {coverage.coverage_percentage < 80 && (
            <Alert
              message="覆盖率偏低"
              description={`当前函数覆盖率为 ${coverage.coverage_percentage}%，建议检查未覆盖的函数是否正确注册到Agent实例上。`}
              type="warning"
              showIcon
              style={{ marginTop: 16 }}
            />
          )}

          {coverage.inactive_instances > 0 && (
            <Alert
              message="发现离线实例"
              description={`有 ${coverage.inactive_instances} 个实例处于离线状态，可能影响函数调用的可用性。`}
              type="error"
              showIcon
              style={{ marginTop: 16 }}
            />
          )}
        </Card>
      )}

      {/* Instances Table */}
      <ProTable<FunctionInstance>
        rowKey="agent_id"
        loading={loading}
        columns={columns}
        dataSource={processedData}
        pagination={{
          pageSize: 10,
          showSizeChanger: true,
          showQuickJumper: true,
          showTotal: (total) => `共 ${total} 个实例`
        }}
        search={{
          filterType: 'light',
          labelWidth: 'auto'
        }}
        dateFormatter="string"
        headerTitle="实例列表"
        rowSelection={{
          type: 'checkbox',
          onChange: (selectedRowKeys, selectedRows) => {
            console.log('selectedRowKeys: ', selectedRowKeys, 'selectedRows: ', selectedRows);
          }
        }}
        toolBarRender={() => [
          <Button key="refresh" icon={<ReloadOutlined />} onClick={fetchData}>
            刷新数据
          </Button>
        ]}
      />
    </PageContainer>
  );
};