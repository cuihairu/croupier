/**
 * PageStudio #30 搜索/状态下推 + 服务端分页：
 * 关键词回车触发带 keyword 的请求（并回到第 1 页）、状态下拉触发带 status
 * 的请求、翻页带 page 参数——列表查询完全由服务端执行，前端不拉全量自算；
 * 「涉及资源」列展示服务端计算的页面→资源关联（多资源页展开）。
 */
import React from 'react';
import { App as AntdApp } from 'antd';
import { configure, fireEvent, render, screen, waitFor } from '@testing-library/react';
import PageStudio from '../index';
import { listPageDrafts, listPageResources } from '@/services/api/pages';
import type { PageDraftListParams, PageDraftListResult } from '@/services/api/pages';
import type { PageSpecDraftSummary } from '@/types/dashboard';

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
    getIntl: () => ({ formatMessage }),
    FormattedMessage: ({ defaultMessage }: { defaultMessage: string }) => defaultMessage,
    history: { push: jest.fn() },
  };
});

jest.mock('@/services/api/pages', () => ({
  __esModule: true,
  listPageDrafts: jest.fn(),
  listPageResources: jest.fn(),
  getPageDraft: jest.fn(),
  listPageVersions: jest.fn(),
  publishPageDraft: jest.fn(),
  regeneratePageDraft: jest.fn(),
  savePageDraft: jest.fn(),
  unpublishPage: jest.fn(),
  updatePageMenu: jest.fn(),
  bulkPublishPages: jest.fn(),
  bulkUnpublishPages: jest.fn(),
}));
jest.mock('@/services/api/menu', () => ({ __esModule: true, listMenus: jest.fn() }));
jest.mock('@/services/api/versioning', () => ({
  __esModule: true,
  getChangeChain: jest.fn(),
  getDiff: jest.fn(),
  mergeChanges: jest.fn(),
  rollbackDraft: jest.fn(),
  rollbackPublish: jest.fn(),
}));

jest.mock('@/components/ProposalInbox', () => ({ __esModule: true, default: () => null }));
jest.mock('@/components/PageWorkflowGuide', () => ({ __esModule: true, default: () => null }));
jest.mock('@/components/MergeConflictModal', () => ({ __esModule: true, default: () => null }));
jest.mock('@/components/SelectorSync/SelectorSyncReportModal', () => ({
  __esModule: true,
  default: () => null,
}));

const mockedListDrafts = listPageDrafts as jest.MockedFunction<typeof listPageDrafts>;
const mockedListResources = listPageResources as jest.MockedFunction<typeof listPageResources>;

const draft = (
  over: Partial<PageSpecDraftSummary> & { pageKey: string },
): PageSpecDraftSummary => ({
  type: 'operation',
  resourceKey: undefined,
  title: { 'zh-CN': over.pageKey, 'en-US': over.pageKey },
  category: { key: 'misc', order: 0 },
  status: 'draft',
  draftRevision: 1,
  updatedAt: '2026-09-27T00:00:00Z',
  ...over,
});

const pageResult = (items: PageSpecDraftSummary[], total = items.length): PageDraftListResult => ({
  items,
  total,
  page: 1,
  pageSize: 20,
});

const fullParams = (overrides: Partial<PageDraftListParams> = {}): PageDraftListParams => ({
  resourceKey: undefined,
  status: undefined,
  keyword: undefined,
  page: 1,
  pageSize: 20,
  ...overrides,
});

const renderStudio = () =>
  render(
    <AntdApp>
      <PageStudio />
    </AntdApp>,
  );

describe('PageStudio 服务端搜索/状态/分页（#30）', () => {
  beforeEach(() => {
    jest.clearAllMocks();
    mockedListResources.mockResolvedValue([{ resourceKey: 'player', pageCount: 1 }]);
  });

  it('关键词回车：请求携带 keyword 且回到第 1 页', async () => {
    mockedListDrafts.mockResolvedValue(pageResult([draft({ pageKey: 'player.manage' })]));
    renderStudio();
    await waitFor(() => expect(mockedListDrafts).toHaveBeenCalled());

    const input = screen.getByPlaceholderText('搜索页面/标题/资源');
    fireEvent.change(input, { target: { value: '玩家' } });
    fireEvent.keyDown(input, { key: 'Enter', keyCode: 13 });

    await waitFor(() =>
      expect(mockedListDrafts).toHaveBeenLastCalledWith(fullParams({ keyword: '玩家' })),
    );
  });

  it('状态下拉：请求携带 status 条件', async () => {
    mockedListDrafts.mockResolvedValue(pageResult([draft({ pageKey: 'guild.manage' })]));
    renderStudio();
    await waitFor(() => expect(mockedListDrafts).toHaveBeenCalled());

    // 工具栏第一个 combobox 是状态过滤（资源过滤在第二个）
    fireEvent.mouseDown(screen.getAllByRole('combobox')[0]);
    const option = await waitFor(() => {
      const el = Array.from(
        document.querySelectorAll('.ant-select-dropdown .ant-select-item-option'),
      ).find((item) => item.textContent === '已发布');
      expect(el).toBeTruthy();
      return el as Element;
    });
    fireEvent.click(option);

    await waitFor(() =>
      expect(mockedListDrafts).toHaveBeenLastCalledWith(fullParams({ status: 'published' })),
    );
  });

  it('翻页由服务端执行：total 驱动页数，翻页请求带 page=2', async () => {
    const items = Array.from({ length: 20 }, (_, i) => draft({ pageKey: `bulk.${i + 1}` }));
    mockedListDrafts.mockResolvedValue(pageResult(items, 25));
    renderStudio();
    await waitFor(() => expect(mockedListDrafts).toHaveBeenCalled());

    const pageTwo = await waitFor(() => {
      const el = document.querySelector('.ant-pagination-item-2');
      expect(el).toBeTruthy();
      return el as HTMLElement;
    });
    fireEvent.click(pageTwo);

    await waitFor(() => expect(mockedListDrafts).toHaveBeenLastCalledWith(fullParams({ page: 2 })));
  });

  it('「涉及资源」列展示服务端计算的关联（多资源页展开；旧 payload 回退列值）', async () => {
    mockedListDrafts.mockResolvedValue(
      pageResult([
        draft({ pageKey: 'mixed.board', resources: ['order', 'player'] }),
        draft({ pageKey: 'legacy.page', resourceKey: 'guild' }),
        draft({ pageKey: 'no.resource' }),
      ]),
    );
    renderStudio();
    expect(await screen.findByText('order / player')).toBeInTheDocument();
    expect(screen.getByText('guild')).toBeInTheDocument();
    const cells = await screen.findAllByText('-');
    expect(cells.length).toBeGreaterThan(0);
  });
});
