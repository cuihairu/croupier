/**
 * PageStudio - 语义化页面编辑器
 *
 * 使用强类型 PageSpec，不使用组件树式页面协议。
 * 支持 ResourcePage、OperationPage、TaskPage、ReportPage 的语义化编辑。
 */

import React, { useEffect, useState, useCallback } from 'react';
import { PageContainer, ProTable, type ProColumns } from '@ant-design/pro-components';
import {
  Alert,
  App,
  Button,
  Card,
  Drawer,
  Empty,
  Modal,
  Popconfirm,
  Space,
  Tag,
  Tabs,
  Typography,
} from 'antd';
import {
  CheckCircleOutlined,
  EyeOutlined,
  HistoryOutlined,
  ReloadOutlined,
  RocketOutlined,
  StopOutlined,
} from '@ant-design/icons';
import PageRenderer from '@/components/PageRenderer';
import {
  getPageDraft,
  listPageDrafts,
  publishPageDraft,
  savePageDraft,
  unpublishPage,
} from '@/services/api/pages';
import type {
  PageSpecDraftSummary,
  PageVersionItem,
  PageSpec,
  PageType,
} from '@/types/dashboard';

const { Text } = Typography;

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

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
    case 'resource': return '资源页面';
    case 'operation': return '操作页面';
    case 'task': return '任务页面';
    case 'report': return '报表页面';
    default: return type;
  }
}

// ---------------------------------------------------------------------------
// PageStudio Component
// ---------------------------------------------------------------------------

export default function PageStudio() {
  const { message } = App.useApp();
  const [drafts, setDrafts] = useState<PageSpecDraftSummary[]>([]);
  const [loading, setLoading] = useState(false);
  const [selectedDraft, setSelectedDraft] = useState<PageSpec | null>(null);
  const [selectedDraftRevision, setSelectedDraftRevision] = useState<number>(0);
  const [previewVisible, setPreviewVisible] = useState(false);
  const [versionsVisible, setVersionsVisible] = useState(false);
  const [versions, setVersions] = useState<PageVersionItem[]>([]);

  // 加载草稿列表
  const loadDrafts = useCallback(async () => {
    setLoading(true);
    try {
      const result = await listPageDrafts();
      setDrafts(result || []);
    } catch (error) {
      message.error('加载页面列表失败');
    } finally {
      setLoading(false);
    }
  }, [message]);

  useEffect(() => {
    loadDrafts();
  }, [loadDrafts]);

  // 加载草稿详情
  const loadDraftDetail = useCallback(async (pageKey: string) => {
    try {
      const draft = await getPageDraft(pageKey);
      setSelectedDraft(draft);
      setSelectedDraftRevision(draft.draftRevision || 0);
    } catch (error) {
      message.error('加载页面详情失败');
    }
  }, [message]);

  // 发布页面
  const handlePublish = useCallback(async (pageKey: string) => {
    try {
      await publishPageDraft(pageKey, selectedDraftRevision);
      message.success('发布成功');
      loadDrafts();
    } catch (error) {
      message.error('发布失败');
    }
  }, [message, loadDrafts, selectedDraftRevision]);

  // 取消发布
  const handleUnpublish = useCallback(async (pageKey: string) => {
    try {
      await unpublishPage(pageKey);
      message.success('已取消发布');
      loadDrafts();
    } catch (error) {
      message.error('取消发布失败');
    }
  }, [message, loadDrafts]);

  // 预览页面
  const handlePreview = useCallback((pageKey: string) => {
    loadDraftDetail(pageKey);
    setPreviewVisible(true);
  }, [loadDraftDetail]);

  // 表格列定义
  const columns: ProColumns<PageSpecDraftSummary>[] = [
    {
      title: '页面标识',
      dataIndex: 'pageKey',
      key: 'pageKey',
      width: 200,
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
      render: (_, record) => (
        <Tag color={statusColor(record.status)}>{record.status}</Tag>
      ),
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
      width: 200,
      render: (_, record) => (
        <Space>
          <Button
            type="link"
            size="small"
            icon={<EyeOutlined />}
            onClick={() => handlePreview(record.pageKey)}
          >
            预览
          </Button>
          {record.status === 'draft' ? (
            <Popconfirm
              title="确认发布此页面？"
              onConfirm={() => handlePublish(record.pageKey)}
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
      title="页面工作室"
      subTitle="管理语义化页面"
    >
      <Card>
        <ProTable<PageSpecDraftSummary>
          columns={columns}
          dataSource={drafts}
          loading={loading}
          rowKey="pageKey"
          search={false}
          pagination={false}
          toolBarRender={() => [
            <Button
              key="refresh"
              icon={<ReloadOutlined />}
              onClick={loadDrafts}
            >
              刷新
            </Button>,
          ]}
        />
      </Card>

      {/* 预览抽屉 */}
      <Drawer
        title="页面预览"
        width={800}
        open={previewVisible}
        onClose={() => setPreviewVisible(false)}
      >
        {selectedDraft ? (
          <PageRenderer
            pageSpec={selectedDraft}
            onExecute={async (bindingId, payload) => {
              console.log('Preview execute:', bindingId, payload);
              return { kind: 'sync', requestId: 'preview', data: {} };
            }}
          />
        ) : (
          <Empty description="请选择页面" />
        )}
      </Drawer>
    </PageContainer>
  );
}
