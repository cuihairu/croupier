import React, { useEffect, useState, useMemo } from 'react';
import { Timeline, Badge, Tag, Space, Typography, Button, Drawer, Descriptions, Card, Empty, Tooltip } from 'antd';
import {
  PlayCircleOutlined,
  CheckCircleOutlined,
  CloseCircleOutlined,
  ClockCircleOutlined,
  EyeOutlined,
  ReloadOutlined,
  RedoOutlined
} from '@ant-design/icons';
import { request } from '@umijs/max';

const { Text, Title, Paragraph } = Typography;

export type FunctionCall = {
  id: string;
  function_id: string;
  user?: string;
  status: 'success' | 'failed' | 'running' | 'cancelled';
  started_at: string;
  completed_at?: string;
  duration?: number;
  payload?: any;
  result?: any;
  error?: string;
  agent_id?: string;
  game_id?: string;
  env?: string;
  job_id?: string;
};

export interface FunctionCallHistoryProps {
  functionId?: string;
  userId?: string;
  gameId?: string;
  limit?: number;
  showRefresh?: boolean;
  compact?: boolean;
  onRefresh?: () => void;
  onViewDetail?: (call: FunctionCall) => void;
  onRerun?: (call: FunctionCall) => void;
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
  onRerun
}) => {
  const [calls, setCalls] = useState<FunctionCall[]>([]);
  const [loading, setLoading] = useState(false);
  const [detailVisible, setDetailVisible] = useState(false);
  const [selectedCall, setSelectedCall] = useState<FunctionCall | null>(null);

  const fetchCalls = async () => {
    setLoading(true);
    try {
      const params: any = { limit };
      if (functionId) params.function_id = functionId;
      if (userId) params.user_id = userId;
      if (gameId) params.game_id = gameId;

      const res: any = await request('/api/function_calls', { params });
      setCalls(res?.calls || []);
    } catch (error) {
      console.error('Failed to fetch function calls:', error);
      setCalls([]);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => { fetchCalls(); }, [functionId, userId, gameId, limit]);

  const handleRefresh = () => {
    fetchCalls();
    onRefresh?.();
  };

  const handleViewDetail = (call: FunctionCall) => {
    setSelectedCall(call);
    setDetailVisible(true);
    onViewDetail?.(call);
  };

  const handleRerun = (call: FunctionCall) => {
    if (call.job_id) {
      // Try to rerun the job
      request(`/api/function_calls/${call.id}/rerun`, { method: 'POST' })
        .then(() => {
          handleRefresh();
        })
        .catch(console.error);
    }
    onRerun?.(call);
  };

  const getStatusIcon = (status: string) => {
    switch (status) {
      case 'success': return <CheckCircleOutlined style={{ color: '#52c41a' }} />;
      case 'failed': return <CloseCircleOutlined style={{ color: '#ff4d4f' }} />;
      case 'running': return <PlayCircleOutlined style={{ color: '#1890ff' }} />;
      case 'cancelled': return <ClockCircleOutlined style={{ color: '#faad14' }} />;
      default: return <ClockCircleOutlined style={{ color: '#d9d9d9' }} />;
    }
  };

  const getStatusText = (status: string) => {
    switch (status) {
      case 'success': return '成功';
      case 'failed': return '失败';
      case 'running': return '运行中';
      case 'cancelled': return '已取消';
      default: return '未知';
    }
  };

  const getStatusBadge = (status: string) => {
    const statusConfig = {
      success: { status: 'success' as const, text: '成功' },
      failed: { status: 'error' as const, text: '失败' },
      running: { status: 'processing' as const, text: '运行中' },
      cancelled: { status: 'warning' as const, text: '已取消' }
    };

    const config = statusConfig[status as keyof typeof statusConfig] || { status: 'default' as const, text: '未知' };
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

  const processedCalls = useMemo(() => {
    return calls.map(call => ({
      ...call,
      durationText: formatDuration(call.duration),
      startedText: formatDate(call.started_at),
      completedText: call.completed_at ? formatDate(call.completed_at) : '-'
    }));
  }, [calls]);

  if (calls.length === 0 && !loading) {
    return (
      <Card size={compact ? 'small' : 'default'}>
        <Empty
          image={Empty.PRESENTED_IMAGE_SIMPLE}
          description="暂无调用记录"
        >
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
              color={call.status === 'success' ? 'green' : call.status === 'failed' ? 'red' : 'blue'}
            >
              <Card size="small" style={{ marginBottom: 8, cursor: 'pointer' }} onClick={() => handleViewDetail(call)}>
                <Space direction="vertical" style={{ width: '100%' }}>
                  <Space style={{ justifyContent: 'space-between', width: '100%' }}>
                    <Space>
                      {getStatusBadge(call.status)}
                      <Text code>{call.function_id}</Text>
                      {call.user && <Text type="secondary">by {call.user}</Text>}
                    </Space>
                    <Space>
                      {call.duration && <Tag color="blue">{call.durationText}</Tag>}
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
                  {(call.game_id || call.env || call.agent_id) && (
                    <Space wrap>
                      {call.game_id && <Tag size="small">Game: {call.game_id}</Tag>}
                      {call.env && <Tag size="small">Env: {call.env}</Tag>}
                      {call.agent_id && <Tag size="small">Agent: {call.agent_id}</Tag>}
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
                  <Text code>{selectedCall.function_id}</Text>
                </Descriptions.Item>
                <Descriptions.Item label="状态">
                  {getStatusBadge(selectedCall.status)}
                </Descriptions.Item>
                <Descriptions.Item label="用户">
                  {selectedCall.user || '-'}
                </Descriptions.Item>
                <Descriptions.Item label="开始时间">
                  {selectedCall.startedText}
                </Descriptions.Item>
                <Descriptions.Item label="结束时间">
                  {selectedCall.completedText}
                </Descriptions.Item>
                <Descriptions.Item label="执行时长">
                  {selectedCall.durationText}
                </Descriptions.Item>
                {selectedCall.job_id && (
                  <Descriptions.Item label="任务ID">
                    <Text code>{selectedCall.job_id}</Text>
                  </Descriptions.Item>
                )}
              </Descriptions>
            </Card>

            {/* Context Information */}
            {(selectedCall.game_id || selectedCall.env || selectedCall.agent_id) && (
              <Card title="上下文信息" size="small">
                <Descriptions column={1} size="small">
                  {selectedCall.game_id && (
                    <Descriptions.Item label="游戏ID">{selectedCall.game_id}</Descriptions.Item>
                  )}
                  {selectedCall.env && (
                    <Descriptions.Item label="环境">{selectedCall.env}</Descriptions.Item>
                  )}
                  {selectedCall.agent_id && (
                    <Descriptions.Item label="Agent ID">
                      <Text code>{selectedCall.agent_id}</Text>
                    </Descriptions.Item>
                  )}
                </Descriptions>
              </Card>
            )}

            {/* Request Payload */}
            {selectedCall.payload && (
              <Card title="请求参数" size="small">
                <pre style={{ backgroundColor: '#f5f5f5', padding: 12, borderRadius: 4, fontSize: '12px' }}>
                  {JSON.stringify(selectedCall.payload, null, 2)}
                </pre>
              </Card>
            )}

            {/* Response Result */}
            {selectedCall.result && (
              <Card title="执行结果" size="small">
                <pre style={{ backgroundColor: '#f5f5f5', padding: 12, borderRadius: 4, fontSize: '12px' }}>
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
                <Button type="primary" icon={<RedoOutlined />} onClick={() => handleRerun(selectedCall)}>
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