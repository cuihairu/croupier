import React, { useCallback, useEffect, useState } from 'react';
import { history } from '@umijs/max';
import { PageContainer, ProTable, type ProColumns } from '@ant-design/pro-components';
import {
  Alert,
  App,
  Button,
  Card,
  Col,
  Collapse,
  Drawer,
  Dropdown,
  Empty,
  Modal,
  Popconfirm,
  Row,
  Space,
  Switch,
  Tag,
  Timeline,
  Tooltip,
  Typography,
} from 'antd';
import {
  DiffOutlined,
  EditOutlined,
  EyeOutlined,
  HistoryOutlined,
  MergeOutlined,
  MoreOutlined,
  ReloadOutlined,
  RocketOutlined,
  StopOutlined,
} from '@ant-design/icons';
import MergeConflictModal from '@/components/MergeConflictModal';
import PageEditor from '@/components/PageEditor';
import PageRenderer from '@/components/PageRenderer';
import ProposalInbox from '@/components/ProposalInbox';
import PageWorkflowGuide from '@/components/PageWorkflowGuide';
import {
  getPageDraft,
  listPageVersions,
  listPageDrafts,
  publishPageDraft,
  regeneratePageDraft,
  savePageDraft,
  unpublishPage,
  bulkPublishPages,
  bulkUnpublishPages,
} from '@/services/api/pages';
import {
  getChangeChain,
  getDiff,
  mergeChanges,
  rollbackDraft,
  rollbackPublish,
  type ChangeChain,
  type ConflictResolution,
  type DiffResponse,
  type MergeResponse,
  type MergeStrategy,
} from '@/services/api/versioning';
import type {
  PageSpec,
  PageSpecDraft,
  PageSpecDraftSummary,
  PageType,
  PageVersionItem,
} from '@/types/dashboard';
import { requestConsoleMenuRefresh } from '@/utils/consoleMenu';
import { localizedText } from '@/utils/localizedText';
import { extractErrorDetails, extractErrorMessage } from '@/utils/errors';

const { Paragraph, Text } = Typography;

function statusColor(status: PageSpecDraftSummary['status']) {
  if (status === 'published') return 'green';
  if (status === 'archived') return 'default';
  return 'blue';
}

function formatDate(value?: string): string {
  if (!value) return '-';
  const time = new Date(value);
  return Number.isNaN(time.getTime()) ? value : time.toLocaleString();
}

function pageTypeLabel(type: PageType): string {
  switch (type) {
    case 'resource':
      return '资源页面';
    case 'operation':
      return '操作页面';
    case 'task':
      return '任务页面';
    case 'report':
      return '报表页面';
    default:
      return type;
  }
}

function currentFocusPageKey(): string {
  if (typeof window === 'undefined') {
    return '';
  }
  return new URLSearchParams(window.location.search).get('focus') || '';
}

/** 结构化错误明细列表：展示后端 details（字段路径 → 失败原因）。 */
function ErrorDetailList({ error }: { error: unknown }) {
  const details = extractErrorDetails(error);
  if (details.length === 0) return null;
  return (
    <ul style={{ margin: 0, paddingLeft: 18, maxHeight: 260, overflowY: 'auto' }}>
      {details.map((d, i) => (
        <li key={`${d.field}-${i}`}>
          {d.field ? (
            <>
              <code>{d.field}</code>
              {': '}
            </>
          ) : null}
          {d.message}
        </li>
      ))}
    </ul>
  );
}

