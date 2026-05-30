import React, { useEffect, useState, useRef } from 'react';
import { PageContainer, ProTable, ProColumns } from '@ant-design/pro-components';
import {
  App,
  Button,
  Space,
  Tag,
  Badge,
  Drawer,
  Descriptions,
  Tooltip,
  Card,
  Statistic,
  Row,
  Col,
  Select,
  DatePicker,
} from 'antd';
import {
  ReloadOutlined,
  EyeOutlined,
  PlayCircleOutlined,
  ClockCircleOutlined,
  CheckCircleOutlined,
  CloseCircleOutlined,
  StopOutlined,
  LoadingOutlined,
} from '@ant-design/icons';
import {
  listFunctionCalls,
  getFunctionCallDetail,
  rerunFunctionCall,
  getFunctionCallStats,
  type FunctionCallItem,
  type FunctionCallStatsResponse,
} from '@/services/api/function-calls';
import dayjs from 'dayjs';

const { RangePicker } = DatePicker;
const { Option } = Select;

// 状态映射
const statusConfig: Record<string, { color: string; icon: React.ReactNode; text: string }> = {
  succeeded: { color: 'success', icon: <CheckCircleOutlined />, text: '成功' },
  failed: { color: 'error', icon: <CloseCircleOutlined />, text: '失败' },
  running: { color: 'processing', icon: <LoadingOutlined />, text: '运行中' },
  cancelled: { color: 'default', icon: <StopOutlined />, text: '已取消' },
  timeout: { color: 'warning', icon: <ClockCircleOutlined />, text: '超时' },
  pending: { color: 'default', icon: <LoadingOutlined />, text: '等待中' },
};

