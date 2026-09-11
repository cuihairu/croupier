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

export function diagnosticSummary(
  intl: IntlFormatter,
  diagnostics?: DiagnosticInfo[],
): React.ReactNode {
  if (!diagnostics || diagnostics.length === 0) {
    return (
      <Tag color="success">
        <FormattedMessage id="component.proposalInbox.diagnostics.none" defaultMessage="无" />
      </Tag>
    );
  }
  const errors = diagnostics.filter((item) => item.severity === 'error').length;
  const warnings = diagnostics.filter((item) => item.severity === 'warning').length;
  const infos = diagnostics.filter((item) => item.severity === 'info').length;
  return (
    <Space>
      {errors > 0 && (
        <Tag color="error">
          {intl.formatMessage(
            {
              id: 'component.proposalInbox.diagnostics.errorCount',
              defaultMessage: `${errors} 错误`,
            },
            { count: errors },
          )}
        </Tag>
      )}
      {warnings > 0 && (
        <Tag color="warning">
          {intl.formatMessage(
            {
              id: 'component.proposalInbox.diagnostics.warningCount',
              defaultMessage: `${warnings} 警告`,
            },
            { count: warnings },
          )}
        </Tag>
      )}
      {infos > 0 && (
        <Tag color="blue">
          {intl.formatMessage(
            {
              id: 'component.proposalInbox.diagnostics.infoCount',
              defaultMessage: `${infos} 信息`,
            },
            { count: infos },
          )}
        </Tag>
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
