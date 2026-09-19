/** ProposalColumns 行渲染变体（分支补齐）：提案列 pageKey 缺省/与
 * proposalKey 同源只显一行、未知 pageType 回退原文、quality 非
 * needs_review 不出「处理」、pending+非 ready/basic 不出「发布」、
 * pageExists 出「去编辑」、status 非 pending 无「更多」下拉。 */
import React from 'react';
import { App as AntdApp, Table } from 'antd';
import { render, screen, fireEvent } from '@testing-library/react';
import { buildProposalColumns } from '../ProposalColumns';
import type { PageProposal } from '@/types/dashboard';

jest.mock('@umijs/max', () => ({
  __esModule: true,
  useIntl: () => ({
    locale: 'zh-CN',
    formatMessage: ({ defaultMessage }: { defaultMessage: string }) => defaultMessage,
  }),
  FormattedMessage: ({ defaultMessage }: { defaultMessage: string }) => defaultMessage,
}));

const intl = {
  locale: 'zh-CN',
  formatMessage: ({ defaultMessage }: { defaultMessage: string }) => defaultMessage,
};

const proposal = (overrides: Partial<PageProposal> = {}): PageProposal => ({
  id: 1,
  proposalKey: 'resource--players',
  pageKey: 'players',
  pageType: 'resource',
  quality: 'ready',
  generatorVersion: 'v1',
  title: { 'zh-CN': '玩家列表' },
  pageSpec: { type: 'resource' } as PageProposal['pageSpec'],
  status: 'pending',
  pageExists: false,
  updatedAt: '2026-09-19T00:00:00Z',
  ...overrides,
});

function renderTable(items: PageProposal[], handlers: Record<string, unknown> = {}) {
  const columns = buildProposalColumns({
    intl,
    // modal 仅用于 reject/customize 的 confirm（渲染路径不触及），替身即可
    modal: { confirm: jest.fn() } as unknown as Parameters<typeof buildProposalColumns>[0]['modal'],
    onViewDetail: jest.fn(),
    onPreview: jest.fn(),
    onAccept: jest.fn(),
    onRequestPublish: jest.fn(),
    onReview: jest.fn(),
    onReject: jest.fn(),
    ...handlers,
  } as Parameters<typeof buildProposalColumns>[0]);
  // Table 直接渲染真实 antd 表格（含展开的行 DOM）
  return render(
    <AntdApp>
      <Table columns={columns} dataSource={items} rowKey="proposalKey" pagination={false} />
    </AntdApp>,
  );
}

describe('ProposalColumns 行渲染变体', () => {
  it('pageKey 与 proposalKey 去前缀同源：只显示一行不重复', () => {
    renderTable([proposal()]);
    expect(screen.getAllByText('resource--players')).toHaveLength(1);
    // pageKey（players）不再单独显示
    expect(screen.queryByText('players')).not.toBeInTheDocument();
  });

  it('pageKey 与 proposalKey 不同源：额外显示 pageKey 行', () => {
    renderTable([proposal({ proposalKey: 'operation--mail.send', pageKey: 'mail' })]);
    expect(screen.getByText('mail')).toBeInTheDocument();
  });

  it('未知 pageType：标签回退显示原始值', () => {
    renderTable([proposal({ pageType: 'custom' as PageProposal['pageType'] })]);
    expect(screen.getByText('custom')).toBeInTheDocument();
  });

  it('quality 非 needs_review：更多菜单无「处理」项', () => {
    renderTable([proposal({ quality: 'ready', pageExists: true })]);
    fireEvent.click(document.querySelector('.anticon-more')?.closest('button') as HTMLElement);
    expect(screen.getByText('自定义编辑')).toBeInTheDocument();
    expect(screen.queryByText('处理')).not.toBeInTheDocument();
  });

  it('pending 但 quality 非 ready/basic：无发布按钮', () => {
    renderTable([proposal({ quality: 'needs_review' })]);
    expect(screen.queryByRole('button', { name: /发\s*布/ })).not.toBeInTheDocument();
  });

  it('pageExists：显示「去编辑」而非「发布」', () => {
    renderTable([proposal({ pageExists: true })]);
    expect(screen.getByRole('button', { name: /去编辑/ })).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /^发\s*布$/ })).not.toBeInTheDocument();
  });

  it('status 非 pending：无更多下拉、无发布/去编辑', () => {
    renderTable([proposal({ status: 'accepted' })]);
    expect(document.querySelector('.anticon-more')).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /去编辑/ })).not.toBeInTheDocument();
  });
});
