/** ProposalInbox 详情 / 深链 / focus 定位链路：
 * focusPageKey 命中「契约变更」队列时切换 Tab（焦点链的 else-if 分支）、
 * 深链 initialProposalKey 拉取详情并打开预览弹窗（含关闭）、行内「查看 / 预览 /
 * 去编辑 / 发布」四个动作（含发布弹窗取消）。 */
import React from 'react';
import { App as AntdApp } from 'antd';
import { render, screen, fireEvent, waitFor, configure } from '@testing-library/react';
import ProposalInbox from '../index';
import { getProposal, listProposalInbox } from '@/services/dashboard';
import { listMenus } from '@/services/api/menu';
import { navigateTo, currentProposalKey, currentResourceKey } from '../urlFocus';
import type { PageProposal, ProposalInbox as ProposalInboxData } from '@/types/dashboard';

configure({ asyncUtilTimeout: 5000 });
jest.setTimeout(20000);

jest.mock('@umijs/max', () => {
  const formatMessage = jest.fn(
    ({ defaultMessage }: { defaultMessage: string }, values?: Record<string, unknown>) =>
      Object.entries(values || {}).reduce(
        (msg: string, [key, val]) => msg.split(`{${key}}`).join(String(val)),
        defaultMessage,
      ),
  );
  return {
    __esModule: true,
    useIntl: () => ({ formatMessage, locale: 'zh-CN' }),
    FormattedMessage: ({ defaultMessage }: { defaultMessage: string }) => defaultMessage,
    history: { push: jest.fn() },
  };
});

jest.mock('@/services/dashboard', () => ({
  __esModule: true,
  listProposalInbox: jest.fn(),
  acceptProposal: jest.fn(),
  acceptAndPublishProposal: jest.fn(),
  rejectProposal: jest.fn(),
  getProposal: jest.fn(),
}));

jest.mock('@/services/api/menu', () => ({ __esModule: true, listMenus: jest.fn() }));
jest.mock('@/services/api/pages', () => ({ __esModule: true, updatePageMenu: jest.fn() }));
jest.mock('@/utils/consoleMenu', () => ({
  __esModule: true,
  requestConsoleMenuRefresh: jest.fn(),
  buildConsolePagePath: jest.fn(() => '/console/page'),
}));

jest.mock('../urlFocus', () => ({
  __esModule: true,
  navigateTo: jest.fn(),
  currentResourceKey: jest.fn(() => ''),
  currentProposalKey: jest.fn(() => ''),
  clearProposalKeyParam: jest.fn(),
}));

// 详情 / 预览弹窗替身：open 才挂载并暴露 onClose 按钮，避免真实渲染依赖
jest.mock('../ProposalDetailModal', () => ({
  __esModule: true,
  default: ({ open, onClose }: { open: boolean; onClose: () => void }) =>
    open ? (
      <button type="button" data-testid="detail-close" onClick={onClose}>
        关闭详情
      </button>
    ) : null,
}));

jest.mock('../ProposalPreviewModal', () => ({
  __esModule: true,
  default: ({ open, onClose }: { open: boolean; onClose: () => void }) =>
    open ? (
      <button type="button" data-testid="preview-close" onClick={onClose}>
        关闭预览
      </button>
    ) : null,
}));

jest.mock('../ContractChangesPanel', () => ({ __esModule: true, default: () => null }));

const mockedInbox = listProposalInbox as jest.MockedFunction<typeof listProposalInbox>;
const mockedGetProposal = getProposal as jest.MockedFunction<typeof getProposal>;
const mockedListMenus = listMenus as jest.MockedFunction<typeof listMenus>;
const mockedNavigateTo = jest.mocked(navigateTo);
const mockedKey = jest.mocked(currentProposalKey);

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

const inboxOf = (data: Partial<ProposalInboxData>): ProposalInboxData => ({
  publishable: [],
  needsReview: [],
  blockedIssues: [],
  contractChanges: [],
  summary: { publishable: 0, needsReview: 0, blockedIssues: 0, contractChanges: 0 },
  ...data,
});

function renderInbox(items: PageProposal[], props: { focusPageKey?: string } = {}) {
  mockedInbox.mockResolvedValue(inboxOf({ publishable: items }));
  return render(
    <AntdApp>
      <ProposalInbox focusPageKey={props.focusPageKey} />
    </AntdApp>,
  );
}

