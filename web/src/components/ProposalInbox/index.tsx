/**
 * ProposalInbox - Page Studio 的唯一三队列入口。
 *
 * 队列数据由后端聚合；前端不从 Proposal quality 推断 stale/blocked。
 */

import React, { useCallback, useEffect, useState } from 'react';
import {
  Alert,
  App,
  Button,
  Card,
  Descriptions,
  Dropdown,
  Empty,
  Input,
  InputNumber,
  Modal,
  Popconfirm,
  Select,
  Space,
  Table,
  Tabs,
  Tag,
  Tooltip,
  Typography,
} from 'antd';
import {
  CheckOutlined,
  CloseOutlined,
  ExclamationCircleOutlined,
  FileTextOutlined,
  MoreOutlined,
  ReloadOutlined,
  RocketOutlined,
  SearchOutlined,
  DeleteOutlined,
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
import MergeConflictModal from '@/components/MergeConflictModal';
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
import { deleteVersioningPage } from '@/services/api/versioning';
import { extractErrorMessage } from '@/utils/errors';
import type { ConflictResolution, MergeResponse } from '@/services/api/versioning';
import { publishPageDraft } from '@/services/api/pages';
import PageRenderer from '@/components/PageRenderer';
import { buildConsolePagePath, requestConsoleMenuRefresh } from '@/utils/consoleMenu';
import { history, request } from '@umijs/max';
import { localizedText } from '@/utils/localizedText';

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
  composite: '组合',
};

const pageTypeColors: Record<PageType, string> = {
  resource: 'blue',
  operation: 'green',
  task: 'orange',
  report: 'purple',
  composite: 'cyan',
};

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
    localizedText(proposal.title, 'zh-CN', ''),
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

function currentProposalKey(): string {
  if (typeof window === 'undefined') {
    return '';
  }
  return new URLSearchParams(window.location.search).get('proposalKey') || '';
}

function clearProposalKeyParam() {
  if (typeof window === 'undefined') {
    return;
  }
  const url = new URL(window.location.href);
  if (!url.searchParams.has('proposalKey')) {
    return;
  }
  url.searchParams.delete('proposalKey');
  window.history.replaceState(null, '', `${url.pathname}${url.search}${url.hash}`);
}

function navigateTo(path: string) {
  if (typeof window !== 'undefined') {
    window.location.assign(path);
  }
}

export interface ProposalInboxProps {
  /** 定位高亮的 pageKey（来自 /functions/pages?focus=）：命中时切换到对应
   *  队列 Tab 并高亮相关行；与编辑器 focus 定位配合使用。 */
  focusPageKey?: string;
}

