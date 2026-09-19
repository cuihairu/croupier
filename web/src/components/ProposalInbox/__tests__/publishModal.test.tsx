/** ProposalInbox 发布确认弹窗（含挂载菜单选择）：
 * 发布按钮直开弹窗并拉菜单树；选菜单发布 → acceptAndPublish +
 * updatePageMenu + 成功提示含挂载说明；不选仅发布（不调 updatePageMenu）；
 * 无菜单显示引导 Alert；发布失败提示且弹窗保留；挂载失败成功 modal 提示
 * 可稍后重挂；pageExists 行「更多」含「挂载菜单」直达入口（focus+mount=1）。 */
import React from 'react';
import { App as AntdApp } from 'antd';
import { render, screen, fireEvent, waitFor, configure } from '@testing-library/react';
import ProposalInbox from '../index';
import { acceptAndPublishProposal, listProposalInbox } from '@/services/dashboard';
import { listMenus } from '@/services/api/menu';
import { updatePageMenu } from '@/services/api/pages';
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

jest.mock('@/services/api/menu', () => ({
  __esModule: true,
  listMenus: jest.fn(),
}));

jest.mock('@/services/api/pages', () => ({
  __esModule: true,
  updatePageMenu: jest.fn(),
}));

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

// 重弹窗替身：避免真实渲染依赖
jest.mock('../ProposalDetailModal', () => ({ __esModule: true, default: () => null }));
jest.mock('../ProposalPreviewModal', () => ({ __esModule: true, default: () => null }));
jest.mock('../ContractChangesPanel', () => ({ __esModule: true, default: () => null }));

const mockedInbox = listProposalInbox as jest.MockedFunction<typeof listProposalInbox>;
const mockedPublish = acceptAndPublishProposal as jest.MockedFunction<
  typeof acceptAndPublishProposal
>;
const mockedListMenus = listMenus as jest.MockedFunction<typeof listMenus>;
const mockedUpdateMenu = updatePageMenu as jest.MockedFunction<typeof updatePageMenu>;
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

const inboxWith = (items: PageProposal[]): ProposalInboxData => ({
  publishable: items,
  needsReview: [],
  blockedIssues: [],
  contractChanges: [],
  summary: { publishable: items.length, needsReview: 0, blockedIssues: 0, contractChanges: 0 },
});

const MENUS = [
  {
    id: 1,
    parentId: null,
    menuKey: 'player',
    labels: { 'zh-CN': '玩家' },
    sortOrder: 0,
    isVisible: true,
    children: [],
  },
];

function renderInbox(items: PageProposal[], props: { focusPageKey?: string } = {}) {
  mockedInbox.mockResolvedValue(inboxWith(items));
  return render(
    <AntdApp>
      <ProposalInbox focusPageKey={props.focusPageKey} />
    </AntdApp>,
  );
}

async function openPublishModal(items: PageProposal[]) {
  const utils = renderInbox(items);
  const publishBtn = await screen.findAllByRole('button', { name: /发\s*布/ });
  fireEvent.click(publishBtn[0]);
  await screen.findByText('发布默认页面？');
  return utils;
}

