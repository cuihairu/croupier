import React from 'react';
import {
  Card,
  Descriptions,
  Tag,
  Badge,
  Button,
  Space,
  Typography,
  Divider,
  Timeline,
  Empty,
  Tooltip
} from 'antd';
import {
  PlayCircleOutlined,
  EditOutlined,
  StopOutlined,
  CopyOutlined,
  InfoCircleOutlined
} from '@ant-design/icons';
import { history } from '@umijs/max';

const { Text, Paragraph } = Typography;

export type FunctionDetail = {
  id: string;
  version?: string;
  enabled?: boolean;
  display_name?: { zh?: string; en?: string };
  summary?: { zh?: string; en?: string };
  description?: { zh?: string; en?: string };
  tags?: string[];
  category?: string;
  author?: string;
  created_at?: string;
  updated_at?: string;
  menu?: {
    section?: string;
    group?: string;
    path?: string;
    order?: number;
    hidden?: boolean;
  };
  instances?: {
    agent_id: string;
    service_id: string;
    addr: string;
    version: string;
    status?: string;
  }[];
  params?: any;
  outputs?: any;
  permissions?: string[];
  recent_calls?: Array<{
    timestamp: string;
    user?: string;
    status: 'success' | 'failed' | 'running';
    duration?: number;
  }>;
};

export interface FunctionDetailPanelProps {
  function: FunctionDetail;
  loading?: boolean;
  showActions?: boolean;
  onInvoke?: () => void;
  onEdit?: () => void;
  onToggleStatus?: () => void;
  compact?: boolean;
}

