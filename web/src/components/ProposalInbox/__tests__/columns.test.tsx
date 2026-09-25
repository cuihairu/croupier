/** ProposalColumns 行渲染变体（分支补齐）：提案列 pageKey 缺省/与
 * proposalKey 同源只显一行、未知 pageType 回退原文、quality 非
 * needs_review 不出「处理」、pending+非 ready/basic 不出「发布」、
 * pageExists 出「去编辑」、status 非 pending 无「更多」下拉；
 * + 诊断计数列、resourceKey/updatedAt 占位与坏值、quality/status 文案、
 * 发布/自定义编辑/拒绝/处理回调接线、标题本地化回退；
 * + buildBlockedColumns 阻断项行（头部回退、修复语义/查看目录跳转）。 */
import React from 'react';
import { App as AntdApp, Table } from 'antd';
import { render, screen, fireEvent } from '@testing-library/react';
import { buildBlockedColumns, buildProposalColumns } from '../ProposalColumns';
import { navigateTo } from '../urlFocus';
import type { BlockedProposalIssue, PageProposal } from '@/types/dashboard';

jest.mock('@umijs/max', () => ({
  __esModule: true,
  useIntl: () => ({
    locale: 'zh-CN',
    formatMessage: ({ defaultMessage }: { defaultMessage: string }) => defaultMessage,
  }),
  FormattedMessage: ({ defaultMessage }: { defaultMessage: string }) => defaultMessage,
}));

