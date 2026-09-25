/** ProposalInbox 行操作与工具栏动作（非发布链）：初始拉取参数、空态、
 * 搜索过滤、刷新重拉、创建组合页跳转、accept 三分支（auto 发布成功 /
 * 发布失败 / 静默跳编辑器）、拒绝二次确认、处理三分支（资源跳目录 /
 * 阻断诊断先预览 / 常规走 accept）、契约变更 Tab 渲染、
 * focusPageKey 命中需要处理队列自动切 Tab。 */
import React from 'react';
import { App as AntdApp } from 'antd';
import { configure, fireEvent, render, screen, waitFor } from '@testing-library/react';
import ProposalInbox from '../index';
import {
  acceptProposal,
  getProposal,
  listProposalInbox,
  rejectProposal,
} from '@/services/dashboard';
import { navigateTo } from '../urlFocus';
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

jest.mock('../ProposalDetailModal', () => ({ __esModule: true, default: () => null }));
jest.mock('../ProposalPreviewModal', () => ({ __esModule: true, default: () => null }));
// 契约变更面板替身：以记录数标记渲染，验证 Tab 激活时才挂载
jest.mock('../ContractChangesPanel', () => ({
  __esModule: true,
  default: ({ records }: { records: unknown[] }) => (
    <div data-testid="cc-panel">{`contract-changes:${records.length}`}</div>
  ),
}));

const umiMock = jest.requireMock('@umijs/max') as { history: { push: jest.Mock } };
const { requestConsoleMenuRefresh } = jest.requireMock('@/utils/consoleMenu') as {
  requestConsoleMenuRefresh: jest.Mock;
};

const mockedList = listProposalInbox as jest.MockedFunction<typeof listProposalInbox>;
const mockedAccept = acceptProposal as jest.MockedFunction<typeof acceptProposal>;
const mockedReject = rejectProposal as jest.MockedFunction<typeof rejectProposal>;
const mockedGetProposal = getProposal as jest.MockedFunction<typeof getProposal>;
const mockedNavigateTo = jest.mocked(navigateTo);

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

function inboxWith(data: Partial<ProposalInboxData>): ProposalInboxData {
  const publishable = data.publishable ?? [];
  const needsReview = data.needsReview ?? [];
  const blockedIssues = data.blockedIssues ?? [];
  const contractChanges = data.contractChanges ?? [];
  return {
    publishable,
    needsReview,
    blockedIssues,
    contractChanges,
    summary: {
      publishable: publishable.length,
      needsReview: needsReview.length,
      blockedIssues: blockedIssues.length,
      contractChanges: contractChanges.length,
    },
  };
}

function renderInbox(data: Partial<ProposalInboxData>, focusPageKey = '') {
  mockedList.mockResolvedValue(inboxWith(data));
  return render(
    <AntdApp>
      <ProposalInbox focusPageKey={focusPageKey} />
    </AntdApp>,
  );
}

/** 打开当前表格行的「更多」下拉（页面仅渲染单行时可用） */
async function openMoreMenu(): Promise<void> {
  const moreBtn = await waitFor(() => {
    const icon = document.querySelector('.anticon-more');
    const btn = icon?.closest('button');
    if (!btn) throw new Error('more button not rendered yet');
    return btn as HTMLElement;
  });
  fireEvent.click(moreBtn);
  await screen.findByText('自定义编辑');
}

function confirmOk(): void {
  fireEvent.click(screen.getByRole('button', { name: /OK|确\s*定/ }));
}