export default function PageStudio() {
  const { message, modal } = App.useApp();
  const [drafts, setDrafts] = useState<PageSpecDraftSummary[]>([]);
  const [loading, setLoading] = useState(false);
  const [selectedDraft, setSelectedDraft] = useState<PageSpecDraft | null>(null);
  const [selectedDraftRevision, setSelectedDraftRevision] = useState(0);
  const [previewVisible, setPreviewVisible] = useState(false);
  const [editorVisible, setEditorVisible] = useState(false);
  const [livePreview, setLivePreview] = useState(true);
  const [saving, setSaving] = useState(false);
  const [changeChainVisible, setChangeChainVisible] = useState(false);
  const [changeChain, setChangeChain] = useState<ChangeChain | null>(null);
  const [changeChainLoading, setChangeChainLoading] = useState(false);
  const [selectedPageKey, setSelectedPageKey] = useState('');
  const [focusPageKey, setFocusPageKey] = useState('');
  const [diffVisible, setDiffVisible] = useState(false);
  const [diffData, setDiffData] = useState<DiffResponse | null>(null);
  const [diffLoading, setDiffLoading] = useState(false);
  const [mergeVisible, setMergeVisible] = useState(false);
  const [mergeLoading, setMergeLoading] = useState(false);
  const [manualMergeVisible, setManualMergeVisible] = useState(false);
  const [manualMergePreview, setManualMergePreview] = useState<MergeResponse | null>(null);
  const [versionsVisible, setVersionsVisible] = useState(false);
  const [versionsLoading, setVersionsLoading] = useState(false);
  const [versionItems, setVersionItems] = useState<PageVersionItem[]>([]);
  // 版本历史服务端分页（版本随编辑/重发布无上限增长）
  const [versionPage, setVersionPage] = useState(1);
  const [versionPageSize, setVersionPageSize] = useState(5);
  const [versionTotal, setVersionTotal] = useState(0);
  const [currentDraftVersion, setCurrentDraftVersion] = useState(0);
  const [currentPublishedVersion, setCurrentPublishedVersion] = useState(0);

  const loadDrafts = useCallback(async () => {
    setLoading(true);
    try {
      setDrafts(await listPageDrafts());
    } catch {
      message.error('加载页面列表失败');
    } finally {
      setLoading(false);
    }
  }, [message]);

  useEffect(() => {
    loadDrafts();
  }, [loadDrafts]);

  const loadDraftDetail = useCallback(
    async (pageKey: string) => {
      try {
        const draft = await getPageDraft(pageKey);
        setSelectedDraft(draft);
        setSelectedDraftRevision(draft.draftRevision || 0);
      } catch {
        message.error('加载页面详情失败');
      }
    },
    [message],
  );

  const handlePublish = useCallback(
    async (pageKey: string, draftRevision: number) => {
      try {
        await publishPageDraft(pageKey, draftRevision);
        requestConsoleMenuRefresh();
        message.success('发布成功');
        loadDrafts();
      } catch {
        message.error('发布失败');
      }
    },
    [loadDrafts, message],
  );

  // F：一键发布全部（ready/basic 提案走真实 accept-and-publish 链路）
  const [bulkLoading, setBulkLoading] = useState<'publish' | 'unpublish' | null>(null);
  const handleBulkPublish = useCallback(async () => {
    setBulkLoading('publish');
    try {
      const res = await bulkPublishPages();
      const published = res.published?.length ?? 0;
      const failed = res.failed?.length ?? 0;
      if (failed > 0) {
        message.warning(`已发布 ${published} 个页面，${failed} 个失败`);
      } else {
        message.success(`已发布 ${published} 个页面`);
      }
      requestConsoleMenuRefresh();
      loadDrafts();
    } catch {
      message.error('一键发布失败');
    } finally {
      setBulkLoading(null);
    }
  }, [loadDrafts]);

  const handleBulkUnpublish = useCallback(async () => {
    setBulkLoading('unpublish');
    try {
      const res = await bulkUnpublishPages();
      const unpublished = res.unpublished?.length ?? 0;
      message.success(`已下架 ${unpublished} 个页面`);
      requestConsoleMenuRefresh();
      loadDrafts();
    } catch {
      message.error('一键下架失败');
    } finally {
      setBulkLoading(null);
    }
  }, [loadDrafts]);

  const handleUnpublish = useCallback(
    async (pageKey: string) => {
      try {
        await unpublishPage(pageKey);
        requestConsoleMenuRefresh();
        message.success('已取消发布');
        loadDrafts();
      } catch {
        message.error('取消发布失败');
      }
    },
    [loadDrafts, message],
  );

  const handlePreview = useCallback(
    (pageKey: string) => {
      loadDraftDetail(pageKey);
      setPreviewVisible(true);
    },
    [loadDraftDetail],
  );

  const handleEdit = useCallback(
    (pageKey: string, pageType?: string) => {
      if (pageType === 'composite') {
        // 复合页走 V3 组件化编辑器（回读 spec 反编译为树）
        history.push(`/functions/pages/composite-editor?pageKey=${encodeURIComponent(pageKey)}`);
        return;
      }
      loadDraftDetail(pageKey);
      setEditorVisible(true);
    },
    [loadDraftDetail],
  );

  useEffect(() => {
    const key = currentFocusPageKey();
    if (key) {
      setFocusPageKey(key);
      handleEdit(key);
    }
  }, [handleEdit]);

  // 编辑器只改页面内容部分；draft 元数据（status/revision/bindingFreshness
  // 等）保留当前 state 的值。
  const updateSelectedDraftSpec = useCallback((value: PageSpec) => {
    setSelectedDraft((prev) => {
      if (!prev) return null;
      // spread union 后 TS 无法证明变体互斥字段，此处断言是安全的：
      // value 是编辑器基于当前 draft 产出的同变体 PageSpec。
      return { ...prev, ...value } as PageSpecDraft;
    });
  }, []);

  const handleSave = useCallback(
    async (options?: { publishAfterSave?: boolean }) => {
      if (!selectedDraft) return;
      const publishAfterSave = options?.publishAfterSave ?? false;
      setSaving(true);
      try {
        const result = await savePageDraft({
          ...selectedDraft,
          draftRevision: selectedDraftRevision,
        });
        setSelectedDraftRevision(result.draftRevision);
        if (publishAfterSave) {
          try {
            await publishPageDraft(selectedDraft.pageKey, result.draftRevision);
            requestConsoleMenuRefresh();
            message.success('已保存并发布');
          } catch (publishError) {
            // 草稿已保存，仅发布失败：保留编辑器打开让用户决定重试或稍后发布
            const reason = extractErrorMessage(publishError, '未知原因');
            modal.error({
              title: '发布失败（草稿已保存）',
              width: 560,
              content: (
                <Space direction="vertical" size={4} style={{ width: '100%' }}>
                  <Typography.Text type="secondary">失败原因：{reason}</Typography.Text>
                  <ErrorDetailList error={publishError} />
                  <Typography.Text type="warning">
                    通常是函数契约已变化导致页面绑定失效，可点击「重新生成草稿」按最新契约重建后再发布。
                  </Typography.Text>
                </Space>
              ),
            });
            loadDrafts();
            return;
          }
        } else {
          message.success('保存成功');
        }
        setEditorVisible(false);
        loadDrafts();
      } catch (saveError) {
        modal.error({
          title: '保存失败',
          width: 560,
          content: (
            <Space direction="vertical" size={4} style={{ width: '100%' }}>
              <Typography.Text type="secondary">
                {extractErrorMessage(saveError, '保存失败，请稍后重试')}
              </Typography.Text>
              <ErrorDetailList error={saveError} />
            </Space>
          ),
        });
      } finally {
        setSaving(false);
      }
    },
    [loadDrafts, message, modal, selectedDraft, selectedDraftRevision],
  );

  const handleRegenerate = useCallback(
    async (pageKey: string, draftRevision: number) => {
      try {
        const result = await regeneratePageDraft(pageKey, draftRevision);
        if (selectedDraft?.pageKey === pageKey) {
          setSelectedDraft(result.page);
          setSelectedDraftRevision(result.draftRevision);
        }
        message.success('已按最新 Proposal 重新生成草稿');
        await loadDrafts();
      } catch {
        message.error('重新生成草稿失败');
      } finally {
      }
    },
    [loadDrafts, message, selectedDraft?.pageKey],
  );

  const handleChangeChain = useCallback(
    async (pageKey: string) => {
      setSelectedPageKey(pageKey);
      setChangeChainVisible(true);
      setChangeChainLoading(true);
      try {
        setChangeChain(await getChangeChain(pageKey));
      } catch {
        message.error('加载变更链失败');
      } finally {
        setChangeChainLoading(false);
      }
    },
    [message],
  );

  const handleDiff = useCallback(
    async (pageKey: string) => {
      setSelectedPageKey(pageKey);
      setDiffVisible(true);
      setDiffLoading(true);
      try {
        const [, diff] = await Promise.all([loadDraftDetail(pageKey), getDiff(pageKey)]);
        setDiffData(diff);
      } catch {
        message.error('加载 Diff 失败');
      } finally {
        setDiffLoading(false);
      }
    },
    [loadDraftDetail, message],
  );

  const loadVersionHistory = useCallback(
    async (pageKey: string, page = 1, pageSize = versionPageSize) => {
      setVersionsLoading(true);
      try {
        const result = await listPageVersions(pageKey, {
          limit: pageSize,
          offset: (page - 1) * pageSize,
        });
        setVersionItems(result.items || []);
        setVersionTotal(result.total ?? (result.items || []).length);
        setCurrentDraftVersion(result.currentDraftRevision || 0);
        setCurrentPublishedVersion(result.currentPublishedVersion || 0);
      } catch {
        message.error('加载版本历史失败');
      } finally {
        setVersionsLoading(false);
      }
    },
    [message, versionPageSize],
  );

  const handleVersions = useCallback(
    async (pageKey: string) => {
      setSelectedPageKey(pageKey);
      setVersionsVisible(true);
      setVersionPage(1);
      setVersionPageSize(5);
      await Promise.all([loadDraftDetail(pageKey), loadVersionHistory(pageKey)]);
    },
    [loadDraftDetail, loadVersionHistory],
  );

  const handleMerge = useCallback(
    async (strategy: MergeStrategy) => {
      if (!selectedPageKey || selectedDraftRevision <= 0) {
        message.error('页面草稿版本无效，请刷新后重试');
        return;
      }
      setMergeLoading(true);
      try {
        const result = await mergeChanges(selectedPageKey, {
          expectedDraftRevision: selectedDraftRevision,
          strategy,
        });
        if (result.draftRevision) {
          setSelectedDraftRevision(result.draftRevision);
        }
        message.success(`合并完成：${result.merged} 项自动合并，${result.conflicts} 项冲突`);
        setMergeVisible(false);
        loadDrafts();
      } catch {
        message.error('合并失败');
      } finally {
        setMergeLoading(false);
      }
    },
    [loadDrafts, message, selectedDraftRevision, selectedPageKey],
  );

  const handleOpenManualMerge = useCallback(async () => {
    if (!selectedPageKey) {
      message.error('请选择页面后再处理冲突');
      return;
    }
    setMergeLoading(true);
    try {
      const preview = await mergeChanges(selectedPageKey, { strategy: 'manual', dryRun: true });
      setManualMergePreview(preview);
      setMergeVisible(false);
      setManualMergeVisible(true);
    } catch {
      message.error('加载冲突预览失败');
    } finally {
      setMergeLoading(false);
    }
  }, [message, selectedPageKey]);

  const handleManualMergeSubmit = useCallback(
    async (payload: { conflicts: ConflictResolution[]; reason?: string }) => {
      if (!selectedPageKey || !manualMergePreview) {
        return;
      }

      setMergeLoading(true);
      try {
        const result = await mergeChanges(selectedPageKey, {
          expectedDraftRevision: selectedDraftRevision,
          strategy: 'manual',
          conflicts: payload.conflicts,
          reason: payload.reason,
        });
        if (result.draftRevision) {
          setSelectedDraftRevision(result.draftRevision);
        }
        message.success(`合并完成：草稿已更新到版本 ${result.draftRevision || '-'}`);
        setManualMergeVisible(false);
        setManualMergePreview(null);
        loadDrafts();
      } catch {
        message.error('手动合并失败');
      } finally {
        setMergeLoading(false);
      }
    },
    [loadDrafts, manualMergePreview, message, selectedDraftRevision, selectedPageKey],
  );

  const handleRollbackDraftVersion = useCallback(
    async (version: number) => {
      if (!selectedPageKey) {
        return;
      }
      try {
        const result = await rollbackDraft(selectedPageKey, {
          expectedDraftRevision: selectedDraftRevision,
          version,
          reason: 'rollback draft from page studio',
        });
        setSelectedDraftRevision(result.draftRevision);
        message.success(result.message);
        await Promise.all([loadDrafts(), loadVersionHistory(selectedPageKey)]);
      } catch {
        message.error('回滚草稿失败');
      }
    },
    [loadDrafts, loadVersionHistory, message, selectedDraftRevision, selectedPageKey],
  );

  const handleRollbackPublishedVersion = useCallback(
    async (version: number) => {
      if (!selectedPageKey) {
        return;
      }
      try {
        const result = await rollbackPublish(selectedPageKey, {
          expectedDraftRevision: selectedDraftRevision,
          version,
          reason: 'rollback published page from page studio',
        });
        setSelectedDraftRevision(result.draftRevision);
        message.success(result.message);
        await Promise.all([loadDrafts(), loadVersionHistory(selectedPageKey)]);
      } catch {
        message.error('回滚发布失败');
      }
    },
    [loadDrafts, loadVersionHistory, message, selectedDraftRevision, selectedPageKey],
  );

  const columns: ProColumns<PageSpecDraftSummary>[] = [
    {
      title: '页面标识',
      dataIndex: 'pageKey',
      key: 'pageKey',
      width: 220,
      render: (_, record) => (
        <Space>
          <Text strong>{record.pageKey}</Text>
          <Tag color="blue">{pageTypeLabel(record.type)}</Tag>
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
      title: '分类',
      dataIndex: ['category', 'key'],
      key: 'category',
      width: 120,
      render: (_, record) => localizedText(record.category?.labels, record.category?.key || '-'),
    },
    {
      title: '状态',
      dataIndex: 'status',
      key: 'status',
      width: 100,
      render: (_, record) => <Tag color={statusColor(record.status)}>{record.status}</Tag>,
    },
    {
      title: '版本',
      dataIndex: 'publishedVersion',
      key: 'version',
      width: 80,
      render: (_, record) => record.publishedVersion || '-',
    },
    {
      title: '更新时间',
      dataIndex: 'updatedAt',
      key: 'updatedAt',
      width: 180,
      render: (_, record) => formatDate(record.updatedAt),
    },
    {
      title: '操作',
      key: 'actions',
      width: 160,
      fixed: 'right',
      render: (_, record) => (
        <Space size={4}>
          <Tooltip title="编辑">
            <Button
              type="link"
              size="small"
              icon={<EditOutlined />}
              onClick={() => handleEdit(record.pageKey, record.type)}
            />
          </Tooltip>
          <Tooltip title="预览">
            <Button
              type="link"
              size="small"
              icon={<EyeOutlined />}
              onClick={() => handlePreview(record.pageKey)}
            />
          </Tooltip>
          {record.status === 'draft' ? (
            <Popconfirm
              title="确认发布此页面？"
              onConfirm={() => handlePublish(record.pageKey, record.draftRevision)}
            >
              <Tooltip title="发布">
                <Button type="link" size="small" icon={<RocketOutlined />} />
              </Tooltip>
            </Popconfirm>
          ) : (
            <Popconfirm
              title="确认取消发布此页面？"
              onConfirm={() => handleUnpublish(record.pageKey)}
            >
              <Tooltip title="取消发布">
                <Button type="link" size="small" icon={<StopOutlined />} danger />
              </Tooltip>
            </Popconfirm>
          )}
          <Dropdown
            menu={{
              items: [
                {
                  key: 'regenerate',
                  icon: <ReloadOutlined />,
                  label: '重新生成',
                  onClick: () =>
                    modal.confirm({
                      title: '确认按最新 Proposal 重新生成草稿？',
                      content: '当前草稿修改将被最新 Proposal 覆盖，已发布版本不会变更。',
                      onOk: () => handleRegenerate(record.pageKey, record.draftRevision),
                    }),
                },
                {
                  key: 'versions',
                  icon: <HistoryOutlined />,
                  label: '版本历史',
                  onClick: () => handleVersions(record.pageKey),
                },
                {
                  key: 'change-chain',
                  icon: <HistoryOutlined />,
                  label: '变更链',
                  onClick: () => handleChangeChain(record.pageKey),
                },
                {
                  key: 'diff',
                  icon: <DiffOutlined />,
                  label: '变更对比',
                  onClick: () => handleDiff(record.pageKey),
                },
              ],
            }}
            trigger={['click']}
          >
            <Button type="link" size="small" icon={<MoreOutlined />} />
          </Dropdown>
        </Space>
      ),
    },
  ];

  return (
    <PageContainer
      title="页面工作台"
      subTitle="注册能力后自动生成默认页面；预览、发布、运行无需手工创建页面"
    >
      <PageWorkflowGuide />

      <ProposalInbox focusPageKey={focusPageKey} />

      <Collapse
        style={{ marginTop: 16 }}
        items={[
          {
            key: 'advanced-page-management',
            label: '高级页面管理（仅在已接受草稿、处理版本或回滚时使用）',
            children: (
              <ProTable<PageSpecDraftSummary>
                columns={columns}
                dataSource={drafts}
                loading={loading}
                rowKey="pageKey"
                search={false}
                pagination={false}
                toolBarRender={() => [
                  <Button key="refresh" icon={<ReloadOutlined />} onClick={loadDrafts}>
                    刷新
                  </Button>,
                  <Button
                    key="bulk-publish"
                    icon={<RocketOutlined />}
                    loading={bulkLoading === 'publish'}
                    onClick={() => {
                      Modal.confirm({
                        title: '一键发布全部',
                        content:
                          '将重算提案并把所有 ready/basic 提案按真实链路发布（同 scope）。确认执行？',
                        okText: '发布',
                        onOk: handleBulkPublish,
                      });
                    }}
                  >
                    一键发布全部
                  </Button>,
                  <Button
                    key="bulk-unpublish"
                    danger
                    loading={bulkLoading === 'unpublish'}
                    onClick={() => {
                      Modal.confirm({
                        title: '一键下架全部',
                        content:
                          '将下线当前 scope 内全部已发布页面（运行控制台菜单随之清空）。确认执行？',
                        okText: '下架',
                        okButtonProps: { danger: true },
                        onOk: handleBulkUnpublish,
                      });
                    }}
                  >
                    一键下架全部
                  </Button>,
                ]}
              />
            ),
          },
        ]}
      />

      <Drawer
        title="页面预览"
        width={900}
        open={previewVisible}
        onClose={() => setPreviewVisible(false)}
      >
        {selectedDraft ? (
          <PageRenderer
            pageSpec={selectedDraft}
            preview
            onExecute={async () => {
              throw new Error('Page Studio 预览不执行函数；发布后请在运行控制台执行。');
            }}
          />
        ) : (
          <Empty description="请选择页面" />
        )}
      </Drawer>

      <Modal
        title={
          <Space>
            <span>页面编辑</span>
            <Text type="secondary" code>
              {selectedPageKey || '-'}
            </Text>
          </Space>
        }
        open={editorVisible}
        onCancel={() => setEditorVisible(false)}
        width="100%"
        style={{ top: 16, maxWidth: 1600, paddingBottom: 0 }}
        styles={{ body: { height: 'calc(100vh - 120px)', overflow: 'hidden', paddingTop: 12 } }}
        footer={
          <Space>
            <Switch
              checkedChildren="预览开"
              unCheckedChildren="预览关"
              checked={livePreview}
              onChange={setLivePreview}
            />
            <Button onClick={() => setEditorVisible(false)}>取消</Button>
            <Button loading={saving} onClick={() => handleSave()}>
              仅保存草稿
            </Button>
            <Button
              type="primary"
              loading={saving}
              onClick={() => handleSave({ publishAfterSave: true })}
            >
              保存并发布
            </Button>
          </Space>
        }
      >
        {selectedDraft ? (
          <Row gutter={16} style={{ height: '100%' }}>
            <Col
              span={livePreview ? 13 : 24}
              style={{ height: '100%', overflow: 'auto', paddingRight: 4 }}
            >
              {selectedDraft.bindingFreshness && selectedDraft.bindingFreshness.length > 0 ? (
                <Alert
                  type="warning"
                  showIcon
                  style={{ marginBottom: 12 }}
                  message="页面绑定与函数契约不一致（发布会校验失败）"
                  description={
                    <ul style={{ margin: 0, paddingLeft: 18 }}>
                      {selectedDraft.bindingFreshness.slice(0, 8).map((item, i) => (
                        <li key={i}>
                          <code>{item.functionId || item.bindingId || '-'}</code>
                          {item.diagnostic?.message ? `：${item.diagnostic.message}` : ''}
                        </li>
                      ))}
                      {selectedDraft.bindingFreshness.length > 8 ? (
                        <li>…以及另外 {selectedDraft.bindingFreshness.length - 8} 条</li>
                      ) : null}
                    </ul>
                  }
                />
              ) : null}
              <PageEditor value={selectedDraft} onChange={updateSelectedDraftSpec} />
            </Col>
            {livePreview ? (
              <Col span={11} style={{ height: '100%', overflow: 'auto' }}>
                <Card
                  size="small"
                  title="实时预览"
                  extra={<Text type="secondary">预览不执行函数；发布后请在运行控制台执行</Text>}
                >
                  <PageRenderer
                    pageSpec={selectedDraft}
                    preview
                    onExecute={async () => {
                      throw new Error('Page Studio 预览不执行函数；发布后请在运行控制台执行。');
                    }}
                  />
                </Card>
              </Col>
            ) : null}
          </Row>
        ) : (
          <Empty description="请选择页面" />
        )}
      </Modal>

      <Drawer
        title="版本历史"
        width={760}
        open={versionsVisible}
        onClose={() => setVersionsVisible(false)}
      >
        <Space direction="vertical" style={{ width: '100%' }} size="middle">
          <Paragraph>
            <Text strong>页面：</Text> {selectedPageKey || '-'}
          </Paragraph>
          <Paragraph>
            <Text strong>当前草稿：</Text> {currentDraftVersion || '-'}，
            <Text strong>当前发布：</Text> {currentPublishedVersion || '-'}
          </Paragraph>
          <ProTable<PageVersionItem>
            columns={[
              {
                title: '版本',
                dataIndex: 'version',
                key: 'version',
                width: 90,
                render: (_, record) => <Text strong>v{record.version}</Text>,
              },
              {
                title: '状态',
                dataIndex: 'status',
                key: 'status',
                width: 100,
                render: (_, record) => (
                  <Tag color={record.status === 'published' ? 'green' : 'blue'}>
                    {record.status}
                  </Tag>
                ),
              },
              {
                title: '当前位置',
                key: 'current',
                width: 160,
                render: (_, record) => (
                  <Space>
                    {record.isCurrentDraft ? <Tag color="blue">当前草稿</Tag> : null}
                    {record.isCurrentPublished ? <Tag color="green">当前发布</Tag> : null}
                  </Space>
                ),
              },
              {
                title: '说明',
                dataIndex: 'message',
                key: 'message',
                render: (_, record) => record.message || '-',
              },
              {
                title: '创建时间',
                dataIndex: 'createdAt',
                key: 'createdAt',
                width: 180,
                render: (_, record) => formatDate(record.createdAt),
              },
              {
                title: '操作',
                key: 'actions',
                width: 220,
                render: (_, record) => (
                  <Space>
                    {!record.isCurrentDraft ? (
                      <Popconfirm
                        title={`确认回滚草稿到版本 ${record.version}？`}
                        onConfirm={() => handleRollbackDraftVersion(record.version)}
                      >
                        <Button type="link" size="small">
                          回滚草稿
                        </Button>
                      </Popconfirm>
                    ) : null}
                    {record.status === 'published' && !record.isCurrentPublished ? (
                      <Popconfirm
                        title={`确认回滚发布到版本 ${record.version}？`}
                        onConfirm={() => handleRollbackPublishedVersion(record.version)}
                      >
                        <Button type="link" size="small">
                          回滚发布
                        </Button>
                      </Popconfirm>
                    ) : null}
                  </Space>
                ),
              },
            ]}
            dataSource={versionItems}
            loading={versionsLoading}
            rowKey="version"
            search={false}
            pagination={{
              current: versionPage,
              pageSize: versionPageSize,
              total: versionTotal,
              showSizeChanger: true,
              pageSizeOptions: [5, 10, 20, 50],
              showTotal: (t) => `共 ${t} 条`,
              onChange: (page, pageSize) => {
                if (!selectedPageKey) return;
                setVersionPage(page);
                setVersionPageSize(pageSize);
                loadVersionHistory(selectedPageKey, page, pageSize);
              },
            }}
            options={false}
            locale={{ emptyText: <Empty description="暂无版本历史" /> }}
          />
        </Space>
      </Drawer>

      <Drawer
        title="变更链"
        width={640}
        open={changeChainVisible}
        onClose={() => setChangeChainVisible(false)}
        loading={changeChainLoading}
      >
        {changeChain ? (
          <Space direction="vertical" style={{ width: '100%' }}>
            <Paragraph>
              <Text strong>页面：</Text> {changeChain.pageKey}
            </Paragraph>
            <Paragraph>
              <Text strong>资源：</Text> {changeChain.resourceKey}
            </Paragraph>
            <Paragraph>
              <Text strong>当前状态：</Text>
              函数版本: {changeChain.current.functionVersion || '-'}, 语义版本:{' '}
              {changeChain.current.semanticVersion || '-'}, 提案版本:{' '}
              {changeChain.current.proposalVersion || '-'}, 草稿版本:{' '}
              {changeChain.current.draftRevision || '-'}, 发布版本:{' '}
              {changeChain.current.publishedVersion || '-'}
            </Paragraph>
            <Timeline
              items={changeChain.items.map((item) => ({
                children: (
                  <Space direction="vertical" size={0}>
                    <Space>
                      <Tag color="blue">{item.type}</Tag>
                      <Text>{item.summary}</Text>
                    </Space>
                    <Text type="secondary">
                      {formatDate(item.timestamp)}
                      {item.actor ? ` - ${item.actor}` : ''}
                    </Text>
                  </Space>
                ),
              }))}
            />
          </Space>
        ) : (
          <Empty description="暂无变更记录" />
        )}
      </Drawer>

      <Drawer
        title="变更对比"
        width={840}
        open={diffVisible}
        onClose={() => setDiffVisible(false)}
        loading={diffLoading}
        extra={
          <Button type="primary" icon={<MergeOutlined />} onClick={() => setMergeVisible(true)}>
            合并变更
          </Button>
        }
      >
        {diffData ? (
          <Space direction="vertical" style={{ width: '100%' }}>
            <Paragraph>
              <Text strong>{diffData.summary}</Text>
            </Paragraph>
            {diffData.autoMergeItems?.length ? (
              <Card size="small" title={`可自动合并 ${diffData.autoMergeItems.length} 个展示字段`}>
                <Space direction="vertical" style={{ width: '100%' }}>
                  {diffData.autoMergeItems.map((item) => (
                    <Space key={item.field}>
                      <Tag color="blue">auto</Tag>
                      <Text code>{item.field}</Text>
                      <Text type="secondary">{item.reason}</Text>
                    </Space>
                  ))}
                </Space>
              </Card>
            ) : null}
            {diffData.conflictItems?.length ? (
              <Card size="small" title={`必须人工确认 ${diffData.conflictItems.length} 个冲突字段`}>
                <Space direction="vertical" style={{ width: '100%' }}>
                  {diffData.conflictItems.map((item) => (
                    <Space key={item.field}>
                      <Tag color="red">conflict</Tag>
                      <Text code>{item.field}</Text>
                      <Text type="secondary">{item.reason}</Text>
                    </Space>
                  ))}
                </Space>
              </Card>
            ) : null}
            {diffData.changes.map((change) => (
              <Card key={`${change.path}:${change.changeType}`} size="small">
                <Space direction="vertical" style={{ width: '100%' }}>
                  <Space>
                    <Tag
                      color={
                        change.changeType === 'added'
                          ? 'green'
                          : change.changeType === 'removed'
                            ? 'red'
                            : 'orange'
                      }
                    >
                      {change.changeType}
                    </Tag>
                    <Text code>{change.path}</Text>
                    {change.isSemantic && <Tag color="purple">语义变更</Tag>}
                  </Space>
                  {change.oldValue && (
                    <pre style={{ margin: 0, padding: 8, background: '#f5f5f5' }}>
                      {JSON.stringify(change.oldValue, null, 2)}
                    </pre>
                  )}
                  {change.newValue && (
                    <pre style={{ margin: 0, padding: 8, background: '#f5f5f5' }}>
                      {JSON.stringify(change.newValue, null, 2)}
                    </pre>
                  )}
                </Space>
              </Card>
            ))}
          </Space>
        ) : (
          <Empty description="暂无变更" />
        )}
      </Drawer>

      <Modal
        title="合并变更"
        open={mergeVisible}
        onCancel={() => setMergeVisible(false)}
        footer={[
          <Button key="cancel" onClick={() => setMergeVisible(false)}>
            取消
          </Button>,
          <Button key="auto" loading={mergeLoading} onClick={() => handleMerge('auto')}>
            自动合并
          </Button>,
          <Button
            key="manual"
            type="primary"
            loading={mergeLoading}
            onClick={handleOpenManualMerge}
          >
            手动处理冲突
          </Button>,
        ]}
      >
        <Paragraph>
          自动合并只会写入展示字段；binding、selector、权限、风险、审批和执行模式必须人工确认后重新发布。
        </Paragraph>
      </Modal>

      <MergeConflictModal
        open={manualMergeVisible}
        loading={mergeLoading}
        preview={manualMergePreview}
        onCancel={() => setManualMergeVisible(false)}
        onSubmit={handleManualMergeSubmit}
      />
    </PageContainer>
  );
}