export default () => {
  const { message, modal } = App.useApp();
  const [loading, setLoading] = useState(false);
  const [dataSource, setDataSource] = useState<FunctionCallItem[]>([]);
  const [total, setTotal] = useState(0);
  const [currentPage, setCurrentPage] = useState(1);
  const [pageSize, setPageSize] = useState(20);
  const [detailVisible, setDetailVisible] = useState(false);
  const [selectedCall, setSelectedCall] = useState<FunctionCallItem | null>(null);
  const [stats, setStats] = useState<FunctionCallStatsResponse | null>(null);
  const [filters, setFilters] = useState<Record<string, any>>({});
  const actionRef = useRef<any>();
  const timerRef = useRef<NodeJS.Timeout>();

  // 加载数据
  const fetchData = async (page = currentPage, size = pageSize) => {
    setLoading(true);
    try {
      const params = {
        page,
        pageSize: size,
        ...filters,
      };
      const response = await listFunctionCalls(params);
      setDataSource(response.calls || []);
      setTotal(response.total || 0);
      setCurrentPage(response.page || 1);
      setPageSize(response.pageSize || 20);
    } catch (error: any) {
      message.error(error?.message || '加载调用历史失败');
    } finally {
      setLoading(false);
    }
  };

  // 加载统计数据
  const fetchStats = async () => {
    try {
      const response = await getFunctionCallStats(filters);
      setStats(response);
    } catch (error) {
      console.warn('加载统计数据失败', error);
    }
  };

  // 初始加载
  useEffect(() => {
    fetchData();
    fetchStats();

    // 自动刷新运行中的任务
    timerRef.current = setInterval(() => {
      const hasRunning = dataSource.some(
        (call) => call.status === 'running' || call.status === 'pending',
      );
      if (hasRunning) {
        fetchData();
        fetchStats();
      }
    }, 5000);

    return () => {
      if (timerRef.current) {
        clearInterval(timerRef.current);
      }
    };
  }, [filters]);

  // 查看详情
  const handleViewDetail = async (record: FunctionCallItem) => {
    try {
      const response = await getFunctionCallDetail(String(record.id));
      setSelectedCall(response || record);
      setDetailVisible(true);
    } catch (error: any) {
      message.error(error?.message || '获取详情失败');
    }
  };

  // 重新执行
  const handleRerun = async (record: FunctionCallItem) => {
    modal.confirm({
      title: '确认重新执行',
      content: `确定要重新执行函数 ${record.functionId} 吗？`,
      onOk: async () => {
        try {
          const response = await rerunFunctionCall(record.jobId);
          message.success(`已创建新任务: ${response.jobId}`);
          fetchData();
          fetchStats();
        } catch (error: any) {
          message.error(error?.message || '重新执行失败');
        }
      },
    });
  };

  // 格式化持续时间
  const formatDuration = (ms?: number) => {
    if (!ms) return '-';
    if (ms < 1000) return `${ms}ms`;
    if (ms < 60000) return `${(ms / 1000).toFixed(2)}s`;
    return `${(ms / 60000).toFixed(2)}m`;
  };

  // 格式化时间
  const formatTime = (timeStr?: string) => {
    if (!timeStr) return '-';
    return dayjs(timeStr).format('YYYY-MM-DD HH:mm:ss');
  };

  const columns: ProColumns<FunctionCallItem>[] = [
    {
      title: 'ID',
      dataIndex: 'id',
      width: 80,
      search: false,
    },
    {
      title: '任务ID',
      dataIndex: 'jobId',
      width: 200,
      copyable: true,
      ellipsis: true,
    },
    {
      title: '函数ID',
      dataIndex: 'functionId',
      width: 250,
      copyable: true,
      ellipsis: true,
    },
    {
      title: '状态',
      dataIndex: 'status',
      width: 100,
      filters: Object.keys(statusConfig).map((key) => ({
        text: statusConfig[key].text,
        value: key,
      })),
      render: (_, record) => {
        const config = statusConfig[record.status] || statusConfig.pending;
        return (
          <Badge
            status={config.color as any}
            text={
              <Space size={4}>
                {config.icon}
                {config.text}
              </Space>
            }
          />
        );
      },
    },
    {
      title: '游戏/环境',
      dataIndex: 'gameId',
      width: 150,
      render: (_, record) => (
        <Space direction="vertical" size={0}>
          <span>{record.gameId || '-'}</span>
          {record.env && <Tag size="small">{record.env}</Tag>}
        </Space>
      ),
    },
    {
      title: 'Agent',
      dataIndex: 'agentId',
      width: 150,
      ellipsis: true,
      search: false,
    },
    {
      title: '执行人',
      dataIndex: 'actorId',
      width: 120,
      ellipsis: true,
      search: false,
    },
    {
      title: '持续时间',
      dataIndex: 'durationMs',
      width: 100,
      search: false,
      render: (_, record) => formatDuration(record.durationMs),
    },
    {
      title: '开始时间',
      dataIndex: 'startedAt',
      width: 180,
      search: false,
      render: (_, record) => formatTime(record.startedAt),
    },
    {
      title: '错误信息',
      dataIndex: 'errorMessage',
      width: 200,
      ellipsis: true,
      search: false,
      render: (_, record) =>
        record.errorMessage ? (
          <Tooltip title={record.errorMessage}>
            <span style={{ color: '#ff4d4f' }}>{record.errorMessage}</span>
          </Tooltip>
        ) : (
          '-'
        ),
    },
    {
      title: '操作',
      valueType: 'option',
      width: 120,
      fixed: 'right',
      render: (_, record) => [
        <Tooltip key="detail" title="查看详情">
          <Button
            type="link"
            size="small"
            icon={<EyeOutlined />}
            onClick={() => handleViewDetail(record)}
          />
        </Tooltip>,
        (record.status === 'failed' ||
          record.status === 'timeout' ||
          record.status === 'cancelled') && (
          <Tooltip key="rerun" title="重新执行">
            <Button
              type="link"
              size="small"
              icon={<PlayCircleOutlined />}
              onClick={() => handleRerun(record)}
            />
          </Tooltip>
        ),
      ],
    },
  ];

  return (
    <PageContainer
      title="函数调用历史"
      subTitle="查看和管理函数调用记录"
      extra={[
        <Button
          key="refresh"
          icon={<ReloadOutlined />}
          onClick={() => {
            fetchData();
            fetchStats();
          }}
        >
          刷新
        </Button>,
      ]}
    >
      {/* 统计卡片 */}
      {stats && (
        <Row gutter={16} style={{ marginBottom: 16 }}>
          <Col span={4}>
            <Card>
              <Statistic title="总调用" value={stats.total} />
            </Card>
          </Col>
          <Col span={4}>
            <Card>
              <Statistic
                title="成功"
                value={stats.succeeded}
                valueStyle={{ color: '#52c41a' }}
                suffix={`/ ${stats.total}`}
              />
            </Card>
          </Col>
          <Col span={4}>
            <Card>
              <Statistic title="失败" value={stats.failed} valueStyle={{ color: '#ff4d4f' }} />
            </Card>
          </Col>
          <Col span={4}>
            <Card>
              <Statistic title="运行中" value={stats.running} valueStyle={{ color: '#1890ff' }} />
            </Card>
          </Col>
          <Col span={4}>
            <Card>
              <Statistic title="平均耗时" value={formatDuration(stats.avgDurationMs)} />
            </Card>
          </Col>
          <Col span={4}>
            <Card>
              <Statistic
                title="成功率"
                value={stats.total > 0 ? ((stats.succeeded / stats.total) * 100).toFixed(1) : 0}
                suffix="%"
              />
            </Card>
          </Col>
        </Row>
      )}

      <ProTable<FunctionCallItem>
        rowKey="id"
        actionRef={actionRef}
        loading={loading}
        columns={columns}
        dataSource={dataSource}
        pagination={{
          current: currentPage,
          pageSize: pageSize,
          total: total,
          showSizeChanger: true,
          showQuickJumper: true,
          showTotal: (total) => `共 ${total} 条记录`,
          onChange: (page, size) => fetchData(page, size),
        }}
        search={{
          filterType: 'light',
          labelWidth: 'auto',
        }}
        dateFormatter="string"
        headerTitle="调用历史"
        toolBarRender={() => [
          <Select
            key="status-filter"
            placeholder="状态筛选"
            allowClear
            style={{ width: 120 }}
            onChange={(value) => {
              setFilters({ ...filters, status: value });
              setCurrentPage(1);
            }}
          >
            {Object.keys(statusConfig).map((key) => (
              <Option key={key} value={key}>
                {statusConfig[key].text}
              </Option>
            ))}
          </Select>,
          <RangePicker
            key="time-range"
            showTime
            placeholder={['开始时间', '结束时间']}
            onChange={(dates) => {
              if (dates && dates[0] && dates[1]) {
                setFilters({
                  ...filters,
                  start_time: dates[0].toISOString(),
                  end_time: dates[1].toISOString(),
                });
              } else {
                const { start_time, end_time, ...rest } = filters;
                setFilters(rest);
              }
              setCurrentPage(1);
            }}
          />,
        ]}
      />

      {/* 详情抽屉 */}
      <Drawer
        title="调用详情"
        width={720}
        open={detailVisible}
        onClose={() => setDetailVisible(false)}
      >
        {selectedCall && (
          <Card>
            <Descriptions column={2} bordered size="small">
              <Descriptions.Item label="ID" span={2}>
                {selectedCall.id}
              </Descriptions.Item>
              <Descriptions.Item label="任务ID" span={2}>
                <Space>
                  <span>{selectedCall.jobId}</span>
                  {(selectedCall.status === 'failed' ||
                    selectedCall.status === 'timeout' ||
                    selectedCall.status === 'cancelled') && (
                    <Button
                      type="primary"
                      size="small"
                      icon={<PlayCircleOutlined />}
                      onClick={() => handleRerun(selectedCall)}
                    >
                      重新执行
                    </Button>
                  )}
                </Space>
              </Descriptions.Item>
              <Descriptions.Item label="函数ID" span={2}>
                {selectedCall.functionId}
              </Descriptions.Item>
              <Descriptions.Item label="状态">
                <Badge
                  status={(statusConfig[selectedCall.status]?.color || 'default') as any}
                  text={statusConfig[selectedCall.status]?.text || selectedCall.status}
                />
              </Descriptions.Item>
              <Descriptions.Item label="重试次数">{selectedCall.retryCount || 0}</Descriptions.Item>
              <Descriptions.Item label="游戏ID">{selectedCall.gameId || '-'}</Descriptions.Item>
              <Descriptions.Item label="环境">{selectedCall.env || '-'}</Descriptions.Item>
              <Descriptions.Item label="Agent ID" span={2}>
                {selectedCall.agentId || '-'}
              </Descriptions.Item>
              <Descriptions.Item label="服务 ID" span={2}>
                {selectedCall.serviceId || '-'}
              </Descriptions.Item>
              <Descriptions.Item label="执行人">{selectedCall.actorId || '-'}</Descriptions.Item>
              <Descriptions.Item label="执行人类型">
                {selectedCall.actorType || '-'}
              </Descriptions.Item>
              <Descriptions.Item label="开始时间" span={2}>
                {formatTime(selectedCall.startedAt)}
              </Descriptions.Item>
              <Descriptions.Item label="结束时间" span={2}>
                {formatTime(selectedCall.finishedAt)}
              </Descriptions.Item>
              <Descriptions.Item label="持续时间">
                {formatDuration(selectedCall.durationMs)}
              </Descriptions.Item>
              <Descriptions.Item label="创建时间" span={2}>
                {formatTime(selectedCall.createdAt)}
              </Descriptions.Item>
              {selectedCall.errorMessage && (
                <Descriptions.Item label="错误信息" span={2}>
                  <span style={{ color: '#ff4d4f' }}>{selectedCall.errorMessage}</span>
                </Descriptions.Item>
              )}
            </Descriptions>

            {/* 请求数据 */}
            {selectedCall.payload && Object.keys(selectedCall.payload).length > 0 && (
              <Card size="small" title="请求数据" style={{ marginTop: 16 }}>
                <pre
                  style={{
                    background: '#f5f5f5',
                    padding: 8,
                    borderRadius: 4,
                    maxHeight: 200,
                    overflow: 'auto',
                  }}
                >
                  {JSON.stringify(selectedCall.payload, null, 2)}
                </pre>
              </Card>
            )}

            {/* 响应数据 */}
            {selectedCall.result && Object.keys(selectedCall.result).length > 0 && (
              <Card size="small" title="响应数据" style={{ marginTop: 16 }}>
                <pre
                  style={{
                    background: '#f5f5f5',
                    padding: 8,
                    borderRadius: 4,
                    maxHeight: 200,
                    overflow: 'auto',
                  }}
                >
                  {JSON.stringify(selectedCall.result, null, 2)}
                </pre>
              </Card>
            )}
          </Card>
        )}
      </Drawer>
    </PageContainer>
  );
};
