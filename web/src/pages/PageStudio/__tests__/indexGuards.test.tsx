/**
 * PageStudio 工作台主页（index.tsx）守卫与回调收口补测
 *
 * studioActions.test.tsx 已登记一批「UI 不可达/桩边界」分支，本套件经
 * 增强桩（MergeConflictModal 提交钮 / SelectorSyncReportModal 应用钮 /
 * PageEditor onChange 触发）证明其中六条实际可达，逐条收口：
 * - 416-417 mount=1 但 focus 页面不在草稿列表（未 accept）的静默分支；
 * - 425-430 updateSelectedDraftSpec（PageEditor 桩产生 spec 变更事件）；
 * - 694-698 handleSyncSelectorsApplied（SelectorSyncReportModal 桩内触发）；
 * - 786-793 handleMerge 无效 revision 守卫（draftRevision=0）；
 * - 858-894 handleManualMergeSubmit（MergeConflictModal 桩提交）；
 * - 1159 EditorModal「一键同步 Selector」→ 打开同步报告弹窗。
 *
 * 仍不可达（入口按钮只在选中页面后渲染，UI 无法触达，不写假用例）：
 * - 830-837 handleOpenManualMerge 无页面守卫（MergeModal 仅经变更对比打开，
 *   而 handleDiff 必先设 selectedPageKey）；
 * - 902-903 / 928-929 回滚守卫（回滚入口仅在版本抽屉内渲染，抽屉必先
 *   经 handleVersions 设 selectedPageKey）。
 */
import React from 'react';
import { App as AntdApp } from 'antd';
import { configure, fireEvent, render, screen, waitFor } from '@testing-library/react';
import PageStudio from '../index';
import { history } from '@umijs/max';
import {
  getPageDraft,
  listPageDrafts,
  listPageResources,
  listPageVersions,
} from '@/services/api/pages';
import { listMenus, type MenuItem } from '@/services/api/menu';
import { getDiff, mergeChanges } from '@/services/api/versioning';
import type { PageSpec, PageSpecDraft, PageSpecDraftSummary } from '@/types/dashboard';
import type { DiffResponse, MergeResponse } from '@/services/api/versioning';

// 本机并行会话高负载下 sequential 服务链用例耗时可达 30s+，放宽到 120s
jest.setTimeout(120000);
configure({ asyncUtilTimeout: 30000 });

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

jest.mock('@/components/ProposalInbox', () => ({
  __esModule: true,
  default: ({ focusPageKey }: { focusPageKey?: string }) => (
    <div data-testid="inbox-stub" data-focus={focusPageKey ?? ''} />
  ),
}));
jest.mock('@/components/PageWorkflowGuide', () => ({ __esModule: true, default: () => null }));
// 增强桩：open 时渲染提交钮，直接触发 onSubmit（覆盖手动合并提交路径）
jest.mock('@/components/MergeConflictModal', () => ({
  __esModule: true,
  default: ({
    open,
    onSubmit,
  }: {
    open: boolean;
    onSubmit: (payload: { conflicts: unknown[]; reason?: string }) => Promise<void>;
  }) =>
    open ? (
      <div data-testid="merge-conflict-stub">
        <button type="button" onClick={() => void onSubmit({ conflicts: [], reason: 'manual' })}>
          提交
        </button>
      </div>
    ) : null,
}));
// 增强桩：open 时渲染应用钮并透出 data-testid（覆盖同步应用路径）
jest.mock('@/components/SelectorSync/SelectorSyncReportModal', () => ({
  __esModule: true,
  default: ({ open, onApplied }: { open: boolean; onApplied: (revision: number) => void }) =>
    open ? (
      <div data-testid="sync-report-stub">
        <button type="button" onClick={() => onApplied(9)}>
          应用
        </button>
      </div>
    ) : null,
}));
// 增强桩：点击即产生 spec 变更事件（覆盖 updateSelectedDraftSpec 路径）
jest.mock('@/components/PageEditor', () => ({
  __esModule: true,
  default: ({
    value,
    onChange,
    mountMenuSlot,
  }: {
    value?: PageSpecDraft;
    onChange?: (v: PageSpec) => void;
    mountMenuSlot?: React.ReactNode;
  }) => (
    <div
      data-testid="page-editor"
      onClick={() =>
        onChange?.({ ...(value as PageSpec), title: { 'zh-CN': '改后标题', 'en-US': '改后标题' } })
      }
    >
      {mountMenuSlot}
      <span data-testid="editor-spec-title">
        {((value?.title as Record<string, string> | undefined) ?? {})['zh-CN']}
      </span>
    </div>
  ),
}));
jest.mock('@/components/PageRenderer', () => ({
  __esModule: true,
  default: () => <div data-testid="page-renderer" />,
}));

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

