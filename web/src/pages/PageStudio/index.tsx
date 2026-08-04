import React, { useCallback, useEffect, useState } from 'react';
import { PageContainer, ProTable, type ProColumns } from '@ant-design/pro-components';
import {
  App,
  Button,
  Card,
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
import PageEditor from '@/components/PageEditor';
import PageRenderer from '@/components/PageRenderer';
import ProposalInbox from '@/components/ProposalInbox';
import {
  getPageDraft,
  listPageDrafts,
  publishPageDraft,
  savePageDraft,
  unpublishPage,
} from '@/services/api/pages';
import {
  getChangeChain,
  getDiff,
  mergeChanges,
  type ChangeChain,
  type DiffResponse,
  type MergeStrategy,
} from '@/services/api/versioning';
import type { JSONValue, PageSpec, PageSpecDraftSummary, PageType } from '@/types/dashboard';

const { Paragraph, Text } = Typography;

function localizedText(text: Record<string, string> | undefined, fallback: string): string {
  if (!text) return fallback;
  return text['zh-CN'] || text['en-US'] || Object.values(text).find((value) => value.trim()) || fallback;
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
  const [changeChainVisible, setChangeChainVisible] = useState(false);
  const [changeChain, setChangeChain] = useState<ChangeChain | null>(null);
  const [changeChainLoading, setChangeChainLoading] = useState(false);
  const [selectedResourceKey, setSelectedResourceKey] = useState('');
  const [diffVisible, setDiffVisible] = useState(false);
  const [diffData, setDiffData] = useState<DiffResponse | null>(null);
  const [diffLoading, setDiffLoading] = useState(false);
  const [mergeVisible, setMergeVisible] = useState(false);
  const [mergeLoading, setMergeLoading] = useState(false);

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

  const handleChangeChain = useCallback(
    async (resourceKey: string) => {
      setSelectedResourceKey(resourceKey);
      setChangeChainVisible(true);
      setChangeChainLoading(true);
      try {
        setChangeChain(await getChangeChain(resourceKey));
      } catch {
        message.error('加载变更链失败');
      } finally {
        setChangeChainLoading(false);
      }
    },
    [message],
  );

  const handleDiff = useCallback(
    async (resourceKey: string) => {
      setSelectedResourceKey(resourceKey);
      setDiffVisible(true);
      setDiffLoading(true);
      try {
        setDiffData(await getDiff(resourceKey));
      } catch {
        message.error('加载 Diff 失败');
      } finally {
        setDiffLoading(false);
      }
    },
    [message],
  );

  const handleMerge = useCallback(
    async (strategy: MergeStrategy) => {
      setMergeLoading(true);
      try {
        const result = await mergeChanges(selectedResourceKey, { strategy });
        message.success(`合并完成：${result.merged} 项自动合并，${result.conflicts} 项冲突`);
        setMergeVisible(false);
        loadDrafts();
      } catch {
        message.error('合并失败');
      } finally {
        setMergeLoading(false);
      }
    },
    [loadDrafts, message, selectedResourceKey],
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
          <Button type="link" size="small" icon={<EditOutlined />} onClick={() => handleEdit(record.pageKey)}>
            编辑
          </Button>
          <Button type="link" size="small" icon={<EyeOutlined />} onClick={() => handlePreview(record.pageKey)}>
            预览
          </Button>
          <Button
            type="link"
            size="small"
            icon={<HistoryOutlined />}
            onClick={() => record.resourceKey && handleChangeChain(record.resourceKey)}
            disabled={!record.resourceKey}
          >
            变更链
          </Button>
          <Button
            type="link"
            size="small"
            icon={<DiffOutlined />}
            onClick={() => record.resourceKey && handleDiff(record.resourceKey)}
            disabled={!record.resourceKey}
          >
            Diff
          </Button>
          {record.status === 'draft' ? (
            <Popconfirm title="确认发布此页面？" onConfirm={() => handlePublish(record.pageKey, record.draftRevision)}>
              <Button type="link" size="small" icon={<RocketOutlined />}>
                发布
              </Button>
            </Popconfirm>
          ) : (
            <Popconfirm title="确认取消发布此页面？" onConfirm={() => handleUnpublish(record.pageKey)}>
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
    <PageContainer title="页面工作室" subTitle="从默认页面提案发布；只有不满意时再编辑页面">
      <ProposalInbox />

      <Card
        title="已接受和编辑中的页面"
        style={{ marginTop: 16 }}
        extra={<Text type="secondary">草稿用于人工调整已接受的 PageSpec；默认生成入口在上方三队列。</Text>}
      >
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
      </Card>

      <Drawer title="页面预览" width={900} open={previewVisible} onClose={() => setPreviewVisible(false)}>
        {selectedDraft ? (
          <PageRenderer
            pageSpec={selectedDraft}
            onExecute={async (): Promise<{ kind: 'sync'; requestId: string; data: JSONValue }> => ({
              kind: 'sync',
              requestId: 'preview',
              data: {},
            })}
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
        {selectedDraft ? <PageEditor value={selectedDraft} onChange={setSelectedDraft} /> : <Empty description="请选择页面" />}
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
              <Text strong>资源：</Text> {changeChain.resourceKey}
            </Paragraph>
            <Paragraph>
              <Text strong>当前状态：</Text>
              函数版本: {changeChain.current.functionVersion || '-'}, 语义版本: {changeChain.current.semanticVersion || '-'},
              提案版本: {changeChain.current.proposalVersion || '-'}, 草稿版本: {changeChain.current.draftRevision || '-'},
              发布版本: {changeChain.current.publishedVersion || '-'}
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
            {diffData.changes.map((change) => (
              <Card key={`${change.path}:${change.changeType}`} size="small">
                <Space direction="vertical" style={{ width: '100%' }}>
                  <Space>
                    <Tag color={change.changeType === 'added' ? 'green' : change.changeType === 'removed' ? 'red' : 'orange'}>
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
          <Button key="manual" type="primary" onClick={() => setEditorVisible(true)}>
            手动处理冲突
          </Button>,
        ]}
      >
        <Paragraph>
          自动合并只会写入展示字段；binding、selector、权限、风险、审批和执行模式必须人工确认后重新发布。
        </Paragraph>
      </Modal>
    </PageContainer>
  );
}
