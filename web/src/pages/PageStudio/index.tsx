import React, { useCallback, useEffect, useRef, useState } from 'react';
import { FormattedMessage, history, useIntl } from '@umijs/max';
import { PageContainer, ProTable } from '@ant-design/pro-components';
import { App, Button, Collapse, Space, Typography } from 'antd';
import { ReloadOutlined, RocketOutlined } from '@ant-design/icons';
import MergeConflictModal from '@/components/MergeConflictModal';
import ProposalInbox from '@/components/ProposalInbox';
import PageWorkflowGuide from '@/components/PageWorkflowGuide';
import PreviewDrawer from './studio/PreviewDrawer';
import EditorModal from './studio/EditorModal';
import VersionsDrawer from './studio/VersionsDrawer';
import ChangeChainDrawer from './studio/ChangeChainDrawer';
import DiffDrawer from './studio/DiffDrawer';
import MergeModal from './studio/MergeModal';
import { buildDraftColumns } from './studio/draftColumns';
import { currentFocusPageKey } from './studio/shared';
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
  PageVersionItem,
} from '@/types/dashboard';
import { requestConsoleMenuRefresh } from '@/utils/consoleMenu';
import { localizedText } from '@/utils/localizedText';
import { extractErrorDetails, extractErrorMessage } from '@/utils/errors';