describe('ProposalInbox 发布确认弹窗（含挂载菜单）', () => {
  beforeEach(() => {
    jest.clearAllMocks();
    mockedListMenus.mockResolvedValue(MENUS as never);
  });

  it('发布按钮打开弹窗并拉取菜单树', async () => {
    await openPublishModal([proposal()]);
    expect(mockedListMenus).toHaveBeenCalledTimes(1);
    expect(screen.getByText('挂载到菜单（可选）')).toBeInTheDocument();
    expect(screen.getByText('会创建草稿并发布。')).toBeInTheDocument();
  });

  it('选择菜单发布：acceptAndPublish + updatePageMenu + 成功提示含挂载说明', async () => {
    mockedPublish.mockResolvedValue({ pageKey: 'players', publishedVersion: 1 } as never);
    await openPublishModal([proposal()]);

    // 展开 TreeSelect 选择「玩家」
    fireEvent.mouseDown(document.querySelector('.ant-select') as HTMLElement);
    const option = await screen.findByText('玩家');
    fireEvent.click(option);

    fireEvent.click(screen.getByRole('button', { name: /确\s*定|OK/ }));
    await waitFor(() => expect(mockedPublish).toHaveBeenCalledWith('resource--players'));
    await waitFor(() => expect(mockedUpdateMenu).toHaveBeenCalledWith('players', 1));
    await screen.findByText(/已挂载到所选菜单，运行控制台导航即时可见/);
  });

  it('不选菜单发布：仅发布不挂载，成功提示说明未挂载后果', async () => {
    mockedPublish.mockResolvedValue({ pageKey: 'players', publishedVersion: 1 } as never);
    await openPublishModal([proposal()]);

    fireEvent.click(screen.getByRole('button', { name: /确\s*定|OK/ }));
    await waitFor(() => expect(mockedPublish).toHaveBeenCalled());
    expect(mockedUpdateMenu).not.toHaveBeenCalled();
    await screen.findByText(/未挂载菜单：页面不会出现在运行控制台导航/);
  });

  it('当前环境无菜单：显示创建菜单引导 Alert', async () => {
    mockedListMenus.mockResolvedValue([]);
    await openPublishModal([proposal()]);
    expect(
      screen.getByText(/当前环境暂无菜单：页面发布后不会出现在运行控制台导航/),
    ).toBeInTheDocument();
  });

  it('发布失败：提示错误且弹窗保留可重试', async () => {
    mockedPublish.mockRejectedValue(new Error('boom'));
    await openPublishModal([proposal()]);

    fireEvent.click(screen.getByRole('button', { name: /确\s*定|OK/ }));
    await waitFor(() => expect(mockedPublish).toHaveBeenCalledTimes(1));
    await waitFor(() => expect(screen.getByText('发布失败，请重试')).toBeInTheDocument());
    // 弹窗未关闭：标题仍在文档中
    expect(screen.getByText('发布默认页面？')).toBeInTheDocument();
  });

  it('挂载失败：发布成功但成功弹窗提示可稍后重挂', async () => {
    mockedPublish.mockResolvedValue({ pageKey: 'players', publishedVersion: 2 } as never);
    mockedUpdateMenu.mockRejectedValue(new Error('mount fail'));
    await openPublishModal([proposal()]);

    fireEvent.mouseDown(document.querySelector('.ant-select') as HTMLElement);
    fireEvent.click(await screen.findByText('玩家'));
    fireEvent.click(screen.getByRole('button', { name: /确\s*定|OK/ }));

    await screen.findByText(/但挂载菜单失败，可稍后在页面工作台重新挂载/);
    expect(mockedUpdateMenu).toHaveBeenCalledWith('players', 1);
  });

  it('pageExists 行「更多」含挂载菜单直达入口（focus + mount=1）', async () => {
    renderInbox([proposal({ pageExists: true, status: 'pending' })]);

    // 「更多」下拉（⋯ 按钮）：等行渲染出 ⋯ 图标后点其宿主按钮
    const moreBtn = await waitFor(() => {
      const icon = document.querySelector('.anticon-more');
      const btn = icon?.closest('button');
      if (!btn) throw new Error('more button not rendered yet');
      return btn;
    });
    fireEvent.click(moreBtn);
    const mountItem = await screen.findByText('挂载菜单');
    fireEvent.click(mountItem);

    await waitFor(() =>
      expect(mockedNavigateTo).toHaveBeenCalledWith('/functions/pages?focus=players&mount=1'),
    );
  });

  it('发布成功且提案无分类：成功弹窗按钮为「打开运行控制台」并跳 /console', async () => {
    // pageSpec 无 category → categoryKey 空串 → 打开运行控制台分支
    mockedPublish.mockResolvedValue({ pageKey: 'players', publishedVersion: 1 } as never);
    await openPublishModal([
      proposal({ pageSpec: { type: 'resource' } as PageProposal['pageSpec'] }),
    ]);

    fireEvent.click(screen.getByRole('button', { name: /确\s*定|OK/ }));
    const consoleBtn = await screen.findByRole('button', { name: /打开运行控制台/ });
    fireEvent.click(consoleBtn);
    await waitFor(() => expect(mockedNavigateTo).toHaveBeenCalledWith('/console'));
  });

  it('listMenus 失败：按无菜单处理并显示创建菜单引导', async () => {
    mockedListMenus.mockRejectedValue(new Error('net'));
    await openPublishModal([proposal()]);
    expect(
      screen.getByText(/当前环境暂无菜单：页面发布后不会出现在运行控制台导航/),
    ).toBeInTheDocument();
  });
});

