import React from 'react';
import { Space, Tag } from 'antd';
import { FormattedMessage } from '@umijs/max';
import type {
  DiagnosticInfo,
  PageProposal,
  PageType,
  ProposalInbox as ProposalInboxData,
  ProposalQuality,
  ProposalStatus,
} from '@/types/dashboard';
import { localizedText } from '@/utils/localizedText';
import { formatDateTime } from '@/utils/format';

/** ProposalInbox 共享常量与纯函数。 */

/** 标签 Map 的展示文案以 id + defaultMessage 双字段承载，渲染处经 intl 解析 */
export type IntlMessage = { id: string; defaultMessage: string };

/** 纯函数接收 intl 的最小结构（@umijs/max 未导出 IntlShape 类型） */
export type IntlFormatter = {
  formatMessage: (
    descriptor: { id: string; defaultMessage: string },
    values?: Record<string, string | number>,
  ) => string;
};

export const emptyInbox: ProposalInboxData = {
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

export const statusColors: Record<ProposalStatus, string> = {
  pending: 'processing',
  accepted: 'success',
  rejected: 'error',
  expired: 'default',
};

export const statusLabels: Record<ProposalStatus, IntlMessage> = {
  pending: { id: 'component.proposalInbox.status.pending', defaultMessage: '待处理' },
  accepted: { id: 'component.proposalInbox.status.accepted', defaultMessage: '已接受' },
  rejected: { id: 'component.proposalInbox.status.rejected', defaultMessage: '已拒绝' },
  expired: { id: 'component.proposalInbox.status.expired', defaultMessage: '已过期' },
};

export const qualityColors: Record<ProposalQuality, string> = {
  ready: 'success',
  basic: 'processing',
  needs_review: 'warning',
};

export const qualityLabels: Record<ProposalQuality, IntlMessage> = {
  ready: { id: 'component.proposalInbox.quality.ready', defaultMessage: '可直接发布' },
  basic: { id: 'component.proposalInbox.quality.basic', defaultMessage: '基础可发布' },
  needs_review: { id: 'component.proposalInbox.quality.needsReview', defaultMessage: '需要处理' },
};

export const pageTypeLabels: Record<PageType, IntlMessage> = {
  resource: { id: 'component.proposalInbox.pageType.resource', defaultMessage: '资源' },
  operation: { id: 'component.proposalInbox.pageType.operation', defaultMessage: '操作' },
  task: { id: 'component.proposalInbox.pageType.task', defaultMessage: '任务' },
  report: { id: 'component.proposalInbox.pageType.report', defaultMessage: '报表' },
  composite: { id: 'component.proposalInbox.pageType.composite', defaultMessage: '组合' },
};

export const pageTypeColors: Record<PageType, string> = {
  resource: 'blue',
  operation: 'green',
  task: 'orange',
  report: 'purple',
  composite: 'cyan',
};

export function formatDate(value?: string): string {
  if (!value) return '-';
  return Number.isNaN(new Date(value).getTime()) ? value : formatDateTime(value);
}

/**
 * 诊断计数标签（OPEN-ISSUES #29）：传入 onJump 时错误/警告/信息标签可点击，
 * 直达对应处理位置（提案详情/编辑器/资源目录/同步报告，由调用方决定落点）；
 * 未传时保持纯展示计数。
 */
export function diagnosticSummary(
  intl: IntlFormatter,
  diagnostics?: DiagnosticInfo[],
  onJump?: (severity: DiagnosticInfo['severity']) => void,
): React.ReactNode {
  if (!diagnostics || diagnostics.length === 0) {
    return (
      <Tag color="success">
        <FormattedMessage id="component.proposalInbox.diagnostics.none" defaultMessage="无" />
      </Tag>
    );
  }
  const countOf = (severity: DiagnosticInfo['severity']) =>
    diagnostics.filter((item) => item.severity === severity).length;
  const entries: {
    severity: DiagnosticInfo['severity'];
    color: string;
    count: number;
    message: { id: string; defaultMessage: string };
  }[] = [
    {
      severity: 'error',
      color: 'error',
      count: countOf('error'),
      message: {
        id: 'component.proposalInbox.diagnostics.errorCount',
        defaultMessage: `${countOf('error')} 错误`,
      },
    },
    {
      severity: 'warning',
      color: 'warning',
      count: countOf('warning'),
      message: {
        id: 'component.proposalInbox.diagnostics.warningCount',
        defaultMessage: `${countOf('warning')} 警告`,
      },
    },
    {
      severity: 'info',
      color: 'blue',
      count: countOf('info'),
      message: {
        id: 'component.proposalInbox.diagnostics.infoCount',
        defaultMessage: `${countOf('info')} 信息`,
      },
    },
  ];
  const jumpTip = intl.formatMessage({
    id: 'component.proposalInbox.diagnostics.jumpTip',
    defaultMessage: '点击直达处理位置',
  });
  return (
    <Space>
      {entries
        .filter((entry) => entry.count > 0)
        .map((entry) =>
          onJump ? (
            <Tag
              key={entry.severity}
              color={entry.color}
              role="button"
              tabIndex={0}
              title={jumpTip}
              style={{ cursor: 'pointer' }}
              onClick={() => onJump(entry.severity)}
              onKeyDown={(event) => {
                if (event.key === 'Enter' || event.key === ' ') {
                  event.preventDefault();
                  onJump(entry.severity);
                }
              }}
            >
              {intl.formatMessage(entry.message, { count: entry.count })}
            </Tag>
          ) : (
            <Tag key={entry.severity} color={entry.color}>
              {intl.formatMessage(entry.message, { count: entry.count })}
            </Tag>
          ),
        )}
    </Space>
  );
}

export function matchesQuery(proposal: PageProposal, query: string): boolean {
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
