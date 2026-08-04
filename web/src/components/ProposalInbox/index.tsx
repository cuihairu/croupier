/**
 * ProposalInbox - Page Studio 的唯一三队列入口。
 *
 * 队列数据由后端聚合；前端不从 Proposal quality 推断 stale/blocked。
 */

import React, { useCallback, useEffect, useState } from 'react';
import {
  Alert,
  Button,
  Card,
  Descriptions,
  Empty,
  Input,
  Modal,
  Popconfirm,
  Space,
  Table,
  Tabs,
  Tag,
  Typography,
} from 'antd';
import {
  CheckOutlined,
  CloseOutlined,
  ExclamationCircleOutlined,
  EyeOutlined,
  FileTextOutlined,
  ReloadOutlined,
  RocketOutlined,
  SearchOutlined,
  SyncOutlined,
} from '@ant-design/icons';
import type { ColumnsType } from 'antd/es/table';
import type {
  BlockedProposalIssue,
  ContractChangeInfo,
  DiagnosticInfo,
  PageProposal,
  PageType,
  ProposalInbox as ProposalInboxData,
  ProposalQuality,
  ProposalStatus,
} from '@/types/dashboard';
import {
  acceptAndPublishProposal,
  acceptProposal,
  getProposal,
  listProposalInbox,
  mergeChanges,
  regenerateProposal,
  rejectProposal,
  republish,
} from '@/services/dashboard';
import { publishPageDraft } from '@/services/api/pages';
import PageRenderer from '@/components/PageRenderer';

const { Paragraph, Text } = Typography;

const emptyInbox: ProposalInboxData = {
  publishable: [],
  needsReview: [],
  blockedIssues: [],
  contractChanges: [],
  summary: {
    publishable: 0,
    needsReview: 0,
    blockedIssues: 0,
    contractChanges: 0,
  },
};

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

const qualityColors: Record<ProposalQuality, string> = {
  ready: 'success',
  basic: 'processing',
  needs_review: 'warning',
};

