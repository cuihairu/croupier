/**
 * ProposalInbox shared 纯函数与展示 helper：
 * formatDate（空/非法/合法）、diagnosticSummary（无诊断 → 「无」；
 * error/warning/info 分档计数标签；0 计数档不显示）、
 * matchesQuery（空关键词全命中；proposalKey/pageKey/resourceKey/title 命中）。
 */
import React from 'react';
import { renderToStaticMarkup } from 'react-dom/server';
import { diagnosticSummary, emptyInbox, formatDate, matchesQuery } from '../shared';
import type { IntlFormatter } from '../shared';
import type { DiagnosticInfo, PageProposal } from '@/types/dashboard';

const intl: IntlFormatter = {
  formatMessage: ({ defaultMessage }, values) =>
    Object.entries(values || {}).reduce(
      (msg: string, [key, val]) => msg.split(`{${key}}`).join(String(val)),
      defaultMessage,
    ),
};

function diag(severity: DiagnosticInfo['severity']): DiagnosticInfo {
  return { severity, code: 'x', message: 'm' } as DiagnosticInfo;
}

function proposal(overrides: Partial<PageProposal>): PageProposal {
  return {
    proposalKey: 'prop--demo',
    pageKey: 'resource--players',
    resourceKey: 'players',
    pageType: 'resource',
    status: 'pending',
    quality: 'ready',
    title: { 'zh-CN': '玩家列表', 'en-US': 'Players' },
    createdAt: '2026-09-01T00:00:00Z',
    ...overrides,
  } as PageProposal;
}

describe('emptyInbox', () => {
  it('空队列与零计数摘要', () => {
    expect(emptyInbox).toEqual({
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
    });
  });
});

describe('formatDate', () => {
  it('空值显示 -', () => {
    expect(formatDate(undefined)).toBe('-');
    expect(formatDate('')).toBe('-');
  });

  it('非法日期原样展示', () => {
    expect(formatDate('not-a-date')).toBe('not-a-date');
  });

  it('合法日期走 formatDateTime', () => {
    expect(formatDate('2026-09-01T08:00:00Z')).not.toBe('2026-09-01T08:00:00Z');
    expect(formatDate('2026-09-01T08:00:00Z')).not.toBe('-');
  });
});

describe('diagnosticSummary', () => {
  it('无诊断显示「无」标签', () => {
    const html = renderToStaticMarkup(diagnosticSummary(intl) as React.ReactElement);
    expect(html).toContain('无');
  });

  it('error/warning/info 分档计数，零计数档不出现', () => {
    const html = renderToStaticMarkup(
      diagnosticSummary(intl, [
        diag('error'),
        diag('error'),
        diag('warning'),
        diag('info'),
        diag('info'),
        diag('info'),
      ]) as React.ReactElement,
    );
    expect(html).toContain('2 错误');
    expect(html).toContain('1 警告');
    expect(html).toContain('3 信息');
  });

  it('仅 info 时不出现错误/警告档', () => {
    const html = renderToStaticMarkup(
      diagnosticSummary(intl, [diag('info')]) as React.ReactElement,
    );
    expect(html).toContain('1 信息');
    expect(html).not.toContain('错误');
    expect(html).not.toContain('警告');
  });
});

describe('matchesQuery', () => {
  const item = proposal({});

  it('空/纯空白关键词全命中', () => {
    expect(matchesQuery(item, '')).toBe(true);
    expect(matchesQuery(item, '   ')).toBe(true);
  });

  it('按 proposalKey 命中（大小写不敏感）', () => {
    expect(matchesQuery(item, 'PROP--DEMO')).toBe(true);
  });

  it('按 pageKey / resourceKey / 中文标题命中', () => {
    expect(matchesQuery(item, 'players')).toBe(true);
    expect(matchesQuery(item, 'resource--')).toBe(true);
    expect(matchesQuery(item, '玩家')).toBe(true);
  });

  it('无命中返回 false（resourceKey 缺省按空串参与拼接）', () => {
    const noResource = proposal({
      resourceKey: undefined,
      pageKey: 'resource--orders',
      title: { 'zh-CN': 'X' },
    });
    expect(matchesQuery(noResource, 'players')).toBe(false);
    expect(matchesQuery(item, 'missing-keyword')).toBe(false);
  });
});
