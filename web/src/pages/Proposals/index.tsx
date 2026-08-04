/**
 * ProposalsPage - 提案管理页面（三队列模式）
 *
 * 展示和管理页面提案，包括三个队列：
 * - 可直接发布：ready/basic Proposal
 * - 需要处理：needs_review Proposal 和 BlockedProposalIssue
 * - 契约变更：source digest 变化导致 stale 的 Draft/PublishedPageSpec
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
  Modal,
  Descriptions,
  Tabs,
  message,
  Typography,
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
  RocketOutlined,
  ExclamationCircleOutlined,
  SyncOutlined,
} from '@ant-design/icons';
import type { ColumnsType } from 'antd/es/table';
import type {
  PageProposal,
  ProposalStatus,
  ProposalQuality,
  PageType,
} from '@/types/dashboard';
import {
  listProposals,
  getProposal,
  acceptProposal,
  rejectProposal,
} from '@/services/dashboard';

const { Text, Title, Paragraph } = Typography;

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
  expired: <StopOutlined />,
};

const qualityColors: Record<ProposalQuality, string> = {
  ready: 'success',
  basic: 'processing',
  needs_review: 'warning',
};

const qualityLabels: Record<ProposalQuality, string> = {
  ready: '可发布',
  basic: '基础',
  needs_review: '需审核',
};

const pageTypeLabels: Record<PageType, string> = {
  resource: '资源',
  operation: '操作',
  task: '任务',
  report: '报表',
};

const pageTypeColors: Record<PageType, string> = {
  resource: 'blue',
  operation: 'green',
  task: 'orange',
  report: 'purple',
};

const hasErrorDiagnostics = (proposal: PageProposal): boolean =>
  proposal.diagnostics?.some((diagnostic) => diagnostic.severity === 'error') ?? false;

// ---------------------------------------------------------------------------
// 三队列分类
// ---------------------------------------------------------------------------

/** 可直接发布的提案 */
const isPublishable = (proposal: PageProposal): boolean =>
  proposal.status === 'pending' &&
  (proposal.quality === 'ready' || proposal.quality === 'basic') &&
  !hasErrorDiagnostics(proposal);

/** 需要处理的提案 */
const needsHandling = (proposal: PageProposal): boolean =>
  proposal.status === 'pending' &&
  (proposal.quality === 'needs_review' || hasErrorDiagnostics(proposal));

/** 契约变更的提案（stale） */
const isStale = (proposal: PageProposal): boolean =>
  proposal.status === 'accepted' &&
  (proposal.diagnostics?.some((d) => d.code?.includes('stale')) ?? false);

// ---------------------------------------------------------------------------
// ProposalsPage 组件
// ---------------------------------------------------------------------------

const ProposalsPage: React.FC = () => {
  const [loading, setLoading] = useState(false);
  const [data, setData] = useState<PageProposal[]>([]);
  const [activeTab, setActiveTab] = useState<string>('publishable');
  const [query, setQuery] = useState<string>('');
  const [selectedProposal, setSelectedProposal] = useState<PageProposal | null>(null);
  const [detailVisible, setDetailVisible] = useState(false);

  // 加载数据
  const fetchData = useCallback(async () => {
    setLoading(true);
    try {
      const result = await listProposals({});
      setData(result);
    } catch (error) {
      const errMsg = error instanceof Error ? error.message : '未知错误';
      message.error('加载失败: ' + errMsg);
    } finally {
      setLoading(false);
    }
  }, []);

  // 初始加载
  useEffect(() => {
    fetchData();
  }, [fetchData]);

  // 按队列过滤数据
  const publishableProposals = data.filter(isPublishable);
  const needsHandlingProposals = data.filter(needsHandling);
  const staleProposals = data.filter(isStale);

  // 查看详情
  const handleViewDetail = useCallback(async (proposalKey: string) => {
    try {
      const detail = await getProposal(proposalKey);
      setSelectedProposal(detail);
      setDetailVisible(true);
    } catch (error) {
      const errMsg = error instanceof Error ? error.message : '未知错误';
      message.error('获取详情失败: ' + errMsg);
    }
  }, []);

  // 接受提案
  const handleAccept = useCallback(
    async (proposalKey: string) => {
      try {
        await acceptProposal(proposalKey);
        message.success('提案已接受');
        fetchData();
      } catch (error) {
        const errMsg = error instanceof Error ? error.message : '未知错误';
        message.error('接受失败: ' + errMsg);
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
      } catch (error) {
        const errMsg = error instanceof Error ? error.message : '未知错误';
        message.error('拒绝失败: ' + errMsg);
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
      render: (type: PageType) => (
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
      render: (diagnostics: Record<string, unknown>[]) => {
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
          {record.status === 'pending' && !hasErrorDiagnostics(record) && (
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
          <Button icon={<ReloadOutlined />} onClick={fetchData}>
            刷新
          </Button>
        </Space>
      </Card>

      {/* 三队列 Tabs */}
      <Card>
        <Tabs
          activeKey={activeTab}
          onChange={setActiveTab}
          items={[
            {
              key: 'publishable',
              label: (
                <Space>
                  <RocketOutlined />
                  可直接发布
                  <Tag color="success">{publishableProposals.length}</Tag>
                </Space>
              ),
              children: (
                <Table
                  columns={columns}
                  dataSource={publishableProposals}
                  rowKey="proposalKey"
                  loading={loading}
                  pagination={{ pageSize: 20 }}
                />
              ),
            },
            {
              key: 'needsHandling',
              label: (
                <Space>
                  <ExclamationCircleOutlined />
                  需要处理
                  <Tag color="warning">{needsHandlingProposals.length}</Tag>
                </Space>
              ),
              children: (
                <Table
                  columns={columns}
                  dataSource={needsHandlingProposals}
                  rowKey="proposalKey"
                  loading={loading}
                  pagination={{ pageSize: 20 }}
                />
              ),
            },
            {
              key: 'stale',
              label: (
                <Space>
                  <SyncOutlined />
                  契约变更
                  <Tag color="error">{staleProposals.length}</Tag>
                </Space>
              ),
              children: (
                <Table
                  columns={columns}
                  dataSource={staleProposals}
                  rowKey="proposalKey"
                  loading={loading}
                  pagination={{ pageSize: 20 }}
                />
              ),
            },
          ]}
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
            {selectedProposal.status === 'pending' && !hasErrorDiagnostics(selectedProposal) && (
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
