import React, { useCallback, useEffect, useState, useRef } from 'react';
import {
  PageContainer,
  ProTable,
  ProColumns,
  StatisticCard,
  type ActionType,
} from '@ant-design/pro-components';
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
  Row,
  Col,
  Select,
  DatePicker,
} from 'antd';
import {
  ReloadOutlined,
  EyeOutlined,
  ClockCircleOutlined,
  CheckCircleOutlined,
  CloseCircleOutlined,
  StopOutlined,
  LoadingOutlined,
} from '@ant-design/icons';
import {
  listFunctionCalls,
  getFunctionCallDetail,
  getFunctionCallStats,
  type FunctionCallItem,
  type FunctionCallStatsResponse,
} from '@/services/api/function-calls';
import type { JSONValue } from '@/types/dashboard';
import { extractErrorMessage } from '@/utils/errors';

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
  const { message } = App.useApp();
  const [detailVisible, setDetailVisible] = useState(false);
  const [selectedCall, setSelectedCall] = useState<FunctionCallItem | null>(null);
  const [stats, setStats] = useState<FunctionCallStatsResponse | null>(null);
  const [filters, setFilters] = useState<Record<string, JSONValue>>({});
  const actionRef = useRef<ActionType | undefined>(undefined);
  // 最近一次列表数据里是否有运行中/等待中的调用，供轮询判断是否自动刷新
  const hasRunningRef = useRef(false);

  // 加载统计数据
  const fetchStats = useCallback(async () => {
    try {
      const response = await getFunctionCallStats(filters);
      setStats(response);
    } catch (error) {
      console.warn('加载统计数据失败', error);
    }
  }, [filters]);

  // 初始/筛选变化加载统计（列表由 ProTable request 驱动）
  useEffect(() => {
    fetchStats();
  }, [fetchStats]);

  // 自动刷新运行中的任务：以最近一次列表数据是否有 running/pending 为准
  useEffect(() => {
    const timer = setInterval(() => {
      if (hasRunningRef.current) {
        actionRef.current?.reload();
        fetchStats();
      }
    }, 5000);
    return () => {
      clearInterval(timer);
    };
  }, [fetchStats]);

  // 查看详情
  const handleViewDetail = async (record: FunctionCallItem) => {
    try {
      const response = await getFunctionCallDetail(String(record.id));
      setSelectedCall(response || record);
      setDetailVisible(true);
    } catch (error) {
      message.error(extractErrorMessage(error, '获取详情失败'));
    }
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
    const date = new Date(timeStr);
    if (Number.isNaN(date.getTime())) return '-';
    return date.toLocaleString('zh-CN', { hour12: false });
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
      dataIndex: 'taskId',
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
            status={config.color as 'success' | 'error' | 'processing' | 'default' | 'warning'}
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
        <Space orientation="vertical" size={0}>
          <span>{record.gameId || '-'}</span>
          {record.env && <Tag>{record.env}</Tag>}
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
            actionRef.current?.reload();
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
            <StatisticCard statistic={{ title: '总调用', value: stats.total }} />
          </Col>
          <Col span={4}>
            <StatisticCard
              statistic={{
                title: '成功',
                value: stats.succeeded,
                styles: { content: { color: '#52c41a' } },
                suffix: `/ ${stats.total}`,
              }}
            />
          </Col>
          <Col span={4}>
            <StatisticCard
              statistic={{
                title: '失败',
                value: stats.failed,
                styles: { content: { color: '#ff4d4f' } },
              }}
            />
          </Col>
          <Col span={4}>
            <StatisticCard
              statistic={{
                title: '运行中',
                value: stats.running,
                styles: { content: { color: '#1890ff' } },
              }}
            />
          </Col>
          <Col span={4}>
            <StatisticCard
              statistic={{ title: '平均耗时', value: formatDuration(stats.avgDurationMs) }}
            />
          </Col>
          <Col span={4}>
            <StatisticCard
              statistic={{
                title: '成功率',
                value: stats.total > 0 ? ((stats.succeeded / stats.total) * 100).toFixed(1) : 0,
                suffix: '%',
              }}
            />
          </Col>
        </Row>
      )}

      <ProTable<FunctionCallItem>
        scroll={{ x: 1700 }}
        rowKey="id"
        actionRef={actionRef}
        columns={columns}
        params={{ filters }}
        request={async ({
          current = 1,
          pageSize = 20,
          filters: currentFilters = {},
          functionId,
          status,
          gameId,
        }) => {
          // 工具栏筛选与查询表单字段合并，表单显式输入优先
          const merged: Record<string, JSONValue> = { ...currentFilters };
          if (functionId) merged.functionId = functionId;
          if (status) merged.status = status;
          if (gameId) merged.gameId = gameId;
          try {
            const response = await listFunctionCalls({ page: current, pageSize, ...merged });
            hasRunningRef.current = (response.calls || []).some(
              (call) => call.status === 'running' || call.status === 'pending',
            );
            return { data: response.calls || [], total: response.total || 0, success: true };
          } catch (error) {
            message.error(extractErrorMessage(error, '加载调用历史失败'));
            return { data: [], total: 0, success: false };
          }
        }}
        pagination={{
          pageSize: 20,
          showSizeChanger: true,
          showQuickJumper: true,
          showTotal: (total) => `共 ${total} 条记录`,
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
              // 筛选变化回第 1 页：params 变化与 setPageInfo 的双触发由
              // ProTable 内部 debounce + abort 合并，不会出现错序数据
              actionRef.current?.setPageInfo?.({ current: 1 });
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
                  startTime: dates[0].toISOString(),
                  endTime: dates[1].toISOString(),
                });
              } else {
                const rest = Object.fromEntries(
                  Object.entries(filters).filter(([k]) => k !== 'start_time' && k !== 'end_time'),
                );
                setFilters(rest);
              }
              actionRef.current?.setPageInfo?.({ current: 1 });
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
                  <span>{selectedCall.taskId}</span>
                </Space>
              </Descriptions.Item>
              <Descriptions.Item label="函数ID" span={2}>
                {selectedCall.functionId}
              </Descriptions.Item>
              <Descriptions.Item label="状态">
                <Badge
                  status={
                    (statusConfig[selectedCall.status]?.color || 'default') as
                      'success' | 'error' | 'processing' | 'default' | 'warning'
                  }
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
                    borderRadius: 8,
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
                    borderRadius: 8,
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
