/**
 * PageStudio #13 资源过滤下拉：选项由服务端聚合接口（GET /pages/resources）
 * 提供（不从当前列表推导），选中后把 resourceKey 下推给列表接口重拉。
 * #30 起列表请求携带服务端分页窗口（page/pageSize），过滤全部下推。
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

// 重子组件替身：本用例只关心过滤下拉与列表参数
jest.mock('@/components/ProposalInbox', () => ({ __esModule: true, default: () => null }));
jest.mock('@/components/PageWorkflowGuide', () => ({ __esModule: true, default: () => null }));
jest.mock('@/components/MergeConflictModal', () => ({ __esModule: true, default: () => null }));
jest.mock('@/components/SelectorSync/SelectorSyncReportModal', () => ({
  __esModule: true,
  default: () => null,
}));

const mockedListDrafts = listPageDrafts as jest.MockedFunction<typeof listPageDrafts>;
const mockedListResources = listPageResources as jest.MockedFunction<typeof listPageResources>;

const draft = (pageKey: string, resourceKey?: string): PageSpecDraftSummary => ({
  pageKey,
  type: 'operation',
  resourceKey,
  title: { 'zh-CN': pageKey, 'en-US': pageKey },
  category: { key: resourceKey || 'misc', order: 0 },
  status: 'draft',
  draftRevision: 1,
  updatedAt: '2026-09-27T00:00:00Z',
});

const pageResult = (items: PageSpecDraftSummary[]): PageDraftListResult => ({
  items,
  total: items.length,
  page: 1,
  pageSize: 20,
});

/** 当前 UI 发出的完整列表参数（#30：过滤 + 分页窗口同请求下发） */
const fullParams = (overrides: Partial<PageDraftListParams> = {}): PageDraftListParams => ({
  resourceKey: undefined,
  status: undefined,
  keyword: undefined,
  page: 1,
  pageSize: 20,
  ...overrides,
});

/** 资源过滤下拉是工具栏第二个 combobox（第一个是状态过滤，#30）。 */
const resourceCombobox = (): HTMLElement => screen.getAllByRole('combobox')[1] as HTMLElement;

const openDropdownLabels = async (): Promise<(string | null)[]> => {
  fireEvent.mouseDown(resourceCombobox());
  await waitFor(() =>
    expect(document.querySelectorAll('.ant-select-dropdown .ant-select-item-option').length),
  );
  return Array.from(document.querySelectorAll('.ant-select-dropdown .ant-select-item-option')).map(
    (el) => el.textContent,
  );
};

/** 按展示文本点选下拉项（antd 选项内容可能被拆分节点，用 textContent 匹配）。 */
const clickOptionByText = async (text: string) => {
  fireEvent.mouseDown(resourceCombobox());
  await waitFor(() => {
    const contents = Array.from(
      document.querySelectorAll('.ant-select-dropdown .ant-select-item-option'),
    );
    expect(contents.some((el) => el.textContent === text)).toBe(true);
  });
  const target = Array.from(
    document.querySelectorAll('.ant-select-dropdown .ant-select-item-option'),
  ).find((el) => el.textContent === text);
  fireEvent.click(target as Element);
};

describe('PageStudio 资源过滤（#13）', () => {
  beforeEach(() => {
    jest.clearAllMocks();
    mockedListResources.mockResolvedValue([
      { resourceKey: 'guild', pageCount: 1 },
      { resourceKey: 'player', pageCount: 2 },
    ]);
  });

  it('选项来自服务端聚合（带页面数），不从当前列表推导', async () => {
    mockedListDrafts.mockResolvedValue(pageResult([draft('player.manage', 'player')]));
    render(
      <AntdApp>
        <PageStudio />
      </AntdApp>,
    );
    await waitFor(() => expect(mockedListResources).toHaveBeenCalled());
    expect(await openDropdownLabels()).toEqual(['guild (1)', 'player (2)']);
  });

  it('选中资源后列表请求带 resourceKey 条件（下推服务端，含分页窗口）', async () => {
    mockedListDrafts.mockResolvedValue(
      pageResult([draft('player.manage', 'player'), draft('guild.manage', 'guild')]),
    );
    render(
      <AntdApp>
        <PageStudio />
      </AntdApp>,
    );
    await waitFor(() => expect(mockedListDrafts).toHaveBeenCalled());
    expect(mockedListDrafts.mock.calls[0][0]).toEqual(fullParams());

    fireEvent.mouseDown(resourceCombobox());
    await clickOptionByText('player (2)');

    await waitFor(() =>
      expect(mockedListDrafts).toHaveBeenLastCalledWith(fullParams({ resourceKey: 'player' })),
    );
  });

  it('清除资源过滤后恢复全量列表请求', async () => {
    mockedListDrafts.mockResolvedValue(pageResult([draft('player.manage', 'player')]));
    render(
      <AntdApp>
        <PageStudio />
      </AntdApp>,
    );
    await waitFor(() => expect(mockedListDrafts).toHaveBeenCalled());

    await clickOptionByText('player (2)');
    await waitFor(() =>
      expect(mockedListDrafts).toHaveBeenLastCalledWith(fullParams({ resourceKey: 'player' })),
    );

    // antd 的清除手柄是 span.ant-select-clear（内含 close-circle 图标），不是 button
    fireEvent.mouseEnter(resourceCombobox());
    const clearBtn = await waitFor(() => {
      const el = document.querySelector('.ant-select-clear');
      expect(el).toBeTruthy();
      return el as Element;
    });
    fireEvent.click(clearBtn);

    await waitFor(() => expect(mockedListDrafts).toHaveBeenLastCalledWith(fullParams()));
  });
});