const fullDraft = (over: Partial<PageSpecDraft> = {}): PageSpecDraft => ({
  pageKey: 'op.a',
  type: 'operation',
  title: { 'zh-CN': 'op.a', 'en-US': 'op.a' },
  category: { key: 'misc', order: 0 },
  status: 'draft',
  draftRevision: 1,
  updatedAt: '2026-09-27T00:00:00Z',
  ...over,
});

const pageResult = (items: PageSpecDraftSummary[], total = items.length) => ({
  items,
  total,
  page: 1,
  pageSize: 20,
});

const menuItem = (id: number, menuKey: string, zh: string): MenuItem => ({
  id,
  parentId: null,
  menuKey,
  labels: { 'zh-CN': zh },
  sortOrder: 0,
  isVisible: true,
  children: [],
});

const diffData = (): DiffResponse => ({
  summary: '共 1 处变更',
  autoMergeItems: [{ field: 'title', reason: '仅草稿侧修改' }],
  conflictItems: [{ field: 'query.params', reason: '双方同时修改' }],
  changes: [],
});

const renderStudio = () =>
  render(
    <AntdApp>
      <PageStudio />
    </AntdApp>,
  );

/** 行内操作按钮序：0 编辑 / 1 预览 / 2 发布或取消发布 / 3 挂载 / 4 更多。 */
const findRowBtn = async (pageKey: string, idx: number): Promise<HTMLElement> => {
  const strong = await screen.findByText(pageKey, { selector: 'strong' });
  const row = strong.closest('tr');
  if (!row) throw new Error(`row not found: ${pageKey}`);
  const btn = row.querySelectorAll('.ant-btn')[idx];
  if (!btn) throw new Error(`button #${idx} not found in row ${pageKey}`);
  return btn as HTMLElement;
};

const openMore = async (pageKey: string, itemText: string): Promise<void> => {
  fireEvent.click(await findRowBtn(pageKey, 4));
  fireEvent.click(await screen.findByText(itemText));
};

const clickPortalBtn = async (selector: string): Promise<void> => {
  const btn = await waitFor(() => {
    const el = document.querySelector(selector) as HTMLButtonElement | null;
    expect(el).toBeTruthy();
    return el;
  });
  fireEvent.click(btn);
};

beforeEach(() => {
  jest.clearAllMocks();
  (listPageDrafts as jest.Mock).mockResolvedValue(pageResult([draft({ pageKey: 'op.a' })]));
  (listPageResources as jest.Mock).mockResolvedValue([]);
  (getPageDraft as jest.Mock).mockResolvedValue(fullDraft());
  (listMenus as jest.Mock).mockResolvedValue([menuItem(1, 'player', '玩家')]);
  (getDiff as jest.Mock).mockResolvedValue(diffData());
});

afterEach(() => {
  window.history.pushState({}, '', '/');
});

describe('mount=1 静默分支（416-417）', () => {
  it('focus 页面不在草稿列表（未 accept）：不打开挂载弹窗、不报错', async () => {
    window.history.pushState({}, '', '/?focus=ghost&mount=1');
    renderStudio();

    // handleEdit('ghost') 先开编辑器（focus 联动），列表正常渲染
    expect(await screen.findByText('op.a', { selector: 'strong' })).toBeInTheDocument();
    // 挂载 effect 找不到 record → 静默返回：无挂载弹窗、无 updatePageMenu
    await waitFor(() => expect(getPageDraft).toHaveBeenCalledWith('ghost'));
    expect(screen.queryByText('挂载菜单：ghost')).not.toBeInTheDocument();
    expect(screen.queryByTestId('sync-report-stub')).not.toBeInTheDocument();
  });
});

describe('updateSelectedDraftSpec（425-430）', () => {
  it('编辑器 spec 变更事件上抛 → selectedDraft 更新并重渲染', async () => {
    renderStudio();
    fireEvent.click(await findRowBtn('op.a', 0));
    const editor = await screen.findByTestId('page-editor');
    expect(screen.getByTestId('editor-spec-title')).toHaveTextContent('op.a');

    fireEvent.click(editor);
    await waitFor(() =>
      expect(screen.getByTestId('editor-spec-title')).toHaveTextContent('改后标题'),
    );
  });
});