describe('ProposalInbox 定位与初始化分支', () => {
  beforeEach(() => {
    jest.clearAllMocks();
    mockedListMenus.mockResolvedValue(MENUS as never);
  });

  it('focusPageKey 命中需要处理队列：自动切到该 Tab 并高亮行', async () => {
    renderInbox([proposal()], { focusPageKey: 'players' });
    // 默认 Tab 即可直接发布；focus 命中 publishable 时行高亮类生效
    const row = await screen.findByText('resource--players');
    await waitFor(() => expect(row.closest('tr')?.className).toContain('proposal-inbox-focus-row'));
  });

  it('URL proposalKey 参数：拉取详情后清理参数（失败分支吞掉 warning）', async () => {
    const { getProposal } = jest.requireMock('@/services/dashboard') as {
      getProposal: jest.Mock;
    };
    const { currentProposalKey, clearProposalKeyParam } = jest.requireMock('../urlFocus') as {
      currentProposalKey: jest.Mock;
      clearProposalKeyParam: jest.Mock;
    };
    currentProposalKey.mockReturnValue('p-1');
    // 持久 rejection：useIntl mock 每渲染新对象会让 effect 重跑，Once 队列会空
    getProposal.mockRejectedValue(new Error('gone'));
    renderInbox([proposal()]);
    await waitFor(() => expect(getProposal).toHaveBeenCalledWith('p-1'));
    await waitFor(() => expect(clearProposalKeyParam).toHaveBeenCalled());
    currentProposalKey.mockReturnValue('');
  });
});

describe('ProposalInbox 发布跳转与资源定位分支', () => {
  const menusWithChild = [
    {
      id: 1,
      parentId: null,
      menuKey: 'player',
      labels: { 'zh-CN': '玩家' },
      sortOrder: 0,
      isVisible: true,
      children: [
        {
          id: 2,
          parentId: 1,
          menuKey: 'profile',
          labels: { 'zh-CN': '资料' },
          sortOrder: 0,
          isVisible: true,
          children: [],
        },
      ],
    },
  ];

  beforeEach(() => {
    jest.clearAllMocks();
    mockedListMenus.mockResolvedValue(menusWithChild as never);
  });

  it('提案带分类 key：成功弹窗按钮为「打开运行页」并跳控制台页面路径', async () => {
    mockedPublish.mockResolvedValue({ pageKey: 'players', publishedVersion: 3 } as never);
    await openPublishModal([
      proposal({
        pageSpec: { type: 'resource', category: { key: 'player' } } as PageProposal['pageSpec'],
      }),
    ]);
    fireEvent.click(screen.getByRole('button', { name: /确\s*定|OK/ }));
    const pageBtn = await screen.findByRole('button', { name: /打开运行页/ });
    fireEvent.click(pageBtn);
    // buildConsolePagePath 被 mock 为 '/console/page'
    await waitFor(() => expect(mockedNavigateTo).toHaveBeenCalledWith('/console/page'));
  });

  it('提案 pageSpec 缺失：按无分类处理并跳运行控制台', async () => {
    mockedPublish.mockResolvedValue({ pageKey: 'players', publishedVersion: 1 } as never);
    await openPublishModal([proposal({ pageSpec: undefined })]);
    fireEvent.click(screen.getByRole('button', { name: /确\s*定|OK/ }));
    fireEvent.click(await screen.findByRole('button', { name: /打开运行控制台/ }));
    await waitFor(() => expect(mockedNavigateTo).toHaveBeenCalledWith('/console'));
  });

  it('菜单树含子菜单：TreeSelect 渲染嵌套节点', async () => {
    await openPublishModal([proposal()]);
    fireEvent.mouseDown(document.querySelector('.ant-select') as HTMLElement);
    expect(await screen.findByText('资料')).toBeInTheDocument();
  });

  it('选中菜单后清除：按未挂载发布', async () => {
    mockedPublish.mockResolvedValue({ pageKey: 'players', publishedVersion: 1 } as never);
    await openPublishModal([proposal()]);

    fireEvent.mouseDown(document.querySelector('.ant-select') as HTMLElement);
    fireEvent.click(await screen.findByText('玩家'));
    // allowClear 清除 → onChange(undefined) → publishMenuId 回 null（仅发布不挂载）
    const clearBtn = await waitFor(() => {
      const el = document.querySelector('.ant-select-clear');
      if (!el) throw new Error('clear icon not rendered');
      return el as HTMLElement;
    });
    fireEvent.click(clearBtn);

    fireEvent.click(screen.getByRole('button', { name: /确\s*定|OK/ }));
    await waitFor(() => expect(mockedPublish).toHaveBeenCalled());
    expect(mockedUpdateMenu).not.toHaveBeenCalled();
    await screen.findByText(/未挂载菜单/);
  });

  it('URL resourceKey 参数：标题区显示当前资源 Tag', async () => {
    const { currentResourceKey } = jest.requireMock('../urlFocus') as {
      currentResourceKey: jest.Mock;
    };
    currentResourceKey.mockReturnValue('player');
    renderInbox([proposal()]);
    expect(await screen.findByText('当前资源：player')).toBeInTheDocument();
    currentResourceKey.mockReturnValue('');
  });
});
