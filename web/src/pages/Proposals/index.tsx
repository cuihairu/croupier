/**
 * ProposalsPage - 提案管理页面
 *
 * 展示和管理页面提案，包括：
 * - 提案列表
 * - 提案详情
 * - 接受/拒绝操作
 *
 * @module pages/Proposals
 */

import React, { useState, useCallback, useEffect } from 'react';
import {
  Card,
  Table,
  Tag,
  Space,
  Button,
  Input,
  Select,
  Modal,
  Descriptions,
  message,
  Typography,
  Tooltip,
  Badge,
  Popconfirm,
} from 'antd';
import {
  SearchOutlined,
  ReloadOutlined,
  EyeOutlined,
  CheckOutlined,
  CloseOutlined,
  FileTextOutlined,
  WarningOutlined,
  CheckCircleOutlined,
  ClockCircleOutlined,
  StopOutlined,
} from '@ant-design/icons';
import type { ColumnsType } from 'antd/es/table';
import type {
  PageProposal,
  ProposalStatus,
  ProposalQuality,
  PageTypeV2,
} from '@/types/dashboard-vnext';
import {
  listProposals,
  getProposal,
  acceptProposal,
  rejectProposal,
} from '@/services/dashboard-vnext';

const { Text, Title, Paragraph } = Typography;
const { Option } = Select;

// ---------------------------------------------------------------------------
// 状态颜色映射
// ---------------------------------------------------------------------------

const statusColors: Record<ProposalStatus, string> = {
  pending: 'processing',
  accepted: 'success',
  rejected: 'error',
  expired: 'default',
};

const statusLabels: Record<ProposalStatus, string> = {
  pending: '待处理',
  accepted: '已接受',
  rejected: '已拒绝',
  expired: '已过期',
};

const statusIcons: Record<ProposalStatus, React.ReactNode> = {
  pending: <ClockCircleOutlined />,
  accepted: <CheckCircleOutlined />,
  rejected: <StopOutlined />,
  expired: <WarningOutlined />,
};

const qualityColors: Record<ProposalQuality, string> = {
  ready: 'success',
  basic: 'processing',
  needs_review: 'warning',
  blocked: 'error',
};

const qualityLabels: Record<ProposalQuality, string> = {
  ready: '可发布',
  basic: '基础',
  needs_review: '需审核',
  blocked: '阻断',
};

const pageTypeLabels: Record<PageTypeV2, string> = {
  resource: '资源',
  operation: '操作',
  task: '任务',
  report: '报表',
};

const pageTypeColors: Record<PageTypeV2, string> = {
  resource: 'blue',
  operation: 'green',
  task: 'orange',
  report: 'purple',
};

// ---------------------------------------------------------------------------
// ProposalsPage 组件
// ---------------------------------------------------------------------------

