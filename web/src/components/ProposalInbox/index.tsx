/**
 * ProposalInbox - Page Studio 的唯一三队列入口。
 *
 * 队列数据由后端聚合；前端不从 Proposal quality 推断 stale/blocked。
 */

import React, { useCallback, useEffect, useMemo, useState } from 'react';
import {
  Alert,
  App,
  Button,
  Card,
  Empty,
  Input,
  Modal,
  Space,
  Table,
  Tabs,
  Tag,
  TreeSelect,
} from 'antd';
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
import { listMenus, type MenuItem } from '@/services/api/menu';
import { updatePageMenu } from '@/services/api/pages';
import { buildConsolePagePath, requestConsoleMenuRefresh } from '@/utils/consoleMenu';
import { localizedText } from '@/utils/localizedText';
import { FormattedMessage, history, useIntl } from '@umijs/max';
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
        message.warning(
          intl.formatMessage(
            {
              id: 'component.proposalInbox.inbox.proposalNotFound',
              defaultMessage: '未找到 Proposal：{key}',
            },
            { key: initialProposalKey },
          ),
        );
      })
      .finally(clearProposalKeyParam);
  }, [initialProposalKey, intl, message]);

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

  // 发布分级（auto env）：accept 落 draft 后由后端自动接续发布。按响应
  // published/publishError 分支提示——发布失败不回滚 accept（draft 已在，
  // 指引走手动发布）；缺省（required env）维持静默跳编辑器的现状。
  const handleAccept = useCallback(
    async (proposal: PageProposal) => {
      const result = await acceptProposal(proposal.proposalKey);
      await fetchData();
      if (result.published) {
        requestConsoleMenuRefresh();
        message.success(
          intl.formatMessage({
            id: 'component.proposalInbox.inbox.acceptAutoPublished',
            defaultMessage: '已接受并自动发布（当前环境为免审核策略）',
          }),
        );
      } else if (result.publishError) {
        message.warning(
          intl.formatMessage(
            {
              id: 'component.proposalInbox.inbox.acceptPublishFailed',
              defaultMessage: '已接受，但自动发布失败：{error}。草稿已保存，可在页面编辑器手动发布',
            },
            { error: result.publishError },
          ),
        );
      }
      navigateTo(`/functions/pages?focus=${encodeURIComponent(proposal.pageKey)}`);
    },
    [fetchData, intl, message],
  );

  const handleAcceptAndPublish = useCallback(
    async (proposal: PageProposal, menuId: number | null) => {
      const result = await acceptAndPublishProposal(proposal.proposalKey);
      let mountFailed = false;
      if (menuId != null) {
        // 挂载写 draft 表 menu_id，控制台导航即时生效，无需重新发布
        try {
          await updatePageMenu(result.pageKey, menuId);
        } catch {
          mountFailed = true;
        }
      }
      await fetchData();
      requestConsoleMenuRefresh();
      const categoryKey = proposal.pageSpec?.category?.key?.trim() || '';
      modal.success({
        title: intl.formatMessage({
          id: 'component.proposalInbox.inbox.acceptAndPublishTitle',
          defaultMessage: '已直接发布',
        }),
        content: mountFailed
          ? intl.formatMessage(
              {
                id: 'component.proposalInbox.inbox.publishMountFailed',
                defaultMessage:
                  '页面 {pageKey} 已发布，版本 {publishedVersion}；但挂载菜单失败，可稍后在页面工作台重新挂载。',
              },
              { pageKey: result.pageKey, publishedVersion: result.publishedVersion },
            )
          : intl.formatMessage(
              {
                id: 'component.proposalInbox.inbox.acceptAndPublishContent',
                defaultMessage: '页面 {pageKey} 已发布，版本 {publishedVersion}。{mountState}',
              },
              {
                pageKey: result.pageKey,
                publishedVersion: result.publishedVersion,
                mountState:
                  menuId != null
                    ? intl.formatMessage({
                        id: 'component.proposalInbox.inbox.mountedHint',
                        defaultMessage: '已挂载到所选菜单，运行控制台导航即时可见。',
                      })
                    : intl.formatMessage({
                        id: 'component.proposalInbox.inbox.notMountedHint',
                        defaultMessage:
                          '未挂载菜单：页面不会出现在运行控制台导航，可稍后在页面工作台挂载。',
                      }),
              },
            ),
        okText: categoryKey
          ? intl.formatMessage({
              id: 'component.proposalInbox.inbox.openRuntimePage',
              defaultMessage: '打开运行页',
            })
          : intl.formatMessage({
              id: 'component.proposalInbox.inbox.openRuntimeConsole',
              defaultMessage: '打开运行控制台',
            }),
        onOk: () =>
          navigateTo(categoryKey ? buildConsolePagePath(categoryKey, result.pageKey) : '/console'),
      });
    },
    [fetchData, intl, modal],
  );

  // 发布确认弹窗（含挂载菜单选择）：发布按钮直开本弹窗，替代原 Popconfirm
  // 直发——控制台导航由 menu_items 唯一驱动，发布时一步完成挂载
  const [publishTarget, setPublishTarget] = useState<PageProposal | null>(null);
  const [publishMenus, setPublishMenus] = useState<MenuItem[]>([]);
  const [publishMenuId, setPublishMenuId] = useState<number | null>(null);
  const [publishing, setPublishing] = useState(false);

  const openPublishModal = useCallback(async (proposal: PageProposal) => {
    setPublishTarget(proposal);
    setPublishMenuId(null);
    try {
      setPublishMenus(await listMenus());
    } catch {
      setPublishMenus([]);
    }
  }, []);

  const menuTreeData = useMemo(() => {
    interface MenuNode {
      value: number;
      title: string;
      children?: MenuNode[];
    }
    const toNode = (node: MenuItem): MenuNode => ({
      value: node.id,
      title: localizedText(node.labels, intl.locale, node.menuKey),
      children: node.children.length ? node.children.map(toNode) : undefined,
    });
    return publishMenus.map(toNode);
  }, [publishMenus, intl.locale]);

  const handlePublishModalOk = useCallback(async () => {
    if (!publishTarget) return;
    setPublishing(true);
    try {
      await handleAcceptAndPublish(publishTarget, publishMenuId);
      setPublishTarget(null);
    } catch {
      message.error(
        intl.formatMessage({
          id: 'component.proposalInbox.inbox.publishFailed',
          defaultMessage: '发布失败，请重试',
        }),
      );
    } finally {
      setPublishing(false);
    }
  }, [publishTarget, publishMenuId, handleAcceptAndPublish, intl, message]);

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
        message.warning(
          intl.formatMessage({
            id: 'component.proposalInbox.inbox.blockedDiagnosticsWarning',
            defaultMessage: '该提案包含阻断诊断，请先查看诊断后再处理。',
          }),
        );
        return;
      }
      await handleAccept(proposal);
    },
    [handleAccept, handlePreview, intl, message],
  );

  const proposalColumns = buildProposalColumns({
    intl,
    modal,
    onViewDetail: handleViewDetail,
    onPreview: handlePreview,
    onAccept: handleAccept,
    onRequestPublish: (proposal) => void openPublishModal(proposal),
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
        title={intl.formatMessage({
          id: 'component.proposalInbox.inbox.alertMessage',
          defaultMessage: '默认页面先生成 Proposal，用户确认后才发布到运行控制台',
        })}
        description={intl.formatMessage({
          id: 'component.proposalInbox.inbox.alertDescription',
          defaultMessage:
            '函数注册只描述能力；页面分类、标题和表单展示由平台生成默认 PageSpec。ready/basic 可以直接发布，不满意再进入编辑。',
        })}
      />

      <Card>
        <Space wrap>
          <Input
            placeholder={intl.formatMessage({
              id: 'component.proposalInbox.inbox.searchPlaceholder',
              defaultMessage: '搜索提案、页面或资源',
            })}
            prefix={<SearchOutlined />}
            value={query}
            onChange={(event) => setQuery(event.target.value)}
            style={{ width: 260 }}
          />
          <Button icon={<ReloadOutlined />} onClick={fetchData}>
            <FormattedMessage id="component.proposalInbox.inbox.refresh" defaultMessage="刷新" />
          </Button>
          <Button
            type="primary"
            ghost
            onClick={() => history.push('/functions/pages/composite-editor')}
          >
            <FormattedMessage
              id="component.proposalInbox.inbox.createComposite"
              defaultMessage="创建组合页"
            />
          </Button>
          {resourceKey && (
            <Tag color="blue">
              {intl.formatMessage(
                {
                  id: 'component.proposalInbox.inbox.currentResourceTag',
                  defaultMessage: '当前资源：{resourceKey}',
                },
                { resourceKey },
              )}
            </Tag>
          )}
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
                <FormattedMessage
                  id="component.proposalInbox.inbox.tabPublishable"
                  defaultMessage="可直接发布"
                />
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
                scroll={{ x: 1260 }}
                locale={{
                  emptyText: (
                    <Empty
                      description={intl.formatMessage({
                        id: 'component.proposalInbox.inbox.emptyPublishable',
                        defaultMessage: '暂无可直接发布的默认页面',
                      })}
                    />
                  ),
                }}
              />
            ),
          },
          {
            key: 'needsReview',
            label: (
              <Space>
                <ExclamationCircleOutlined />
                <FormattedMessage
                  id="component.proposalInbox.inbox.tabNeedsReview"
                  defaultMessage="需要处理"
                />
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
                  scroll={{ x: 1260 }}
                  locale={{
                    emptyText: (
                      <Empty
                        description={intl.formatMessage({
                          id: 'component.proposalInbox.inbox.emptyNeedsReview',
                          defaultMessage: '暂无需要处理的 Proposal',
                        })}
                      />
                    ),
                  }}
                />
                <Table
                  columns={blockedColumns}
                  dataSource={blockedIssues}
                  rowKey="id"
                  loading={loading}
                  scroll={{ x: 840 }}
                  locale={{
                    emptyText: (
                      <Empty
                        description={intl.formatMessage({
                          id: 'component.proposalInbox.inbox.emptyBlocked',
                          defaultMessage: '暂无阻断项',
                        })}
                      />
                    ),
                  }}
                />
              </Space>
            ),
          },
          {
            key: 'contractChanges',
            label: (
              <Space>
                <SyncOutlined />
                <FormattedMessage
                  id="component.proposalInbox.inbox.tabContractChanges"
                  defaultMessage="契约变更"
                />
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

      <Modal
        open={publishTarget != null}
        title={intl.formatMessage({
          id: 'component.proposalInbox.column.action.publishConfirmTitle',
          defaultMessage: '发布默认页面？',
        })}
        confirmLoading={publishing}
        onOk={() => void handlePublishModalOk()}
        onCancel={() => setPublishTarget(null)}
        destroyOnHidden
      >
        <Space orientation="vertical" style={{ width: '100%' }} size={12}>
          <span>
            {intl.formatMessage({
              id: 'component.proposalInbox.column.action.publishConfirmDescription',
              defaultMessage: '会创建草稿并发布。',
            })}
          </span>
          {publishMenus.length > 0 ? (
            <>
              <span>
                {intl.formatMessage({
                  id: 'component.proposalInbox.publish.mountField',
                  defaultMessage: '挂载到菜单（可选）',
                })}
              </span>
              <TreeSelect
                style={{ width: '100%' }}
                value={publishMenuId ?? undefined}
                treeData={menuTreeData}
                treeDefaultExpandAll
                allowClear
                placeholder={intl.formatMessage({
                  id: 'component.proposalInbox.publish.mountPlaceholder',
                  defaultMessage: '选择挂载的菜单；不选则仅发布',
                })}
                onChange={(value: number | undefined) => setPublishMenuId(value ?? null)}
              />
            </>
          ) : (
            <Alert
              type="warning"
              showIcon
              title={intl.formatMessage({
                id: 'component.proposalInbox.publish.noMenus',
                defaultMessage:
                  '当前环境暂无菜单：页面发布后不会出现在运行控制台导航，可先到「菜单管理」创建菜单。',
              })}
            />
          )}
        </Space>
      </Modal>

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