describe('ProposalInbox 工具栏与空态', () => {
  beforeEach(() => {
    jest.clearAllMocks();
  });

  it('初始拉取不带 resourceKey（无 URL 参数）', async () => {
    renderInbox({ publishable: [proposal()] });
    await waitFor(() => expect(mockedList).toHaveBeenCalledWith({ resourceKey: undefined }));
  });

  it('三队列全空：各 Tab 展示空态文案', async () => {
    renderInbox({});
    expect(await screen.findByText('暂无可直接发布的默认页面')).toBeInTheDocument();
  });

  it('搜索过滤：不匹配出空态，恢复关键词行回来', async () => {
    renderInbox({ publishable: [proposal()] });
    await screen.findByText('resource--players');

    const input = screen.getByPlaceholderText('搜索提案、页面或资源');
    fireEvent.change(input, { target: { value: 'zzz' } });
    expect(await screen.findByText('暂无可直接发布的默认页面')).toBeInTheDocument();

    fireEvent.change(input, { target: { value: 'players' } });
    expect(await screen.findByText('resource--players')).toBeInTheDocument();
    // 过滤只影响前端展示，不触发重新拉取
    expect(mockedList).toHaveBeenCalledTimes(1);
  });

  it('刷新按钮重拉队列', async () => {
    renderInbox({ publishable: [proposal()] });
    await waitFor(() => expect(mockedList).toHaveBeenCalledTimes(1));

    fireEvent.click(screen.getByRole('button', { name: /刷新/ }));
    await waitFor(() => expect(mockedList).toHaveBeenCalledTimes(2));
  });

  it('创建组合页 → history.push 组合编辑器', async () => {
    renderInbox({});
    await screen.findByText('暂无可直接发布的默认页面');

    fireEvent.click(screen.getByRole('button', { name: /创建组合页/ }));
    expect(umiMock.history.push).toHaveBeenCalledWith('/functions/pages/composite-editor');
  });

  it('契约变更 Tab 激活时渲染 ContractChangesPanel（含记录数）', async () => {
    renderInbox({ contractChanges: [{ id: 1 } as never, { id: 2 } as never] });
    await screen.findByText('暂无可直接发布的默认页面');

    fireEvent.click(screen.getByRole('tab', { name: /契约变更/ }));
    expect(await screen.findByTestId('cc-panel')).toHaveTextContent('contract-changes:2');
  });

  it('focusPageKey 命中需要处理队列：自动切到该 Tab', async () => {
    renderInbox({ needsReview: [proposal({ quality: 'needs_review' })] }, 'players');
    await waitFor(() =>
      expect(screen.getByRole('tab', { name: /需要处理/ })).toHaveAttribute(
        'aria-selected',
        'true',
      ),
    );
  });
});

describe('ProposalInbox accept 三分支（自定义编辑 → 确认）', () => {
  beforeEach(() => {
    jest.clearAllMocks();
  });

  async function confirmAccept(): Promise<void> {
    await openMoreMenu();
    fireEvent.click(screen.getByText('自定义编辑'));
    // confirm 弹窗同时渲染 ant-modal-title 与 ant-modal-confirm-title 两份标题
    await screen.findAllByText('接受为草稿并自定义页面？');
    confirmOk();
  }

  it('auto env 发布成功：成功提示 + 刷新控制台导航 + 跳编辑器 focus', async () => {
    mockedAccept.mockResolvedValue({ published: true } as never);
    renderInbox({ publishable: [proposal()] });

    await confirmAccept();

    await waitFor(() => expect(mockedAccept).toHaveBeenCalledWith('resource--players'));
    await screen.findByText('已接受并自动发布（当前环境为免审核策略）');
    expect(requestConsoleMenuRefresh).toHaveBeenCalled();
    await waitFor(() =>
      expect(mockedNavigateTo).toHaveBeenCalledWith('/functions/pages?focus=players'),
    );
    // accept 后重拉队列
    await waitFor(() => expect(mockedList).toHaveBeenCalledTimes(2));
  });

  it('auto env 发布失败：警告含原因，草稿已保存不回滚', async () => {
    mockedAccept.mockResolvedValue({ publishError: 'env required' } as never);
    renderInbox({ publishable: [proposal()] });

    await confirmAccept();

    await screen.findByText(
      '已接受，但自动发布失败：env required。草稿已保存，可在页面编辑器手动发布',
    );
    expect(requestConsoleMenuRefresh).not.toHaveBeenCalled();
    await waitFor(() =>
      expect(mockedNavigateTo).toHaveBeenCalledWith('/functions/pages?focus=players'),
    );
  });

  it('required env（无发布结果）：静默跳编辑器，无成功/失败提示', async () => {
    mockedAccept.mockResolvedValue({} as never);
    renderInbox({ publishable: [proposal()] });

    await confirmAccept();

    await waitFor(() =>
      expect(mockedNavigateTo).toHaveBeenCalledWith('/functions/pages?focus=players'),
    );
    expect(screen.queryByText(/已接受并自动发布/)).not.toBeInTheDocument();
    expect(screen.queryByText(/已接受，但自动发布失败/)).not.toBeInTheDocument();
    expect(requestConsoleMenuRefresh).not.toHaveBeenCalled();
  });
});