const ProposalsPage: React.FC = () => {
  const [loading, setLoading] = useState(false);
  const [data, setData] = useState<PageProposal[]>([]);
  const [status, setStatus] = useState<string>('');
  const [query, setQuery] = useState<string>('');
  const [selectedProposal, setSelectedProposal] = useState<PageProposal | null>(null);
  const [detailVisible, setDetailVisible] = useState(false);

  // 加载数据
  const fetchData = useCallback(async () => {
    setLoading(true);
    try {
      const result = await listProposals({
        status: status || undefined,
      });
      setData(result);
    } catch (error: any) {
      message.error('加载失败: ' + (error.message || '未知错误'));
    } finally {
      setLoading(false);
    }
  }, [status]);

  // 初始加载
  useEffect(() => {
    fetchData();
  }, [fetchData]);

  // 查看详情
  const handleViewDetail = useCallback(async (proposalKey: string) => {
    try {
      const detail = await getProposal(proposalKey);
      setSelectedProposal(detail);
      setDetailVisible(true);
    } catch (error: any) {
      message.error('获取详情失败: ' + (error.message || '未知错误'));
    }
  }, []);

  // 接受提案
  const handleAccept = useCallback(
    async (proposalKey: string) => {
      try {
        await acceptProposal(proposalKey);
        message.success('提案已接受');
        fetchData();
      } catch (error: any) {
        message.error('接受失败: ' + (error.message || '未知错误'));
      }
    },
    [fetchData]
  );

  // 拒绝提案
  const handleReject = useCallback(
    async (proposalKey: string) => {
      try {
        await rejectProposal(proposalKey);
        message.success('提案已拒绝');
        fetchData();
      } catch (error: any) {
        message.error('拒绝失败: ' + (error.message || '未知错误'));
      }
    },
    [fetchData]
  );

  // 表格列定义
  const columns: ColumnsType<PageProposal> = [
    {
      title: '提案标识',
      dataIndex: 'proposalKey',
      key: 'proposalKey',
      render: (text) => <Text strong>{text}</Text>,
    },
    {
      title: '页面标识',
      dataIndex: 'pageKey',
      key: 'pageKey',
    },
    {
      title: '页面类型',
      dataIndex: 'pageType',
      key: 'pageType',
      render: (type: PageTypeV2) => (
        <Tag color={pageTypeColors[type]}>
          {pageTypeLabels[type] || type}
        </Tag>
      ),
    },
    {
      title: '资源',
      dataIndex: 'resourceKey',
      key: 'resourceKey',
      render: (text) => text || '-',
    },
    {
      title: '质量',
      dataIndex: 'quality',
      key: 'quality',
      render: (quality: ProposalQuality) => (
        <Tag color={qualityColors[quality]}>
          {qualityLabels[quality] || quality}
        </Tag>
      ),
    },
    {
      title: '状态',
      dataIndex: 'status',
      key: 'status',
      render: (status: ProposalStatus) => (
        <Tag color={statusColors[status]} icon={statusIcons[status]}>
          {statusLabels[status] || status}
        </Tag>
      ),
    },
    {
      title: '诊断',
      dataIndex: 'diagnostics',
      key: 'diagnostics',
      render: (diagnostics: any[]) => {
        if (!diagnostics || diagnostics.length === 0) {
          return <Tag color="success">无</Tag>;
        }
        const errors = diagnostics.filter((d) => d.severity === 'error').length;
        const warnings = diagnostics.filter((d) => d.severity === 'warning').length;
        return (
          <Space>
            {errors > 0 && <Tag color="error">{errors} 错误</Tag>}
            {warnings > 0 && <Tag color="warning">{warnings} 警告</Tag>}
          </Space>
        );
      },
    },
    {
      title: '更新时间',
      dataIndex: 'updatedAt',
      key: 'updatedAt',
      render: (text) => text ? new Date(text).toLocaleString() : '-',
    },
    {
      title: '操作',
      key: 'action',
      render: (_, record) => (
        <Space>
          <Button
            type="link"
            icon={<EyeOutlined />}
            onClick={() => handleViewDetail(record.proposalKey)}
          >
            查看
          </Button>
          {record.status === 'pending' && record.quality !== 'blocked' && (
            <>
              <Popconfirm
                title="确定要接受此提案吗？"
                onConfirm={() => handleAccept(record.proposalKey)}
              >
                <Button type="link" icon={<CheckOutlined />} style={{ color: '#52c41a' }}>
                  接受
                </Button>
              </Popconfirm>
              <Popconfirm
                title="确定要拒绝此提案吗？"
                onConfirm={() => handleReject(record.proposalKey)}
              >
                <Button type="link" danger icon={<CloseOutlined />}>
                  拒绝
                </Button>
              </Popconfirm>
            </>
          )}
        </Space>
      ),
    },
  ];

  return (
    <div>
      {/* 搜索栏 */}
      <Card style={{ marginBottom: 16 }}>
        <Space>
          <Input
            placeholder="搜索提案"
            prefix={<SearchOutlined />}
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            style={{ width: 200 }}
          />
          <Select
            placeholder="选择状态"
            value={status || undefined}
            onChange={(value) => setStatus(value || '')}
            allowClear
            style={{ width: 150 }}
          >
            <Option value="">全部</Option>
            <Option value="pending">待处理</Option>
            <Option value="accepted">已接受</Option>
            <Option value="rejected">已拒绝</Option>
          </Select>
          <Button type="primary" icon={<SearchOutlined />} onClick={fetchData}>
            搜索
          </Button>
          <Button icon={<ReloadOutlined />} onClick={fetchData}>
            刷新
          </Button>
        </Space>
      </Card>

      {/* 提案列表 */}
      <Card title="提案管理">
        <Table
          columns={columns}
          dataSource={data}
          rowKey="proposalKey"
          loading={loading}
          pagination={{
            pageSize: 20,
            showSizeChanger: true,
            showTotal: (total) => `共 ${total} 条`,
          }}
        />
      </Card>

      {/* 详情弹窗 */}
      <Modal
        title="提案详情"
        open={detailVisible}
        onCancel={() => setDetailVisible(false)}
        footer={null}
        width={900}
      >
        {selectedProposal && (
          <div>
            <Descriptions column={2} bordered>
              <Descriptions.Item label="提案标识">{selectedProposal.proposalKey}</Descriptions.Item>
              <Descriptions.Item label="页面标识">{selectedProposal.pageKey}</Descriptions.Item>
              <Descriptions.Item label="页面类型">
                <Tag color={pageTypeColors[selectedProposal.pageType]}>
                  {pageTypeLabels[selectedProposal.pageType]}
                </Tag>
              </Descriptions.Item>
              <Descriptions.Item label="资源">{selectedProposal.resourceKey || '-'}</Descriptions.Item>
              <Descriptions.Item label="质量">
                <Tag color={qualityColors[selectedProposal.quality]}>
                  {qualityLabels[selectedProposal.quality]}
                </Tag>
              </Descriptions.Item>
              <Descriptions.Item label="状态">
                <Tag color={statusColors[selectedProposal.status]} icon={statusIcons[selectedProposal.status]}>
                  {statusLabels[selectedProposal.status]}
                </Tag>
              </Descriptions.Item>
              <Descriptions.Item label="生成器版本">{selectedProposal.generatorVersion}</Descriptions.Item>
              <Descriptions.Item label="更新时间">
                {selectedProposal.updatedAt ? new Date(selectedProposal.updatedAt).toLocaleString() : '-'}
              </Descriptions.Item>
            </Descriptions>

            {/* 标题和描述 */}
            {selectedProposal.title && (
              <div style={{ marginTop: 16 }}>
                <Title level={5}>标题</Title>
                <Paragraph>{selectedProposal.title['zh-CN'] || selectedProposal.title['en'] || '-'}</Paragraph>
              </div>
            )}

            {selectedProposal.description && (
              <div style={{ marginTop: 16 }}>
                <Title level={5}>描述</Title>
                <Paragraph>{selectedProposal.description['zh-CN'] || selectedProposal.description['en'] || '-'}</Paragraph>
              </div>
            )}

            {/* 诊断信息 */}
            {selectedProposal.diagnostics && selectedProposal.diagnostics.length > 0 && (
              <div style={{ marginTop: 16 }}>
                <Title level={5}>
                  <WarningOutlined /> 诊断信息
                </Title>
                <Table
                  dataSource={selectedProposal.diagnostics}
                  rowKey="code"
                  pagination={false}
                  size="small"
                  columns={[
                    { title: '代码', dataIndex: 'code', key: 'code' },
                    {
                      title: '级别',
                      dataIndex: 'severity',
                      key: 'severity',
                      render: (text) => (
                        <Tag
                          color={
                            text === 'error' ? 'red' : text === 'warning' ? 'orange' : 'blue'
                          }
                        >
                          {text}
                        </Tag>
                      ),
                    },
                    { title: '消息', dataIndex: 'message', key: 'message' },
                  ]}
                />
              </div>
            )}

            {/* 页面规格预览 */}
            {selectedProposal.pageSpec && (
              <div style={{ marginTop: 16 }}>
                <Title level={5}>
                  <FileTextOutlined /> 页面规格
                </Title>
                <pre
                  style={{
                    maxHeight: 300,
                    overflow: 'auto',
                    background: '#f5f5f5',
                    padding: 12,
                    borderRadius: 4,
                  }}
                >
                  {JSON.stringify(selectedProposal.pageSpec, null, 2)}
                </pre>
              </div>
            )}

            {/* 操作按钮 */}
            {selectedProposal.status === 'pending' && selectedProposal.quality !== 'blocked' && (
              <div style={{ marginTop: 16, textAlign: 'right' }}>
                <Space>
                  <Popconfirm
                    title="确定要接受此提案吗？"
                    onConfirm={() => {
                      handleAccept(selectedProposal.proposalKey);
                      setDetailVisible(false);
                    }}
                  >
                    <Button type="primary" icon={<CheckOutlined />}>
                      接受提案
                    </Button>
                  </Popconfirm>
                  <Popconfirm
                    title="确定要拒绝此提案吗？"
                    onConfirm={() => {
                      handleReject(selectedProposal.proposalKey);
                      setDetailVisible(false);
                    }}
                  >
                    <Button danger icon={<CloseOutlined />}>
                      拒绝提案
                    </Button>
                  </Popconfirm>
                </Space>
              </div>
            )}
          </div>
        )}
      </Modal>
    </div>
  );
};

export default ProposalsPage;
