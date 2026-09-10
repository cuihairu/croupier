import React from 'react';
import { App, Button, Dropdown, Popconfirm, Space, Tag, Tooltip, Typography } from 'antd';
import {
  CheckOutlined,
  CloseOutlined,
  ExclamationCircleOutlined,
  FileTextOutlined,
  MoreOutlined,
  RocketOutlined,
} from '@ant-design/icons';
import type { ColumnsType } from 'antd/es/table';
import type {
  BlockedProposalIssue,
  PageProposal,
  PageType,
  ProposalQuality,
  ProposalStatus,
} from '@/types/dashboard';
import { localizedText } from '@/utils/localizedText';
import { navigateTo } from './urlFocus';
import {
  diagnosticSummary,
  formatDate,
  pageTypeColors,
  pageTypeLabels,
  qualityColors,
  qualityLabels,
  statusColors,
  statusLabels,
} from './shared';

const { Text } = Typography;

/** 提案/阻断项列定义：操作回调由页面注入（modal 实例用于接受/拒绝二次确认）。 */

export function buildProposalColumns({
  modal,
  onViewDetail,
  onPreview,
  onAccept,
  onAcceptAndPublish,
  onReview,
  onReject,
}: {
  modal: ReturnType<typeof App.useApp>['modal'];
  onViewDetail: (proposalKey: string) => void;
  onPreview: (proposalKey: string) => void;
  onAccept: (proposal: PageProposal) => Promise<void>;
  onAcceptAndPublish: (proposal: PageProposal) => Promise<void>;
  onReview: (proposal: PageProposal) => Promise<void>;
  onReject: (proposalKey: string) => Promise<void>;
}): ColumnsType<PageProposal> {
  return [
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
                onOk: () => onAccept(record),
              }),
          });
          if (record.quality === 'needs_review') {
            moreItems.push({
              key: 'review',
              icon: <ExclamationCircleOutlined />,
              label: '处理',
              onClick: () => onReview(record),
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
                onOk: () => onReject(record.proposalKey),
              }),
          });
        }
        return (
          <Space size={0}>
            <Button type="link" size="small" onClick={() => onViewDetail(record.proposalKey)}>
              查看
            </Button>
            <Button type="link" size="small" onClick={() => onPreview(record.proposalKey)}>
              预览
            </Button>
            {record.status === 'pending' &&
              (record.quality === 'ready' || record.quality === 'basic') &&
              !record.pageExists && (
                <Popconfirm
                  title="发布默认页面？"
                  description="会创建草稿并发布到运行控制台左侧动态菜单。"
                  onConfirm={() => onAcceptAndPublish(record)}
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
}

export function buildBlockedColumns(): ColumnsType<BlockedProposalIssue> {
  return [
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
}
