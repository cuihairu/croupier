/**
 * 资源目录页面定位提示体系：PageContainer 标题/副标题、SummaryOverview
 * 概览卡（统计项随数据派生）、边界 Alert 与「进入 Page Studio」跳转。
 * 提示层是纯展示，列表/搜索行为回归只做存在性断言，交互细节由组件自身
 * 测试覆盖。
 */
import React from 'react';
import { App as AntdApp } from 'antd';
import { configure, fireEvent, render, screen, waitFor } from '@testing-library/react';
import ResourceCatalogPage from '../index';

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
  const intl = { formatMessage };
  return {
    __esModule: true,
    useIntl: () => intl,
    getIntl: () => intl,
    // 工厂内联创建：jest.mock 提升早于模块体 const 声明，外部引用会 TDZ 报错
    history: { push: jest.fn() },
    FormattedMessage: ({ defaultMessage }: { defaultMessage: string }) => defaultMessage,
  };
});

const umiMock = jest.requireMock('@umijs/max') as {
  history: { push: jest.Mock };
};
const mockHistoryPush = umiMock.history.push;

// jsdom 下 PageContainer 依赖 ProLayout 的 RouteContext，mock 成透传渲染，
// 同时把 title/subTitle 落到 DOM 便于断言。
jest.mock('@ant-design/pro-components', () => ({
  __esModule: true,
  PageContainer: ({
    title,
    subTitle,
    children,
  }: {
    title: React.ReactNode;
    subTitle?: React.ReactNode;
    children: React.ReactNode;
  }) => (
    <div>
      <h1>{title}</h1>
      <p data-testid="page-subtitle">{subTitle}</p>
      {children}
    </div>
  ),
}));

const mockListResourceCatalog = jest.fn();

jest.mock('@/services/dashboard', () => ({
  __esModule: true,
  listResourceCatalog: (...args: unknown[]) => mockListResourceCatalog(...args),
  getResourceDetail: jest.fn(),
  getResourceSemanticConflicts: jest.fn(),
  getResourceSemanticVersions: jest.fn(),
  resolveResourceSemanticConflict: jest.fn(),
  updateResourceSemantics: jest.fn(),
}));

const fixtureItems = [
  {
    resourceKey: 'player',
    labels: { 'zh-CN': '玩家' },
    categoryKey: '玩家运营',
    status: 'ready',
    functions: [{ id: 'player.kick' }],
    semantics: { version: 3 },
    diagnostics: [],
  },
  {
    resourceKey: 'mail',
    labels: { 'zh-CN': '邮件' },
    categoryKey: '玩家运营',
    status: 'draft',
    functions: [{ id: 'mail.send' }],
    semantics: undefined,
    diagnostics: [{ severity: 'warning', message: 'collection missing' }],
  },
];

function renderPage() {
  return render(
    <AntdApp>
      <ResourceCatalogPage />
    </AntdApp>,
  );
}

describe('ResourceCatalogPage 定位提示体系', () => {
  beforeEach(() => {
    jest.clearAllMocks();
    mockListResourceCatalog.mockResolvedValue({ items: fixtureItems, total: 2 });
  });

  it('渲染页面标题与定位副标题（页面、菜单和分类在 Page Studio 确定）', async () => {
    renderPage();

    expect(screen.getByRole('heading', { name: '资源目录' })).toBeInTheDocument();
    expect(screen.getByTestId('page-subtitle')).toHaveTextContent(
      '资源目录只管理函数聚合后的资源语义；页面、菜单和分类在 Page Studio 中确定',
    );
    await waitFor(() => expect(mockListResourceCatalog).toHaveBeenCalled());
  });

  it('概览卡按数据派生统计项：总数/分类/已声明语义/诊断异常', async () => {
    renderPage();

    await waitFor(() => expect(screen.getByText('资源概览')).toBeInTheDocument());
    expect(screen.getByText('总数 2')).toBeInTheDocument();
    expect(screen.getByText('分类 1')).toBeInTheDocument();
    // 只有 player 声明了 semantics.version
    expect(screen.getByText('已声明语义 1')).toBeInTheDocument();
    // 只有 mail 有 warning 诊断
    expect(screen.getByText('诊断异常 1')).toBeInTheDocument();
    expect(
      screen.getByText(
        '资源层负责语义供给，Page Studio 负责页面装配，运行端菜单只来自已发布 PageSpec。',
      ),
    ).toBeInTheDocument();
  });

  it('边界 Alert 展示定位说明，按钮跳转 Page Studio', async () => {
    renderPage();

    await waitFor(() =>
      expect(screen.getByText('资源目录只展示语义与候选，不承载页面 UI')).toBeInTheDocument(),
    );
    expect(
      screen.getByText(
        '页面标题、菜单、表格列与按钮位置在 Page Studio 的 Proposal 中确定；确认资源语义后，请进入 Page Studio 生成或调整页面。',
      ),
    ).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: '进入 Page Studio' }));
    expect(mockHistoryPush).toHaveBeenCalledWith('/functions/pages');
  });

  it('列表与搜索区仍正常渲染（提示层不破坏既有结构）', async () => {
    renderPage();

    await waitFor(() => expect(screen.getByText('资源能力目录')).toBeInTheDocument());
    expect(screen.getByPlaceholderText('搜索资源')).toBeInTheDocument();
    expect(screen.getByText('player')).toBeInTheDocument();
    expect(screen.getByText('mail')).toBeInTheDocument();
  });
});