const activeTabText = () => document.querySelector('.ant-tabs-tab-active')?.textContent ?? '';

describe('ProposalInbox 详情 / 深链 / focus 定位', () => {
  beforeEach(() => {
    jest.clearAllMocks();
    mockedListMenus.mockResolvedValue([]);
    mockedKey.mockReturnValue('');
    jest.mocked(currentResourceKey).mockReturnValue('');
    mockedGetProposal.mockResolvedValue(proposal() as never);
  });

  it('focusPageKey 命中契约变更队列：切换到「契约变更」Tab', async () => {
    mockedInbox.mockResolvedValue(
      inboxOf({
        contractChanges: [{ pageKey: 'players', pageType: 'resource', kind: 'published' }],
        summary: {
          publishable: 0,
          needsReview: 0,
          blockedIssues: 0,
          contractChanges: 1,
        },
      }),
    );

    render(
      <AntdApp>
        <ProposalInbox focusPageKey="players" />
      </AntdApp>,
    );

    await waitFor(() => expect(activeTabText()).toContain('契约变更'));
  });

  it('focusPageKey 未命中任何队列：保持默认 Tab', async () => {
    renderInbox([proposal()], { focusPageKey: 'nope' });

    await waitFor(() => expect(activeTabText()).toContain('可直接发布'));
    expect(activeTabText()).not.toContain('需要处理');
  });

  it('深链 initialProposalKey：拉详情、开预览弹窗并可关闭', async () => {
    mockedKey.mockReturnValue('resource--players');
    mockedInbox.mockResolvedValue(inboxOf({}));

    render(
      <AntdApp>
        <ProposalInbox />
      </AntdApp>,
    );

    await waitFor(() => expect(mockedGetProposal).toHaveBeenCalledWith('resource--players'));
    const closeBtn = await screen.findByTestId('preview-close');
    fireEvent.click(closeBtn);
    await waitFor(() => expect(screen.queryByTestId('preview-close')).not.toBeInTheDocument());
  });

  it('深链拉到 quality=needs_review 的详情：切到「需要处理」Tab', async () => {
    mockedKey.mockReturnValue('resource--players');
    mockedInbox.mockResolvedValue(inboxOf({}));
    mockedGetProposal.mockResolvedValue(proposal({ quality: 'needs_review' }) as never);

    render(
      <AntdApp>
        <ProposalInbox />
      </AntdApp>,
    );

    await waitFor(() => expect(mockedGetProposal).toHaveBeenCalledWith('resource--players'));
    await waitFor(() => expect(activeTabText()).toContain('需要处理'));
    expect(activeTabText()).not.toContain('可直接发布');
  });

  it('行内「查看」：打开详情弹窗并可关闭', async () => {
    renderInbox([proposal()]);

    fireEvent.click(await screen.findByRole('button', { name: '查看' }));
    const closeBtn = await screen.findByTestId('detail-close');
    fireEvent.click(closeBtn);
    await waitFor(() => expect(screen.queryByTestId('detail-close')).not.toBeInTheDocument());
  });

  it('行内「预览」：打开预览弹窗', async () => {
    renderInbox([proposal()]);

    fireEvent.click(await screen.findByRole('button', { name: '预览' }));
    await screen.findByTestId('preview-close');
  });

  it('行内「去编辑」：navigateTo focus 链接', async () => {
    renderInbox([proposal({ pageExists: true, status: 'pending' })]);

    fireEvent.click(await screen.findByRole('button', { name: /去编辑/ }));

    await waitFor(() =>
      expect(mockedNavigateTo).toHaveBeenCalledWith('/functions/pages?focus=players'),
    );
  });

  it('行内「发布」：打开确认弹窗后取消（onCancel 关闭）', async () => {
    renderInbox([proposal({ pageExists: false, quality: 'ready' })]);

    fireEvent.click(await screen.findByRole('button', { name: /发\s*布/ }));
    await screen.findByText('发布默认页面？');

    fireEvent.click(screen.getByRole('button', { name: /取\s*消|Cancel/ }));
    await waitFor(() => expect(screen.queryByText('发布默认页面？')).not.toBeInTheDocument());
  });
});
