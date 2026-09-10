/**
 * ProposalInbox - Page Studio 的唯一三队列入口。
 *
 * 队列数据由后端聚合；前端不从 Proposal quality 推断 stale/blocked。
 */

import React, { useCallback, useEffect, useState } from 'react';
import { Alert, App, Button, Card, Empty, Input, Space, Table, Tabs, Tag } from 'antd';
import {
  ExclamationCircleOutlined,
  ReloadOutlined,
  RocketOutlined,
  SearchOutlined,
  SyncOutlined,
} from '@ant-design/icons';
import type { PageProposal, ProposalInbox as ProposalInboxData } from '@/types/dashboard';
import {
  acceptAndPublishProposal,
  acceptProposal,
  getProposal,
  listProposalInbox,
  rejectProposal,
} from '@/services/dashboard';
import { buildConsolePagePath, requestConsoleMenuRefresh } from '@/utils/consoleMenu';
import { history, useIntl } from '@umijs/max';
import { emptyInbox, matchesQuery } from './shared';
import { buildBlockedColumns, buildProposalColumns } from './ProposalColumns';
import ContractChangesPanel from './ContractChangesPanel';
import ProposalDetailModal from './ProposalDetailModal';
import ProposalPreviewModal from './ProposalPreviewModal';
import {
  clearProposalKeyParam,
  currentProposalKey,
  currentResourceKey,
  navigateTo,
} from './urlFocus';

export interface ProposalInboxProps {
  /** 定位高亮的 pageKey（来自 /functions/pages?focus=）：命中时切换到对应
   *  队列 Tab 并高亮相关行；与编辑器 focus 定位配合使用。 */
  focusPageKey?: string;
}

export default function ProposalInbox({ focusPageKey = '' }: ProposalInboxProps) {
  const { message, modal } = App.useApp();
  const intl = useIntl();
  const [loading, setLoading] = useState(false);
  const [inbox, setInbox] = useState<ProposalInboxData>(emptyInbox);
  const [query, setQuery] = useState('');
  const [activeTab, setActiveTab] = useState('publishable');
  const [selectedProposal, setSelectedProposal] = useState<PageProposal | null>(null);
  const [detailVisible, setDetailVisible] = useState(false);
  const [previewVisible, setPreviewVisible] = useState(false);
  const [resourceKey] = useState(currentResourceKey);
  const [initialProposalKey] = useState(currentProposalKey);

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
      modal.success({
        title: '已直接发布',
        content: `页面 ${result.pageKey} 已发布，版本 ${result.publishedVersion}。运行控制台菜单会从已发布快照生成。`,
        okText: categoryKey ? '打开运行页' : '打开运行控制台',
        onOk: () =>
          navigateTo(categoryKey ? buildConsolePagePath(categoryKey, result.pageKey) : '/console'),
      });
    },
    [fetchData, modal],
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

  const proposalColumns = buildProposalColumns({
    intl,
    modal,
    onViewDetail: handleViewDetail,
    onPreview: handlePreview,
    onAccept: handleAccept,
    onAcceptAndPublish: handleAcceptAndPublish,
    onReview: handleReviewProposal,
    onReject: handleReject,
  });
  const blockedColumns = buildBlockedColumns({ intl });

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
              <ContractChangesPanel
                records={contractChanges}
                loading={loading}
                focusPageKey={focusPageKey}
                onChanged={fetchData}
              />
            ),
          },
        ]}
      />

      <ProposalDetailModal
        open={detailVisible}
        proposal={selectedProposal}
        onClose={() => setDetailVisible(false)}
      />

      <ProposalPreviewModal
        open={previewVisible}
        proposal={selectedProposal}
        onClose={() => setPreviewVisible(false)}
      />
    </Space>
  );
}