describe('handleSyncSelectorsApplied（694-698）', () => {
  it('同步报告弹窗应用 → 重载草稿详情并刷新列表', async () => {
    (getPageDraft as jest.Mock).mockResolvedValue(
      fullDraft({
        bindingFreshness: [
          {
            bindingId: 'bd-1',
            functionId: 'fn.stale',
            status: 'function_version_stale',
            diagnostic: { code: 'STALE', severity: 'warning', message: '契约已变化' },
          },
        ],
      }),
    );
    renderStudio();
    fireEvent.click(await findRowBtn('op.a', 0));
    // bindingFreshness 非空 → 编辑器内渲染「一键同步 Selector」
    fireEvent.click(await screen.findByRole('button', { name: /一键同步 Selector/ }));
    // 桩内「应用」单次点击 → onApplied(9) 单次触发（双击会触发两次重载）。
    // 记录点击前基数再断言增量：全量跑时前序用例的延迟调用会污染绝对次数。
    const detailBefore = (getPageDraft as jest.Mock).mock.calls.length;
    const listBefore = (listPageDrafts as jest.Mock).mock.calls.length;
    fireEvent.click(await screen.findByText('应用'));

    // 应用后：loadDraftDetail(selectedPageKey) 重载 + loadDrafts 重拉
    await waitFor(() => {
      expect((getPageDraft as jest.Mock).mock.calls.length).toBeGreaterThan(detailBefore);
      expect((listPageDrafts as jest.Mock).mock.calls.length).toBeGreaterThan(listBefore);
    });
    expect(getPageDraft).toHaveBeenLastCalledWith('op.a');
  });
});

describe('handleMerge 无效 revision 守卫（786-793）', () => {
  it('draftRevision=0：自动合并被守卫拦截，mergeChanges 不调用', async () => {
    (getPageDraft as jest.Mock).mockResolvedValue(fullDraft({ draftRevision: 0 }));
    renderStudio();
    await openMore('op.a', '变更对比');
    fireEvent.click(await screen.findByRole('button', { name: /合并变更/ }));
    fireEvent.click(await screen.findByRole('button', { name: /自动合并/ }));

    expect(await screen.findByText('页面草稿版本无效，请刷新后重试')).toBeInTheDocument();
    expect(mergeChanges).not.toHaveBeenCalled();
  });
});

describe('handleManualMergeSubmit（858-894）', () => {
  it('dry-run 预览 → 桩提交 → 手动合并成功并更新 revision', async () => {
    (mergeChanges as jest.Mock)
      .mockResolvedValueOnce({
        conflictItems: [{ field: 'query.params', reason: '双方同时修改' }],
      } as MergeResponse)
      .mockResolvedValueOnce({ draftRevision: 8, merged: 0, conflicts: 1 } as MergeResponse);
    renderStudio();
    await openMore('op.a', '变更对比');
    fireEvent.click(await screen.findByRole('button', { name: /合并变更/ }));
    fireEvent.click(await screen.findByRole('button', { name: /手动处理冲突/ }));

    // dry-run 预览就绪 → 桩弹窗打开
    const submitBtn = await waitFor(() => {
      const el = document.querySelector(
        '[data-testid="merge-conflict-stub"] button',
      ) as HTMLButtonElement | null;
      expect(el).toBeTruthy();
      return el;
    });
    fireEvent.click(submitBtn);

    expect(await screen.findByText('合并完成：草稿已更新到版本 8')).toBeInTheDocument();
    expect(mergeChanges).toHaveBeenLastCalledWith(
      'op.a',
      expect.objectContaining({ strategy: 'manual', conflicts: [], reason: 'manual' }),
    );
  });
});

describe('EditorModal 同步入口（1159）', () => {
  it('bindingFreshness 非空 → 一键同步 Selector 打开同步报告弹窗', async () => {
    (getPageDraft as jest.Mock).mockResolvedValue(
      fullDraft({
        bindingFreshness: [
          {
            bindingId: 'bd-1',
            functionId: 'fn.stale',
            status: 'function_version_stale',
            diagnostic: { code: 'STALE', severity: 'warning', message: '契约已变化' },
          },
        ],
      }),
    );
    renderStudio();
    fireEvent.click(await findRowBtn('op.a', 0));
    fireEvent.click(await screen.findByRole('button', { name: /一键同步 Selector/ }));
    expect(await screen.findByTestId('sync-report-stub')).toBeInTheDocument();
  });
});
