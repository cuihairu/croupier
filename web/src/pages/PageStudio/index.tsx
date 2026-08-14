import React, { useCallback, useEffect, useState } from 'react';
import { PageContainer, ProTable, type ProColumns } from '@ant-design/pro-components';
import {
  App,
  Button,
  Card,
  Collapse,
  Drawer,
  Empty,
  Modal,
  Popconfirm,
  Space,
  Tag,
  Timeline,
  Typography,
} from 'antd';
import {
  DiffOutlined,
  EditOutlined,
  EyeOutlined,
  HistoryOutlined,
  MergeOutlined,
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
import type { PageSpec, PageSpecDraftSummary, PageType, PageVersionItem } from '@/types/dashboard';
import { requestConsoleMenuRefresh } from '@/utils/consoleMenu';

const { Paragraph, Text } = Typography;

function localizedText(text: Record<string, string> | undefined, fallback: string): string {
  if (!text) return fallback;
  return (
    text['zh-CN'] || text['en-US'] || Object.values(text).find((value) => value.trim()) || fallback
  );
}

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

export default function PageStudio() {
  const { message } = App.useApp();
  const [drafts, setDrafts] = useState<PageSpecDraftSummary[]>([]);
  const [loading, setLoading] = useState(false);
  const [selectedDraft, setSelectedDraft] = useState<PageSpec | null>(null);
  const [selectedDraftRevision, setSelectedDraftRevision] = useState(0);
  const [previewVisible, setPreviewVisible] = useState(false);
  const [editorVisible, setEditorVisible] = useState(false);
  const [saving, setSaving] = useState(false);
  const [regeneratingPageKey, setRegeneratingPageKey] = useState('');
  const [changeChainVisible, setChangeChainVisible] = useState(false);
  const [changeChain, setChangeChain] = useState<ChangeChain | null>(null);
  const [changeChainLoading, setChangeChainLoading] = useState(false);
  const [selectedPageKey, setSelectedPageKey] = useState('');
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
    (pageKey: string) => {
      loadDraftDetail(pageKey);
      setEditorVisible(true);
    },
    [loadDraftDetail],
  );

  useEffect(() => {
    const focusPageKey = currentFocusPageKey();
    if (focusPageKey) {
      handleEdit(focusPageKey);
    }
  }, [handleEdit]);

  const handleSave = useCallback(async () => {
    if (!selectedDraft) return;
    setSaving(true);
    try {
      const result = await savePageDraft({
        ...selectedDraft,
        draftRevision: selectedDraftRevision,
      });
      setSelectedDraftRevision(result.draftRevision);
      message.success('保存成功');
      setEditorVisible(false);
      loadDrafts();
    } catch {
      message.error('保存失败');
    } finally {
      setSaving(false);
    }
  }, [loadDrafts, message, selectedDraft, selectedDraftRevision]);

  const handleRegenerate = useCallback(
    async (pageKey: string, draftRevision: number) => {
      setRegeneratingPageKey(pageKey);
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
        setRegeneratingPageKey('');
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
    async (pageKey: string) => {
      setVersionsLoading(true);
      try {
        const result = await listPageVersions(pageKey);
        setVersionItems(result.items || []);
        setCurrentDraftVersion(result.currentDraftRevision || 0);
        setCurrentPublishedVersion(result.currentPublishedVersion || 0);
      } catch {
        message.error('加载版本历史失败');
      } finally {
        setVersionsLoading(false);
      }
    },
    [message],
  );

  const handleVersions = useCallback(
    async (pageKey: string) => {
      setSelectedPageKey(pageKey);
      setVersionsVisible(true);
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
      width: 400,
      render: (_, record) => (
        <Space>
          <Button
            type="link"
            size="small"
            icon={<EditOutlined />}
            onClick={() => handleEdit(record.pageKey)}
          >
            编辑
          </Button>
          <Button
            type="link"
            size="small"
            icon={<EyeOutlined />}
            onClick={() => handlePreview(record.pageKey)}
          >
            预览
          </Button>
          <Popconfirm
            title="确认按最新 Proposal 重新生成草稿？"
            description="当前草稿修改将被最新 Proposal 覆盖，已发布版本不会变更。"
            onConfirm={() => handleRegenerate(record.pageKey, record.draftRevision)}
          >
            <Button
              type="link"
              size="small"
              icon={<ReloadOutlined />}
              loading={regeneratingPageKey === record.pageKey}
            >
              重新生成
            </Button>
          </Popconfirm>
          <Button
            type="link"
            size="small"
            icon={<HistoryOutlined />}
            onClick={() => handleChangeChain(record.pageKey)}
          >
            变更链
          </Button>
          <Button
            type="link"
            size="small"
            icon={<DiffOutlined />}
            onClick={() => handleDiff(record.pageKey)}
          >
            Diff
          </Button>
          <Button type="link" size="small" onClick={() => handleVersions(record.pageKey)}>
            版本
          </Button>
          {record.status === 'draft' ? (
            <Popconfirm
              title="确认发布此页面？"
              onConfirm={() => handlePublish(record.pageKey, record.draftRevision)}
            >
              <Button type="link" size="small" icon={<RocketOutlined />}>
                发布
              </Button>
            </Popconfirm>
          ) : (
            <Popconfirm
              title="确认取消发布此页面？"
              onConfirm={() => handleUnpublish(record.pageKey)}
            >
              <Button type="link" size="small" icon={<StopOutlined />} danger>
                取消发布
              </Button>
            </Popconfirm>
          )}
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

      <ProposalInbox />

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
                    刷新草稿
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

      <Drawer
        title="页面编辑"
        width={900}
        open={editorVisible}
        onClose={() => setEditorVisible(false)}
        extra={
          <Space>
            <Button onClick={() => setEditorVisible(false)}>取消</Button>
            <Button type="primary" loading={saving} onClick={handleSave}>
              保存
            </Button>
          </Space>
        }
      >
        {selectedDraft ? (
          <PageEditor value={selectedDraft} onChange={setSelectedDraft} />
        ) : (
          <Empty description="请选择页面" />
        )}
      </Drawer>

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
            pagination={false}
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