describe('ProposalInbox 拒绝与处理分支', () => {
  beforeEach(() => {
    jest.clearAllMocks();
  });

  it('拒绝：二次确认后调用 rejectProposal 并重拉队列', async () => {
    mockedReject.mockResolvedValue(undefined as never);
    renderInbox({ publishable: [proposal()] });

    await openMoreMenu();
    fireEvent.click(screen.getByText('拒绝'));
    await screen.findAllByText('拒绝此提案？');
    confirmOk();

    await waitFor(() => expect(mockedReject).toHaveBeenCalledWith('resource--players'));
    await waitFor(() => expect(mockedList).toHaveBeenCalledTimes(2));
  });

  async function openNeedsReview(): Promise<void> {
    // 先切 Tab（默认可直接发布 Tab 为空时无「更多」按钮），再开行菜单
    fireEvent.click(screen.getByRole('tab', { name: /需要处理/ }));
    await openMoreMenu();
    await screen.findByText('处理');
  }

  it('处理 resource 提案：直接跳资源目录（不 accept）', async () => {
    renderInbox({
      needsReview: [proposal({ quality: 'needs_review', resourceKey: 'player' })],
    });

    await openNeedsReview();
    fireEvent.click(screen.getByText('处理'));

    await waitFor(() =>
      expect(mockedNavigateTo).toHaveBeenCalledWith(
        '/functions/resource-catalog?resourceKey=player',
      ),
    );
    expect(mockedAccept).not.toHaveBeenCalled();
  });

  it('处理含阻断诊断：先开预览并提示，不走 accept', async () => {
    mockedGetProposal.mockResolvedValue(proposal() as never);
    renderInbox({
      needsReview: [
        proposal({
          quality: 'needs_review',
          pageType: 'operation',
          resourceKey: undefined,
          diagnostics: [{ code: 'e1', severity: 'error', message: 'broken' }],
        }),
      ],
    });

    await openNeedsReview();
    fireEvent.click(screen.getByText('处理'));

    await screen.findByText('该提案包含阻断诊断，请先查看诊断后再处理。');
    expect(mockedGetProposal).toHaveBeenCalledWith('resource--players');
    expect(mockedAccept).not.toHaveBeenCalled();
  });

  it('处理无阻断诊断的非资源提案：走 accept 链', async () => {
    mockedAccept.mockResolvedValue({} as never);
    renderInbox({
      needsReview: [
        proposal({
          quality: 'needs_review',
          pageType: 'operation',
          resourceKey: undefined,
          diagnostics: [{ code: 'w1', severity: 'warning', message: 'soft' }],
        }),
      ],
    });

    await openNeedsReview();
    fireEvent.click(screen.getByText('处理'));

    await waitFor(() => expect(mockedAccept).toHaveBeenCalledWith('resource--players'));
    expect(mockedGetProposal).not.toHaveBeenCalled();
  });
});
