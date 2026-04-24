import React, { useEffect, useState, useMemo } from 'react';
import {
  Timeline,
  Badge,
  Tag,
  Space,
  Typography,
  Button,
  Drawer,
  Descriptions,
  Card,
  Empty,
  Tooltip,
} from 'antd';
import {
  PlayCircleOutlined,
  CheckCircleOutlined,
  CloseCircleOutlined,
  ClockCircleOutlined,
  EyeOutlined,
  ReloadOutlined,
  RedoOutlined,
} from '@ant-design/icons';
import {
  listFunctionCalls,
  rerunFunctionCall,
  type FunctionCallItem,
  type FunctionCallsListParams,
} from '@/services/api';

const { Text, Title, Paragraph } = Typography;

type FunctionCall = FunctionCallItem;

type FunctionCallView = FunctionCallItem & {
  durationText: string;
  startedText: string;
  completedText: string;
};

export interface FunctionCallHistoryProps {
  functionId?: string;
  userId?: string;
  gameId?: string;
  limit?: number;
  showRefresh?: boolean;
  compact?: boolean;
  onRefresh?: () => void;
  onViewDetail?: (call: FunctionCallItem) => void;
  onRerun?: (call: FunctionCallItem) => void;
}

export const FunctionCallHistory: React.FC<FunctionCallHistoryProps> = ({
  functionId,
  userId,
  gameId,
  limit = 20,
  showRefresh = true,
  compact = false,
  onRefresh,
  onViewDetail,
  onRerun,
}) => {
  const [calls, setCalls] = useState<FunctionCall[]>([]);
  const [loading, setLoading] = useState(false);
  const [detailVisible, setDetailVisible] = useState(false);
  const [selectedCall, setSelectedCall] = useState<FunctionCallView | null>(null);

  const fetchCalls = async () => {
    setLoading(true);
    try {
      const params: FunctionCallsListParams = {};
      if (functionId) params.functionId = functionId;
      if (userId) params.actorId = userId;
      if (gameId) params.gameId = gameId;
      if (limit) params.pageSize = limit;

      const res = await listFunctionCalls(params);
      setCalls(res?.calls || []);
    } catch (error) {
      console.error('Failed to fetch function calls:', error);
      setCalls([]);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchCalls();
  }, [functionId, userId, gameId, limit]);

  const handleRefresh = () => {
    fetchCalls();
    onRefresh?.();
  };

  const handleViewDetail = (call: FunctionCall) => {
    setSelectedCall(decorateCall(call));
    setDetailVisible(true);
    onViewDetail?.(call);
  };

  const handleRerun = (call: FunctionCall) => {
    if (call.jobId) {
      // Try to rerun the job
      rerunFunctionCall(call.id).then(handleRefresh).catch(console.error);
    }
    onRerun?.(call);
  };

  const getStatusIcon = (status: string) => {
    switch (status) {
      case 'success':
        return <CheckCircleOutlined style={{ color: '#52c41a' }} />;
      case 'failed':
        return <CloseCircleOutlined style={{ color: '#ff4d4f' }} />;
      case 'running':
        return <PlayCircleOutlined style={{ color: '#1890ff' }} />;
      case 'cancelled':
        return <ClockCircleOutlined style={{ color: '#faad14' }} />;
      default:
        return <ClockCircleOutlined style={{ color: '#d9d9d9' }} />;
    }
  };

  const getStatusText = (status: string) => {
    switch (status) {
      case 'success':
        return '成功';
      case 'failed':
        return '失败';
      case 'running':
        return '运行中';
      case 'cancelled':
        return '已取消';
      default:
        return '未知';
    }
  };

  const getStatusBadge = (status: string) => {
    const statusConfig = {
      success: { status: 'success' as const, text: '成功' },
      failed: { status: 'error' as const, text: '失败' },
      running: { status: 'processing' as const, text: '运行中' },
      cancelled: { status: 'warning' as const, text: '已取消' },
    };

    const config = statusConfig[status as keyof typeof statusConfig] || {
      status: 'default' as const,
      text: '未知',
    };
    return <Badge {...config} />;
  };

  const formatDuration = (duration?: number) => {
    if (!duration) return '-';
    if (duration < 1000) return `${duration}ms`;
    return `${(duration / 1000).toFixed(2)}s`;
  };

  const formatDate = (dateString: string) => {
    return new Date(dateString).toLocaleString('zh-CN');
  };

  const decorateCall = (call: FunctionCall): FunctionCallView => ({
    ...call,
    durationText: formatDuration(call.durationMs),
    startedText: formatDate(call.startedAt || call.createdAt),
    completedText: call.finishedAt ? formatDate(call.finishedAt) : '-',
  });

  const processedCalls = useMemo(() => {
    return calls.map(decorateCall);
  }, [calls]);

  if (calls.length === 0 && !loading) {
    return (
      <Card size={compact ? 'small' : 'default'}>
        <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="暂无调用记录">
          {showRefresh && (
            <Button icon={<ReloadOutlined />} onClick={handleRefresh}>
              刷新
            </Button>
          )}
        </Empty>
      </Card>
    );
  }

  return (
    <>
      <Card
        size={compact ? 'small' : 'default'}
        title="调用历史"
        extra={
          showRefresh && (
            <Button
              size="small"
              icon={<ReloadOutlined />}
              onClick={handleRefresh}
              loading={loading}
            >
              刷新
            </Button>
          )
        }
      >
        <Timeline mode={compact ? 'left' : 'alternate'}>
          {processedCalls.map((call, index) => (
            <Timeline.Item
              key={call.id}
              dot={getStatusIcon(call.status)}
              color={
                call.status === 'success' ? 'green' : call.status === 'failed' ? 'red' : 'blue'
              }
            >
              <Card
                size="small"
                style={{ marginBottom: 8, cursor: 'pointer' }}
                onClick={() => handleViewDetail(call)}
              >
                <Space direction="vertical" style={{ width: '100%' }}>
                  <Space style={{ justifyContent: 'space-between', width: '100%' }}>
                    <Space>
                      {getStatusBadge(call.status)}
                      <Text code>{call.functionId}</Text>
                      {call.user && <Text type="secondary">by {call.user}</Text>}
                    </Space>
                    <Space>
                      {call.durationMs && <Tag color="blue">{call.durationText}</Tag>}
                      <Button
                        size="small"
                        type="link"
                        icon={<EyeOutlined />}
                        onClick={(e) => {
                          e.stopPropagation();
                          handleViewDetail(call);
                        }}
                      />
                      {call.status === 'failed' && onRerun && (
                        <Button
                          size="small"
                          type="link"
                          icon={<RedoOutlined />}
                          onClick={(e) => {
                            e.stopPropagation();
                            handleRerun(call);
                          }}
                        />
                      )}
                    </Space>
                  </Space>
                  <Text type="secondary" style={{ fontSize: '12px' }}>
                    {call.startedText}
                  </Text>
                  {call.error && (
                    <Text type="danger" style={{ fontSize: '12px' }}>
                      {call.error}
                    </Text>
                  )}
                  {(call.gameId || call.env || call.agentId) && (
                    <Space wrap>
                      {call.gameId && <Tag>Game: {call.gameId}</Tag>}
                      {call.env && <Tag>Env: {call.env}</Tag>}
                      {call.agentId && <Tag>Agent: {call.agentId}</Tag>}
                    </Space>
                  )}
                </Space>
              </Card>
            </Timeline.Item>
          ))}
        </Timeline>
      </Card>

      {/* Detail Drawer */}
      <Drawer
        title="调用详情"
        width={600}
        open={detailVisible}
        onClose={() => setDetailVisible(false)}
      >
        {selectedCall && (
          <Space direction="vertical" style={{ width: '100%' }}>
            <Card title="基本信息" size="small">
              <Descriptions column={1} size="small">
                <Descriptions.Item label="调用ID">
                  <Text code>{selectedCall.id}</Text>
                </Descriptions.Item>
                <Descriptions.Item label="函数ID">
                  <Text code>{selectedCall.functionId}</Text>
                </Descriptions.Item>
                <Descriptions.Item label="状态">
                  {getStatusBadge(selectedCall.status)}
                </Descriptions.Item>
                <Descriptions.Item label="用户">{selectedCall.user || '-'}</Descriptions.Item>
                <Descriptions.Item label="开始时间">{selectedCall.startedText}</Descriptions.Item>
                <Descriptions.Item label="结束时间">{selectedCall.completedText}</Descriptions.Item>
                <Descriptions.Item label="执行时长">{selectedCall.durationText}</Descriptions.Item>
                {selectedCall.jobId && (
                  <Descriptions.Item label="任务ID">
                    <Text code>{selectedCall.jobId}</Text>
                  </Descriptions.Item>
                )}
              </Descriptions>
            </Card>

            {/* Context Information */}
            {(selectedCall.gameId || selectedCall.env || selectedCall.agentId) && (
              <Card title="上下文信息" size="small">
                <Descriptions column={1} size="small">
                  {selectedCall.gameId && (
                    <Descriptions.Item label="游戏ID">{selectedCall.gameId}</Descriptions.Item>
                  )}
                  {selectedCall.env && (
                    <Descriptions.Item label="环境">{selectedCall.env}</Descriptions.Item>
                  )}
                  {selectedCall.agentId && (
                    <Descriptions.Item label="Agent ID">
                      <Text code>{selectedCall.agentId}</Text>
                    </Descriptions.Item>
                  )}
                </Descriptions>
              </Card>
            )}

            {/* Request Payload */}
            {selectedCall.payload && (
              <Card title="请求参数" size="small">
                <pre
                  style={{
                    backgroundColor: '#f5f5f5',
                    padding: 12,
                    borderRadius: 4,
                    fontSize: '12px',
                  }}
                >
                  {JSON.stringify(selectedCall.payload, null, 2)}
                </pre>
              </Card>
            )}

            {/* Response Result */}
            {selectedCall.result && (
              <Card title="执行结果" size="small">
                <pre
                  style={{
                    backgroundColor: '#f5f5f5',
                    padding: 12,
                    borderRadius: 4,
                    fontSize: '12px',
                  }}
                >
                  {JSON.stringify(selectedCall.result, null, 2)}
                </pre>
              </Card>
            )}

            {/* Error Information */}
            {selectedCall.error && (
              <Card title="错误信息" size="small">
                <Text type="danger">{selectedCall.error}</Text>
              </Card>
            )}

            {/* Actions */}
            <Space>
              {onRerun && selectedCall.status === 'failed' && (
                <Button
                  type="primary"
                  icon={<RedoOutlined />}
                  onClick={() => handleRerun(selectedCall)}
                >
                  重新运行
                </Button>
              )}
              <Button icon={<ReloadOutlined />} onClick={handleRefresh}>
                刷新历史
              </Button>
            </Space>
          </Space>
        )}
      </Drawer>
    </>
  );
};

export default FunctionCallHistory;
