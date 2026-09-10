import React from 'react';
import { Space, Tag } from 'antd';
import type {
  DiagnosticInfo,
  PageProposal,
  PageType,
  ProposalInbox as ProposalInboxData,
  ProposalQuality,
  ProposalStatus,
} from '@/types/dashboard';
import { localizedText } from '@/utils/localizedText';

/** ProposalInbox 共享常量与纯函数。 */

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

export const statusLabels: Record<ProposalStatus, string> = {
  pending: '待处理',
  accepted: '已接受',
  rejected: '已拒绝',
  expired: '已过期',
};

export const qualityColors: Record<ProposalQuality, string> = {
  ready: 'success',
  basic: 'processing',
  needs_review: 'warning',
};

export const qualityLabels: Record<ProposalQuality, string> = {
  ready: '可直接发布',
  basic: '基础可发布',
  needs_review: '需要处理',
};

export const pageTypeLabels: Record<PageType, string> = {
  resource: '资源',
  operation: '操作',
  task: '任务',
  report: '报表',
  composite: '组合',
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
  const time = new Date(value);
  return Number.isNaN(time.getTime()) ? value : time.toLocaleString();
}

export function diagnosticSummary(diagnostics?: DiagnosticInfo[]): React.ReactNode {
  if (!diagnostics || diagnostics.length === 0) {
    return <Tag color="success">无</Tag>;
  }
  const errors = diagnostics.filter((item) => item.severity === 'error').length;
  const warnings = diagnostics.filter((item) => item.severity === 'warning').length;
  const infos = diagnostics.filter((item) => item.severity === 'info').length;
  return (
    <Space>
      {errors > 0 && <Tag color="error">{errors} 错误</Tag>}
      {warnings > 0 && <Tag color="warning">{warnings} 警告</Tag>}
      {infos > 0 && <Tag color="blue">{infos} 信息</Tag>}
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