const qualityLabels: Record<ProposalQuality, string> = {
  ready: '可直接发布',
  basic: '基础可发布',
  needs_review: '需要处理',
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

function localizedText(text: Record<string, string> | undefined, fallback: string): string {
  if (!text) return fallback;
  return text['zh-CN'] || text['en-US'] || Object.values(text).find((value) => value.trim()) || fallback;
}

function formatDate(value?: string): string {
  if (!value) return '-';
  const time = new Date(value);
  return Number.isNaN(time.getTime()) ? value : time.toLocaleString();
}

function diagnosticSummary(diagnostics?: DiagnosticInfo[]): React.ReactNode {
  if (!diagnostics || diagnostics.length === 0) {
    return <Tag color="success">无</Tag>;
  }
  const errors = diagnostics.filter((item) => item.severity === 'error').length;
  const warnings = diagnostics.filter((item) => item.severity === 'warning').length;
  const infos = diagnostics.filter((item) => item.severity === 'info').length;
  return (
    <Space>
      {errors > 0 && <Tag color="error">{errors} 错误</Tag>}
      {warnings > 0 && <Tag color="warning">{warnings} 警告</Tag>}
      {infos > 0 && <Tag color="blue">{infos} 信息</Tag>}
    </Space>
  );
}

function matchesQuery(proposal: PageProposal, query: string): boolean {
  const keyword = query.trim().toLowerCase();
  if (!keyword) return true;
  return [
    proposal.proposalKey,
    proposal.pageKey,
    proposal.resourceKey || '',
    localizedText(proposal.title, ''),
  ]
    .join(' ')
    .toLowerCase()
    .includes(keyword);
}

function currentResourceKey(): string {
  if (typeof window === 'undefined') {
    return '';
  }
  return new URLSearchParams(window.location.search).get('resourceKey') || '';
}

function navigateTo(path: string) {
  if (typeof window !== 'undefined') {
    window.location.assign(path);
  }
}

export default function ProposalInbox() {
  const [loading, setLoading] = useState(false);
  const [inbox, setInbox] = useState<ProposalInboxData>(emptyInbox);
  const [query, setQuery] = useState('');
  const [activeTab, setActiveTab] = useState('publishable');
  const [selectedProposal, setSelectedProposal] = useState<PageProposal | null>(null);
  const [detailVisible, setDetailVisible] = useState(false);
  const [previewVisible, setPreviewVisible] = useState(false);
  const [resourceKey] = useState(currentResourceKey);
  const [contractActionKey, setContractActionKey] = useState('');

  const fetchData = useCallback(async () => {
    setLoading(true);
    try {
      const result = await listProposalInbox({ resourceKey: resourceKey || undefined });
      setInbox(result);
    } finally {
      setLoading(false);
    }
  }, [resourceKey]);

  useEffect(() => {
    fetchData();
  }, [fetchData]);

  const handleViewDetail = useCallback(async (proposalKey: string) => {
    const detail = await getProposal(proposalKey);
    setSelectedProposal(detail);
    setDetailVisible(true);
  }, []);

  const handlePreview = useCallback(async (proposalKey: string) => {
    const detail = await getProposal(proposalKey);
    setSelectedProposal(detail);
    setPreviewVisible(true);
  }, []);

  const handleAccept = useCallback(
    async (proposalKey: string) => {
      await acceptProposal(proposalKey);
      await fetchData();
    },
    [fetchData],
  );

  const handleAcceptAndPublish = useCallback(
    async (proposalKey: string) => {
      const result = await acceptAndPublishProposal(proposalKey);
      await fetchData();
      Modal.success({
        title: '已直接发布',
        content: `页面 ${result.pageKey} 已发布，版本 ${result.publishedVersion}。运行控制台菜单会从已发布快照生成。`,
      });
    },
    [fetchData],
  );

  const handleReject = useCallback(
    async (proposalKey: string) => {
      await rejectProposal(proposalKey);
      await fetchData();
    },
    [fetchData],
  );

  const handleReviewProposal = useCallback(
    async (proposal: PageProposal) => {
      if (proposal.resourceKey) {
        navigateTo(`/system/functions/resource-catalog?resourceKey=${encodeURIComponent(proposal.resourceKey)}`);
        return;
      }
      await handleViewDetail(proposal.proposalKey);
    },
    [handleViewDetail],
  );

  const runContractAction = useCallback(
    async (key: string, action: () => Promise<void>) => {
      setContractActionKey(key);
      try {
        await action();
        await fetchData();
      } finally {
        setContractActionKey('');
      }
    },
    [fetchData],
  );

  const handleRegenerateProposal = useCallback(
    async (record: ContractChangeInfo) => {
      if (!record.resourceKey) {
        Modal.warning({
          title: '无法按资源重新生成',
          content: '该页面没有 resourceKey，请进入页面编辑器手动处理契约变化。',
        });
        return;
      }
      await runContractAction(`regenerate:${record.pageKey}`, async () => {
        const result = await regenerateProposal(record.resourceKey || '');
        Modal.success({ title: '已重新生成 Proposal', content: result.message });
      });
    },
    [runContractAction],
  );

  const handleAutoMerge = useCallback(
    async (record: ContractChangeInfo) => {
      if (!record.resourceKey) {
        Modal.warning({
          title: '无法自动合并',
          content: '当前自动合并只支持 ResourcePage。请进入页面编辑器处理 Operation/Task/Report 的契约变化。',
        });
        return;
      }
      await runContractAction(`merge:${record.pageKey}`, async () => {
        const result = await mergeChanges(record.resourceKey || '', { strategy: 'auto' });
        Modal.info({
          title: '自动合并结果',
          content: `${result.message}。安全合并 ${result.merged} 项，仍有 ${result.conflicts} 项需要人工处理。`,
        });
      });
    },
    [runContractAction],
  );

  const handleRepublish = useCallback(
    async (record: ContractChangeInfo) => {
      await runContractAction(`republish:${record.pageKey}`, async () => {
        if (record.draftRevision && record.draftRevision > 0) {
          const result = await publishPageDraft(record.pageKey, record.draftRevision);
          Modal.success({
            title: '已重新发布',
            content: `页面 ${result.pageKey} 已发布，版本 ${result.publishedVersion}。`,
          });
          return;
        }
        if (record.resourceKey) {
          const result = await republish(record.resourceKey);
          Modal.success({ title: '已重新发布', content: result.message });
          return;
        }
        Modal.warning({
          title: '无法直接重新发布',
          content: '该页面缺少草稿版本，请进入页面编辑器处理后发布。',
        });
      });
    },
    [runContractAction],
  );

  const proposalColumns: ColumnsType<PageProposal> = [
    {
      title: '提案',
      dataIndex: 'proposalKey',
      key: 'proposalKey',
      render: (_, record) => (
        <Space direction="vertical" size={0}>
          <Text strong>{record.proposalKey}</Text>
          <Text type="secondary">{record.pageKey}</Text>
        </Space>
      ),
    },
    {
      title: '标题',
      dataIndex: 'title',
      key: 'title',
      render: (_, record) => localizedText(record.title, record.pageKey),
    },
    {
      title: '类型',
      dataIndex: 'pageType',
      key: 'pageType',
      width: 90,
      render: (type: PageType) => <Tag color={pageTypeColors[type]}>{pageTypeLabels[type] || type}</Tag>,
    },
    {
      title: '资源',
      dataIndex: 'resourceKey',
      key: 'resourceKey',
      width: 140,
      render: (value) => value || '-',
    },
    {
      title: '质量',
      dataIndex: 'quality',
      key: 'quality',
      width: 120,
      render: (quality: ProposalQuality) => <Tag color={qualityColors[quality]}>{qualityLabels[quality]}</Tag>,
    },
    {
      title: '状态',
      dataIndex: 'status',
      key: 'status',
      width: 100,
      render: (status: ProposalStatus) => <Tag color={statusColors[status]}>{statusLabels[status]}</Tag>,
    },
    {
      title: '诊断',
      dataIndex: 'diagnostics',
      key: 'diagnostics',
      width: 160,
      render: diagnosticSummary,
    },
    {
      title: '更新时间',
      dataIndex: 'updatedAt',
      key: 'updatedAt',
      width: 180,
      render: formatDate,
    },
    {
      title: '操作',
      key: 'action',
      width: 260,
      render: (_, record) => (
        <Space>
          <Button type="link" icon={<EyeOutlined />} onClick={() => handleViewDetail(record.proposalKey)}>
            查看
          </Button>
          <Button type="link" icon={<FileTextOutlined />} onClick={() => handlePreview(record.proposalKey)}>
            预览
          </Button>
          {record.status === 'pending' && (
            <>
              {(record.quality === 'ready' || record.quality === 'basic') && (
                <Popconfirm
                  title="直接发布默认页面？"
                  description="会创建草稿并发布到运行控制台左侧动态菜单。"
                  onConfirm={() => handleAcceptAndPublish(record.proposalKey)}
                >
                  <Button type="link" icon={<RocketOutlined />}>
                    发布
                  </Button>
                </Popconfirm>
              )}
              <Popconfirm title="接受为草稿？" onConfirm={() => handleAccept(record.proposalKey)}>
                <Button type="link" icon={<CheckOutlined />}>
                  接受
                </Button>
              </Popconfirm>
              {record.quality === 'needs_review' && (
                <Button type="link" icon={<ExclamationCircleOutlined />} onClick={() => handleReviewProposal(record)}>
                  处理
                </Button>
              )}
              <Popconfirm title="拒绝此提案？" onConfirm={() => handleReject(record.proposalKey)}>
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

  const blockedColumns: ColumnsType<BlockedProposalIssue> = [
    {
      title: '阻断项',
      dataIndex: 'id',
      key: 'id',
      render: (_, record) => (
        <Space direction="vertical" size={0}>
          <Text strong>{record.functionId || record.resourceKey || `issue-${record.id}`}</Text>
          <Text type="secondary">{record.resourceKey || '-'}</Text>
        </Space>
      ),
    },
    {
      title: '修复提示',
      dataIndex: 'repairHint',
      key: 'repairHint',
      render: (_, record) => localizedText(record.repairHint, '-'),
    },
    {
      title: '诊断',
      dataIndex: 'diagnostics',
      key: 'diagnostics',
      width: 160,
      render: diagnosticSummary,
    },
    {
      title: '更新时间',
      dataIndex: 'updatedAt',
      key: 'updatedAt',
      width: 180,
      render: formatDate,
    },
    {
      title: '操作',
      key: 'action',
      width: 120,
      render: (_, record) =>
        record.resourceKey ? (
          <Button
            type="link"
            icon={<ExclamationCircleOutlined />}
            onClick={() => navigateTo(`/system/functions/resource-catalog?resourceKey=${encodeURIComponent(record.resourceKey || '')}`)}
          >
            修复语义
          </Button>
        ) : (
          <Button type="link" icon={<ExclamationCircleOutlined />} onClick={() => navigateTo('/system/functions/resource-catalog')}>
            查看目录
          </Button>
        ),
    },
  ];

  const contractColumns: ColumnsType<ContractChangeInfo> = [
    {
      title: '页面',
      dataIndex: 'pageKey',
      key: 'pageKey',
      render: (_, record) => (
        <Space direction="vertical" size={0}>
          <Text strong>{localizedText(record.title, record.pageKey)}</Text>
          <Text type="secondary">{record.pageKey}</Text>
        </Space>
      ),
    },
    {
      title: '类型',
      dataIndex: 'pageType',
      key: 'pageType',
      width: 90,
      render: (type: PageType) => <Tag color={pageTypeColors[type]}>{pageTypeLabels[type] || type}</Tag>,
    },
    {
      title: '对象',
      dataIndex: 'resourceKey',
      key: 'resourceKey',
      width: 140,
      render: (value) => value || '-',
    },
    {
      title: '位置',
      dataIndex: 'kind',
      key: 'kind',
      width: 100,
      render: (kind: ContractChangeInfo['kind']) => (
        <Tag color={kind === 'published' ? 'error' : 'warning'}>{kind === 'published' ? '已发布' : '草稿'}</Tag>
      ),
    },
    {
      title: '变更原因',
      dataIndex: 'bindingFreshness',
      key: 'bindingFreshness',
      render: (_, record) => {
        const diagnostics = record.bindingFreshness?.map((item) => item.diagnostic) || [];
        return diagnosticSummary(diagnostics);
      },
    },
    {
      title: '更新时间',
      dataIndex: 'updatedAt',
      key: 'updatedAt',
      width: 180,
      render: formatDate,
    },
    {
      title: '操作',
      key: 'action',
      width: 360,
      render: (_, record) => (
        <Space>
          <Button
            type="link"
            icon={<ReloadOutlined />}
            loading={contractActionKey === `regenerate:${record.pageKey}`}
            disabled={!record.resourceKey}
            onClick={() => handleRegenerateProposal(record)}
          >
            重生成
          </Button>
          <Button
            type="link"
            icon={<SyncOutlined />}
            loading={contractActionKey === `merge:${record.pageKey}`}
            disabled={!record.resourceKey}
            onClick={() => handleAutoMerge(record)}
          >
            合并
          </Button>
          <Popconfirm title="确认重新发布当前草稿快照？" onConfirm={() => handleRepublish(record)}>
            <Button
              type="link"
              icon={<RocketOutlined />}
              loading={contractActionKey === `republish:${record.pageKey}`}
            >
              重发布
            </Button>
          </Popconfirm>
          <Button type="link" onClick={() => navigateTo(`/system/functions/pages?focus=${encodeURIComponent(record.pageKey)}`)}>
            编辑
          </Button>
        </Space>
      ),
    },
  ];

  const publishable = inbox.publishable.filter((item) => matchesQuery(item, query));
  const needsReview = inbox.needsReview.filter((item) => matchesQuery(item, query));
  const blockedIssues = inbox.blockedIssues;
  const contractChanges = inbox.contractChanges;

  return (
    <Space direction="vertical" style={{ width: '100%' }} size={16}>
      <Alert
        type="info"
        showIcon
        message="默认页面先生成 Proposal，用户确认后才发布到运行控制台"
        description="函数注册只描述能力；页面分类、标题和表单展示由平台生成默认 PageSpec。ready/basic 可以直接发布，不满意再进入编辑。"
      />

      <Card>
        <Space wrap>
          <Input
            placeholder="搜索提案、页面或资源"
            prefix={<SearchOutlined />}
            value={query}
            onChange={(event) => setQuery(event.target.value)}
            style={{ width: 260 }}
          />
          <Button icon={<ReloadOutlined />} onClick={fetchData}>
            刷新
          </Button>
          {resourceKey && <Tag color="blue">当前资源：{resourceKey}</Tag>}
        </Space>
      </Card>

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
                <Tag color="success">{inbox.summary.publishable}</Tag>
              </Space>
            ),
            children: (
              <Table
                columns={proposalColumns}
                dataSource={publishable}
                rowKey="proposalKey"
                loading={loading}
                locale={{ emptyText: <Empty description="暂无可直接发布的默认页面" /> }}
              />
            ),
          },
          {
            key: 'needsReview',
            label: (
              <Space>
                <ExclamationCircleOutlined />
                需要处理
                <Tag color="warning">{inbox.summary.needsReview + inbox.summary.blockedIssues}</Tag>
              </Space>
            ),
            children: (
              <Space direction="vertical" style={{ width: '100%' }} size={16}>
                <Table
                  columns={proposalColumns}
                  dataSource={needsReview}
                  rowKey="proposalKey"
                  loading={loading}
                  locale={{ emptyText: <Empty description="暂无需要处理的 Proposal" /> }}
                />
                <Table
                  columns={blockedColumns}
                  dataSource={blockedIssues}
                  rowKey="id"
                  loading={loading}
                  locale={{ emptyText: <Empty description="暂无阻断项" /> }}
                />
              </Space>
            ),
          },
          {
            key: 'contractChanges',
            label: (
              <Space>
                <SyncOutlined />
                契约变更
                <Tag color="error">{inbox.summary.contractChanges}</Tag>
              </Space>
            ),
            children: (
              <Table
                columns={contractColumns}
                dataSource={contractChanges}
                rowKey={(record) => `${record.kind}:${record.pageKey}`}
                loading={loading}
                locale={{ emptyText: <Empty description="暂无 stale 草稿或已发布页面" /> }}
              />
            ),
          },
        ]}
      />

      <Modal title="提案详情" open={detailVisible} onCancel={() => setDetailVisible(false)} footer={null} width={920}>
        {selectedProposal && (
          <Space direction="vertical" style={{ width: '100%' }} size={16}>
            <Descriptions column={2} bordered>
              <Descriptions.Item label="提案">{selectedProposal.proposalKey}</Descriptions.Item>
              <Descriptions.Item label="页面">{selectedProposal.pageKey}</Descriptions.Item>
              <Descriptions.Item label="标题">{localizedText(selectedProposal.title, selectedProposal.pageKey)}</Descriptions.Item>
              <Descriptions.Item label="类型">
                <Tag color={pageTypeColors[selectedProposal.pageType]}>{pageTypeLabels[selectedProposal.pageType]}</Tag>
              </Descriptions.Item>
              <Descriptions.Item label="质量">
                <Tag color={qualityColors[selectedProposal.quality]}>{qualityLabels[selectedProposal.quality]}</Tag>
              </Descriptions.Item>
              <Descriptions.Item label="状态">
                <Tag color={statusColors[selectedProposal.status]}>{statusLabels[selectedProposal.status]}</Tag>
              </Descriptions.Item>
              <Descriptions.Item label="函数摘要">{selectedProposal.functionDigest || '-'}</Descriptions.Item>
              <Descriptions.Item label="语义摘要">{selectedProposal.semanticsDigest || '-'}</Descriptions.Item>
            </Descriptions>
            {selectedProposal.diagnostics && selectedProposal.diagnostics.length > 0 && (
              <Table
                size="small"
                dataSource={selectedProposal.diagnostics}
                rowKey={(record) => `${record.code}:${record.field || ''}:${record.functionId || ''}`}
                pagination={false}
                columns={[
                  { title: '代码', dataIndex: 'code', key: 'code' },
                  {
                    title: '级别',
                    dataIndex: 'severity',
                    key: 'severity',
                    render: (severity: DiagnosticInfo['severity']) => (
                      <Tag color={severity === 'error' ? 'error' : severity === 'warning' ? 'warning' : 'blue'}>{severity}</Tag>
                    ),
                  },
                  { title: '字段', dataIndex: 'field', key: 'field', render: (value) => value || '-' },
                  { title: '说明', dataIndex: 'message', key: 'message' },
                ]}
              />
            )}
            <Paragraph type="secondary">
              PageSpec 为发布快照输入，不在正常路径手工编辑 JSON；如需调整请进入 Page Studio 编辑器。
            </Paragraph>
          </Space>
        )}
      </Modal>

      <Modal title="默认页面预览" open={previewVisible} onCancel={() => setPreviewVisible(false)} footer={null} width={1100}>
        {selectedProposal?.pageSpec ? (
          <PageRenderer
            pageSpec={selectedProposal.pageSpec}
            onExecute={async () => {
              throw new Error('Proposal 预览不执行函数；发布后请在运行控制台执行。');
            }}
          />
        ) : (
          <Empty description="暂无可预览页面" />
        )}
      </Modal>
    </Space>
  );
}