export const FunctionDetailPanel: React.FC<FunctionDetailPanelProps> = ({
  function: func,
  loading = false,
  showActions = true,
  onInvoke,
  onEdit,
  onToggleStatus,
  compact = false
}) => {
  const formatDate = (dateString?: string) => {
    if (!dateString) return '未知';
    return new Date(dateString).toLocaleString('zh-CN');
  };

  const handleCopyId = () => {
    navigator.clipboard.writeText(func.id);
  };

  const getStatusColor = (enabled?: boolean) => {
    return enabled ? 'success' : 'default';
  };

  const getStatusText = (enabled?: boolean) => {
    return enabled ? '启用' : '禁用';
  };

  return (
    <Space direction="vertical" size="middle" style={{ width: '100%' }}>
      {/* Header with basic info and actions */}
      <Card size={compact ? 'small' : 'default'}>
        <Space split={<Divider type="vertical" />} style={{ width: '100%', justifyContent: 'space-between' }}>
          <Space>
            <Badge status={getStatusColor(func.enabled)} />
            <Text strong>{func.display_name?.zh || func.display_name?.en || func.id}</Text>
            {func.version && <Tag color="blue">v{func.version}</Tag>}
            {func.category && <Tag color="geekblue">{func.category}</Tag>}
          </Space>

          {showActions && (
            <Space>
              <Tooltip title="复制函数ID">
                <Button size="small" icon={<CopyOutlined />} onClick={handleCopyId} />
              </Tooltip>
              {onToggleStatus && (
                <Button
                  size="small"
                  type={func.enabled ? 'default' : 'primary'}
                  danger={func.enabled}
                  icon={func.enabled ? <StopOutlined /> : <PlayCircleOutlined />}
                  onClick={onToggleStatus}
                >
                  {func.enabled ? '禁用' : '启用'}
                </Button>
              )}
              {onEdit && (
                <Button size="small" icon={<EditOutlined />} onClick={onEdit}>
                  编辑
                </Button>
              )}
              {onInvoke && (
                <Button type="primary" size="small" icon={<PlayCircleOutlined />} onClick={onInvoke}>
                  调用函数
                </Button>
              )}
            </Space>
          )}
        </Space>
      </Card>

      {/* Basic Information */}
      <Card title="基本信息" size={compact ? 'small' : 'default'}>
        <Descriptions column={compact ? 1 : 2} size={compact ? 'small' : 'default'}>
          <Descriptions.Item label="函数ID">
            <Space>
              <Text code copyable>{func.id}</Text>
            </Space>
          </Descriptions.Item>
          <Descriptions.Item label="状态">
            <Badge
              status={getStatusColor(func.enabled)}
              text={getStatusText(func.enabled)}
            />
          </Descriptions.Item>
          <Descriptions.Item label="版本">
            {func.version || <Text type="secondary">未指定</Text>}
          </Descriptions.Item>
          <Descriptions.Item label="分类">
            <Tag color={func.category ? 'geekblue' : 'default'}>
              {func.category || '未分类'}
            </Tag>
          </Descriptions.Item>
          <Descriptions.Item label="创建时间">
            {formatDate(func.created_at)}
          </Descriptions.Item>
          <Descriptions.Item label="更新时间">
            {formatDate(func.updated_at)}
          </Descriptions.Item>
          {func.author && (
            <Descriptions.Item label="作者">
              {func.author}
            </Descriptions.Item>
          )}
        </Descriptions>
      </Card>

      {/* Description */}
      {(func.summary?.zh || func.summary?.en || func.description?.zh || func.description?.en) && (
        <Card title="函数描述" size={compact ? 'small' : 'default'}>
          <Space direction="vertical" style={{ width: '100%' }}>
            {(func.summary?.zh || func.summary?.en) && (
              <div>
                <Text strong>摘要：</Text>
                <Paragraph>
                  {func.summary?.zh || func.summary?.en}
                </Paragraph>
              </div>
            )}
            {(func.description?.zh || func.description?.en) && (
              <div>
                <Text strong>详细描述：</Text>
                <Paragraph>
                  {func.description?.zh || func.description?.en}
                </Paragraph>
              </div>
            )}
          </Space>
        </Card>
      )}

      {/* Tags */}
      {func.tags && func.tags.length > 0 && (
        <Card title="标签" size={compact ? 'small' : 'default'}>
          <Space wrap>
            {func.tags.map(tag => (
              <Tag key={tag} color="processing">{tag}</Tag>
            ))}
          </Space>
        </Card>
      )}

      {/* Menu Information */}
      {func.menu && (
        <Card title="菜单信息" size={compact ? 'small' : 'default'}>
          <Descriptions column={compact ? 1 : 2} size={compact ? 'small' : 'default'}>
            {func.menu.section && (
              <Descriptions.Item label="分组">{func.menu.section}</Descriptions.Item>
            )}
            {func.menu.group && (
              <Descriptions.Item label="子分组">{func.menu.group}</Descriptions.Item>
            )}
            {func.menu.path && (
              <Descriptions.Item label="路径">
                <Text code>{func.menu.path}</Text>
              </Descriptions.Item>
            )}
            {func.menu.order !== undefined && (
              <Descriptions.Item label="排序">{func.menu.order}</Descriptions.Item>
            )}
            {func.menu.hidden && (
              <Descriptions.Item label="隐藏">
                <Tag color="warning">是</Tag>
              </Descriptions.Item>
            )}
          </Descriptions>
        </Card>
      )}

      {/* Instances Coverage */}
      {func.instances && func.instances.length > 0 && (
        <Card
          title={`实例覆盖 (${func.instances.length} 个)`}
          size={compact ? 'small' : 'default'}
        >
          <Space direction="vertical" style={{ width: '100%' }}>
            {func.instances.map((instance, index) => (
              <Card key={index} size="small" style={{ backgroundColor: '#fafafa' }}>
                <Descriptions column={compact ? 1 : 2} size="small">
                  <Descriptions.Item label="Agent ID">
                    <Text code>{instance.agent_id}</Text>
                  </Descriptions.Item>
                  <Descriptions.Item label="Service ID">
                    <Text code>{instance.service_id}</Text>
                  </Descriptions.Item>
                  <Descriptions.Item label="地址">
                    {instance.addr}
                  </Descriptions.Item>
                  <Descriptions.Item label="版本">
                    <Tag color="blue">{instance.version}</Tag>
                  </Descriptions.Item>
                  {instance.status && (
                    <Descriptions.Item label="状态">
                      <Badge
                        status={instance.status === 'running' ? 'success' : 'default'}
                        text={instance.status}
                      />
                    </Descriptions.Item>
                  )}
                </Descriptions>
              </Card>
            ))}
          </Space>
        </Card>
      )}

      {/* Parameters Schema */}
      {func.params && !compact && (
        <Card title="参数定义" size="default">
          <pre style={{ backgroundColor: '#f5f5f5', padding: 16, borderRadius: 6 }}>
            {JSON.stringify(func.params, null, 2)}
          </pre>
        </Card>
      )}

      {/* Recent Calls */}
      {func.recent_calls && func.recent_calls.length > 0 && (
        <Card title="最近调用" size={compact ? 'small' : 'default'}>
          <Timeline>
            {func.recent_calls.map((call, index) => (
              <Timeline.Item
                key={index}
                color={call.status === 'success' ? 'green' : call.status === 'failed' ? 'red' : 'blue'}
              >
                <Space direction="vertical" size="small">
                  <Space>
                    <Badge
                      status={call.status === 'success' ? 'success' : call.status === 'failed' ? 'error' : 'processing'}
                      text={call.status === 'success' ? '成功' : call.status === 'failed' ? '失败' : '运行中'}
                    />
                    {call.user && <Text type="secondary">by {call.user}</Text>}
                    {call.duration && <Text type="secondary">{call.duration}ms</Text>}
                  </Space>
                  <Text type="secondary">{formatDate(call.timestamp)}</Text>
                </Space>
              </Timeline.Item>
            ))}
          </Timeline>
        </Card>
      )}

      {/* Empty State */}
      {!func.instances && !func.params && !func.recent_calls && (
        <Card size="small">
          <Empty
            image={Empty.PRESENTED_IMAGE_SIMPLE}
            description={
              <Space direction="vertical">
                <Text type="secondary">暂无详细信息</Text>
                {showActions && onInvoke && (
                  <Button type="primary" size="small" icon={<PlayCircleOutlined />} onClick={onInvoke}>
                    调用函数
                  </Button>
                )}
              </Space>
            }
          />
        </Card>
      )}
    </Space>
  );
};

export default FunctionDetailPanel;