jest.mock('../urlFocus', () => ({
  __esModule: true,
  navigateTo: jest.fn(),
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

describe('ProposalColumns 展示列与回调接线', () => {
  it('诊断列：缺省出「无」；error/warning/info 按计数出标签', () => {
    renderTable([
      proposal(),
      proposal({
        proposalKey: 'operation--diag',
        pageKey: 'diag',
        diagnostics: [
          { code: 'e1', severity: 'error', message: 'x' },
          { code: 'e2', severity: 'error', message: 'y' },
          { code: 'w1', severity: 'warning', message: 'z' },
          { code: 'i1', severity: 'info', message: 'i' },
        ],
      }),
    ]);
    expect(screen.getByText('无')).toBeInTheDocument();
    expect(screen.getByText('2 错误')).toBeInTheDocument();
    expect(screen.getByText('1 警告')).toBeInTheDocument();
    expect(screen.getByText('1 信息')).toBeInTheDocument();
  });

  it('resourceKey 缺省与 updatedAt 空/坏值均回退占位', () => {
    renderTable([
      proposal({ resourceKey: undefined, updatedAt: undefined }),
      proposal({
        proposalKey: 'operation--bad',
        pageKey: 'bad',
        resourceKey: 'mail',
        updatedAt: 'not-a-date',
      }),
    ]);
    // 行1：resource - + updatedAt -；行2：坏时间戳原样展示
    expect(screen.getAllByText('-')).toHaveLength(2);
    expect(screen.getByText('mail')).toBeInTheDocument();
    expect(screen.getByText('not-a-date')).toBeInTheDocument();
  });

  it('quality/status 标签文案（ready/basic × pending/accepted）', () => {
    renderTable([
      proposal(),
      proposal({
        proposalKey: 'operation--b',
        pageKey: 'b',
        quality: 'basic',
        status: 'accepted',
      }),
    ]);
    expect(screen.getByText('可直接发布')).toBeInTheDocument();
    expect(screen.getByText('待处理')).toBeInTheDocument();
    expect(screen.getByText('基础可发布')).toBeInTheDocument();
    expect(screen.getByText('已接受')).toBeInTheDocument();
  });

  it('标题本地化：无 zh-CN 时回退 en-US', () => {
    renderTable([proposal({ title: { 'en-US': 'Players' } })]);
    expect(screen.getByText('Players')).toBeInTheDocument();
    expect(screen.queryByText('玩家列表')).not.toBeInTheDocument();
  });

  it('发布按钮 → onRequestPublish 携带整条提案', () => {
    const onRequestPublish = jest.fn();
    renderTable([proposal()], { onRequestPublish });
    fireEvent.click(screen.getByRole('button', { name: /发\s*布/ }));
    expect(onRequestPublish).toHaveBeenCalledWith(
      expect.objectContaining({ proposalKey: 'resource--players' }),
    );
  });

  it('更多 → 自定义编辑：确认标题与 onOk 接线到 onAccept', () => {
    const onAccept = jest.fn();
    const confirm = jest.fn();
    renderTable([proposal()], { modal: { confirm }, onAccept });
    fireEvent.click(document.querySelector('.anticon-more')?.closest('button') as HTMLElement);
    fireEvent.click(screen.getByText('自定义编辑'));

    expect(confirm).toHaveBeenCalledTimes(1);
    expect(confirm.mock.calls[0][0]).toMatchObject({
      title: '接受为草稿并自定义页面？',
    });
    confirm.mock.calls[0][0].onOk();
    expect(onAccept).toHaveBeenCalledWith(expect.objectContaining({ pageKey: 'players' }));
  });

  it('更多 → 拒绝：确认标题与 onOk 接线到 onReject（传 proposalKey）', () => {
    const onReject = jest.fn();
    const confirm = jest.fn();
    renderTable([proposal()], { modal: { confirm }, onReject });
    fireEvent.click(document.querySelector('.anticon-more')?.closest('button') as HTMLElement);
    fireEvent.click(screen.getByText('拒绝'));

    expect(confirm).toHaveBeenCalledTimes(1);
    expect(confirm.mock.calls[0][0]).toMatchObject({ title: '拒绝此提案？' });
    confirm.mock.calls[0][0].onOk();
    expect(onReject).toHaveBeenCalledWith('resource--players');
  });

  it('quality=needs_review：更多出「处理」并直接触发 onReview', () => {
    const onReview = jest.fn();
    renderTable([proposal({ quality: 'needs_review' })], { onReview });
    fireEvent.click(document.querySelector('.anticon-more')?.closest('button') as HTMLElement);
    fireEvent.click(screen.getByText('处理'));
    expect(onReview).toHaveBeenCalledWith(
      expect.objectContaining({ proposalKey: 'resource--players' }),
    );
  });
});

describe('buildBlockedColumns 阻断项行', () => {
  const blocked = (over: Partial<BlockedProposalIssue> = {}): BlockedProposalIssue => ({
    id: 7,
    resourceKey: 'player',
    functionId: 'player.kick',
    status: 'open',
    updatedAt: '2026-09-19T00:00:00Z',
    ...over,
  });

  function renderBlocked(items: BlockedProposalIssue[]) {
    const columns = buildBlockedColumns({
      intl: {
        locale: 'zh-CN',
        formatMessage: ({ defaultMessage }: { defaultMessage: string }) => defaultMessage,
      },
    });
    return render(
      <AntdApp>
        <Table columns={columns} dataSource={items} rowKey="id" pagination={false} />
      </AntdApp>,
    );
  }

  beforeEach(() => {
    jest.clearAllMocks();
  });

  it('头部展示 functionId 与 resourceKey；缺 functionId 回退 resourceKey', () => {
    renderBlocked([blocked(), blocked({ id: 8, functionId: undefined, resourceKey: 'mail' })]);
    expect(screen.getByText('player.kick')).toBeInTheDocument();
    // 缺 functionId 时头部回退 resourceKey，与副行同文出现
    expect(screen.getAllByText('mail')).toHaveLength(2);
  });

  it('两者皆缺：回退 issue-{id}；resourceKey 副行与 repairHint 缺省占位', () => {
    renderBlocked([blocked({ id: 9, functionId: undefined, resourceKey: undefined })]);
    expect(screen.getByText('issue-9')).toBeInTheDocument();
    // 副行（无 resourceKey）与 repairHint 缺省各占位一次
    expect(screen.getAllByText('-')).toHaveLength(2);
  });

  it('repairHint 本地化与诊断计数列', () => {
    renderBlocked([
      blocked({
        repairHint: { 'zh-CN': '补齐语义' },
        diagnostics: [
          { code: 'e1', severity: 'error', message: 'x' },
          { code: 'w1', severity: 'warning', message: 'w' },
        ],
      }),
    ]);
    expect(screen.getByText('补齐语义')).toBeInTheDocument();
    expect(screen.getByText('1 错误')).toBeInTheDocument();
    expect(screen.getByText('1 警告')).toBeInTheDocument();
  });

  it('有 resourceKey：「修复语义」跳资源目录并携带参数', () => {
    renderBlocked([blocked()]);
    fireEvent.click(screen.getByRole('button', { name: /修复语义/ }));
    expect(navigateTo).toHaveBeenCalledWith('/functions/resource-catalog?resourceKey=player');
  });

  it('无 resourceKey：「查看目录」跳资源目录根', () => {
    renderBlocked([blocked({ id: 11, resourceKey: undefined })]);
    fireEvent.click(screen.getByRole('button', { name: /查看目录/ }));
    expect(navigateTo).toHaveBeenCalledWith('/functions/resource-catalog');
  });
});
