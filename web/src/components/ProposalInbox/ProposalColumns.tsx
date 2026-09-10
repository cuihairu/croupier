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
import { FormattedMessage } from '@umijs/max';
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

/** 模块级文案助手接收 intl 的最小结构（@umijs/max 未导出 IntlShape 类型） */
type IntlFormatter = {
  formatMessage: (
    descriptor: { id: string; defaultMessage: string },
    values?: Record<string, string | number>,
  ) => string;
};

/** 提案/阻断项列定义：操作回调由页面注入（modal 实例用于接受/拒绝二次确认）。 */

export function buildProposalColumns({
  intl,
  modal,
  onViewDetail,
  onPreview,
  onAccept,
  onAcceptAndPublish,
  onReview,
  onReject,
}: {
  intl: IntlFormatter;
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
      title: intl.formatMessage({
        id: 'component.proposalInbox.column.proposal',
        defaultMessage: '提案',
      }),
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
      title: intl.formatMessage({
        id: 'component.proposalInbox.column.title',
        defaultMessage: '标题',
      }),
      dataIndex: 'title',
      key: 'title',
      render: (_, record) => localizedText(record.title, record.pageKey),
    },
    {
      title: intl.formatMessage({
        id: 'component.proposalInbox.column.type',
        defaultMessage: '类型',
      }),
      dataIndex: 'pageType',
      key: 'pageType',
      width: 90,
      render: (type: PageType) => (
        <Tag color={pageTypeColors[type]}>{pageTypeLabels[type] || type}</Tag>
      ),
    },
    {
      title: intl.formatMessage({
        id: 'component.proposalInbox.column.resource',
        defaultMessage: '资源',
      }),
      dataIndex: 'resourceKey',
      key: 'resourceKey',
      width: 140,
      render: (value) => value || '-',
    },
    {
      title: intl.formatMessage({
        id: 'component.proposalInbox.column.quality',
        defaultMessage: '质量',
      }),
      dataIndex: 'quality',
      key: 'quality',
      width: 120,
      render: (quality: ProposalQuality) => (
        <Tag color={qualityColors[quality]}>{qualityLabels[quality]}</Tag>
      ),
    },
    {
      title: intl.formatMessage({
        id: 'component.proposalInbox.column.status',
        defaultMessage: '状态',
      }),
      dataIndex: 'status',
      key: 'status',
      width: 100,
      render: (status: ProposalStatus) => (
        <Tag color={statusColors[status]}>{statusLabels[status]}</Tag>
      ),
    },
    {
      title: intl.formatMessage({
        id: 'component.proposalInbox.column.diagnostics',
        defaultMessage: '诊断',
      }),
      dataIndex: 'diagnostics',
      key: 'diagnostics',
      width: 160,
      render: diagnosticSummary,
    },
    {
      title: intl.formatMessage({
        id: 'component.proposalInbox.column.updatedAt',
        defaultMessage: '更新时间',
      }),
      dataIndex: 'updatedAt',
      key: 'updatedAt',
      width: 180,
      render: formatDate,
    },
    {
      title: intl.formatMessage({
        id: 'component.proposalInbox.column.actions',
        defaultMessage: '操作',
      }),
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
            label: intl.formatMessage({
              id: 'component.proposalInbox.column.action.customize',
              defaultMessage: '自定义编辑',
            }),
            onClick: () =>
              modal.confirm({
                title: intl.formatMessage({
                  id: 'component.proposalInbox.column.action.customizeConfirm',
                  defaultMessage: '接受为草稿并自定义页面？',
                }),
                onOk: () => onAccept(record),
              }),
          });
          if (record.quality === 'needs_review') {
            moreItems.push({
              key: 'review',
              icon: <ExclamationCircleOutlined />,
              label: intl.formatMessage({
                id: 'component.proposalInbox.column.action.review',
                defaultMessage: '处理',
              }),
              onClick: () => onReview(record),
            });
          }
          moreItems.push({
            key: 'reject',
            icon: <CloseOutlined />,
            label: intl.formatMessage({
              id: 'component.proposalInbox.column.action.reject',
              defaultMessage: '拒绝',
            }),
            danger: true,
            onClick: () =>
              modal.confirm({
                title: intl.formatMessage({
                  id: 'component.proposalInbox.column.action.rejectConfirm',
                  defaultMessage: '拒绝此提案？',
                }),
                onOk: () => onReject(record.proposalKey),
              }),
          });
        }
        return (
          <Space size={0}>
            <Button type="link" size="small" onClick={() => onViewDetail(record.proposalKey)}>
              <FormattedMessage
                id="component.proposalInbox.column.action.view"
                defaultMessage="查看"
              />
            </Button>
            <Button type="link" size="small" onClick={() => onPreview(record.proposalKey)}>
              <FormattedMessage
                id="component.proposalInbox.column.action.preview"
                defaultMessage="预览"
              />
            </Button>
            {record.status === 'pending' &&
              (record.quality === 'ready' || record.quality === 'basic') &&
              !record.pageExists && (
                <Popconfirm
                  title={intl.formatMessage({
                    id: 'component.proposalInbox.column.action.publishConfirmTitle',
                    defaultMessage: '发布默认页面？',
                  })}
                  description={intl.formatMessage({
                    id: 'component.proposalInbox.column.action.publishConfirmDescription',
                    defaultMessage: '会创建草稿并发布到运行控制台左侧动态菜单。',
                  })}
                  onConfirm={() => onAcceptAndPublish(record)}
                >
                  <Button type="link" size="small" icon={<RocketOutlined />}>
                    <FormattedMessage
                      id="component.proposalInbox.column.action.publish"
                      defaultMessage="发布"
                    />
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
                <FormattedMessage
                  id="component.proposalInbox.column.action.edit"
                  defaultMessage="去编辑"
                />
              </Button>
            )}
            {moreItems.length > 0 && (
              <Dropdown menu={{ items: moreItems }} trigger={['click']}>
                <Tooltip
                  title={intl.formatMessage({
                    id: 'component.proposalInbox.column.action.more',
                    defaultMessage: '更多',
                  })}
                >
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

export function buildBlockedColumns({
  intl,
}: {
  intl: IntlFormatter;
}): ColumnsType<BlockedProposalIssue> {
  return [
    {
      title: intl.formatMessage({
        id: 'component.proposalInbox.column.blocked.item',
        defaultMessage: '阻断项',
      }),
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
      title: intl.formatMessage({
        id: 'component.proposalInbox.column.blocked.repairHint',
        defaultMessage: '修复提示',
      }),
      dataIndex: 'repairHint',
      key: 'repairHint',
      render: (_, record) => localizedText(record.repairHint, 'zh-CN', '-'),
    },
    {
      title: intl.formatMessage({
        id: 'component.proposalInbox.column.blocked.diagnostics',
        defaultMessage: '诊断',
      }),
      dataIndex: 'diagnostics',
      key: 'diagnostics',
      width: 160,
      render: diagnosticSummary,
    },
    {
      title: intl.formatMessage({
        id: 'component.proposalInbox.column.blocked.updatedAt',
        defaultMessage: '更新时间',
      }),
      dataIndex: 'updatedAt',
      key: 'updatedAt',
      width: 180,
      render: formatDate,
    },
    {
      title: intl.formatMessage({
        id: 'component.proposalInbox.column.blocked.actions',
        defaultMessage: '操作',
      }),
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
            <FormattedMessage
              id="component.proposalInbox.column.blocked.action.repairSemantics"
              defaultMessage="修复语义"
            />
          </Button>
        ) : (
          <Button
            type="link"
            icon={<ExclamationCircleOutlined />}
            onClick={() => navigateTo('/functions/resource-catalog')}
          >
            <FormattedMessage
              id="component.proposalInbox.column.blocked.action.viewCatalog"
              defaultMessage="查看目录"
            />
          </Button>
        ),
    },
  ];
}