export default function ProposalInbox({ focusPageKey = '' }: ProposalInboxProps) {
  const { message, modal } = App.useApp();
  const [loading, setLoading] = useState(false);
  const [inbox, setInbox] = useState<ProposalInboxData>(emptyInbox);
  const [query, setQuery] = useState('');
  const [activeTab, setActiveTab] = useState('publishable');
  const [selectedProposal, setSelectedProposal] = useState<PageProposal | null>(null);
  const [detailVisible, setDetailVisible] = useState(false);
  const [previewVisible, setPreviewVisible] = useState(false);
  const [resourceKey] = useState(currentResourceKey);
  const [initialProposalKey] = useState(currentProposalKey);
  const [contractActionKey, setContractActionKey] = useState('');
  const [manualMergeVisible, setManualMergeVisible] = useState(false);
  // 组合页创建：函数区块编排

  const [manualMergeLoading, setManualMergeLoading] = useState(false);
  const [manualMergePreview, setManualMergePreview] = useState<MergeResponse | null>(null);
  const [manualMergeRecord, setManualMergeRecord] = useState<ContractChangeInfo | null>(null);

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

  // focus 定位：命中某个队列时自动切到对应 Tab（行高亮由 rowClassName 提供）
  useEffect(() => {
    if (!focusPageKey || loading) return;
    const inPublishable = inbox.publishable.some((item) => item.pageKey === focusPageKey);
    const inNeedsReview = inbox.needsReview.some((item) => item.pageKey === focusPageKey);
    const inContractChanges = inbox.contractChanges.some((item) => item.pageKey === focusPageKey);
    if (inNeedsReview) {
      setActiveTab('needsReview');
    } else if (inContractChanges) {
      setActiveTab('contractChanges');
    } else if (inPublishable) {
      setActiveTab('publishable');
    }
  }, [focusPageKey, inbox, loading]);

  useEffect(() => {
    if (!initialProposalKey) {
      return;
    }
    getProposal(initialProposalKey)
      .then((detail) => {
        setSelectedProposal(detail);
        setActiveTab(detail.quality === 'needs_review' ? 'needsReview' : 'publishable');
        setPreviewVisible(true);
      })
      .catch(() => {
        message.warning(`未找到 Proposal：${initialProposalKey}`);
      })
      .finally(clearProposalKeyParam);
  }, [initialProposalKey, message]);

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
    async (proposal: PageProposal) => {
      await acceptProposal(proposal.proposalKey);
      await fetchData();
      navigateTo(`/functions/pages?focus=${encodeURIComponent(proposal.pageKey)}`);
    },
    [fetchData],
  );

  const handleAcceptAndPublish = useCallback(
    async (proposal: PageProposal) => {
      const result = await acceptAndPublishProposal(proposal.proposalKey);
      await fetchData();
      requestConsoleMenuRefresh();
      const categoryKey = proposal.pageSpec?.category?.key?.trim() || '';
      Modal.success({
        title: '已直接发布',
        content: `页面 ${result.pageKey} 已发布，版本 ${result.publishedVersion}。运行控制台菜单会从已发布快照生成。`,
        okText: categoryKey ? '打开运行页' : '打开运行控制台',
        onOk: () =>
          navigateTo(categoryKey ? buildConsolePagePath(categoryKey, result.pageKey) : '/console'),
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
      if (proposal.pageType === 'resource' && proposal.resourceKey) {
        navigateTo(
          `/functions/resource-catalog?resourceKey=${encodeURIComponent(proposal.resourceKey)}`,
        );
        return;
      }
      if (proposal.diagnostics?.some((item) => item.severity === 'error')) {
        await handlePreview(proposal.proposalKey);
        message.warning('该提案包含阻断诊断，请先查看诊断后再处理。');
        return;
      }
      await handleAccept(proposal);
    },
    [handleAccept, handlePreview, message],
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
      await runContractAction(`regenerate:${record.pageKey}`, async () => {
        try {
          const result = await regenerateProposal(record.pageKey);
          Modal.success({ title: '已重新生成 Proposal', content: result.message });
        } catch (e) {
          // 生成失败（函数禁用/schema 非法等错误级诊断）必须显式反馈，
          // 静默成功会让用户误以为报错已修复。
          Modal.error({
            title: '重新生成失败',
            content: extractErrorMessage(e, '生成页面时出现错误级诊断，请查看页面详情'),
          });
          throw e;
        }
      });
    },
    [runContractAction],
  );

  const handleDeletePage = useCallback(
    async (record: ContractChangeInfo) => {
      Modal.confirm({
        title: '删除页面',
        content: `将删除页面 ${record.pageKey} 的草稿、已发布版本与待审提案，且不可恢复。确认删除？`,
        okType: 'danger',
        okText: '删除',
        onOk: async () => {
          await runContractAction(`delete:${record.pageKey}`, async () => {
            await deleteVersioningPage(record.pageKey);
            Modal.success({ title: '页面已删除' });
          });
        },
      });
    },
    [runContractAction],
  );

  const handleAutoMerge = useCallback(
    async (record: ContractChangeInfo) => {
      await runContractAction(`merge:${record.pageKey}`, async () => {
        const result = await mergeChanges(record.pageKey, { strategy: 'auto' });
        Modal.info({
          title: '自动合并结果',
          content: `${result.message}。安全合并 ${result.merged} 项，仍有 ${result.conflicts} 项需要人工处理。`,
        });
      });
    },
    [runContractAction],
  );

  const handleOpenManualMerge = useCallback(
    async (record: ContractChangeInfo) => {
      setManualMergeLoading(true);
      try {
        const preview = await mergeChanges(record.pageKey, { strategy: 'manual', dryRun: true });
        setManualMergeRecord(record);
        setManualMergePreview(preview);
        setManualMergeVisible(true);
      } catch {
        message.error('加载冲突预览失败');
      } finally {
        setManualMergeLoading(false);
      }
    },
    [message],
  );

  const handleManualMergeSubmit = useCallback(
    async (payload: { conflicts: ConflictResolution[]; reason?: string }) => {
      if (!manualMergeRecord) {
        return;
      }
      setManualMergeLoading(true);
      try {
        const result = await mergeChanges(manualMergeRecord.pageKey, {
          strategy: 'manual',
          conflicts: payload.conflicts,
          reason: payload.reason,
        });
        setManualMergeVisible(false);
        setManualMergePreview(null);
        setManualMergeRecord(null);
        await fetchData();
        Modal.success({
          title: '冲突已处理',
          content: `页面 ${manualMergeRecord.pageKey} 的草稿已更新到版本 ${result.draftRevision || '-'}，请确认后重新发布。`,
        });
      } catch {
        message.error('手动合并失败');
      } finally {
        setManualMergeLoading(false);
      }
    },
    [fetchData, manualMergeRecord, message],
  );

  const handleRepublish = useCallback(
    async (record: ContractChangeInfo) => {
      await runContractAction(`republish:${record.pageKey}`, async () => {
        if (record.draftRevision && record.draftRevision > 0) {
          const result = await publishPageDraft(record.pageKey, record.draftRevision);
          requestConsoleMenuRefresh();
          Modal.success({
            title: '已重新发布',
            content: `页面 ${result.pageKey} 已发布，版本 ${result.publishedVersion}。`,
          });
          return;
        }
        const result = await republish(record.pageKey);
        requestConsoleMenuRefresh();
        Modal.success({ title: '已重新发布', content: result.message });
      });
    },
    [runContractAction],
  );

  const proposalColumns: ColumnsType<PageProposal> = [
    {
      title: '提案',
      dataIndex: 'proposalKey',
      key: 'proposalKey',
      render: (_, record) => {
        // proposalKey 形如 operation--mail.send / resource--mail，pageKey 是其去前缀形态；
        // 两者一致时只显示一行，避免相邻两行看起来是重复字段。
        const bareKey = record.proposalKey.replace(/^(operation|resource|task|report)--/, '');
        return (
          <Space orientation="vertical" size={0}>
            <Text strong>{record.proposalKey}</Text>
            {record.pageKey && record.pageKey !== bareKey && (
              <Text type="secondary">{record.pageKey}</Text>
            )}
          </Space>
        );
      },
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
      render: (type: PageType) => (
        <Tag color={pageTypeColors[type]}>{pageTypeLabels[type] || type}</Tag>
      ),
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
      render: (quality: ProposalQuality) => (
        <Tag color={qualityColors[quality]}>{qualityLabels[quality]}</Tag>
      ),
    },
    {
      title: '状态',
      dataIndex: 'status',
      key: 'status',
      width: 100,
      render: (status: ProposalStatus) => (
        <Tag color={statusColors[status]}>{statusLabels[status]}</Tag>
      ),
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
      width: 168,
      fixed: 'right',
      render: (_, record) => {
        // 主操作保留文字按钮；其余收进"更多"下拉，避免操作列被撑到 600px 级别。
        const moreItems = [];
        if (record.status === 'pending') {
          moreItems.push({
            key: 'customize',
            icon: <CheckOutlined />,
            label: '自定义编辑',
            onClick: () =>
              modal.confirm({
                title: '接受为草稿并自定义页面？',
                onOk: () => handleAccept(record),
              }),
          });
          if (record.quality === 'needs_review') {
            moreItems.push({
              key: 'review',
              icon: <ExclamationCircleOutlined />,
              label: '处理',
              onClick: () => handleReviewProposal(record),
            });
          }
          moreItems.push({
            key: 'reject',
            icon: <CloseOutlined />,
            label: '拒绝',
            danger: true,
            onClick: () =>
              modal.confirm({
                title: '拒绝此提案？',
                onOk: () => handleReject(record.proposalKey),
              }),
          });
        }
        return (
          <Space size={0}>
            <Button type="link" size="small" onClick={() => handleViewDetail(record.proposalKey)}>
              查看
            </Button>
            <Button type="link" size="small" onClick={() => handlePreview(record.proposalKey)}>
              预览
            </Button>
            {record.status === 'pending' &&
              (record.quality === 'ready' || record.quality === 'basic') &&
              !record.pageExists && (
                <Popconfirm
                  title="发布默认页面？"
                  description="会创建草稿并发布到运行控制台左侧动态菜单。"
                  onConfirm={() => handleAcceptAndPublish(record)}
                >
                  <Button type="link" size="small" icon={<RocketOutlined />}>
                    发布
                  </Button>
                </Popconfirm>
              )}
            {record.status === 'pending' && record.pageExists && (
              <Button
                type="link"
                size="small"
                icon={<FileTextOutlined />}
                onClick={() =>
                  navigateTo(`/functions/pages?focus=${encodeURIComponent(record.pageKey)}`)
                }
              >
                去编辑
              </Button>
            )}
            {moreItems.length > 0 && (
              <Dropdown menu={{ items: moreItems }} trigger={['click']}>
                <Tooltip title="更多">
                  <Button type="link" size="small" icon={<MoreOutlined />} />
                </Tooltip>
              </Dropdown>
            )}
          </Space>
        );
      },
    },
  ];

  const blockedColumns: ColumnsType<BlockedProposalIssue> = [
    {
      title: '阻断项',
      dataIndex: 'id',
      key: 'id',
      render: (_, record) => (
        <Space orientation="vertical" size={0}>
          <Text strong>{record.functionId || record.resourceKey || `issue-${record.id}`}</Text>
          <Text type="secondary">{record.resourceKey || '-'}</Text>
        </Space>
      ),
    },
    {
      title: '修复提示',
      dataIndex: 'repairHint',
      key: 'repairHint',
      render: (_, record) => localizedText(record.repairHint, 'zh-CN', '-'),
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
            onClick={() =>
              navigateTo(
                `/functions/resource-catalog?resourceKey=${encodeURIComponent(record.resourceKey || '')}`,
              )
            }
          >
            修复语义
          </Button>
        ) : (
          <Button
            type="link"
            icon={<ExclamationCircleOutlined />}
            onClick={() => navigateTo('/functions/resource-catalog')}
          >
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
        <Space orientation="vertical" size={0}>
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
      render: (type: PageType) => (
        <Tag color={pageTypeColors[type]}>{pageTypeLabels[type] || type}</Tag>
      ),
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
        <Tag color={kind === 'published' ? 'error' : 'warning'}>
          {kind === 'published' ? '已发布' : '草稿'}
        </Tag>
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
      width: 208,
      fixed: 'right',
      render: (_, record) => (
        <Space size={0}>
          <Popconfirm title="确认重新发布当前草稿快照？" onConfirm={() => handleRepublish(record)}>
            <Button
              type="link"
              size="small"
              icon={<RocketOutlined />}
              loading={contractActionKey === `republish:${record.pageKey}`}
            >
              重发布
            </Button>
          </Popconfirm>
          <Button
            type="link"
            size="small"
            onClick={() =>
              navigateTo(`/functions/pages?focus=${encodeURIComponent(record.pageKey)}`)
            }
          >
            编辑
          </Button>
          <Dropdown
            trigger={['click']}
            menu={{
              items: [
                {
                  key: 'regenerate',
                  icon: <ReloadOutlined />,
                  label: '重生成',
                  onClick: () => handleRegenerateProposal(record),
                },
                {
                  key: 'auto-merge',
                  icon: <SyncOutlined />,
                  label: '自动合并',
                  onClick: () => handleAutoMerge(record),
                },
                { type: 'divider' },
                {
                  key: 'delete-page',
                  icon: <DeleteOutlined />,
                  label: '删除页面',
                  danger: true,
                  onClick: () => handleDeletePage(record),
                },
                {
                  key: 'manual-merge',
                  label: '处理冲突',
                  onClick: () => handleOpenManualMerge(record),
                },
              ],
            }}
          >
            <Tooltip title="更多">
              <Button type="link" size="small" icon={<MoreOutlined />} />
            </Tooltip>
          </Dropdown>
        </Space>
      ),
    },
  ];

  const publishable = inbox.publishable.filter((item) => matchesQuery(item, query));
  const needsReview = inbox.needsReview.filter((item) => matchesQuery(item, query));
  const blockedIssues = inbox.blockedIssues;
  const contractChanges = inbox.contractChanges;

  return (
    <Space orientation="vertical" style={{ width: '100%', marginTop: 16 }} size={16}>
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
          <Button
            type="primary"
            ghost
            onClick={() => history.push('/functions/pages/composite-editor')}
          >
            创建组合页
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
                rowClassName={(record) =>
                  record.pageKey === focusPageKey ? 'proposal-inbox-focus-row' : ''
                }
                loading={loading}
                scroll={{ x: 'max-content' }}
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
              <Space orientation="vertical" style={{ width: '100%' }} size={16}>
                <Table
                  columns={proposalColumns}
                  dataSource={needsReview}
                  rowKey="proposalKey"
                  rowClassName={(record) =>
                    record.pageKey === focusPageKey ? 'proposal-inbox-focus-row' : ''
                  }
                  loading={loading}
                  scroll={{ x: 'max-content' }}
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
                rowClassName={(record) =>
                  record.pageKey === focusPageKey ? 'proposal-inbox-focus-row' : ''
                }
                loading={loading}
                scroll={{ x: 'max-content' }}
                locale={{ emptyText: <Empty description="暂无 stale 草稿或已发布页面" /> }}
              />
            ),
          },
        ]}
      />

      <Modal
        title="提案详情"
        open={detailVisible}
        onCancel={() => setDetailVisible(false)}
        footer={null}
        width={920}
      >
        {selectedProposal && (
          <Space orientation="vertical" style={{ width: '100%' }} size={16}>
            <Descriptions column={2} bordered>
              <Descriptions.Item label="提案">{selectedProposal.proposalKey}</Descriptions.Item>
              <Descriptions.Item label="页面">{selectedProposal.pageKey}</Descriptions.Item>
              <Descriptions.Item label="标题">
                {localizedText(selectedProposal.title, selectedProposal.pageKey)}
              </Descriptions.Item>
              <Descriptions.Item label="类型">
                <Tag color={pageTypeColors[selectedProposal.pageType]}>
                  {pageTypeLabels[selectedProposal.pageType]}
                </Tag>
              </Descriptions.Item>
              <Descriptions.Item label="质量">
                <Tag color={qualityColors[selectedProposal.quality]}>
                  {qualityLabels[selectedProposal.quality]}
                </Tag>
              </Descriptions.Item>
              <Descriptions.Item label="状态">
                <Tag color={statusColors[selectedProposal.status]}>
                  {statusLabels[selectedProposal.status]}
                </Tag>
              </Descriptions.Item>
              <Descriptions.Item label="函数摘要">
                {selectedProposal.functionDigest || '-'}
              </Descriptions.Item>
              <Descriptions.Item label="语义摘要">
                {selectedProposal.semanticsDigest || '-'}
              </Descriptions.Item>
            </Descriptions>
            {selectedProposal.diagnostics && selectedProposal.diagnostics.length > 0 && (
              <Table
                size="small"
                dataSource={selectedProposal.diagnostics}
                rowKey={(record) =>
                  `${record.code}:${record.field || ''}:${record.functionId || ''}`
                }
                pagination={false}
                columns={[
                  { title: '代码', dataIndex: 'code', key: 'code' },
                  {
                    title: '级别',
                    dataIndex: 'severity',
                    key: 'severity',
                    render: (severity: DiagnosticInfo['severity']) => (
                      <Tag
                        color={
                          severity === 'error'
                            ? 'error'
                            : severity === 'warning'
                              ? 'warning'
                              : 'blue'
                        }
                      >
                        {severity}
                      </Tag>
                    ),
                  },
                  {
                    title: '字段',
                    dataIndex: 'field',
                    key: 'field',
                    render: (value) => value || '-',
                  },
                  { title: '说明', dataIndex: 'message', key: 'message' },
                ]}
              />
            )}
            <Paragraph type="secondary">
              PageSpec 为发布快照输入，不在正常路径手工编辑 JSON；如需调整请进入 Page Studio
              编辑器。
            </Paragraph>
          </Space>
        )}
      </Modal>

      <Modal
        title="默认页面预览"
        open={previewVisible}
        onCancel={() => setPreviewVisible(false)}
        footer={null}
        width={1100}
      >
        {selectedProposal?.pageSpec ? (
          <PageRenderer
            pageSpec={selectedProposal.pageSpec}
            preview
            onExecute={async () => {
              throw new Error('Proposal 预览不执行函数；发布后请在运行控制台执行。');
            }}
          />
        ) : (
          <Empty description="暂无可预览页面" />
        )}
      </Modal>

      <MergeConflictModal
        open={manualMergeVisible}
        loading={manualMergeLoading}
        preview={manualMergePreview}
        onCancel={() => setManualMergeVisible(false)}
        onSubmit={handleManualMergeSubmit}
      />
    </Space>
  );
}