const { Text } = Typography;

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
  const intl = useIntl();
  // useIntl 在测试 mock 下每次渲染返回新引用，直接进 useCallback 依赖会让请求
  // effect 反复触发；回调内文案统一走 intlRef（渲染期同步写回）。
  const intlRef = useRef(intl);
  intlRef.current = intl;
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
      message.error(
        intlRef.current.formatMessage({
          id: 'pages.pageStudio.load.listFailed',
          defaultMessage: '加载页面列表失败',
        }),
      );
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
        message.error(
          intlRef.current.formatMessage({
            id: 'pages.pageStudio.load.detailFailed',
            defaultMessage: '加载页面详情失败',
          }),
        );
      }
    },
    [message],
  );

  const handlePublish = useCallback(
    async (pageKey: string, draftRevision: number) => {
      try {
        await publishPageDraft(pageKey, draftRevision);
        requestConsoleMenuRefresh();
        message.success(
          intlRef.current.formatMessage({
            id: 'pages.pageStudio.publish.success',
            defaultMessage: '发布成功',
          }),
        );
        loadDrafts();
      } catch {
        message.error(
          intlRef.current.formatMessage({
            id: 'pages.pageStudio.publish.failed',
            defaultMessage: '发布失败',
          }),
        );
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
        message.warning(
          intlRef.current.formatMessage(
            {
              id: 'pages.pageStudio.bulkPublish.partialWarning',
              defaultMessage: '已发布 {published} 个页面，{failed} 个失败',
            },
            { published, failed },
          ),
        );
      } else {
        message.success(
          intlRef.current.formatMessage(
            {
              id: 'pages.pageStudio.bulkPublish.success',
              defaultMessage: '已发布 {published} 个页面',
            },
            { published },
          ),
        );
      }
      requestConsoleMenuRefresh();
      loadDrafts();
    } catch {
      message.error(
        intlRef.current.formatMessage({
          id: 'pages.pageStudio.bulkPublish.failed',
          defaultMessage: '一键发布失败',
        }),
      );
    } finally {
      setBulkLoading(null);
    }
  }, [loadDrafts]);

  const handleBulkUnpublish = useCallback(async () => {
    setBulkLoading('unpublish');
    try {
      const res = await bulkUnpublishPages();
      const unpublished = res.unpublished?.length ?? 0;
      message.success(
        intlRef.current.formatMessage(
          {
            id: 'pages.pageStudio.bulkUnpublish.success',
            defaultMessage: '已下架 {unpublished} 个页面',
          },
          { unpublished },
        ),
      );
      requestConsoleMenuRefresh();
      loadDrafts();
    } catch {
      message.error(
        intlRef.current.formatMessage({
          id: 'pages.pageStudio.bulkUnpublish.failed',
          defaultMessage: '一键下架失败',
        }),
      );
    } finally {
      setBulkLoading(null);
    }
  }, [loadDrafts]);

  const handleUnpublish = useCallback(
    async (pageKey: string) => {
      try {
        await unpublishPage(pageKey);
        requestConsoleMenuRefresh();
        message.success(
          intlRef.current.formatMessage({
            id: 'pages.pageStudio.unpublish.success',
            defaultMessage: '已取消发布',
          }),
        );
        loadDrafts();
      } catch {
        message.error(
          intlRef.current.formatMessage({
            id: 'pages.pageStudio.unpublish.failed',
            defaultMessage: '取消发布失败',
          }),
        );
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
      // inbox=1 表示来自「打开 Proposal Inbox」入口：只定位 Inbox 队列项
      // （focusPageKey 驱动切 Tab + 行高亮），不自动打开编辑器。
      if (new URLSearchParams(window.location.search).get('inbox') !== '1') {
        handleEdit(key);
      }
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
            message.success(
              intlRef.current.formatMessage({
                id: 'pages.pageStudio.save.savedAndPublished',
                defaultMessage: '已保存并发布',
              }),
            );
          } catch (publishError) {
            // 草稿已保存，仅发布失败：保留编辑器打开让用户决定重试或稍后发布
            const reason = extractErrorMessage(
              publishError,
              intlRef.current.formatMessage({
                id: 'pages.pageStudio.publish.unknownReason',
                defaultMessage: '未知原因',
              }),
            );
            modal.error({
              title: intlRef.current.formatMessage({
                id: 'pages.pageStudio.publish.failedDraftSaved',
                defaultMessage: '发布失败（草稿已保存）',
              }),
              width: 560,
              content: (
                <Space orientation="vertical" size={4} style={{ width: '100%' }}>
                  <Typography.Text type="secondary">
                    {intlRef.current.formatMessage(
                      {
                        id: 'pages.pageStudio.publish.failureReason',
                        defaultMessage: '失败原因：{reason}',
                      },
                      { reason },
                    )}
                  </Typography.Text>
                  <ErrorDetailList error={publishError} />
                  <Typography.Text type="warning">
                    <FormattedMessage
                      id="pages.pageStudio.publish.contractStaleHint"
                      defaultMessage="通常是函数契约已变化导致页面绑定失效，可点击「重新生成草稿」按最新契约重建后再发布。"
                    />
                  </Typography.Text>
                </Space>
              ),
            });
            loadDrafts();
            return;
          }
        } else {
          message.success(
            intlRef.current.formatMessage({
              id: 'pages.pageStudio.save.success',
              defaultMessage: '保存成功',
            }),
          );
        }
        setEditorVisible(false);
        loadDrafts();
      } catch (saveError) {
        modal.error({
          title: intlRef.current.formatMessage({
            id: 'pages.pageStudio.save.failed',
            defaultMessage: '保存失败',
          }),
          width: 560,
          content: (
            <Space orientation="vertical" size={4} style={{ width: '100%' }}>
              <Typography.Text type="secondary">
                {extractErrorMessage(
                  saveError,
                  intlRef.current.formatMessage({
                    id: 'pages.pageStudio.save.retryLater',
                    defaultMessage: '保存失败，请稍后重试',
                  }),
                )}
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
        message.success(
          intlRef.current.formatMessage({
            id: 'pages.pageStudio.regenerate.success',
            defaultMessage: '已按最新 Proposal 重新生成草稿',
          }),
        );
        await loadDrafts();
      } catch {
        message.error(
          intlRef.current.formatMessage({
            id: 'pages.pageStudio.regenerate.failed',
            defaultMessage: '重新生成草稿失败',
          }),
        );
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
        message.error(
          intlRef.current.formatMessage({
            id: 'pages.pageStudio.changeChain.loadFailed',
            defaultMessage: '加载变更链失败',
          }),
        );
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
        message.error(
          intlRef.current.formatMessage({
            id: 'pages.pageStudio.diff.loadFailed',
            defaultMessage: '加载 Diff 失败',
          }),
        );
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
        message.error(
          intlRef.current.formatMessage({
            id: 'pages.pageStudio.versions.loadFailed',
            defaultMessage: '加载版本历史失败',
          }),
        );
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
        message.error(
          intlRef.current.formatMessage({
            id: 'pages.pageStudio.merge.invalidRevision',
            defaultMessage: '页面草稿版本无效，请刷新后重试',
          }),
        );
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
        message.success(
          intlRef.current.formatMessage(
            {
              id: 'pages.pageStudio.merge.autoSuccess',
              defaultMessage: '合并完成：{merged} 项自动合并，{conflicts} 项冲突',
            },
            { merged: result.merged, conflicts: result.conflicts },
          ),
        );
        setMergeVisible(false);
        loadDrafts();
      } catch {
        message.error(
          intlRef.current.formatMessage({
            id: 'pages.pageStudio.merge.failed',
            defaultMessage: '合并失败',
          }),
        );
      } finally {
        setMergeLoading(false);
      }
    },
    [loadDrafts, message, selectedDraftRevision, selectedPageKey],
  );

  const handleOpenManualMerge = useCallback(async () => {
    if (!selectedPageKey) {
      message.error(
        intlRef.current.formatMessage({
          id: 'pages.pageStudio.merge.selectPageFirst',
          defaultMessage: '请选择页面后再处理冲突',
        }),
      );
      return;
    }
    setMergeLoading(true);
    try {
      const preview = await mergeChanges(selectedPageKey, { strategy: 'manual', dryRun: true });
      setManualMergePreview(preview);
      setMergeVisible(false);
      setManualMergeVisible(true);
    } catch {
      message.error(
        intlRef.current.formatMessage({
          id: 'pages.pageStudio.merge.previewLoadFailed',
          defaultMessage: '加载冲突预览失败',
        }),
      );
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
        message.success(
          intlRef.current.formatMessage(
            {
              id: 'pages.pageStudio.merge.manualSuccess',
              defaultMessage: '合并完成：草稿已更新到版本 {revision}',
            },
            { revision: result.draftRevision || '-' },
          ),
        );
        setManualMergeVisible(false);
        setManualMergePreview(null);
        loadDrafts();
      } catch {
        message.error(
          intlRef.current.formatMessage({
            id: 'pages.pageStudio.merge.manualFailed',
            defaultMessage: '手动合并失败',
          }),
        );
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
        message.error(
          intlRef.current.formatMessage({
            id: 'pages.pageStudio.rollback.draftFailed',
            defaultMessage: '回滚草稿失败',
          }),
        );
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
        message.error(
          intlRef.current.formatMessage({
            id: 'pages.pageStudio.rollback.publishFailed',
            defaultMessage: '回滚发布失败',
          }),
        );
      }
    },
    [loadDrafts, loadVersionHistory, message, selectedDraftRevision, selectedPageKey],
  );

  const columns = buildDraftColumns(
    {
      onEdit: handleEdit,
      onPreview: handlePreview,
      onPublish: handlePublish,
      onUnpublish: handleUnpublish,
      onRegenerate: handleRegenerate,
      onVersions: handleVersions,
      onChangeChain: handleChangeChain,
      onDiff: handleDiff,
    },
    modal,
  );

  return (
    <PageContainer
      title={intl.formatMessage({ id: 'pages.pageStudio.title', defaultMessage: '页面工作台' })}
      subTitle={intl.formatMessage({
        id: 'pages.pageStudio.subtitle',
        defaultMessage: '注册能力后自动生成默认页面；预览、发布、运行无需手工创建页面',
      })}
    >
      <PageWorkflowGuide />

      <ProposalInbox focusPageKey={focusPageKey} />

      <Collapse
        style={{ marginTop: 16 }}
        items={[
          {
            key: 'advanced-page-management',
            label: intl.formatMessage({
              id: 'pages.pageStudio.advancedPanel.label',
              defaultMessage: '高级页面管理（仅在已接受草稿、处理版本或回滚时使用）',
            }),
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
                    <FormattedMessage id="pages.pageStudio.action.refresh" defaultMessage="刷新" />
                  </Button>,
                  <Button
                    key="bulk-publish"
                    icon={<RocketOutlined />}
                    loading={bulkLoading === 'publish'}
                    onClick={() => {
                      modal.confirm({
                        title: intl.formatMessage({
                          id: 'pages.pageStudio.bulkPublish.confirmTitle',
                          defaultMessage: '一键发布全部',
                        }),
                        content: intl.formatMessage({
                          id: 'pages.pageStudio.bulkPublish.confirmContent',
                          defaultMessage:
                            '将重算提案并把所有 ready/basic 提案按真实链路发布（同 scope）。确认执行？',
                        }),
                        okText: intl.formatMessage({
                          id: 'pages.pageStudio.bulkPublish.confirmOk',
                          defaultMessage: '发布',
                        }),
                        onOk: handleBulkPublish,
                      });
                    }}
                  >
                    <FormattedMessage
                      id="pages.pageStudio.bulkPublish.button"
                      defaultMessage="一键发布全部"
                    />
                  </Button>,
                  <Button
                    key="bulk-unpublish"
                    danger
                    loading={bulkLoading === 'unpublish'}
                    onClick={() => {
                      modal.confirm({
                        title: intl.formatMessage({
                          id: 'pages.pageStudio.bulkUnpublish.confirmTitle',
                          defaultMessage: '一键下架全部',
                        }),
                        content: intl.formatMessage({
                          id: 'pages.pageStudio.bulkUnpublish.confirmContent',
                          defaultMessage:
                            '将下线当前 scope 内全部已发布页面（运行控制台菜单随之清空）。确认执行？',
                        }),
                        okText: intl.formatMessage({
                          id: 'pages.pageStudio.bulkUnpublish.confirmOk',
                          defaultMessage: '下架',
                        }),
                        okButtonProps: { danger: true },
                        onOk: handleBulkUnpublish,
                      });
                    }}
                  >
                    <FormattedMessage
                      id="pages.pageStudio.bulkUnpublish.button"
                      defaultMessage="一键下架全部"
                    />
                  </Button>,
                ]}
              />
            ),
          },
        ]}
      />

      <PreviewDrawer
        open={previewVisible}
        draft={selectedDraft}
        onClose={() => setPreviewVisible(false)}
      />

      <EditorModal
        open={editorVisible}
        pageKey={selectedPageKey}
        draft={selectedDraft}
        livePreview={livePreview}
        saving={saving}
        onClose={() => setEditorVisible(false)}
        onLivePreviewChange={setLivePreview}
        onSave={(options) => void handleSave(options)}
        onSpecChange={updateSelectedDraftSpec}
      />

      <VersionsDrawer
        open={versionsVisible}
        pageKey={selectedPageKey}
        items={versionItems}
        loading={versionsLoading}
        page={versionPage}
        pageSize={versionPageSize}
        total={versionTotal}
        currentDraftVersion={currentDraftVersion}
        currentPublishedVersion={currentPublishedVersion}
        onClose={() => setVersionsVisible(false)}
        onPageChange={(p, ps) => {
          if (!selectedPageKey) return;
          setVersionPage(p);
          setVersionPageSize(ps);
          loadVersionHistory(selectedPageKey, p, ps);
        }}
        onRollbackDraft={(version) => void handleRollbackDraftVersion(version)}
        onRollbackPublished={(version) => void handleRollbackPublishedVersion(version)}
      />

      <ChangeChainDrawer
        open={changeChainVisible}
        chain={changeChain}
        loading={changeChainLoading}
        onClose={() => setChangeChainVisible(false)}
      />

      <DiffDrawer
        open={diffVisible}
        data={diffData}
        loading={diffLoading}
        onClose={() => setDiffVisible(false)}
        onMerge={() => setMergeVisible(true)}
      />

      <MergeModal
        open={mergeVisible}
        loading={mergeLoading}
        onCancel={() => setMergeVisible(false)}
        onAutoMerge={() => void handleMerge('auto')}
        onManualMerge={() => void handleOpenManualMerge()}
      />

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
