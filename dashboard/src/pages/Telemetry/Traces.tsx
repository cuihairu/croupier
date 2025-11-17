import React, { useState, useEffect } from 'react';
import { Card, Table, Space, Button, Input, Select, Tag, Typography, Timeline, Modal, Descriptions, Row, Col } from 'antd';
import { SearchOutlined, EyeOutlined, ReloadOutlined } from '@ant-design/icons';

const { Search } = Input;
const { Text } = Typography;
const { Option } = Select;

interface SpanData {
  spanId: string;
  operationName: string;
  startTime: number;
  duration: number;
  status: 'success' | 'error';
  tags: Record<string, any>;
}

interface TraceDetail {
  traceId: string;
  serviceName: string;
  startTime: string;
  duration: number;
  totalSpans: number;
  errorSpans: number;
  spans: SpanData[];
}

export default function TracesPage() {
  const [loading, setLoading] = useState(false);
  const [traces, setTraces] = useState<any[]>([]);
  const [selectedTrace, setSelectedTrace] = useState<TraceDetail | null>(null);
  const [modalVisible, setModalVisible] = useState(false);
  const [filters, setFilters] = useState({
    service: '',
    operation: '',
    status: '',
    minDuration: '',
  });

  const mockTraces = [
    {
      traceId: 'trace_abc123',
      serviceName: 'croupier-server',
      operation: 'session.start',
      startTime: new Date().toISOString(),
      duration: 124,
      spans: 5,
      status: 'success',
      errorCount: 0,
      userId: 'user123',
      gameId: 'tower-defense',
    },
    {
      traceId: 'trace_def456',
      serviceName: 'croupier-server',
      operation: 'level.complete',
      startTime: new Date(Date.now() - 60000).toISOString(),
      duration: 256,
      spans: 8,
      status: 'success',
      errorCount: 0,
      userId: 'user456',
      gameId: 'tower-defense',
    },
    {
      traceId: 'trace_ghi789',
      serviceName: 'croupier-agent',
      operation: 'function.invoke',
      startTime: new Date(Date.now() - 120000).toISOString(),
      duration: 1024,
      spans: 12,
      status: 'error',
      errorCount: 2,
      userId: 'user789',
      gameId: 'card-game',
    },
  ];

  const mockTraceDetail: TraceDetail = {
    traceId: 'trace_abc123',
    serviceName: 'croupier-server',
    startTime: new Date().toISOString(),
    duration: 124,
    totalSpans: 5,
    errorSpans: 0,
    spans: [
      {
        spanId: 'span_1',
        operationName: 'HTTP GET /api/session/start',
        startTime: 0,
        duration: 124,
        status: 'success',
        tags: {
          'http.method': 'GET',
          'http.url': '/api/session/start',
          'http.status_code': 200,
          'game.user_id': 'user123',
          'game.session_id': 'session456',
        },
      },
      {
        spanId: 'span_2',
        operationName: 'telemetry.start_user_session',
        startTime: 5,
        duration: 15,
        status: 'success',
        tags: {
          'game.platform': 'ios',
          'game.region': 'us-east',
          'game.type': 'tower_defense',
        },
      },
      {
        spanId: 'span_3',
        operationName: 'analytics.send_event',
        startTime: 25,
        duration: 45,
        status: 'success',
        tags: {
          'event.type': 'session.start',
          'analytics.bridge.enabled': true,
        },
      },
      {
        spanId: 'span_4',
        operationName: 'redis.xadd',
        startTime: 30,
        duration: 35,
        status: 'success',
        tags: {
          'redis.command': 'XADD',
          'redis.key': 'game:events:session.start',
        },
      },
      {
        spanId: 'span_5',
        operationName: 'response.send',
        startTime: 100,
        duration: 24,
        status: 'success',
        tags: {
          'response.size': 156,
        },
      },
    ],
  };

  useEffect(() => {
    setTraces(mockTraces);
  }, []);

  const columns = [
    {
      title: 'Trace ID',
      dataIndex: 'traceId',
      key: 'traceId',
      render: (traceId: string) => (
        <Space>
          <Text code copyable style={{ fontSize: '12px' }}>
            {traceId}
          </Text>
          <Button
            type="link"
            size="small"
            icon={<EyeOutlined />}
            onClick={() => showTraceDetail(traceId)}
          />
        </Space>
      ),
    },
    {
      title: 'Service',
      dataIndex: 'serviceName',
      key: 'serviceName',
      render: (service: string) => <Tag color="blue">{service}</Tag>,
    },
    {
      title: 'Operation',
      dataIndex: 'operation',
      key: 'operation',
      render: (operation: string) => <Tag>{operation}</Tag>,
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
      render: (status: string, record: any) => {
        const color = status === 'success' ? 'green' : 'red';
        const errorText = record.errorCount > 0 ? ` (${record.errorCount} errors)` : '';
        return (
          <Tag color={color}>
            {status.toUpperCase()}{errorText}
          </Tag>
        );
      },
    },
    {
      title: 'User',
      dataIndex: 'userId',
      key: 'userId',
      render: (userId: string) => <Text code>{userId}</Text>,
    },
    {
      title: 'Game',
      dataIndex: 'gameId',
      key: 'gameId',
      render: (gameId: string) => <Tag color="orange">{gameId}</Tag>,
    },
    {
      title: 'Start Time',
      dataIndex: 'startTime',
      key: 'startTime',
      render: (time: string) => new Date(time).toLocaleString(),
    },
  ];

  const showTraceDetail = (traceId: string) => {
    setSelectedTrace(mockTraceDetail);
    setModalVisible(true);
  };

  const renderSpanTags = (tags: Record<string, any>) => (
    <Space wrap>
      {Object.entries(tags).map(([key, value]) => (
        <Tag key={key} color="geekblue">
          {key}: {String(value)}
        </Tag>
      ))}
    </Space>
  );

  return (
    <Card
      title="链路追踪"
      extra={
        <Space>
          <Button icon={<ReloadOutlined />} onClick={() => setTraces(mockTraces)}>
            刷新
          </Button>
          <Select
            placeholder="服务"
            style={{ width: 120 }}
            value={filters.service}
            onChange={(value) => setFilters({ ...filters, service: value })}
          >
            <Option value="">全部服务</Option>
            <Option value="croupier-server">Server</Option>
            <Option value="croupier-agent">Agent</Option>
          </Select>
          <Select
            placeholder="状态"
            style={{ width: 100 }}
            value={filters.status}
            onChange={(value) => setFilters({ ...filters, status: value })}
          >
            <Option value="">全部</Option>
            <Option value="success">成功</Option>
            <Option value="error">错误</Option>
          </Select>
          <Search
            placeholder="搜索操作或Trace ID"
            style={{ width: 200 }}
            onSearch={(value) => console.log('Search:', value)}
          />
        </Space>
      }
    >
      <Table
        columns={columns}
        dataSource={traces}
        rowKey="traceId"
        loading={loading}
        pagination={{ pageSize: 20 }}
        size="small"
      />

      {/* Trace详情模态框 */}
      <Modal
        title={`Trace详情 - ${selectedTrace?.traceId}`}
        open={modalVisible}
        onCancel={() => setModalVisible(false)}
        footer={null}
        width={1000}
      >
        {selectedTrace && (
          <Space direction="vertical" style={{ width: '100%' }} size="middle">
            {/* 基本信息 */}
            <Descriptions title="基本信息" column={3} bordered size="small">
              <Descriptions.Item label="Trace ID">{selectedTrace.traceId}</Descriptions.Item>
              <Descriptions.Item label="Service">{selectedTrace.serviceName}</Descriptions.Item>
              <Descriptions.Item label="Duration">{selectedTrace.duration}ms</Descriptions.Item>
              <Descriptions.Item label="Total Spans">{selectedTrace.totalSpans}</Descriptions.Item>
              <Descriptions.Item label="Error Spans">{selectedTrace.errorSpans}</Descriptions.Item>
              <Descriptions.Item label="Start Time">
                {new Date(selectedTrace.startTime).toLocaleString()}
              </Descriptions.Item>
            </Descriptions>

            {/* Span时间线 */}
            <Card size="small" title="Span时间线">
              <Timeline mode="left">
                {selectedTrace.spans.map((span, index) => (
                  <Timeline.Item
                    key={span.spanId}
                    color={span.status === 'success' ? 'green' : 'red'}
                    label={`+${span.startTime}ms`}
                  >
                    <div>
                      <Text strong>{span.operationName}</Text>
                      <br />
                      <Text type="secondary">Duration: {span.duration}ms</Text>
                      <br />
                      <Text type="secondary">Status: </Text>
                      <Tag color={span.status === 'success' ? 'green' : 'red'}>
                        {span.status}
                      </Tag>
                      <br />
                      <div style={{ marginTop: 8 }}>
                        {renderSpanTags(span.tags)}
                      </div>
                    </div>
                  </Timeline.Item>
                ))}
              </Timeline>
            </Card>

            {/* Span列表 */}
            <Card size="small" title="Span详情">
              <Table
                size="small"
                dataSource={selectedTrace.spans}
                rowKey="spanId"
                columns={[
                  { title: 'Span ID', dataIndex: 'spanId', key: 'spanId', width: 100 },
                  { title: 'Operation', dataIndex: 'operationName', key: 'operationName' },
                  { title: 'Start', dataIndex: 'startTime', key: 'startTime', render: (t: number) => `+${t}ms`, width: 80 },
                  { title: 'Duration', dataIndex: 'duration', key: 'duration', render: (d: number) => `${d}ms`, width: 80 },
                  {
                    title: 'Status',
                    dataIndex: 'status',
                    key: 'status',
                    width: 80,
                    render: (status: string) => (
                      <Tag color={status === 'success' ? 'green' : 'red'}>{status}</Tag>
                    ),
                  },
                ]}
                pagination={false}
              />
            </Card>
          </Space>
        )}
      </Modal>
    </Card>
  );
}